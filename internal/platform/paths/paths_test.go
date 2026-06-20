package paths

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveWith(t *testing.T) {
	t.Parallel()

	const home = "/home/op"

	tests := []struct {
		name       string
		goos       string
		env        map[string]string
		wantConfig string
		wantData   string
	}{
		{
			name:       "linux defaults to XDG fallbacks",
			goos:       "linux",
			env:        map[string]string{},
			wantConfig: filepath.Join(home, ".config", "conductor"),
			wantData:   filepath.Join(home, ".local", "share", "conductor"),
		},
		{
			name:       "linux honours XDG overrides",
			goos:       "linux",
			env:        map[string]string{"XDG_CONFIG_HOME": "/cfg", "XDG_DATA_HOME": "/dat"},
			wantConfig: filepath.Join("/cfg", "conductor"),
			wantData:   filepath.Join("/dat", "conductor"),
		},
		{
			name:       "darwin uses Application Support and ignores XDG",
			goos:       "darwin",
			env:        map[string]string{"XDG_CONFIG_HOME": "/cfg"},
			wantConfig: filepath.Join(home, "Library", "Application Support", "conductor"),
			wantData:   filepath.Join(home, "Library", "Application Support", "conductor", "data"),
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := resolveWith(tc.goos, func(k string) string { return tc.env[k] }, home)
			assert.Equal(t, tc.wantConfig, got.ConfigDir)
			assert.Equal(t, tc.wantData, got.DataDir)
		})
	}
}

func TestEnsureDirsCreatesWithOwnerOnlyPerms(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	p := Paths{
		ConfigDir: filepath.Join(root, "cfg", "conductor"),
		DataDir:   filepath.Join(root, "data", "conductor"),
	}
	require.NoError(t, EnsureDirs(p))
	require.NoError(t, EnsureDirs(p)) // idempotent

	for _, dir := range []string{p.ConfigDir, p.DataDir} {
		info, err := os.Stat(dir)
		require.NoError(t, err)
		assert.True(t, info.IsDir(), "%s should be a directory", dir)
		assert.Equal(t, dirPerm, info.Mode().Perm(), "%s should be owner-only", dir)
	}
}
