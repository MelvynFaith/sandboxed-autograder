package store_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/harness"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/queue"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/store"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/testdb"
)

// env is a migrated schema holding one student, one assignment with limits and two live test cases
// (and one soft-deleted), and a Pipeline on it.
type env struct {
	pool                *pgxpool.Pool
	p                   *store.Pipeline
	student, assignment int64
	tc1, tc2            int64
}

func setup(t *testing.T) *env {
	t.Helper()
	pool := testdb.Pool(t)
	e := &env{pool: pool, p: store.NewPipeline(pool)}

	lecturer := e.insert(t, `INSERT INTO users (name, email, password_hash, role)
		VALUES ('Lecturer', 'lecturer@example.test', 'x', 'LECTURER') RETURNING id`)
	e.student = e.insert(t, `INSERT INTO users (name, email, password_hash, role)
		VALUES ('Student', 'student@example.test', 'x', 'STUDENT') RETURNING id`)
	course := e.insert(t, `INSERT INTO courses (code, name, lecturer_id, term) VALUES ('C1', 'Course', $1, '2026-1') RETURNING id`, lecturer)
	e.assignment = e.insert(t, `INSERT INTO assignments (course_id, title, deadline_at, late_until_at)
		VALUES ($1, 'A1', now(), now()) RETURNING id`, course)
	e.insert(t, `INSERT INTO resource_limits (assignment_id, cpu_time_ms, memory_mb, max_processes, timeout_s,
		max_output_bytes, tmpfs_size_mb) VALUES ($1, 2000, 64, 5, 5, 1048576, 32) RETURNING id`, e.assignment)
	e.tc1 = e.insert(t, `INSERT INTO test_cases (assignment_id, external_id, input, expected_output, weight, match_mode)
		VALUES ($1, 'tc_01', E'5\n', E'hidden-expected\n', 20, 'EXACT') RETURNING id`, e.assignment)
	e.tc2 = e.insert(t, `INSERT INTO test_cases (assignment_id, external_id, input, expected_output, weight, match_mode)
		VALUES ($1, 'tc_02', '0', '1', 30, 'WHITESPACE_INSENSITIVE') RETURNING id`, e.assignment)
	e.insert(t, `INSERT INTO test_cases (assignment_id, external_id, input, expected_output, weight, match_mode, is_deleted)
		VALUES ($1, 'tc_03', '', '', 10, 'EXACT', true) RETURNING id`, e.assignment)
	return e
}

func (e *env) insert(t *testing.T, query string, args ...any) int64 {
	t.Helper()
	var id int64
	if err := e.pool.QueryRow(context.Background(), query, args...).Scan(&id); err != nil {
		t.Fatalf("%v\n%s", err, query)
	}
	return id
}

func (e *env) exec(t *testing.T, query string, args ...any) {
	t.Helper()
	if _, err := e.pool.Exec(context.Background(), query, args...); err != nil {
		t.Fatalf("%v\n%s", err, query)
	}
}

func (e *env) count(t *testing.T, query string, args ...any) int {
	t.Helper()
	var n int
	if err := e.pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("%v\n%s", err, query)
	}
	return n
}

// submission inserts a submission in the given status, submitted at the given time.
func (e *env) submission(t *testing.T, status string, at time.Time) int64 {
	t.Helper()
	return e.insert(t, `INSERT INTO submissions (user_id, assignment_id, code_path, original_filename, size_bytes, status, submitted_at)
		VALUES ($1, $2, 'stored.py', 'main.py', 10, $3, $4) RETURNING id`, e.student, e.assignment, status, at)
}

func (e *env) status(t *testing.T, id int64) string {
	t.Helper()
	var s string
	if err := e.pool.QueryRow(context.Background(), `SELECT status FROM submissions WHERE id = $1`, id).Scan(&s); err != nil {
		t.Fatal(err)
	}
	return s
}

