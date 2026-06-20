package sqlitestore_test

import (
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
