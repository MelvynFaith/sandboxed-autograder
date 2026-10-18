// Package worker is the pool of goroutines that grades submissions. Each one takes a job from the
// queue, claims the submission, runs the Grading Harness and stores the report (SRS 3.3, steps 3 to 8).
//
// What the pool guarantees, by the SRS rules for workers:
//
//   - A job runs under recover(). A panic marks that submission FAILED and the worker carries on.
//   - A job has a backstop deadline derived from its own definition, so a Harness that hangs cannot hold
//     a worker for ever.
//   - At start-up, and then periodically, RUNNING submissions that have not changed for a lease are put
//     back in the queue, and QUEUED submissions the queue does not hold are added to it (NFR-04).
//   - On shutdown, a job in progress is interrupted and left RUNNING. It is not marked FAILED: the next
//     start-up requeues it, so a restart does not cost anyone a grade.
package worker

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime/debug"
	"sync"
	"sync/atomic"
	"time"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/harness"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/queue"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/store"
)

// Store is the database side of grading. *store.Pipeline implements it.
type Store interface {
	Claim(ctx context.Context, submissionID int64) (*store.Claim, error)
	Complete(ctx context.Context, c *store.Claim, r *harness.Report) error
	Fail(ctx context.Context, submissionID int64) error
	Reconcile(ctx context.Context, staleAfter time.Duration) ([]queue.Job, error)
}

// Grader runs the Harness for one submission. *harness.Runner implements it.
type Grader interface {
	Run(ctx context.Context, def *harness.Definition, codePath string) (*harness.Report, error)
}

// Defaults for the zero fields of Config.
const (
	DefaultLease          = 30 * time.Minute
	DefaultReconcileEvery = time.Minute
	DefaultTestOverhead   = 2 * time.Second
	DefaultMargin         = 30 * time.Second

	// failTimeout bounds the write that marks a submission FAILED, which must not wait on a context
	// that has been cancelled.
	failTimeout = 10 * time.Second

	// retryDelay is how long a worker waits after the queue fails for a reason other than shutdown.
	retryDelay = time.Second
)

// Config configures a Pool.
type Config struct {
	// Workers is the number of submissions graded at once. It must be at least 1.
	Workers int

	// CodeDir holds the stored student files; a submission's code_path is relative to it.
	CodeDir string

	// Lease is how long a RUNNING submission may go unchanged before it is taken for lost and
	// requeued. It must be longer than any job runs, which is at most Lease/2 (see backstop).
	// Zero means DefaultLease.
	Lease time.Duration

	// ReconcileEvery is how often the pool looks for lost submissions. Zero means DefaultReconcileEvery.
	ReconcileEvery time.Duration

	// TestOverhead is the time allowed per test case beyond its own timeout_s, for the sandbox and
	// interpreter to start and the output to be compared. Zero means DefaultTestOverhead.
	TestOverhead time.Duration

	// Margin is added to the sum over test cases to give a job's backstop deadline. Zero means
	// DefaultMargin.
	Margin time.Duration

	// Sweep removes what dead sandbox runs left behind (empty cgroups, run directories). Run calls it
	// once, before it requeues anything, when no sandbox of this process can be alive. A failure is
	// logged and does not stop the pool. Nil means no sweep; sandbox.Sweeper.Sweep fits.
	Sweep func(ctx context.Context) error

	// Logger receives one line per job. Nil means slog.Default().
	Logger *slog.Logger
}

// Pool grades the jobs of a queue.
type Pool struct {
	cfg   Config
	queue queue.JobQueue
	store Store
	grade Grader
	log   *slog.Logger
	alive atomic.Int32 // worker goroutines running
}

// New returns a Pool. It does not start any goroutine; Run does.
func New(cfg Config, q queue.JobQueue, s Store, g Grader) (*Pool, error) {
	if cfg.Workers < 1 {
		return nil, fmt.Errorf("worker: %d workers, need at least 1", cfg.Workers)
	}
	if cfg.CodeDir == "" {
		return nil, errors.New("worker: no code directory")
	}
	if cfg.Lease == 0 {
		cfg.Lease = DefaultLease
	}
	if cfg.ReconcileEvery == 0 {
		cfg.ReconcileEvery = DefaultReconcileEvery
	}
	if cfg.TestOverhead == 0 {
		cfg.TestOverhead = DefaultTestOverhead
	}
	if cfg.Margin == 0 {
		cfg.Margin = DefaultMargin
	}
	log := cfg.Logger
	if log == nil {
		log = slog.Default()
	}
	return &Pool{cfg: cfg, queue: q, store: s, grade: g, log: log}, nil
}

// Run requeues what a previous run left behind, then grades jobs until ctx is done. It returns after
// every worker has stopped. The error is non-nil only if the start-up reconciliation fails: a pool that
// cannot read the database should not start.
func (p *Pool) Run(ctx context.Context) error {
	if p.cfg.Sweep != nil {
		if err := p.cfg.Sweep(ctx); err != nil {
			p.log.Error("sweep of dead sandbox runs failed; continuing", "err", err)
		} else {
			p.log.Info("swept leftovers of dead sandbox runs")
		}
	}
	// Nothing is running yet, so every RUNNING submission belongs to a previous process.
	if err := p.reconcile(ctx, 0); err != nil {
		return fmt.Errorf("worker: reconcile at start-up: %w", err)
	}

	var wg sync.WaitGroup
	for range p.cfg.Workers {
		wg.Add(1)
		p.alive.Add(1)
		go func() {
			defer wg.Done()
			defer p.alive.Add(-1)
			p.work(ctx)
		}()
	}
	wg.Add(1)
	go func() {
		defer wg.Done()
		p.watch(ctx)
	}()
	wg.Wait()
	return nil
}

