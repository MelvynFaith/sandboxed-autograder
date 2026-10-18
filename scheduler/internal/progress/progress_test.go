package progress_test

import (
	"sync"
	"testing"
	"time"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/progress"
)

const patience = 10 * time.Second

func running(id int64, current, total int) progress.Event {
	return progress.Event{SubmissionID: id, Status: progress.StatusRunning, CurrentTest: current, TotalTests: total}
}

// next waits for the pending event of s.
func next(t *testing.T, s *progress.Subscription) progress.Event {
	t.Helper()
	for {
		select {
		case <-s.Ready():
			if e, ok := s.Next(); ok {
				return e
			}
		case <-time.After(patience):
			t.Fatal("no event")
		}
	}
}

func pending(s *progress.Subscription) bool {
	select {
	case <-s.Ready():
		_, ok := s.Next()
		return ok
	default:
		return false
	}
}

func TestPublishReachesEverySubscriberOfThatSubmission(t *testing.T) {
	h := progress.NewHub()
	a, b, other := h.Subscribe(1), h.Subscribe(1), h.Subscribe(2)
	h.Publish(running(1, 1, 3))
	for _, s := range []*progress.Subscription{a, b} {
		if e := next(t, s); e != running(1, 1, 3) {
			t.Errorf("got %+v", e)
		}
	}
	if pending(other) {
		t.Error("a subscriber of another submission got the event")
	}
}

// A slow subscriber only ever needs the latest state: Publish replaces the pending event and never
// blocks.
func TestASlowSubscriberGetsTheLatestEvent(t *testing.T) {
	h := progress.NewHub()
	s := h.Subscribe(1)
	for i := 1; i <= 1000; i++ {
		h.Publish(running(1, i%5, 5))
	}
	h.Publish(progress.Event{SubmissionID: 1, Status: progress.StatusCompleted, CurrentTest: 5, TotalTests: 5})
	if e := next(t, s); e.Status != progress.StatusCompleted {
		t.Fatalf("got %+v, want the COMPLETED event", e)
	}
	if pending(s) {
		t.Fatal("an older event was still pending")
	}
}

func TestALateSubscriberSeesWhereGradingIs(t *testing.T) {
	h := progress.NewHub()
	h.Publish(running(1, 2, 4))
	if e := next(t, h.Subscribe(1)); e != running(1, 2, 4) {
		t.Fatalf("got %+v", e)
	}

	// Once the run has ended there is nothing to replay; the database has the final state.
	h.Publish(progress.Event{SubmissionID: 1, Status: progress.StatusFailed})
	if pending(h.Subscribe(1)) {
		t.Fatal("a finished run was replayed")
	}
}

func TestClosedSubscriptionGetsNothing(t *testing.T) {
	h := progress.NewHub()
	s := h.Subscribe(1)
	s.Close()
	s.Close()
	h.Publish(running(1, 1, 1))
	if pending(s) {
		t.Fatal("a closed subscription got an event")
	}
}

func TestHubCloseEndsSubscriptions(t *testing.T) {
	h := progress.NewHub()
	s := h.Subscribe(1)
	h.Close()
	h.Close()
	select {
	case <-s.Done():
	default:
		t.Fatal("Done is open after the hub closed")
	}
	select {
	case <-h.Subscribe(1).Done():
	default:
		t.Fatal("a subscription made after the hub closed is not done")
	}
	h.Publish(running(1, 1, 1))
	s.Close()
	if pending(s) {
		t.Fatal("an event arrived after the hub closed")
	}
}

func TestTerminal(t *testing.T) {
	for status, want := range map[string]bool{
		progress.StatusQueued: false, progress.StatusRunning: false,
		progress.StatusCompleted: true, progress.StatusFailed: true, progress.StatusCancelled: true,
	} {
		if got := (progress.Event{Status: status}).Terminal(); got != want {
			t.Errorf("%s: Terminal() = %t", status, got)
		}
	}
}

func TestConcurrentUse(t *testing.T) {
	h := progress.NewHub()
	var wg sync.WaitGroup
	for id := int64(1); id <= 8; id++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			for i := range 200 {
				h.Publish(running(id, i%3, 3))
			}
			h.Publish(progress.Event{SubmissionID: id, Status: progress.StatusCompleted, CurrentTest: 3, TotalTests: 3})
		}()
		go func() {
			defer wg.Done()
			for range 50 {
				s := h.Subscribe(id)
				pending(s)
				s.Close()
			}
		}()
	}
	wg.Wait()
	h.Close()
}
