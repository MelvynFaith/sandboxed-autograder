// Package settings reads and changes the settings an administrator controls: the submission rate
// limits of FR-14 (US-20). They live in a single row of the settings table.
package settings

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// EventSettingsChanged is the audit log event written when an administrator saves new settings.
const EventSettingsChanged = "SETTINGS_CHANGED"

// Bounds the schema also enforces. A value of 0 switches a rule off.
const (
	maxCount    = 10000
	maxInterval = 86400
)

// ErrInvalid means a value is outside its bounds. Nothing was saved.
var ErrInvalid = errors.New("settings: value out of range")

// Settings are the rate limit thresholds. The JSON names follow the column names.
type Settings struct {
	MaxActiveSubmissions        int       `json:"max_active_submissions"`
	MinSubmissionIntervalS      int       `json:"min_submission_interval_s"`
	MaxSubmissionsPerAssignment int       `json:"max_submissions_per_assignment"`
	UpdatedAt                   time.Time `json:"updated_at"`
}

// Validate checks the bounds, with a message the API can show to the administrator.
func (s Settings) Validate() error {
	switch {
	case s.MaxActiveSubmissions < 0 || s.MaxActiveSubmissions > maxCount:
		return fmt.Errorf("%w: max_active_submissions must be 0 to %d", ErrInvalid, maxCount)
	case s.MinSubmissionIntervalS < 0 || s.MinSubmissionIntervalS > maxInterval:
		return fmt.Errorf("%w: min_submission_interval_s must be 0 to %d", ErrInvalid, maxInterval)
	case s.MaxSubmissionsPerAssignment < 0 || s.MaxSubmissionsPerAssignment > maxCount:
		return fmt.Errorf("%w: max_submissions_per_assignment must be 0 to %d", ErrInvalid, maxCount)
	}
	return nil
}

// Store reads and writes the settings row. It is safe for concurrent use.
type Store struct {
	pool *pgxpool.Pool
}

// NewStore returns a Store that uses pool.
func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

const selectSQL = `
	SELECT max_active_submissions, min_submission_interval_s, max_submissions_per_assignment, updated_at
	FROM settings WHERE id = 1`

func scan(row pgx.Row) (Settings, error) {
	var s Settings
	err := row.Scan(&s.MaxActiveSubmissions, &s.MinSubmissionIntervalS, &s.MaxSubmissionsPerAssignment, &s.UpdatedAt)
	return s, err
}

// Get returns the current settings. The row is created by the migration, so it always exists.
func (st *Store) Get(ctx context.Context) (Settings, error) {
	s, err := scan(st.pool.QueryRow(ctx, selectSQL))
	if err != nil {
		return Settings{}, fmt.Errorf("settings: read: %w", err)
	}
	return s, nil
}

// Update saves new settings on behalf of the administrator actor and returns what was stored. The
// change and an audit log row naming the actor and the old and new values are written in one
// transaction. It returns ErrInvalid, having changed nothing, if a value is out of range.
func (st *Store) Update(ctx context.Context, actorID int64, next Settings) (Settings, error) {
	if err := next.Validate(); err != nil {
		return Settings{}, err
	}
	var saved Settings
	err := pgx.BeginFunc(ctx, st.pool, func(tx pgx.Tx) error {
		// FOR UPDATE keeps two concurrent saves from logging the same "old" values.
		old, err := scan(tx.QueryRow(ctx, selectSQL+` FOR UPDATE`))
		if err != nil {
			return err
		}
		saved, err = scan(tx.QueryRow(ctx, `
			UPDATE settings
			SET max_active_submissions = $1, min_submission_interval_s = $2, max_submissions_per_assignment = $3
			WHERE id = 1
			RETURNING max_active_submissions, min_submission_interval_s, max_submissions_per_assignment, updated_at`,
			next.MaxActiveSubmissions, next.MinSubmissionIntervalS, next.MaxSubmissionsPerAssignment))
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `
			INSERT INTO audit_logs (user_id, event_type, detail) VALUES ($1, $2, $3)`,
			actorID, EventSettingsChanged, fmt.Sprintf(
				"max_active_submissions %d to %d, min_submission_interval_s %d to %d, max_submissions_per_assignment %d to %d",
				old.MaxActiveSubmissions, saved.MaxActiveSubmissions,
				old.MinSubmissionIntervalS, saved.MinSubmissionIntervalS,
				old.MaxSubmissionsPerAssignment, saved.MaxSubmissionsPerAssignment))
		return err
	})
	if err != nil {
		return Settings{}, fmt.Errorf("settings: update: %w", err)
	}
	return saved, nil
}