// claimed returns a submission that a worker has claimed.
func (e *env) claimed(t *testing.T) (int64, *store.Claim) {
	t.Helper()
	id := e.submission(t, "QUEUED", time.Now())
	c, err := e.p.Claim(context.Background(), id)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	return id, c
}

// report answers the claim's definition with the given status for each test case, in order. A PASSED
// test case scores its weight; the others score 0.
func report(c *store.Claim, statuses ...string) *harness.Report {
	r := &harness.Report{SubmissionID: c.SubmissionID}
	for i, tc := range c.Definition.TestCases {
		res := harness.Result{
			TestCaseID: tc.ID, ExternalID: tc.ExternalID, Status: statuses[i], ActualOutput: "out\n",
			ExecTimeMS: 12, MemoryKB: 4096, MemorySource: harness.MemoryCgroupPeak,
		}
		if res.Status == harness.StatusPassed {
			res.Score = tc.Weight
		}
		if res.Status == harness.StatusSecurityViolation {
			res.Violation = &harness.Violation{SyscallNr: 41}
		}
		r.MaxScore += tc.Weight
		r.TotalScore += res.Score
		r.Results = append(r.Results, res)
	}
	return r
}

func TestClaimMovesToRunningAndReadsTheDefinition(t *testing.T) {
	e := setup(t)
	id := e.submission(t, "QUEUED", time.Now())
	e.exec(t, `UPDATE submissions SET stale_grade = true WHERE id = $1`, id)

	c, err := e.p.Claim(context.Background(), id)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if c.SubmissionID != id || c.UserID != e.student || c.AssignmentID != e.assignment || c.CodePath != "stored.py" {
		t.Errorf("claim = %+v", c)
	}
	if got := e.status(t, id); got != "RUNNING" {
		t.Errorf("status = %s, want RUNNING", got)
	}
	if e.count(t, `SELECT count(*) FROM submissions WHERE id = $1 AND stale_grade`, id) != 0 {
		t.Error("stale_grade was not cleared")
	}

	def := c.Definition
	if def.SchemaVersion != harness.DefinitionVersion || def.SubmissionID != id {
		t.Errorf("definition header = %d, %d", def.SchemaVersion, def.SubmissionID)
	}
	if want := (harness.ResourceLimit{CPUTimeMS: 2000, MemoryMB: 64, MaxProcesses: 5, TimeoutS: 5, MaxOutputBytes: 1 << 20, TmpfsSizeMB: 32}); def.ResourceLimit != want {
		t.Errorf("limits = %+v, want %+v", def.ResourceLimit, want)
	}
	// The soft-deleted test case is not graded, and the rest come oldest first.
	if len(def.TestCases) != 2 || def.TestCases[0].ID != e.tc1 || def.TestCases[1].ID != e.tc2 {
		t.Fatalf("test cases = %+v", def.TestCases)
	}
	want := harness.TestCase{ID: e.tc1, ExternalID: "tc_01", Input: "5\n", ExpectedOutput: "hidden-expected\n", Weight: 20, MatchMode: harness.MatchExact}
	if def.TestCases[0] != want {
		t.Errorf("test case = %+v, want %+v", def.TestCases[0], want)
	}
}

func TestClaimRefusesWhatIsNotQueued(t *testing.T) {
	e := setup(t)
	for _, status := range []string{"RUNNING", "COMPLETED", "FAILED", "CANCELLED"} {
		t.Run(status, func(t *testing.T) {
			id := e.submission(t, status, time.Now())
			if _, err := e.p.Claim(context.Background(), id); !errors.Is(err, store.ErrNotQueued) {
				t.Fatalf("err = %v, want ErrNotQueued", err)
			}
			if got := e.status(t, id); got != status {
				t.Fatalf("status changed to %s", got)
			}
		})
	}
	if _, err := e.p.Claim(context.Background(), 424242); !errors.Is(err, store.ErrNotQueued) {
		t.Fatalf("unknown id: err = %v, want ErrNotQueued", err)
	}
}

