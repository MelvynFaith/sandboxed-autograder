package stream_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/auth"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/progress"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/store"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/stream"
)

const patience = 10 * time.Second

// fakeStates answers Progress from a map; a missing submission is store.ErrNotFound.
type fakeStates struct {
	mu     sync.Mutex
	states map[int64]progress.Event
	err    error
}

func (f *fakeStates) Progress(_ context.Context, id int64) (progress.Event, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.err != nil {
		return progress.Event{}, f.err
	}
	e, ok := f.states[id]
	if !ok {
		return progress.Event{}, store.ErrNotFound
	}
	return e, nil
}

type rig struct {
	t       *testing.T
	srv     *httptest.Server
	tickets *auth.TicketStore
	states  *fakeStates
	hub     *progress.Hub
	active  atomic.Int32 // handler calls that have not returned
}

func newRig(t *testing.T) *rig {
	t.Helper()
	tickets, err := auth.NewTicketStore(auth.MaxTicketTTL)
	if err != nil {
		t.Fatal(err)
	}
	r := &rig{t: t, tickets: tickets, states: &fakeStates{states: map[int64]progress.Event{}}, hub: progress.NewHub()}
	mux := http.NewServeMux()
	h := stream.New(tickets, r.states, r.hub)
	mux.HandleFunc("GET /ws/v1/submissions/{id}", func(w http.ResponseWriter, req *http.Request) {
		r.active.Add(1)
		defer r.active.Add(-1)
		h.ServeHTTP(w, req)
	})
	r.srv = httptest.NewServer(mux)
	t.Cleanup(r.srv.Close)
	return r
}

func (r *rig) set(e progress.Event) {
	r.states.mu.Lock()
	defer r.states.mu.Unlock()
	r.states.states[e.SubmissionID] = e
}

func (r *rig) ticket(user, submission int64) string {
	r.t.Helper()
	v, _, err := r.tickets.Issue(user, submission)
	if err != nil {
		r.t.Fatal(err)
	}
	return v
}

func (r *rig) url(path string) string { return "ws" + strings.TrimPrefix(r.srv.URL, "http") + path }

// dial connects for the submission with the ticket; it fails the test unless the handshake succeeds.
func (r *rig) dial(submission string, ticket string) *websocket.Conn {
	r.t.Helper()
	conn, resp, err := websocket.DefaultDialer.Dial(r.url("/ws/v1/submissions/"+submission+"?ticket="+ticket), nil)
	if err != nil {
		code := 0
		if resp != nil {
			code = resp.StatusCode
		}
		r.t.Fatalf("dial: %v (HTTP %d)", err, code)
	}
	r.t.Cleanup(func() { conn.Close() })
	return conn
}

// refused dials and returns the HTTP status of the refused handshake.
func (r *rig) refused(path string) int {
	r.t.Helper()
	conn, resp, err := websocket.DefaultDialer.Dial(r.url(path), nil)
	if err == nil {
		conn.Close()
		r.t.Fatalf("handshake to %s succeeded", path)
	}
	if resp == nil {
		r.t.Fatalf("dial %s: %v", path, err)
	}
	return resp.StatusCode
}

func ev(id int64, status string, current, total int) progress.Event {
	return progress.Event{SubmissionID: id, Status: status, CurrentTest: current, TotalTests: total}
}

// frame reads one frame and checks it is exactly the dashboard's three fields.
func frame(t *testing.T, conn *websocket.Conn) progress.Event {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(patience))
	_, data, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	var fields map[string]any
	if err := json.Unmarshal(data, &fields); err != nil || len(fields) != 3 {
		t.Fatalf("frame %s: want status, current_test and total_tests only", data)
	}
	var e progress.Event
	if err := json.Unmarshal(data, &e); err != nil {
		t.Fatal(err)
	}
	return e
}

// closedWith reads until the server closes and checks the close code.
func closedWith(t *testing.T, conn *websocket.Conn, code int) {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(patience))
	_, data, err := conn.ReadMessage()
	var ce *websocket.CloseError
	if !errors.As(err, &ce) || ce.Code != code {
		t.Fatalf("got %q, %v; want close %d", data, err, code)
	}
}

func same(t *testing.T, got progress.Event, want progress.Event) {
	t.Helper()
	got.SubmissionID = want.SubmissionID // not on the wire
	if got != want {
		t.Fatalf("frame %+v, want %+v", got, want)
	}
}

func TestStreamsAGradingRunToTheEnd(t *testing.T) {
	r := newRig(t)
	r.set(ev(7, progress.StatusQueued, 0, 2))
	conn := r.dial("7", r.ticket(1, 7))

	same(t, frame(t, conn), ev(7, progress.StatusQueued, 0, 2))
	for _, e := range []progress.Event{
		ev(7, progress.StatusRunning, 0, 2),
		ev(7, progress.StatusRunning, 1, 2),
		ev(7, progress.StatusRunning, 2, 2),
		ev(7, progress.StatusCompleted, 2, 2),
	} {
		r.hub.Publish(e)
		same(t, frame(t, conn), e)
	}
	closedWith(t, conn, websocket.CloseNormalClosure)
}

