// Package ratelimit enforces the per-student submission limits of FR-14: at most MaxActive submissions
// QUEUED or RUNNING per assignment, at least MinInterval between two submissions, and at most
// MaxPerAssignment submissions per assignment.
//
// The limits are counted in the database, not in memory, so they survive a restart and cannot be
// bypassed by a second connection. Check is meant to run in the same transaction that inserts the
// submission: it takes a lock per student and assignment, so two simultaneous uploads cannot both pass
// a limit of one.
package ratelimit

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
)

// Limits are the thresholds of FR-14. A value of zero or less switches that rule off. The SRS makes the
// thresholds configurable by an administrator; where they are stored is not decided yet, so the caller
// supplies them.
type Limits struct {
	MaxActive        int
	MinInterval      time.Duration
	MaxPerAssignment int
}

// Default holds the values given in FR-14.
var Default = Limits{MaxActive: 1, MinInterval: 30 * time.Second, MaxPerAssignment: 30}

// Rule names the limit that refused a submission.
type Rule int

const (
	RuleActive   Rule = iota + 1 // too many submissions QUEUED or RUNNING
	RuleInterval                 // too soon after the previous submission
	RuleQuota                    // too many submissions for this assignment
)

// Error is returned by Check when a limit is reached. FR-14 requires a clear refusal, so the handler
// maps it to 429 and a message that names the rule and, for RuleInterval, when to try again.
type Error struct {
	Rule Rule
	// Limit is the threshold that was reached: MaxActive or MaxPerAssignment. It is 0 for RuleInterval.
	Limit int
	// RetryAfter is how long until the next submission is accepted. It is set only for RuleInterval.
	RetryAfter time.Duration
}

func (e *Error) Error() string {
	switch e.Rule {
	case RuleActive:
		return fmt.Sprintf("ratelimit: %d submission(s) for this assignment are still queued or running", e.Limit)
	case RuleInterval:
		return fmt.Sprintf("ratelimit: wait %s before submitting again", e.RetryAfter.Round(time.Second))
	default:
		return fmt.Sprintf("ratelimit: at most %d submissions per assignment", e.Limit)
	}
}

// Check reports whether the student may submit to the assignment now. It returns nil, or an *Error.
//
// tx must be the transaction that goes on to insert the submission. Check first takes a lock, held
// until tx ends, on the pair (student, assignment); a second upload by the same student waits there
// and then sees the first one's row.
//
// Cancelled submissions do not count against MaxPerAssignment, so a wrong upload does not cost an
// attempt (US-14). They do count for MinInterval and never for MaxActive.
func (l Limits) Check(ctx context.Context, tx pgx.Tx, userID, assignmentID int64) error {
	if l.MaxActive <= 0 && l.MinInterval <= 0 && l.MaxPerAssignment <= 0 {
		return nil
	}
	if _, err := tx.Exec(ctx,
		`SELECT pg_advisory_xact_lock(hashtextextended('ratelimit:' || $1::bigint::text || ':' || $2::bigint::text, 0))`,
		userID, assignmentID); err != nil {
		return fmt.Errorf("ratelimit: lock: %w", err)
	}

	var active, counted int
	var sinceLast *float64 // seconds; NULL when there is no earlier submission
	err := tx.QueryRow(ctx, `
		SELECT count(*) FILTER (WHERE status IN ('QUEUED', 'RUNNING')),
		       count(*) FILTER (WHERE status <> 'CANCELLED'),
		       extract(epoch FROM now() - max(submitted_at))
		FROM submissions WHERE user_id = $1 AND assignment_id = $2`, userID, assignmentID,
	).Scan(&active, &counted, &sinceLast)
	if err != nil {
		return fmt.Errorf("ratelimit: count submissions: %w", err)
	}

	switch {
	case l.MaxActive > 0 && active >= l.MaxActive:
		return &Error{Rule: RuleActive, Limit: l.MaxActive}
	case l.MinInterval > 0 && sinceLast != nil:
		elapsed := time.Duration(*sinceLast * float64(time.Second))
		if elapsed < l.MinInterval {
			return &Error{Rule: RuleInterval, RetryAfter: l.MinInterval - elapsed}
		}
	}
	if l.MaxPerAssignment > 0 && counted >= l.MaxPerAssignment {
		return &Error{Rule: RuleQuota, Limit: l.MaxPerAssignment}
	}
	return nil
}
