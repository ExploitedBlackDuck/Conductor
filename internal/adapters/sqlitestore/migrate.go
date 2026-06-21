package sqlitestore

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"sort"
	"strconv"
	"strings"

	"github.com/conductor-app/conductor/internal/core/coreerr"
	"github.com/conductor-app/conductor/migrations"
)

// ExpectedSchemaVersion is the schema version this binary was built against. It
// equals the highest embedded migration number and is asserted at Open so a
// binary never runs against a newer or partially-migrated database.
const ExpectedSchemaVersion = 6

// migration is one parsed, embedded SQL migration.
type migration struct {
	version int
	name    string
	sql     string
}

// migrate applies every embedded migration whose version exceeds the database's
// current version, in ascending order, each in its own transaction. It is
// idempotent: re-running applies nothing.
func migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS schema_migrations (
			version    INTEGER PRIMARY KEY,
			applied_at TEXT    NOT NULL
		)`); err != nil {
		return coreerr.New(coreerr.CodeStoreMigration, "creating schema_migrations", err)
	}

	current, err := schemaVersion(ctx, db)
	if err != nil {
		return err
	}

	all, err := loadMigrations()
	if err != nil {
		return err
	}

	for _, m := range all {
		if m.version <= current {
			continue
		}
		if err := applyMigration(ctx, db, m); err != nil {
			return err
		}
	}
	return nil
}

// applyMigration runs one migration and records its version atomically.
func applyMigration(ctx context.Context, db *sql.DB, m migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return coreerr.New(coreerr.CodeStoreMigration, fmt.Sprintf("beginning migration %d", m.version), err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, m.sql); err != nil {
		return coreerr.New(coreerr.CodeStoreMigration, fmt.Sprintf("applying migration %04d_%s", m.version, m.name), err)
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO schema_migrations (version, applied_at) VALUES (?, datetime('now'))`, m.version); err != nil {
		return coreerr.New(coreerr.CodeStoreMigration, fmt.Sprintf("recording migration %d", m.version), err)
	}
	if err := tx.Commit(); err != nil {
		return coreerr.New(coreerr.CodeStoreMigration, fmt.Sprintf("committing migration %d", m.version), err)
	}
	return nil
}

// schemaVersion returns the highest applied migration version, or 0 when none
// have been applied.
func schemaVersion(ctx context.Context, db *sql.DB) (int, error) {
	var version sql.NullInt64
	err := db.QueryRowContext(ctx, `SELECT MAX(version) FROM schema_migrations`).Scan(&version)
	if err != nil {
		return 0, coreerr.New(coreerr.CodeStoreMigration, "reading schema version", err)
	}
	if !version.Valid {
		return 0, nil
	}
	return int(version.Int64), nil
}

// loadMigrations parses and sorts the embedded migration files. Filenames must
// be NNNN_description.sql.
func loadMigrations() ([]migration, error) {
	entries, err := fs.ReadDir(migrations.FS(), ".")
	if err != nil {
		return nil, coreerr.New(coreerr.CodeStoreMigration, "reading embedded migrations", err)
	}

	var out []migration
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		version, name, err := parseMigrationName(e.Name())
		if err != nil {
			return nil, err
		}
		body, err := fs.ReadFile(migrations.FS(), e.Name())
		if err != nil {
			return nil, coreerr.New(coreerr.CodeStoreMigration, "reading migration "+e.Name(), err)
		}
		out = append(out, migration{version: version, name: name, sql: string(body)})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].version < out[j].version })

	// Guard against duplicate or gapped versions, which would make ordering
	// ambiguous.
	for i, m := range out {
		if m.version != i+1 {
			return nil, coreerr.New(coreerr.CodeStoreMigration,
				fmt.Sprintf("migration versions must be contiguous from 1; got %d at position %d", m.version, i+1), nil)
		}
	}
	return out, nil
}

// parseMigrationName splits "0001_audit_log.sql" into (1, "audit_log").
func parseMigrationName(filename string) (int, string, error) {
	base := strings.TrimSuffix(filename, ".sql")
	idx := strings.IndexByte(base, '_')
	if idx <= 0 {
		return 0, "", coreerr.New(coreerr.CodeStoreMigration, "malformed migration filename "+filename, nil)
	}
	version, err := strconv.Atoi(base[:idx])
	if err != nil {
		return 0, "", coreerr.New(coreerr.CodeStoreMigration, "migration filename must start with a number: "+filename, err)
	}
	return version, base[idx+1:], nil
}
