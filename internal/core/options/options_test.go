package options_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/conductor-app/conductor/internal/core/coreerr"
	"github.com/conductor-app/conductor/internal/core/domain"
	"github.com/conductor-app/conductor/internal/core/options"
)

func loadCatalog(t *testing.T) *options.Catalog {
	t.Helper()
	c, err := options.Load()
	require.NoError(t, err)
	return c
}

func TestCatalogLoadsAndValidates(t *testing.T) {
	t.Parallel()
	c := loadCatalog(t)
	assert.NotEmpty(t, c.Flags())

	// A few safety-relevant flags must be present and correctly classified.
	transfers, ok := c.Lookup("--transfers")
	require.True(t, ok)
	assert.Equal(t, options.GovernedTransfers, transfers.Governed)

	resync, ok := c.Lookup("--resync")
	require.True(t, ok)
	assert.Equal(t, options.RiskDestructive, resync.Risk)
	assert.True(t, resync.AffectsData)
}

func TestBuildAssemblesArgvAndRCParams(t *testing.T) {
	t.Parallel()
	c := loadCatalog(t)

	sel := options.Selection{
		Single: map[string]string{
			"--transfers": "4",
			"--checksum":  "true",
		},
		Multi: map[string][]string{
			"--exclude": {"*.tmp", "cache/**"},
		},
	}
	built, err := c.Build(sel, domain.KindCopy, options.Ceilings{})
	require.NoError(t, err)

	assert.Contains(t, built.Argv, "--transfers")
	assert.Contains(t, built.Argv, "--checksum")
	assert.Equal(t, 4, built.ConfigParams["Transfers"])
	assert.Equal(t, true, built.ConfigParams["CheckSum"])
	assert.Equal(t, []string{"*.tmp", "cache/**"}, built.FilterParams["ExcludeRule"])
}

func TestBuildRejectsUnknownFlag(t *testing.T) {
	t.Parallel()
	c := loadCatalog(t)
	_, err := c.Build(options.Selection{Single: map[string]string{"--rm-rf-everything": "true"}}, domain.KindCopy, options.Ceilings{})
	require.Error(t, err)
	code, _ := coreerr.CodeOf(err)
	assert.Equal(t, coreerr.CodeOptionInvalid, code)
}

func TestBuildRejectsBadType(t *testing.T) {
	t.Parallel()
	c := loadCatalog(t)
	_, err := c.Build(options.Selection{Single: map[string]string{"--transfers": "lots"}}, domain.KindCopy, options.Ceilings{})
	require.Error(t, err)
	code, _ := coreerr.CodeOf(err)
	assert.Equal(t, coreerr.CodeOptionInvalid, code)
}

func TestBuildRejectsConflicts(t *testing.T) {
	t.Parallel()
	c := loadCatalog(t)
	sel := options.Selection{Single: map[string]string{"--checksum": "true", "--size-only": "true"}}
	_, err := c.Build(sel, domain.KindCopy, options.Ceilings{})
	require.Error(t, err)
	code, _ := coreerr.CodeOf(err)
	assert.Equal(t, coreerr.CodeOptionConflict, code)
}

func TestBuildRejectsDeleteTimingConflict(t *testing.T) {
	t.Parallel()
	c := loadCatalog(t)
	sel := options.Selection{Single: map[string]string{"--delete-during": "true", "--delete-after": "true"}}
	_, err := c.Build(sel, domain.KindSync, options.Ceilings{})
	require.Error(t, err)
	code, _ := coreerr.CodeOf(err)
	assert.Equal(t, coreerr.CodeOptionConflict, code)
}

func TestBuildEnforcesRequires(t *testing.T) {
	t.Parallel()
	c := loadCatalog(t)

	// --suffix requires --backup-dir (Conductor policy).
	_, err := c.Build(options.Selection{Single: map[string]string{"--suffix": ".bak"}}, domain.KindSync, options.Ceilings{})
	require.Error(t, err)
	code, _ := coreerr.CodeOf(err)
	assert.Equal(t, coreerr.CodeOptionInvalid, code)

	// With the required flag present it builds.
	built, err := c.Build(options.Selection{Single: map[string]string{"--suffix": ".bak", "--backup-dir": "old:backups"}}, domain.KindSync, options.Ceilings{})
	require.NoError(t, err)
	assert.Contains(t, built.Argv, "--suffix")
}

