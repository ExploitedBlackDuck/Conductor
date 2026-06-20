//go:build integration

// Drift guard (§7.5, P3 gate): parse the pinned rclone binary's actual flags
// and assert the catalog references only real flags for that version. An rclone
// upgrade that renames or removes a flag fails this test until the catalog is
// updated, so the catalog cannot drift silently. Build-tagged; skips when the
// pinned binary is absent.
package options_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/conductor-app/conductor/internal/core/options"
	"github.com/conductor-app/conductor/internal/core/rclonebin"
)

func pinnedBinaryPath(t *testing.T) string {
	t.Helper()
	if env := os.Getenv("CONDUCTOR_RCLONE_BIN"); env != "" {
		return env
	}
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	switch runtime.GOOS {
	case "darwin":
		return filepath.Join(home, "Library", "Application Support", "conductor", "data", "bin", "rclone")
	default:
		return filepath.Join(home, ".local", "share", "conductor", "bin", "rclone")
	}
}

var flagToken = regexp.MustCompile(`--[a-z][a-z0-9-]+`)

// realFlags returns the set of flags the pinned binary actually accepts, taken
// from global flags and the bisync command's flags.
func realFlags(t *testing.T, bin string) map[string]bool {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	flags := map[string]bool{}
	for _, args := range [][]string{{"help", "flags"}, {"bisync", "--help"}} {
		out, err := exec.CommandContext(ctx, bin, args...).CombinedOutput() //nolint:gosec // pinned absolute binary, argv-style
		require.NoError(t, err, "running rclone %v", args)
		for _, m := range flagToken.FindAllString(string(out), -1) {
			flags[m] = true
		}
	}
	return flags
}

func TestCatalogDoesNotDriftFromBinary(t *testing.T) {
	bin := pinnedBinaryPath(t)
	if _, err := os.Stat(bin); err != nil {
		t.Skipf("pinned rclone not installed at %s", bin)
	}

	// Sanity: the binary reports the pinned version.
	verOut, err := exec.CommandContext(context.Background(), bin, "version").CombinedOutput() //nolint:gosec // pinned absolute binary
	require.NoError(t, err)
	require.Contains(t, string(verOut), rclonebin.PinnedVersion)

	real := realFlags(t, bin)

	catalog, err := options.Load()
	require.NoError(t, err)

	for _, flag := range catalog.Flags() {
		assert.Truef(t, real[flag], "catalog flag %s is not a real flag of rclone %s", flag, rclonebin.PinnedVersion)
		opt, _ := catalog.Lookup(flag)
		for _, alias := range opt.Aliases {
			assert.Truef(t, real[alias], "catalog alias %s (of %s) is not a real flag", alias, flag)
		}
	}
}
