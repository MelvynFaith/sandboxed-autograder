package api

import (
	"context"
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/harness"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/queue"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/store"
)

// regradeFixture is a submission fixture whose rate limits are off, so a test may upload as many
// submissions as it needs.
func newRegradeFixture(t *testing.T, env *testEnvironment) submissionFixture {
	t.Helper()
	fixture := newSubmissionFixture(t, env)
	disableSubmissionRateLimits(t, env)
	return fixture
}

func (f submissionFixture) assignmentPath() string {
	return fmt.Sprintf("/api/v1/assignments/%d", f.assignmentID)
}

// addTestCase creates a visible test case with the given weight and returns its id.
func addTestCase(t *testing.T, env *testEnvironment, f submissionFixture, externalID string, weight int) int64 {
	t.Helper()
	response, payload := env.request(t, http.MethodPost, f.assignmentPath()+"/testcases", map[string]any{
		"external_id": externalID, "input": "in", "expected_output": "out",
		"weight": weight, "is_hidden": false, "match_mode": "EXACT",
	}, f.lecturerAccess)
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("create test case %s = %d, %#v", externalID, response.StatusCode, payload)
	}
	return int64Field(t, payload, "id")
}

// upload submits a file as the student and returns the submission id; it stays QUEUED.
func uploadAsStudent(t *testing.T, env *testEnvironment, f submissionFixture, assignmentID int64) int64 {
	t.Helper()
	response, payload := env.upload(t, assignmentID, "solution.py", []byte("print(1)"), f.studentAccess)
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("upload = %d, %#v", response.StatusCode, payload)
	}
	return int64Field(t, payload, "id")
}

// workerTookJob drops the job an upload queued, as a worker does when it dequeues it, so that a later
// regrade can queue the submission again (the queue refuses a duplicate).
func workerTookJob(t *testing.T, env *testEnvironment, submissionID int64) {
	t.Helper()
	if removed, err := env.jobs.Remove(t.Context(), submissionID); err != nil || !removed {
		t.Fatalf("take job of submission %d = %t, %v", submissionID, removed, err)
	}
}

// gradeLikeAWorker runs a QUEUED submission through the same Pipeline calls a worker makes: Claim,
// then Complete with a report in which every test case passes and scores its weight.
func gradeLikeAWorker(t *testing.T, env *testEnvironment, submissionID int64) {
	t.Helper()
	pipeline := store.NewPipeline(env.pool)
	claim, err := pipeline.Claim(t.Context(), submissionID)
	if err != nil {
		t.Fatalf("claim submission %d: %v", submissionID, err)
	}
	report := &harness.Report{SubmissionID: submissionID}
	for _, tc := range claim.Definition.TestCases {
		report.Results = append(report.Results, harness.Result{
			TestCaseID: tc.ID, ExternalID: tc.ExternalID, Status: harness.StatusPassed, Score: tc.Weight,
			ActualOutput: "out", ExecTimeMS: 5, MemoryKB: 1024, MemorySource: harness.MemoryCgroupPeak,
		})
		report.TotalScore += tc.Weight
		report.MaxScore += tc.Weight
	}
	if err := pipeline.Complete(t.Context(), claim, report); err != nil {
		t.Fatalf("complete submission %d: %v", submissionID, err)
	}
}

// countField reads a JSON number that may be zero.
func countField(t *testing.T, object map[string]any, key string) int64 {
	t.Helper()
	number, ok := object[key].(float64)
	if !ok {
		t.Fatalf("field %q = %#v; want a number", key, object[key])
	}
	return int64(number)
}

// setStatus puts a submission directly in a state, with a grade when the state has one.
func setStatus(t *testing.T, env *testEnvironment, submissionID int64, status string) {
	t.Helper()
	query := `UPDATE submissions SET status = $2 WHERE id = $1`
	if status == "COMPLETED" {
		query = `UPDATE submissions SET status = $2, total_score = 1, max_score = 1, graded_at = now() WHERE id = $1`
	}
	if _, err := env.pool.Exec(t.Context(), query, submissionID, status); err != nil {
		t.Fatalf("set submission %d to %s: %v", submissionID, status, err)
	}
}

// stateOf reads the columns a regrade or a cancel may change.
type submissionState struct {
	Status      string
	Attempts    int
	Stale       bool
	Total, Max  *int
	GradedAt    *time.Time
	SubmittedAt time.Time
	IsLate      bool
	Results     int
}