func TestOnlyOneClaimWins(t *testing.T) {
	e := setup(t)
	id := e.submission(t, "QUEUED", time.Now())
	if _, err := e.p.Claim(context.Background(), id); err != nil {
		t.Fatalf("first Claim: %v", err)
	}
	if _, err := e.p.Claim(context.Background(), id); !errors.Is(err, store.ErrNotQueued) {
		t.Fatalf("second Claim: err = %v, want ErrNotQueued", err)
	}
}

func TestClaimWithoutResourceLimitLeavesTheSubmissionToFail(t *testing.T) {
	e := setup(t)
	e.exec(t, `DELETE FROM resource_limits`)
	id := e.submission(t, "QUEUED", time.Now())

	if _, err := e.p.Claim(context.Background(), id); !errors.Is(err, store.ErrNoResourceLimit) {
		t.Fatalf("err = %v, want ErrNoResourceLimit", err)
	}
	if got := e.status(t, id); got != "RUNNING" {
		t.Fatalf("status = %s, want RUNNING so that the worker can fail it", got)
	}
	if err := e.p.Fail(context.Background(), id); err != nil {
		t.Fatalf("Fail: %v", err)
	}
}

func TestCompleteStoresResultsTotalsAndAuditEvents(t *testing.T) {
	tests := []struct {
		name        string
		statuses    []string
		wantEvent   string
		wantSyscall bool
	}{
		{"sandbox violation", []string{harness.StatusPassed, harness.StatusSecurityViolation}, store.EventSandboxViolation, true},
		{"time limit", []string{harness.StatusPassed, harness.StatusTimeout}, store.EventResourceLimitExceeded, false},
		{"memory limit", []string{harness.StatusMemoryLimitExceeded, harness.StatusPassed}, store.EventResourceLimitExceeded, false},
		{"ordinary failures are not audited", []string{harness.StatusFailed, harness.StatusRuntimeError}, "", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			e := setup(t)
			id, c := e.claimed(t)
			r := report(c, tc.statuses...)
			msg := "Traceback"
			r.Results[0].ErrorMessage = &msg

			if err := e.p.Complete(context.Background(), c, r); err != nil {
				t.Fatalf("Complete: %v", err)
			}

			var status string
			var total, maxScore int
			var gradedSet bool
			if err := e.pool.QueryRow(context.Background(),
				`SELECT status, total_score, max_score, graded_at IS NOT NULL FROM submissions WHERE id = $1`, id,
			).Scan(&status, &total, &maxScore, &gradedSet); err != nil {
				t.Fatal(err)
			}
			if status != "COMPLETED" || total != r.TotalScore || maxScore != 50 || !gradedSet {
				t.Errorf("submission = %s %d/%d graded_at set: %v", status, total, maxScore, gradedSet)
			}

			var actual, source string
			var errMsg *string
			var score, execMS, memKB int
			var truncated bool
			if err := e.pool.QueryRow(context.Background(),
				`SELECT actual_output, error_message, is_truncated, exec_time_ms, memory_kb, memory_source, score
				 FROM results WHERE submission_id = $1 AND testcase_id = $2`, id, e.tc1,
			).Scan(&actual, &errMsg, &truncated, &execMS, &memKB, &source, &score); err != nil {
				t.Fatal(err)
			}
			if actual != "out\n" || errMsg == nil || *errMsg != msg || truncated || execMS != 12 || memKB != 4096 || source != "CGROUP_PEAK" {
				t.Errorf("result = %q %v %v %d %d %s", actual, errMsg, truncated, execMS, memKB, source)
			}
			if n := e.count(t, `SELECT count(*) FROM results WHERE submission_id = $1`, id); n != 2 {
				t.Errorf("%d result rows, want 2", n)
			}

			wantAudit := 0
			if tc.wantEvent != "" {
				wantAudit = 1
			}
			if n := e.count(t, `SELECT count(*) FROM audit_logs WHERE submission_id = $1`, id); n != wantAudit {
				t.Fatalf("%d audit rows, want %d", n, wantAudit)
			}
			if wantAudit == 0 {
				return
			}
			var user int64
			var event, detail string
			var syscall *int
			if err := e.pool.QueryRow(context.Background(),
				`SELECT user_id, event_type, syscall_nr, detail FROM audit_logs WHERE submission_id = $1`, id,
			).Scan(&user, &event, &syscall, &detail); err != nil {
				t.Fatal(err)
			}
			if user != e.student || event != tc.wantEvent || (syscall != nil) != tc.wantSyscall || !strings.Contains(detail, "tc_0") {
				t.Errorf("audit = user %d, %s, syscall %v, %q", user, event, syscall, detail)
			}
			if tc.wantSyscall && *syscall != 41 {
				t.Errorf("syscall_nr = %d, want 41", *syscall)
			}
		})
	}
}

