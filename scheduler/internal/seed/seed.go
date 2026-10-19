// Package seed creates the first administrator account (FR-19, SRS 13.2).
//
// Registration always yields a STUDENT, and only an administrator can grant the LECTURER and ADMIN
// roles, so a fresh deployment has nobody who can appoint a lecturer. The deployment therefore runs
// `autograder seed-admin` once, with the password in a one-time environment variable; it is never an
// HTTP endpoint. Once an administrator exists the seed refuses, so the command cannot become a second
// way in: further administrators are appointed through the admin API, which audits role changes.
package seed

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/MelvynFaith/sandboxed-autograder/scheduler/internal/auth"
)

// EventAdminSeeded is the audit log event written with the new account.
const EventAdminSeeded = "ADMIN_SEEDED"

// Limits on the input. Name and e-mail follow the registration endpoint. The password must be longer
// than a registration's minimum because this account can change every other one; the upper bound
// keeps Argon2id's input small.
const (
	maxNameLength     = 200
	maxEmailLength    = 254
	minPasswordLength = 12
	maxPasswordLength = 1024
)

var (
	// ErrAdminExists means an administrator already exists; nothing was written.
	ErrAdminExists = errors.New("seed: an administrator already exists")

	// ErrEmailTaken means another account already uses the e-mail address; nothing was written.
	ErrEmailTaken = errors.New("seed: the e-mail address is already registered")

	// ErrInvalid means the name, e-mail address or password is not acceptable.
	ErrInvalid = errors.New("seed: invalid input")
)

// FirstAdmin creates an ADMIN account and its audit log row in one transaction and returns its id. It
// returns ErrAdminExists if any administrator exists, and ErrEmailTaken if the address is in use, so
// an existing account is never changed. The e-mail address is stored trimmed and in lower case, as
// registration stores it.
func FirstAdmin(ctx context.Context, pool *pgxpool.Pool, name, email, password string) (int64, error) {
	email = strings.ToLower(strings.TrimSpace(email))
	if err := validate(name, email, password); err != nil {
		return 0, err
	}
	// Hash before the transaction: Argon2id takes a noticeable moment and needs no lock.
	hash, err := auth.HashPassword(password)
	if err != nil {
		return 0, fmt.Errorf("seed: hash password: %w", err)
	}

	var id int64
	err = pgx.BeginFunc(ctx, pool, func(tx pgx.Tx) error {
		// Two seeds started at once would both see no administrator; the lock makes them take turns.
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended('seed:first-admin', 0))`); err != nil {
			return err
		}
		var admins, sameEmail bool
		if err := tx.QueryRow(ctx, `SELECT
			EXISTS (SELECT 1 FROM users WHERE role = 'ADMIN'),
			EXISTS (SELECT 1 FROM users WHERE email = $1)`, email,
		).Scan(&admins, &sameEmail); err != nil {
			return err
		}
		switch {
		case admins:
			return ErrAdminExists
		case sameEmail:
			return ErrEmailTaken
		}
		if err := tx.QueryRow(ctx, `
			INSERT INTO users (name, email, password_hash, role) VALUES ($1, $2, $3, 'ADMIN')
			RETURNING id`, name, email, hash).Scan(&id); err != nil {
			return err
		}
		detail, err := json.Marshal(struct {
			Email string `json:"email"`
		}{email})
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO audit_logs (user_id, event_type, detail) VALUES ($1, $2, $3)`,
			id, EventAdminSeeded, string(detail))
		return err
	})
	if errors.Is(err, ErrAdminExists) || errors.Is(err, ErrEmailTaken) {
		return 0, err
	}
	if err != nil {
		return 0, fmt.Errorf("seed: create administrator: %w", err)
	}
	return id, nil
}

// validate applies the registration endpoint's rules to name and e-mail, and a stricter one to the
// password.
func validate(name, email, password string) error {
	if name == "" || utf8.RuneCountInString(name) > maxNameLength {
		return fmt.Errorf("%w: the name must have 1 to %d characters", ErrInvalid, maxNameLength)
	}
	local, domain, ok := strings.Cut(email, "@")
	if len(email) > maxEmailLength || !ok || local == "" || strings.Contains(domain, "@") ||
		!strings.Contains(domain, ".") || strings.HasPrefix(domain, ".") || strings.HasSuffix(domain, ".") {
		return fmt.Errorf("%w: %q is not an e-mail address", ErrInvalid, email)
	}
	if utf8.RuneCountInString(password) < minPasswordLength || len(password) > maxPasswordLength {
		return fmt.Errorf("%w: the password must have %d to %d characters", ErrInvalid, minPasswordLength, maxPasswordLength)
	}
	return nil
}
