// Package migrations embeds the schema migrations, so the scheduler binary carries the SQL it was
// built with. Files are named NNNNNN_name.up.sql and NNNNNN_name.down.sql for golang-migrate; every
// up migration has a down migration.
package migrations

import "embed"

// FS holds the migration files at its root.
//
//go:embed *.sql
var FS embed.FS
