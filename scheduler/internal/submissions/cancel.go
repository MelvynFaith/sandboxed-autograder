package submissions

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

var (
	// ErrCancelRunning means a worker has the submission, or took it while it was being cancelled. A
	// RUNNING submission is not cancelled, so that no execution stops half-way with partial results
	// (FR-16).
	ErrCancelRunning = errors.New("submission: a RUNNING submission cannot be cancelled")

	// ErrCancelStatus means the submission is neither QUEUED nor RUNNING: it is already graded,
	// failed or cancelled.
	ErrCancelStatus = errors.New("submission: only a QUEUED submission can be cancelled")
)

// Cancel moves a student's own QUEUED submission to CANCELLED (FR-16). The check and the change are
// one conditional UPDATE, so a worker that claims the submission at the same moment wins or loses
// cleanly: Claim only takes a QUEUED submission, so a job it dequeued before the cancel finds
// CANCELLED and is dropped.
//
// The caller removes the job from the queue afterwards. The database decides; the queue only orders.
//
// It returns ErrNotFound if userID has no such submission, ErrCancelRunning if a worker has it, and
// ErrCancelStatus for any other state; none of them changes anything.
func (s *Store) Cancel(ctx context.Context, submissionID, userID int64) error {
	// The CTE's UPDATE and the outer SELECT run on one snapshot, so the SELECT reports the state the
	// UPDATE saw: when nothing was cancelled it says why, without a second round trip that could race
	// with a worker or a regrade.
	var status string
	var cancelled bool
	err := s.pool.QueryRow(ctx, `
		WITH cancelled AS (
			UPDATE submissions SET status = 'CANCELLED'
			WHERE id = $1 AND user_id = $2 AND status = 'QUEUED'
			RETURNING id
		)
		SELECT s.status, EXISTS (SELECT 1 FROM cancelled)
		FROM submissions s WHERE s.id = $1 AND s.user_id = $2`, submissionID, userID,
	).Scan(&status, &cancelled)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return fmt.Errorf("submission: cancel %d: %w", submissionID, err)
	}
	switch {
	case cancelled:
		return nil
	case status == "RUNNING", status == "QUEUED":
		// QUEUED here means a worker claimed it after the snapshot and before the UPDATE's check.
		return ErrCancelRunning
	default:
		return ErrCancelStatus
	}
}
