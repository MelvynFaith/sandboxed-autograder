package api

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/auth"
)

// resultsTestGet sends an authenticated GET and returns the raw body, because the export answers
// CSV, not JSON.
func resultsTestGet(t *testing.T, env *testEnvironment, path, token string) (*http.Response, []byte) {
	t.Helper()
	request, err := http.NewRequest(http.MethodGet, env.server.URL+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		request.Header.Set("Authorization", "Bearer "+token)
	}
	response, err := env.server.Client().Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read GET %s body: %v", path, err)
	}
	return response, body
}

func resultsTestInsertSubmission(
	t *testing.T, env *testEnvironment, userID, assignmentID int64, status string,
	submittedAt time.Time, isLate, staleGrade bool, totalScore, maxScore *int,
) int64 {
	t.Helper()
	var id int64
	err := env.pool.QueryRow(t.Context(), `
		INSERT INTO submissions (
			user_id, assignment_id, code_path, original_filename, size_bytes, status,
			is_late, stale_grade, total_score, max_score, submitted_at
		) VALUES ($1, $2, $3, 'solution.py', 1, $4, $5, $6, $7, $8, $9)
		RETURNING id
	`, userID, assignmentID, fmt.Sprintf("test/results/%d/%d", userID, submittedAt.UnixNano()), status,
		isLate, staleGrade, totalScore, maxScore, submittedAt).Scan(&id)
	if err != nil {
		t.Fatalf("insert %s submission for user %d: %v", status, userID, err)
	}
	return id
}

func resultsTestEnroll(t *testing.T, env *testEnvironment, coursePath, email, token string) {
	t.Helper()
	response, payload := env.request(t, http.MethodPost, coursePath+"/enrollments", map[string]string{
		"email": email,
	}, token)
	if response.StatusCode != http.StatusNoContent {
		t.Fatalf("enroll %s = %d, %#v", email, response.StatusCode, payload)
	}
}

func resultsTestScore(value int) *int {
	return &value
}

func TestAssignmentResultsAccess(t *testing.T) {
	env := newTestEnvironment(t)
	fixture := newSubmissionFixture(t, env)
	assignmentPath := fmt.Sprintf("/api/v1/assignments/%d", fixture.assignmentID)

	for _, suffix := range []string{"/results", "/results/export"} {
		for _, actor := range []struct {
			name  string
			token string
			want  int
			code  string
		}{
			{"owning lecturer", fixture.lecturerAccess, http.StatusOK, ""},
			{"admin", fixture.adminAccess, http.StatusOK, ""},
			{"other lecturer", fixture.otherLecturerAuth, http.StatusNotFound, "not_found"},
			{"enrolled student", fixture.studentAccess, http.StatusForbidden, "forbidden"},
			{"unenrolled student", fixture.otherStudentAuth, http.StatusNotFound, "not_found"},
			{"missing token", "", http.StatusUnauthorized, "unauthorized"},
		} {
			t.Run(actor.name+suffix, func(t *testing.T) {
				response, body := resultsTestGet(t, env, assignmentPath+suffix, actor.token)
				if response.StatusCode != actor.want {
					t.Fatalf("GET %s as %s = %d, %s; want %d", suffix, actor.name, response.StatusCode, body, actor.want)
				}
				if actor.code == "" {
					return
				}
				payload := map[string]any{}
				if err := json.Unmarshal(body, &payload); err != nil {
					t.Fatalf("decode error body %q: %v", body, err)
				}
				assertError(t, payload, actor.code)
			})
		}
		for _, path := range []string{"/api/v1/assignments/999999" + suffix, "/api/v1/assignments/abc" + suffix} {
			response, body := resultsTestGet(t, env, path, fixture.adminAccess)
			if response.StatusCode != http.StatusNotFound {
				t.Fatalf("GET %s as admin = %d, %s; want 404", path, response.StatusCode, body)
			}
		}
	}
}

