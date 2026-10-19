package retention_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/retention"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/testdb"
)

// fixture is a database with a student, an assignment that closed two years ago and one that
// closed half a year ago, and a code directory for their submissions' files.
type fixture struct {
	pool    *pgxpool.Pool
	codeDir string
	student int64
	old     int64
	recent  int64
}

// cutoff is what the command passes: one year before now.
var cutoff = time.Now().AddDate(-1, 0, 0)

func newFixture(t *testing.T) *fixture {
	t.Helper()
	f := &fixture{pool: testdb.Pool(t), codeDir: filepath.Join(t.TempDir(), "code")}
	if err := os.Mkdir(f.codeDir, 0o700); err != nil {
		t.Fatal(err)
	}
	f.student = f.id(t, `INSERT INTO users (name, email, password_hash, role)
		VALUES ('Student', 'student@example.test', 'x', 'STUDENT') RETURNING id`)
	lecturer := f.id(t, `INSERT INTO users (name, email, password_hash, role)
		VALUES ('Lecturer', 'lecturer@example.test', 'x', 'LECTURER') RETURNING id`)
	course := f.id(t, `INSERT INTO courses (code, name, lecturer_id, term)
		VALUES ('C1', 'Course', $1, '2024-1') RETURNING id`, lecturer)
	f.old = f.id(t, `INSERT INTO assignments (course_id, title, deadline_at, late_until_at)
		VALUES ($1, 'Old', now() - interval '2 years 1 day', now() - interval '2 years') RETURNING id`, course)
	f.recent = f.id(t, `INSERT INTO assignments (course_id, title, deadline_at, late_until_at)
		VALUES ($1, 'Recent', now() - interval '6 months 1 day', now() - interval '6 months')
		RETURNING id`, course)
	return f
}

func (f *fixture) id(t *testing.T, sql string, args ...any) int64 {
	t.Helper()
	var id int64
	if err := f.pool.QueryRow(context.Background(), sql, args...).Scan(&id); err != nil {
		t.Fatalf("%v\n%s", err, sql)
	}
	return id
}

// submission stores a submission of the student with the given status and code_path, and writes its
// file when withFile is set. A COMPLETED one gets a grade of 80/100.
func (f *fixture) submission(t *testing.T, assignment int64, status, path string, withFile bool) int64 {
	t.Helper()
	var score any
	if status == "COMPLETED" {
		score = 80
	}
	id := f.id(t, `INSERT INTO submissions (user_id, assignment_id, code_path, original_filename,
		size_bytes, status, total_score, max_score)
		VALUES ($1, $2, $3, 'main.py', 1, $4, $5, CASE WHEN $5::int IS NULL THEN NULL ELSE 100 END)
		RETURNING id`, f.student, assignment, path, status, score)
	if withFile {
		f.write(t, filepath.Join(f.codeDir, path))
	}
	return id
}

func (f *fixture) write(t *testing.T, name string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(name), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte("print(1)\n"), 0o600); err != nil {
		t.Fatal(err)
	}
}

// deleted reports whether the row of submission id has code_deleted_at set.
func (f *fixture) deleted(t *testing.T, id int64) bool {
	t.Helper()
	var at *time.Time
	if err := f.pool.QueryRow(context.Background(),
		`SELECT code_deleted_at FROM submissions WHERE id = $1`, id).Scan(&at); err != nil {
		t.Fatal(err)
	}
	return at != nil
}

func exists(t *testing.T, name string) bool {
	t.Helper()
	_, err := os.Stat(name)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return err == nil
}

func TestExpireDeletesOldFilesAndKeepsTheGrades(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	completed := f.submission(t, f.old, "COMPLETED", "a.py", true)
	failed := f.submission(t, f.old, "FAILED", "b.py", true)
	cancelled := f.submission(t, f.old, "CANCELLED", "c.py", true)
	recent := f.submission(t, f.recent, "COMPLETED", "d.py", true)
	queued := f.submission(t, f.old, "QUEUED", "e.py", true)
	running := f.submission(t, f.old, "RUNNING", "f.py", true)
	missing := f.submission(t, f.old, "COMPLETED", "g.py", false)
	tc := f.id(t, `INSERT INTO test_cases (assignment_id, external_id, input, expected_output, weight,
		match_mode) VALUES ($1, 'tc_01', '', '', 100, 'EXACT') RETURNING id`, f.old)
	f.id(t, `INSERT INTO results (submission_id, testcase_id, status, actual_output, exec_time_ms,
		memory_kb, memory_source, score) VALUES ($1, $2, 'FAILED', '41', 5, 1024, 'CGROUP_PEAK', 80)
		RETURNING id`, completed, tc)

	if n, err := retention.Pending(ctx, f.pool, cutoff); err != nil || n != 4 {
		t.Fatalf("Pending: %d, %v; want 4, nil", n, err)
	}
	r, err := retention.Expire(ctx, f.pool, f.codeDir, cutoff)
	if err != nil {
		t.Fatalf("Expire: %v", err)
	}
	if want := (retention.Report{Deleted: 3, Missing: 1}); r != want {
		t.Fatalf("report %+v, want %+v", r, want)
	}

	for _, c := range []struct {
		name    string
		id      int64
		path    string
		removed bool
	}{
		{"completed", completed, "a.py", true},
		{"failed", failed, "b.py", true},
		{"cancelled", cancelled, "c.py", true},
		{"closed less than a year ago", recent, "d.py", false},
		{"queued", queued, "e.py", false},
		{"running", running, "f.py", false},
		{"file already gone", missing, "g.py", true},
	} {
		if got := exists(t, filepath.Join(f.codeDir, c.path)); got == c.removed {
			t.Errorf("%s: file exists %t, want %t", c.name, got, !c.removed)
		}
		if got := f.deleted(t, c.id); got != c.removed {
			t.Errorf("%s: code_deleted_at set %t, want %t", c.name, got, c.removed)
		}
	}

	var score int
	var output string
	if err := f.pool.QueryRow(ctx, `SELECT s.total_score, r.actual_output FROM submissions s
		JOIN results r ON r.submission_id = s.id WHERE s.id = $1`, completed).Scan(&score, &output); err != nil {
		t.Fatalf("grade after expiry: %v", err)
	}
	if score != 80 || output != "41" {
		t.Fatalf("grade after expiry: score %d, output %q; want 80, \"41\"", score, output)
	}

	if r, err := retention.Expire(ctx, f.pool, f.codeDir, cutoff); err != nil || r != (retention.Report{}) {
		t.Fatalf("second Expire: %+v, %v; want nothing to do", r, err)
	}
	if n, err := retention.Pending(ctx, f.pool, cutoff); err != nil || n != 0 {
		t.Fatalf("Pending after Expire: %d, %v; want 0, nil", n, err)
	}
}

