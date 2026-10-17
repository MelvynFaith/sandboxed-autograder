package migrations_test

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/migrations"
)

// Most tests need a PostgreSQL server. Point AUTOGRADER_TEST_DATABASE_URL at an empty or throwaway
// database; each test creates its own schema inside it and drops the schema afterwards. CI sets the
// variable, and a missing value there is a failure rather than a skip.
const databaseURLEnv = "AUTOGRADER_TEST_DATABASE_URL"

// SQLSTATE codes the schema is expected to raise. Deleting a row that an ON DELETE RESTRICT key
// protects raises foreign_key_violation on PostgreSQL 16 and restrict_violation on 18, so that case
// accepts either.
const (
	notNullViolation    = "23502"
	foreignKeyViolation = "23503"
	uniqueViolation     = "23505"
	checkViolation      = "23514"
	restrictViolation   = "23001"
)

// matches reports whether got is the SQLSTATE the case wants.
func matches(got, want string) bool {
	return got == want || (want == restrictViolation && got == foreignKeyViolation)
}

// tables is every table created by the migrations, which the round trip counts.
const tables = 11

// newMigrator creates an empty schema for the test and returns a migrator and a connection that both
// use it.
func newMigrator(t *testing.T) (*migrate.Migrate, *pgx.Conn) {
	t.Helper()
	base := os.Getenv(databaseURLEnv)
	if base == "" {
		if os.Getenv("CI") != "" {
			t.Fatalf("%s must be set in CI", databaseURLEnv)
		}
		t.Skipf("%s is not set", databaseURLEnv)
	}

	ctx := context.Background()
	admin, err := pgx.Connect(ctx, base)
	if err != nil {
		t.Fatalf("connect: %v", err)
	}
	schema := pgx.Identifier{fmt.Sprintf("migrations_test_%d_%d", os.Getpid(), time.Now().UnixNano())}
	if _, err := admin.Exec(ctx, "CREATE SCHEMA "+schema.Sanitize()); err != nil {
		t.Fatalf("create schema: %v", err)
	}
	t.Cleanup(func() {
		_, _ = admin.Exec(ctx, "DROP SCHEMA "+schema.Sanitize()+" CASCADE")
		_ = admin.Close(ctx)
	})

	u, err := url.Parse(base)
	if err != nil {
		t.Fatalf("parse %s: %v", databaseURLEnv, err)
	}
	q := u.Query()
	q.Set("search_path", schema[0])
	u.RawQuery = q.Encode()

	conn, err := pgx.Connect(ctx, u.String())
	if err != nil {
		t.Fatalf("connect to schema: %v", err)
	}
	t.Cleanup(func() { _ = conn.Close(ctx) })

	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		t.Fatalf("open migrations: %v", err)
	}
	u.Scheme = "pgx5" // the scheme golang-migrate's pgx v5 driver registers
	m, err := migrate.NewWithSourceInstance("iofs", src, u.String())
	if err != nil {
		t.Fatalf("new migrator: %v", err)
	}
	t.Cleanup(func() { _, _ = m.Close() })
	return m, conn
}

func count(t *testing.T, conn *pgx.Conn, query string) int {
	t.Helper()
	var n int
	if err := conn.QueryRow(context.Background(), query).Scan(&n); err != nil {
		t.Fatalf("%s: %v", query, err)
	}
	return n
}

// objects counts what the migrations leave behind in the test schema, apart from golang-migrate's
// own bookkeeping table.
func objects(t *testing.T, conn *pgx.Conn) (tableCount, functionCount int) {
	t.Helper()
	tableCount = count(t, conn, `SELECT count(*) FROM information_schema.tables
		WHERE table_schema = current_schema() AND table_name <> 'schema_migrations'`)
	functionCount = count(t, conn, `SELECT count(*) FROM pg_proc
		WHERE pronamespace = (SELECT oid FROM pg_namespace WHERE nspname = current_schema())`)
	return tableCount, functionCount
}

