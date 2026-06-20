// Package catalogs embeds Conductor's versioned rclone option catalogs
// (ADR-0011) and exposes them as an fs.FS for the options package to load. The
// catalog is maintained per pinned rclone version and tested against the
// binary's actual flags so it cannot drift silently (§7.5).
package catalogs

import "embed"

//go:embed *.toml
var files embed.FS

// FS returns the embedded catalog files.
func FS() embed.FS {
	return files
}
