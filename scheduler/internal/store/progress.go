package store

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/progress"
)

// ErrNotFound means the submission does not exist.
var ErrNotFound = errors.New("store: submission not found")

// Progress returns a submission's state as the database holds it, for a WebSocket client that has just
// connected (FR-08). Which test case is running is not stored, so a QUEUED, RUNNING, FAILED or
// CANCELLED submission reports test 0 of the assignment's live test cases; the worker's events fill in
// the rest. A COMPLETED one reports n of n, where n is the number of results it was graded with.
func (p *Pipeline) Progress(ctx context.Context, submissionID int64) (progress.Event, error) {
	e := progress.Event{SubmissionID: submissionID}
	err := p.pool.QueryRow(ctx, `
		SELECT s.status, CASE WHEN s.status = 'COMPLETED'
			THEN (SELECT count(*) FROM results r WHERE r.submission_id = s.id)
			ELSE (SELECT count(*) FROM test_cases t WHERE t.assignment_id = s.assignment_id AND NOT t.is_deleted)
		END
		FROM submissions s WHERE s.id = $1`, submissionID,
	).Scan(&e.Status, &e.TotalTests)
	if errors.Is(err, pgx.ErrNoRows) {
		return progress.Event{}, ErrNotFound
	}
	if err != nil {
		return progress.Event{}, fmt.Errorf("store: read progress of submission %d: %w", submissionID, err)
	}
	if e.Status == progress.StatusCompleted {
		e.CurrentTest = e.TotalTests
	}
	return e, nil
}
