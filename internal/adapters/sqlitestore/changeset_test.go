package sqlitestore_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/chacha20poly1305"

	"github.com/conductor-app/conductor/internal/core/domain"
	"github.com/conductor-app/conductor/internal/core/secrets"
)

func TestChangeSetSealedRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := openTemp(t)

	// A change set must hang off a real operation row (FK).
	seedOp(t, store, "op-cs", domain.KindSync, "/src", "s3:dst", time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC))

	sealer, err := secrets.NewSealer(bytes.Repeat([]byte{0x44}, chacha20poly1305.KeySize))
	require.NoError(t, err)

	plaintext := []byte(`{"deletes":["secret/path.txt"]}`)
	sealed, err := sealer.Seal(plaintext, []byte("op-cs"))
	require.NoError(t, err)

	rec := domain.ChangeSetRecord{
		OperationID: "op-cs", CreateCount: 3, UpdateCount: 1, DeleteCount: 1, Truncated: false,
		AcknowledgedAt: time.Date(2026, 6, 21, 11, 59, 0, 0, time.UTC),
		Nonce:          sealed.Nonce, SealedBytes: sealed.Ciphertext, BytesLen: len(plaintext),
	}
	require.NoError(t, store.InsertChangeSet(ctx, rec))

	got, ok, err := store.ChangeSetFor(ctx, "op-cs")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, 3, got.CreateCount)
	assert.Equal(t, 1, got.DeleteCount)
	assert.True(t, rec.AcknowledgedAt.Equal(got.AcknowledgedAt))

	// The sealed path list opens back to the original plaintext, and the stored
	// bytes are not the plaintext (sensitive paths are sealed at rest).
	opened, err := sealer.Open(secrets.Sealed{Nonce: got.Nonce, Ciphertext: got.SealedBytes}, []byte("op-cs"))
	require.NoError(t, err)
	assert.Equal(t, plaintext, opened)
	assert.NotContains(t, string(got.SealedBytes), "secret/path.txt")
}

func TestChangeSetMissing(t *testing.T) {
	t.Parallel()
	store := openTemp(t)
	_, ok, err := store.ChangeSetFor(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestClearHistoryRemovesChangeSets(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := openTemp(t)
	seedOp(t, store, "op-cs", domain.KindSync, "/a", "/b", time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC))
	require.NoError(t, store.InsertChangeSet(ctx, domain.ChangeSetRecord{
		OperationID: "op-cs", DeleteCount: 1, AcknowledgedAt: time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC),
		Nonce: []byte{1}, SealedBytes: []byte{2}, BytesLen: 1,
	}))

	_, err := store.ClearHistory(ctx)
	require.NoError(t, err)

	_, ok, err := store.ChangeSetFor(ctx, "op-cs")
	require.NoError(t, err)
	assert.False(t, ok, "clear history removes the sealed change set")
}
