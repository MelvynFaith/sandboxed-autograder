package health

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
)

type fakeDB struct {
	err   error
	pings atomic.Int32
}

func (d *fakeDB) Ping(ctx context.Context) error {
	d.pings.Add(1)
	if d.err != nil {
		return d.err
	}
	return ctx.Err()
}

type fakeWorkers struct{ alive, configured int }

func (w fakeWorkers) Workers() (int, int) { return w.alive, w.configured }

type fakeQueue int

func (q fakeQueue) Len() int { return int(q) }

// get serves one GET /healthz through a mux, the way the binary mounts it.
func get(t *testing.T, c *Checker, method string) (int, string) {
	t.Helper()
	mux := http.NewServeMux()
	mux.Handle("GET /healthz", c)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(method, "/healthz", nil))
	if ct := rec.Header().Get("Content-Type"); rec.Code != http.StatusMethodNotAllowed && ct != "application/json" {
		t.Errorf("Content-Type %q", ct)
	}
	return rec.Code, rec.Body.String()
}

func decode(t *testing.T, body string) Report {
	t.Helper()
	var r Report
	if err := json.Unmarshal([]byte(body), &r); err != nil {
		t.Fatalf("body %q: %v", body, err)
	}
	return r
}

func TestHealthy(t *testing.T) {
	code, body := get(t, New(&fakeDB{}, fakeWorkers{4, 4}, fakeQueue(7)), http.MethodGet)
	want := Report{Status: StatusOK, Database: DatabaseOK, Workers: WorkerCount{4, 4}, QueueDepth: 7}
	if got := decode(t, body); code != http.StatusOK || got != want {
		t.Fatalf("got %d %+v, want 200 %+v", code, got, want)
	}
}

func TestAPIOnlyProcessDependsOnTheDatabaseAlone(t *testing.T) {
	code, body := get(t, New(&fakeDB{}, nil, nil), http.MethodGet)
	want := Report{Status: StatusOK, Database: DatabaseOK}
	if got := decode(t, body); code != http.StatusOK || got != want {
		t.Fatalf("got %d %+v, want 200 %+v", code, got, want)
	}
}

// SRS 11.6 (NFR-09): with the database down /healthz must report unhealthy, not stay at 200. The
// error text, which can name the host and the user, stays out of the response.
func TestDatabaseDownIsUnhealthy(t *testing.T) {
	db := &fakeDB{err: errors.New("dial tcp db.internal:5432: user=grader password=hunter2")}
	code, body := get(t, New(db, fakeWorkers{4, 4}, fakeQueue(0)), http.MethodGet)
	got := decode(t, body)
	if code != http.StatusServiceUnavailable || got.Status != StatusUnhealthy || got.Database != DatabaseUnreached {
		t.Fatalf("got %d %+v, want 503 unhealthy/unreachable", code, got)
	}
	for _, leak := range []string{"hunter2", "db.internal", "grader"} {
		if strings.Contains(body, leak) {
			t.Errorf("body %q contains %q", body, leak)
		}
	}
}

func TestMissingWorkersAreUnhealthy(t *testing.T) {
	for _, w := range []fakeWorkers{{3, 4}, {0, 4}} {
		code, body := get(t, New(&fakeDB{}, w, fakeQueue(0)), http.MethodGet)
		if got := decode(t, body); code != http.StatusServiceUnavailable || got.Status != StatusUnhealthy {
			t.Errorf("%d of %d workers: got %d %+v, want 503 unhealthy", w.alive, w.configured, code, got)
		}
	}
}

func TestHeadAndOtherMethods(t *testing.T) {
	c := New(&fakeDB{}, nil, nil)
	// net/http drops the body of a HEAD response; the recorder does not, so only the code is checked.
	if code, _ := get(t, c, http.MethodHead); code != http.StatusOK {
		t.Errorf("HEAD: %d, want 200", code)
	}
	if code, _ := get(t, c, http.MethodPost); code != http.StatusMethodNotAllowed {
		t.Errorf("POST: %d, want 405", code)
	}
}

func TestOneCheckPerSecond(t *testing.T) {
	db := &fakeDB{}
	c := New(db, nil, nil)
	now := time.Unix(1000, 0)
	c.now = func() time.Time { return now }

	var wg sync.WaitGroup
	for range 50 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			get(t, c, http.MethodGet)
		}()
	}
	wg.Wait()
	if n := db.pings.Load(); n != 1 {
		t.Fatalf("%d pings for 50 requests within a second, want 1", n)
	}

	now = now.Add(cacheFor)
	get(t, c, http.MethodGet)
	if n := db.pings.Load(); n != 2 {
		t.Fatalf("%d pings after the cache expired, want 2", n)
	}
}

// A client that hangs up must not make the shared result say the database is unreachable.
func TestCancelledRequestStillPingsToTheEnd(t *testing.T) {
	c := New(&fakeDB{}, nil, nil)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if rep := c.check(ctx); rep.Database != DatabaseOK {
		t.Fatalf("database %q after the client hung up, want ok", rep.Database)
	}
}