func readState(t *testing.T, env *testEnvironment, submissionID int64) submissionState {
	t.Helper()
	var s submissionState
	err := env.pool.QueryRow(t.Context(), `
		SELECT status, attempts, stale_grade, total_score, max_score, graded_at, submitted_at, is_late,
			(SELECT count(*) FROM results WHERE submission_id = s.id)
		FROM submissions s WHERE id = $1`, submissionID,
	).Scan(&s.Status, &s.Attempts, &s.Stale, &s.Total, &s.Max, &s.GradedAt, &s.SubmittedAt, &s.IsLate, &s.Results)
	if err != nil {
		t.Fatalf("read submission %d: %v", submissionID, err)
	}
	return s
}

func TestRegradeUsesTheNormalWorkerPathAndReplacesTheGrade(t *testing.T) {
	env := newTestEnvironment(t)
	f := newRegradeFixture(t, env)
	addTestCase(t, env, f, "tc_01", 1)
	heavy := addTestCase(t, env, f, "tc_02", 1)

	submissionID := uploadAsStudent(t, env, f, f.assignmentID)
	// A worker takes the original job and grades it: 2 of 2.
	if _, err := env.jobs.Dequeue(t.Context()); err != nil {
		t.Fatal(err)
	}
	gradeLikeAWorker(t, env, submissionID)
	before := readState(t, env, submissionID)
	if before.Status != "COMPLETED" || *before.Total != 2 || *before.Max != 2 || before.Results != 2 {
		t.Fatalf("first grading = %+v; want COMPLETED 2/2 with 2 results", before)
	}

	// The lecturer raises a weight: the grade is stale (SRS 7.1), nothing is regraded by itself.
	updated, payload := env.request(t, http.MethodPut, fmt.Sprintf("/api/v1/testcases/%d", heavy), map[string]any{
		"external_id": "tc_02", "input": "in", "expected_output": "out", "weight": 5,
		"is_hidden": false, "match_mode": "EXACT",
	}, f.lecturerAccess)
	if updated.StatusCode != http.StatusOK {
		t.Fatalf("update weight = %d, %#v", updated.StatusCode, payload)
	}
	assertSubmissionStale(t, env, submissionID, true)
	if got := readState(t, env, submissionID); got.Status != "COMPLETED" || *got.Total != 2 {
		t.Fatalf("a test case edit changed the grade by itself: %+v", got)
	}

	path := fmt.Sprintf("/api/v1/submissions/%d/regrade", submissionID)
	response, regraded := env.request(t, http.MethodPost, path, nil, f.lecturerAccess)
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("regrade = %d, %#v; want 202", response.StatusCode, regraded)
	}
	if regraded["status"] != "QUEUED" || regraded["stale_grade"] != false || regraded["is_late"] != before.IsLate {
		t.Fatalf("regrade response = %#v", regraded)
	}
	for _, field := range []string{"total_score", "max_score", "graded_at"} {
		if _, present := regraded[field]; present {
			t.Fatalf("regrade response still has %s: %#v", field, regraded)
		}
	}
	queued := readState(t, env, submissionID)
	if queued.Status != "QUEUED" || queued.Attempts != 0 || queued.Stale || queued.Total != nil || queued.Max != nil ||
		queued.GradedAt != nil || queued.Results != 0 {
		t.Fatalf("after regrade = %+v; want QUEUED, attempts 0, no grade, no results, not stale", queued)
	}
	if !queued.SubmittedAt.Equal(before.SubmittedAt) || queued.IsLate != before.IsLate {
		t.Fatalf("regrade changed submitted_at/is_late: %+v -> %+v", before, queued)
	}

	// The job is in the queue again, for the student who owns the submission.
	job, err := env.jobs.Dequeue(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if job != (queue.Job{SubmissionID: submissionID, UserID: f.student.ID}) {
		t.Fatalf("regrade queued %+v; want submission %d for user %d", job, submissionID, f.student.ID)
	}

	// The worker grades it the way it grades any submission, against the edited definition.
	gradeLikeAWorker(t, env, submissionID)
	after := readState(t, env, submissionID)
	if after.Status != "COMPLETED" || *after.Total != 6 || *after.Max != 6 || after.Results != 2 || after.Stale || after.Attempts != 0 {
		t.Fatalf("second grading = %+v; want COMPLETED 6/6 with 2 results, not stale", after)
	}
	if !after.SubmittedAt.Equal(before.SubmittedAt) || after.IsLate != before.IsLate {
		t.Fatalf("regrading changed submitted_at/is_late: %+v -> %+v", before, after)
	}
	detail, detailPayload := env.request(t, http.MethodGet, fmt.Sprintf("/api/v1/submissions/%d", submissionID), nil, f.studentAccess)
	if detail.StatusCode != http.StatusOK || int64Field(t, detailPayload, "total_score") != 6 || int64Field(t, detailPayload, "max_score") != 6 {
		t.Fatalf("detail after regrade = %d, %#v", detail.StatusCode, detailPayload)
	}
}