func TestCompleteRollsBackEverythingOnAnError(t *testing.T) {
	e := setup(t)
	id, c := e.claimed(t)
	// The first result queues an audit row; the second names a test case that does not exist, which
	// fails after the submission was updated and the first rows were queued.
	r := report(c, harness.StatusSecurityViolation, harness.StatusPassed)
	r.Results[1].TestCaseID = 999999

	if err := e.p.Complete(context.Background(), c, r); err == nil {
		t.Fatal("Complete succeeded with a result for an unknown test case")
	}
	if got := e.status(t, id); got != "RUNNING" {
		t.Errorf("status = %s, want RUNNING", got)
	}
	if n := e.count(t, `SELECT count(*) FROM results`); n != 0 {
		t.Errorf("%d result rows survived the rollback", n)
	}
	if n := e.count(t, `SELECT count(*) FROM audit_logs`); n != 0 {
		t.Errorf("%d audit rows survived the rollback", n)
	}
	if n := e.count(t, `SELECT count(*) FROM submissions WHERE total_score IS NOT NULL`); n != 0 {
		t.Error("total_score survived the rollback")
	}
}

func TestCompleteReplacesTheResultsOfAnEarlierGrading(t *testing.T) {
	e := setup(t)
	id, c := e.claimed(t)
	if err := e.p.Complete(context.Background(), c, report(c, harness.StatusPassed, harness.StatusPassed)); err != nil {
		t.Fatalf("first Complete: %v", err)
	}

	// A regrade (FR-17): the submission goes back to QUEUED and is graded again.
	e.exec(t, `UPDATE submissions SET status = 'QUEUED' WHERE id = $1`, id)
	c, err := e.p.Claim(context.Background(), id)
	if err != nil {
		t.Fatalf("Claim: %v", err)
	}
	if err := e.p.Complete(context.Background(), c, report(c, harness.StatusFailed, harness.StatusPassed)); err != nil {
		t.Fatalf("second Complete: %v", err)
	}

	if n := e.count(t, `SELECT count(*) FROM results WHERE submission_id = $1`, id); n != 2 {
		t.Fatalf("%d result rows, want 2", n)
	}
	if n := e.count(t, `SELECT count(*) FROM results WHERE submission_id = $1 AND testcase_id = $2 AND status = 'FAILED'`, id, e.tc1); n != 1 {
		t.Error("the first result was not replaced")
	}
	if n := e.count(t, `SELECT count(*) FROM submissions WHERE id = $1 AND total_score = 30 AND max_score = 50`, id); n != 1 {
		t.Error("totals were not updated")
	}
}

func TestCompleteRefusesASubmissionThatIsNotRunning(t *testing.T) {
	e := setup(t)
	id, c := e.claimed(t)
	// The submission was requeued while the worker was still grading.
	e.exec(t, `UPDATE submissions SET status = 'QUEUED' WHERE id = $1`, id)

	if err := e.p.Complete(context.Background(), c, report(c, harness.StatusPassed, harness.StatusPassed)); !errors.Is(err, store.ErrNotRunning) {
		t.Fatalf("err = %v, want ErrNotRunning", err)
	}
	if n := e.count(t, `SELECT count(*) FROM results`); n != 0 {
		t.Errorf("%d result rows were written", n)
	}
	if got := e.status(t, id); got != "QUEUED" {
		t.Errorf("status = %s, want QUEUED", got)
	}
}

