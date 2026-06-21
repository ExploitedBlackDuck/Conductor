// Package sqlitestore is the SQLite adapter implementing Conductor's persistence
// ports (ADR-0007). It is the only place SQL lives; core packages depend on
// small interfaces (e.g. audit.Store) that this package satisfies. The driver
// is the pure-Go modernc.org/sqlite, so cross-compilation stays CGO-free.
package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	_ "modernc.org/sqlite" // registers the "sqlite" database/sql driver

	"github.com/conductor-app/conductor/internal/core/coreerr"
	"github.com/conductor-app/conductor/internal/core/domain"
)

// timeLayout is how timestamps are stored and read back. It must match the
// audit package's layout so the chain hash round-trips exactly.
const timeLayout = "2006-01-02T15:04:05.000000000Z07:00"

// Store is a SQLite-backed implementation of the core persistence ports.
type Store struct {
	db *sql.DB
}

// Open opens (creating if absent) the SQLite database at path, applies all
// pending migrations, and verifies the resulting schema version matches the
// version the binary expects. A migration or version-check failure yields
// ERR_STORE_MIGRATION (§8.4).
func Open(ctx context.Context, path string) (*Store, error) {
	// Pragmas: enforce foreign keys, wait on locks rather than failing
	// immediately, and use WAL for better concurrent read/write behaviour.
	dsn := path + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)"
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, coreerr.New(coreerr.CodeStoreMigration, "opening database", err)
	}
	// modernc's driver is not safe for concurrent writers on one *DB handle in
	// WAL with multiple connections writing; cap to a single connection to keep
	// write ordering simple for a single-operator app.
	db.SetMaxOpenConns(1)

	if err := db.PingContext(ctx); err != nil {
		_ = db.Close()
		return nil, coreerr.New(coreerr.CodeStoreMigration, "connecting to database", err)
	}

	if err := migrate(ctx, db); err != nil {
		_ = db.Close()
		return nil, err
	}

	current, err := schemaVersion(ctx, db)
	if err != nil {
		_ = db.Close()
		return nil, err
	}
	if current != ExpectedSchemaVersion {
		_ = db.Close()
		return nil, coreerr.New(coreerr.CodeStoreMigration,
			fmt.Sprintf("schema version %d does not match expected %d", current, ExpectedSchemaVersion), nil)
	}

	return &Store{db: db}, nil
}

// Close releases the database handle.
func (s *Store) Close() error {
	if err := s.db.Close(); err != nil {
		return fmt.Errorf("closing database: %w", err)
	}
	return nil
}

// SchemaVersion returns the highest applied migration version.
func (s *Store) SchemaVersion(ctx context.Context) (int, error) {
	return schemaVersion(ctx, s.db)
}

// AppendEntry implements audit.Store: it reads the current chain tail and
// inserts the built entry within one transaction, so the chain cannot fork.
func (s *Store) AppendEntry(ctx context.Context, build func(prev *domain.AuditEntry) (domain.AuditEntry, error)) (domain.AuditEntry, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return domain.AuditEntry{}, fmt.Errorf("beginning audit transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }() // no-op after a successful Commit

	prev, err := lastEntryTx(ctx, tx)
	if err != nil {
		return domain.AuditEntry{}, err
	}

	entry, err := build(prev)
	if err != nil {
		return domain.AuditEntry{}, fmt.Errorf("building audit entry: %w", err)
	}

	_, err = tx.ExecContext(
		ctx,
		`INSERT INTO audit_log (seq, at, action, subject, detail, prev_hash, hash)
		 VALUES (?, ?, ?, ?, ?, ?, ?)`,
		entry.Seq, entry.At.UTC().Format(timeLayout), string(entry.Action), entry.Subject,
		string(entry.Detail), entry.PrevHash, entry.Hash,
	)
	if err != nil {
		return domain.AuditEntry{}, fmt.Errorf("inserting audit entry %d: %w", entry.Seq, err)
	}

	if err := tx.Commit(); err != nil {
		return domain.AuditEntry{}, fmt.Errorf("committing audit entry %d: %w", entry.Seq, err)
	}
	return entry, nil
}

// Entries implements audit.Store: it returns all entries in ascending Seq order.
func (s *Store) Entries(ctx context.Context) ([]domain.AuditEntry, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT seq, at, action, subject, detail, prev_hash, hash
		 FROM audit_log ORDER BY seq ASC`)
	if err != nil {
		return nil, fmt.Errorf("querying audit entries: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []domain.AuditEntry
	for rows.Next() {
		e, scanErr := scanEntry(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		out = append(out, e)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating audit entries: %w", err)
	}
	return out, nil
}

// InsertAnchor appends a signed chain head (ADR-0010). Anchors are append-only;
// the newest is the current signed head.
func (s *Store) InsertAnchor(ctx context.Context, a domain.AuditAnchor) error {
	if _, err := s.db.ExecContext(
		ctx,
		`INSERT INTO audit_anchors (seq, head_hash, signature, signed_at) VALUES (?, ?, ?, ?)`,
		a.Seq, a.HeadHash, a.Signature, a.SignedAt.UTC().Format(timeLayout),
	); err != nil {
		return fmt.Errorf("inserting audit anchor at seq %d: %w", a.Seq, err)
	}
	return nil
}

// LatestAnchor returns the most recently signed chain head, or false when none
// has been signed yet.
func (s *Store) LatestAnchor(ctx context.Context) (domain.AuditAnchor, bool, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT seq, head_hash, signature, signed_at FROM audit_anchors
		 ORDER BY signed_at DESC, seq DESC LIMIT 1`)
	var (
		a        domain.AuditAnchor
		signedAt string
	)
	err := row.Scan(&a.Seq, &a.HeadHash, &a.Signature, &signedAt)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.AuditAnchor{}, false, nil
	}
	if err != nil {
		return domain.AuditAnchor{}, false, fmt.Errorf("reading latest audit anchor: %w", err)
	}
	at, err := time.Parse(timeLayout, signedAt)
	if err != nil {
		return domain.AuditAnchor{}, false, fmt.Errorf("parsing anchor signed_at %q: %w", signedAt, err)
	}
	a.SignedAt = at
	return a, true, nil
}

// lastEntryTx returns the tail entry within a transaction, or nil when the log
// is empty.
func lastEntryTx(ctx context.Context, tx *sql.Tx) (*domain.AuditEntry, error) {
	row := tx.QueryRowContext(ctx,
		`SELECT seq, at, action, subject, detail, prev_hash, hash
		 FROM audit_log ORDER BY seq DESC LIMIT 1`)
	e, err := scanEntry(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil //nolint:nilnil // (entry, ok) is expressed as (*entry, error); nil,nil means "empty log"
	}
	if err != nil {
		return nil, err
	}
	return &e, nil
}

// scanner is satisfied by both *sql.Row and *sql.Rows.
type scanner interface {
	Scan(dest ...any) error
}

func scanEntry(s scanner) (domain.AuditEntry, error) {
	var (
		e      domain.AuditEntry
		atStr  string
		action string
		detail string
	)
	if err := s.Scan(&e.Seq, &atStr, &action, &e.Subject, &detail, &e.PrevHash, &e.Hash); err != nil {
		return domain.AuditEntry{}, err
	}
	at, err := time.Parse(timeLayout, atStr)
	if err != nil {
		return domain.AuditEntry{}, fmt.Errorf("parsing audit timestamp %q: %w", atStr, err)
	}
	e.At = at
	e.Action = domain.AuditAction(action)
	e.Detail = []byte(detail)
	return e, nil
}
