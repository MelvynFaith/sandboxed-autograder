package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/auth"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/courses"
)

// yamlFixture is a course with one lecturer, an enrolled student and an assignment.
type yamlFixture struct {
	env                 *testEnvironment
	lecturerAccess      string
	otherLecturerAccess string
	studentAccess       string
	otherStudentAccess  string
	adminAccess         string
	studentID           int64
	coursePath          string
	assignmentID        int64
	assignmentPath      string
}

func newYAMLFixture(t *testing.T) *yamlFixture {
	t.Helper()
	env := newTestEnvironment(t)
	f := &yamlFixture{env: env}
	_, f.lecturerAccess = createCourseTestUser(t, env, "Lecturer", "lecturer@example.test", auth.RoleLecturer)
	_, f.otherLecturerAccess = createCourseTestUser(t, env, "Other Lecturer", "other@example.test", auth.RoleLecturer)
	student, studentAccess := createCourseTestUser(t, env, "Student", "student@example.test", auth.RoleStudent)
	f.studentID, f.studentAccess = student.ID, studentAccess
	_, f.otherStudentAccess = createCourseTestUser(t, env, "Other Student", "other-student@example.test", auth.RoleStudent)
	_, f.adminAccess = createCourseTestUser(t, env, "Admin", "admin@example.test", auth.RoleAdmin)

	response, course := env.request(t, http.MethodPost, "/api/v1/courses", map[string]string{
		"name": "Algorithms", "code": "CS101", "term": "2026",
	}, f.lecturerAccess)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create course = %d, %#v", response.StatusCode, course)
	}
	f.coursePath = fmt.Sprintf("/api/v1/courses/%d", int64Field(t, course, "id"))
	if enrolled, _ := env.request(t, http.MethodPost, f.coursePath+"/enrollments", map[string]string{
		"email": student.Email,
	}, f.lecturerAccess); enrolled.StatusCode != http.StatusNoContent {
		t.Fatalf("enroll student = %d", enrolled.StatusCode)
	}
	f.assignmentID, f.assignmentPath = f.newAssignment(t, "Rekursi")
	return f
}

func (f *yamlFixture) newAssignment(t *testing.T, title string) (int64, string) {
	t.Helper()
	response, assignment := f.env.request(t, http.MethodPost, f.coursePath+"/assignments", map[string]any{
		"title": title, "description": "",
		"deadline_at": "2026-10-20T12:00:00Z", "late_until_at": "2026-10-21T12:00:00Z",
		"resource_limit": map[string]int{
			"cpu_time_ms": 2000, "memory_mb": 64, "max_processes": 5,
			"timeout_s": 5, "max_output_bytes": 1 << 20, "tmpfs_size_mb": 32,
		},
	}, f.lecturerAccess)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create assignment = %d, %#v", response.StatusCode, assignment)
	}
	id := int64Field(t, assignment, "id")
	return id, fmt.Sprintf("/api/v1/assignments/%d", id)
}

// rawRequest sends a non-JSON body and returns the status, content type and body unparsed.
func (f *yamlFixture) rawRequest(t *testing.T, method, path, body, token string) (*http.Response, string) {
	t.Helper()
	request, err := http.NewRequest(method, f.env.server.URL+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/yaml")
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := f.env.server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	data, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatal(err)
	}
	return response, string(data)
}

func (f *yamlFixture) export(t *testing.T, path, token string) string {
	t.Helper()
	response, body := f.rawRequest(t, http.MethodGet, path+"/testcases/export", "", token)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("export %s = %d, %s", path, response.StatusCode, body)
	}
	return body
}

// importYAML posts a document and decodes the JSON answer.
func (f *yamlFixture) importYAML(t *testing.T, path, document, token string) (int, map[string]any) {
	t.Helper()
	response, body := f.rawRequest(t, http.MethodPost, path+"/testcases/import", document, token)
	payload := map[string]any{}
	if err := json.Unmarshal([]byte(body), &payload); err != nil {
		t.Fatalf("decode import answer (status %d): %v: %q", response.StatusCode, err, body)
	}
	return response.StatusCode, payload
}

