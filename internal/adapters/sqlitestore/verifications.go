package sqlitestore

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/conductor-app/conductor/internal/core/domain"
)

// InsertVerification persists an integrity-check result (§7.12). Only counts and
// the verdict are stored; the offending paths are shown live, not persisted.
func (s *Store) InsertVerification(ctx context.Context, v domain.Verification) error {
	var endedAt any
	if !v.EndedAt.IsZero() {
		endedAt = v.EndedAt.UTC().Format(timeLayout)
	}
	if _, err := s.db.ExecContext(
		ctx,
		`INSERT INTO verifications
		 (id, kind, src, dst, started_at, ended_at, match_count, differ, missing, error_count, result)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		v.ID, string(v.Kind), v.Src, v.Dst, v.StartedAt.UTC().Format(timeLayout), endedAt,
		v.Match, v.Differ, v.Missing, v.ErrorCount, string(v.Result),
	); err != nil {
		return fmt.Errorf("inserting verification %s: %w", v.ID, err)
	}
	return nil
}

// Verifications returns the most recent verifications, newest first.
func (s *Store) Verifications(ctx context.Context, limit int) ([]domain.Verification, error) {
	if limit <= 0 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, kind, src, dst, started_at, ended_at, match_count, differ, missing, error_count, result
		 FROM verifications ORDER BY started_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("querying verifications: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []domain.Verification
	for rows.Next() {
		v, err := scanVerification(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating verifications: %w", err)
	}
	return out, nil
}

func scanVerification(sc scanner) (domain.Verification, error) {
	var (
		v       domain.Verification
		kind    string
		result  string
		started string
		ended   sql.NullString
	)
	if err := sc.Scan(&v.ID, &kind, &v.Src, &v.Dst, &started, &ended,
		&v.Match, &v.Differ, &v.Missing, &v.ErrorCount, &result); err != nil {
		return domain.Verification{}, err
	}
	v.Kind = domain.VerificationKind(kind)
	v.Result = domain.VerifyResult(result)
	at, err := time.Parse(timeLayout, started)
	if err != nil {
		return domain.Verification{}, fmt.Errorf("parsing verification started_at %q: %w", started, err)
	}
	v.StartedAt = at
	if ended.Valid {
		e, err := time.Parse(timeLayout, ended.String)
		if err != nil {
			return domain.Verification{}, fmt.Errorf("parsing verification ended_at %q: %w", ended.String, err)
		}
		v.EndedAt = e
	}
	return v, nil
}
