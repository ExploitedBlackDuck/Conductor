package sqlitestore_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/conductor-app/conductor/internal/core/domain"
)

func TestVerificationRoundTrip(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := openTemp(t)

	base := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)
	v := domain.Verification{
		ID: "ver-1", Kind: domain.VerifyCheck, Src: "/src", Dst: "s3:dst",
		StartedAt: base, EndedAt: base.Add(time.Minute),
		Match: 10, Differ: 1, Missing: 2, ErrorCount: 0, Result: domain.VerifyMismatch,
	}
	require.NoError(t, store.InsertVerification(ctx, v))

	got, err := store.Verifications(ctx, 10)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "ver-1", got[0].ID)
	assert.Equal(t, domain.VerifyCheck, got[0].Kind)
	assert.Equal(t, 10, got[0].Match)
	assert.Equal(t, 1, got[0].Differ)
	assert.Equal(t, 2, got[0].Missing)
	assert.Equal(t, domain.VerifyMismatch, got[0].Result)
	assert.True(t, base.Equal(got[0].StartedAt))
	assert.True(t, base.Add(time.Minute).Equal(got[0].EndedAt))
}

func TestVerificationsNewestFirst(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := openTemp(t)
	base := time.Date(2026, 6, 21, 12, 0, 0, 0, time.UTC)

	for i, id := range []string{"ver-old", "ver-new"} {
		require.NoError(t, store.InsertVerification(ctx, domain.Verification{
			ID: id, Kind: domain.VerifyCheck, Src: "/a", Dst: "/b",
			StartedAt: base.Add(time.Duration(i) * time.Hour), Result: domain.VerifyMatch,
		}))
	}
	got, err := store.Verifications(ctx, 10)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "ver-new", got[0].ID)
}