type storedCase struct {
	ExternalID, Input, ExpectedOutput, MatchMode string
	Weight                                       int
	IsHidden, IsDeleted                          bool
}

func (f *yamlFixture) storedCases(t *testing.T, assignmentID int64) map[string]storedCase {
	t.Helper()
	rows, err := f.env.pool.Query(t.Context(), `
		SELECT external_id, input, expected_output, weight, is_hidden, match_mode, is_deleted
		FROM test_cases WHERE assignment_id = $1
	`, assignmentID)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	stored := map[string]storedCase{}
	for rows.Next() {
		var c storedCase
		if err := rows.Scan(&c.ExternalID, &c.Input, &c.ExpectedOutput, &c.Weight, &c.IsHidden, &c.MatchMode, &c.IsDeleted); err != nil {
			t.Fatal(err)
		}
		stored[c.ExternalID] = c
	}
	if err := rows.Err(); err != nil {
		t.Fatal(err)
	}
	return stored
}

func (f *yamlFixture) insertSubmission(t *testing.T) int64 {
	t.Helper()
	var id int64
	if err := f.env.pool.QueryRow(t.Context(), `
		INSERT INTO submissions (user_id, assignment_id, code_path, original_filename, size_bytes)
		VALUES ($1, $2, 'test/path', 'solution.py', 1) RETURNING id
	`, f.studentID, f.assignmentID).Scan(&id); err != nil {
		t.Fatal(err)
	}
	return id
}

func (f *yamlFixture) setStale(t *testing.T, submissionID int64, stale bool) {
	t.Helper()
	if _, err := f.env.pool.Exec(t.Context(), `UPDATE submissions SET stale_grade = $2 WHERE id = $1`, submissionID, stale); err != nil {
		t.Fatal(err)
	}
}

const yamlLimit = `resource_limit:
  cpu_time_ms: 2000
  memory_mb: 64
  tmpfs_size_mb: 32
  max_processes: 5
  timeout_s: 5
  max_output_bytes: 1048576
`

