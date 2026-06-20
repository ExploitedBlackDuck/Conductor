package rclonebin

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/conductor-app/conductor/internal/core/coreerr"
)

func TestChecksumsForKnownPlatforms(t *testing.T) {
	t.Parallel()
	for _, p := range []struct{ os, arch string }{
		{"darwin", "arm64"}, {"darwin", "amd64"}, {"linux", "arm64"}, {"linux", "amd64"},
	} {
		cs, err := ChecksumsFor(p.os, p.arch)
		require.NoError(t, err)
		assert.Len(t, cs.Binary, 64, "binary sha256 hex length")
		assert.Len(t, cs.Archive, 64, "archive sha256 hex length")
	}
}

func TestChecksumsForUnsupportedPlatform(t *testing.T) {
	t.Parallel()
	_, err := ChecksumsFor("plan9", "mips")
	require.Error(t, err)
}

func TestVerifyChecksumMissingFile(t *testing.T) {
	t.Parallel()
	err := VerifyChecksum(filepath.Join(t.TempDir(), "no-such-rclone"))
	require.Error(t, err)
	code, ok := coreerr.CodeOf(err)
	require.True(t, ok)
	assert.Equal(t, coreerr.CodeRcloneBinaryMissing, code)
}

func TestVerifyChecksumMismatch(t *testing.T) {
	t.Parallel()
	// A file whose contents are not the pinned binary must fail the integrity
	// check with the checksum error code.
	path := filepath.Join(t.TempDir(), "rclone")
	require.NoError(t, os.WriteFile(path, []byte("not the real rclone"), 0o755)) //nolint:gosec // test fixture

	err := VerifyChecksum(path)
	require.Error(t, err)
	code, ok := coreerr.CodeOf(err)
	require.True(t, ok)
	assert.Equal(t, coreerr.CodeRcloneChecksum, code)
}

func TestVerifyChecksumAcceptsPinnedBinary(t *testing.T) {
	t.Parallel()
	// When a correctly-pinned binary is present at the documented dev location,
	// verification passes. Skip when it is absent so the unit suite stays green
	// on a bare machine.
	home, err := os.UserHomeDir()
	require.NoError(t, err)
	var binPath string
	switch runtime.GOOS {
	case "darwin":
		binPath = filepath.Join(home, "Library", "Application Support", "conductor", "data", "bin", "rclone")
	default:
		binPath = filepath.Join(home, ".local", "share", "conductor", "bin", "rclone")
	}
	if _, statErr := os.Stat(binPath); statErr != nil {
		t.Skipf("pinned rclone not installed at %s", binPath)
	}
	assert.NoError(t, VerifyChecksum(binPath))
}
