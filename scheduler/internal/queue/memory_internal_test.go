package queue

import (
	"context"
	"testing"
	"time"
)

// A job handed to a Dequeue whose context has just ended must still be returned. The ordering that
// matters is rare under load: Enqueue has to take the lock between the Dequeue noticing the
// cancellation and the Dequeue taking the lock itself. The test forces it by holding the lock.
func TestJobHandedToACancelledDequeueIsNotLost(t *testing.T) {
	type result struct {
		job Job
		err error
	}
	for range 20 {
		q := NewMemory()
		c, cancel := context.WithCancel(context.Background())
		got := make(chan result, 1)
		go func() {
			job, err := q.Dequeue(c)
			got <- result{job, err}
		}()
		for deadline := time.Now().Add(10 * time.Second); q.Waiting() != 1; time.Sleep(time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatal("Dequeue never started waiting")
			}
		}

		q.mu.Lock()
		cancel()
		// Let the Dequeue see the cancellation and block on q.mu. If it has not got that far yet,
		// the run is merely less strict; it can never fail wrongly.
		time.Sleep(5 * time.Millisecond)
		err := q.enqueueLocked(Job{SubmissionID: 1, UserID: 1})
		q.mu.Unlock()
		if err != nil {
			t.Fatalf("enqueue: %v", err)
		}

		select {
		case r := <-got:
			if r.err != nil || r.job.SubmissionID != 1 {
				t.Fatalf("Dequeue = %+v, %v; want the handed-over job 1 and no error", r.job, r.err)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("Dequeue did not return")
		}
	}
}