func TestTestCaseYAMLRoundTripKeepsEveryField(t *testing.T) {
	f := newYAMLFixture(t)
	// Strings YAML tends to mangle: block scalars, leading and trailing spaces, tabs, CRLF, values
	// that look like other types, an empty string, multi-byte text.
	cases := []map[string]any{
		{"external_id": "tc_01", "input": "5\n", "expected_output": "120\n", "weight": 20, "is_hidden": false, "match_mode": "EXACT"},
		{"external_id": "hidden_case", "input": "  secret input  \n\n", "expected_output": "a\tb\r\nc", "weight": 30, "is_hidden": true, "match_mode": "WHITESPACE_INSENSITIVE"},
		{"external_id": "looks_typed", "input": "123", "expected_output": "null", "weight": 1, "is_hidden": false, "match_mode": "EXACT"},
		{"external_id": "empty", "input": "", "expected_output": "true", "weight": 2, "is_hidden": true, "match_mode": "EXACT"},
		{"external_id": "unicode", "input": "halo dunia: \"kutip\" # bukan komentar\n", "expected_output": "héllo 世界 ✓\n", "weight": 3, "is_hidden": false, "match_mode": "EXACT"},
	}
	for _, c := range cases {
		response, payload := f.env.request(t, http.MethodPost, f.assignmentPath+"/testcases", c, f.lecturerAccess)
		if response.StatusCode != http.StatusCreated {
			t.Fatalf("create %v = %d, %#v", c["external_id"], response.StatusCode, payload)
		}
	}
	// A soft-deleted test case must not be exported.
	retired := f.storedCases(t, f.assignmentID)
	if len(retired) != len(cases) {
		t.Fatalf("stored %d test cases; want %d", len(retired), len(cases))
	}
	if _, err := f.env.pool.Exec(t.Context(), `
		UPDATE test_cases SET is_deleted = true WHERE assignment_id = $1 AND external_id = 'empty'
	`, f.assignmentID); err != nil {
		t.Fatal(err)
	}

	response, body := f.rawRequest(t, http.MethodGet, f.assignmentPath+"/testcases/export", "", f.lecturerAccess)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("export = %d, %s", response.StatusCode, body)
	}
	if got := response.Header.Get("Content-Type"); !strings.HasPrefix(got, "application/yaml") {
		t.Fatalf("export Content-Type = %q; want application/yaml", got)
	}
	if got := response.Header.Get("Content-Disposition"); !strings.HasPrefix(got, "attachment;") ||
		!strings.Contains(got, fmt.Sprintf("testcases-assignment-%d.yaml", f.assignmentID)) {
		t.Fatalf("export Content-Disposition = %q", got)
	}
	for _, want := range []string{"schema_version: 1", "assignment: Rekursi", "cpu_time_ms: 2000", "external_id: tc_01"} {
		if !strings.Contains(body, want) {
			t.Fatalf("export lacks %q:\n%s", want, body)
		}
	}
	if strings.Contains(body, "external_id: empty") {
		t.Fatalf("export contains a soft-deleted test case:\n%s", body)
	}

	// Import into a second assignment and compare field by field.
	otherID, otherPath := f.newAssignment(t, "Salinan")
	status, payload := f.importYAML(t, otherPath, body, f.lecturerAccess)
	if status != http.StatusOK || payload["created"] != float64(4) || payload["updated"] != float64(0) {
		t.Fatalf("import = %d, %#v; want created 4, updated 0", status, payload)
	}
	got := f.storedCases(t, otherID)
	for _, c := range cases {
		id := c["external_id"].(string)
		if id == "empty" {
			if _, exists := got[id]; exists {
				t.Fatal("soft-deleted test case was imported")
			}
			continue
		}
		want := storedCase{
			ExternalID: id, Input: c["input"].(string), ExpectedOutput: c["expected_output"].(string),
			Weight: c["weight"].(int), IsHidden: c["is_hidden"].(bool), MatchMode: c["match_mode"].(string),
		}
		if got[id] != want {
			t.Errorf("round trip of %s = %+v; want %+v", id, got[id], want)
		}
	}
	if !got["hidden_case"].IsHidden {
		t.Error("hidden flag was not preserved")
	}

	// The second export differs only by the assignment title, and importing it again changes nothing.
	again := f.export(t, otherPath, f.lecturerAccess)
	if strings.Replace(again, "assignment: Salinan", "assignment: Rekursi", 1) != body {
		t.Errorf("second export differs:\n%s\nfirst:\n%s", again, body)
	}
	status, payload = f.importYAML(t, otherPath, again, f.lecturerAccess)
	if status != http.StatusOK || payload["created"] != float64(0) || payload["updated"] != float64(0) {
		t.Fatalf("re-import = %d, %#v; want 0 created, 0 updated", status, payload)
	}
}

