package api

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/auth"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/ratelimit"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/submissions"
)

type submissionFixture struct {
	assignmentID      int64
	coursePath        string
	lecturer          auth.User
	lecturerAccess    string
	otherLecturer     auth.User
	otherLecturerAuth string
	student           auth.User
	studentAccess     string
	otherStudent      auth.User
	otherStudentAuth  string
	admin             auth.User
	adminAccess       string
}

func newSubmissionFixture(t *testing.T, env *testEnvironment) submissionFixture {
	t.Helper()
	lecturer, lecturerAccess := createCourseTestUser(t, env, "Lecturer", "lecturer@example.test", auth.RoleLecturer)
	otherLecturer, otherLecturerAuth := createCourseTestUser(t, env, "Other Lecturer", "other-lecturer@example.test", auth.RoleLecturer)
	student, studentAccess := createCourseTestUser(t, env, "Student", "student@example.test", auth.RoleStudent)
	otherStudent, otherStudentAuth := createCourseTestUser(t, env, "Other Student", "other-student@example.test", auth.RoleStudent)
	admin, adminAccess := createCourseTestUser(t, env, "Admin", "admin@example.test", auth.RoleAdmin)

	courseResponse, course := env.request(t, http.MethodPost, "/api/v1/courses", map[string]string{
		"name": "Submission course", "code": "SUB101", "term": "2026",
	}, lecturerAccess)
	if courseResponse.StatusCode != http.StatusCreated {
		t.Fatalf("create submission course = %d, %#v", courseResponse.StatusCode, course)
	}
	coursePath := fmt.Sprintf("/api/v1/courses/%d", int64Field(t, course, "id"))
	enrolled, enrolledPayload := env.request(t, http.MethodPost, coursePath+"/enrollments", map[string]string{
		"email": student.Email,
	}, lecturerAccess)
	if enrolled.StatusCode != http.StatusNoContent {
		t.Fatalf("enroll student = %d, %#v", enrolled.StatusCode, enrolledPayload)
	}

	now := time.Now().UTC()
	deadline := now.Add(2 * time.Hour)
	lateUntil := now.Add(4 * time.Hour)
	assignmentResponse, assignment := env.request(t, http.MethodPost, coursePath+"/assignments", map[string]any{
		"title": "Submission assignment", "description": "Submit Python.",
		"deadline_at": deadline.Format(time.RFC3339Nano), "late_until_at": lateUntil.Format(time.RFC3339Nano),
		"resource_limit": map[string]int{
			"cpu_time_ms": 1000, "memory_mb": 64, "max_processes": 5,
			"timeout_s": 5, "max_output_bytes": 1 << 20, "tmpfs_size_mb": 32,
		},
	}, lecturerAccess)
	if assignmentResponse.StatusCode != http.StatusCreated {
		t.Fatalf("create submission assignment = %d, %#v", assignmentResponse.StatusCode, assignment)
	}
	return submissionFixture{
		assignmentID: int64Field(t, assignment, "id"), coursePath: coursePath,
		lecturer: lecturer, lecturerAccess: lecturerAccess,
		otherLecturer: otherLecturer, otherLecturerAuth: otherLecturerAuth,
		student: student, studentAccess: studentAccess,
		otherStudent: otherStudent, otherStudentAuth: otherStudentAuth,
		admin: admin, adminAccess: adminAccess,
	}
}

func disableSubmissionRateLimits(t *testing.T, env *testEnvironment) {
	t.Helper()
	if _, err := env.pool.Exec(t.Context(), `
		UPDATE settings SET max_active_submissions = 0,
			min_submission_interval_s = 0, max_submissions_per_assignment = 0
		WHERE id = 1
	`); err != nil {
		t.Fatalf("disable submission rate limits: %v", err)
	}
}

