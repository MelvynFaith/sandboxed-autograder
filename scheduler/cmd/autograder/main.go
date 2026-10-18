package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/api"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/auth"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/harness"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/queue"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/sandbox"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/store"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/worker"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
)

// shutdownTimeout bounds how long open HTTP requests may take to finish on shutdown.
const shutdownTimeout = 10 * time.Second

func main() {
	// One JSON object per line (NFR-09); the log package's output goes through the same handler.
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))
	if err := run(); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func run() error {
	cfg, err := loadConfig(os.Getenv)
	if err != nil {
		return err
	}
	tokens, err := auth.NewTokens([]byte(cfg.jwtSecret))
	if err != nil {
		return err
	}
	ticketStore, err := auth.NewTicketStore(auth.MaxTicketTTL)
	if err != nil {
		return err
	}
	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	version, err := migrations.Up(cfg.databaseURL)
	if err != nil {
		return err
	}
	slog.Info("database schema is up to date", "version", version)

	pool, err := pgxpool.New(ctx, cfg.databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()
	if err := pool.Ping(ctx); err != nil {
		return err
	}

	// grading is nil, and never ready, when the pool is off.
	var grading chan error
	if cfg.workers > 0 {
		workers, err := newWorkers(cfg, pool)
		if err != nil {
			return err
		}
		grading = make(chan error, 1)
		go func() { grading <- workers.Run(ctx) }()
		slog.Info("grading worker pool started", "workers", cfg.workers)
	} else {
		slog.Warn("WORKERS=0: no submission will be graded by this process")
	}

	server := &http.Server{
		Addr:              cfg.httpAddr,
		Handler:           api.NewHandler(auth.NewStore(pool), tokens, ticketStore),
		ReadHeaderTimeout: 5 * time.Second,
		IdleTimeout:       60 * time.Second,
	}
	serverErrors := make(chan error, 1)
	go func() {
		log.Printf("scheduler API listening on %s", cfg.httpAddr)
		serverErrors <- server.ListenAndServe()
	}()

	// Whichever ends first takes the other down with it: a signal, a server that cannot listen, or a
	// pool that cannot read the database at start-up. Run only returns early in that last case.
	var errs []error
	select {
	case err := <-serverErrors:
		errs = append(errs, err)
	case err := <-grading:
		errs = append(errs, err)
		grading = nil
	case <-ctx.Done():
	}
	cancel()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer shutdownCancel()
	errs = append(errs, server.Shutdown(shutdownCtx))
	if grading != nil {
		// A job in progress is interrupted and left RUNNING for the next start-up.
		errs = append(errs, <-grading)
	}
	return errors.Join(errs...)
}

// newWorkers prepares the delegated cgroup and builds the worker pool that grades submissions through
// the Harness (SRS 3.3). The queue is the in-process one; Reconcile fills it from the database at
// start-up and every minute after.
func newWorkers(cfg config, pool *pgxpool.Pool) (*worker.Pool, error) {
	cgroupRoot, err := sandbox.Delegate()
	if err != nil {
		return nil, err
	}
	slog.Info("delegated cgroup ready", "cgroup_root", cgroupRoot)
	runner := &harness.Runner{
		Command: cfg.harnessCommand,
		Env: []string{
			harness.EnvSandboxExec + "=" + cfg.sandboxExec,
			harness.EnvCgroupRoot + "=" + cgroupRoot,
		},
	}
	sweeper := sandbox.Sweeper{Binary: cfg.sandboxExec, CgroupRoot: cgroupRoot}
	return worker.New(worker.Config{
		Workers: cfg.workers,
		CodeDir: cfg.codeDir,
		Sweep:   sweeper.Sweep,
	}, queue.NewMemory(), store.NewPipeline(pool), runner)
}
