package app

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/conductor-app/conductor/internal/core/options"
)

func testApp(t *testing.T) *App {
	t.Helper()
	cat, err := options.Load()
	require.NoError(t, err)
	return &App{catalog: cat}
}

func TestGetCatalogIsOrderedAndPopulated(t *testing.T) {
	t.Parallel()
	a := testApp(t)
	cat := a.GetCatalog()

	assert.NotEmpty(t, cat.RcloneVersion)
	require.NotEmpty(t, cat.Categories)
	// performance is first in the preferred order and must precede deletion.
	perfIdx, delIdx := -1, -1
	for i, c := range cat.Categories {
		switch c.Name {
		case "performance":
			perfIdx = i
		case "deletion":
			delIdx = i
		}
	}
	require.GreaterOrEqual(t, perfIdx, 0)
	require.GreaterOrEqual(t, delIdx, 0)
	assert.Less(t, perfIdx, delIdx)
}

func TestPreviewSyncRequiresAck(t *testing.T) {
	t.Parallel()
	a := testApp(t)

	out := a.PreviewOperation(PreviewRequest{
		Kind:   "sync",
		Single: map[string]string{"--bwlimit": "10M"},
		Src:    EndpointDTO{Remote: "example-s3", Path: "data"},
		Dst:    EndpointDTO{Path: "/local/backup"},
	})

	assert.Nil(t, out.Error)
	assert.True(t, out.RequiresAck, "a sync deletes at the destination and must require ack")
	assert.Equal(t, "destructive", out.RiskLevel)
	assert.True(t, strings.HasPrefix(out.Command, "rclone sync example-s3:data /local/backup"))
}

func TestPreviewInvalidSelectionSurfacesError(t *testing.T) {
	t.Parallel()
	a := testApp(t)

	out := a.PreviewOperation(PreviewRequest{
		Kind:   "copy",
		Single: map[string]string{"--checksum": "true", "--size-only": "true"},
	})

	require.NotNil(t, out.Error)
	assert.Equal(t, "ERR_OPTION_CONFLICT", out.Error.Code)
	assert.Empty(t, out.Argv)
	// Impacts are still computed even when the build fails.
	assert.NotNil(t, out.Impacts)
}

func TestPreviewClampSurfacesInDTO(t *testing.T) {
	t.Parallel()
	a := testApp(t)

	out := a.PreviewOperation(PreviewRequest{
		Kind:     "copy",
		Single:   map[string]string{"--transfers": "64"},
		Ceilings: CeilingsDTO{Transfers: 8},
	})
	require.Nil(t, out.Error)
	require.Len(t, out.Clamps, 1)
	assert.Equal(t, "--transfers", out.Clamps[0].Flag)
	assert.Equal(t, "8", out.Clamps[0].Applied)
}
