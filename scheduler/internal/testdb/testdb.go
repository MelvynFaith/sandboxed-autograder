// Package testdb gives a test a PostgreSQL schema of its own with every migration applied.
//
// Point AUTOGRADER_TEST_DATABASE_URL at an empty or throwaway database. Each test that asks for a
// Pool gets a new schema inside it, so tests do not see each other's rows and can run in any order.
package testdb

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/migrations"
)

const urlEnv = "AUTOGRADER_TEST_DATABASE_URL"

// Pool returns a pool whose connections all use a fresh, fully migrated schema. The schema is
// dropped when the test ends.
//
// A test that needs a database is skipped when the variable is unset. In CI (the CI variable is
// set) a missing variable fails the test instead, so the tests cannot be skipped silently.
func Pool(t testing.TB) *pgxpool.Pool {
	t.Helper()
	base := os.Getenv(urlEnv)
	if base == "" {
		if os.Getenv("CI") != "" {
			t.Fatalf("%s must be set in CI", urlEnv)
		}
		t.Skipf("%s is not set", urlEnv)
	}

	ctx := context.Background()
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	schema := pgx.Identifier{fmt.Sprintf("testdb_%d_%d", os.Getpid(), time.Now().UnixNano())}
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema.Sanitize()); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(ctx, "DROP SCHEMA "+schema.Sanitize()+" CASCADE")
		_ = admin.Close(ctx)
	})

	u, err := url.Parse(base)
	if err != nil {
		t.Fatalf("parse %s: %v", urlEnv, err)
	}
	q := u.Query()
	q.Set("search_path", schema[0])
	u.RawQuery = q.Encode()

	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		t.Fatalf("open migrations: %v", err)
	}
	mu := *u
	mu.Scheme = "pgx5" // the scheme golang-migrate's pgx v5 driver registers
	m, err := migrate.NewWithSourceInstance("iofs", src, mu.String())
	if err != nil {
		t.Fatalf("new migrator: %v", err)
	}
	if err := m.Up(); err != nil {
		t.Fatalf("migrate up: %v", err)
	}
	if srcErr, dbErr := m.Close(); srcErr != nil || dbErr != nil {
		t.Fatalf("close migrator: %v, %v", srcErr, dbErr)
	}

	pool, err := pgxpool.New(ctx, u.String())
	if err != nil {
		t.Fatalf("open pool: %v", err)
	}
	t.Cleanup(pool.Close)
	return pool
}