func TestRegradeAccess(t *testing.T) {
	env := newTestEnvironment(t)
	f := newRegradeFixture(t, env)
	for _, request := range []struct {
		name   string
		token  string
		want   int
		queued bool
	}{
		{"owner student", f.studentAccess, http.StatusForbidden, false},
		{"other student", f.otherStudentAuth, http.StatusNotFound, false},
		{"other lecturer", f.otherLecturerAuth, http.StatusNotFound, false},
		{"missing token", "", http.StatusUnauthorized, false},
		{"course lecturer", f.lecturerAccess, http.StatusAccepted, true},
		{"admin", f.adminAccess, http.StatusAccepted, true},
	} {
		t.Run(request.name, func(t *testing.T) {
			submissionID := uploadAsStudent(t, env, f, f.assignmentID)
			workerTookJob(t, env, submissionID)
			setStatus(t, env, submissionID, "COMPLETED")
			jobsBefore := len(env.jobs.enqueued())
			response, payload := env.request(t, http.MethodPost, fmt.Sprintf("/api/v1/submissions/%d/regrade", submissionID), nil, request.token)
			if response.StatusCode != request.want {
				t.Fatalf("regrade as %s = %d, %#v; want %d", request.name, response.StatusCode, payload, request.want)
			}
			switch request.want {
			case http.StatusForbidden:
				assertError(t, payload, "forbidden")
			case http.StatusNotFound:
				assertError(t, payload, "not_found")
			}
			wantStatus, wantJobs := "COMPLETED", 0
			if request.queued {
				wantStatus, wantJobs = "QUEUED", 1
			}
			if got := readState(t, env, submissionID).Status; got != wantStatus {
				t.Fatalf("status after %s = %s; want %s", request.name, got, wantStatus)
			}
			if got := len(env.jobs.enqueued()) - jobsBefore; got != wantJobs {
				t.Fatalf("%s queued %d jobs; want %d", request.name, got, wantJobs)
			}
		})
	}

	for _, id := range []string{"0", "abc", "999999"} {
		response, payload := env.request(t, http.MethodPost, "/api/v1/submissions/"+id+"/regrade", nil, f.adminAccess)
		if response.StatusCode != http.StatusNotFound {
			t.Fatalf("regrade submission %q = %d, %#v; want 404", id, response.StatusCode, payload)
		}
		assertError(t, payload, "not_found")
	}
}

