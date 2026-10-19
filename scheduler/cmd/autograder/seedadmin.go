package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/seed"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/migrations"
)

// adminPasswordEnv carries the first administrator's password. A flag would put it in the process
// list and the shell history; the deployment reads it into the variable without echo and unsets it
// afterwards (infra/README.md).
const adminPasswordEnv = "ADMIN_PASSWORD"

const seedAdminUsage = "usage: autograder seed-admin --name NAME --email EMAIL " +
	"(with DATABASE_URL and " + adminPasswordEnv + " in the environment)"

// seedAdmin implements `autograder seed-admin`: it applies the migrations, so it works before the
// service has ever started, and creates the first administrator (FR-19, SRS 13.2).
func seedAdmin(args []string, getenv func(string) string) error {
	fs := flag.NewFlagSet("seed-admin", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	name := fs.String("name", "", "display name")
	email := fs.String("email", "", "e-mail address to log in with")
	if err := fs.Parse(args); err != nil || fs.NArg() != 0 || *name == "" || *email == "" {
		return errors.New(seedAdminUsage)
	}
	databaseURL, password := getenv("DATABASE_URL"), getenv(adminPasswordEnv)
	if databaseURL == "" || password == "" {
		return errors.New(seedAdminUsage)
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

	id, err := seed.FirstAdmin(ctx, pool, *name, *email, password)
	switch {
	case errors.Is(err, seed.ErrAdminExists):
		return fmt.Errorf("%w; appoint further administrators through the admin API", err)
	case err != nil:
		return err
	}
	slog.Info("administrator created", "user", id, "email", strings.ToLower(strings.TrimSpace(*email)))
	return nil
}
