package queue_test

import (
	"context"
	"errors"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/queue"
)

// patience bounds every call that is expected to succeed, so a bug fails a test instead of hanging it.
const patience = 10 * time.Second

func ctx(t *testing.T) context.Context {
	t.Helper()
	c, cancel := context.WithTimeout(context.Background(), patience)
	t.Cleanup(cancel)
	return c
}

func enqueue(t *testing.T, q queue.JobQueue, submission, user int64) {
	t.Helper()
	if err := q.Enqueue(ctx(t), queue.Job{SubmissionID: submission, UserID: user}); err != nil {
		t.Fatalf("enqueue %d: %v", submission, err)
	}
}

func dequeue(t *testing.T, q queue.JobQueue) queue.Job {
	t.Helper()
	job, err := q.Dequeue(ctx(t))
	if err != nil {
		t.Fatalf("dequeue: %v", err)
	}
	return job
}

// waitForWaiters returns once n Dequeue calls are blocked on the queue.
func waitForWaiters(t *testing.T, q *queue.Memory, n int) {
	t.Helper()
	deadline := time.Now().Add(patience)
	for q.Waiting() != n {
		if time.Now().After(deadline) {
			t.Fatalf("%d Dequeue calls waiting, want %d", q.Waiting(), n)
		}
		time.Sleep(time.Millisecond)
	}
}

// dequeueIDs takes n jobs and returns their submission IDs in service order.
func dequeueIDs(t *testing.T, q queue.JobQueue, n int) []int64 {
	t.Helper()
	ids := make([]int64, 0, n)
	for range n {
		ids = append(ids, dequeue(t, q).SubmissionID)
	}
	return ids
}

func TestRoundRobinAcrossStudentsFIFOWithinOne(t *testing.T) {
	q := queue.NewMemory()
	// Submission IDs read as <student><n>: 11, 12, 13 belong to student 1, 21 to student 2, and so on.
	for _, j := range []struct{ id, user int64 }{{11, 1}, {12, 1}, {13, 1}, {21, 2}, {31, 3}, {32, 3}} {
		enqueue(t, q, j.id, j.user)
	}
	want := []int64{11, 21, 31, 12, 32, 13}
	if got := dequeueIDs(t, q, len(want)); !slices.Equal(got, want) {
		t.Fatalf("service order %v, want %v", got, want)
	}
}

func TestHeavyStudentDoesNotStarveOthers(t *testing.T) {
	q := queue.NewMemory()
	for id := int64(1); id <= 100; id++ {
		enqueue(t, q, id, 1)
	}
	enqueue(t, q, 1000, 2)
	if got := dequeueIDs(t, q, 2); got[1] != 1000 {
		t.Fatalf("the second student was served as %v, want within the first two", got)
	}
}

func TestStudentWhoReturnsJoinsTheBackOfTheRing(t *testing.T) {
	q := queue.NewMemory()
	enqueue(t, q, 11, 1)
	enqueue(t, q, 21, 2)
	if got := dequeue(t, q).SubmissionID; got != 11 {
		t.Fatalf("first job %d, want 11", got)
	}
	// Student 1 had nothing left, left the ring, and now rejoins behind student 2.
	enqueue(t, q, 12, 1)
	if got := dequeueIDs(t, q, 2); !slices.Equal(got, []int64{21, 12}) {
		t.Fatalf("service order %v, want [21 12]", got)
	}
}

func TestDequeueWaitsForEnqueue(t *testing.T) {
	q := queue.NewMemory()
	type result struct {
		job queue.Job
		err error
	}
	got := make(chan result, 1)
	go func() {
		job, err := q.Dequeue(ctx(t))
		got <- result{job, err}
	}()

	waitForWaiters(t, q, 1)
	enqueue(t, q, 7, 1)
	if r := <-got; r.err != nil || r.job.SubmissionID != 7 {
		t.Fatalf("Dequeue = %+v, %v; want submission 7", r.job, r.err)
	}
}

func TestDequeueOnEmptyQueueEndsWithContext(t *testing.T) {
	q := queue.NewMemory()
	c, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if _, err := q.Dequeue(c); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("got %v, want context.DeadlineExceeded", err)
	}
}

func TestCalledWithDoneContextDoesNothing(t *testing.T) {
	q := queue.NewMemory()
	enqueue(t, q, 1, 1)
	done, cancel := context.WithCancel(context.Background())
	cancel()

	if err := q.Enqueue(done, queue.Job{SubmissionID: 2, UserID: 1}); !errors.Is(err, context.Canceled) {
		t.Errorf("Enqueue: got %v, want context.Canceled", err)
	}
	if _, err := q.Dequeue(done); !errors.Is(err, context.Canceled) {
		t.Errorf("Dequeue: got %v, want context.Canceled", err)
	}
	if _, err := q.Remove(done, 1); !errors.Is(err, context.Canceled) {
		t.Errorf("Remove: got %v, want context.Canceled", err)
	}
	// Job 1 is still there and job 2 was never added.
	if got := dequeue(t, q).SubmissionID; got != 1 {
		t.Fatalf("got submission %d, want 1", got)
	}
	if ok, err := q.Remove(ctx(t), 2); ok || err != nil {
		t.Fatalf("Remove(2) = %v, %v; want false, nil", ok, err)
	}
}

