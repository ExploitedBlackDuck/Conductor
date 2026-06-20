package sqlitestore

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/conductor-app/conductor/internal/core/domain"
)

// This file implements the saved-pair, profile, and per-remote ceiling
// persistence ports backing the pairs/governance services (§7.5–7.7, ADR-0013).

// SavePair inserts or updates a saved sync/bisync pair. It writes every column
// from the struct, including last_run_at, so an edit must load-modify-save to
// preserve a prior run timestamp; TouchPairRun updates only that timestamp.
func (s *Store) SavePair(ctx context.Context, p domain.SavedPair) error {
	if _, err := s.db.ExecContext(
		ctx,
		`INSERT INTO saved_pairs (id, name, kind, path1, path2, profile_id, last_run_at)
		 VALUES (?, ?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET
		   name = excluded.name, kind = excluded.kind, path1 = excluded.path1,
		   path2 = excluded.path2, profile_id = excluded.profile_id,
		   last_run_at = excluded.last_run_at`,
		p.ID, p.Name, string(p.Kind), p.Path1, p.Path2, nullString(p.ProfileID), nullTime(p.LastRun),
	); err != nil {
		return fmt.Errorf("saving pair %s: %w", p.ID, err)
	}
	return nil
}

// TouchPairRun records that a pair has run by stamping last_run_at. After the
// first run a pair is no longer "new", so its next run is live by default (§7.4).
func (s *Store) TouchPairRun(ctx context.Context, id string, at time.Time) error {
	res, err := s.db.ExecContext(ctx,
		`UPDATE saved_pairs SET last_run_at = ? WHERE id = ?`, at.UTC().Format(timeLayout), id)
	if err != nil {
		return fmt.Errorf("touching pair %s: %w", id, err)
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return fmt.Errorf("touching pair %s: no such pair", id)
	}
	return nil
}

// Pair returns one saved pair. The bool is false when no pair has that id.
func (s *Store) Pair(ctx context.Context, id string) (domain.SavedPair, bool, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT id, name, kind, path1, path2, profile_id, last_run_at FROM saved_pairs WHERE id = ?`, id)
	p, err := scanPair(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.SavedPair{}, false, nil
	}
	if err != nil {
		return domain.SavedPair{}, false, err
	}
	return p, true, nil
}

// Pairs returns all saved pairs ordered by name.
func (s *Store) Pairs(ctx context.Context) ([]domain.SavedPair, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, name, kind, path1, path2, profile_id, last_run_at FROM saved_pairs ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("querying pairs: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []domain.SavedPair
	for rows.Next() {
		p, err := scanPair(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating pairs: %w", err)
	}
	return out, nil
}

// DeletePair removes a saved pair.
func (s *Store) DeletePair(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM saved_pairs WHERE id = ?`, id); err != nil {
		return fmt.Errorf("deleting pair %s: %w", id, err)
	}
	return nil
}

func scanPair(sc scanner) (domain.SavedPair, error) {
	var (
		p         domain.SavedPair
		kind      string
		profileID sql.NullString
		lastRun   sql.NullString
	)
	if err := sc.Scan(&p.ID, &p.Name, &kind, &p.Path1, &p.Path2, &profileID, &lastRun); err != nil {
		return domain.SavedPair{}, err
	}
	p.Kind = domain.PairKind(kind)
	if profileID.Valid {
		p.ProfileID = profileID.String
	}
	if lastRun.Valid {
		t, err := time.Parse(timeLayout, lastRun.String)
		if err != nil {
			return domain.SavedPair{}, fmt.Errorf("parsing last_run_at %q: %w", lastRun.String, err)
		}
		p.LastRun = t
	}
	return p, nil
}