func TestRegradeOnlyFromCompletedOrFailed(t *testing.T) {
	env := newTestEnvironment(t)
	f := newRegradeFixture(t, env)
	addTestCase(t, env, f, "tc_01", 1)

	for _, status := range []string{"QUEUED", "RUNNING", "CANCELLED"} {
		t.Run(status, func(t *testing.T) {
			submissionID := uploadAsStudent(t, env, f, f.assignmentID)
			setStatus(t, env, submissionID, status)
			jobsBefore := len(env.jobs.enqueued())
			response, payload := env.request(t, http.MethodPost, fmt.Sprintf("/api/v1/submissions/%d/regrade", submissionID), nil, f.lecturerAccess)
			if response.StatusCode != http.StatusConflict {
				t.Fatalf("regrade a %s submission = %d, %#v; want 409", status, response.StatusCode, payload)
			}
			assertError(t, payload, "invalid_status")
			if got := readState(t, env, submissionID).Status; got != status {
				t.Fatalf("a refused regrade moved %s to %s", status, got)
			}
			if len(env.jobs.enqueued()) != jobsBefore {
				t.Fatalf("a refused regrade queued a job")
			}
		})
	}

	t.Run("FAILED", func(t *testing.T) {
		submissionID := uploadAsStudent(t, env, f, f.assignmentID)
		workerTookJob(t, env, submissionID)
		setStatus(t, env, submissionID, "FAILED")
		// A submission that was lost twice before it failed starts again with a clean count.
		if _, err := env.pool.Exec(t.Context(), `UPDATE submissions SET attempts = 2 WHERE id = $1`, submissionID); err != nil {
			t.Fatal(err)
		}
		response, payload := env.request(t, http.MethodPost, fmt.Sprintf("/api/v1/submissions/%d/regrade", submissionID), nil, f.lecturerAccess)
		if response.StatusCode != http.StatusAccepted || payload["status"] != "QUEUED" {
			t.Fatalf("regrade a FAILED submission = %d, %#v; want 202 QUEUED", response.StatusCode, payload)
		}
		if got := readState(t, env, submissionID); got.Status != "QUEUED" || got.Attempts != 0 {
			t.Fatalf("after regrading FAILED = %+v", got)
		}
	})

	t.Run("code deleted", func(t *testing.T) {
		submissionID := uploadAsStudent(t, env, f, f.assignmentID)
		workerTookJob(t, env, submissionID)
		gradeLikeAWorker(t, env, submissionID)
		if _, err := env.pool.Exec(t.Context(), `UPDATE submissions SET code_deleted_at = now() WHERE id = $1`, submissionID); err != nil {
			t.Fatal(err)
		}
		before := readState(t, env, submissionID)
		jobsBefore := len(env.jobs.enqueued())
		response, payload := env.request(t, http.MethodPost, fmt.Sprintf("/api/v1/submissions/%d/regrade", submissionID), nil, f.lecturerAccess)
		if response.StatusCode != http.StatusGone {
			t.Fatalf("regrade with deleted code = %d, %#v; want 410", response.StatusCode, payload)
		}
		assertError(t, payload, "code_deleted")
		if after := readState(t, env, submissionID); after.Status != "COMPLETED" || after.Results != before.Results || *after.Total != *before.Total {
			t.Fatalf("a refused regrade changed the submission: %+v -> %+v", before, after)
		}
		if len(env.jobs.enqueued()) != jobsBefore {
			t.Fatalf("a refused regrade queued a job")
		}
	})
}

