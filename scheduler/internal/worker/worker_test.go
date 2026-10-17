package worker_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/harness"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/queue"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/store"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/worker"
)

// patience bounds every wait, so a bug fails a test instead of hanging it.
const patience = 10 * time.Second

// fakeQueue is a queue.JobQueue that remembers which submissions it holds, like the real one.
type fakeQueue struct {
	mu   sync.Mutex
	jobs chan queue.Job
	held map[int64]bool
}

func newFakeQueue() *fakeQueue {
	return &fakeQueue{jobs: make(chan queue.Job, 100), held: map[int64]bool{}}
}

func (q *fakeQueue) Enqueue(_ context.Context, job queue.Job) error {
	q.mu.Lock()
	defer q.mu.Unlock()
	if q.held[job.SubmissionID] {
		return queue.ErrDuplicate
	}
	q.held[job.SubmissionID] = true
	q.jobs <- job
	return nil
}

func (q *fakeQueue) Dequeue(ctx context.Context) (queue.Job, error) {
	select {
	case job := <-q.jobs:
		q.mu.Lock()
		delete(q.held, job.SubmissionID)
		q.mu.Unlock()
		return job, nil
	case <-ctx.Done():
		return queue.Job{}, ctx.Err()
	}
}

func (q *fakeQueue) Remove(context.Context, int64) (bool, error) { return false, nil }

// fakeStore records what the worker does and answers from hooks that a test can replace.
type fakeStore struct {
	mu        sync.Mutex
	claims    []int64
	completed []int64
	failed    []int64
	staleArgs []time.Duration

	claim     func(id int64) (*store.Claim, error)
	complete  func(c *store.Claim) error
	reconcile func(call int, staleAfter time.Duration) ([]queue.Job, error)
}

func (s *fakeStore) Claim(_ context.Context, id int64) (*store.Claim, error) {
	s.mu.Lock()
	s.claims = append(s.claims, id)
	hook := s.claim
	s.mu.Unlock()
	if hook != nil {
		return hook(id)
	}
	return defaultClaim(id), nil
}

func (s *fakeStore) Complete(_ context.Context, c *store.Claim, _ *harness.Report) error {
	s.mu.Lock()
	hook := s.complete
	s.mu.Unlock()
	if hook != nil {
		if err := hook(c); err != nil {
			return err
		}
	}
	s.mu.Lock()
	s.completed = append(s.completed, c.SubmissionID)
	s.mu.Unlock()
	return nil
}

func (s *fakeStore) Fail(_ context.Context, id int64) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.failed = append(s.failed, id)
	return nil
}

func (s *fakeStore) Reconcile(_ context.Context, staleAfter time.Duration) ([]queue.Job, error) {
	s.mu.Lock()
	s.staleArgs = append(s.staleArgs, staleAfter)
	call := len(s.staleArgs)
	hook := s.reconcile
	s.mu.Unlock()
	if hook != nil {
		return hook(call, staleAfter)
	}
	return nil, nil
}

func (s *fakeStore) snapshot() (claims, completed, failed []int64, stale []time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.claims), slices.Clone(s.completed), slices.Clone(s.failed), slices.Clone(s.staleArgs)
}

func defaultClaim(id int64) *store.Claim {
	return &store.Claim{
		SubmissionID: id, UserID: 7, AssignmentID: 1, CodePath: "stored.py",
		Definition: &harness.Definition{
			SubmissionID:  id,
			ResourceLimit: harness.ResourceLimit{TimeoutS: 5},
			TestCases:     []harness.TestCase{{ID: 1, Weight: 10}, {ID: 2, Weight: 10}},
		},
	}
}

// fakeGrader answers from a hook, or with an empty report.
type fakeGrader struct {
	mu    sync.Mutex
	paths []string
	run   func(ctx context.Context, def *harness.Definition) (*harness.Report, error)
}

func (g *fakeGrader) Run(ctx context.Context, def *harness.Definition, codePath string) (*harness.Report, error) {
	g.mu.Lock()
	g.paths = append(g.paths, codePath)
	hook := g.run
	g.mu.Unlock()
	if hook != nil {
		return hook(ctx, def)
	}
	return &harness.Report{SubmissionID: def.SubmissionID}, nil
}