func TestSubmissionUploadLimitsQueueAndAccess(t *testing.T) {
	env := newTestEnvironment(t)
	fixture := newSubmissionFixture(t, env)
	tooLarge, tooLargePayload := env.upload(t, fixture.assignmentID, "solution.py", []byte(strings.Repeat("x", maxSubmissionBytes+1)), fixture.studentAccess)
	if tooLarge.StatusCode != http.StatusRequestEntityTooLarge {
		t.Fatalf("upload 256 KiB + 1 byte = %d, %#v; want 413", tooLarge.StatusCode, tooLargePayload)
	}
	assertError(t, tooLargePayload, "file_too_large")
	invalidExtension, invalidExtensionPayload := env.upload(t, fixture.assignmentID, "solution.txt", []byte("print(1)"), fixture.studentAccess)
	if invalidExtension.StatusCode != http.StatusBadRequest {
		t.Fatalf("upload non-Python file = %d, %#v; want 400", invalidExtension.StatusCode, invalidExtensionPayload)
	}
	assertError(t, invalidExtensionPayload, "invalid_file")
	for _, filename := range []string{"bad\x00name.py", string([]byte{0xff, '.', 'p', 'y'})} {
		invalidName, invalidNamePayload := env.upload(t, fixture.assignmentID, filename, []byte("print(1)"), fixture.studentAccess)
		if invalidName.StatusCode != http.StatusBadRequest {
			t.Fatalf("upload invalid filename %q = %d, %#v; want 400", filename, invalidName.StatusCode, invalidNamePayload)
		}
		if nested, _ := invalidNamePayload["error"].(map[string]any); nested["code"] != "invalid_file" {
			t.Fatalf("upload filename %q error payload = %#v; want invalid_file", filename, invalidNamePayload)
		}
	}

	unregistered, unregisteredPayload := env.upload(t, fixture.assignmentID, "solution.py", []byte("print(1)"), fixture.otherStudentAuth)
	if unregistered.StatusCode != http.StatusNotFound {
		t.Fatalf("unenrolled student upload = %d, %#v; want 404", unregistered.StatusCode, unregisteredPayload)
	}
	assertError(t, unregisteredPayload, "not_found")
	lecturerUpload, lecturerUploadPayload := env.upload(t, fixture.assignmentID, "solution.py", []byte("print(1)"), fixture.lecturerAccess)
	if lecturerUpload.StatusCode != http.StatusNotFound {
		t.Fatalf("lecturer upload = %d, %#v; want 404", lecturerUpload.StatusCode, lecturerUploadPayload)
	}
	assertError(t, lecturerUploadPayload, "not_found")

	data := []byte(strings.Repeat("p", maxSubmissionBytes))
	created, payload := env.upload(t, fixture.assignmentID, "solution.PY", data, fixture.studentAccess)
	if created.StatusCode != http.StatusAccepted {
		t.Fatalf("upload exactly 256 KiB = %d, %#v; want 202", created.StatusCode, payload)
	}
	submissionID := int64Field(t, payload, "id")
	if payload["status"] != "QUEUED" || payload["is_late"] != false ||
		int64Field(t, payload, "size_bytes") != maxSubmissionBytes {
		t.Fatalf("created submission = %#v", payload)
	}
	if submittedAt, ok := payload["submitted_at"].(string); !ok || submittedAt == "" {
		t.Fatalf("accepted submission has no submitted_at: %#v", payload)
	}
	filePath := env.submissionFile(t, payload)
	saved, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatalf("read stored submission: %v", err)
	}
	if len(saved) != maxSubmissionBytes || string(saved) != string(data) {
		t.Fatalf("stored file has %d bytes and does not match upload", len(saved))
	}
	jobs := env.jobs.enqueued()
	if len(jobs) != 1 || jobs[0].SubmissionID != submissionID || jobs[0].UserID != fixture.student.ID {
		t.Fatalf("queued jobs = %#v; want submission %d for user %d", jobs, submissionID, fixture.student.ID)
	}

	ownDetail, ownDetailPayload := env.request(t, http.MethodGet, fmt.Sprintf("/api/v1/submissions/%d", submissionID), nil, fixture.studentAccess)
	if ownDetail.StatusCode != http.StatusOK || int64Field(t, ownDetailPayload, "id") != submissionID {
		t.Fatalf("owner detail = %d, %#v", ownDetail.StatusCode, ownDetailPayload)
	}
	ownerDetail, ownerPayload := env.request(t, http.MethodGet, fmt.Sprintf("/api/v1/submissions/%d", submissionID), nil, fixture.lecturerAccess)
	if ownerDetail.StatusCode != http.StatusOK || int64Field(t, ownerPayload, "id") != submissionID {
		t.Fatalf("course owner detail = %d, %#v", ownerDetail.StatusCode, ownerPayload)
	}
	adminDetail, adminPayload := env.request(t, http.MethodGet, fmt.Sprintf("/api/v1/submissions/%d", submissionID), nil, fixture.adminAccess)
	if adminDetail.StatusCode != http.StatusOK || int64Field(t, adminPayload, "id") != submissionID {
		t.Fatalf("admin detail = %d, %#v", adminDetail.StatusCode, adminPayload)
	}
	for name, token := range map[string]string{
		"another student": fixture.otherStudentAuth,
		"other lecturer":  fixture.otherLecturerAuth,
	} {
		response, errorPayload := env.request(t, http.MethodGet, fmt.Sprintf("/api/v1/submissions/%d", submissionID), nil, token)
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("%s detail = %d, %#v; want 404", name, response.StatusCode, errorPayload)
		}
		assertError(t, errorPayload, "not_found")
	}

	demoted, demotedPayload := env.request(t, http.MethodPut,
		fmt.Sprintf("/api/v1/admin/users/%d/role", fixture.lecturer.ID),
		map[string]string{"role": auth.RoleStudent}, fixture.adminAccess)
	if demoted.StatusCode != http.StatusOK || demotedPayload["role"] != auth.RoleStudent {
		t.Fatalf("demote course owner = %d, %#v", demoted.StatusCode, demotedPayload)
	}
	demotedDetail, demotedPayload := env.request(t, http.MethodGet,
		fmt.Sprintf("/api/v1/submissions/%d", submissionID), nil, fixture.lecturerAccess)
	if demotedDetail.StatusCode != http.StatusNotFound {
		t.Fatalf("demoted course owner detail = %d, %#v; want 404", demotedDetail.StatusCode, demotedPayload)
	}
	assertError(t, demotedPayload, "not_found")
	demotedReport, demotedReportPayload := env.request(t, http.MethodGet,
		fmt.Sprintf("/api/v1/submissions/%d/report", submissionID), nil, fixture.lecturerAccess)
	if demotedReport.StatusCode != http.StatusNotFound {
		t.Fatalf("demoted course owner report = %d, %#v; want 404", demotedReport.StatusCode, demotedReportPayload)
	}
	assertError(t, demotedReportPayload, "not_found")
}

