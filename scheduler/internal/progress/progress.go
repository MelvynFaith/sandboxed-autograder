// Package progress carries grading progress from the worker pool to the WebSocket handler (FR-08):
// queued, running test n of total, and the final status.
//
// The worker publishes an Event at each step; every connection watching that submission receives it.
// An event describes the whole state, not a change, so a subscriber that falls behind only needs the
// latest one: each subscription holds one pending event, a newer event replaces an older one, and
// Publish never waits for a slow client.
package progress

import "sync"

// Submission.status values (SRS 7.1).
const (
	StatusQueued    = "QUEUED"
	StatusRunning   = "RUNNING"
	StatusCompleted = "COMPLETED"
	StatusFailed    = "FAILED"
	StatusCancelled = "CANCELLED"
)

// Event is the state of one submission, in the shape the dashboard reads
// (frontend/src/api/submissionStatus.js): CurrentTest is the test case being run, 1-based, or 0
// before the first; it never exceeds TotalTests.
type Event struct {
	SubmissionID int64  `json:"-"`
	Status       string `json:"status"`
	CurrentTest  int    `json:"current_test"`
	TotalTests   int    `json:"total_tests"`
}

// Terminal reports whether no further event can follow for this grading run. A regrade starts a new
// run with a new connection.
func (e Event) Terminal() bool {
	switch e.Status {
	case StatusCompleted, StatusFailed, StatusCancelled:
		return true
	}
	return false
}

// Hub routes events to subscriptions. The zero value is not usable; call NewHub. It is safe for
// concurrent use.
type Hub struct {
	mu     sync.Mutex
	subs   map[int64]map[*Subscription]struct{}
	last   map[int64]Event // latest event of each submission still being graded
	closed bool
}

// NewHub returns an empty hub.
func NewHub() *Hub {
	return &Hub{subs: make(map[int64]map[*Subscription]struct{}), last: make(map[int64]Event)}
}

// Subscription receives the events of one submission. Wait on Ready, then call Next.
type Subscription struct {
	hub   *Hub
	id    int64
	ready chan struct{} // holds a token while an event is pending
	done  chan struct{} // closed when the hub shuts down
	ev    Event         // pending event, guarded by hub.mu
	has   bool
}

// Subscribe starts watching a submission. If the submission is being graded, its latest event is
// pending at once, so a connection that opens half-way through sees where grading is.
func (h *Hub) Subscribe(submissionID int64) *Subscription {
	s := &Subscription{hub: h, id: submissionID, ready: make(chan struct{}, 1), done: make(chan struct{})}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		close(s.done)
		return s
	}
	set := h.subs[submissionID]
	if set == nil {
		set = make(map[*Subscription]struct{})
		h.subs[submissionID] = set
	}
	set[s] = struct{}{}
	if e, ok := h.last[submissionID]; ok {
		s.deliverLocked(e)
	}
	return s
}

// Publish delivers e to every subscription of its submission without blocking. The latest event of a
// submission is kept for later subscribers until a terminal event ends the run.
func (h *Hub) Publish(e Event) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	if e.Terminal() {
		delete(h.last, e.SubmissionID)
	} else {
		h.last[e.SubmissionID] = e
	}
	for s := range h.subs[e.SubmissionID] {
		s.deliverLocked(e)
	}
}

// Close ends every subscription, present and future: their Done channels close. Use it on shutdown,
// so that connections are told the server is going away instead of being cut.
func (h *Hub) Close() {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	h.closed = true
	for _, set := range h.subs {
		for s := range set {
			close(s.done)
		}
	}
	h.subs, h.last = nil, nil
}

// deliverLocked makes e the pending event. The caller holds hub.mu.
func (s *Subscription) deliverLocked(e Event) {
	s.ev, s.has = e, true
	select {
	case s.ready <- struct{}{}:
	default: // a token is already there; Next will return e
	}
}

// Ready receives a value when an event is pending.
func (s *Subscription) Ready() <-chan struct{} { return s.ready }

// Done is closed when the hub shuts down.
func (s *Subscription) Done() <-chan struct{} { return s.done }

// Next takes the pending event. ok is false if there is none, which happens when a token was left
// over from an event an earlier Next already returned.
func (s *Subscription) Next() (e Event, ok bool) {
	s.hub.mu.Lock()
	defer s.hub.mu.Unlock()
	e, ok = s.ev, s.has
	s.has = false
	return e, ok
}

// Close stops the subscription. It is safe to call more than once and after the hub has closed.
func (s *Subscription) Close() {
	h := s.hub
	h.mu.Lock()
	defer h.mu.Unlock()
	set := h.subs[s.id]
	if _, ok := set[s]; !ok {
		return
	}
	delete(set, s)
	if len(set) == 0 {
		delete(h.subs, s.id)
	}
}