type rig struct {
	t      *testing.T
	dir    string
	q      *fakeQueue
	s      *fakeStore
	g      *fakeGrader
	cancel context.CancelFunc
	done   chan error
}

// start runs a pool with the given config over fresh fakes. The test may set hooks before it enqueues.
func start(t *testing.T, cfg worker.Config) *rig {
	t.Helper()
	r := &rig{t: t, dir: t.TempDir(), q: newFakeQueue(), s: &fakeStore{}, g: &fakeGrader{}}
	if cfg.Workers == 0 {
		cfg.Workers = 1
	}
	cfg.CodeDir = r.dir
	cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	return r.run(cfg)
}

func (r *rig) run(cfg worker.Config) *rig {
	r.t.Helper()
	pool, err := worker.New(cfg, r.q, r.s, r.g)
	if err != nil {
		r.t.Fatalf("New: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel
	r.done = make(chan error, 1)
	go func() { r.done <- pool.Run(ctx) }()
	r.t.Cleanup(func() { r.stop() })
	return r
}

// stop shuts the pool down and waits for Run to return.
func (r *rig) stop() error {
	r.cancel()
	select {
	case err := <-r.done:
		r.done <- err // for a second stop
		return err
	case <-time.After(patience):
		r.t.Error("Run did not return after shutdown")
		return nil
	}
}

func (r *rig) enqueue(id int64) {
	r.t.Helper()
	if err := r.q.Enqueue(context.Background(), queue.Job{SubmissionID: id, UserID: 7}); err != nil {
		r.t.Fatal(err)
	}
}

// eventually waits until cond holds.
func eventually(t *testing.T, what string, cond func() bool) {
	t.Helper()
	for deadline := time.Now().Add(patience); !cond(); time.Sleep(time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("timed out waiting for %s", what)
		}
	}
}

func (r *rig) waitCompleted(ids ...int64) {
	r.t.Helper()
	eventually(r.t, fmt.Sprintf("submissions %v to complete", ids), func() bool {
		_, completed, _, _ := r.s.snapshot()
		return slices.Equal(completed, ids)
	})
}

func TestNewRejectsABadConfig(t *testing.T) {
	q, s, g := newFakeQueue(), &fakeStore{}, &fakeGrader{}
	if _, err := worker.New(worker.Config{Workers: 0, CodeDir: "x"}, q, s, g); err == nil {
		t.Error("New accepted zero workers")
	}
	if _, err := worker.New(worker.Config{Workers: 1}, q, s, g); err == nil {
		t.Error("New accepted an empty code directory")
	}
}

func TestGradesAQueuedJob(t *testing.T) {
	r := start(t, worker.Config{})
	r.enqueue(1)
	r.waitCompleted(1)

	_, _, failed, stale := r.s.snapshot()
	if len(failed) != 0 {
		t.Errorf("failed = %v", failed)
	}
	if want := filepath.Join(r.dir, "stored.py"); len(r.g.paths) != 1 || r.g.paths[0] != want {
		t.Errorf("grader got paths %v, want [%s]", r.g.paths, want)
	}
	if len(stale) == 0 || stale[0] != 0 {
		t.Errorf("start-up reconcile was called with %v, want 0 first", stale)
	}
}

func TestDropsAJobThatIsNoLongerQueued(t *testing.T) {
	r := start(t, worker.Config{})
	r.s.claim = func(id int64) (*store.Claim, error) {
		if id == 1 {
			return nil, store.ErrNotQueued
		}
		return defaultClaim(id), nil
	}
	r.enqueue(1)
	r.enqueue(2)
	r.waitCompleted(2) // one worker takes them in order, so job 1 is settled by now

	_, _, failed, _ := r.s.snapshot()
	if len(failed) != 0 || len(r.g.paths) != 1 {
		t.Fatalf("failed = %v, grader runs = %d; want a dropped job to cost nothing", failed, len(r.g.paths))
	}
}

func TestMarksTheSubmissionFailedWhenThePipelineBreaks(t *testing.T) {
	tests := []struct {
		name  string
		setup func(r *rig)
	}{
		{"claim fails", func(r *rig) {
			r.s.claim = func(id int64) (*store.Claim, error) { return nil, errors.New("connection reset") }
		}},
		{"no resource limit", func(r *rig) {
			r.s.claim = func(id int64) (*store.Claim, error) {
				return nil, fmt.Errorf("%w: assignment 1", store.ErrNoResourceLimit)
			}
		}},
		{"the Harness fails", func(r *rig) {
			r.g.run = func(context.Context, *harness.Definition) (*harness.Report, error) {
				return nil, fmt.Errorf("%w: exit status 2", harness.ErrHarness)
			}
		}},
		{"the report is invalid", func(r *rig) {
			r.g.run = func(context.Context, *harness.Definition) (*harness.Report, error) {
				return nil, fmt.Errorf("%w: totals", harness.ErrReport)
			}
		}},
		{"the Harness panics", func(r *rig) {
			r.g.run = func(context.Context, *harness.Definition) (*harness.Report, error) { panic("boom") }
		}},
		{"storing the report fails", func(r *rig) {
			r.s.complete = func(*store.Claim) error { return errors.New("deadlock detected") }
		}},
		{"the code path leaves the code directory", func(r *rig) {
			r.s.claim = func(id int64) (*store.Claim, error) {
				c := defaultClaim(id)
				c.CodePath = "../../etc/passwd"
				return c, nil
			}
		}},
		{"the code path is absolute", func(r *rig) {
			r.s.claim = func(id int64) (*store.Claim, error) {
				c := defaultClaim(id)
				c.CodePath = "/etc/passwd"
				return c, nil
			}
		}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := start(t, worker.Config{})
			// The hooks apply to job 1 only; job 2 proves the worker is still alive afterwards.
			tc.setup(r)
			r.enqueue(1)
			eventually(t, "job 1 to be failed", func() bool {
				_, _, failed, _ := r.s.snapshot()
				return slices.Equal(failed, []int64{1})
			})
			if _, completed, _, _ := r.s.snapshot(); len(completed) != 0 {
				t.Fatalf("completed = %v after a pipeline failure", completed)
			}

			r.s.mu.Lock()
			r.s.claim, r.s.complete = nil, nil
			r.s.mu.Unlock()
			r.g.mu.Lock()
			r.g.run = nil
			r.g.mu.Unlock()
			r.enqueue(2)
			r.waitCompleted(2)
		})
	}
}