func TestFail(t *testing.T) {
	e := setup(t)
	id, _ := e.claimed(t)
	if err := e.p.Fail(context.Background(), id); err != nil {
		t.Fatalf("Fail: %v", err)
	}
	if got := e.status(t, id); got != "FAILED" {
		t.Fatalf("status = %s, want FAILED", got)
	}
	if err := e.p.Fail(context.Background(), id); !errors.Is(err, store.ErrNotRunning) {
		t.Fatalf("second Fail: err = %v, want ErrNotRunning", err)
	}
	queued := e.submission(t, "QUEUED", time.Now())
	if err := e.p.Fail(context.Background(), queued); !errors.Is(err, store.ErrNotRunning) {
		t.Fatalf("Fail on QUEUED: err = %v, want ErrNotRunning", err)
	}
	if got := e.status(t, queued); got != "QUEUED" {
		t.Fatalf("status = %s, want QUEUED", got)
	}
}

// backdate makes a submission look as if it last changed d ago. A trigger sets updated_at on every
// change, so it is switched off for the statement.
func (e *env) backdate(t *testing.T, id int64, d time.Duration) {
	t.Helper()
	e.exec(t, `ALTER TABLE submissions DISABLE TRIGGER submissions_set_updated_at`)
	e.exec(t, `UPDATE submissions SET updated_at = now() - make_interval(secs => $2) WHERE id = $1`, id, d.Seconds())
	e.exec(t, `ALTER TABLE submissions ENABLE TRIGGER submissions_set_updated_at`)
}

func ids(jobs []queue.Job) []int64 {
	out := make([]int64, len(jobs))
	for i, j := range jobs {
		out[i] = j.SubmissionID
	}
	return out
}

func TestReconcile(t *testing.T) {
	e := setup(t)
	t0 := time.Now().Add(-time.Hour)
	stale := e.submission(t, "RUNNING", t0)
	queued1 := e.submission(t, "QUEUED", t0.Add(1*time.Minute))
	fresh := e.submission(t, "RUNNING", t0.Add(2*time.Minute))
	queued2 := e.submission(t, "QUEUED", t0.Add(3*time.Minute))
	done := e.submission(t, "COMPLETED", t0.Add(4*time.Minute))
	e.backdate(t, stale, time.Hour)

	jobs, err := e.p.Reconcile(context.Background(), 10*time.Minute)
	if err != nil {
		t.Fatalf("Reconcile: %v", err)
	}
	if want := []int64{stale, queued1, queued2}; !slices.Equal(ids(jobs), want) {
		t.Fatalf("jobs = %v, want %v (oldest first, the fresh RUNNING one left alone)", ids(jobs), want)
	}
	for _, j := range jobs {
		if j.UserID != e.student {
			t.Errorf("job %d has user %d, want %d", j.SubmissionID, j.UserID, e.student)
		}
	}
	for id, want := range map[int64]string{stale: "QUEUED", fresh: "RUNNING", done: "COMPLETED"} {
		if got := e.status(t, id); got != want {
			t.Errorf("submission %d is %s, want %s", id, got, want)
		}
	}

	// At start-up nothing is running yet, so a staleness of zero requeues the rest.
	jobs, err = e.p.Reconcile(context.Background(), 0)
	if err != nil {
		t.Fatalf("Reconcile(0): %v", err)
	}
	if want := []int64{stale, queued1, fresh, queued2}; !slices.Equal(ids(jobs), want) {
		t.Fatalf("jobs = %v, want %v", ids(jobs), want)
	}
}

func (e *env) attempts(t *testing.T, id int64) int {
	t.Helper()
	return e.count(t, `SELECT attempts FROM submissions WHERE id = $1`, id)
}