func TestAssignmentResultsRecapAndPagination(t *testing.T) {
	env := newTestEnvironment(t)
	fixture := newSubmissionFixture(t, env)
	assignmentPath := fmt.Sprintf("/api/v1/assignments/%d", fixture.assignmentID)

	// The fixture's "Student" is enrolled already. Names are created out of order so that the recap
	// order (name, then id) differs from the id order, and two students share a name.
	citra, _ := createCourseTestUser(t, env, "Citra", "citra@example.test", auth.RoleStudent)
	budi, _ := createCourseTestUser(t, env, "Budi", "budi@example.test", auth.RoleStudent)
	ani, _ := createCourseTestUser(t, env, "Ani", "ani@example.test", auth.RoleStudent)
	ani2, _ := createCourseTestUser(t, env, "Ani", "ani2@example.test", auth.RoleStudent)
	for _, email := range []string{citra.Email, budi.Email, ani2.Email, ani.Email} {
		resultsTestEnroll(t, env, fixture.coursePath, email, fixture.lecturerAccess)
	}

	otherAssignment, otherPayload := env.request(t, http.MethodPost, fixture.coursePath+"/assignments", map[string]any{
		"title": "Other assignment", "description": "",
		"deadline_at": "2026-10-20T12:00:00Z", "late_until_at": "2026-10-21T12:00:00Z",
		"resource_limit": map[string]int{
			"cpu_time_ms": 1000, "memory_mb": 64, "max_processes": 5,
			"timeout_s": 5, "max_output_bytes": 1 << 20, "tmpfs_size_mb": 32,
		},
	}, fixture.lecturerAccess)
	if otherAssignment.StatusCode != http.StatusCreated {
		t.Fatalf("create other assignment = %d, %#v", otherAssignment.StatusCode, otherPayload)
	}
	otherAssignmentID := int64Field(t, otherPayload, "id")

	base := time.Date(2026, 10, 19, 8, 0, 0, 123456000, time.UTC)
	resultsTestInsertSubmission(t, env, budi.ID, fixture.assignmentID, "COMPLETED", base, false, false,
		resultsTestScore(50), resultsTestScore(100))
	budiLatest := resultsTestInsertSubmission(t, env, budi.ID, fixture.assignmentID, "COMPLETED",
		base.Add(time.Hour), true, true, resultsTestScore(80), resultsTestScore(100))
	// A newer cancelled submission and one on another assignment must not change Budi's row.
	resultsTestInsertSubmission(t, env, budi.ID, fixture.assignmentID, "CANCELLED", base.Add(2*time.Hour),
		false, false, nil, nil)
	resultsTestInsertSubmission(t, env, budi.ID, otherAssignmentID, "COMPLETED", base.Add(3*time.Hour),
		false, false, resultsTestScore(1), resultsTestScore(1))
	citraLatest := resultsTestInsertSubmission(t, env, citra.ID, fixture.assignmentID, "QUEUED",
		base.Add(30*time.Minute), false, false, nil, nil)
	// Same submitted_at: the higher id wins.
	resultsTestInsertSubmission(t, env, fixture.student.ID, fixture.assignmentID, "FAILED", base,
		false, false, nil, nil)
	studentLatest := resultsTestInsertSubmission(t, env, fixture.student.ID, fixture.assignmentID,
		"COMPLETED", base, false, false, resultsTestScore(0), resultsTestScore(100))
	// Ani2 only cancelled: listed with no latest submission and a count of 0.
	resultsTestInsertSubmission(t, env, ani2.ID, fixture.assignmentID, "CANCELLED", base, false, false, nil, nil)
	// A user who is not enrolled is not part of the recap, even with a submission.
	resultsTestInsertSubmission(t, env, fixture.otherStudent.ID, fixture.assignmentID, "COMPLETED", base,
		false, false, resultsTestScore(100), resultsTestScore(100))

	response, payload := env.request(t, http.MethodGet, assignmentPath+"/results?page=1&page_size=100", nil, fixture.lecturerAccess)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("GET results = %d, %#v", response.StatusCode, payload)
	}
	if int64Field(t, payload, "assignment_id") != fixture.assignmentID ||
		payload["assignment_title"] != "Submission assignment" ||
		payload["page"] != float64(1) || payload["page_size"] != float64(100) || payload["has_next"] != false {
		t.Fatalf("results envelope = %#v", payload)
	}
	items := pageItems(t, payload)
	wantOrder := []int64{ani.ID, ani2.ID, budi.ID, citra.ID, fixture.student.ID}
	if len(items) != len(wantOrder) {
		t.Fatalf("results items = %d, %#v; want %d", len(items), items, len(wantOrder))
	}
	rows := make(map[int64]map[string]any, len(items))
	for i, raw := range items {
		item, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("results item %d = %#v", i, raw)
		}
		if userID := int64Field(t, item, "user_id"); userID != wantOrder[i] {
			t.Fatalf("results item %d user_id = %d; want %d (order by name, then id)", i, userID, wantOrder[i])
		}
		rows[int64Field(t, item, "user_id")] = item
	}

	for _, userID := range []int64{ani.ID, ani2.ID} {
		row := rows[userID]
		latest, present := row["latest_submission"]
		if !present || latest != nil || row["submission_count"] != float64(0) {
			t.Fatalf("student without graded submission = %#v; want latest_submission null and count 0", row)
		}
	}
	if rows[ani.ID]["name"] != "Ani" || rows[ani.ID]["email"] != ani.Email {
		t.Fatalf("student identity = %#v", rows[ani.ID])
	}

	budiRow := rows[budi.ID]
	if budiRow["submission_count"] != float64(2) {
		t.Fatalf("Budi submission_count = %#v; want 2 (cancelled and other assignment excluded)", budiRow["submission_count"])
	}
	budiSubmission, ok := budiRow["latest_submission"].(map[string]any)
	if !ok || int64Field(t, budiSubmission, "id") != budiLatest || budiSubmission["status"] != "COMPLETED" ||
		budiSubmission["total_score"] != float64(80) || budiSubmission["max_score"] != float64(100) ||
		budiSubmission["is_late"] != true || budiSubmission["stale_grade"] != true {
		t.Fatalf("Budi latest_submission = %#v; want submission %d, 80/100, late and stale", budiRow["latest_submission"], budiLatest)
	}
	submittedAt, err := time.Parse(time.RFC3339Nano, stringField(t, budiSubmission, "submitted_at"))
	if err != nil || !submittedAt.Equal(base.Add(time.Hour)) {
		t.Fatalf("Budi submitted_at = %#v, %v; want %s", budiSubmission["submitted_at"], err, base.Add(time.Hour))
	}

	citraSubmission, ok := rows[citra.ID]["latest_submission"].(map[string]any)
	if !ok || int64Field(t, citraSubmission, "id") != citraLatest || citraSubmission["status"] != "QUEUED" ||
		citraSubmission["is_late"] != false || citraSubmission["stale_grade"] != false {
		t.Fatalf("Citra latest_submission = %#v", rows[citra.ID]["latest_submission"])
	}
	if _, present := citraSubmission["total_score"]; present {
		t.Fatalf("ungraded latest_submission has total_score: %#v", citraSubmission)
	}
	if _, present := citraSubmission["max_score"]; present {
		t.Fatalf("ungraded latest_submission has max_score: %#v", citraSubmission)
	}

	studentRow := rows[fixture.student.ID]
	studentSubmission, ok := studentRow["latest_submission"].(map[string]any)
	if !ok || int64Field(t, studentSubmission, "id") != studentLatest || studentSubmission["total_score"] != float64(0) ||
		studentRow["submission_count"] != float64(2) {
		t.Fatalf("tie on submitted_at = %#v; want submission %d with score 0", studentRow, studentLatest)
	}

	firstPage, firstPayload := env.request(t, http.MethodGet, assignmentPath+"/results?page=1&page_size=2", nil, fixture.adminAccess)
	firstItems := pageItems(t, firstPayload)
	if firstPage.StatusCode != http.StatusOK || len(firstItems) != 2 || firstPayload["has_next"] != true ||
		int64Field(t, firstItems[0].(map[string]any), "user_id") != ani.ID {
		t.Fatalf("results page 1 = %d, %#v", firstPage.StatusCode, firstPayload)
	}
	lastPage, lastPayload := env.request(t, http.MethodGet, assignmentPath+"/results?page=3&page_size=2", nil, fixture.adminAccess)
	lastItems := pageItems(t, lastPayload)
	if lastPage.StatusCode != http.StatusOK || len(lastItems) != 1 || lastPayload["has_next"] != false ||
		int64Field(t, lastItems[0].(map[string]any), "user_id") != fixture.student.ID {
		t.Fatalf("results page 3 = %d, %#v", lastPage.StatusCode, lastPayload)
	}
	emptyPage, emptyPayload := env.request(t, http.MethodGet, assignmentPath+"/results?page=4&page_size=2", nil, fixture.adminAccess)
	if emptyPage.StatusCode != http.StatusOK || len(pageItems(t, emptyPayload)) != 0 || emptyPayload["has_next"] != false {
		t.Fatalf("results past the end = %d, %#v", emptyPage.StatusCode, emptyPayload)
	}
	for _, query := range []string{"page=0", "page_size=0", "page_size=101", "page=x"} {
		response, payload := env.request(t, http.MethodGet, assignmentPath+"/results?"+query, nil, fixture.lecturerAccess)
		if response.StatusCode != http.StatusBadRequest {
			t.Fatalf("results ?%s = %d, %#v; want 400", query, response.StatusCode, payload)
		}
		assertError(t, payload, "invalid_pagination")
	}
}

