package sqlitestore_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/conductor-app/conductor/internal/core/domain"
)

func TestProfileRoundTripAndOptions(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := openTemp(t)

	prof := domain.Profile{
		ID:   "prof-1",
		Name: "safe copy",
		Kind: domain.KindCopy,
		Options: []domain.ProfileOption{
			{Flag: "--checksum", Value: "true"},
			{Flag: "--transfers", Value: "4"},
		},
	}
	require.NoError(t, store.SaveProfile(ctx, prof))

	got, ok, err := store.Profile(ctx, "prof-1")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, "safe copy", got.Name)
	assert.Equal(t, domain.KindCopy, got.Kind)
	assert.Len(t, got.Options, 2)

	// Re-saving replaces the option set rather than appending.
	prof.Options = []domain.ProfileOption{{Flag: "--checksum", Value: "true"}}
	require.NoError(t, store.SaveProfile(ctx, prof))
	got, _, err = store.Profile(ctx, "prof-1")
	require.NoError(t, err)
	assert.Len(t, got.Options, 1)
}

func TestSavedPairRoundTripAndTouch(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := openTemp(t)

	require.NoError(t, store.SaveProfile(ctx, domain.Profile{ID: "prof-1", Name: "mirror", Kind: domain.KindBisync}))

	pair := domain.SavedPair{
		ID:        "pair-1",
		Name:      "laptop ↔ drive",
		Kind:      domain.PairBisync,
		Path1:     "/home/me",
		Path2:     "gdrive:backup",
		ProfileID: "prof-1",
	}
	require.NoError(t, store.SavePair(ctx, pair))

	got, ok, err := store.Pair(ctx, "pair-1")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, domain.PairBisync, got.Kind)
	assert.Equal(t, "prof-1", got.ProfileID)
	assert.False(t, got.HasRun(), "a freshly saved pair has never run")

	// Touching the pair marks it as run.
	at := time.Date(2026, 6, 20, 9, 0, 0, 0, time.UTC)
	require.NoError(t, store.TouchPairRun(ctx, "pair-1", at))
	got, _, err = store.Pair(ctx, "pair-1")
	require.NoError(t, err)
	assert.True(t, got.HasRun())
	assert.True(t, at.Equal(got.LastRun))
}

func TestDeleteProfileNullsPairReference(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := openTemp(t)

	require.NoError(t, store.SaveProfile(ctx, domain.Profile{ID: "prof-1", Name: "p", Kind: domain.KindSync}))
	require.NoError(t, store.SavePair(ctx, domain.SavedPair{
		ID: "pair-1", Name: "n", Kind: domain.PairSync, Path1: "a:", Path2: "b:", ProfileID: "prof-1",
	}))

	require.NoError(t, store.DeleteProfile(ctx, "prof-1"))

	got, ok, err := store.Pair(ctx, "pair-1")
	require.NoError(t, err)
	require.True(t, ok, "deleting a profile must not delete pairs that referenced it")
	assert.Empty(t, got.ProfileID, "the dangling profile reference is nulled")
}

func TestPairWithoutProfile(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := openTemp(t)

	require.NoError(t, store.SavePair(ctx, domain.SavedPair{
		ID: "pair-1", Name: "n", Kind: domain.PairSync, Path1: "a:", Path2: "b:",
	}))
	got, ok, err := store.Pair(ctx, "pair-1")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Empty(t, got.ProfileID)
}

func TestRemoteCeilingRoundTripAndUpsert(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := openTemp(t)

	require.NoError(t, store.SetCeiling(ctx, domain.RemoteCeiling{Remote: "s3", Transfers: 2, Bwlimit: "10M"}))

	got, ok, err := store.Ceiling(ctx, "s3")
	require.NoError(t, err)
	require.True(t, ok)
	assert.Equal(t, 2, got.Transfers)
	assert.Equal(t, "10M", got.Bwlimit)

	// A second set for the same remote updates in place.
	require.NoError(t, store.SetCeiling(ctx, domain.RemoteCeiling{Remote: "s3", Transfers: 4, Checkers: 8}))
	require.NoError(t, store.SetCeiling(ctx, domain.RemoteCeiling{Remote: "b2", Tpslimit: 5}))

	all, err := store.Ceilings(ctx)
	require.NoError(t, err)
	require.Len(t, all, 2)
	assert.Equal(t, "b2", all[0].Remote) // ordered by remote
	assert.Equal(t, 4, all[1].Transfers)
	assert.Empty(t, all[1].Bwlimit, "the update replaced the row wholesale")
}

func TestPairMissing(t *testing.T) {
	t.Parallel()
	store := openTemp(t)
	_, ok, err := store.Pair(context.Background(), "nope")
	require.NoError(t, err)
	assert.False(t, ok)
}