// SaveProfile inserts or updates a named option profile and replaces its options
// atomically.
func (s *Store) SaveProfile(ctx context.Context, p domain.Profile) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("beginning profile transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO profiles (id, name, kind) VALUES (?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET name = excluded.name, kind = excluded.kind`,
		p.ID, p.Name, string(p.Kind),
	); err != nil {
		return fmt.Errorf("saving profile %s: %w", p.ID, err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM profile_options WHERE profile_id = ?`, p.ID); err != nil {
		return fmt.Errorf("clearing options for profile %s: %w", p.ID, err)
	}
	for _, o := range p.Options {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT INTO profile_options (profile_id, flag, value) VALUES (?, ?, ?)`,
			p.ID, o.Flag, o.Value,
		); err != nil {
			return fmt.Errorf("saving option %s for profile %s: %w", o.Flag, p.ID, err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("committing profile %s: %w", p.ID, err)
	}
	return nil
}

// Profile returns one profile with its options. The bool is false when no
// profile has that id.
func (s *Store) Profile(ctx context.Context, id string) (domain.Profile, bool, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id, name, kind FROM profiles WHERE id = ?`, id)
	var (
		p    domain.Profile
		kind string
	)
	err := row.Scan(&p.ID, &p.Name, &kind)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.Profile{}, false, nil
	}
	if err != nil {
		return domain.Profile{}, false, fmt.Errorf("reading profile %s: %w", id, err)
	}
	p.Kind = domain.OperationKind(kind)
	opts, err := s.profileOptions(ctx, id)
	if err != nil {
		return domain.Profile{}, false, err
	}
	p.Options = opts
	return p, true, nil
}

// Profiles returns all profiles (without their options) ordered by name.
func (s *Store) Profiles(ctx context.Context) ([]domain.Profile, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, name, kind FROM profiles ORDER BY name`)
	if err != nil {
		return nil, fmt.Errorf("querying profiles: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []domain.Profile
	for rows.Next() {
		var (
			p    domain.Profile
			kind string
		)
		if err := rows.Scan(&p.ID, &p.Name, &kind); err != nil {
			return nil, fmt.Errorf("scanning profile: %w", err)
		}
		p.Kind = domain.OperationKind(kind)
		out = append(out, p)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating profiles: %w", err)
	}
	return out, nil
}

// DeleteProfile removes a profile; its options cascade, and any saved pair
// referencing it has its profile_id set to NULL (ON DELETE SET NULL).
func (s *Store) DeleteProfile(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, `DELETE FROM profiles WHERE id = ?`, id); err != nil {
		return fmt.Errorf("deleting profile %s: %w", id, err)
	}
	return nil
}

func (s *Store) profileOptions(ctx context.Context, profileID string) ([]domain.ProfileOption, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT flag, value FROM profile_options WHERE profile_id = ? ORDER BY flag`, profileID)
	if err != nil {
		return nil, fmt.Errorf("querying profile options: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []domain.ProfileOption
	for rows.Next() {
		var o domain.ProfileOption
		if err := rows.Scan(&o.Flag, &o.Value); err != nil {
			return nil, fmt.Errorf("scanning profile option: %w", err)
		}
		out = append(out, o)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating profile options: %w", err)
	}
	return out, nil
}

// SetCeiling inserts or updates the governance ceiling for a remote (§7.6).
func (s *Store) SetCeiling(ctx context.Context, c domain.RemoteCeiling) error {
	if _, err := s.db.ExecContext(
		ctx,
		`INSERT INTO remote_ceilings (remote, transfers, checkers, bwlimit, tpslimit)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(remote) DO UPDATE SET
		   transfers = excluded.transfers, checkers = excluded.checkers,
		   bwlimit = excluded.bwlimit, tpslimit = excluded.tpslimit`,
		c.Remote, c.Transfers, c.Checkers, c.Bwlimit, c.Tpslimit,
	); err != nil {
		return fmt.Errorf("setting ceiling for %s: %w", c.Remote, err)
	}
	return nil
}

// Ceiling returns the saved ceiling for a remote. The bool is false when the
// remote has none.
func (s *Store) Ceiling(ctx context.Context, remote string) (domain.RemoteCeiling, bool, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT remote, transfers, checkers, bwlimit, tpslimit FROM remote_ceilings WHERE remote = ?`, remote)
	c, err := scanCeiling(row)
	if errors.Is(err, sql.ErrNoRows) {
		return domain.RemoteCeiling{}, false, nil
	}
	if err != nil {
		return domain.RemoteCeiling{}, false, fmt.Errorf("reading ceiling for %s: %w", remote, err)
	}
	return c, true, nil
}

// Ceilings returns all per-remote ceilings ordered by remote.
func (s *Store) Ceilings(ctx context.Context) ([]domain.RemoteCeiling, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT remote, transfers, checkers, bwlimit, tpslimit FROM remote_ceilings ORDER BY remote`)
	if err != nil {
		return nil, fmt.Errorf("querying ceilings: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []domain.RemoteCeiling
	for rows.Next() {
		c, err := scanCeiling(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterating ceilings: %w", err)
	}
	return out, nil
}

func scanCeiling(sc scanner) (domain.RemoteCeiling, error) {
	var c domain.RemoteCeiling
	if err := sc.Scan(&c.Remote, &c.Transfers, &c.Checkers, &c.Bwlimit, &c.Tpslimit); err != nil {
		return domain.RemoteCeiling{}, err
	}
	return c, nil
}

// nullString maps "" to a SQL NULL, so an absent profile reference is stored as
// NULL rather than an empty foreign key (which would violate the constraint).
func nullString(s string) any {
	if s == "" {
		return nil
	}
	return s
}

// nullTime maps a zero time to SQL NULL; a non-zero time is stored in UTC.
func nullTime(t time.Time) any {
	if t.IsZero() {
		return nil
	}
	return t.UTC().Format(timeLayout)
}
