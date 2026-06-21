package sqlitestore_test

import (
	"bytes"
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/conductor-app/conductor/internal/adapters/sqlitestore"
	"github.com/conductor-app/conductor/internal/core/audit"
	"github.com/conductor-app/conductor/internal/core/domain"
)

func openTemp(t *testing.T) *sqlitestore.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "conductor.db")
	store, err := sqlitestore.Open(context.Background(), path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	return store
}

func TestOpenRunsMigrationsAndVerifiesVersion(t *testing.T) {
	t.Parallel()
	store := openTemp(t)

	version, err := store.SchemaVersion(context.Background())
	require.NoError(t, err)
	assert.Equal(t, sqlitestore.ExpectedSchemaVersion, version)
}

func TestOpenIsIdempotent(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "conductor.db")

	first, err := sqlitestore.Open(ctx, path)
	require.NoError(t, err)
	require.NoError(t, first.Close())

	// Reopening an already-migrated database applies nothing and still matches.
	second, err := sqlitestore.Open(ctx, path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = second.Close() })

	version, err := second.SchemaVersion(ctx)
	require.NoError(t, err)
	assert.Equal(t, sqlitestore.ExpectedSchemaVersion, version)
}

// fixedClock is a deterministic, advancing clock.
type fixedClock struct{ t time.Time }

func (c *fixedClock) Now() time.Time {
	c.t = c.t.Add(time.Second)
	return c.t
}

func TestAuditChainPersistsAndVerifiesOnRealSQLite(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := openTemp(t)

	svc := audit.New(store, &fixedClock{t: time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)})

	for i := 0; i < 6; i++ {
		_, err := svc.Record(ctx, domain.ActionOperationStart, "op-1", map[string]int{"i": i})
		require.NoError(t, err)
	}

	entries, err := store.Entries(ctx)
	require.NoError(t, err)
	require.Len(t, entries, 6)
	assert.Equal(t, int64(1), entries[0].Seq)
	assert.Empty(t, entries[0].PrevHash)
	assert.Equal(t, entries[0].Hash, entries[1].PrevHash)

	res, err := svc.Verify(ctx)
	require.NoError(t, err)
	assert.True(t, res.Intact)
	assert.Equal(t, 6, res.Count)
}

func TestSignedHeadPersistsAndVerifiesOnRealSQLite(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "conductor.db")
	store, err := sqlitestore.Open(ctx, path)
	require.NoError(t, err)

	key := bytes.Repeat([]byte{0x77}, 32)
	svc := audit.New(store, &fixedClock{t: time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)}, audit.WithSigningKey(key))
	for i := 0; i < 4; i++ {
		_, rerr := svc.Record(ctx, domain.ActionOperationStart, "op", map[string]int{"i": i})
		require.NoError(t, rerr)
	}
	anchor, err := svc.SignHead(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(4), anchor.Seq)

	// The anchor round-trips through the real store and the signed head verifies.
	got, ok, err := store.LatestAnchor(ctx)
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, anchor.HeadHash, got.HeadHash)
	res, err := svc.Verify(ctx)
	require.NoError(t, err)
	assert.True(t, res.Trustworthy())
	assert.True(t, res.SignatureValid)
	require.NoError(t, store.Close())

	// Tamper the signed head on disk to forge a different head; the signature is
	// not re-derivable without the key, so verification catches it on reopen.
	raw, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	_, err = raw.ExecContext(ctx, `UPDATE audit_anchors SET head_hash = 'deadbeef'`)
	require.NoError(t, err)
	require.NoError(t, raw.Close())

	reopened, err := sqlitestore.Open(ctx, path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = reopened.Close() })
	svc2 := audit.New(reopened, &fixedClock{t: time.Date(2026, 6, 21, 13, 0, 0, 0, time.UTC)}, audit.WithSigningKey(key))
	res2, err := svc2.Verify(ctx)
	require.NoError(t, err)
	assert.True(t, res2.Intact, "the entry chain itself is untouched")
	assert.True(t, res2.HeadSigned)
	assert.False(t, res2.SignatureValid, "a forged signed head is caught")
	assert.False(t, res2.Trustworthy())
}

func TestAuditTamperOnDiskBreaksVerification(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "conductor.db")

	store, err := sqlitestore.Open(ctx, path)
	require.NoError(t, err)

	svc := audit.New(store, &fixedClock{t: time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC)})
	for i := 0; i < 4; i++ {
		_, err := svc.Record(ctx, domain.ActionDestructiveConfirmed, "op-2", map[string]int{"i": i})
		require.NoError(t, err)
	}
	require.NoError(t, store.Close())

	// Tamper directly on disk, bypassing the store's append-only path.
	raw, err := sql.Open("sqlite", path)
	require.NoError(t, err)
	_, err = raw.ExecContext(ctx, `UPDATE audit_log SET subject = 'op-evil' WHERE seq = 2`)
	require.NoError(t, err)
	require.NoError(t, raw.Close())

	// Reopen and verify: the chain must now report tampering.
	reopened, err := sqlitestore.Open(ctx, path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = reopened.Close() })

	svc2 := audit.New(reopened, &fixedClock{t: time.Now()})
	res, err := svc2.Verify(ctx)
	require.NoError(t, err)
	assert.False(t, res.Intact, "on-disk tampering must be detected")
	assert.Equal(t, int64(2), res.BrokenAtSeq)
}
