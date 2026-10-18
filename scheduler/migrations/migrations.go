// Package migrations embeds the schema migrations, so the scheduler binary carries the SQL it was
// built with. Files are named NNNNNN_name.up.sql and NNNNNN_name.down.sql for golang-migrate; every
// up migration has a down migration.
package migrations

import (
	"embed"
	"errors"
	"fmt"
	"net/url"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// FS holds the migration files at its root.
//
//go:embed *.sql
var FS embed.FS

// Up applies the migrations the database does not have yet and returns the schema version it is at.
// databaseURL is a postgres:// or postgresql:// URL. golang-migrate takes a lock in the database, so
// two processes starting at once do not both migrate.
//
// A database left dirty by a migration that failed half-way is an error: it needs a person to look at
// it and run `migrate force`, not a restart.
func Up(databaseURL string) (uint, error) {
	// The url.Parse error quotes the whole URL, password included, so it is not passed on.
	u, err := url.Parse(databaseURL)
	if err != nil || (u.Scheme != "postgres" && u.Scheme != "postgresql") {
		return 0, errors.New("migrations: the database address is not a postgres:// URL")
	}
	u.Scheme = "pgx5" // the scheme golang-migrate's pgx v5 driver registers

	src, err := iofs.New(FS, ".")
	if err != nil {
		return 0, fmt.Errorf("migrations: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, u.String())
	if err != nil {
		return 0, fmt.Errorf("migrations: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return 0, fmt.Errorf("migrations: up: %w", err)
	}
	version, _, err := m.Version()
	if err != nil {
		return 0, fmt.Errorf("migrations: read version: %w", err)
	}
	return version, nil
}
