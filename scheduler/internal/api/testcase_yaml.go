package api

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/auth"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/courses"
	"github.com/jackc/pgx/v5"
	"gopkg.in/yaml.v3"
)

// testCaseSchemaVersion is the only schema_version of the import/export file the API understands
// (SRS 7.3). A later format change bumps it, so an old server refuses a file it would misread.
const testCaseSchemaVersion = 1

// testCaseDocument is the YAML file of SRS 7.3. Its keys are the attribute names of SRS 7.1.
type testCaseDocument struct {
	SchemaVersion *int               `yaml:"schema_version"`
	Assignment    string             `yaml:"assignment"`
	ResourceLimit *yamlResourceLimit `yaml:"resource_limit,omitempty"`
	TestCases     []yamlTestCase     `yaml:"test_cases"`
}

type yamlResourceLimit struct {
	CPUTimeMS      int `yaml:"cpu_time_ms"`
	MemoryMB       int `yaml:"memory_mb"`
	TmpfsSizeMB    int `yaml:"tmpfs_size_mb"`
	MaxProcesses   int `yaml:"max_processes"`
	TimeoutS       int `yaml:"timeout_s"`
	MaxOutputBytes int `yaml:"max_output_bytes"`
}

type yamlTestCase struct {
	ExternalID     string `yaml:"external_id"`
	Input          string `yaml:"input"`
	ExpectedOutput string `yaml:"expected_output"`
	Weight         int    `yaml:"weight"`
	IsHidden       bool   `yaml:"is_hidden"`
	MatchMode      string `yaml:"match_mode"`
}

