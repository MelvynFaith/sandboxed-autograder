package settings_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/settings"
	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/testdb"
)

func setup(t *testing.T) (*settings.Store, *pgxpool.Pool, int64) {
	t.Helper()
	pool := testdb.Pool(t)
	var admin int64
	if err := pool.QueryRow(context.Background(),
		`INSERT INTO users (name, email, password_hash, role) VALUES ('A', 'a@example.test', 'x', 'ADMIN') RETURNING id`,
	).Scan(&admin); err != nil {
		t.Fatal(err)
	}
	return settings.NewStore(pool), pool, admin
}

func count(t *testing.T, pool *pgxpool.Pool, query string, args ...any) int {
	t.Helper()
	var n int
	if err := pool.QueryRow(context.Background(), query, args...).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

func TestGetReturnsTheValuesOfFR14(t *testing.T) {
	st, _, _ := setup(t)
	s, err := st.Get(context.Background())
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if s.MaxActiveSubmissions != 1 || s.MinSubmissionIntervalS != 30 || s.MaxSubmissionsPerAssignment != 30 {
		t.Fatalf("settings = %+v, want 1, 30, 30", s)
	}
}

func TestUpdateSavesAndLogsWhoChangedWhat(t *testing.T) {
	st, pool, admin := setup(t)
	ctx := context.Background()
	before, _ := st.Get(ctx)

	saved, err := st.Update(ctx, admin, settings.Settings{MaxActiveSubmissions: 2, MinSubmissionIntervalS: 0, MaxSubmissionsPerAssignment: 50})
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if saved.MaxActiveSubmissions != 2 || saved.MinSubmissionIntervalS != 0 || saved.MaxSubmissionsPerAssignment != 50 {
		t.Fatalf("saved = %+v", saved)
	}
	if got, _ := st.Get(ctx); got != saved {
		t.Errorf("Get = %+v, want %+v", got, saved)
	}
	if !saved.UpdatedAt.After(before.UpdatedAt) {
		t.Error("updated_at did not move")
	}

	var user int64
	var event, detail string
	if err := pool.QueryRow(ctx, `SELECT user_id, event_type, detail FROM audit_logs`).Scan(&user, &event, &detail); err != nil {
		t.Fatalf("audit row: %v", err)
	}
	want := "max_active_submissions 1 to 2, min_submission_interval_s 30 to 0, max_submissions_per_assignment 30 to 50"
	if user != admin || event != settings.EventSettingsChanged || detail != want {
		t.Errorf("audit = %d, %s, %q", user, event, detail)
	}
}

func TestUpdateRejectsOutOfRangeValuesAndChangesNothing(t *testing.T) {
	st, pool, admin := setup(t)
	good := settings.Settings{MaxActiveSubmissions: 1, MinSubmissionIntervalS: 30, MaxSubmissionsPerAssignment: 30}
	for name, mutate := range map[string]func(*settings.Settings){
		"negative active":     func(s *settings.Settings) { s.MaxActiveSubmissions = -1 },
		"huge active":         func(s *settings.Settings) { s.MaxActiveSubmissions = 10001 },
		"negative interval":   func(s *settings.Settings) { s.MinSubmissionIntervalS = -1 },
		"interval over a day": func(s *settings.Settings) { s.MinSubmissionIntervalS = 86401 },
		"negative quota":      func(s *settings.Settings) { s.MaxSubmissionsPerAssignment = -5 },
		"huge quota":          func(s *settings.Settings) { s.MaxSubmissionsPerAssignment = 10001 },
	} {
		t.Run(name, func(t *testing.T) {
			next := good
			mutate(&next)
			if _, err := st.Update(context.Background(), admin, next); !errors.Is(err, settings.ErrInvalid) {
				t.Fatalf("err = %v, want ErrInvalid", err)
			}
		})
	}
	if n := count(t, pool, `SELECT count(*) FROM audit_logs`); n != 0 {
		t.Errorf("%d audit rows after rejected updates", n)
	}
	if got, _ := st.Get(context.Background()); got.MaxActiveSubmissions != 1 || got.MinSubmissionIntervalS != 30 {
		t.Errorf("settings changed to %+v", got)
	}
}

func TestUpdateByAnUnknownUserRollsBack(t *testing.T) {
	st, pool, _ := setup(t)
	_, err := st.Update(context.Background(), 424242, settings.Settings{MaxActiveSubmissions: 5, MinSubmissionIntervalS: 5, MaxSubmissionsPerAssignment: 5})
	if err == nil {
		t.Fatal("Update succeeded for a user that does not exist")
	}
	if got, _ := st.Get(context.Background()); got.MaxActiveSubmissions != 1 {
		t.Errorf("settings changed to %+v although the audit row failed", got)
	}
	if n := count(t, pool, `SELECT count(*) FROM audit_logs`); n != 0 {
		t.Errorf("%d audit rows", n)
	}
}
