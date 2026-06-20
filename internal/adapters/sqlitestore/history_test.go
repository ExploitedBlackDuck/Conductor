package sqlitestore_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/conductor-app/conductor/internal/adapters/sqlitestore"
	"github.com/conductor-app/conductor/internal/core/domain"
)

// seedOp inserts a minimal completed operation.
func seedOp(t *testing.T, store *sqlitestore.Store, id string, kind domain.OperationKind, src, dst string, at time.Time, opts ...domain.OperationOption) {
	t.Helper()
	op := domain.Operation{
		ID: id, Kind: kind, Src: src, Dst: dst, RcloneVersion: "v1.74.3",
		StartedAt: at, EndedAt: at.Add(time.Minute), Result: domain.ResultSuccess,
	}
	require.NoError(t, store.InsertOperation(context.Background(), op, opts, nil))
}

func TestHistoryQueries(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := openTemp(t)

	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	seedOp(t, store, "op-copy", domain.KindCopy, "s3:data", "/local/a", base)
	seedOp(t, store, "op-sync", domain.KindSync, "/local/b", "s3:mirror", base.Add(24*time.Hour))
	seedOp(t, store, "op-move", domain.KindMove, "b2:x", "b2:y", base.Add(48*time.Hour))
	// A bisync carrying an acknowledged destructive option (resync).
	seedOp(t, store, "op-bisync", domain.KindBisync, "/home", "gdrive:bak", base.Add(72*time.Hour),
		domain.OperationOption{Flag: "--resync", Value: "true", Risk: "destructive", Acknowledged: true})

	t.Run("recent is newest-first", func(t *testing.T) {
		ops, err := store.RecentOperations(ctx, 10)
		require.NoError(t, err)
		require.Len(t, ops, 4)
		assert.Equal(t, "op-bisync", ops[0].ID)
		assert.Equal(t, "op-copy", ops[3].ID)
	})

	t.Run("by remote matches src or dst", func(t *testing.T) {
		ops, err := store.OperationsByRemote(ctx, "s3")
		require.NoError(t, err)
		require.Len(t, ops, 2)
		ids := []string{ops[0].ID, ops[1].ID}
		assert.Contains(t, ids, "op-copy")
		assert.Contains(t, ids, "op-sync")
	})

	t.Run("in range is half-open", func(t *testing.T) {
		ops, err := store.OperationsInRange(ctx, base.Add(24*time.Hour), base.Add(72*time.Hour))
		require.NoError(t, err)
		require.Len(t, ops, 2) // sync (day 1) and move (day 2); bisync (day 3) excluded
		assert.Equal(t, "op-move", ops[0].ID)
		assert.Equal(t, "op-sync", ops[1].ID)
	})

	t.Run("destructive covers kinds and acknowledged options", func(t *testing.T) {
		ops, err := store.DestructiveOperations(ctx)
		require.NoError(t, err)
		ids := map[string]bool{}
		for _, o := range ops {
			ids[o.ID] = true
		}
		assert.True(t, ids["op-sync"], "sync is destructive by kind")
		assert.True(t, ids["op-bisync"], "bisync with acknowledged --resync is destructive")
		assert.False(t, ids["op-copy"], "copy is not destructive")
		assert.False(t, ids["op-move"], "move is not destructive")
	})

	t.Run("last run for pair matches endpoints", func(t *testing.T) {
		op, ok, err := store.LastRunForPair(ctx, "/home", "gdrive:bak")
		require.NoError(t, err)
		require.True(t, ok)
		assert.Equal(t, "op-bisync", op.ID)

		_, ok, err = store.LastRunForPair(ctx, "/never", "remote:run")
		require.NoError(t, err)
		assert.False(t, ok)
	})
}

func TestClearHistoryRemovesRowsNotAudit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := openTemp(t)

	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	seedOp(t, store, "op-1", domain.KindCopy, "s3:a", "/b", base,
		domain.OperationOption{Flag: "--checksum", Value: "true", Risk: "passive"})

	n, err := store.ClearHistory(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(1), n)

	ops, err := store.RecentOperations(ctx, 10)
	require.NoError(t, err)
	assert.Empty(t, ops)

	_, _, found, err := store.OperationByID(ctx, "op-1")
	require.NoError(t, err)
	assert.False(t, found, "options and operation are gone")
}