func TestSubmissionRateLimitAndHistoryPagination(t *testing.T) {
	env := newTestEnvironment(t)
	fixture := newSubmissionFixture(t, env)
	first, firstPayload := env.upload(t, fixture.assignmentID, "one.py", []byte("print(1)"), fixture.studentAccess)
	if first.StatusCode != http.StatusAccepted {
		t.Fatalf("first upload = %d, %#v", first.StatusCode, firstPayload)
	}
	limited, limitedPayload := env.upload(t, fixture.assignmentID, "two.py", []byte("print(2)"), fixture.studentAccess)
	if limited.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("second upload under default FR-14 limits = %d, %#v; want 429", limited.StatusCode, limitedPayload)
	}
	assertError(t, limitedPayload, "rate_limit_exceeded")
	if message, _ := limitedPayload["error"].(map[string]any)["message"].(string); !strings.Contains(message, "antre atau dinilai") || strings.Contains(message, "ratelimit:") {
		t.Fatalf("active-submission message = %q", message)
	}

	if _, err := env.pool.Exec(t.Context(), `UPDATE submissions SET status = 'COMPLETED' WHERE id = $1`, int64Field(t, firstPayload, "id")); err != nil {
		t.Fatalf("complete first submission for interval test: %v", err)
	}
	if _, err := env.pool.Exec(t.Context(), `
		UPDATE settings SET max_active_submissions = 0, min_submission_interval_s = 30,
			max_submissions_per_assignment = 0 WHERE id = 1
	`); err != nil {
		t.Fatalf("set interval rate limit: %v", err)
	}
	intervalLimited, intervalPayload := env.upload(t, fixture.assignmentID, "interval.py", []byte("print(1)"), fixture.studentAccess)
	if intervalLimited.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("upload under interval limit = %d, %#v; want 429", intervalLimited.StatusCode, intervalPayload)
	}
	if message, _ := intervalPayload["error"].(map[string]any)["message"].(string); !strings.Contains(message, "Tunggu") || strings.Contains(message, "ratelimit:") {
		t.Fatalf("interval message = %q", message)
	}
	if _, err := env.pool.Exec(t.Context(), `
		UPDATE settings SET max_active_submissions = 0, min_submission_interval_s = 0,
			max_submissions_per_assignment = 1 WHERE id = 1
	`); err != nil {
		t.Fatalf("set submission quota: %v", err)
	}
	quotaLimited, quotaPayload := env.upload(t, fixture.assignmentID, "quota.py", []byte("print(1)"), fixture.studentAccess)
	if quotaLimited.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("upload under assignment quota = %d, %#v; want 429", quotaLimited.StatusCode, quotaPayload)
	}
	if message, _ := quotaPayload["error"].(map[string]any)["message"].(string); !strings.Contains(message, "maksimal 1 submission") || strings.Contains(message, "ratelimit:") {
		t.Fatalf("quota message = %q", message)
	}

	disableSubmissionRateLimits(t, env)
	for i := 0; i < 2; i++ {
		response, payload := env.upload(t, fixture.assignmentID, fmt.Sprintf("more-%d.py", i), []byte("print(3)"), fixture.studentAccess)
		if response.StatusCode != http.StatusAccepted {
			t.Fatalf("additional upload %d = %d, %#v", i, response.StatusCode, payload)
		}
	}
	pageOne, pageOnePayload := env.request(t, http.MethodGet, "/api/v1/submissions?page=1&page_size=2", nil, fixture.studentAccess)
	if pageOne.StatusCode != http.StatusOK || len(pageItems(t, pageOnePayload)) != 2 || pageOnePayload["has_next"] != true {
		t.Fatalf("submission history page one = %d, %#v", pageOne.StatusCode, pageOnePayload)
	}
	pageTwo, pageTwoPayload := env.request(t, http.MethodGet, "/api/v1/submissions?page=2&page_size=2", nil, fixture.studentAccess)
	if pageTwo.StatusCode != http.StatusOK || len(pageItems(t, pageTwoPayload)) != 1 || pageTwoPayload["has_next"] != false {
		t.Fatalf("submission history page two = %d, %#v", pageTwo.StatusCode, pageTwoPayload)
	}
	secondAssignmentResponse, secondAssignment := env.request(t, http.MethodPost, fixture.coursePath+"/assignments", map[string]any{
		"title": "Second submission assignment", "description": "Submit again.",
		"deadline_at":   time.Now().UTC().Add(2 * time.Hour).Format(time.RFC3339Nano),
		"late_until_at": time.Now().UTC().Add(4 * time.Hour).Format(time.RFC3339Nano),
		"resource_limit": map[string]int{
			"cpu_time_ms": 1000, "memory_mb": 64, "max_processes": 5,
			"timeout_s": 5, "max_output_bytes": 1 << 20, "tmpfs_size_mb": 32,
		},
	}, fixture.lecturerAccess)
	if secondAssignmentResponse.StatusCode != http.StatusCreated {
		t.Fatalf("create second submission assignment = %d, %#v", secondAssignmentResponse.StatusCode, secondAssignment)
	}
	secondAssignmentID := int64Field(t, secondAssignment, "id")
	secondAssignmentSubmission, secondAssignmentSubmissionPayload := env.upload(
		t, secondAssignmentID, "second-assignment.py", []byte("print(4)"), fixture.studentAccess,
	)
	if secondAssignmentSubmission.StatusCode != http.StatusAccepted {
		t.Fatalf("upload second assignment = %d, %#v", secondAssignmentSubmission.StatusCode, secondAssignmentSubmissionPayload)
	}
	filtered, filteredPayload := env.request(t, http.MethodGet,
		fmt.Sprintf("/api/v1/submissions?assignment_id=%d", fixture.assignmentID), nil, fixture.studentAccess)
	if filtered.StatusCode != http.StatusOK || len(pageItems(t, filteredPayload)) != 3 {
		t.Fatalf("history filtered by first assignment = %d, %#v", filtered.StatusCode, filteredPayload)
	}
	filteredSecond, filteredSecondPayload := env.request(t, http.MethodGet,
		fmt.Sprintf("/api/v1/submissions?assignment_id=%d", secondAssignmentID), nil, fixture.studentAccess)
	if filteredSecond.StatusCode != http.StatusOK || len(pageItems(t, filteredSecondPayload)) != 1 ||
		int64Field(t, pageItems(t, filteredSecondPayload)[0].(map[string]any), "assignment_id") != secondAssignmentID {
		t.Fatalf("history filtered by second assignment = %d, %#v", filteredSecond.StatusCode, filteredSecondPayload)
	}
	for _, query := range []string{"assignment_id=0", "assignment_id=not-a-number", fmt.Sprintf("assignment_id=%d&assignment_id=%d", fixture.assignmentID, secondAssignmentID)} {
		invalidFilter, invalidFilterPayload := env.request(t, http.MethodGet, "/api/v1/submissions?"+query, nil, fixture.studentAccess)
		if invalidFilter.StatusCode != http.StatusBadRequest {
			t.Fatalf("invalid assignment filter %q = %d, %#v; want 400", query, invalidFilter.StatusCode, invalidFilterPayload)
		}
		assertError(t, invalidFilterPayload, "invalid_assignment_id")
	}
	otherHistory, otherHistoryPayload := env.request(t, http.MethodGet, "/api/v1/submissions", nil, fixture.otherStudentAuth)
	if otherHistory.StatusCode != http.StatusOK || len(pageItems(t, otherHistoryPayload)) != 0 {
		t.Fatalf("other student's history = %d, %#v", otherHistory.StatusCode, otherHistoryPayload)
	}
}