func TestDoesNotFailASubmissionWhoseReportIsDiscarded(t *testing.T) {
	r := start(t, worker.Config{})
	r.s.complete = func(c *store.Claim) error {
		if c.SubmissionID == 1 {
			return store.ErrNotRunning // requeued while it was being graded
		}
		return nil
	}
	r.enqueue(1)
	r.enqueue(2)
	r.waitCompleted(2)
	if _, _, failed, _ := r.s.snapshot(); len(failed) != 0 {
		t.Fatalf("failed = %v; a requeued submission must be left alone", failed)
	}
}

func TestBackstopDeadlineFailsAHungHarness(t *testing.T) {
	// Two test cases at 0 s each, plus 1 ms overhead each and a 1 ms margin: a 3 ms deadline.
	r := start(t, worker.Config{TestOverhead: time.Millisecond, Margin: time.Millisecond})
	r.s.claim = func(id int64) (*store.Claim, error) {
		c := defaultClaim(id)
		c.Definition.ResourceLimit.TimeoutS = 0
		return c, nil
	}
	r.g.run = func(ctx context.Context, _ *harness.Definition) (*harness.Report, error) {
		<-ctx.Done() // a Harness that never answers
		return nil, ctx.Err()
	}
	r.enqueue(1)
	eventually(t, "the hung job to be failed", func() bool {
		_, _, failed, _ := r.s.snapshot()
		return slices.Equal(failed, []int64{1})
	})
}

