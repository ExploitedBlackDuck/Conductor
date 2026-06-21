package sqlitestore_test

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/chacha20poly1305"

	"github.com/conductor-app/conductor/internal/adapters/sqlitestore"
	"github.com/conductor-app/conductor/internal/core/domain"
	"github.com/conductor-app/conductor/internal/core/secrets"
)

func TestMigrationsReachV7(t *testing.T) {
	t.Parallel()
	store := openTemp(t)
	v, err := store.SchemaVersion(context.Background())
	require.NoError(t, err)
	assert.Equal(t, 7, v)
	assert.Equal(t, 7, sqlitestore.ExpectedSchemaVersion)
}

func TestInsertAndQueryOperationWithSealedLog(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := openTemp(t)

	sealer, err := secrets.NewSealer(bytes.Repeat([]byte{0x33}, chacha20poly1305.KeySize))
	require.NoError(t, err)

	op := domain.Operation{
		ID:            "op-123",
		Kind:          domain.KindCopy,
		Src:           "example-s3:data",
		Dst:           "/local/backup",
		RcloneVersion: "v1.74.3",
		Intensity:     `{"transfers":4}`,
		StartedAt:     time.Date(2026, 6, 20, 12, 0, 0, 0, time.UTC),
		EndedAt:       time.Date(2026, 6, 20, 12, 5, 0, 0, time.UTC),
		BytesMoved:    1024,
		FilesMoved:    3,
		Result:        domain.ResultSuccess,
		LogRef:        "log-1",
	}
	opts := []domain.OperationOption{
		{Flag: "--transfers", Value: "4", Risk: "passive"},
		{Flag: "--checksum", Value: "true", Risk: "passive"},
	}

	plaintext := []byte(`{"stats":{"bytes":1024},"jobStatus":{"success":true}}`)
	sealed, err := sealer.Seal(plaintext, []byte(op.ID))
	require.NoError(t, err)
	log := &domain.CapturedLog{
		ID:          "log-1",
		OperationID: op.ID,
		Nonce:       sealed.Nonce,
		SealedBytes: sealed.Ciphertext,
		BytesLen:    len(plaintext),
	}

	require.NoError(t, store.InsertOperation(ctx, op, opts, log))

	// The operation and its options round-trip.
	gotOp, gotOpts, found, err := store.OperationByID(ctx, op.ID)
	require.NoError(t, err)
	require.True(t, found)
	assert.Equal(t, op.Kind, gotOp.Kind)
	assert.Equal(t, op.BytesMoved, gotOp.BytesMoved)
	assert.Equal(t, domain.ResultSuccess, gotOp.Result)
	assert.True(t, op.StartedAt.Equal(gotOp.StartedAt))
	assert.True(t, op.EndedAt.Equal(gotOp.EndedAt))
	assert.Len(t, gotOpts, 2)

	// The sealed log round-trips and opens to the original plaintext.
	gotLog, ok, err := store.OperationLog(ctx, op.ID)
	require.NoError(t, err)
	require.True(t, ok)
	opened, err := sealer.Open(secrets.Sealed{Nonce: gotLog.Nonce, Ciphertext: gotLog.SealedBytes}, []byte(op.ID))
	require.NoError(t, err)
	assert.Equal(t, plaintext, opened)
	// The sealed-at-rest bytes are not the plaintext.
	assert.NotContains(t, string(gotLog.SealedBytes), "success")
}

func TestOperationByIDMissing(t *testing.T) {
	t.Parallel()
	store := openTemp(t)
	_, _, found, err := store.OperationByID(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, found)
}