func TestImportTestCasesCreatesUpdatesAndNeverDeletes(t *testing.T) {
	f := newYAMLFixture(t)
	first := "schema_version: 1\n" + yamlLimit + `test_cases:
  - external_id: tc_01
    input: "1\n"
    expected_output: "1\n"
    weight: 10
    match_mode: EXACT
  - external_id: tc_02
    input: "2\n"
    expected_output: "2\n"
    weight: 10
    match_mode: EXACT
`
	if status, payload := f.importYAML(t, f.assignmentPath, first, f.lecturerAccess); status != http.StatusOK ||
		payload["created"] != float64(2) || payload["updated"] != float64(0) {
		t.Fatalf("first import = %d, %#v", status, payload)
	}
	submissionID := f.insertSubmission(t)
	f.setStale(t, submissionID, false)

	// Re-importing the same content is a no-op: nothing is stale afterwards.
	if status, payload := f.importYAML(t, f.assignmentPath, first, f.lecturerAccess); status != http.StatusOK ||
		payload["created"] != float64(0) || payload["updated"] != float64(0) {
		t.Fatalf("unchanged import = %d, %#v", status, payload)
	}
	assertSubmissionStale(t, f.env, submissionID, false)

	// A resource limit change alone does not mark submissions stale, like PUT /resource-limit.
	limitOnly := "schema_version: 1\nresource_limit:\n  cpu_time_ms: 3000\n  memory_mb: 80\n  tmpfs_size_mb: 32\n  max_processes: 7\n  timeout_s: 8\n  max_output_bytes: 1048576\n"
	if status, payload := f.importYAML(t, f.assignmentPath, limitOnly, f.lecturerAccess); status != http.StatusOK ||
		payload["created"] != float64(0) || payload["updated"] != float64(0) {
		t.Fatalf("limit-only import = %d, %#v", status, payload)
	}
	assertSubmissionStale(t, f.env, submissionID, false)
	var cpu, memory, processes, timeout int
	if err := f.env.pool.QueryRow(t.Context(), `
		SELECT cpu_time_ms, memory_mb, max_processes, timeout_s FROM resource_limits WHERE assignment_id = $1
	`, f.assignmentID).Scan(&cpu, &memory, &processes, &timeout); err != nil {
		t.Fatal(err)
	}
	if cpu != 3000 || memory != 80 || processes != 7 || timeout != 8 {
		t.Fatalf("resource limit = %d %d %d %d; want 3000 80 7 8", cpu, memory, processes, timeout)
	}

	// tc_01 changes, tc_03 is new, tc_02 is absent from the file and must survive.
	second := `schema_version: 1
test_cases:
  - external_id: tc_01
    input: "1\n"
    expected_output: "uno\n"
    weight: 15
    is_hidden: true
    match_mode: WHITESPACE_INSENSITIVE
  - external_id: tc_03
    input: "3\n"
    expected_output: "3\n"
    weight: 5
    match_mode: EXACT
`
	status, payload := f.importYAML(t, f.assignmentPath, second, f.lecturerAccess)
	if status != http.StatusOK || payload["created"] != float64(1) || payload["updated"] != float64(1) {
		t.Fatalf("second import = %d, %#v; want created 1, updated 1", status, payload)
	}
	stored := f.storedCases(t, f.assignmentID)
	if len(stored) != 3 {
		t.Fatalf("stored %d test cases; want 3 (never delete): %+v", len(stored), stored)
	}
	if want := (storedCase{"tc_01", "1\n", "uno\n", "WHITESPACE_INSENSITIVE", 15, true, false}); stored["tc_01"] != want {
		t.Errorf("tc_01 = %+v; want %+v", stored["tc_01"], want)
	}
	if stored["tc_02"].IsDeleted || stored["tc_02"].Weight != 10 {
		t.Errorf("tc_02 was touched: %+v", stored["tc_02"])
	}
	assertSubmissionStale(t, f.env, submissionID, true)

	// The id of an updated test case stays, so old results keep their link.
	var idBefore, idAfter int64
	if err := f.env.pool.QueryRow(t.Context(), `SELECT id FROM test_cases WHERE assignment_id = $1 AND external_id = 'tc_01'`, f.assignmentID).Scan(&idBefore); err != nil {
		t.Fatal(err)
	}
	third := strings.Replace(second, "weight: 15", "weight: 16", 1)
	f.importYAML(t, f.assignmentPath, third, f.lecturerAccess)
	if err := f.env.pool.QueryRow(t.Context(), `SELECT id FROM test_cases WHERE assignment_id = $1 AND external_id = 'tc_01'`, f.assignmentID).Scan(&idAfter); err != nil || idAfter != idBefore {
		t.Fatalf("tc_01 id %d -> %d (%v); want unchanged", idBefore, idAfter, err)
	}

	// A soft-deleted test case that the file mentions comes back with the same id.
	if _, err := f.env.pool.Exec(t.Context(), `UPDATE test_cases SET is_deleted = true WHERE assignment_id = $1 AND external_id = 'tc_03'`, f.assignmentID); err != nil {
		t.Fatal(err)
	}
	if status, payload := f.importYAML(t, f.assignmentPath, third, f.lecturerAccess); status != http.StatusOK ||
		payload["created"] != float64(0) || payload["updated"] != float64(1) {
		t.Fatalf("restore import = %d, %#v", status, payload)
	}
	if f.storedCases(t, f.assignmentID)["tc_03"].IsDeleted {
		t.Error("soft-deleted tc_03 was not restored")
	}
}