// exportTestCases answers GET /assignments/{id}/testcases/export with the SRS 7.3 document. Hidden
// test cases are included, so only the lecturer of the course and an admin may call it.
func (a *API) exportTestCases(w http.ResponseWriter, r *http.Request, user auth.User) {
	assignmentID, ok := coursePathID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Assignment tidak ditemukan.")
		return
	}
	if !a.requireAssignmentManager(w, r, user, assignmentID) {
		return
	}
	export, err := a.courses.ExportTestCases(r.Context(), assignmentID)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "Assignment tidak ditemukan.")
		return
	}
	if err != nil {
		internalError(w, "export test cases", err)
		return
	}

	limit := export.ResourceLimit
	document := testCaseDocument{
		SchemaVersion: new(int),
		Assignment:    export.AssignmentTitle,
		ResourceLimit: &yamlResourceLimit{
			CPUTimeMS: limit.CPUTimeMS, MemoryMB: limit.MemoryMB, TmpfsSizeMB: limit.TmpfsSizeMB,
			MaxProcesses: limit.MaxProcesses, TimeoutS: limit.TimeoutS, MaxOutputBytes: limit.MaxOutputBytes,
		},
		TestCases: make([]yamlTestCase, 0, len(export.TestCases)),
	}
	*document.SchemaVersion = testCaseSchemaVersion
	for _, testCase := range export.TestCases {
		document.TestCases = append(document.TestCases, yamlTestCase{
			ExternalID: testCase.ExternalID, Input: testCase.Input, ExpectedOutput: testCase.ExpectedOutput,
			Weight: testCase.Weight, IsHidden: testCase.IsHidden, MatchMode: testCase.MatchMode,
		})
	}
	encoded, err := yaml.Marshal(document)
	if err != nil {
		internalError(w, "encode test case YAML", err)
		return
	}
	w.Header().Set("Content-Type", "application/yaml; charset=utf-8")
	w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="testcases-assignment-%d.yaml"`, assignmentID))
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write(encoded)
}

// importTestCases answers POST /assignments/{id}/testcases/import. The body is the SRS 7.3 document.
// The resource limit and every test case are validated before anything is written, then applied in
// one transaction: a file with one bad entry changes nothing.
func (a *API) importTestCases(w http.ResponseWriter, r *http.Request, user auth.User) {
	assignmentID, ok := coursePathID(r, "id")
	if !ok {
		writeError(w, http.StatusNotFound, "not_found", "Assignment tidak ditemukan.")
		return
	}
	if !a.requireAssignmentManager(w, r, user, assignmentID) {
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBytes)
	data, err := io.ReadAll(r.Body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			writeError(w, http.StatusRequestEntityTooLarge, "payload_too_large", "Berkas YAML maksimal 1 MiB.")
			return
		}
		writeError(w, http.StatusBadRequest, "invalid_request", "Body tidak dapat dibaca.")
		return
	}
	limit, testCases, err := parseTestCaseDocument(data)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid_yaml", err.Error())
		return
	}

	result, err := a.courses.ImportTestCases(r.Context(), assignmentID, limit, testCases)
	if errors.Is(err, pgx.ErrNoRows) {
		writeError(w, http.StatusNotFound, "not_found", "Assignment tidak ditemukan.")
		return
	}
	if err != nil {
		internalError(w, "import test cases", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]int{"created": result.Created, "updated": result.Updated})
}

// parseTestCaseDocument decodes and validates an import file with the rules of the JSON endpoints
// (invalidTestCaseField, validResourceLimit). The returned error text is shown to the lecturer, so
// it names the offending field. The resource limit is nil when the file has none.
func parseTestCaseDocument(data []byte) (*courses.ResourceLimit, []courses.TestCase, error) {
	// Unknown keys are refused rather than ignored: a misspelt "wieght" must not silently import
	// the default weight.
	decoder := yaml.NewDecoder(bytes.NewReader(data))
	decoder.KnownFields(true)
	var document testCaseDocument
	if err := decoder.Decode(&document); err != nil {
		if errors.Is(err, io.EOF) {
			return nil, nil, errors.New("Dokumen YAML kosong.")
		}
		return nil, nil, fmt.Errorf("Dokumen YAML tidak valid: %s", strings.TrimPrefix(err.Error(), "yaml: "))
	}
	if err := decoder.Decode(&yaml.Node{}); !errors.Is(err, io.EOF) {
		return nil, nil, errors.New("Berkas YAML harus berisi satu dokumen.")
	}
	if document.SchemaVersion == nil || *document.SchemaVersion != testCaseSchemaVersion {
		return nil, nil, fmt.Errorf("schema_version wajib diisi dan harus bernilai %d.", testCaseSchemaVersion)
	}

	var limit *courses.ResourceLimit
	if document.ResourceLimit != nil {
		l := document.ResourceLimit
		fields := []struct {
			name  string
			value int
		}{
			{"cpu_time_ms", l.CPUTimeMS}, {"memory_mb", l.MemoryMB}, {"max_processes", l.MaxProcesses},
			{"timeout_s", l.TimeoutS}, {"max_output_bytes", l.MaxOutputBytes}, {"tmpfs_size_mb", l.TmpfsSizeMB},
		}
		for _, field := range fields {
			if field.value <= 0 || field.value > math.MaxInt32 {
				return nil, nil, fmt.Errorf("resource_limit.%s wajib berupa bilangan bulat positif (maksimal %d).", field.name, math.MaxInt32)
			}
		}
		if !validResourceLimit(l.CPUTimeMS, l.MemoryMB, l.MaxProcesses, l.TimeoutS, l.MaxOutputBytes, l.TmpfsSizeMB) {
			return nil, nil, fmt.Errorf("resource_limit tidak valid: %s", resourceLimitRulesMessage)
		}
		limit = &courses.ResourceLimit{
			CPUTimeMS: l.CPUTimeMS, MemoryMB: l.MemoryMB, MaxProcesses: l.MaxProcesses,
			TimeoutS: l.TimeoutS, MaxOutputBytes: l.MaxOutputBytes, TmpfsSizeMB: l.TmpfsSizeMB,
		}
	}

	testCases := make([]courses.TestCase, 0, len(document.TestCases))
	seen := make(map[string]struct{}, len(document.TestCases))
	for i, entry := range document.TestCases {
		externalID := strings.TrimSpace(entry.ExternalID)
		prefix := fmt.Sprintf("test_cases[%d]", i)
		if externalID == "" {
			return nil, nil, fmt.Errorf("%s.external_id wajib diisi.", prefix)
		}
		if _, dup := seen[externalID]; dup {
			return nil, nil, fmt.Errorf("%s.external_id %q muncul lebih dari satu kali.", prefix, externalID)
		}
		seen[externalID] = struct{}{}
		switch invalidTestCaseField(externalID, entry.Weight, entry.MatchMode) {
		case "external_id":
			return nil, nil, fmt.Errorf("%s.external_id tidak boleh mengandung NUL.", prefix)
		case "weight":
			return nil, nil, fmt.Errorf("%s.weight harus bilangan bulat positif.", prefix)
		case "match_mode":
			return nil, nil, fmt.Errorf("%s.match_mode harus EXACT atau WHITESPACE_INSENSITIVE.", prefix)
		}
		// PostgreSQL text cannot hold NUL; refuse it here instead of failing the whole transaction.
		if strings.ContainsRune(entry.Input, '\x00') {
			return nil, nil, fmt.Errorf("%s.input tidak boleh mengandung NUL.", prefix)
		}
		if strings.ContainsRune(entry.ExpectedOutput, '\x00') {
			return nil, nil, fmt.Errorf("%s.expected_output tidak boleh mengandung NUL.", prefix)
		}
		testCases = append(testCases, courses.TestCase{
			ExternalID: externalID, Input: entry.Input, ExpectedOutput: entry.ExpectedOutput,
			Weight: entry.Weight, IsHidden: entry.IsHidden, MatchMode: entry.MatchMode,
		})
	}
	return limit, testCases, nil
}
