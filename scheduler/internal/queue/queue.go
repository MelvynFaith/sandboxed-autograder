// Package queue defines the boundary between the code that accepts submissions and the worker pool
// that grades them. Handlers only enqueue and cancel; workers only dequeue.
//
// The first implementation, Memory, is in-process. It sits behind JobQueue so a distributed queue
// can replace it without touching the handlers (SRS 3.2.2).
package queue

import (
	"context"
	"errors"
)

// ErrDuplicate is returned by Enqueue when the submission is already waiting in the queue. Grading
// one submission twice at once would write its results twice.
var ErrDuplicate = errors.New("queue: submission already queued")

// Job names one submission to grade.
//
// UserID travels with the job so an implementation can schedule per student (FR-13) without a
// database lookup.
type Job struct {
	SubmissionID int64
	UserID       int64
}

// JobQueue holds submissions that are accepted and not yet picked up by a worker.
//
// The database is the source of truth for submission status: a queue only decides order. After a
// restart the queue is rebuilt from the submissions still marked QUEUED.
type JobQueue interface {
	// Enqueue adds job. The caller has already stored the submission as QUEUED. It does not wait
	// for a free worker, and returns ErrDuplicate if the submission is already queued. A submission
	// a worker has taken may be enqueued again, which is how a regrade works.
	Enqueue(ctx context.Context, job Job) error

	// Dequeue blocks until a job is available or ctx is done. Jobs are served round-robin across
	// UserID, and in Enqueue order within one user, so one student cannot starve the others (FR-13).
	// It returns a job or an error, never both: if ctx ends just as a job is handed over, the job
	// is returned, because the submission is already marked QUEUED and nobody else would pick it up.
	Dequeue(ctx context.Context) (Job, error)

	// Remove withdraws a job that no worker has taken, for a cancelled submission (FR-16). It
	// reports whether the job was still queued; false means a worker already has it, and a RUNNING
	// submission cannot be cancelled.
	Remove(ctx context.Context, submissionID int64) (bool, error)
}
