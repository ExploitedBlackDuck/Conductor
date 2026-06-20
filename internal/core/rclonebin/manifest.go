// Package rclonebin pins the exact rclone version Conductor supervises and
// verifies a binary against that pin (ADR-0008). Conductor never downloads
// rclone silently; acquisition is an operator-initiated step, and the binary is
// checksum-verified on every launch before the daemon is allowed to start.
package rclonebin

import (
	"fmt"
	"runtime"
)

// PinnedVersion is the exact rclone version Conductor is built and tested
// against. The catalog and rc-shape expectations track this version; bumping it
// is a deliberate, reviewed change accompanied by refreshed checksums.
const PinnedVersion = "v1.74.3"

// Checksums holds the SHA-256 of the upstream release archive (the
// upstream-published value) and of the extracted rclone binary (what Conductor
// verifies at launch) for one platform.
type Checksums struct {
	// Archive is the SHA-256 of the upstream rclone-<ver>-<plat>.zip, matching
	// the published SHA256SUMS — used when acquiring the binary.
	Archive string
	// Binary is the SHA-256 of the extracted rclone executable — verified on
	// every launch.
	Binary string
}

// platform keys the manifest by GOOS/GOARCH.
type platform struct {
	os   string
	arch string
}

// manifest pins, per supported platform, the archive and binary checksums for
// PinnedVersion. Values are derived from the upstream SHA256SUMS and the
// extracted binaries; see Taskfile target `rclone:fetch`.
var manifest = map[platform]Checksums{
	{"darwin", "arm64"}: {
		Archive: "33a435ab17023b686918ce9a3975aceb75fe1796c694f38f1993024be1f063f5",
		Binary:  "9a156afbdd0a6ade42b0b40e7c30240119e2c82914bc8d7059a94dd9242ca2ed",
	},
	{"darwin", "amd64"}: {
		Archive: "417cabd402d57806d597bd0ba8fb33a434ca8c2a1a5aa98de5a0bd4b52b39202",
		Binary:  "830416be0a57ccab22970ea58afe92f491547989c24bc587afc5ed7322a73b72",
	},
	{"linux", "arm64"}: {
		Archive: "8f8d47446e061f80c3256659fe8e21f56d72d96aaefe1275d088ea5eb6b42aa7",
		Binary:  "646d2db7e701a4d41d39ed38a71f63373ab051b270ee5f0d6ae14b24cc17c923",
	},
	{"linux", "amd64"}: {
		Archive: "dbee7ccd7a5d617e4ed4cd4555c16669b511abfe8d31164f61be35ac9e999bd2",
		Binary:  "9700aa1273ac73d6d0833c43ba63fe830516422cb131960b8c1a24ced789cba0",
	},
}

// ChecksumsFor returns the pinned checksums for the given platform, or an error
// if Conductor does not support it.
func ChecksumsFor(goos, goarch string) (Checksums, error) {
	cs, ok := manifest[platform{goos, goarch}]
	if !ok {
		return Checksums{}, fmt.Errorf("unsupported platform %s/%s for rclone %s", goos, goarch, PinnedVersion)
	}
	return cs, nil
}

// ExpectedBinarySHA256 returns the pinned binary checksum for the host platform.
func ExpectedBinarySHA256() (string, error) {
	cs, err := ChecksumsFor(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}
	return cs.Binary, nil
}
