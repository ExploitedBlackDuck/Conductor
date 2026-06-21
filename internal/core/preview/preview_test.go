package preview

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/conductor-app/conductor/internal/core/coreerr"
	"github.com/conductor-app/conductor/internal/core/daemon"
	"github.com/conductor-app/conductor/internal/core/domain"
	"github.com/conductor-app/conductor/internal/core/options"
)

// fakeRunner captures the spec it was asked to run and returns canned output,
// standing in for a real rclone dry-run subprocess.
type fakeRunner struct {
	out  []byte
	err  error
	spec daemon.Spec
}

func (f *fakeRunner) Output(_ context.Context, spec daemon.Spec) ([]byte, error) {
	f.spec = spec
	return f.out, f.err
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "changeset", "testdata", name))
	require.NoError(t, err)
	return b
}

func TestPreviewSyncUsesCombinedAndParses(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{out: fixture(t, "sync_combined.txt")}
	svc := New(Config{BinaryPath: "/bin/rclone", Runner: r})

	built := options.Built{Argv: []string{"--checksum"}}
	cs, err := svc.Preview(context.Background(), domain.KindSync,
		domain.Endpoint{Remote: "s3", Path: "data"}, domain.Endpoint{Path: "/local"}, built)
	require.NoError(t, err)

	// Parsed the real --combined fixture.
	assert.Equal(t, 2, cs.CreateCount)
	assert.Equal(t, 1, cs.UpdateCount)
	assert.Equal(t, 1, cs.DeleteCount)
	assert.True(t, cs.HasDeletes())

	// Assembled the dry-run argv: subcommand, resolved endpoints, the real run's
	// flags, then the capture flags.
	assert.Equal(t, []string{"sync", "s3:data", "/local", "--checksum", "--dry-run", "--combined", "-"}, r.spec.Args)
	assert.Equal(t, "/bin/rclone", r.spec.Path)
}

func TestPreviewBisyncUsesJSONLog(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{out: fixture(t, "bisync_resync_dryrun.ndjson")}
	svc := New(Config{BinaryPath: "/bin/rclone", Runner: r})

	built := options.Built{Argv: []string{"--resync"}}
	cs, err := svc.Preview(context.Background(), domain.KindBisync,
		domain.Endpoint{Path: "/a"}, domain.Endpoint{Path: "/b"}, built)
	require.NoError(t, err)

	assert.Equal(t, 2, cs.CreateCount, "bisync resync copies are reported as creates")
	assert.Equal(t, 0, cs.DeleteCount)
	assert.Equal(t, []string{"bisync", "/a", "/b", "--resync", "--dry-run", "--use-json-log"}, r.spec.Args)
}

func TestPreviewAppendsConfigPath(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{out: []byte("")}
	svc := New(Config{BinaryPath: "/bin/rclone", ConfigPath: "/cfg/rclone.conf", Runner: r})

	_, err := svc.Preview(context.Background(), domain.KindSync,
		domain.Endpoint{Path: "/a"}, domain.Endpoint{Path: "/b"}, options.Built{})
	require.NoError(t, err)
	assert.Equal(t, []string{"sync", "/a", "/b", "--dry-run", "--combined", "-", "--config", "/cfg/rclone.conf"}, r.spec.Args)
}

func TestPreviewRunnerFailureIsCoded(t *testing.T) {
	t.Parallel()
	r := &fakeRunner{err: errors.New("exit status 1")}
	svc := New(Config{BinaryPath: "/bin/rclone", Runner: r})

	_, err := svc.Preview(context.Background(), domain.KindSync,
		domain.Endpoint{Path: "/a"}, domain.Endpoint{Path: "/b"}, options.Built{})
	require.Error(t, err)
	code, _ := coreerr.CodeOf(err)
	assert.Equal(t, coreerr.CodeDryRunPreviewFailed, code)
}

func TestPreviewUnsupportedKind(t *testing.T) {
	t.Parallel()
	svc := New(Config{BinaryPath: "/bin/rclone", Runner: &fakeRunner{}})
	_, err := svc.Preview(context.Background(), domain.KindMount,
		domain.Endpoint{Path: "/a"}, domain.Endpoint{Path: "/b"}, options.Built{})
	require.Error(t, err)
	code, _ := coreerr.CodeOf(err)
	assert.Equal(t, coreerr.CodeOptionInvalid, code)
}
