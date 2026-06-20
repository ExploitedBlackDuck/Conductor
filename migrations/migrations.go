// Package migrations holds Conductor's forward-only, embedded SQL migrations
// (ADR-0007) and exposes them as an fs.FS for the sqlitestore adapter to apply.
// Migrations are named NNNN_description.sql; NNNN is the schema version applied
// in ascending order. They are never edited once shipped — schema changes are
// new files.
package migrations

import "embed"

//go:embed *.sql
var files embed.FS

// FS returns the embedded migration files.
func FS() embed.FS {
	return files
}
