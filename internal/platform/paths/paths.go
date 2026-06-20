// Package paths resolves Conductor's XDG-respecting configuration and data
// locations (§4.1). Conductor never writes into the application bundle or the
// working directory; directories are created with restrictive permissions
// (0700), and callers writing files within them use 0600.
package paths

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// appDir is the per-application subdirectory name used under every base path.
const appDir = "conductor"

// dirPerm is the permission applied to Conductor's directories: owner-only
// access, so no other local user can read history, captured logs, or config.
const dirPerm os.FileMode = 0o700

// Paths holds the resolved, absolute locations Conductor uses at runtime.
type Paths struct {
	// ConfigDir holds config.toml (§4.1).
	ConfigDir string
	// DataDir holds the SQLite database, audit log, and the pinned rclone
	// binary (§4.1).
	DataDir string
}

// Resolve computes Conductor's configuration and data directories for the host
// operating system, honouring XDG_CONFIG_HOME and XDG_DATA_HOME when set. It
// does not create the directories; see EnsureDirs.
func Resolve() (Paths, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return Paths{}, fmt.Errorf("resolving home directory: %w", err)
	}
	return resolveWith(runtime.GOOS, os.Getenv, home), nil
}

// resolveWith is the testable core of Resolve: it takes the target OS, an
// environment lookup, and the home directory explicitly so the platform matrix
// can be exercised without mutating process state.
func resolveWith(goos string, getenv func(string) string, home string) Paths {
	switch goos {
	case "darwin":
		base := filepath.Join(home, "Library", "Application Support", appDir)
		return Paths{
			ConfigDir: base,
			DataDir:   filepath.Join(base, "data"),
		}
	default: // linux and other XDG-style systems
		configBase := getenv("XDG_CONFIG_HOME")
		if configBase == "" {
			configBase = filepath.Join(home, ".config")
		}
		dataBase := getenv("XDG_DATA_HOME")
		if dataBase == "" {
			dataBase = filepath.Join(home, ".local", "share")
		}
		return Paths{
			ConfigDir: filepath.Join(configBase, appDir),
			DataDir:   filepath.Join(dataBase, appDir),
		}
	}
}

// EnsureDirs creates the configuration and data directories with owner-only
// permissions, creating parents as needed. It is idempotent.
func EnsureDirs(p Paths) error {
	for _, dir := range []string{p.ConfigDir, p.DataDir} {
		if err := os.MkdirAll(dir, dirPerm); err != nil {
			return fmt.Errorf("creating directory %s: %w", dir, err)
		}
	}
	return nil
}