func TestImportTestCasesIsAllOrNothing(t *testing.T) {
	f := newYAMLFixture(t)
	if status, payload := f.importYAML(t, f.assignmentPath, "schema_version: 1\n"+yamlLimit+`test_cases:
  - {external_id: tc_01, input: "a", expected_output: "b", weight: 10, match_mode: EXACT}
`, f.lecturerAccess); status != http.StatusOK {
		t.Fatalf("seed import = %d, %#v", status, payload)
	}
	before := f.storedCases(t, f.assignmentID)

	t.Run("validation failure on the last case", func(t *testing.T) {
		status, payload := f.importYAML(t, f.assignmentPath, `schema_version: 1
resource_limit: {cpu_time_ms: 9000, memory_mb: 128, tmpfs_size_mb: 32, max_processes: 5, timeout_s: 5, max_output_bytes: 1048576}
test_cases:
  - {external_id: tc_01, input: "changed", expected_output: "b", weight: 10, match_mode: EXACT}
  - {external_id: tc_new, input: "x", expected_output: "y", weight: 10, match_mode: EXACT}
  - {external_id: tc_bad, input: "x", expected_output: "y", weight: 0, match_mode: EXACT}
`, f.lecturerAccess)
		if status != http.StatusBadRequest {
			t.Fatalf("status = %d, %#v; want 400", status, payload)
		}
		assertError(t, payload, "invalid_yaml")
		if message := errorMessage(t, payload); !strings.Contains(message, "test_cases[2].weight") {
			t.Fatalf("message %q does not name test_cases[2].weight", message)
		}
		f.assertUnchanged(t, before)
	})

	t.Run("database failure after earlier writes", func(t *testing.T) {
		// A NUL byte passes the Go side when the store is called directly, and PostgreSQL refuses it
		// at the last statement: the limit and the earlier test cases must roll back.
		_, err := courses.NewStore(f.env.pool).ImportTestCases(t.Context(), f.assignmentID,
			&courses.ResourceLimit{CPUTimeMS: 9000, MemoryMB: 128, MaxProcesses: 5, TimeoutS: 5, MaxOutputBytes: 1 << 20, TmpfsSizeMB: 32},
			[]courses.TestCase{
				{ExternalID: "tc_01", Input: "changed", ExpectedOutput: "b", Weight: 10, MatchMode: "EXACT"},
				{ExternalID: "tc_new", Input: "x", ExpectedOutput: "y", Weight: 10, MatchMode: "EXACT"},
				{ExternalID: "tc_nul", Input: "x\x00", ExpectedOutput: "y", Weight: 10, MatchMode: "EXACT"},
			})
		if err == nil {
			t.Fatal("import with a NUL byte succeeded")
		}
		f.assertUnchanged(t, before)
	})
}

func (f *yamlFixture) assertUnchanged(t *testing.T, before map[string]storedCase) {
	t.Helper()
	after := f.storedCases(t, f.assignmentID)
	if len(after) != len(before) || after["tc_01"] != before["tc_01"] {
		t.Fatalf("test cases changed: before %+v, after %+v", before, after)
	}
	var cpu int
	if err := f.env.pool.QueryRow(t.Context(), `SELECT cpu_time_ms FROM resource_limits WHERE assignment_id = $1`, f.assignmentID).Scan(&cpu); err != nil {
		t.Fatal(err)
	}
	if cpu != 2000 {
		t.Fatalf("cpu_time_ms = %d; want the original 2000", cpu)
	}
}

