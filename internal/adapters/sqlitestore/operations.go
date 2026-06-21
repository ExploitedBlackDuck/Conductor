package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/conductor-app/conductor/internal/core/domain"
)

// InsertOperation persists a completed operation, its resolved options, and its
// sealed captured log atomically. The log is already sealed by the caller
// (ADR-0009); the store never sees plaintext.
func (s *Store) InsertOperation(ctx context.Context, op domain.Operation, opts []domain.OperationOption, log *domain.CapturedLog) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning operation transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var endedAt any
	if !op.EndedAt.IsZero() {
		endedAt = op.EndedAt.UTC().Format(timeLayout)
	}
	var logRef any
	if op.LogRef != "" {
		logRef = op.LogRef
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO operations
		 (id, kind, src, dst, rclone_version, intensity, started_at, ended_at, bytes_moved, files_moved, result, log_blob_id)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		op.ID, string(op.Kind), op.Src, op.Dst, op.RcloneVersion, op.Intensity,
		op.StartedAt.UTC().Format(timeLayout), endedAt, op.BytesMoved, op.FilesMoved, string(op.Result), logRef,
	); err != nil {
		return fmt.Errorf("inserting operation %s: %w", op.ID, err)
	}

	for _, o := range opts {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO operation_options (operation_id, flag, value, risk, acknowledged)
			 VALUES (?, ?, ?, ?, ?)`,
			op.ID, o.Flag, o.Value, o.Risk, o.Acknowledged,
		); err != nil {
			return fmt.Errorf("inserting option %s for %s: %w", o.Flag, op.ID, err)
		}
	}

	if log != nil {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO log_blobs (id, operation_id, nonce, sealed_bytes, sha256_plaintext, bytes_len)
			 VALUES (?, ?, ?, ?, ?, ?)`,
			log.ID, op.ID, log.Nonce, log.SealedBytes, log.SHA256Plaintext, log.BytesLen,
		); err != nil {
			return fmt.Errorf("inserting captured log for %s: %w", op.ID, err)
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing operation %s: %w", op.ID, err)
	}
	return nil
}

// InsertChangeSet persists the sealed dry-run change set an operation was
// confirmed against (ADR-0015). The path lists are already sealed by the caller
// (ADR-0009); the store never sees them in the clear. One row per operation.
func (s *Store) InsertChangeSet(ctx context.Context, cs domain.ChangeSetRecord) error {
	if _, err := s.db.ExecContext(
		ctx,
		`INSERT INTO change_sets
		 (operation_id, create_count, update_count, delete_count, truncated,
		  acknowledged_at, nonce, sealed_bytes, sha256_plaintext, bytes_len)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		cs.OperationID, cs.CreateCount, cs.UpdateCount, cs.DeleteCount, cs.Truncated,
		cs.AcknowledgedAt.UTC().Format(timeLayout), cs.Nonce, cs.SealedBytes, cs.SHA256Plaintext, cs.BytesLen,
	); err != nil {
		return fmt.Errorf("inserting change set for %s: %w", cs.OperationID, err)
	}
	return nil
}

// ChangeSetFor returns the sealed change set persisted for an operation, if any.
// The bool is false when the operation has no recorded change set.
func (s *Store) ChangeSetFor(ctx context.Context, operationID string) (domain.ChangeSetRecord, bool, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT operation_id, create_count, update_count, delete_count, truncated,
		        acknowledged_at, nonce, sealed_bytes, sha256_plaintext, bytes_len
		 FROM change_sets WHERE operation_id = ?`, operationID)
	var (
		cs    domain.ChangeSetRecord
		ackAt string
	)
	err := row.Scan(&cs.OperationID, &cs.CreateCount, &cs.UpdateCount, &cs.DeleteCount, &cs.Truncated,
		&ackAt, &cs.Nonce, &cs.SealedBytes, &cs.SHA256Plaintext, &cs.BytesLen)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.ChangeSetRecord{}, false, nil
	}
	if err != nil {
		return domain.ChangeSetRecord{}, false, fmt.Errorf("reading change set for %s: %w", operationID, err)
	}
	at, err := time.Parse(timeLayout, ackAt)
	if err != nil {
		return domain.ChangeSetRecord{}, false, fmt.Errorf("parsing acknowledged_at %q: %w", ackAt, err)
	}
	cs.AcknowledgedAt = at
	return cs, true, nil
}

// OperationByID returns one operation and its options. The bool is false when no
// operation with that id exists.
func (s *Store) OperationByID(ctx context.Context, id string) (domain.Operation, []domain.OperationOption, bool, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, kind, src, dst, rclone_version, intensity, started_at, ended_at, bytes_moved, files_moved, result, log_blob_id
		 FROM operations WHERE id = ?`, id)
	op, err := scanOperation(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Operation{}, nil, false, nil
	}
	if err != nil {
		return domain.Operation{}, nil, false, err
	}
	opts, err := s.operationOptions(ctx, id)
	if err != nil {
		return domain.Operation{}, nil, false, err
	}
	return op, opts, true, nil
}

// OperationLog returns the sealed captured log for an operation, if present.
func (s *Store) OperationLog(ctx context.Context, operationID string) (domain.CapturedLog, bool, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, operation_id, nonce, sealed_bytes, sha256_plaintext, bytes_len
		 FROM log_blobs WHERE operation_id = ?`, operationID)
	var log domain.CapturedLog
	err := row.Scan(&log.ID, &log.OperationID, &log.Nonce, &log.SealedBytes, &log.SHA256Plaintext, &log.BytesLen)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.CapturedLog{}, false, nil
	}
	if err != nil {
		return domain.CapturedLog{}, false, fmt.Errorf("reading captured log for %s: %w", operationID, err)
	}
	return log, true, nil
}

func (s *Store) operationOptions(ctx context.Context, operationID string) ([]domain.OperationOption, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT flag, value, risk, acknowledged FROM operation_options WHERE operation_id = ? ORDER BY flag`, operationID)
	if err != nil {
		return nil, fmt.Errorf("querying operation options: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []domain.OperationOption
	for rows.Next() {
		var o domain.OperationOption
		if err := rows.Scan(&o.Flag, &o.Value, &o.Risk, &o.Acknowledged); err != nil {
			return nil, fmt.Errorf("scanning operation option: %w", err)
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating operation options: %w", err)
	}
	return out, nil
}

func scanOperation(s scanner) (domain.Operation, error) {
	var (
		op      domain.Operation
		kind    string
		result  string
		started string
		ended   sql.NullString
		logBlob sql.NullString
	)
	if err := s.Scan(&op.ID, &kind, &op.Src, &op.Dst, &op.RcloneVersion, &op.Intensity,
		&started, &ended, &op.BytesMoved, &op.FilesMoved, &result, &logBlob); err != nil {
		return domain.Operation{}, err
	}
	op.Kind = domain.OperationKind(kind)
	op.Result = domain.Result(result)
	startedAt, err := time.Parse(timeLayout, started)
	if err != nil {
		return domain.Operation{}, fmt.Errorf("parsing started_at %q: %w", started, err)
	}
	op.StartedAt = startedAt
	if ended.Valid {
		endedAt, err := time.Parse(timeLayout, ended.String)
		if err != nil {
			return domain.Operation{}, fmt.Errorf("parsing ended_at %q: %w", ended.String, err)
		}
		op.EndedAt = endedAt
	}
	if logBlob.Valid {
		op.LogRef = logBlob.String
	}
	return op, nil
}
