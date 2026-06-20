// Package frontend embeds the production build of Conductor's Svelte frontend
// so it ships inside the single Go binary and is served by the desktop shell's
// asset server (§3). The embedded tree is rooted at "dist"; the shell calls
// fs.Sub to serve it at the web root.
package frontend

import "embed"

//go:embed all:dist
var dist embed.FS

// Dist returns the embedded frontend build (Vite output under dist/). The
// directory must be built before the Go binary is compiled; the Taskfile's
// build target enforces that ordering.
func Dist() embed.FS {
	return dist
}