func TestImportTestCasesRejectsInvalidDocuments(t *testing.T) {
	f := newYAMLFixture(t)
	good := `{external_id: tc_01, input: "a", expected_output: "b", weight: 10, match_mode: EXACT}`
	tests := []struct {
		name     string
		document string
		field    string // text the message must contain
	}{
		{"empty body", "", "kosong"},
		{"not yaml", "schema_version: [1\n", "tidak valid"},
		{"missing schema_version", "test_cases: [" + good + "]\n", "schema_version"},
		{"unsupported schema_version", "schema_version: 2\ntest_cases: [" + good + "]\n", "schema_version"},
		{"schema_version as text", "schema_version: \"1\"\n", "line 1"},
		{"unknown top-level key", "schema_version: 1\nextra: 1\n", "extra"},
		{"unknown test case key", "schema_version: 1\ntest_cases:\n  - {external_id: tc_01, wieght: 5, match_mode: EXACT}\n", "wieght"},
		{"two documents", "schema_version: 1\n---\nschema_version: 1\n", "satu dokumen"},
		{"weight as text", "schema_version: 1\ntest_cases:\n  - {external_id: tc_01, weight: many, match_mode: EXACT}\n", "line 3"},
		{"missing external_id", "schema_version: 1\ntest_cases:\n  - {weight: 1, match_mode: EXACT}\n", "test_cases[0].external_id"},
		{"blank external_id", "schema_version: 1\ntest_cases:\n  - {external_id: \"  \", weight: 1, match_mode: EXACT}\n", "test_cases[0].external_id"},
		{"duplicate external_id", "schema_version: 1\ntest_cases:\n  - " + good + "\n  - " + good + "\n", "test_cases[1].external_id"},
		{"zero weight", "schema_version: 1\ntest_cases:\n  - {external_id: tc_01, weight: 0, match_mode: EXACT}\n", "test_cases[0].weight"},
		{"negative weight", "schema_version: 1\ntest_cases:\n  - {external_id: tc_01, weight: -4, match_mode: EXACT}\n", "test_cases[0].weight"},
		{"weight beyond int32", "schema_version: 1\ntest_cases:\n  - {external_id: tc_01, weight: 3000000000, match_mode: EXACT}\n", "test_cases[0].weight"},
		{"missing match_mode", "schema_version: 1\ntest_cases:\n  - {external_id: tc_01, weight: 1}\n", "test_cases[0].match_mode"},
		{"unknown match_mode", "schema_version: 1\ntest_cases:\n  - {external_id: tc_01, weight: 1, match_mode: REGEX}\n", "test_cases[0].match_mode"},
		{"NUL in input", "schema_version: 1\ntest_cases:\n  - {external_id: tc_01, input: \"a\\0b\", weight: 1, match_mode: EXACT}\n", "test_cases[0].input"},
		{"NUL in expected_output", "schema_version: 1\ntest_cases:\n  - {external_id: tc_01, expected_output: \"a\\0b\", weight: 1, match_mode: EXACT}\n", "test_cases[0].expected_output"},
		{"limit field missing", "schema_version: 1\nresource_limit: {cpu_time_ms: 2000, memory_mb: 64, tmpfs_size_mb: 32, max_processes: 5, timeout_s: 5}\n", "resource_limit.max_output_bytes"},
		{"limit field zero", "schema_version: 1\nresource_limit: {cpu_time_ms: 0, memory_mb: 64, tmpfs_size_mb: 32, max_processes: 5, timeout_s: 5, max_output_bytes: 1048576}\n", "resource_limit.cpu_time_ms"},
		{"limit beyond int32", "schema_version: 1\nresource_limit: {cpu_time_ms: 2000, memory_mb: 5000000000, tmpfs_size_mb: 32, max_processes: 5, timeout_s: 5, max_output_bytes: 1048576}\n", "resource_limit.memory_mb"},
		{"tmpfs does not fit memory", "schema_version: 1\nresource_limit: {cpu_time_ms: 2000, memory_mb: 40, tmpfs_size_mb: 32, max_processes: 5, timeout_s: 5, max_output_bytes: 1048576}\n", "tmpfs_size_mb + 16"},
		{"output does not fit tmpfs", "schema_version: 1\nresource_limit: {cpu_time_ms: 2000, memory_mb: 64, tmpfs_size_mb: 1, max_processes: 5, timeout_s: 5, max_output_bytes: 1048576}\n", "2 * max_output_bytes"},
	}
	before := f.storedCases(t, f.assignmentID)
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			status, payload := f.importYAML(t, f.assignmentPath, test.document, f.lecturerAccess)
			if status != http.StatusBadRequest {
				t.Fatalf("status = %d, %#v; want 400", status, payload)
			}
			assertError(t, payload, "invalid_yaml")
			if message := errorMessage(t, payload); !strings.Contains(message, test.field) {
				t.Fatalf("message %q does not contain %q", message, test.field)
			}
		})
	}
	if after := f.storedCases(t, f.assignmentID); len(after) != len(before) {
		t.Fatalf("rejected documents created test cases: %+v", after)
	}
}