func TestBuildRejectsKindMismatch(t *testing.T) {
	t.Parallel()
	c := loadCatalog(t)
	// --resync is bisync-only.
	_, err := c.Build(options.Selection{Single: map[string]string{"--resync": "true"}}, domain.KindCopy, options.Ceilings{})
	require.Error(t, err)
	code, _ := coreerr.CodeOf(err)
	assert.Equal(t, coreerr.CodeOptionInvalid, code)
}

func TestBuildClampsToGovernanceCeiling(t *testing.T) {
	t.Parallel()
	c := loadCatalog(t)

	sel := options.Selection{Single: map[string]string{"--transfers": "64", "--bwlimit": "off"}}
	built, err := c.Build(sel, domain.KindCopy, options.Ceilings{Transfers: 8, Bwlimit: "10M"})
	require.NoError(t, err)

	assert.Equal(t, 8, built.ConfigParams["Transfers"], "transfers must be clamped to the ceiling")
	assert.Equal(t, "10M", built.ConfigParams["BwLimit"], "unlimited bwlimit must be clamped to the ceiling")
	require.Len(t, built.Clamps, 2)
}

func TestImpactRules(t *testing.T) {
	t.Parallel()
	c := loadCatalog(t)

	hasLevelForTitleContains := func(impacts []options.Impact, level options.ImpactLevel, substr string) bool {
		for _, im := range impacts {
			if im.Level == level && (strings.Contains(im.Title, substr) || strings.Contains(im.Detail, substr)) {
				return true
			}
		}
		return false
	}

	t.Run("sync requires acknowledgement of deletion", func(t *testing.T) {
		t.Parallel()
		im := c.Evaluate(options.EvalInput{Kind: domain.KindSync, Selection: options.Selection{Single: map[string]string{"--bwlimit": "1M"}}})
		assert.True(t, hasLevelForTitleContains(im, options.ImpactRequireAck, "deleted"))
	})

	t.Run("bisync resync requires acknowledgement", func(t *testing.T) {
		t.Parallel()
		im := c.Evaluate(options.EvalInput{Kind: domain.KindBisync, Selection: options.Selection{Single: map[string]string{"--resync": "true", "--bwlimit": "1M"}}})
		assert.True(t, hasLevelForTitleContains(im, options.ImpactRequireAck, "baseline"))
	})

	t.Run("delete-during requires acknowledgement", func(t *testing.T) {
		t.Parallel()
		im := c.Evaluate(options.EvalInput{Kind: domain.KindSync, Selection: options.Selection{Single: map[string]string{"--delete-during": "true", "--bwlimit": "1M"}}})
		assert.True(t, hasLevelForTitleContains(im, options.ImpactRequireAck, "DURING"))
	})

	t.Run("no bwlimit warns", func(t *testing.T) {
		t.Parallel()
		im := c.Evaluate(options.EvalInput{Kind: domain.KindCopy, Selection: options.Selection{}})
		assert.True(t, hasLevelForTitleContains(im, options.ImpactWarn, "bandwidth"))
	})

	t.Run("size-only warns about weakened detection", func(t *testing.T) {
		t.Parallel()
		im := c.Evaluate(options.EvalInput{Kind: domain.KindCopy, Selection: options.Selection{Single: map[string]string{"--size-only": "true", "--bwlimit": "1M"}}})
		assert.True(t, hasLevelForTitleContains(im, options.ImpactWarn, "change detection"))
	})

	t.Run("high transfers warns", func(t *testing.T) {
		t.Parallel()
		im := c.Evaluate(options.EvalInput{Kind: domain.KindCopy, Selection: options.Selection{Single: map[string]string{"--transfers": "64", "--bwlimit": "1M"}}})
		assert.True(t, hasLevelForTitleContains(im, options.ImpactWarn, "high"))
	})

	t.Run("safe copy has no acknowledgement requirement", func(t *testing.T) {
		t.Parallel()
		im := c.Evaluate(options.EvalInput{Kind: domain.KindCopy, Selection: options.Selection{Single: map[string]string{"--bwlimit": "1M"}}})
		for _, i := range im {
			assert.NotEqual(t, options.ImpactRequireAck, i.Level, "a plain bandwidth-capped copy should need no ack")
		}
	})
}
