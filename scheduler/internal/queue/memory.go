package queue

import (
	"container/list"
	"context"
	"slices"
	"sync"
)

// Memory is the in-process JobQueue. Its size is bounded by the submission rate limits (FR-14), not
// by the queue itself.
//
// Scheduling is round-robin over students. Each student with queued jobs holds one place in a ring;
// Dequeue serves the student at the front, takes that student's oldest job, and sends the student
// to the back of the ring if more jobs remain. A student who joins is placed at the back. Every
// operation is O(1) in the number of queued jobs; the list of waiting Dequeue calls is as long as
// the number of idle workers, and is shifted when one is served or gives up.
type Memory struct {
	mu       sync.Mutex
	students map[int64]*student      // only students that have queued jobs
	ring     list.List               // of *student, in service order
	queued   map[int64]*list.Element // submission ID to its element in student.jobs
	waiters  []chan Job              // Dequeue calls that found the queue empty, oldest first
}

type student struct {
	id   int64
	jobs list.List     // of Job, oldest first
	turn *list.Element // this student's place in Memory.ring
}

var _ JobQueue = (*Memory)(nil)

// NewMemory returns an empty queue.
func NewMemory() *Memory {
	return &Memory{
		students: make(map[int64]*student),
		queued:   make(map[int64]*list.Element),
	}
}

// Enqueue implements JobQueue.
func (q *Memory) Enqueue(ctx context.Context, job Job) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.enqueueLocked(job)
}

// enqueueLocked is Enqueue without the context check. The caller holds q.mu.
func (q *Memory) enqueueLocked(job Job) error {
	if _, ok := q.queued[job.SubmissionID]; ok {
		return ErrDuplicate
	}
	// A Dequeue waits only while the queue is empty, so a waiting worker is owed exactly this job.
	// The channel holds one job and the worker is gone from the list, so the send cannot block.
	if len(q.waiters) > 0 {
		w := q.waiters[0]
		q.waiters = slices.Delete(q.waiters, 0, 1)
		w <- job
		return nil
	}

	s := q.students[job.UserID]
	if s == nil {
		s = &student{id: job.UserID}
		s.turn = q.ring.PushBack(s)
		q.students[job.UserID] = s
	}
	q.queued[job.SubmissionID] = s.jobs.PushBack(job)
	return nil
}

// Dequeue implements JobQueue.
func (q *Memory) Dequeue(ctx context.Context) (Job, error) {
	if err := ctx.Err(); err != nil {
		return Job{}, err
	}
	q.mu.Lock()
	if front := q.ring.Front(); front != nil {
		job := q.take(front.Value.(*student))
		q.mu.Unlock()
		return job, nil
	}
	w := make(chan Job, 1)
	q.waiters = append(q.waiters, w)
	q.mu.Unlock()

	select {
	case job := <-w:
		return job, nil
	case <-ctx.Done():
	}

	q.mu.Lock()
	defer q.mu.Unlock()
	if i := slices.Index(q.waiters, w); i >= 0 {
		q.waiters = slices.Delete(q.waiters, i, i+1)
		return Job{}, ctx.Err()
	}
	// Enqueue removed w from the list before ctx was done, and it sent the job while holding the
	// lock we now hold. Return the job: dropping it would lose a submission that is marked QUEUED.
	return <-w, nil
}

// Remove implements JobQueue.
func (q *Memory) Remove(ctx context.Context, submissionID int64) (bool, error) {
	if err := ctx.Err(); err != nil {
		return false, err
	}
	q.mu.Lock()
	defer q.mu.Unlock()

	e, ok := q.queued[submissionID]
	if !ok {
		return false, nil
	}
	s := q.students[e.Value.(Job).UserID]
	s.jobs.Remove(e)
	delete(q.queued, submissionID)
	if s.jobs.Len() == 0 {
		q.retire(s)
	}
	return true, nil
}

// take removes the oldest job of s, which must have one, and advances the ring. The caller holds
// q.mu.
func (q *Memory) take(s *student) Job {
	job := s.jobs.Remove(s.jobs.Front()).(Job)
	delete(q.queued, job.SubmissionID)
	if s.jobs.Len() == 0 {
		q.retire(s)
	} else {
		q.ring.MoveToBack(s.turn)
	}
	return job
}

// retire drops a student that has no queued jobs left. The caller holds q.mu.
func (q *Memory) retire(s *student) {
	q.ring.Remove(s.turn)
	delete(q.students, s.id)
}