func TestSubmissionDeadlineBoundaries(t *testing.T) {
	env := newTestEnvironment(t)
	fixture := newSubmissionFixture(t, env)
	disableSubmissionRateLimits(t, env)
	deadline := time.Date(2026, 10, 19, 12, 0, 0, 0, time.UTC)
	lateUntil := deadline.Add(time.Hour)
	store := submissions.NewStore(env.pool)
	for _, tc := range []struct {
		name     string
		at       time.Time
		wantLate bool
		wantErr  error
	}{
		{name: "before deadline", at: deadline.Add(-time.Nanosecond), wantLate: false},
		{name: "exactly at deadline", at: deadline, wantLate: false},
		{name: "after deadline", at: deadline.Add(time.Nanosecond), wantLate: true},
		{name: "exactly at late cutoff", at: lateUntil, wantLate: true},
		{name: "after late cutoff", at: lateUntil.Add(time.Nanosecond), wantErr: submissions.ErrDeadlinePassed},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := env.pool.Exec(t.Context(), `
				UPDATE assignments SET deadline_at = $2, late_until_at = $3 WHERE id = $1
			`, fixture.assignmentID, deadline, lateUntil)
			if err != nil {
				t.Fatalf("set assignment deadlines: %v", err)
			}
			created, err := store.CreateAt(
				t.Context(), fixture.student.ID, fixture.assignmentID, "boundary.py",
				fmt.Sprintf("%s-%d.py", strings.ReplaceAll(tc.name, " ", "-"), tc.at.UnixNano()),
				1, tc.at, ratelimit.Limits{},
			)
			if tc.wantErr != nil {
				if !errors.Is(err, tc.wantErr) {
					t.Fatalf("create at boundary error = %v; want %v", err, tc.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("create at boundary: %v", err)
			}
			if created.IsLate != tc.wantLate {
				t.Fatalf("is_late = %t; want %t", created.IsLate, tc.wantLate)
			}
		})
	}
}

