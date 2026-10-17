package harness

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ErrReport means the Harness ran but its output cannot be stored: it is not a valid report, or it
// contradicts the definition the worker gave it.
var ErrReport = errors.New("harness: invalid report")

// Parse decodes a report. Unknown fields are ignored, so the Harness can add information without
// breaking the worker, but the output must be exactly one JSON document.
//
// NUL characters are replaced in every string: PostgreSQL text cannot store one, and a student's
// program can print it. Invalid UTF-8 is already replaced by the decoder.
func Parse(data []byte) (*Report, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	var r Report
	if err := dec.Decode(&r); err != nil {
		return nil, fmt.Errorf("%w: %v", ErrReport, err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("%w: data after the JSON document", ErrReport)
	}
	for i := range r.Results {
		res := &r.Results[i]
		res.ExternalID = clean(res.ExternalID)
		res.ActualOutput = clean(res.ActualOutput)
		if res.ErrorMessage != nil {
			msg := clean(*res.ErrorMessage)
			res.ErrorMessage = &msg
		}
	}
	return &r, nil
}

func clean(s string) string {
	if strings.IndexByte(s, 0) < 0 {
		return s
	}
	return strings.ReplaceAll(s, "\x00", "�")
}

// Check verifies that r answers def: the same submission, one result for every test case and nothing
// else, scores within each test case's weight, and totals that add up. The worker writes these
// numbers to the database in one transaction, so a report that fails here is a pipeline failure and
// is not stored.
func (r *Report) Check(def *Definition) error {
	if r.SubmissionID != def.SubmissionID {
		return fmt.Errorf("%w: report is for submission %d, expected %d", ErrReport, r.SubmissionID, def.SubmissionID)
	}
	weights := make(map[int64]int, len(def.TestCases))
	maxScore := 0
	for _, tc := range def.TestCases {
		if _, dup := weights[tc.ID]; dup {
			return fmt.Errorf("%w: definition lists test case %d twice", ErrReport, tc.ID)
		}
		weights[tc.ID] = tc.Weight
		maxScore += tc.Weight
	}
	if len(r.Results) != len(def.TestCases) {
		return fmt.Errorf("%w: %d results for %d test cases", ErrReport, len(r.Results), len(def.TestCases))
	}

	seen := make(map[int64]struct{}, len(r.Results))
	total := 0
	for i := range r.Results {
		res := &r.Results[i]
		weight, ok := weights[res.TestCaseID]
		if !ok {
			return fmt.Errorf("%w: result for unknown test case %d", ErrReport, res.TestCaseID)
		}
		if _, dup := seen[res.TestCaseID]; dup {
			return fmt.Errorf("%w: two results for test case %d", ErrReport, res.TestCaseID)
		}
		seen[res.TestCaseID] = struct{}{}

		switch {
		case !validStatus(res.Status):
			return fmt.Errorf("%w: test case %d has status %q", ErrReport, res.TestCaseID, res.Status)
		case !validMemorySource(res.MemorySource):
			return fmt.Errorf("%w: test case %d has memory_source %q", ErrReport, res.TestCaseID, res.MemorySource)
		case res.Score < 0 || res.Score > weight:
			return fmt.Errorf("%w: test case %d scores %d, weight is %d", ErrReport, res.TestCaseID, res.Score, weight)
		case res.ExecTimeMS < 0 || res.MemoryKB < 0:
			return fmt.Errorf("%w: test case %d has a negative measurement", ErrReport, res.TestCaseID)
		case res.Violation != nil && res.Violation.SyscallNr < 0:
			return fmt.Errorf("%w: test case %d has a negative syscall number", ErrReport, res.TestCaseID)
		}
		total += res.Score
	}
	if r.MaxScore != maxScore || r.TotalScore != total {
		return fmt.Errorf("%w: totals %d/%d, results add up to %d/%d",
			ErrReport, r.TotalScore, r.MaxScore, total, maxScore)
	}
	return nil
}

func validStatus(s string) bool {
	switch s {
	case StatusPassed, StatusFailed, StatusTimeout, StatusMemoryLimitExceeded,
		StatusRuntimeError, StatusSyntaxError, StatusSecurityViolation:
		return true
	}
	return false
}

func validMemorySource(s string) bool {
	switch s {
	case MemoryCgroupPeak, MemoryCgroupSampled, MemoryRusage:
		return true
	}
	return false
}
