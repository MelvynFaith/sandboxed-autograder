// Package store holds the scheduler's database access.
//
// Pipeline is the part the grading worker uses: it moves a submission through QUEUED, RUNNING and
// COMPLETED or FAILED (SRS Gambar 4) and writes the results. Every state change is a conditional
// UPDATE, so two workers, a cancel and a restart can race without a submission ending in a state the
// diagram does not allow.
package store

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/harness"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/queue"
)

var (
	// ErrNotQueued means the submission is not QUEUED: it was cancelled, a worker already took it, or
	// it does not exist. The caller drops the job.
	ErrNotQueued = errors.New("store: submission is not QUEUED")

	// ErrNotRunning means the submission is not RUNNING any more, for example because it was
	// requeued while its worker was still grading. Nothing was written.
	ErrNotRunning = errors.New("store: submission is not RUNNING")

	// ErrNoResourceLimit means the assignment has no resource limits, so it cannot be graded. The
	// submission is RUNNING when Claim returns this; the caller marks it FAILED.
	ErrNoResourceLimit = errors.New("store: assignment has no resource limit")
)

// Audit log event types written when a report is stored (FR-12).
const (
	EventSandboxViolation      = "SANDBOX_VIOLATION"
	EventResourceLimitExceeded = "RESOURCE_LIMIT_EXCEEDED"
)

// DefaultMaxAttempts is how many times a submission may be started before Reconcile gives up on it.
const DefaultMaxAttempts = 3

// Pipeline is the database side of grading. It is safe for concurrent use.
type Pipeline struct {
	pool *pgxpool.Pool

	// MaxAttempts is how many times a worker may start grading a submission that is then lost (the
	// scheduler died, or the worker was shut down) before Reconcile marks it FAILED instead of requeuing
	// it. Zero means DefaultMaxAttempts; a negative value means no limit. Set it before first use.
	MaxAttempts int
}

// NewPipeline returns a Pipeline that uses pool.
func NewPipeline(pool *pgxpool.Pool) *Pipeline { return &Pipeline{pool: pool} }

// Claim is a submission taken by a worker, with the definition to grade it against.
type Claim struct {
	SubmissionID int64
	UserID       int64
	AssignmentID int64
	// CodePath is the server-generated storage name of the student's file (SRS 7.1).
	CodePath   string
	Definition *harness.Definition
}

// Claim moves a QUEUED submission to RUNNING and reads what is needed to grade it: the assignment's
// resource limits and its test cases that are not soft-deleted, oldest first.
//
// It returns ErrNotQueued if the submission is in any other state. For any other error the caller
// calls Fail, which does nothing harmful if the submission never reached RUNNING; if the definition
// could not be read the submission is RUNNING, and there is no way back to QUEUED from here.
//
// Claiming clears stale_grade: the definition is read after the flag is cleared, so an edit that
// lands later sets it again, and one that landed earlier is part of what is graded. It also counts an
// attempt, which Complete and Fail clear, so that only a submission that keeps being lost mid-grading
// accumulates them (see MaxAttempts).
func (p *Pipeline) Claim(ctx context.Context, submissionID int64) (*Claim, error) {
	c := &Claim{SubmissionID: submissionID}
	err := p.pool.QueryRow(ctx, `
		UPDATE submissions SET status = 'RUNNING', stale_grade = false, attempts = attempts + 1
		WHERE id = $1 AND status = 'QUEUED'
		RETURNING user_id, assignment_id, code_path`, submissionID,
	).Scan(&c.UserID, &c.AssignmentID, &c.CodePath)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, ErrNotQueued
	}
	if err != nil {
		return nil, fmt.Errorf("store: claim submission %d: %w", submissionID, err)
	}

	if c.Definition, err = p.definition(ctx, c); err != nil {
		return nil, err
	}
	return c, nil
}

