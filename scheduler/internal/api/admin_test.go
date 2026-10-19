package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/audit"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/auth"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/courses"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/harness"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/health"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/queue"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/settings"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/store"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/submissions"
	"github.com/jackc/pgx/v5/pgxpool"
)

type fixedWorkers struct{ alive, configured int }

func (w fixedWorkers) Workers() (int, int) { return w.alive, w.configured }

// withChecker returns a copy of env whose server is a second handler on the same database, with
// checker as the source of the worker count and queue depth.
func withChecker(t *testing.T, env *testEnvironment, checker *health.Checker) *testEnvironment {
	t.Helper()
	tokens, err := auth.NewTokens([]byte(testJWTSecret))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(NewHandler(
		env.store, tokens, env.tickets, courses.NewStore(env.pool), submissions.NewStore(env.pool),
		settings.NewStore(env.pool), audit.NewStore(env.pool), checker, env.jobs, env.codeDir,
	))
	t.Cleanup(server.Close)
	copied := *env
	copied.server = server
	return &copied
}

// A database failure behind the admin endpoints answers 500 internal_error and shows nothing of the
// error. The handler authenticates through the working pool and reads everything else through a pool
// that is already closed.
func TestAdminEndpointsHideDatabaseFailures(t *testing.T) {
	env := newTestEnvironment(t)
	_, adminAccess := createCourseTestUser(t, env, "Admin", "admin@example.test", auth.RoleAdmin)
	broken, err := pgxpool.NewWithConfig(t.Context(), env.pool.Config())
	if err != nil {
		t.Fatal(err)
	}
	broken.Close()
	tokens, err := auth.NewTokens([]byte(testJWTSecret))
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(NewHandler(
		env.store, tokens, env.tickets, courses.NewStore(env.pool), submissions.NewStore(broken),
		settings.NewStore(broken), audit.NewStore(broken), health.New(env.pool, nil, nil), env.jobs, env.codeDir,
	))
	t.Cleanup(server.Close)
	failing := *env
	failing.server = server

	for _, endpoint := range []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, "/api/v1/admin/audit-logs", nil},
		{http.MethodGet, "/api/v1/admin/queue", nil},
		{http.MethodGet, "/api/v1/admin/rate-limits", nil},
		{http.MethodPut, "/api/v1/admin/rate-limits", rateLimitsBody(1, 30, 30)},
	} {
		response, payload := failing.request(t, endpoint.method, endpoint.path, endpoint.body, adminAccess)
		if response.StatusCode != http.StatusInternalServerError {
			t.Fatalf("%s %s with the database down = %d, %#v; want 500", endpoint.method, endpoint.path,
				response.StatusCode, payload)
		}
		assertError(t, payload, "internal_error")
		if message := errorMessage(t, payload); strings.Contains(message, "closed") || strings.Contains(message, "pool") {
			t.Fatalf("%s %s leaks the database error: %q", endpoint.method, endpoint.path, message)
		}
	}
}

func assertNull(t *testing.T, object map[string]any, key string) {
	t.Helper()
	if value, present := object[key]; !present || value != nil {
		t.Fatalf("field %q = %#v (present: %v); want null", key, value, present)
	}
}

func rateLimitsBody(active, interval, perAssignment any) map[string]any {
	return map[string]any{
		"max_active_submissions":         active,
		"min_submission_interval_s":      interval,
		"max_submissions_per_assignment": perAssignment,
	}
}

