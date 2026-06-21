package rclonebin

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/conductor-app/conductor/internal/core/coreerr"
)

// makeArchive builds a release-shaped zip (binary nested under a versioned dir)
// and returns the zip bytes alongside the binary content.
func makeArchive(t *testing.T, binary []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	// A directory entry and the nested executable, as the upstream archive ships.
	_, err := zw.Create("rclone-v1.74.3-osx-arm64/")
	require.NoError(t, err)
	w, err := zw.Create("rclone-v1.74.3-osx-arm64/rclone")
	require.NoError(t, err)
	_, err = w.Write(binary)
	require.NoError(t, err)
	require.NoError(t, zw.Close())
	return buf.Bytes()
}

func TestAcquireVerifiesAndInstalls(t *testing.T) {
	t.Parallel()
	binary := []byte("FAKE-RCLONE-BINARY")
	archive := makeArchive(t, binary)
	cs := Checksums{Archive: sha256hex(archive), Binary: sha256hex(binary)}

	dst := filepath.Join(t.TempDir(), "bin", "rclone")
	fetch := func(_ context.Context, _ string) ([]byte, error) { return archive, nil }

	require.NoError(t, acquire(context.Background(), dst, "https://example/x.zip", cs, fetch))

	got, err := os.ReadFile(dst)
	require.NoError(t, err)
	assert.Equal(t, binary, got)

	info, err := os.Stat(dst)
	require.NoError(t, err)
	assert.NotZero(t, info.Mode()&0o100, "the installed binary is executable")
}

func TestAcquireRejectsTamperedArchive(t *testing.T) {
	t.Parallel()
	archive := makeArchive(t, []byte("real"))
	// Expect a different archive checksum than what is fetched.
	cs := Checksums{Archive: sha256hex([]byte("something else")), Binary: sha256hex([]byte("real"))}
	fetch := func(_ context.Context, _ string) ([]byte, error) { return archive, nil }

	err := acquire(context.Background(), filepath.Join(t.TempDir(), "rclone"), "u", cs, fetch)
	require.Error(t, err)
	code, _ := coreerr.CodeOf(err)
	assert.Equal(t, coreerr.CodeRcloneChecksum, code)
}

func TestAcquireRejectsTamperedBinary(t *testing.T) {
	t.Parallel()
	binary := []byte("swapped-binary")
	archive := makeArchive(t, binary)
	// Archive checksum matches, but the expected binary checksum does not.
	cs := Checksums{Archive: sha256hex(archive), Binary: sha256hex([]byte("the-real-one"))}
	fetch := func(_ context.Context, _ string) ([]byte, error) { return archive, nil }

	err := acquire(context.Background(), filepath.Join(t.TempDir(), "rclone"), "u", cs, fetch)
	require.Error(t, err)
	code, _ := coreerr.CodeOf(err)
	assert.Equal(t, coreerr.CodeRcloneChecksum, code)
}

func TestAcquireDownloadFailureIsCoded(t *testing.T) {
	t.Parallel()
	fetch := func(_ context.Context, _ string) ([]byte, error) { return nil, errors.New("no network") }
	err := acquire(context.Background(), filepath.Join(t.TempDir(), "rclone"),
		"u", Checksums{Archive: "x", Binary: "y"}, fetch)
	require.Error(t, err)
	code, _ := coreerr.CodeOf(err)
	assert.Equal(t, coreerr.CodeRcloneBinaryMissing, code)
}

func TestDownloadURL(t *testing.T) {
	t.Parallel()
	u, err := DownloadURL("darwin", "arm64")
	require.NoError(t, err)
	assert.Equal(t, "https://downloads.rclone.org/v1.74.3/rclone-v1.74.3-osx-arm64.zip", u)

	u, err = DownloadURL("linux", "amd64")
	require.NoError(t, err)
	assert.Contains(t, u, "rclone-v1.74.3-linux-amd64.zip")

	_, err = DownloadURL("windows", "amd64")
	require.Error(t, err)
}
