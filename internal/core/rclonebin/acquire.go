package rclonebin

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"time"

	"github.com/conductor-app/conductor/internal/core/coreerr"
)

// Fetcher downloads the bytes at url. It is injected so acquisition is testable
// without the network; production uses HTTPFetcher.
type Fetcher func(ctx context.Context, url string) ([]byte, error)

// DownloadURL returns the upstream archive URL for the pinned rclone on the
// given platform, mirroring the documented `rclone:fetch` task.
func DownloadURL(goos, goarch string) (string, error) {
	var osName string
	switch goos {
	case "darwin":
		osName = "osx"
	case "linux":
		osName = "linux"
	default:
		return "", fmt.Errorf("unsupported OS %q for rclone %s", goos, PinnedVersion)
	}
	if goarch != "amd64" && goarch != "arm64" {
		return "", fmt.Errorf("unsupported arch %q for rclone %s", goarch, PinnedVersion)
	}
	return fmt.Sprintf("https://downloads.rclone.org/%s/rclone-%s-%s-%s.zip",
		PinnedVersion, PinnedVersion, osName, goarch), nil
}

// Acquire downloads the pinned rclone for the host platform, verifies it against
// the committed manifest (the archive's published SHA-256, then the extracted
// binary's, ADR-0008), and installs the binary at dst. Acquisition is always
// operator-initiated; Conductor never downloads rclone silently. The binary is
// re-verified on every launch regardless (VerifyChecksum).
func Acquire(ctx context.Context, dst string, fetch Fetcher) error {
	cs, err := ChecksumsFor(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return coreerr.New(coreerr.CodeRcloneBinaryMissing, err.Error(), err)
	}
	url, err := DownloadURL(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return coreerr.New(coreerr.CodeRcloneBinaryMissing, err.Error(), err)
	}
	return acquire(ctx, dst, url, cs, fetch)
}

// acquire is the testable core, taking the URL and expected checksums explicitly.
func acquire(ctx context.Context, dst, url string, cs Checksums, fetch Fetcher) error {
	archive, err := fetch(ctx, url)
	if err != nil {
		return coreerr.New(coreerr.CodeRcloneBinaryMissing, "downloading rclone from "+url, err)
	}
	if got := sha256hex(archive); got != cs.Archive {
		return coreerr.New(coreerr.CodeRcloneChecksum,
			fmt.Sprintf("downloaded rclone archive failed integrity check: expected %s, got %s", cs.Archive, got), nil)
	}

	binary, err := extractBinary(archive)
	if err != nil {
		return coreerr.New(coreerr.CodeRcloneBinaryMissing, "extracting rclone from archive", err)
	}
	if got := sha256hex(binary); got != cs.Binary {
		return coreerr.New(coreerr.CodeRcloneChecksum,
			fmt.Sprintf("extracted rclone binary failed integrity check: expected %s, got %s", cs.Binary, got), nil)
	}

	if err := installBinary(dst, binary); err != nil {
		return coreerr.New(coreerr.CodeRcloneBinaryMissing, "installing rclone binary", err)
	}
	return nil
}

// extractBinary returns the rclone executable's bytes from the release zip. The
// upstream archive nests the binary under a versioned directory; the entry whose
// base name is exactly "rclone" is the executable.
func extractBinary(archive []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(archive), int64(len(archive)))
	if err != nil {
		return nil, fmt.Errorf("opening zip: %w", err)
	}
	for _, f := range zr.File {
		if f.FileInfo().IsDir() || path.Base(f.Name) != "rclone" {
			continue
		}
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("opening %s: %w", f.Name, err)
		}
		defer func() { _ = rc.Close() }()
		// Bound the read so a malicious archive can't exhaust memory.
		const maxBinary = 256 << 20 // 256 MiB
		b, err := io.ReadAll(io.LimitReader(rc, maxBinary))
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", f.Name, err)
		}
		return b, nil
	}
	return nil, fmt.Errorf("no rclone executable found in archive")
}

// installBinary writes the binary to dst atomically (temp file + rename) with
// the executable bit set, creating the destination directory if needed.
func installBinary(dst string, binary []byte) error {
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, binary, 0o755); err != nil { //nolint:gosec // an executable must be executable
		return err
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return err
	}
	return nil
}

// HTTPFetcher downloads url over HTTP with a generous timeout. It is the
// production Fetcher passed to Acquire.
func HTTPFetcher(ctx context.Context, url string) ([]byte, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("downloading %s: status %d", url, resp.StatusCode)
	}
	return io.ReadAll(resp.Body)
}

func sha256hex(b []byte) string {
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}
