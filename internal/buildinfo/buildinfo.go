// Package buildinfo exposes the application's version metadata. The version is
// injected at release-build time via the linker and otherwise derived from the
// module's VCS build information, so a development build is still
// self-identifying in logs and the UI (§7.11, §9).
package buildinfo

import "runtime/debug"

// version is set at build time via -ldflags "-X
// github.com/conductor-app/conductor/internal/buildinfo.version=vX.Y.Z". It is
// intentionally unexported; callers read it through Version.
var version string

// Version returns the build's semantic version. When the linker-injected value
// is absent (a plain `go build`), it falls back to a short VCS revision, and
// finally to "dev".
func Version() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		var revision string
		var dirty bool
		for _, s := range info.Settings {
			switch s.Key {
			case "vcs.revision":
				revision = s.Value
			case "vcs.modified":
				dirty = s.Value == "true"
			}
		}
		if len(revision) >= 12 {
			v := "dev+" + revision[:12]
			if dirty {
				v += "-dirty"
			}
			return v
		}
	}
	return "dev"
}
