package submissions

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/queue"
)

var (
	// ErrRegradeStatus means the submission is not COMPLETED or FAILED, so there is no grade to redo
	// (FR-17): it is waiting or being graded already, or it was cancelled.
	ErrRegradeStatus = errors.New("submission: only a COMPLETED or FAILED submission can be regraded")

	// ErrRegradeCodeDeleted means the retention job deleted the stored file (NFR-13), so there is
	// nothing left to grade.
	ErrRegradeCodeDeleted = errors.New("submission: the stored code was deleted")
)

// Regrade puts a COMPLETED or FAILED submission back in the QUEUED state to be graded again (FR-17,
// SRS Gambar 4). In one transaction it clears total_score, max_score and graded_at, deletes the old
// results, and resets attempts, so the grading starts clean. submitted_at and is_late are not touched:
// a regrade is the same submission graded again, not a new one. FR-14 does not apply to it.
//
// stale_grade is left as it is. The worker clears it when it claims the submission, after which the
// definition it grades against is the current one (see store.Pipeline.Claim).
//
// It returns the job the caller must enqueue, which names the student that owns the submission. The
// row is locked while its state is checked, so two regrades, or a regrade and the retention job,
// cannot both act on it. ErrNotFound, ErrRegradeStatus and ErrRegradeCodeDeleted leave it unchanged;
// the status is checked first, because only a graded submission has a grade to redo.
func (s *Store) Regrade(ctx context.Context, submissionID int64) (queue.Job, error) {
	job := queue.Job{SubmissionID: submissionID}
	err := pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		var status string
		var codeDeleted bool
		err := tx.QueryRow(ctx, `
			SELECT user_id, status, code_deleted_at IS NOT NULL
			FROM submissions WHERE id = $1 FOR UPDATE`, submissionID,
		).Scan(&job.UserID, &status, &codeDeleted)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		if err != nil {
			return err
		}
		if status != "COMPLETED" && status != "FAILED" {
			return ErrRegradeStatus
		}
		if codeDeleted {
			return ErrRegradeCodeDeleted
		}
		return requeueForRegrade(ctx, tx, []int64{submissionID})
	})
	switch {
	case errors.Is(err, ErrNotFound), errors.Is(err, ErrRegradeStatus), errors.Is(err, ErrRegradeCodeDeleted):
		return queue.Job{}, err
	case err != nil:
		return queue.Job{}, fmt.Errorf("submission: regrade %d: %w", submissionID, err)
	}
	return job, nil
}

// RegradeAssignment regrades every COMPLETED or FAILED submission of an assignment whose code still
// exists, in one transaction, exactly as Regrade does for one. It returns the jobs to enqueue, oldest
// submission first, and how many COMPLETED or FAILED submissions were skipped because retention
// deleted their code. Submissions that are QUEUED, RUNNING or CANCELLED are not graded results and
// are not counted.
func (s *Store) RegradeAssignment(ctx context.Context, assignmentID int64) (jobs []queue.Job, skipped int, err error) {
	err = pgx.BeginFunc(ctx, s.pool, func(tx pgx.Tx) error {
		// Rows are locked in id order, the order a bulk UPDATE of the assignment tends to take them in,
		// so a test-case edit that marks them stale at the same time does not deadlock with this.
		rows, err := tx.Query(ctx, `
			SELECT id, user_id, code_deleted_at IS NOT NULL
			FROM submissions
			WHERE assignment_id = $1 AND status IN ('COMPLETED', 'FAILED')
			ORDER BY id FOR UPDATE`, assignmentID)
		if err != nil {
			return err
		}
		defer rows.Close()

		jobs, skipped = nil, 0
		var ids []int64
		for rows.Next() {
			var job queue.Job
			var codeDeleted bool
			if err := rows.Scan(&job.SubmissionID, &job.UserID, &codeDeleted); err != nil {
				return err
			}
			if codeDeleted {
				skipped++
				continue
			}
			jobs = append(jobs, job)
			ids = append(ids, job.SubmissionID)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		rows.Close()
		if len(ids) == 0 {
			return nil
		}
		return requeueForRegrade(ctx, tx, ids)
	})
	if err != nil {
		return nil, 0, fmt.Errorf("submission: regrade assignment %d: %w", assignmentID, err)
	}
	return jobs, skipped, nil
}

// requeueForRegrade is the change a regrade makes to the given rows, which the caller has locked and
// checked. The results go with the grade: a QUEUED submission has no results until it is graded
// again, and the worker would replace them anyway.
func requeueForRegrade(ctx context.Context, tx pgx.Tx, ids []int64) error {
	if _, err := tx.Exec(ctx, `
		UPDATE submissions
		SET status = 'QUEUED', attempts = 0, total_score = NULL, max_score = NULL, graded_at = NULL
		WHERE id = ANY($1)`, ids); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `DELETE FROM results WHERE submission_id = ANY($1)`, ids)
	return err
}