func TestUpDownUp(t *testing.T) {
	m, conn := newMigrator(t)

	if err := m.Up(); err != nil {
		t.Fatalf("up: %v", err)
	}
	if tc, fc := objects(t, conn); tc != tables || fc != 1 {
		t.Fatalf("after up: %d tables, %d functions; want %d, 1", tc, fc, tables)
	}

	if err := m.Down(); err != nil {
		t.Fatalf("down: %v", err)
	}
	if tc, fc := objects(t, conn); tc != 0 || fc != 0 {
		t.Fatalf("after down: %d tables, %d functions left; want 0, 0", tc, fc)
	}

	if err := m.Up(); err != nil {
		t.Fatalf("second up: %v", err)
	}
}

// seed inserts one complete, valid graph. The constraint tests refer to its rows by id: a fresh
// schema numbers the first row of every table 1, and the student is user 2.
func seed(t *testing.T, conn *pgx.Conn) {
	t.Helper()
	stmts := []string{
		`INSERT INTO users (name, email, password_hash, role) VALUES
			('Lecturer', 'lecturer@example.test', 'x', 'LECTURER'),
			('Student', 'student@example.test', 'x', 'STUDENT')`,
		`INSERT INTO courses (code, name, lecturer_id, term) VALUES ('C1', 'Course', 1, '2026-1')`,
		`INSERT INTO enrollments (course_id, user_id) VALUES (1, 2)`,
		`INSERT INTO assignments (course_id, title, deadline_at, late_until_at)
			VALUES (1, 'A1', now(), now() + interval '1 day')`,
		`INSERT INTO resource_limits (assignment_id, cpu_time_ms, memory_mb, max_processes, timeout_s,
			max_output_bytes, tmpfs_size_mb) VALUES (1, 2000, 64, 5, 5, 1048576, 32)`,
		`INSERT INTO test_cases (assignment_id, external_id, input, expected_output, weight, match_mode)
			VALUES (1, 'tc_01', '', '', 20, 'EXACT')`,
		`INSERT INTO submissions (user_id, assignment_id, code_path, original_filename, size_bytes)
			VALUES (2, 1, 'f0', 'main.py', 10)`,
		`INSERT INTO results (submission_id, testcase_id, status, exec_time_ms, memory_kb, memory_source, score)
			VALUES (1, 1, 'PASSED', 5, 1024, 'CGROUP_PEAK', 20)`,
		`INSERT INTO audit_logs (user_id, submission_id, event_type, syscall_nr) VALUES (2, 1, 'SANDBOX_VIOLATION', 41)`,
	}
	for _, s := range stmts {
		if _, err := conn.Exec(context.Background(), s); err != nil {
			t.Fatalf("seed: %v\n%s", err, s)
		}
	}
}