// Workers reports how many worker goroutines are running and how many the pool was configured with,
// for /healthz (NFR-09). None run until Run has finished the start-up reconciliation, and none after
// Run returns, so fewer than configured means the pool is not grading at full strength.
func (p *Pool) Workers() (alive, configured int) {
	return int(p.alive.Load()), p.cfg.Workers
}

// work takes jobs until ctx is done.
func (p *Pool) work(ctx context.Context) {
	for {
		job, err := p.queue.Dequeue(ctx)
		if err != nil {
			if ctx.Err() != nil {
				return
			}
			p.log.Error("dequeue failed", "err", err)
			select {
			case <-time.After(retryDelay):
			case <-ctx.Done():
				return
			}
			continue
		}
		p.process(ctx, job)
	}
}

// watch looks for lost submissions every ReconcileEvery.
func (p *Pool) watch(ctx context.Context) {
	t := time.NewTicker(p.cfg.ReconcileEvery)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			if err := p.reconcile(ctx, p.cfg.Lease); err != nil && ctx.Err() == nil {
				p.log.Error("reconcile failed", "err", err)
			}
		}
	}
}

// reconcile requeues submissions the database holds RUNNING for longer than staleAfter and puts every
// QUEUED submission in the queue. The queue turns away a submission it already holds.
func (p *Pool) reconcile(ctx context.Context, staleAfter time.Duration) error {
	jobs, err := p.store.Reconcile(ctx, staleAfter)
	if err != nil {
		return err
	}
	for _, job := range jobs {
		switch err := p.queue.Enqueue(ctx, job); {
		case err == nil:
			p.log.Info("queued", "submission", job.SubmissionID, "user", job.UserID, "reason", "reconcile")
		case errors.Is(err, queue.ErrDuplicate):
		default:
			return fmt.Errorf("enqueue submission %d: %w", job.SubmissionID, err)
		}
	}
	return nil
}

// process grades one job. It never panics and never returns an error: whatever goes wrong is logged
// and, if the pipeline broke, recorded as FAILED.
func (p *Pool) process(ctx context.Context, job queue.Job) {
	log := p.log.With("submission", job.SubmissionID, "user", job.UserID)
	defer func() {
		if r := recover(); r != nil {
			log.Error("grading panicked", "panic", r, "stack", string(debug.Stack()))
			p.fail(ctx, log, job.SubmissionID)
		}
	}()

	claim, err := p.store.Claim(ctx, job.SubmissionID)
	switch {
	case errors.Is(err, store.ErrNotQueued):
		log.Info("dropped: the submission is no longer queued")
		return
	case err != nil:
		if ctx.Err() == nil {
			log.Error("claim failed", "err", err)
			p.fail(ctx, log, job.SubmissionID)
		}
		return
	}

	if !filepath.IsLocal(claim.CodePath) {
		log.Error("stored code path escapes the code directory", "code_path", claim.CodePath)
		p.fail(ctx, log, job.SubmissionID)
		return
	}

	start := time.Now()
	jobCtx, cancel := context.WithTimeout(ctx, p.backstop(claim.Definition))
	report, err := p.grade.Run(jobCtx, claim.Definition, filepath.Join(p.cfg.CodeDir, claim.CodePath))
	cancel()
	if err != nil {
		if ctx.Err() != nil {
			log.Warn("interrupted by shutdown; left RUNNING for the next start-up", "err", err)
			return
		}
		log.Error("grading failed", "err", err, "elapsed", time.Since(start))
		p.fail(ctx, log, job.SubmissionID)
		return
	}

	switch err := p.store.Complete(ctx, claim, report); {
	case err == nil:
		log.Info("graded", "score", report.TotalScore, "max", report.MaxScore, "elapsed", time.Since(start))
	case errors.Is(err, store.ErrNotRunning):
		log.Warn("report discarded: the submission was requeued while it was being graded")
	case ctx.Err() != nil:
		log.Warn("interrupted by shutdown before the report was stored; left RUNNING", "err", err)
	default:
		log.Error("storing the report failed", "err", err)
		p.fail(ctx, log, job.SubmissionID)
	}
}

// backstop is the longest a job may take: every test case may use its full wall-clock timeout plus
// the overhead, plus the margin, and never more than half the lease so that a job always finishes
// before its submission can be taken for lost.
func (p *Pool) backstop(def *harness.Definition) time.Duration {
	each := time.Duration(def.ResourceLimit.TimeoutS)*time.Second + p.cfg.TestOverhead
	d := time.Duration(len(def.TestCases))*each + p.cfg.Margin
	return min(d, p.cfg.Lease/2)
}

// fail marks the submission FAILED. It does not use ctx's cancellation: a worker that decides to fail
// a submission just as the pool shuts down should still write it.
func (p *Pool) fail(ctx context.Context, log *slog.Logger, submissionID int64) {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), failTimeout)
	defer cancel()
	switch err := p.store.Fail(ctx, submissionID); {
	case err == nil:
		log.Info("marked FAILED")
	case errors.Is(err, store.ErrNotRunning):
		// It never reached RUNNING, or something else changed it first; there is nothing to fail.
	default:
		log.Error("could not mark FAILED; reconciliation will pick the submission up again", "err", err)
	}
}
