package ratelimit_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/ratelimit"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/testdb"
)

type env struct {
	pool                *pgxpool.Pool
	student, assignment int64
	otherStudent        int64
	otherAssignment     int64
}

func setup(t *testing.T) *env {
	t.Helper()
	pool := testdb.Pool(t)
	e := &env{pool: pool}
	insert := func(query string, args ...any) int64 {
		t.Helper()
		var id int64
		if err := pool.QueryRow(context.Background(), query, args...).Scan(&id); err != nil {
			t.Fatalf("%v\n%s", err, query)
		}
		return id
	}
	lecturer := insert(`INSERT INTO users (name, email, password_hash, role) VALUES ('L', 'l@example.test', 'x', 'LECTURER') RETURNING id`)
	e.student = insert(`INSERT INTO users (name, email, password_hash, role) VALUES ('S', 's@example.test', 'x', 'STUDENT') RETURNING id`)
	e.otherStudent = insert(`INSERT INTO users (name, email, password_hash, role) VALUES ('T', 't@example.test', 'x', 'STUDENT') RETURNING id`)
	course := insert(`INSERT INTO courses (code, name, lecturer_id, term) VALUES ('C', 'C', $1, 't') RETURNING id`, lecturer)
	e.assignment = insert(`INSERT INTO assignments (course_id, title, deadline_at, late_until_at) VALUES ($1, 'A', now(), now()) RETURNING id`, course)
	e.otherAssignment = insert(`INSERT INTO assignments (course_id, title, deadline_at, late_until_at) VALUES ($1, 'B', now(), now()) RETURNING id`, course)
	return e
}

// submit inserts a submission with the given status, made ago before now.
func (e *env) submit(t *testing.T, user, assignment int64, status string, ago time.Duration) {
	t.Helper()
	_, err := e.pool.Exec(context.Background(), `
		INSERT INTO submissions (user_id, assignment_id, code_path, original_filename, size_bytes, status, submitted_at)
		VALUES ($1, $2, 'f', 'main.py', 1, $3, now() - make_interval(secs => $4))`,
		user, assignment, status, ago.Seconds())
	if err != nil {
		t.Fatal(err)
	}
}

// check runs Check in a transaction of its own and rolls it back.
func (e *env) check(t *testing.T, l ratelimit.Limits, user, assignment int64) error {
	t.Helper()
	ctx := context.Background()
	tx, err := e.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	return l.Check(ctx, tx, user, assignment)
}

func wantRule(t *testing.T, err error, rule ratelimit.Rule) *ratelimit.Error {
	t.Helper()
	var re *ratelimit.Error
	if !errors.As(err, &re) || re.Rule != rule {
		t.Fatalf("err = %v, want rule %d", err, rule)
	}
	return re
}

func TestFirstSubmissionIsAccepted(t *testing.T) {
	e := setup(t)
	if err := e.check(t, ratelimit.Default, e.student, e.assignment); err != nil {
		t.Fatalf("Check: %v", err)
	}
}

func TestOneActiveSubmissionPerAssignment(t *testing.T) {
	for _, status := range []string{"QUEUED", "RUNNING"} {
		t.Run(status, func(t *testing.T) {
			e := setup(t)
			e.submit(t, e.student, e.assignment, status, time.Hour)
			re := wantRule(t, e.check(t, ratelimit.Default, e.student, e.assignment), ratelimit.RuleActive)
			if re.Limit != 1 {
				t.Errorf("Limit = %d, want 1", re.Limit)
			}
		})
	}
	t.Run("finished submissions do not count", func(t *testing.T) {
		e := setup(t)
		for _, status := range []string{"COMPLETED", "FAILED", "CANCELLED"} {
			e.submit(t, e.student, e.assignment, status, time.Hour)
		}
		if err := e.check(t, ratelimit.Default, e.student, e.assignment); err != nil {
			t.Fatalf("Check: %v", err)
		}
	})
}

func TestMinimumIntervalBetweenSubmissions(t *testing.T) {
	e := setup(t)
	e.submit(t, e.student, e.assignment, "COMPLETED", 10*time.Second)
	re := wantRule(t, e.check(t, ratelimit.Default, e.student, e.assignment), ratelimit.RuleInterval)
	if re.RetryAfter < 19*time.Second || re.RetryAfter > 20*time.Second {
		t.Errorf("RetryAfter = %v, want about 20s", re.RetryAfter)
	}

	e = setup(t)
	e.submit(t, e.student, e.assignment, "COMPLETED", 31*time.Second)
	if err := e.check(t, ratelimit.Default, e.student, e.assignment); err != nil {
		t.Fatalf("after 31s: %v", err)
	}
}

