package harness_test

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/harness"
)

// goodReport answers definition(): both test cases passed.
func goodReport() *harness.Report {
	return &harness.Report{
		SubmissionID: 128, TotalScore: 50, MaxScore: 50,
		Results: []harness.Result{
			{TestCaseID: 501, ExternalID: "tc_01", Status: harness.StatusPassed, Score: 20, ExecTimeMS: 5, MemoryKB: 9000, MemorySource: harness.MemoryCgroupPeak},
			{TestCaseID: 502, ExternalID: "tc_02", Status: harness.StatusPassed, Score: 30, ExecTimeMS: 5, MemoryKB: 9000, MemorySource: harness.MemoryCgroupPeak},
		},
	}
}

func TestCheckAcceptsAConsistentReport(t *testing.T) {
	if err := goodReport().Check(definition()); err != nil {
		t.Fatalf("Check: %v", err)
	}
}

func TestCheckAcceptsPartialScoresAndEveryStatus(t *testing.T) {
	r := goodReport()
	r.Results[0].Status, r.Results[0].Score = harness.StatusSecurityViolation, 0
	r.Results[0].Violation = &harness.Violation{SyscallNr: 41}
	r.Results[1].Status, r.Results[1].Score = harness.StatusFailed, 10
	r.TotalScore = 10
	if err := r.Check(definition()); err != nil {
		t.Fatalf("Check: %v", err)
	}
}

func TestCheckRejects(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*harness.Report)
		want   string
	}{
		{"another submission", func(r *harness.Report) { r.SubmissionID = 129 }, "submission"},
		{"a missing result", func(r *harness.Report) { r.Results = r.Results[:1]; r.TotalScore, r.MaxScore = 20, 20 }, "1 results for 2"},
		{"an extra result", func(r *harness.Report) {
			r.Results = append(r.Results, harness.Result{TestCaseID: 503, Status: harness.StatusPassed, MemorySource: harness.MemoryRusage})
		}, "3 results for 2"},
		{"an unknown test case", func(r *harness.Report) { r.Results[1].TestCaseID = 999 }, "unknown test case 999"},
		{"a duplicated test case", func(r *harness.Report) { r.Results[1].TestCaseID = 501 }, "two results for test case 501"},
		{"an unknown status", func(r *harness.Report) { r.Results[0].Status = "OK" }, `status "OK"`},
		{"an empty status", func(r *harness.Report) { r.Results[0].Status = "" }, "status"},
		{"an unknown memory source", func(r *harness.Report) { r.Results[0].MemorySource = "GUESS" }, "memory_source"},
		{"a score above the weight", func(r *harness.Report) { r.Results[0].Score = 21; r.TotalScore = 51 }, "scores 21, weight is 20"},
		{"a negative score", func(r *harness.Report) { r.Results[0].Score = -1; r.TotalScore = 29 }, "scores -1"},
		{"a negative time", func(r *harness.Report) { r.Results[0].ExecTimeMS = -1 }, "negative measurement"},
		{"a negative memory", func(r *harness.Report) { r.Results[0].MemoryKB = -1 }, "negative measurement"},
		{"a negative syscall", func(r *harness.Report) { r.Results[0].Violation = &harness.Violation{SyscallNr: -1} }, "syscall"},
		{"a wrong total", func(r *harness.Report) { r.TotalScore = 40 }, "totals 40/50"},
		{"a wrong maximum", func(r *harness.Report) { r.MaxScore = 100 }, "totals 50/100"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := goodReport()
			tc.mutate(r)
			err := r.Check(definition())
			if !errors.Is(err, harness.ErrReport) {
				t.Fatalf("err = %v, want ErrReport", err)
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Errorf("err = %q, want it to mention %q", err, tc.want)
			}
		})
	}
}

func TestCheckRejectsADefinitionWithARepeatedTestCase(t *testing.T) {
	def := definition()
	def.TestCases[1].ID = def.TestCases[0].ID
	if err := goodReport().Check(def); !errors.Is(err, harness.ErrReport) {
		t.Fatalf("err = %v, want ErrReport", err)
	}
}

func TestParse(t *testing.T) {
	enc, err := json.Marshal(goodReport())
	if err != nil {
		t.Fatal(err)
	}

	t.Run("round trip", func(t *testing.T) {
		r, err := harness.Parse(enc)
		if err != nil || r.TotalScore != 50 || len(r.Results) != 2 {
			t.Fatalf("Parse = %+v, %v", r, err)
		}
	})
	t.Run("ignores fields it does not know", func(t *testing.T) {
		withExtra := strings.Replace(string(enc), `"results"`, `"harness_version":"2","results"`, 1)
		if _, err := harness.Parse([]byte(withExtra)); err != nil {
			t.Fatalf("Parse: %v", err)
		}
	})
	t.Run("keeps the optional parts", func(t *testing.T) {
		src := `{"results":[{"error_message":"boom","violation":{"syscall_nr":41}}]}`
		r, err := harness.Parse([]byte(src))
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		if res := r.Results[0]; res.ErrorMessage == nil || *res.ErrorMessage != "boom" || res.Violation.SyscallNr != 41 {
			t.Fatalf("result = %+v", res)
		}
	})
	t.Run("replaces NUL in every string field", func(t *testing.T) {
		src := `{"results":[{"external_id":"a\u0000","actual_output":"b\u0000","error_message":"c\u0000"}]}`
		r, err := harness.Parse([]byte(src))
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		res := r.Results[0]
		if res.ExternalID != "a�" || res.ActualOutput != "b�" || *res.ErrorMessage != "c�" {
			t.Fatalf("result = %+v", res)
		}
	})
	t.Run("replaces invalid UTF-8", func(t *testing.T) {
		r, err := harness.Parse([]byte("{\"results\":[{\"actual_output\":\"a\xffb\"}]}"))
		if err != nil || r.Results[0].ActualOutput != "a�b" {
			t.Fatalf("Parse = %+v, %v", r, err)
		}
	})
	for name, src := range map[string]string{
		"empty output":       "",
		"text":               "Traceback (most recent call last)",
		"a truncated JSON":   string(enc[:len(enc)/2]),
		"two documents":      string(enc) + string(enc),
		"trailing garbage":   string(enc) + " x",
		"a wrong field type": `{"submission_id":"128"}`,
	} {
		t.Run("rejects "+name, func(t *testing.T) {
			if _, err := harness.Parse([]byte(src)); !errors.Is(err, harness.ErrReport) {
				t.Fatalf("err = %v, want ErrReport", err)
			}
		})
	}
}