// A stored path is server-generated, so one that leaves the code directory means a bug or a
// tampered row: it is never followed, and its row stays unmarked so that someone looks at it.
func TestExpireNeverLeavesTheCodeDirectory(t *testing.T) {
	f := newFixture(t)
	parent := filepath.Dir(f.codeDir)
	outside := filepath.Join(parent, "outside.py")
	absolute := filepath.Join(parent, "absolute.py")
	f.write(t, outside)
	f.write(t, absolute)
	climbing := f.submission(t, f.old, "COMPLETED", "../outside.py", false)
	rooted := f.submission(t, f.old, "COMPLETED", absolute, false)
	normal := f.submission(t, f.old, "COMPLETED", "a.py", true)

	r, err := retention.Expire(context.Background(), f.pool, f.codeDir, cutoff)
	if err == nil || !strings.Contains(err.Error(), "leaves the code directory") {
		t.Fatalf("Expire error %v, want the paths refused", err)
	}
	if want := (retention.Report{Deleted: 1, Failed: 2}); r != want {
		t.Fatalf("report %+v, want %+v", r, want)
	}
	if !exists(t, outside) || !exists(t, absolute) {
		t.Fatal("a file outside the code directory was deleted")
	}
	if f.deleted(t, climbing) || f.deleted(t, rooted) || !f.deleted(t, normal) {
		t.Fatal("only the submission inside the code directory may be marked")
	}
}

func TestExpireLeavesARowWhoseFileCannotBeDeleted(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root ignores the directory permissions this test relies on")
	}
	// One row per batch: the unmarked row must not be selected again in the next batch.
	retention.SetBatchSize(t, 1)
	f := newFixture(t)
	stuck := f.submission(t, f.old, "COMPLETED", "locked/a.py", true)
	later := f.submission(t, f.old, "COMPLETED", "b.py", true)
	locked := filepath.Join(f.codeDir, "locked")
	if err := os.Chmod(locked, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(locked, 0o700) })

	r, err := retention.Expire(context.Background(), f.pool, f.codeDir, cutoff)
	if !errors.Is(err, os.ErrPermission) {
		t.Fatalf("Expire error %v, want a permission error", err)
	}
	if want := (retention.Report{Deleted: 1, Failed: 1}); r != want {
		t.Fatalf("report %+v, want %+v", r, want)
	}
	if f.deleted(t, stuck) || !exists(t, filepath.Join(locked, "a.py")) {
		t.Fatal("the file that could not be deleted must keep its row unmarked")
	}
	if !f.deleted(t, later) {
		t.Fatal("a failure must not stop the submissions after it")
	}
}

func TestExpireWorksInBatches(t *testing.T) {
	retention.SetBatchSize(t, 2)
	f := newFixture(t)
	var ids []int64
	for _, name := range []string{"a.py", "b.py", "c.py", "d.py", "e.py"} {
		ids = append(ids, f.submission(t, f.old, "COMPLETED", name, true))
	}
	r, err := retention.Expire(context.Background(), f.pool, f.codeDir, cutoff)
	if err != nil || r != (retention.Report{Deleted: 5}) {
		t.Fatalf("Expire: %+v, %v; want 5 deleted", r, err)
	}
	for _, id := range ids {
		if !f.deleted(t, id) {
			t.Fatalf("submission %d not marked", id)
		}
	}
}

// A regrade that queues the submission while Expire holds its row waits, and then finds the row
// marked; a row that someone else holds is skipped until the next run.
func TestExpireSkipsALockedRow(t *testing.T) {
	f := newFixture(t)
	ctx := context.Background()
	held := f.submission(t, f.old, "COMPLETED", "a.py", true)
	free := f.submission(t, f.old, "COMPLETED", "b.py", true)

	tx, err := f.pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT 1 FROM submissions WHERE id = $1 FOR UPDATE`, held); err != nil {
		t.Fatal(err)
	}

	r, err := retention.Expire(ctx, f.pool, f.codeDir, cutoff)
	if err != nil || r != (retention.Report{Deleted: 1}) {
		t.Fatalf("Expire: %+v, %v; want 1 deleted", r, err)
	}
	if f.deleted(t, held) || !exists(t, filepath.Join(f.codeDir, "a.py")) || !f.deleted(t, free) {
		t.Fatal("only the free row may be expired")
	}
	if err := tx.Rollback(ctx); err != nil && !errors.Is(err, pgx.ErrTxClosed) {
		t.Fatal(err)
	}
	if r, err := retention.Expire(ctx, f.pool, f.codeDir, cutoff); err != nil || r != (retention.Report{Deleted: 1}) {
		t.Fatalf("Expire after the lock is gone: %+v, %v; want 1 deleted", r, err)
	}
}