// definition reads the limits and test cases in one snapshot, so an edit between the two reads
// cannot give the Harness a mix of old and new.
func (p *Pipeline) definition(ctx context.Context, c *Claim) (*harness.Definition, error) {
	tx, err := p.pool.BeginTx(ctx, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly})
	if err != nil {
		return nil, fmt.Errorf("store: read definition for submission %d: %w", c.SubmissionID, err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	def := &harness.Definition{SchemaVersion: harness.DefinitionVersion, SubmissionID: c.SubmissionID}
	l := &def.ResourceLimit
	err = tx.QueryRow(ctx, `
		SELECT cpu_time_ms, memory_mb, max_processes, timeout_s, max_output_bytes, tmpfs_size_mb
		FROM resource_limits WHERE assignment_id = $1`, c.AssignmentID,
	).Scan(&l.CPUTimeMS, &l.MemoryMB, &l.MaxProcesses, &l.TimeoutS, &l.MaxOutputBytes, &l.TmpfsSizeMB)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, fmt.Errorf("%w: assignment %d", ErrNoResourceLimit, c.AssignmentID)
	}
	if err != nil {
		return nil, fmt.Errorf("store: read limits of assignment %d: %w", c.AssignmentID, err)
	}

	rows, err := tx.Query(ctx, `
		SELECT id, external_id, input, expected_output, weight, match_mode
		FROM test_cases WHERE assignment_id = $1 AND NOT is_deleted ORDER BY id`, c.AssignmentID)
	if err != nil {
		return nil, fmt.Errorf("store: read test cases of assignment %d: %w", c.AssignmentID, err)
	}
	defer rows.Close()
	for rows.Next() {
		var tc harness.TestCase
		if err := rows.Scan(&tc.ID, &tc.ExternalID, &tc.Input, &tc.ExpectedOutput, &tc.Weight, &tc.MatchMode); err != nil {
			return nil, fmt.Errorf("store: read test cases of assignment %d: %w", c.AssignmentID, err)
		}
		def.TestCases = append(def.TestCases, tc)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("store: read test cases of assignment %d: %w", c.AssignmentID, err)
	}
	return def, nil
}

// Complete stores a checked report and marks the submission COMPLETED, in one transaction: the
// results replace any earlier ones (a regrade), total_score and max_score are set, and each sandbox
// violation or exceeded resource limit gets an audit log row (FR-12).
//
// It returns ErrNotRunning, having written nothing, if the submission is not RUNNING. Any other
// error also rolls everything back and leaves the submission RUNNING, for the caller to fail.
func (p *Pipeline) Complete(ctx context.Context, c *Claim, r *harness.Report) error {
	err := pgx.BeginFunc(ctx, p.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx, `
			UPDATE submissions
			SET status = 'COMPLETED', total_score = $2, max_score = $3, graded_at = now(), attempts = 0
			WHERE id = $1 AND status = 'RUNNING'`, c.SubmissionID, r.TotalScore, r.MaxScore)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotRunning
		}
		if _, err := tx.Exec(ctx, `DELETE FROM results WHERE submission_id = $1`, c.SubmissionID); err != nil {
			return err
		}

		var batch pgx.Batch
		for i := range r.Results {
			res := &r.Results[i]
			batch.Queue(`
				INSERT INTO results (submission_id, testcase_id, status, actual_output, error_message,
					is_truncated, exec_time_ms, memory_kb, memory_source, score)
				VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`,
				c.SubmissionID, res.TestCaseID, res.Status, res.ActualOutput, res.ErrorMessage,
				res.IsTruncated, res.ExecTimeMS, res.MemoryKB, res.MemorySource, res.Score)
			if event, syscall, detail := audit(res); event != "" {
				batch.Queue(`
					INSERT INTO audit_logs (user_id, submission_id, event_type, syscall_nr, detail)
					VALUES ($1, $2, $3, $4, $5)`, c.UserID, c.SubmissionID, event, syscall, detail)
			}
		}
		results := tx.SendBatch(ctx, &batch)
		for range batch.Len() {
			if _, err := results.Exec(); err != nil {
				_ = results.Close()
				return err
			}
		}
		return results.Close()
	})
	if errors.Is(err, ErrNotRunning) {
		return err
	}
	if err != nil {
		return fmt.Errorf("store: complete submission %d: %w", c.SubmissionID, err)
	}
	return nil
}

