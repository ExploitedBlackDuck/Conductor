// Package config loads Conductor's typed configuration from a single TOML file
// (§4). There is exactly one loader and one typed Config; the rest of the
// application receives values, never raw file access. A missing file is not an
// error — first run yields defaults — so the app is usable before any config
// is written.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"

	"github.com/BurntSushi/toml"
)

// Config is Conductor's fully-resolved configuration. Zero values are replaced
// by Defaults during Load, so every field is meaningful by the time the
// application sees it.
type Config struct {
	Log       Log       `toml:"log"`
	Window    Window    `toml:"window"`
	Rclone    Rclone    `toml:"rclone"`
	Transfers Transfers `toml:"transfers"`
}

// Transfers configures operation execution (§7.6).
type Transfers struct {
	// MaxConcurrent caps how many operations run at once (a Conductor-level
	// limit, distinct from rclone's intra-job --transfers; §2.3). Excess
	// launches queue. Zero resolves to the conservative built-in default; a
	// negative value means unbounded.
	MaxConcurrent int `toml:"max_concurrent"`
}

// Rclone configures how Conductor locates the pinned rclone binary and its
// configuration (ADR-0008). Empty paths resolve to documented defaults.
type Rclone struct {
	// BinaryPath is the absolute path to the pinned rclone binary; empty
	// resolves to <data dir>/bin/rclone.
	BinaryPath string `toml:"binary_path"`
	// ConfigPath optionally points rclone at a specific rclone.conf; empty uses
	// rclone's own default location.
	ConfigPath string `toml:"config_path"`
}

// Log configures the operational logger (§2.4).
type Log struct {
	// Level is one of debug, info, warn, error (case-insensitive).
	Level string `toml:"level"`
	// JSON selects machine-readable output over the human-readable dev handler.
	JSON bool `toml:"json"`
}

// Window configures the initial desktop window geometry (§7.11.1).
type Window struct {
	Width  int `toml:"width"`
	Height int `toml:"height"`
}

// Defaults returns Conductor's built-in configuration: info-level
// human-readable logging and a window comfortably above the documented minimum
// (§7.11.1).
func Defaults() Config {
	return Config{
		Log:       Log{Level: "info", JSON: false},
		Window:    Window{Width: 1200, Height: 800},
		Transfers: Transfers{MaxConcurrent: 4},
	}
}

// SlogLevel maps the configured level name to a slog.Level, defaulting to info
// for an unrecognised value.
func (l Log) SlogLevel() slog.Level {
	switch l.Level {
	case "debug", "DEBUG":
		return slog.LevelDebug
	case "warn", "WARN", "warning":
		return slog.LevelWarn
	case "error", "ERROR":
		return slog.LevelError
	default:
		return slog.LevelInfo
	}
}

// Load reads configuration from path, layering any values found over Defaults.
// A non-existent file yields Defaults with no error (§4.1 first-run behaviour);
// a malformed file is a hard error so misconfiguration is never silently
// ignored.
func Load(path string) (Config, error) {
	cfg := Defaults()

	data, err := os.ReadFile(path) //nolint:gosec // path is composed from the resolved config dir, not operator input
	if errors.Is(err, fs.ErrNotExist) {
		return cfg, nil
	}
	if err != nil {
		return Config{}, fmt.Errorf("reading config %s: %w", path, err)
	}

	if err := toml.Unmarshal(data, &cfg); err != nil {
		return Config{}, fmt.Errorf("parsing config %s: %w", path, err)
	}
	return cfg, nil
}