func TestAttemptsAreCountedByClaimAndClearedByCompleteAndFail(t *testing.T) {
	e := setup(t)
	id, c := e.claimed(t)
	if got := e.attempts(t, id); got != 1 {
		t.Fatalf("attempts after Claim = %d, want 1", got)
	}
	if err := e.p.Complete(context.Background(), c, report(c, harness.StatusPassed, harness.StatusPassed)); err != nil {
		t.Fatal(err)
	}
	if got := e.attempts(t, id); got != 0 {
		t.Errorf("attempts after Complete = %d, want 0", got)
	}

	id, _ = e.claimed(t)
	if err := e.p.Fail(context.Background(), id); err != nil {
		t.Fatal(err)
	}
	if got := e.attempts(t, id); got != 0 {
		t.Errorf("attempts after Fail = %d, want 0", got)
	}
}

// A submission that crashes the scheduler on every grading: it is claimed, the process dies, and
// start-up reconciliation runs. After MaxAttempts claims it must be given up on, not requeued again.
func TestReconcileGivesUpOnASubmissionThatKeepsBeingLost(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	id := e.submission(t, "QUEUED", time.Now())

	for attempt := 1; attempt <= store.DefaultMaxAttempts; attempt++ {
		if _, err := e.p.Claim(ctx, id); err != nil {
			t.Fatalf("Claim %d: %v", attempt, err)
		}
		jobs, err := e.p.Reconcile(ctx, 0) // the scheduler restarted
		if err != nil {
			t.Fatalf("Reconcile %d: %v", attempt, err)
		}
		if attempt < store.DefaultMaxAttempts {
			if len(jobs) != 1 || jobs[0].SubmissionID != id || e.status(t, id) != "QUEUED" {
				t.Fatalf("after attempt %d: jobs = %v, status %s; want it requeued", attempt, jobs, e.status(t, id))
			}
		}
	}
	if got := e.status(t, id); got != "FAILED" {
		t.Fatalf("status after %d lost attempts = %s, want FAILED", store.DefaultMaxAttempts, got)
	}
	if got := e.attempts(t, id); got != 0 {
		t.Errorf("attempts = %d, want 0 so that a regrade starts clean", got)
	}
	if jobs, _ := e.p.Reconcile(ctx, 0); len(jobs) != 0 {
		t.Errorf("the failed submission was listed again: %v", jobs)
	}
}

func TestMaxAttemptsIsConfigurable(t *testing.T) {
	e := setup(t)
	ctx := context.Background()
	stale := func(attempts int) int64 {
		id := e.submission(t, "RUNNING", time.Now())
		e.exec(t, `UPDATE submissions SET attempts = $2 WHERE id = $1`, id, attempts)
		return id
	}

	e.p.MaxAttempts = 5
	four, five := stale(4), stale(5)
	if _, err := e.p.Reconcile(ctx, 0); err != nil {
		t.Fatal(err)
	}
	if e.status(t, four) != "QUEUED" || e.status(t, five) != "FAILED" {
		t.Errorf("with a limit of 5: attempts 4 is %s, attempts 5 is %s; want QUEUED and FAILED", e.status(t, four), e.status(t, five))
	}

	e.p.MaxAttempts = -1 // no limit
	many := stale(1000)
	if _, err := e.p.Reconcile(ctx, 0); err != nil {
		t.Fatal(err)
	}
	if got := e.status(t, many); got != "QUEUED" {
		t.Errorf("with no limit, 1000 attempts is %s, want QUEUED", got)
	}
}

func TestFreshRunningSubmissionIsNotGivenUpOnBeforeItIsStale(t *testing.T) {
	e := setup(t)
	id := e.submission(t, "RUNNING", time.Now())
	e.exec(t, `UPDATE submissions SET attempts = 9 WHERE id = $1`, id)
	if _, err := e.p.Reconcile(context.Background(), time.Hour); err != nil {
		t.Fatal(err)
	}
	if got := e.status(t, id); got != "RUNNING" {
		t.Fatalf("status = %s, want RUNNING: it is not stale yet", got)
	}
}