func TestRegradeAssignment(t *testing.T) {
	env := newTestEnvironment(t)
	f := newRegradeFixture(t, env)
	addTestCase(t, env, f, "tc_01", 1)

	other, otherPayload := env.request(t, http.MethodPost, f.coursePath+"/assignments", map[string]any{
		"title": "Another assignment", "description": "Other.",
		"deadline_at":   time.Now().Add(2 * time.Hour).UTC().Format(time.RFC3339Nano),
		"late_until_at": time.Now().Add(4 * time.Hour).UTC().Format(time.RFC3339Nano),
		"resource_limit": map[string]int{
			"cpu_time_ms": 1000, "memory_mb": 64, "max_processes": 5,
			"timeout_s": 5, "max_output_bytes": 1 << 20, "tmpfs_size_mb": 32,
		},
	}, f.lecturerAccess)
	if other.StatusCode != http.StatusCreated {
		t.Fatalf("create second assignment = %d, %#v", other.StatusCode, otherPayload)
	}
	otherAssignment := int64Field(t, otherPayload, "id")

	submit := func(assignmentID int64, status string) int64 {
		id := uploadAsStudent(t, env, f, assignmentID)
		if status != "QUEUED" {
			workerTookJob(t, env, id)
		}
		setStatus(t, env, id, status)
		return id
	}
	completed := submit(f.assignmentID, "COMPLETED")
	failed := submit(f.assignmentID, "FAILED")
	deleted := submit(f.assignmentID, "COMPLETED")
	if _, err := env.pool.Exec(t.Context(), `UPDATE submissions SET code_deleted_at = now() WHERE id = $1`, deleted); err != nil {
		t.Fatal(err)
	}
	waiting := submit(f.assignmentID, "QUEUED")
	running := submit(f.assignmentID, "RUNNING")
	cancelled := submit(f.assignmentID, "CANCELLED")
	elsewhere := submit(otherAssignment, "COMPLETED")
	keep := map[int64]string{
		deleted: "COMPLETED", waiting: "QUEUED", running: "RUNNING", cancelled: "CANCELLED", elsewhere: "COMPLETED",
	}

	path := f.assignmentPath() + "/regrade"
	for _, request := range []struct {
		name  string
		token string
		want  int
		code  string
	}{
		{"student who can see it", f.studentAccess, http.StatusForbidden, "forbidden"},
		{"student of no course", f.otherStudentAuth, http.StatusNotFound, "not_found"},
		{"other lecturer", f.otherLecturerAuth, http.StatusNotFound, "not_found"},
		{"missing token", "", http.StatusUnauthorized, ""},
	} {
		response, payload := env.request(t, http.MethodPost, path, nil, request.token)
		if response.StatusCode != request.want {
			t.Fatalf("regrade assignment as %s = %d, %#v; want %d", request.name, response.StatusCode, payload, request.want)
		}
		if request.code != "" {
			assertError(t, payload, request.code)
		}
		if got := readState(t, env, completed).Status; got != "COMPLETED" {
			t.Fatalf("a refused %s request moved a submission to %s", request.name, got)
		}
	}
	if response, payload := env.request(t, http.MethodPost, "/api/v1/assignments/999999/regrade", nil, f.adminAccess); response.StatusCode != http.StatusNotFound {
		t.Fatalf("regrade a missing assignment = %d, %#v; want 404", response.StatusCode, payload)
	}

	jobsBefore := len(env.jobs.enqueued())
	response, payload := env.request(t, http.MethodPost, path, nil, f.lecturerAccess)
	if response.StatusCode != http.StatusAccepted {
		t.Fatalf("regrade assignment = %d, %#v; want 202", response.StatusCode, payload)
	}
	if int64Field(t, payload, "assignment_id") != f.assignmentID || countField(t, payload, "queued") != 2 || countField(t, payload, "skipped") != 1 {
		t.Fatalf("regrade assignment body = %#v; want queued 2, skipped 1", payload)
	}
	for _, id := range []int64{completed, failed} {
		if got := readState(t, env, id); got.Status != "QUEUED" || got.Total != nil || got.Results != 0 {
			t.Fatalf("regraded submission %d = %+v", id, got)
		}
	}
	for id, status := range keep {
		if got := readState(t, env, id).Status; got != status {
			t.Fatalf("submission %d is %s after the regrade; want %s unchanged", id, got, status)
		}
	}
	queuedNow := env.jobs.enqueued()[jobsBefore:]
	if len(queuedNow) != 2 || queuedNow[0].SubmissionID != completed || queuedNow[1].SubmissionID != failed ||
		queuedNow[0].UserID != f.student.ID || queuedNow[1].UserID != f.student.ID {
		t.Fatalf("regrade queued %+v; want submissions %d and %d, oldest first, for user %d", queuedNow, completed, failed, f.student.ID)
	}

	// Nothing is COMPLETED or FAILED any more, so a second request finds nothing to do, and an admin
	// may make it.
	again, againPayload := env.request(t, http.MethodPost, path, nil, f.adminAccess)
	if again.StatusCode != http.StatusAccepted || countField(t, againPayload, "queued") != 0 || countField(t, againPayload, "skipped") != 1 {
		t.Fatalf("second regrade as admin = %d, %#v; want 202 with queued 0, skipped 1", again.StatusCode, againPayload)
	}
}

func TestReconcileTreatsRegradedAndCancelledRowsCorrectly(t *testing.T) {
	env := newTestEnvironment(t)
	f := newRegradeFixture(t, env)
	addTestCase(t, env, f, "tc_01", 1)

	regraded := uploadAsStudent(t, env, f, f.assignmentID)
	workerTookJob(t, env, regraded)
	gradeLikeAWorker(t, env, regraded)
	if response, payload := env.request(t, http.MethodPost, fmt.Sprintf("/api/v1/submissions/%d/regrade", regraded), nil, f.lecturerAccess); response.StatusCode != http.StatusAccepted {
		t.Fatalf("regrade = %d, %#v", response.StatusCode, payload)
	}
	cancelled := uploadAsStudent(t, env, f, f.assignmentID)
	if response, payload := env.request(t, http.MethodDelete, fmt.Sprintf("/api/v1/submissions/%d", cancelled), nil, f.studentAccess); response.StatusCode != http.StatusOK {
		t.Fatalf("cancel = %d, %#v", response.StatusCode, payload)
	}

	// A restart empties the in-memory queue; the database is what Reconcile rebuilds it from.
	jobs, err := store.NewPipeline(env.pool).Reconcile(context.Background(), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0] != (queue.Job{SubmissionID: regraded, UserID: f.student.ID}) {
		t.Fatalf("Reconcile listed %+v; want only the regraded submission %d", jobs, regraded)
	}
	if got := readState(t, env, cancelled).Status; got != "CANCELLED" {
		t.Fatalf("Reconcile moved a CANCELLED submission to %s", got)
	}
}