func TestConstraints(t *testing.T) {
	m, conn := newMigrator(t)
	if err := m.Up(); err != nil {
		t.Fatalf("up: %v", err)
	}
	seed(t, conn)

	cases := []struct {
		name string
		sql  string
		code string
	}{
		{"email must be lower case",
			`INSERT INTO users (name, email, password_hash, role) VALUES ('n', 'A@example.test', 'x', 'STUDENT')`,
			checkViolation},
		{"email is unique",
			`INSERT INTO users (name, email, password_hash, role) VALUES ('n', 'student@example.test', 'x', 'STUDENT')`,
			uniqueViolation},
		{"role is one of the three",
			`INSERT INTO users (name, email, password_hash, role) VALUES ('n', 'b@example.test', 'x', 'OWNER')`,
			checkViolation},
		{"late period cannot end before the deadline",
			`INSERT INTO assignments (course_id, title, deadline_at, late_until_at)
				VALUES (1, 'A2', now(), now() - interval '1 day')`,
			checkViolation},
		{"output must fit twice into the tmpfs",
			`UPDATE resource_limits SET tmpfs_size_mb = 2, memory_mb = 64 WHERE assignment_id = 1`,
			checkViolation},
		{"tmpfs needs 16 MiB of headroom below memory",
			`UPDATE resource_limits SET memory_mb = 47 WHERE assignment_id = 1`,
			checkViolation},
		{"one resource limit per assignment",
			`INSERT INTO resource_limits (assignment_id, cpu_time_ms, memory_mb, max_processes, timeout_s,
				max_output_bytes, tmpfs_size_mb) VALUES (1, 2000, 64, 5, 5, 1048576, 32)`,
			uniqueViolation},
		{"external_id is unique per assignment",
			`INSERT INTO test_cases (assignment_id, external_id, input, expected_output, weight, match_mode)
				VALUES (1, 'tc_01', '', '', 10, 'EXACT')`,
			uniqueViolation},
		{"a test case with results cannot be deleted",
			`DELETE FROM test_cases WHERE id = 1`,
			restrictViolation},
		{"a course with assignments cannot be deleted",
			`DELETE FROM courses WHERE id = 1`,
			restrictViolation},
		{"a submission needs an existing user",
			`INSERT INTO submissions (user_id, assignment_id, code_path, original_filename, size_bytes)
				VALUES (999, 1, 'f1', 'main.py', 10)`,
			foreignKeyViolation},
		{"submission status is one of the five",
			`UPDATE submissions SET status = 'DONE' WHERE id = 1`,
			checkViolation},
		{"total_score cannot exceed max_score",
			`UPDATE submissions SET total_score = 30, max_score = 20 WHERE id = 1`,
			checkViolation},
		{"total_score and max_score are set together",
			`UPDATE submissions SET total_score = 20 WHERE id = 1`,
			checkViolation},
		{"result status is one of the seven",
			`UPDATE results SET status = 'OK' WHERE id = 1`,
			checkViolation},
		{"memory_source is one of the three",
			`UPDATE results SET memory_source = 'GUESS' WHERE id = 1`,
			checkViolation},
		{"one result per submission and test case",
			`INSERT INTO results (submission_id, testcase_id, status, exec_time_ms, memory_kb, memory_source, score)
				VALUES (1, 1, 'FAILED', 5, 1024, 'CGROUP_PEAK', 0)`,
			uniqueViolation},
		{"an audit entry names its user",
			`INSERT INTO audit_logs (user_id, event_type) VALUES (NULL, 'SANDBOX_VIOLATION')`,
			notNullViolation},
		{"settings holds one row",
			`INSERT INTO settings (id) VALUES (2)`,
			checkViolation},
		{"the settings row exists once",
			`INSERT INTO settings (id) VALUES (1)`,
			uniqueViolation},
		{"a rate limit cannot be negative",
			`UPDATE settings SET min_submission_interval_s = -1`,
			checkViolation},
		{"a rate limit has an upper bound",
			`UPDATE settings SET max_submissions_per_assignment = 10001`,
			checkViolation},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := conn.Exec(context.Background(), tc.sql)
			var pgErr *pgconn.PgError
			if !errors.As(err, &pgErr) {
				t.Fatalf("got %v, want SQLSTATE %s", err, tc.code)
			}
			if !matches(pgErr.Code, tc.code) {
				t.Fatalf("got SQLSTATE %s (%s), want %s", pgErr.Code, pgErr.Message, tc.code)
			}
		})
	}
}

func TestUpdatedAt(t *testing.T) {
	m, conn := newMigrator(t)
	if err := m.Up(); err != nil {
		t.Fatalf("up: %v", err)
	}
	seed(t, conn)
	ctx := context.Background()

	updatedAt := func() time.Time {
		var ts time.Time
		if err := conn.QueryRow(ctx, `SELECT updated_at FROM users WHERE id = 2`).Scan(&ts); err != nil {
			t.Fatalf("read updated_at: %v", err)
		}
		return ts
	}

	before := updatedAt()
	if _, err := conn.Exec(ctx, `UPDATE users SET name = name WHERE id = 2`); err != nil {
		t.Fatalf("no-op update: %v", err)
	}
	if got := updatedAt(); !got.Equal(before) {
		t.Errorf("a no-op update moved updated_at from %v to %v", before, got)
	}

	if _, err := conn.Exec(ctx, `UPDATE users SET name = 'Renamed' WHERE id = 2`); err != nil {
		t.Fatalf("update: %v", err)
	}
	if got := updatedAt(); !got.After(before) {
		t.Errorf("updated_at stayed at %v after a change", got)
	}
}