func TestShutdownInterruptsAJobAndLeavesItRunning(t *testing.T) {
	r := start(t, worker.Config{})
	started := make(chan struct{})
	r.g.run = func(ctx context.Context, _ *harness.Definition) (*harness.Report, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	r.enqueue(1)
	<-started

	if err := r.stop(); err != nil {
		t.Fatalf("Run: %v", err)
	}
	_, completed, failed, _ := r.s.snapshot()
	if len(completed) != 0 || len(failed) != 0 {
		t.Fatalf("completed = %v, failed = %v; an interrupted job must stay RUNNING for the next start-up", completed, failed)
	}
}

func TestWorkersRunConcurrently(t *testing.T) {
	const workers = 3
	r := start(t, worker.Config{Workers: workers})
	var arrived sync.WaitGroup
	arrived.Add(workers)
	released := make(chan struct{})
	go func() { arrived.Wait(); close(released) }()
	r.g.run = func(ctx context.Context, def *harness.Definition) (*harness.Report, error) {
		arrived.Done()
		select {
		case <-released: // every worker is inside Run at the same time
			return &harness.Report{SubmissionID: def.SubmissionID}, nil
		case <-time.After(patience):
			return nil, errors.New("the other workers never arrived")
		}
	}
	for id := int64(1); id <= workers; id++ {
		r.enqueue(id)
	}
	eventually(t, "all jobs to complete", func() bool {
		_, completed, failed, _ := r.s.snapshot()
		return len(completed) == workers && len(failed) == 0
	})
}

func TestStartUpRequeuesWhatThePreviousRunLeft(t *testing.T) {
	q, s, g := newFakeQueue(), &fakeStore{}, &fakeGrader{}
	s.reconcile = func(call int, staleAfter time.Duration) ([]queue.Job, error) {
		if call == 1 {
			return []queue.Job{{SubmissionID: 5, UserID: 7}, {SubmissionID: 6, UserID: 8}}, nil
		}
		return nil, nil
	}
	r := &rig{t: t, dir: t.TempDir(), q: q, s: s, g: g}
	r.run(worker.Config{Workers: 1, CodeDir: r.dir, Logger: slog.New(slog.NewTextHandler(io.Discard, nil))})
	r.waitCompleted(5, 6)
}

func TestStartUpFailureStopsThePool(t *testing.T) {
	q, s, g := newFakeQueue(), &fakeStore{}, &fakeGrader{}
	s.reconcile = func(int, time.Duration) ([]queue.Job, error) { return nil, errors.New("database down") }
	pool, err := worker.New(worker.Config{Workers: 1, CodeDir: t.TempDir(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil))}, q, s, g)
	if err != nil {
		t.Fatal(err)
	}
	if err := pool.Run(context.Background()); err == nil {
		t.Fatal("Run succeeded although it could not read the database")
	}
}

func TestPeriodicReconcileUsesTheLeaseAndToleratesDuplicates(t *testing.T) {
	q, s, g := newFakeQueue(), &fakeStore{}, &fakeGrader{}
	blocked := make(chan struct{})
	g.run = func(ctx context.Context, def *harness.Definition) (*harness.Report, error) {
		<-blocked // keep the only worker busy, so the job it is not holding stays queued
		return &harness.Report{SubmissionID: def.SubmissionID}, nil
	}
	defer close(blocked)
	// Every reconciliation after the first lists job 9 as QUEUED, which the queue already holds.
	s.reconcile = func(call int, staleAfter time.Duration) ([]queue.Job, error) {
		if call == 1 {
			return nil, nil
		}
		return []queue.Job{{SubmissionID: 9, UserID: 7}}, nil
	}
	r := &rig{t: t, dir: t.TempDir(), q: q, s: s, g: g}
	r.enqueue(8) // the worker takes job 8 and blocks; job 9 arrives by reconciliation and waits
	r.run(worker.Config{
		Workers: 1, CodeDir: r.dir, Lease: 42 * time.Minute, ReconcileEvery: 2 * time.Millisecond,
		Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	eventually(t, "several periodic reconciliations", func() bool {
		_, _, _, stale := s.snapshot()
		return len(stale) >= 4
	})
	_, _, _, stale := s.snapshot()
	if stale[0] != 0 {
		t.Errorf("start-up reconcile used %v, want 0", stale[0])
	}
	for _, d := range stale[1:] {
		if d != 42*time.Minute {
			t.Fatalf("periodic reconcile used %v, want the lease", d)
		}
	}
	q.mu.Lock()
	defer q.mu.Unlock()
	if len(q.jobs) != 1 {
		t.Fatalf("%d jobs queued, want 1: the duplicate listing must be ignored", len(q.jobs))
	}
}
