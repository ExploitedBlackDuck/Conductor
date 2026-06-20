package rclonebin

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"runtime"

	"github.com/conductor-app/conductor/internal/core/coreerr"
)

// VerifyChecksum confirms the binary at path matches the pinned binary checksum
// for the host platform (ADR-0008). A missing file yields
// ERR_RCLONE_BINARY_MISSING; a mismatch yields ERR_RCLONE_BINARY_CHECKSUM. The
// daemon refuses to start unless this passes.
func VerifyChecksum(path string) error {
	expected, err := ExpectedBinarySHA256()
	if err != nil {
		return coreerr.New(coreerr.CodeRcloneBinaryMissing, err.Error(), err)
	}

	f, err := os.Open(path) //nolint:gosec // path comes from resolved config, not operator-controlled input
	if errors.Is(err, fs.ErrNotExist) {
		return coreerr.New(coreerr.CodeRcloneBinaryMissing,
			fmt.Sprintf("rclone binary not found at %s (run `task rclone:fetch` or install rclone %s)", path, PinnedVersion), err)
	}
	if err != nil {
		return coreerr.New(coreerr.CodeRcloneBinaryMissing, "opening rclone binary", err)
	}
	defer func() { _ = f.Close() }()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return coreerr.New(coreerr.CodeRcloneBinaryMissing, "reading rclone binary", err)
	}
	got := hex.EncodeToString(h.Sum(nil))

	if got != expected {
		return coreerr.New(coreerr.CodeRcloneChecksum,
			fmt.Sprintf("rclone binary at %s failed integrity check for %s/%s %s: expected %s, got %s",
				path, runtime.GOOS, runtime.GOARCH, PinnedVersion, expected, got), nil)
	}
	return nil
}
