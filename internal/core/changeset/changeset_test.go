package changeset

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/conductor-app/conductor/internal/core/domain"
)

// fixtureLog is a representative slice of rclone --use-json-log dry-run output.
// These message shapes are pinned here and must be re-validated against the
// real binary on an rclone upgrade (§7.5 drift guard) — the parser keys off the
// emitted event, never the human-readable text.
const fixtureLog = `{"level":"notice","msg":"Skipped copy as --dry-run is set (size 1.234Ki)","object":"new/a.txt"}
{"level":"notice","msg":"Skipped update as --dry-run is set (size 10)","object":"changed/b.txt"}
{"level":"notice","msg":"Skipped delete as --dry-run is set (size 5)","object":"gone/c.txt"}
{"level":"info","msg":"There was nothing to transfer"}
not-json garbage line
{"level":"notice","msg":"Skipped server-side move as --dry-run is set","object":"moved/d.txt"}
`

func TestParseClassifiesEachKind(t *testing.T) {
	t.Parallel()
	cs, err := Parse([]byte(fixtureLog))
	require.NoError(t, err)

	assert.Equal(t, 2, cs.CreateCount, "copy and server-side move are creates at the destination")
	assert.Equal(t, 1, cs.UpdateCount)
	assert.Equal(t, 1, cs.DeleteCount)
	assert.False(t, cs.Truncated)
	assert.True(t, cs.HasDeletes())
	assert.Equal(t, 4, cs.Total())

	require.Len(t, cs.Deletes, 1)
	assert.Equal(t, "gone/c.txt", cs.Deletes[0].Path)
	assert.Equal(t, domain.ChangeDelete, cs.Deletes[0].Kind)
}

func TestParseEmptyIsCleanNonDestructive(t *testing.T) {
	t.Parallel()
	cs, err := Parse([]byte(`{"level":"info","msg":"There was nothing to transfer"}` + "\n"))
	require.NoError(t, err)
	assert.True(t, cs.Empty())
	assert.False(t, cs.HasDeletes())
}

func TestParseIgnoresNonDryRunAndMalformed(t *testing.T) {
	t.Parallel()
	cs, err := Parse([]byte("\n   \nnot json\n{\"level\":\"error\",\"msg\":\"failed to copy\",\"object\":\"x\"}\n"))
	require.NoError(t, err)
	assert.True(t, cs.Empty(), "only 'as --dry-run is set' skip records count")
}

func TestParseDeletesAreNeverTruncated(t *testing.T) {
	t.Parallel()
	var b strings.Builder
	const deletes = 5
	for i := 0; i < maxListed+50; i++ {
		fmt.Fprintf(&b, `{"level":"notice","msg":"Skipped copy as --dry-run is set","object":"c/%d"}`+"\n", i)
	}
	for i := 0; i < deletes; i++ {
		fmt.Fprintf(&b, `{"level":"notice","msg":"Skipped delete as --dry-run is set","object":"d/%d"}`+"\n", i)
	}
	cs, err := Parse([]byte(b.String()))
	require.NoError(t, err)

	assert.Equal(t, maxListed+50, cs.CreateCount, "counts stay exact past the cap")
	assert.Len(t, cs.Creates, maxListed, "create list is capped")
	assert.True(t, cs.Truncated)
	assert.Equal(t, deletes, cs.DeleteCount)
	assert.Len(t, cs.Deletes, deletes, "every delete is enumerable regardless of cap")
}

func TestClassify(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		kind domain.ChangeKind
		ok   bool
	}{
		"Skipped copy as --dry-run is set":             {domain.ChangeCreate, true},
		"Skipped update as --dry-run is set":           {domain.ChangeUpdate, true},
		"Skipped delete as --dry-run is set":           {domain.ChangeDelete, true},
		"Skipped server-side move as --dry-run is set": {domain.ChangeCreate, true},
		"Skipped copy (server-side)":                   {"", false}, // not a dry-run record
		"There was nothing to transfer":                {"", false},
	}
	for msg, want := range cases {
		kind, ok := classify(msg)
		assert.Equal(t, want.ok, ok, msg)
		assert.Equal(t, want.kind, kind, msg)
	}
}