// audit says whether a result is an audit event, and describes it. The syscall number is nil unless
// the sandbox reported one.
func audit(res *harness.Result) (event string, syscall *int, detail string) {
	switch res.Status {
	case harness.StatusSecurityViolation:
		if res.Violation == nil {
			return EventSandboxViolation, nil, fmt.Sprintf("test case %s: the sandbox denied a syscall", res.ExternalID)
		}
		nr := res.Violation.SyscallNr
		return EventSandboxViolation, &nr, fmt.Sprintf("test case %s: the sandbox denied syscall %d", res.ExternalID, nr)
	case harness.StatusTimeout:
		return EventResourceLimitExceeded, nil, fmt.Sprintf("test case %s: time limit exceeded", res.ExternalID)
	case harness.StatusMemoryLimitExceeded:
		return EventResourceLimitExceeded, nil, fmt.Sprintf("test case %s: memory limit exceeded", res.ExternalID)
	}
	return "", nil, ""
}

// Fail marks a RUNNING submission FAILED: the grading pipeline broke, not the student's program. It
// returns ErrNotRunning if the submission is in another state.
func (p *Pipeline) Fail(ctx context.Context, submissionID int64) error {
	tag, err := p.pool.Exec(ctx, `
		UPDATE submissions SET status = 'FAILED', attempts = 0 WHERE id = $1 AND status = 'RUNNING'`, submissionID)
	if err != nil {
		return fmt.Errorf("store: fail submission %d: %w", submissionID, err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotRunning
	}
	return nil
}

// Reconcile prepares the queue after a start-up, or after a worker is lost (NFR-04). In one
// transaction it returns to QUEUED every RUNNING submission that has not changed for staleAfter, and
// then lists every QUEUED submission, oldest first, for the caller to enqueue. A staleAfter of zero
// requeues all RUNNING submissions, which is right at start-up when nothing is running yet.
//
// A stale RUNNING submission that has already been started MaxAttempts times is marked FAILED instead
// (and logged): grading it keeps losing the worker, so requeuing it again would only repeat the crash.
// A regrade of it starts with a clean count.
//
// The queue is in memory, so a restart empties it while the database still says QUEUED; this list is
// how the queue is rebuilt. Calling it while the queue holds jobs lists those too, and the queue
// rejects the duplicates.
func (p *Pipeline) Reconcile(ctx context.Context, staleAfter time.Duration) ([]queue.Job, error) {
	var jobs []queue.Job
	maxAttempts := p.MaxAttempts
	if maxAttempts == 0 {
		maxAttempts = DefaultMaxAttempts
	}
	err := pgx.BeginFunc(ctx, p.pool, func(tx pgx.Tx) error {
		if maxAttempts > 0 {
			rows, err := tx.Query(ctx, `
				UPDATE submissions SET status = 'FAILED', attempts = 0
				WHERE status = 'RUNNING' AND attempts >= $2 AND updated_at <= now() - make_interval(secs => $1)
				RETURNING id`, staleAfter.Seconds(), maxAttempts)
			if err != nil {
				return err
			}
			ids, err := pgx.CollectRows(rows, pgx.RowTo[int64])
			if err != nil {
				return err
			}
			if len(ids) > 0 {
				slog.Warn("gave up on submissions that were lost mid-grading", "submissions", ids, "attempts", maxAttempts)
			}
		}
		if _, err := tx.Exec(ctx, `
			UPDATE submissions SET status = 'QUEUED'
			WHERE status = 'RUNNING' AND updated_at <= now() - make_interval(secs => $1)`,
			staleAfter.Seconds()); err != nil {
			return err
		}
		rows, err := tx.Query(ctx, `SELECT id, user_id FROM submissions WHERE status = 'QUEUED' ORDER BY submitted_at, id`)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			var j queue.Job
			if err := rows.Scan(&j.SubmissionID, &j.UserID); err != nil {
				return err
			}
			jobs = append(jobs, j)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, fmt.Errorf("store: reconcile: %w", err)
	}
	return jobs, nil
}