func TestAFinishedSubmissionGetsOneFrame(t *testing.T) {
	for _, status := range []string{progress.StatusCompleted, progress.StatusFailed, progress.StatusCancelled} {
		t.Run(status, func(t *testing.T) {
			r := newRig(t)
			r.set(ev(7, status, 0, 3))
			conn := r.dial("7", r.ticket(1, 7))
			same(t, frame(t, conn), ev(7, status, 0, 3))
			closedWith(t, conn, websocket.CloseNormalClosure)
		})
	}
}

// A connection that opens half-way through sees where grading is, not the database's test 0.
func TestJoiningMidRunStartsFromTheLatestEvent(t *testing.T) {
	r := newRig(t)
	r.set(ev(7, progress.StatusRunning, 0, 3))
	r.hub.Publish(ev(7, progress.StatusRunning, 2, 3))
	conn := r.dial("7", r.ticket(1, 7))
	same(t, frame(t, conn), ev(7, progress.StatusRunning, 2, 3))
}

func TestOtherSubmissionsEventsAreNotSent(t *testing.T) {
	r := newRig(t)
	r.set(ev(7, progress.StatusQueued, 0, 2))
	conn := r.dial("7", r.ticket(1, 7))
	frame(t, conn)
	r.hub.Publish(ev(8, progress.StatusCompleted, 2, 2))
	r.hub.Publish(ev(7, progress.StatusFailed, 0, 2))
	same(t, frame(t, conn), ev(7, progress.StatusFailed, 0, 2))
}

func TestShutdownSaysGoingAway(t *testing.T) {
	r := newRig(t)
	r.set(ev(7, progress.StatusQueued, 0, 2))
	conn := r.dial("7", r.ticket(1, 7))
	frame(t, conn)
	r.hub.Close()
	closedWith(t, conn, websocket.CloseGoingAway)
}

func TestRefusedHandshakes(t *testing.T) {
	r := newRig(t)
	r.set(ev(7, progress.StatusQueued, 0, 2))

	if code := r.refused("/ws/v1/submissions/7?ticket=forged"); code != http.StatusUnauthorized {
		t.Errorf("forged ticket: HTTP %d, want 401", code)
	}
	if code := r.refused("/ws/v1/submissions/7"); code != http.StatusUnauthorized {
		t.Errorf("no ticket: HTTP %d, want 401", code)
	}

	// A ticket is good once, whatever happens to the handshake.
	used := r.ticket(1, 7)
	r.dial("7", used).Close()
	if code := r.refused("/ws/v1/submissions/7?ticket=" + used); code != http.StatusUnauthorized {
		t.Errorf("reused ticket: HTTP %d, want 401", code)
	}

	// NFR-10: a ticket for one submission does not open another; the answer is 404, not 403.
	r.set(ev(8, progress.StatusQueued, 0, 2))
	if code := r.refused("/ws/v1/submissions/8?ticket=" + r.ticket(1, 7)); code != http.StatusNotFound {
		t.Errorf("ticket for another submission: HTTP %d, want 404", code)
	}
	if code := r.refused("/ws/v1/submissions/x?ticket=" + r.ticket(1, 7)); code != http.StatusNotFound {
		t.Errorf("bad id: HTTP %d, want 404", code)
	}
	if code := r.refused("/ws/v1/submissions/9?ticket=" + r.ticket(1, 9)); code != http.StatusNotFound {
		t.Errorf("deleted submission: HTTP %d, want 404", code)
	}

	r.states.mu.Lock()
	r.states.err = errors.New("database down")
	r.states.mu.Unlock()
	if code := r.refused("/ws/v1/submissions/7?ticket=" + r.ticket(1, 7)); code != http.StatusInternalServerError {
		t.Errorf("database error: HTTP %d, want 500", code)
	}
}

// A plain GET is not a handshake. It is refused before the ticket is looked at, so the ticket still
// works afterwards.
func TestPlainRequestDoesNotUseTheTicket(t *testing.T) {
	r := newRig(t)
	r.set(ev(7, progress.StatusQueued, 0, 2))
	ticket := r.ticket(1, 7)
	resp, err := http.Get(r.srv.URL + "/ws/v1/submissions/7?ticket=" + ticket)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("plain GET: HTTP %d, want 400", resp.StatusCode)
	}
	same(t, frame(t, r.dial("7", ticket)), ev(7, progress.StatusQueued, 0, 2))
}

func TestClientLeavingEndsTheStream(t *testing.T) {
	r := newRig(t)
	r.set(ev(7, progress.StatusQueued, 0, 2))
	conn := r.dial("7", r.ticket(1, 7))
	frame(t, conn)
	_ = conn.WriteControl(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""), time.Now().Add(time.Second))
	conn.Close()

	// The handler returns, closing its subscription, although no event ever ends the run.
	for deadline := time.Now().Add(patience); r.active.Load() != 0; time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the handler is still running after the client left")
		}
	}
	r.hub.Publish(ev(7, progress.StatusRunning, 1, 2))
}
