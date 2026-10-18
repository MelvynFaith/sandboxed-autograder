package seed_test

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/auth"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/seed"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/testdb"
)

const password = "a long enough password"

func count(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatalf("%v\n%s", err, query)
	}
	return n
}

func TestFirstAdmin(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()

	id, err := seed.FirstAdmin(ctx, pool, "Admin", "  Admin@Example.TEST ", password)
	if err != nil {
		t.Fatal(err)
	}

	var name, email, role, hash string
	if err := pool.QueryRow(ctx, `SELECT name, email, role, password_hash FROM users WHERE id = $1`, id).
		Scan(&name, &email, &role, &hash); err != nil {
		t.Fatal(err)
	}
	if name != "Admin" || email != "admin@example.test" || role != "ADMIN" {
		t.Errorf("stored %q %q %q, want Admin admin@example.test ADMIN", name, email, role)
	}
	// The hash is the API's own Argon2id format, so the administrator can log in through the API.
	if ok, err := auth.VerifyPassword(hash, password); err != nil || !ok {
		t.Errorf("the stored hash does not verify the password: %v, %v", ok, err)
	}
	if strings.Contains(hash, password) {
		t.Error("the password is stored in clear")
	}
	if n := count(t, pool, `SELECT count(*) FROM audit_logs WHERE user_id = $1 AND event_type = $2
		AND detail LIKE '%admin@example.test%'`, id, seed.EventAdminSeeded); n != 1 {
		t.Errorf("%d ADMIN_SEEDED audit rows, want 1", n)
	}
}

// Once an administrator exists the seed is not a second way to make one.
func TestRefusesOnceAnAdminExists(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	if _, err := seed.FirstAdmin(ctx, pool, "Admin", "admin@example.test", password); err != nil {
		t.Fatal(err)
	}
	if _, err := seed.FirstAdmin(ctx, pool, "Other", "other@example.test", password); !errors.Is(err, seed.ErrAdminExists) {
		t.Fatalf("second seed: %v, want ErrAdminExists", err)
	}
	if n := count(t, pool, `SELECT count(*) FROM users`); n != 1 {
		t.Fatalf("%d users after a refused seed, want 1", n)
	}
}

// An address that a student registered is not taken over and turned into an administrator.
func TestLeavesAnExistingAccountAlone(t *testing.T) {
	pool := testdb.Pool(t)
	ctx := context.Background()
	if _, err := pool.Exec(ctx, `INSERT INTO users (name, email, password_hash, role)
		VALUES ('Student', 'admin@example.test', 'x', 'STUDENT')`); err != nil {
		t.Fatal(err)
	}
	if _, err := seed.FirstAdmin(ctx, pool, "Admin", "ADMIN@example.test", password); !errors.Is(err, seed.ErrEmailTaken) {
		t.Fatalf("seed over a student: %v, want ErrEmailTaken", err)
	}
	if n := count(t, pool, `SELECT count(*) FROM users WHERE role = 'ADMIN' OR password_hash <> 'x'`); n != 0 {
		t.Fatal("the existing account was changed")
	}
}

func TestRejectsBadInput(t *testing.T) {
	pool := testdb.Pool(t)
	for name, in := range map[string][3]string{
		"empty name":          {"", "admin@example.test", password},
		"long name":           {strings.Repeat("n", 201), "admin@example.test", password},
		"no @":                {"Admin", "admin.example.test", password},
		"two @":               {"Admin", "a@b@example.test", password},
		"no dot in domain":    {"Admin", "admin@localhost", password},
		"short password":      {"Admin", "admin@example.test", "elevenchars"},
		"oversized password":  {"Admin", "admin@example.test", strings.Repeat("p", 1025)},
		"twelve bytes, fewer": {"Admin", "admin@example.test", "ééééééa"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := seed.FirstAdmin(context.Background(), pool, in[0], in[1], in[2]); !errors.Is(err, seed.ErrInvalid) {
				t.Fatalf("got %v, want ErrInvalid", err)
			}
		})
	}
	if n := count(t, pool, `SELECT count(*) FROM users`); n != 0 {
		t.Fatalf("%d users after rejected input, want 0", n)
	}
}

// Two seeds started together, for different addresses, make exactly one administrator.
func TestConcurrentSeedsMakeOneAdmin(t *testing.T) {
	pool := testdb.Pool(t)
	const n = 4
	errs := make([]error, n)
	var wg sync.WaitGroup
	for i := range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = seed.FirstAdmin(context.Background(), pool, "Admin", strings.Repeat("a", i+1)+"@example.test", password)
		}()
	}
	wg.Wait()
	created := 0
	for _, err := range errs {
		switch {
		case err == nil:
			created++
		case !errors.Is(err, seed.ErrAdminExists):
			t.Errorf("unexpected error: %v", err)
		}
	}
	if created != 1 || count(t, pool, `SELECT count(*) FROM users WHERE role = 'ADMIN'`) != 1 {
		t.Fatalf("%d seeds succeeded; want exactly one administrator", created)
	}
}
