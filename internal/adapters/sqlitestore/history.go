package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/conductor-app/conductor/internal/core/domain"
)

// This file implements the intention-revealing history queries the workflow
// needs (§7.7, §7.11.7). The UI never composes SQL; it calls these named
// queries. They return operations without their options — the detail view loads
// options via OperationByID.

const operationColumns = `id, kind, src, dst, rclone_version, server_side, intensity,
	started_at, ended_at, bytes_moved, files_moved, result, log_blob_id`

// RecentOperations returns the most recent operations, newest first, capped at
// limit (a limit <= 0 applies a sane default).
func (s *Store) RecentOperations(ctx context.Context, limit int) ([]domain.Operation, error) {
	if limit <= 0 {
		limit = 200
	}
	return s.queryOperations(ctx,
		`SELECT `+operationColumns+` FROM operations ORDER BY started_at DESC LIMIT ?`, limit)
}

// OperationsByRemote returns operations whose source or destination is on the
// given remote, newest first.
func (s *Store) OperationsByRemote(ctx context.Context, remote string) ([]domain.Operation, error) {
	like := remote + ":%"
	return s.queryOperations(ctx,
		`SELECT `+operationColumns+` FROM operations
		 WHERE src LIKE ? OR dst LIKE ? ORDER BY started_at DESC`, like, like)
}

// OperationsInRange returns operations started within [from, to), newest first.
func (s *Store) OperationsInRange(ctx context.Context, from, to time.Time) ([]domain.Operation, error) {
	return s.queryOperations(ctx,
		`SELECT `+operationColumns+` FROM operations
		 WHERE started_at >= ? AND started_at < ? ORDER BY started_at DESC`,
		from.UTC().Format(timeLayout), to.UTC().Format(timeLayout))
}

// DestructiveOperations returns operations that deleted or could overwrite data:
// destructive kinds (sync/delete/purge) or any operation that carried an
// acknowledged destructive option (e.g. bisync --resync), newest first (§7.4).
func (s *Store) DestructiveOperations(ctx context.Context) ([]domain.Operation, error) {
	return s.queryOperations(ctx,
		`SELECT `+operationColumns+` FROM operations
		 WHERE kind IN ('sync', 'delete', 'purge')
		    OR id IN (SELECT operation_id FROM operation_options WHERE acknowledged = 1)
		 ORDER BY started_at DESC`)
}

// LastRunForPair returns the most recent operation matching a saved pair's
// endpoints. The bool is false when the pair has never produced an operation.
func (s *Store) LastRunForPair(ctx context.Context, path1, path2 string) (domain.Operation, bool, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT `+operationColumns+` FROM operations
		 WHERE src = ? AND dst = ? ORDER BY started_at DESC LIMIT 1`, path1, path2)
	op, err := scanOperation(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Operation{}, false, nil
	}
	if err != nil {
		return domain.Operation{}, false, err
	}
	return op, true, nil
}

// ClearHistory deletes all operations and their sealed logs and options (the
// "clear history" action, §7.11.7). The audit log is append-only and is never
// touched here. Returns the number of operations removed.
func (s *Store) ClearHistory(ctx context.Context) (int64, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("beginning clear-history transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	// operation_options and log_blobs cascade from operations via foreign keys,
	// but delete them explicitly so this does not depend on cascade being on.
	if _, err := tx.ExecContext(ctx, `DELETE FROM log_blobs`); err != nil {
		return 0, fmt.Errorf("clearing log blobs: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM change_sets`); err != nil {
		return 0, fmt.Errorf("clearing change sets: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM operation_options`); err != nil {
		return 0, fmt.Errorf("clearing operation options: %w", err)
	}
	res, err := tx.ExecContext(ctx, `DELETE FROM operations`)
	if err != nil {
		return 0, fmt.Errorf("clearing operations: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("committing clear-history: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// ReconcileRunningOperations closes any operation still marked running — these
// were orphaned by an unclean exit, since rclone jobs die with the daemon
// (§2.3, ADR-0005). They are stamped interrupted with the given end time.
// Returns the number reconciled (zero on a clean restart).
func (s *Store) ReconcileRunningOperations(ctx context.Context, at time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`UPDATE operations SET result = ?, ended_at = ?
		 WHERE result = ?`,
		string(domain.ResultInterrupted), at.UTC().Format(timeLayout), string(domain.ResultRunning))
	if err != nil {
		return 0, fmt.Errorf("reconciling running operations: %w", err)
	}
	n, _ := res.RowsAffected()
	return n, nil
}

// queryOperations runs a SELECT returning the standard operation columns and
// scans every row.
func (s *Store) queryOperations(ctx context.Context, query string, args ...any) ([]domain.Operation, error) {
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("querying operations: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []domain.Operation
	for rows.Next() {
		op, err := scanOperation(rows)
		if err != nil {
			return nil, fmt.Errorf("scanning operation: %w", err)
		}
		out = append(out, op)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating operations: %w", err)
	}
	return out, nil
}