func TestAdminEndpointsAreForAdministratorsOnly(t *testing.T) {
	env := newTestEnvironment(t)
	_, studentAccess := createCourseTestUser(t, env, "Student", "student@example.test", auth.RoleStudent)
	_, lecturerAccess := createCourseTestUser(t, env, "Lecturer", "lecturer@example.test", auth.RoleLecturer)
	_, adminAccess := createCourseTestUser(t, env, "Admin", "admin@example.test", auth.RoleAdmin)
	limits := rateLimitsBody(2, 10, 5)

	for _, endpoint := range []struct {
		method, path string
		body         any
	}{
		{http.MethodGet, "/api/v1/admin/audit-logs", nil},
		{http.MethodGet, "/api/v1/admin/queue", nil},
		{http.MethodGet, "/api/v1/admin/rate-limits", nil},
		{http.MethodPut, "/api/v1/admin/rate-limits", limits},
	} {
		for _, caller := range []struct {
			role, token, code string
			status            int
		}{
			{"anonymous", "", "unauthorized", http.StatusUnauthorized},
			{auth.RoleStudent, studentAccess, "forbidden", http.StatusForbidden},
			{auth.RoleLecturer, lecturerAccess, "forbidden", http.StatusForbidden},
		} {
			response, payload := env.request(t, endpoint.method, endpoint.path, endpoint.body, caller.token)
			if response.StatusCode != caller.status {
				t.Fatalf("%s %s as %s = %d, %#v; want %d", endpoint.method, endpoint.path, caller.role,
					response.StatusCode, payload, caller.status)
			}
			assertError(t, payload, caller.code)
		}
		response, payload := env.request(t, endpoint.method, endpoint.path, endpoint.body, adminAccess)
		if response.StatusCode != http.StatusOK {
			t.Fatalf("%s %s as ADMIN = %d, %#v; want 200", endpoint.method, endpoint.path, response.StatusCode, payload)
		}
	}

	// Only the administrator's PUT reached the settings row and the audit log.
	stored, err := settings.NewStore(env.pool).Get(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if stored.MaxActiveSubmissions != 2 || stored.MinSubmissionIntervalS != 10 || stored.MaxSubmissionsPerAssignment != 5 {
		t.Fatalf("stored settings = %+v; want 2, 10, 5", stored)
	}
	var changes int
	if err := env.pool.QueryRow(t.Context(), `SELECT count(*) FROM audit_logs WHERE event_type = $1`,
		settings.EventSettingsChanged).Scan(&changes); err != nil {
		t.Fatal(err)
	}
	if changes != 1 {
		t.Fatalf("%d SETTINGS_CHANGED audit rows; want 1", changes)
	}
}

// The audit log lists what its writers store: the grading pipeline's two rows for one submission,
// written in one transaction and so with one created_at, and a role change that has neither a
// submission nor a syscall.
func TestAdminAuditLogsAreNewestFirstAndPaginated(t *testing.T) {
	env := newTestEnvironment(t)
	fixture := newSubmissionFixture(t, env)
	const path = "/api/v1/admin/audit-logs"

	empty, emptyPayload := env.request(t, http.MethodGet, path, nil, fixture.adminAccess)
	if empty.StatusCode != http.StatusOK || len(pageItems(t, emptyPayload)) != 0 || emptyPayload["has_next"] != false ||
		emptyPayload["page"] != float64(1) || emptyPayload["page_size"] != float64(50) {
		t.Fatalf("empty audit log = %d, %#v", empty.StatusCode, emptyPayload)
	}

	assignmentPath := fmt.Sprintf("/api/v1/assignments/%d", fixture.assignmentID)
	for _, externalID := range []string{"tc_violation", "tc_timeout"} {
		created, createdPayload := env.request(t, http.MethodPost, assignmentPath+"/testcases", map[string]any{
			"external_id": externalID, "input": "", "expected_output": "ok\n", "weight": 1,
			"is_hidden": false, "match_mode": "EXACT",
		}, fixture.lecturerAccess)
		if created.StatusCode != http.StatusCreated {
			t.Fatalf("create test case %s = %d, %#v", externalID, created.StatusCode, createdPayload)
		}
	}
	uploaded, uploadedPayload := env.upload(t, fixture.assignmentID, "solution.py", []byte("print('ok')"), fixture.studentAccess)
	if uploaded.StatusCode != http.StatusAccepted {
		t.Fatalf("upload = %d, %#v", uploaded.StatusCode, uploadedPayload)
	}
	submissionID := int64Field(t, uploadedPayload, "id")

	pipeline := store.NewPipeline(env.pool)
	claim, err := pipeline.Claim(t.Context(), submissionID)
	if err != nil {
		t.Fatalf("claim: %v", err)
	}
	report := &harness.Report{SubmissionID: submissionID}
	for i, testCase := range claim.Definition.TestCases {
		result := harness.Result{
			TestCaseID: testCase.ID, ExternalID: testCase.ExternalID, Status: harness.StatusTimeout,
			MemorySource: harness.MemoryCgroupPeak,
		}
		if i == 0 {
			result.Status = harness.StatusSecurityViolation
			result.Violation = &harness.Violation{SyscallNr: 41}
		}
		report.MaxScore += testCase.Weight
		report.Results = append(report.Results, result)
	}
	if err := pipeline.Complete(t.Context(), claim, report); err != nil {
		t.Fatalf("complete: %v", err)
	}
	promoted, promotedPayload := env.request(t, http.MethodPut,
		fmt.Sprintf("/api/v1/admin/users/%d/role", fixture.otherStudent.ID),
		map[string]string{"role": auth.RoleLecturer}, fixture.adminAccess)
	if promoted.StatusCode != http.StatusOK {
		t.Fatalf("role change = %d, %#v", promoted.StatusCode, promotedPayload)
	}

	pageOne, pageOnePayload := env.request(t, http.MethodGet, path+"?page=1&page_size=2", nil, fixture.adminAccess)
	if pageOne.StatusCode != http.StatusOK || pageOnePayload["page"] != float64(1) ||
		pageOnePayload["page_size"] != float64(2) || pageOnePayload["has_next"] != true {
		t.Fatalf("audit log page one = %d, %#v", pageOne.StatusCode, pageOnePayload)
	}
	pageTwo, pageTwoPayload := env.request(t, http.MethodGet, path+"?page=2&page_size=2", nil, fixture.adminAccess)
	if pageTwo.StatusCode != http.StatusOK || pageTwoPayload["has_next"] != false {
		t.Fatalf("audit log page two = %d, %#v", pageTwo.StatusCode, pageTwoPayload)
	}
	items := append(pageItems(t, pageOnePayload), pageItems(t, pageTwoPayload)...)
	if len(items) != 3 {
		t.Fatalf("audit log items = %#v; want 3", items)
	}
	entries := make([]map[string]any, len(items))
	for i, item := range items {
		entries[i] = item.(map[string]any)
		if len(entries[i]) != 7 {
			t.Fatalf("audit log item %d = %#v; want exactly the 7 FR-12 fields", i, entries[i])
		}
	}

	roleChange, timeout, violation := entries[0], entries[1], entries[2]
	if roleChange["event_type"] != "ROLE_CHANGED" || int64Field(t, roleChange, "user_id") != fixture.admin.ID ||
		!strings.Contains(stringField(t, roleChange, "detail"), `"new_role":"LECTURER"`) {
		t.Fatalf("newest entry = %#v; want the admin's role change", roleChange)
	}
	assertNull(t, roleChange, "submission_id")
	assertNull(t, roleChange, "syscall_nr")
	for _, entry := range []map[string]any{timeout, violation} {
		if int64Field(t, entry, "user_id") != fixture.student.ID || int64Field(t, entry, "submission_id") != submissionID {
			t.Fatalf("pipeline entry = %#v; want student %d, submission %d", entry, fixture.student.ID, submissionID)
		}
	}
	if timeout["event_type"] != store.EventResourceLimitExceeded || !strings.Contains(stringField(t, timeout, "detail"), "tc_timeout") {
		t.Fatalf("second entry = %#v; want the time limit of tc_timeout", timeout)
	}
	assertNull(t, timeout, "syscall_nr")
	if violation["event_type"] != store.EventSandboxViolation || violation["syscall_nr"] != float64(41) ||
		!strings.Contains(stringField(t, violation, "detail"), "tc_violation") {
		t.Fatalf("oldest entry = %#v; want the denied syscall 41 of tc_violation", violation)
	}

	// Newest first by created_at, and by id between rows of one transaction. Times are UTC.
	var previousID int64
	var previousAt time.Time
	for i, entry := range entries {
		id := int64Field(t, entry, "id")
		createdAt, err := time.Parse(time.RFC3339Nano, stringField(t, entry, "created_at"))
		if err != nil || createdAt.Location() != time.UTC {
			t.Fatalf("entry %d created_at = %#v (%v); want RFC 3339 in UTC", i, entry["created_at"], err)
		}
		if i > 0 && (id >= previousID || createdAt.After(previousAt)) {
			t.Fatalf("entry %d (id %d, %s) is not older than entry %d (id %d, %s)", i, id, createdAt, i-1, previousID, previousAt)
		}
		previousID, previousAt = id, createdAt
	}
	if timeout["created_at"] != violation["created_at"] {
		t.Fatalf("pipeline rows have created_at %v and %v; want one transaction's time", timeout["created_at"], violation["created_at"])
	}

	for _, query := range []string{"page=0", "page_size=0", "page_size=101", "page=abc"} {
		invalid, invalidPayload := env.request(t, http.MethodGet, path+"?"+query, nil, fixture.adminAccess)
		if invalid.StatusCode != http.StatusBadRequest {
			t.Fatalf("audit log with %s = %d; want 400", query, invalid.StatusCode)
		}
		assertError(t, invalidPayload, "invalid_pagination")
	}
}

// The worker count and queue depth come from the /healthz checker; the submission counts come from
// the database and leave out finished and cancelled submissions.
func TestAdminQueueReportsHealthNumbersAndActiveSubmissions(t *testing.T) {
	env := newTestEnvironment(t)
	fixture := newSubmissionFixture(t, env)

	// The test environment, like a process started with WORKERS=0, has no worker pool.
	idle, idlePayload := env.request(t, http.MethodGet, "/api/v1/admin/queue", nil, fixture.adminAccess)
	want := map[string]any{
		"queue_depth": float64(0),
		"workers":     map[string]any{"alive": float64(0), "configured": float64(0)},
		"submissions": map[string]any{"queued": float64(0), "running": float64(0)},
	}
	if idle.StatusCode != http.StatusOK || fmt.Sprint(idlePayload) != fmt.Sprint(want) {
		t.Fatalf("idle queue = %d, %#v; want %#v", idle.StatusCode, idlePayload, want)
	}

	for i, status := range []string{"QUEUED", "QUEUED", "RUNNING", "COMPLETED", "FAILED", "CANCELLED"} {
		if _, err := env.pool.Exec(t.Context(), `
			INSERT INTO submissions (user_id, assignment_id, code_path, original_filename, size_bytes, status)
			VALUES ($1, $2, $3, 'main.py', 1, $4)`,
			fixture.student.ID, fixture.assignmentID, fmt.Sprintf("queue-%d.py", i), status); err != nil {
			t.Fatalf("insert %s submission: %v", status, err)
		}
	}
	jobs := queue.NewMemory()
	for i := range 3 {
		if err := jobs.Enqueue(t.Context(), queue.Job{SubmissionID: int64(100 + i), UserID: int64(i%2 + 1)}); err != nil {
			t.Fatal(err)
		}
	}
	checker := health.New(env.pool, fixedWorkers{alive: 3, configured: 4}, jobs)
	busy, busyPayload := withChecker(t, env, checker).request(t, http.MethodGet, "/api/v1/admin/queue", nil, fixture.adminAccess)
	want = map[string]any{
		"queue_depth": float64(3),
		"workers":     map[string]any{"alive": float64(3), "configured": float64(4)},
		"submissions": map[string]any{"queued": float64(2), "running": float64(1)},
	}
	if busy.StatusCode != http.StatusOK || fmt.Sprint(busyPayload) != fmt.Sprint(want) {
		t.Fatalf("busy queue = %d, %#v; want %#v", busy.StatusCode, busyPayload, want)
	}

	recorder := httptest.NewRecorder()
	checker.ServeHTTP(recorder, httptest.NewRequest(http.MethodGet, "/healthz", nil))
	var healthz health.Report
	if err := json.Unmarshal(recorder.Body.Bytes(), &healthz); err != nil {
		t.Fatal(err)
	}
	if healthz.QueueDepth != 3 || healthz.Workers != (health.WorkerCount{Alive: 3, Configured: 4}) {
		t.Fatalf("/healthz = %+v; want the numbers of the admin queue", healthz)
	}
}

func TestAdminRateLimitsValidateAndApplyToTheNextUpload(t *testing.T) {
	env := newTestEnvironment(t)
	fixture := newSubmissionFixture(t, env)
	const path = "/api/v1/admin/rate-limits"
	defaults := rateLimitsBody(float64(1), float64(30), float64(30))

	current, currentPayload := env.request(t, http.MethodGet, path, nil, fixture.adminAccess)
	if current.StatusCode != http.StatusOK || fmt.Sprint(currentPayload) != fmt.Sprint(defaults) {
		t.Fatalf("default rate limits = %d, %#v; want the FR-14 values %#v", current.StatusCode, currentPayload, defaults)
	}
	first, firstPayload := env.upload(t, fixture.assignmentID, "first.py", []byte("print(1)"), fixture.studentAccess)
	if first.StatusCode != http.StatusAccepted {
		t.Fatalf("first upload = %d, %#v", first.StatusCode, firstPayload)
	}
	limited, limitedPayload := env.upload(t, fixture.assignmentID, "second.py", []byte("print(2)"), fixture.studentAccess)
	if limited.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("second upload under the defaults = %d, %#v; want 429", limited.StatusCode, limitedPayload)
	}

	for _, invalid := range []struct {
		name string
		body any
		code string
	}{
		{"negative", rateLimitsBody(-1, 0, 0), "invalid_settings"},
		{"interval above a day", rateLimitsBody(0, settings.MaxInterval+1, 0), "invalid_settings"},
		{"active above the bound", rateLimitsBody(settings.MaxCount+1, 0, 0), "invalid_settings"},
		{"quota above the bound", rateLimitsBody(0, 0, settings.MaxCount+1), "invalid_settings"},
		{"missing field", map[string]any{"max_active_submissions": 0, "min_submission_interval_s": 0}, "invalid_settings"},
		{"null field", rateLimitsBody(0, nil, 0), "invalid_settings"},
		{"fraction", rateLimitsBody(0, 1.5, 0), "invalid_request"},
		{"string", rateLimitsBody(0, "30", 0), "invalid_request"},
		{"unknown field", map[string]any{
			"max_active_submissions": 0, "min_submission_interval_s": 0, "max_submissions_per_assignment": 0,
			"updated_at": "2026-10-19T00:00:00Z",
		}, "invalid_request"},
		{"not an object", []int{0, 0, 0}, "invalid_request"},
	} {
		response, payload := env.request(t, http.MethodPut, path, invalid.body, fixture.adminAccess)
		if response.StatusCode != http.StatusBadRequest {
			t.Fatalf("PUT %s = %d, %#v; want 400", invalid.name, response.StatusCode, payload)
		}
		assertError(t, payload, invalid.code)
		if invalid.code == "invalid_settings" && !strings.Contains(errorMessage(t, payload), "0 sampai 86400") {
			t.Fatalf("PUT %s message = %q; want the bounds", invalid.name, errorMessage(t, payload))
		}
	}
	unchanged, unchangedPayload := env.request(t, http.MethodGet, path, nil, fixture.adminAccess)
	if unchanged.StatusCode != http.StatusOK || fmt.Sprint(unchangedPayload) != fmt.Sprint(defaults) {
		t.Fatalf("rate limits after refused PUTs = %d, %#v; want the defaults", unchanged.StatusCode, unchangedPayload)
	}

	// Switch the active and interval rules off and allow two submissions per assignment.
	next := rateLimitsBody(float64(0), float64(0), float64(2))
	saved, savedPayload := env.request(t, http.MethodPut, path, next, fixture.adminAccess)
	if saved.StatusCode != http.StatusOK || fmt.Sprint(savedPayload) != fmt.Sprint(next) {
		t.Fatalf("PUT rate limits = %d, %#v; want %#v", saved.StatusCode, savedPayload, next)
	}
	read, readPayload := env.request(t, http.MethodGet, path, nil, fixture.adminAccess)
	if read.StatusCode != http.StatusOK || fmt.Sprint(readPayload) != fmt.Sprint(next) {
		t.Fatalf("GET after PUT = %d, %#v; want %#v", read.StatusCode, readPayload, next)
	}
	accepted, acceptedPayload := env.upload(t, fixture.assignmentID, "third.py", []byte("print(3)"), fixture.studentAccess)
	if accepted.StatusCode != http.StatusAccepted {
		t.Fatalf("upload after the PUT = %d, %#v; want 202 with the active and interval rules off", accepted.StatusCode, acceptedPayload)
	}
	quota, quotaPayload := env.upload(t, fixture.assignmentID, "fourth.py", []byte("print(4)"), fixture.studentAccess)
	if quota.StatusCode != http.StatusTooManyRequests || !strings.Contains(errorMessage(t, quotaPayload), "maksimal 2 submission") {
		t.Fatalf("upload over the new quota = %d, %#v; want 429 naming 2", quota.StatusCode, quotaPayload)
	}

	logged, loggedPayload := env.request(t, http.MethodGet, "/api/v1/admin/audit-logs?page_size=1", nil, fixture.adminAccess)
	if logged.StatusCode != http.StatusOK || len(pageItems(t, loggedPayload)) != 1 {
		t.Fatalf("audit log = %d, %#v", logged.StatusCode, loggedPayload)
	}
	entry := pageItems(t, loggedPayload)[0].(map[string]any)
	if entry["event_type"] != settings.EventSettingsChanged || int64Field(t, entry, "user_id") != fixture.admin.ID ||
		!strings.Contains(stringField(t, entry, "detail"), "max_submissions_per_assignment 30 to 2") {
		t.Fatalf("newest audit entry = %#v; want the admin's SETTINGS_CHANGED", entry)
	}
	assertNull(t, entry, "submission_id")
	assertNull(t, entry, "syscall_nr")
}
