// Package queue defines the boundary between the code that accepts submissions and the worker pool
// that grades them. Handlers only enqueue and cancel; workers only dequeue.
//
// The first implementation is in-process (goroutines and channels). It sits behind JobQueue so a
// distributed queue can replace it without touching the handlers (SRS 3.2.2).
package queue

import "context"

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
	// for a free worker.
	Enqueue(ctx context.Context, job Job) error

	// Dequeue blocks until a job is available or ctx is done. Jobs are served round-robin across
	// UserID, and in Enqueue order within one user, so one student cannot starve the others (FR-13).
	Dequeue(ctx context.Context) (Job, error)

	// Remove withdraws a job that no worker has taken, for a cancelled submission (FR-16). It
	// reports whether the job was still queued; false means a worker already has it, and a RUNNING
	// submission cannot be cancelled.
	Remove(ctx context.Context, submissionID int64) (bool, error)
}
