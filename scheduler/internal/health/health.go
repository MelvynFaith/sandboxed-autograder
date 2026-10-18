// Package health serves /healthz (NFR-09): whether the database answers, how many grading workers
// run, and how many submissions wait in the queue. systemd and Nginx poll it, so that a process that
// is alive but cannot grade, because the database is unreachable or the worker pool is gone, is told
// apart from a dead one and restarted (SRS 13.2).
//
// The endpoint is outside /api/v1 and unauthenticated, so the response carries aggregate values only,
// never an error message, a host name or anything about a user (SRS 6.3.1).
package health

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"sync"
	"time"
)

const (
	// pingTimeout bounds the database check. An answer slower than this counts as unreachable.
	pingTimeout = 2 * time.Second

	// cacheFor is how long one check answers every request, so that polling /healthz in a tight
	// loop cannot take database connections away from the API.
	cacheFor = time.Second
)

// Values of Report.Status and Report.Database.
const (
	StatusOK          = "ok"
	StatusUnhealthy   = "unhealthy"
	DatabaseOK        = "ok"
	DatabaseUnreached = "unreachable"
)

// DB is the database check; *pgxpool.Pool implements it.
type DB interface {
	Ping(ctx context.Context) error
}

// Workers counts the grading workers; *worker.Pool implements it.
type Workers interface {
	Workers() (alive, configured int)
}

// Queue reports the queue depth; *queue.Memory implements it.
type Queue interface {
	Len() int
}

// Report is the response body.
type Report struct {
	Status     string      `json:"status"`
	Database   string      `json:"database"`
	Workers    WorkerCount `json:"workers"`
	QueueDepth int         `json:"queue_depth"`
}

// WorkerCount is how many grading workers run, of how many configured.
type WorkerCount struct {
	Alive      int `json:"alive"`
	Configured int `json:"configured"`
}

// Checker is the /healthz handler. Mount it as "GET /healthz", which also answers HEAD.
type Checker struct {
	db      DB
	workers Workers
	queue   Queue
	now     func() time.Time

	mu   sync.Mutex // held during a check, so at most one runs at a time
	at   time.Time  // when last was computed; zero before the first check
	last Report
}

// New returns a Checker. workers and queue are nil when the process grades nothing; the report then
// shows zero of zero workers and an empty queue, and only the database decides the status.
func New(db DB, workers Workers, queue Queue) *Checker {
	return &Checker{db: db, workers: workers, queue: queue, now: time.Now}
}

// ServeHTTP answers 200 with status "ok" when the database answers and every configured worker runs,
// and 503 with status "unhealthy" otherwise.
func (c *Checker) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	rep := c.check(r.Context())
	code := http.StatusOK
	if rep.Status != StatusOK {
		code = http.StatusServiceUnavailable
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	if err := json.NewEncoder(w).Encode(rep); err != nil {
		slog.Debug("write /healthz response", "err", err)
	}
}

// check returns the cached report while it is younger than cacheFor, and runs a new check otherwise.
func (c *Checker) check(ctx context.Context) Report {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.at.IsZero() && c.now().Sub(c.at) < cacheFor {
		return c.last
	}

	rep := Report{Status: StatusOK, Database: DatabaseOK}
	// The result is shared with other requests, so a client that hangs up must not cut the ping
	// short and make the database look unreachable to everyone else.
	pingCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), pingTimeout)
	defer cancel()
	if err := c.db.Ping(pingCtx); err != nil {
		rep.Status, rep.Database = StatusUnhealthy, DatabaseUnreached
		slog.Warn("health check: database unreachable", "err", err)
	}
	if c.workers != nil {
		rep.Workers.Alive, rep.Workers.Configured = c.workers.Workers()
		if rep.Workers.Alive < rep.Workers.Configured {
			rep.Status = StatusUnhealthy
		}
	}
	if c.queue != nil {
		rep.QueueDepth = c.queue.Len()
	}

	c.at, c.last = c.now(), rep
	return rep
}
