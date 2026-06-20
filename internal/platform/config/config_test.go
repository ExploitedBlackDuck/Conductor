package config

import (
	"log/slog"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// write is a tiny helper writing TOML fixture content to path.
func write(t *testing.T, path, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(path, []byte(content), 0o600))
}

func TestLoadMissingFileReturnsDefaults(t *testing.T) {
	t.Parallel()

	cfg, err := Load(filepath.Join(t.TempDir(), "does-not-exist.toml"))
	require.NoError(t, err)
	assert.Equal(t, Defaults(), cfg)
}

func TestLoadLayersOverDefaults(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	write(t, path, `
[log]
level = "debug"
json = true

[window]
width = 1440
`)

	cfg, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, "debug", cfg.Log.Level)
	assert.True(t, cfg.Log.JSON)
	assert.Equal(t, 1440, cfg.Window.Width)
	// Unspecified field falls back to the default.
	assert.Equal(t, Defaults().Window.Height, cfg.Window.Height)
}

func TestLoadMalformedFileErrors(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "config.toml")
	write(t, path, "this is = = not toml")

	_, err := Load(path)
	require.Error(t, err)
}

func TestSlogLevel(t *testing.T) {
	t.Parallel()

	tests := []struct {
		in   string
		want slog.Level
	}{
		{"debug", slog.LevelDebug},
		{"info", slog.LevelInfo},
		{"warn", slog.LevelWarn},
		{"error", slog.LevelError},
		{"nonsense", slog.LevelInfo},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, tc.want, Log{Level: tc.in}.SlogLevel())
		})
	}
}