func TestACancelledSubmissionStillCountsForTheInterval(t *testing.T) {
	e := setup(t)
	e.submit(t, e.student, e.assignment, "CANCELLED", 5*time.Second)
	wantRule(t, e.check(t, ratelimit.Default, e.student, e.assignment), ratelimit.RuleInterval)
}

func TestQuotaPerAssignmentIgnoresCancelled(t *testing.T) {
	e := setup(t)
	for range 30 {
		e.submit(t, e.student, e.assignment, "COMPLETED", time.Hour)
	}
	re := wantRule(t, e.check(t, ratelimit.Default, e.student, e.assignment), ratelimit.RuleQuota)
	if re.Limit != 30 {
		t.Errorf("Limit = %d, want 30", re.Limit)
	}

	e = setup(t)
	for range 29 {
		e.submit(t, e.student, e.assignment, "COMPLETED", time.Hour)
	}
	for range 10 {
		e.submit(t, e.student, e.assignment, "CANCELLED", time.Hour)
	}
	if err := e.check(t, ratelimit.Default, e.student, e.assignment); err != nil {
		t.Fatalf("29 counted submissions and 10 cancelled: %v", err)
	}
}

func TestLimitsAreSeparatePerStudentAndAssignment(t *testing.T) {
	e := setup(t)
	e.submit(t, e.student, e.assignment, "QUEUED", time.Minute)
	if err := e.check(t, ratelimit.Default, e.otherStudent, e.assignment); err != nil {
		t.Errorf("another student: %v", err)
	}
	if err := e.check(t, ratelimit.Default, e.student, e.otherAssignment); err != nil {
		t.Errorf("another assignment: %v", err)
	}
}

func TestThresholdsAreConfigurableAndZeroSwitchesARuleOff(t *testing.T) {
	e := setup(t)
	e.submit(t, e.student, e.assignment, "QUEUED", time.Second)

	two := ratelimit.Limits{MaxActive: 2}
	if err := e.check(t, two, e.student, e.assignment); err != nil {
		t.Errorf("one active under a limit of two: %v", err)
	}
	e.submit(t, e.student, e.assignment, "RUNNING", time.Second)
	wantRule(t, e.check(t, two, e.student, e.assignment), ratelimit.RuleActive)

	if err := e.check(t, ratelimit.Limits{}, e.student, e.assignment); err != nil {
		t.Errorf("no limits: %v", err)
	}
}

// Two uploads by the same student at the same moment must not both pass a limit of one. Each runs the
// way the handler will: Check, then the insert, in one transaction.
func TestSimultaneousUploadsCannotBothPass(t *testing.T) {
	e := setup(t)
	const uploads = 4
	var accepted, refused int
	var mu sync.Mutex
	var wg sync.WaitGroup
	for range uploads {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx := context.Background()
			err := pgx.BeginFunc(ctx, e.pool, func(tx pgx.Tx) error {
				if err := ratelimit.Default.Check(ctx, tx, e.student, e.assignment); err != nil {
					return err
				}
				time.Sleep(50 * time.Millisecond) // widen the window between the check and the insert
				_, err := tx.Exec(ctx, `
					INSERT INTO submissions (user_id, assignment_id, code_path, original_filename, size_bytes)
					VALUES ($1, $2, 'f', 'main.py', 1)`, e.student, e.assignment)
				return err
			})
			mu.Lock()
			defer mu.Unlock()
			var re *ratelimit.Error
			switch {
			case err == nil:
				accepted++
			case errors.As(err, &re):
				refused++
			default:
				t.Errorf("unexpected error: %v", err)
			}
		}()
	}
	wg.Wait()
	if accepted != 1 || refused != uploads-1 {
		t.Fatalf("%d accepted, %d refused; want 1 and %d", accepted, refused, uploads-1)
	}
}

func TestErrorMessages(t *testing.T) {
	for _, e := range []*ratelimit.Error{
		{Rule: ratelimit.RuleActive, Limit: 1},
		{Rule: ratelimit.RuleInterval, RetryAfter: 19400 * time.Millisecond},
		{Rule: ratelimit.RuleQuota, Limit: 30},
	} {
		if e.Error() == "" {
			t.Errorf("rule %d has an empty message", e.Rule)
		}
	}
}
