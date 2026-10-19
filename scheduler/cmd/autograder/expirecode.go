package main

import (
	"context"
	"errors"
	"flag"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/retention"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/migrations"
)

const expireCodeUsage = "usage: autograder expire-code [--dry-run] " +
	"(with DATABASE_URL and an absolute CODE_DIR in the environment)"

// expireCode implements `autograder expire-code`, which infra/systemd/autograder-expire.timer runs
// daily: it deletes the stored files of submissions whose assignment closed more than one academic
// year ago, taken as one calendar year, and keeps their grades (NFR-13, SRS 13.4). With --dry-run
// it only counts them. Like seed-admin it applies the migrations first, because it needs
// submissions.code_deleted_at.
func expireCode(args []string, getenv func(string) string) error {
	fs := flag.NewFlagSet("expire-code", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	dryRun := fs.Bool("dry-run", false, "count the files due for deletion without deleting them")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 {
		return errors.New(expireCodeUsage)
	}
	databaseURL, codeDir := getenv("DATABASE_URL"), getenv("CODE_DIR")
	if databaseURL == "" || !filepath.IsAbs(codeDir) {
		return errors.New(expireCodeUsage)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()
	if _, err := migrations.Up(databaseURL); err != nil {
		return err
	}
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		return err
	}
	defer pool.Close()

	closedBefore := time.Now().UTC().AddDate(-1, 0, 0)
	if *dryRun {
		n, err := retention.Pending(ctx, pool, closedBefore)
		if err != nil {
			return err
		}
		slog.Info("submission files due for deletion", "count", n, "closed_before", closedBefore)
		return nil
	}
	r, err := retention.Expire(ctx, pool, codeDir, closedBefore)
	slog.Info("deleted the files of expired submissions", "deleted", r.Deleted, "missing", r.Missing,
		"failed", r.Failed, "closed_before", closedBefore)
	return err
}