func TestSubmissionAfterLateCutoffReturnsConflict(t *testing.T) {
	env := newTestEnvironment(t)
	fixture := newSubmissionFixture(t, env)
	if _, err := env.pool.Exec(t.Context(), `
		UPDATE assignments SET deadline_at = now() - interval '2 hours',
			late_until_at = now() - interval '1 hour' WHERE id = $1
	`, fixture.assignmentID); err != nil {
		t.Fatalf("expire assignment deadline: %v", err)
	}
	response, payload := env.upload(t, fixture.assignmentID, "late.py", []byte("print(1)"), fixture.studentAccess)
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("upload after late cutoff = %d, %#v; want 409", response.StatusCode, payload)
	}
	assertError(t, payload, "deadline_passed")
	if len(env.jobs.enqueued()) != 0 {
		t.Fatalf("late upload was enqueued: %#v", env.jobs.enqueued())
	}
}

func TestSubmissionReportRedactsHiddenCasesAndSurvivesConcurrentDelete(t *testing.T) {
	env := newTestEnvironment(t)
	fixture := newSubmissionFixture(t, env)
	assignmentPath := fmt.Sprintf("/api/v1/assignments/%d", fixture.assignmentID)
	statuses := []string{"PASSED", "FAILED", "TIMEOUT", "MEMORY_LIMIT_EXCEEDED", "RUNTIME_ERROR", "SYNTAX_ERROR", "SECURITY_VIOLATION"}
	testCaseIDs := make([]int64, len(statuses))
	for i := range statuses {
		hidden := i == len(statuses)-1
		externalID := fmt.Sprintf("visible_%d", i)
		input, expected := fmt.Sprintf("input-%d", i), fmt.Sprintf("expected-%d", i)
		if hidden {
			externalID, input, expected = "hidden_case", "secret input", "secret expected"
		}
		createdCase, casePayload := env.request(t, http.MethodPost, assignmentPath+"/testcases", map[string]any{
			"external_id": externalID, "input": input, "expected_output": expected,
			"weight": 1, "is_hidden": hidden, "match_mode": "EXACT",
		}, fixture.lecturerAccess)
		if createdCase.StatusCode != http.StatusCreated {
			t.Fatalf("create report test case %d = %d, %#v", i, createdCase.StatusCode, casePayload)
		}
		testCaseIDs[i] = int64Field(t, casePayload, "id")
	}

	created, submissionPayload := env.upload(t, fixture.assignmentID, "report.py", []byte("print(1)"), fixture.studentAccess)
	if created.StatusCode != http.StatusAccepted {
		t.Fatalf("create report submission = %d, %#v", created.StatusCode, submissionPayload)
	}
	submissionID := int64Field(t, submissionPayload, "id")
	if _, err := env.pool.Exec(t.Context(), `
		UPDATE submissions SET status = 'COMPLETED', total_score = 6, max_score = 7 WHERE id = $1
	`, submissionID); err != nil {
		t.Fatalf("complete report submission: %v", err)
	}
	for i, status := range statuses {
		_, err := env.pool.Exec(t.Context(), `
			INSERT INTO results (submission_id, testcase_id, status, actual_output, error_message,
				exec_time_ms, memory_kb, memory_source, score)
			VALUES ($1, $2, $3, 'secret actual', 'secret error', 10, 20, 'CGROUP_PEAK', $4)
		`, submissionID, testCaseIDs[i], status, i)
		if err != nil {
			t.Fatalf("insert report result %d: %v", i, err)
		}
	}
	studentReportResponse, studentReport := env.request(t, http.MethodGet,
		fmt.Sprintf("/api/v1/submissions/%d/report", submissionID), nil, fixture.studentAccess)
	if studentReportResponse.StatusCode != http.StatusOK {
		t.Fatalf("student report = %d, %#v", studentReportResponse.StatusCode, studentReport)
	}
	studentResults := pageItems(t, map[string]any{"items": studentReport["results"]})
	if len(studentResults) != len(statuses) {
		t.Fatalf("student report has %d results; want %d", len(studentResults), len(statuses))
	}
	for name, token := range map[string]string{
		"another student": fixture.otherStudentAuth,
		"other lecturer":  fixture.otherLecturerAuth,
	} {
		response, payload := env.request(t, http.MethodGet,
			fmt.Sprintf("/api/v1/submissions/%d/report", submissionID), nil, token)
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("%s report = %d, %#v; want 404", name, response.StatusCode, payload)
		}
		assertError(t, payload, "not_found")
	}
	adminReportResponse, adminReport := env.request(t, http.MethodGet,
		fmt.Sprintf("/api/v1/submissions/%d/report", submissionID), nil, fixture.adminAccess)
	if adminReportResponse.StatusCode != http.StatusOK || len(pageItems(t, map[string]any{"items": adminReport["results"]})) != len(statuses) {
		t.Fatalf("admin report = %d, %#v", adminReportResponse.StatusCode, adminReport)
	}
	for i, raw := range studentResults {
		entry, ok := raw.(map[string]any)
		if !ok || entry["status"] != statuses[i] {
			t.Fatalf("student report entry %d = %#v", i, raw)
		}
		if i == len(statuses)-1 {
			wantKeys := map[string]bool{"status": true, "score": true, "exec_time_ms": true, "memory_kb": true}
			for _, key := range []string{"actual_output", "error_message", "input", "expected_output"} {
				if _, exists := entry[key]; exists {
					t.Fatalf("student report entry %d leaks %q: %#v", i, key, entry)
				}
			}
			for key := range entry {
				if !wantKeys[key] {
					t.Fatalf("hidden report entry includes unexpected field %q: %#v", key, entry)
				}
			}
			if len(entry) != len(wantKeys) {
				t.Fatalf("hidden report entry has fields %#v; want only status, score, time, and memory", entry)
			}
		} else {
			for _, key := range []string{"actual_output", "error_message", "input", "expected_output"} {
				if _, exists := entry[key]; !exists {
					t.Fatalf("visible student report entry %d is missing %q: %#v", i, key, entry)
				}
			}
		}
	}

	promoted, promotedPayload := env.request(t, http.MethodPut,
		fmt.Sprintf("/api/v1/admin/users/%d/role", fixture.student.ID),
		map[string]string{"role": auth.RoleLecturer}, fixture.adminAccess)
	if promoted.StatusCode != http.StatusOK || promotedPayload["role"] != auth.RoleLecturer {
		t.Fatalf("promote submission owner to lecturer = %d, %#v", promoted.StatusCode, promotedPayload)
	}
	otherCourseResponse, otherCourse := env.request(t, http.MethodPost, "/api/v1/courses", map[string]string{
		"name": "Other teaching course", "code": "SUB202", "term": "2026",
	}, fixture.studentAccess)
	if otherCourseResponse.StatusCode != http.StatusCreated {
		t.Fatalf("create other course for promoted submission owner = %d, %#v", otherCourseResponse.StatusCode, otherCourse)
	}
	promotedOwnerReportResponse, promotedOwnerReport := env.request(t, http.MethodGet,
		fmt.Sprintf("/api/v1/submissions/%d/report", submissionID), nil, fixture.studentAccess)
	if promotedOwnerReportResponse.StatusCode != http.StatusOK {
		t.Fatalf("promoted owner's report = %d, %#v", promotedOwnerReportResponse.StatusCode, promotedOwnerReport)
	}
	promotedHiddenResults := pageItems(t, map[string]any{"items": promotedOwnerReport["results"]})
	promotedHiddenEntry := promotedHiddenResults[len(promotedHiddenResults)-1].(map[string]any)
	for _, key := range []string{"actual_output", "error_message", "input", "expected_output"} {
		if _, exists := promotedHiddenEntry[key]; exists {
			t.Fatalf("promoted owner can see hidden report field %q: %#v", key, promotedHiddenEntry)
		}
	}

	lecturerReportResponse, lecturerReport := env.request(t, http.MethodGet,
		fmt.Sprintf("/api/v1/submissions/%d/report", submissionID), nil, fixture.lecturerAccess)
	if lecturerReportResponse.StatusCode != http.StatusOK {
		t.Fatalf("lecturer report = %d, %#v", lecturerReportResponse.StatusCode, lecturerReport)
	}
	lecturerResults := pageItems(t, map[string]any{"items": lecturerReport["results"]})
	hiddenLecturerResult := lecturerResults[len(lecturerResults)-1].(map[string]any)
	for _, key := range []string{"actual_output", "error_message", "input", "expected_output"} {
		if _, exists := hiddenLecturerResult[key]; !exists {
			t.Fatalf("lecturer hidden report is missing %q: %#v", key, hiddenLecturerResult)
		}
	}

	raceCaseResponse, raceCasePayload := env.request(t, http.MethodPost, assignmentPath+"/testcases", map[string]any{
		"external_id": "race_case", "input": "race input", "expected_output": "race output",
		"weight": 1, "is_hidden": false, "match_mode": "EXACT",
	}, fixture.lecturerAccess)
	if raceCaseResponse.StatusCode != http.StatusCreated {
		t.Fatalf("create concurrent-delete test case = %d, %#v", raceCaseResponse.StatusCode, raceCasePayload)
	}
	raceCaseID := int64Field(t, raceCasePayload, "id")
	if _, err := env.pool.Exec(t.Context(), `UPDATE submissions SET status = 'RUNNING' WHERE id = $1`, submissionID); err != nil {
		t.Fatalf("mark report submission running: %v", err)
	}
	deleted, deletePayload := env.request(t, http.MethodDelete,
		fmt.Sprintf("/api/v1/testcases/%d", raceCaseID), nil, fixture.lecturerAccess)
	if deleted.StatusCode != http.StatusNoContent {
		t.Fatalf("delete test case while running = %d, %#v", deleted.StatusCode, deletePayload)
	}
	if _, err := env.pool.Exec(t.Context(), `
		INSERT INTO results (submission_id, testcase_id, status, actual_output, exec_time_ms, memory_kb, memory_source, score)
		VALUES ($1, $2, 'PASSED', 'late result', 1, 1, 'CGROUP_PEAK', 1)
	`, submissionID, raceCaseID); err != nil {
		t.Fatalf("insert result after test case delete while RUNNING: %v", err)
	}
	runningReportResponse, runningReport := env.request(t, http.MethodGet,
		fmt.Sprintf("/api/v1/submissions/%d/report", submissionID), nil, fixture.studentAccess)
	if runningReportResponse.StatusCode != http.StatusOK || len(pageItems(t, map[string]any{"items": runningReport["results"]})) != len(statuses)+1 {
		t.Fatalf("report after running deletion = %d, %#v", runningReportResponse.StatusCode, runningReport)
	}
}
