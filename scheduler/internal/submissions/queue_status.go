package submissions

import (
	"context"
	"fmt"
)

// ActiveCounts are how many submissions of all students wait for a worker and how many are being
// graded, as the database records them. The admin queue panel shows them (FR-13, US-21).
type ActiveCounts struct {
	Queued  int `json:"queued"`
	Running int `json:"running"`
}

// CountActive counts the QUEUED and RUNNING submissions. The database, not the in-memory queue, is
// the source of truth for status, so Queued can briefly differ from the queue depth: an upload is
// stored before it is enqueued, and after a restart a QUEUED row waits for Reconcile to enqueue it.
// The WHERE clause matches the partial index submissions_active_idx.
func (s *Store) CountActive(ctx context.Context) (ActiveCounts, error) {
	var c ActiveCounts
	err := s.pool.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE status = 'QUEUED'), count(*) FILTER (WHERE status = 'RUNNING')
		FROM submissions WHERE status IN ('QUEUED', 'RUNNING')`).Scan(&c.Queued, &c.Running)
	if err != nil {
		return ActiveCounts{}, fmt.Errorf("submission: count active: %w", err)
	}
	return c, nil
}