func TestImportTestCasesBodySizeLimit(t *testing.T) {
	f := newYAMLFixture(t)
	// Just under 1 MiB is accepted; over it is refused with 413 before anything is parsed.
	padding := func(size int) string { return "schema_version: 1\n# " + strings.Repeat("x", size) + "\n" }
	if status, payload := f.importYAML(t, f.assignmentPath, padding(1<<20-64), f.lecturerAccess); status != http.StatusOK {
		t.Fatalf("document under the limit = %d, %#v", status, payload)
	}
	status, payload := f.importYAML(t, f.assignmentPath, padding(1<<20), f.lecturerAccess)
	if status != http.StatusRequestEntityTooLarge {
		t.Fatalf("document over the limit = %d, %#v; want 413", status, payload)
	}
	assertError(t, payload, "payload_too_large")
}

func TestTestCaseYAMLAccessMatrix(t *testing.T) {
	f := newYAMLFixture(t)
	hiddenSecret := `{external_id: hidden_case, input: "secret input", expected_output: "secret output", weight: 1, is_hidden: true, match_mode: EXACT}`
	document := "schema_version: 1\ntest_cases:\n  - " + hiddenSecret + "\n"
	if status, payload := f.importYAML(t, f.assignmentPath, document, f.lecturerAccess); status != http.StatusOK {
		t.Fatalf("seed = %d, %#v", status, payload)
	}
	missingPath := "/api/v1/assignments/999999"

	tests := []struct {
		name  string
		path  string
		token string
		want  int // for both export and import
	}{
		{"owner lecturer", f.assignmentPath, f.lecturerAccess, http.StatusOK},
		{"admin", f.assignmentPath, f.adminAccess, http.StatusOK},
		{"other lecturer", f.assignmentPath, f.otherLecturerAccess, http.StatusNotFound},
		{"enrolled student", f.assignmentPath, f.studentAccess, http.StatusNotFound},
		{"unenrolled student", f.assignmentPath, f.otherStudentAccess, http.StatusNotFound},
		{"no token", f.assignmentPath, "", http.StatusUnauthorized},
		{"missing assignment as admin", missingPath, f.adminAccess, http.StatusNotFound},
		{"malformed id", "/api/v1/assignments/abc", f.lecturerAccess, http.StatusNotFound},
	}
	for _, test := range tests {
		t.Run(test.name+" export", func(t *testing.T) {
			response, body := f.rawRequest(t, http.MethodGet, test.path+"/testcases/export", "", test.token)
			if response.StatusCode != test.want {
				t.Fatalf("export = %d, %s; want %d", response.StatusCode, body, test.want)
			}
			if test.want == http.StatusOK && !strings.Contains(body, "secret output") {
				t.Fatalf("manager export lacks the hidden test case:\n%s", body)
			}
			if test.want != http.StatusOK && (strings.Contains(body, "secret") || strings.Contains(body, "hidden_case")) {
				t.Fatalf("refused export leaks hidden data: %s", body)
			}
		})
		t.Run(test.name+" import", func(t *testing.T) {
			response, body := f.rawRequest(t, http.MethodPost, test.path+"/testcases/import", document, test.token)
			if response.StatusCode != test.want {
				t.Fatalf("import = %d, %s; want %d", response.StatusCode, body, test.want)
			}
		})
	}

	// A refused import must not have written anything.
	other := "schema_version: 1\ntest_cases:\n  - {external_id: sneaky, weight: 1, match_mode: EXACT}\n"
	f.rawRequest(t, http.MethodPost, f.assignmentPath+"/testcases/import", other, f.studentAccess)
	f.rawRequest(t, http.MethodPost, f.assignmentPath+"/testcases/import", other, f.otherLecturerAccess)
	if _, exists := f.storedCases(t, f.assignmentID)["sneaky"]; exists {
		t.Fatal("a refused import created a test case")
	}
}
