package changeset

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The testdata fixtures are captured verbatim from the pinned rclone v1.74.3.
// They are the ground truth these parsers are pinned to; an rclone upgrade that
// changes the dry-run output shape must refresh them (caught by §7.5's drift
// guard), never silently mis-parse.

func loadFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return b
}

// --- --combined parser (sync/copy/move) ------------------------------------

func TestParseCombinedRealSyncFixture(t *testing.T) {
	t.Parallel()
	// sync_combined.txt: + a.txt, + sub/nested.txt (creates), * b.txt (update),
	// - c.txt (delete), = keep.txt (match, ignored).
	cs, err := ParseCombined(loadFixture(t, "sync_combined.txt"))
	require.NoError(t, err)

	assert.Equal(t, 2, cs.CreateCount)
	assert.Equal(t, 1, cs.UpdateCount)
	assert.Equal(t, 1, cs.DeleteCount)
	assert.True(t, cs.HasDeletes())
	assert.Equal(t, 4, cs.Total())
	assert.False(t, cs.Truncated)

	require.Len(t, cs.Deletes, 1)
	assert.Equal(t, "c.txt", cs.Deletes[0].Path)
	assert.Equal(t, "b.txt", cs.Updates[0].Path)
	assert.ElementsMatch(t, []string{"a.txt", "sub/nested.txt"},
		[]string{cs.Creates[0].Path, cs.Creates[1].Path})
}

func TestParseCombinedIgnoresMatchAndBlank(t *testing.T) {
	t.Parallel()
	cs, err := ParseCombined([]byte("= same.txt\n\n  \n! broke.txt\n"))
	require.NoError(t, err)
	assert.True(t, cs.Empty(), "match and error markers are not changes")
}

// --- --use-json-log parser (bisync/delete/purge) ----------------------------

func TestParseJSONLogRealPurgeFixture(t *testing.T) {
	t.Parallel()
	cs, err := ParseJSONLog(loadFixture(t, "purge_dryrun.ndjson"))
	require.NoError(t, err)
	assert.Equal(t, 0, cs.CreateCount)
	assert.Equal(t, 2, cs.DeleteCount)
	assert.ElementsMatch(t, []string{"f1.txt", "sub/f2.txt"},
		[]string{cs.Deletes[0].Path, cs.Deletes[1].Path})
}

func TestParseJSONLogRealSyncFixtureExcludesFsSummary(t *testing.T) {
	t.Parallel()
	// sync_dryrun.ndjson: copy a.txt, copy b.txt (writes -> creates), delete
	// c.txt, plus a server-side-directory-move summary at *local.Fs level and
	// config/stats noise that must be ignored.
	cs, err := ParseJSONLog(loadFixture(t, "sync_dryrun.ndjson"))
	require.NoError(t, err)
	assert.Equal(t, 2, cs.CreateCount, "writes are reported as creates on the json-log path")
	assert.Equal(t, 1, cs.DeleteCount)
	for _, c := range cs.Creates {
		assert.NotContains(t, c.Path, "Local file system at", "filesystem-level summaries are excluded")
	}
}

func TestParseJSONLogIgnoresNonSkipAndMalformed(t *testing.T) {
	t.Parallel()
	in := "\n   \nnot json\n" +
		`{"level":"error","msg":"Failed to copy","object":"x.txt","objectType":"*local.Object"}` + "\n"
	cs, err := ParseJSONLog([]byte(in))
	require.NoError(t, err)
	assert.True(t, cs.Empty(), "only records carrying a `skipped` action count")
}

// --- shared accumulator behaviour ------------------------------------------

func TestDeletesAreNeverTruncated(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	const deletes = 5
	for i := 0; i < maxListed+50; i++ {
		fmt.Fprintf(&b, "+ c/%d\n", i)
	}
	for i := 0; i < deletes; i++ {
		fmt.Fprintf(&b, "- d/%d\n", i)
	}
	cs, err := ParseCombined([]byte(b.String()))
	require.NoError(t, err)

	assert.Equal(t, maxListed+50, cs.CreateCount, "counts stay exact past the cap")
	assert.Len(t, cs.Creates, maxListed, "create list is capped")
	assert.True(t, cs.Truncated)
	assert.Equal(t, deletes, cs.DeleteCount)
	assert.Len(t, cs.Deletes, deletes, "every delete is enumerable regardless of cap")
}