func TestAssignmentResultsWithoutEnrollments(t *testing.T) {
	env := newTestEnvironment(t)
	_, lecturerAccess := createCourseTestUser(t, env, "Lecturer", "lecturer@example.test", auth.RoleLecturer)
	courseResponse, course := env.request(t, http.MethodPost, "/api/v1/courses", map[string]string{
		"name": "Empty", "code": "EMP101", "term": "2026",
	}, lecturerAccess)
	if courseResponse.StatusCode != http.StatusCreated {
		t.Fatalf("create course = %d, %#v", courseResponse.StatusCode, course)
	}
	assignmentResponse, assignment := env.request(t, http.MethodPost,
		fmt.Sprintf("/api/v1/courses/%d/assignments", int64Field(t, course, "id")), map[string]any{
			"title": "Nobody enrolled", "description": "",
			"deadline_at": "2026-10-20T12:00:00Z", "late_until_at": "2026-10-21T12:00:00Z",
			"resource_limit": map[string]int{
				"cpu_time_ms": 1000, "memory_mb": 64, "max_processes": 5,
				"timeout_s": 5, "max_output_bytes": 1 << 20, "tmpfs_size_mb": 32,
			},
		}, lecturerAccess)
	if assignmentResponse.StatusCode != http.StatusCreated {
		t.Fatalf("create assignment = %d, %#v", assignmentResponse.StatusCode, assignment)
	}
	assignmentPath := fmt.Sprintf("/api/v1/assignments/%d", int64Field(t, assignment, "id"))

	response, payload := env.request(t, http.MethodGet, assignmentPath+"/results", nil, lecturerAccess)
	if response.StatusCode != http.StatusOK || len(pageItems(t, payload)) != 0 ||
		payload["assignment_title"] != "Nobody enrolled" || payload["page_size"] != float64(50) {
		t.Fatalf("empty results = %d, %#v", response.StatusCode, payload)
	}
	exported, body := resultsTestGet(t, env, assignmentPath+"/results/export", lecturerAccess)
	want := "name,email,submission_id,status,total_score,max_score,is_late,stale_grade,submitted_at\r\n"
	if exported.StatusCode != http.StatusOK || string(body) != want {
		t.Fatalf("empty export = %d, %q; want header only", exported.StatusCode, body)
	}
}