// A Dequeue that gave up must not stay in the list of waiters: Enqueue would hand it the next job,
// and nobody would ever read it.
func TestCancelledDequeueDoesNotSwallowTheNextJob(t *testing.T) {
	q := queue.NewMemory()
	c, cancel := context.WithCancel(context.Background())
	errc := make(chan error)
	go func() { _, err := q.Dequeue(c); errc <- err }()

	waitForWaiters(t, q, 1)
	cancel()
	if err := <-errc; !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v, want context.Canceled", err)
	}
	if n := q.Waiting(); n != 0 {
		t.Fatalf("%d cancelled Dequeue calls are still listed as waiting", n)
	}
	enqueue(t, q, 5, 1)
	if got := dequeue(t, q).SubmissionID; got != 5 {
		t.Fatalf("got submission %d, want 5", got)
	}
}

func TestEnqueueRejectsAJobAlreadyQueued(t *testing.T) {
	q := queue.NewMemory()
	enqueue(t, q, 1, 1)
	if err := q.Enqueue(ctx(t), queue.Job{SubmissionID: 1, UserID: 1}); !errors.Is(err, queue.ErrDuplicate) {
		t.Fatalf("got %v, want ErrDuplicate", err)
	}
	// Once a worker has taken it, the same submission can be queued again (regrade).
	dequeue(t, q)
	enqueue(t, q, 1, 1)
}

func TestRemove(t *testing.T) {
	q := queue.NewMemory()
	enqueue(t, q, 11, 1)
	enqueue(t, q, 12, 1)
	enqueue(t, q, 21, 2)

	if ok, err := q.Remove(ctx(t), 11); !ok || err != nil {
		t.Fatalf("Remove(11) = %v, %v; want true, nil", ok, err)
	}
	if ok, _ := q.Remove(ctx(t), 11); ok {
		t.Fatal("Remove(11) succeeded twice")
	}
	if ok, _ := q.Remove(ctx(t), 99); ok {
		t.Fatal("Remove(99) succeeded for a submission that was never queued")
	}

	taken := dequeue(t, q) // student 1 is first in the ring, with 12 left
	if taken.SubmissionID != 12 {
		t.Fatalf("got submission %d, want 12", taken.SubmissionID)
	}
	if ok, _ := q.Remove(ctx(t), 12); ok {
		t.Fatal("Remove succeeded for a job a worker already has")
	}

	// Removing a student's last job takes the student out of the ring.
	if ok, _ := q.Remove(ctx(t), 21); !ok {
		t.Fatal("Remove(21) failed")
	}
	c, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	if job, err := q.Dequeue(c); err == nil {
		t.Fatalf("queue should be empty, got %+v", job)
	}
}

// Exactly-once delivery under contention. The consumers use timeouts of a few microseconds so that
// cancellation keeps racing with hand-off, which is where a job could be lost or delivered twice.
func TestConcurrentDeliveryIsExactlyOnce(t *testing.T) {
	const (
		producers   = 8
		perProducer = 500
		consumers   = 4
		total       = producers * perProducer
	)
	q := queue.NewMemory()
	var (
		delivered atomic.Int64
		counts    [total + 1]atomic.Int32
		done      = make(chan struct{})
		produce   sync.WaitGroup
		consume   sync.WaitGroup
	)

	for p := range producers {
		produce.Add(1)
		go func() {
			defer produce.Done()
			for i := range perProducer {
				job := queue.Job{SubmissionID: int64(p*perProducer + i + 1), UserID: int64(p%3 + 1)}
				if err := q.Enqueue(context.Background(), job); err != nil {
					t.Errorf("enqueue %d: %v", job.SubmissionID, err)
					return
				}
			}
		}()
	}
	for range consumers {
		consume.Add(1)
		go func() {
			defer consume.Done()
			for {
				c, cancel := context.WithTimeout(context.Background(), 20*time.Microsecond)
				job, err := q.Dequeue(c)
				cancel()
				if err == nil {
					counts[job.SubmissionID].Add(1)
					if delivered.Add(1) == total {
						close(done)
					}
				}
				select {
				case <-done:
					return
				default:
				}
			}
		}()
	}

	select {
	case <-done:
	case <-time.After(60 * time.Second):
		t.Fatalf("delivered %d of %d jobs", delivered.Load(), total)
	}
	produce.Wait()
	consume.Wait()
	for id := 1; id <= total; id++ {
		if n := counts[id].Load(); n != 1 {
			t.Fatalf("submission %d delivered %d times", id, n)
		}
	}
}