func TestAssignmentResultsCSVExport(t *testing.T) {
	env := newTestEnvironment(t)
	fixture := newSubmissionFixture(t, env)
	assignmentPath := fmt.Sprintf("/api/v1/assignments/%d", fixture.assignmentID)

	// Names are user-controlled (registration, profile update). Every one of these would be a
	// formula, or a quoting hazard, if written to the file as is. want is the cell as a CSV reader
	// sees it.
	hostile := []struct {
		name string
		want string
	}{
		{"=HYPERLINK(\"http://evil.example\",\"x\")", "'=HYPERLINK(\"http://evil.example\",\"x\")"},
		{"+SUM(1,2)", "'+SUM(1,2)"},
		{"-2+3", "'-2+3"},
		{"@cmd", "'@cmd"},
		{"\t=1+1", "'\t=1+1"},
		// encoding/csv drops a lone CR when it writes CRLF line endings, so the CR has to be
		// defused before the writer sees it or the cell would start with "=".
		{"\r=1+1", "'=1+1"},
		// encoding/csv reads the quoted CRLF back as LF.
		{"Doe, \"Jane\"\nSecond line", "Doe, \"Jane\"\nSecond line"},
	}
	users := make(map[string]auth.User, len(hostile))
	for i, entry := range hostile {
		user, _ := createCourseTestUser(t, env, entry.name, fmt.Sprintf("hostile%d@example.test", i), auth.RoleStudent)
		resultsTestEnroll(t, env, fixture.coursePath, user.Email, fixture.lecturerAccess)
		users[entry.name] = user
	}
	injectedEmail, _ := createCourseTestUser(t, env, "Zed", "=zed@example.test", auth.RoleStudent)
	resultsTestEnroll(t, env, fixture.coursePath, injectedEmail.Email, fixture.lecturerAccess)

	submittedAt := time.Date(2026, 10, 19, 1, 2, 3, 456789000, time.UTC)
	graded := resultsTestInsertSubmission(t, env, fixture.student.ID, fixture.assignmentID, "COMPLETED",
		submittedAt, true, true, resultsTestScore(40), resultsTestScore(100))
	running := resultsTestInsertSubmission(t, env, users["@cmd"].ID, fixture.assignmentID, "RUNNING",
		submittedAt, false, false, nil, nil)

	response, body := resultsTestGet(t, env, assignmentPath+"/results/export", fixture.lecturerAccess)
	if response.StatusCode != http.StatusOK {
		t.Fatalf("export = %d, %s", response.StatusCode, body)
	}
	if got := response.Header.Get("Content-Type"); got != "text/csv; charset=utf-8" {
		t.Fatalf("export Content-Type = %q", got)
	}
	wantDisposition := fmt.Sprintf(`attachment; filename="rekap-assignment-%d.csv"`, fixture.assignmentID)
	if got := response.Header.Get("Content-Disposition"); got != wantDisposition {
		t.Fatalf("export Content-Disposition = %q; want %q", got, wantDisposition)
	}
	wantHeader := "name,email,submission_id,status,total_score,max_score,is_late,stale_grade,submitted_at\r\n"
	if !bytes.HasPrefix(body, []byte(wantHeader)) {
		t.Fatalf("export does not start with the CRLF header line: %q", body)
	}
	if !bytes.HasSuffix(body, []byte("\r\n")) || bytes.Count(body, []byte("\n")) != bytes.Count(body, []byte("\r\n")) {
		t.Fatalf("export line endings are not CRLF only: %q", body)
	}
	if !bytes.Contains(body, []byte("\"Doe, \"\"Jane\"\"\r\nSecond line\"")) {
		t.Fatalf("export does not quote a cell with a comma, quotes and a line break per RFC 4180: %q", body)
	}

	reader := csv.NewReader(bytes.NewReader(body))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("parse exported CSV: %v\n%q", err, body)
	}
	// Fixture student "Student", seven hostile names and Zed; ordered by name.
	if len(records) != 1+len(hostile)+2 {
		t.Fatalf("export has %d records; want %d: %q", len(records), 1+len(hostile)+2, records)
	}
	byEmail := make(map[string][]string, len(records)-1)
	for _, record := range records[1:] {
		if len(record) != 9 {
			t.Fatalf("record has %d fields; want 9: %q", len(record), record)
		}
		byEmail[record[1]] = record
	}
	for _, entry := range hostile {
		record, ok := byEmail[users[entry.name].Email]
		if !ok {
			t.Fatalf("export has no row for %q", entry.name)
		}
		if record[0] != entry.want {
			t.Fatalf("exported name = %q; want %q", record[0], entry.want)
		}
	}
	if _, ok := byEmail["'=zed@example.test"]; !ok {
		t.Fatalf("email starting with = was not defused: %q", records)
	}

	wantGraded := []string{
		"Student", fixture.student.Email, strconv.FormatInt(graded, 10), "COMPLETED", "40", "100",
		"true", "true", "2026-10-19T01:02:03Z",
	}
	if got := byEmail[fixture.student.Email]; strings.Join(got, "|") != strings.Join(wantGraded, "|") {
		t.Fatalf("graded row = %q; want %q", got, wantGraded)
	}
	wantRunning := []string{
		"'@cmd", users["@cmd"].Email, strconv.FormatInt(running, 10), "RUNNING", "", "",
		"false", "false", "2026-10-19T01:02:03Z",
	}
	if got := byEmail[users["@cmd"].Email]; strings.Join(got, "|") != strings.Join(wantRunning, "|") {
		t.Fatalf("ungraded row = %q; want %q", got, wantRunning)
	}
	wantEmpty := []string{"'+SUM(1,2)", users["+SUM(1,2)"].Email, "", "", "", "", "", "", ""}
	if got := byEmail[users["+SUM(1,2)"].Email]; strings.Join(got, "|") != strings.Join(wantEmpty, "|") {
		t.Fatalf("row without submission = %q; want %q", got, wantEmpty)
	}
}

func TestCSVSafeCell(t *testing.T) {
	for _, test := range []struct {
		cell string
		want string
	}{
		{"", ""},
		{"Ani", "Ani"},
		{"=1+1", "'=1+1"},
		{"+1", "'+1"},
		{"-1", "'-1"},
		{"@SUM(A1)", "'@SUM(A1)"},
		{"\t=1", "'\t=1"},
		{"\r=1", "'\r=1"},
		{" =1", " =1"},
		{"a=1", "a=1"},
		{"2026-10-19T01:02:03Z", "2026-10-19T01:02:03Z"},
		{"Ánh", "Ánh"},
	} {
		if got := csvSafeCell(test.cell); got != test.want {
			t.Errorf("csvSafeCell(%q) = %q; want %q", test.cell, got, test.want)
		}
	}
}
