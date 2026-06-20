package control

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/conductor-app/conductor/internal/core/daemon"
	"github.com/conductor-app/conductor/internal/core/domain"
	"github.com/conductor-app/conductor/internal/core/livestats"
)

type fakeDaemon struct {
	startErr error
	addr     string
	creds    daemon.Credentials
	running  bool
}

func (f *fakeDaemon) Start(context.Context) error {
	if f.startErr != nil {
		return f.startErr
	}
	f.running = true
	return nil
}
func (f *fakeDaemon) Stop(context.Context) error { f.running = false; return nil }
func (f *fakeDaemon) Addr() (string, error) {
	if !f.running {
		return "", daemon.ErrNotRunning
	}
	return f.addr, nil
}

func (f *fakeDaemon) Credentials() (daemon.Credentials, error) {
	if !f.running {
		return daemon.Credentials{}, daemon.ErrNotRunning
	}
	return f.creds, nil
}

type fakeRC struct {
	remotes    []string
	stats      domain.TransferStats
	remotesErr error
	statsErr   error
}

func (f *fakeRC) Ping(context.Context) error { return nil }
func (f *fakeRC) ConfigListRemotes(context.Context) ([]string, error) {
	return f.remotes, f.remotesErr
}

func (f *fakeRC) CoreStats(context.Context) (domain.TransferStats, error) {
	return f.stats, f.statsErr
}

func (f *fakeRC) RunningJobIDs(context.Context) ([]int64, error) { return nil, nil }

// noopEmitter discards live-stats snapshots.
type noopEmitter struct{}

func (noopEmitter) EmitStats(livestats.Snapshot) {}

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestStartThenStatusReportsRemotesAndStats(t *testing.T) {
	t.Parallel()
	d := &fakeDaemon{addr: "127.0.0.1:5572", creds: daemon.Credentials{User: "u", Pass: "p"}}
	rc := &fakeRC{remotes: []string{"example-s3"}, stats: domain.TransferStats{Bytes: 42}}
	svc := New(d, func(string, string, string) RC { return rc }, quietLogger())

	require.NoError(t, svc.Start(context.Background(), noopEmitter{}))
	t.Cleanup(func() { _ = svc.Stop(context.Background()) })

	st := svc.Status(context.Background())
	assert.True(t, st.DaemonRunning)
	assert.Equal(t, []string{"example-s3"}, st.Remotes)
	assert.Equal(t, int64(42), st.Stats.Bytes)
	assert.NoError(t, st.Err)
}

func TestStartFailureSurfacesInStatus(t *testing.T) {
	t.Parallel()
	startErr := errors.New("rclone missing")
	d := &fakeDaemon{startErr: startErr}
	svc := New(d, func(string, string, string) RC { return &fakeRC{} }, quietLogger())

	require.Error(t, svc.Start(context.Background(), noopEmitter{}))

	st := svc.Status(context.Background())
	assert.False(t, st.DaemonRunning)
	assert.ErrorIs(t, st.Err, startErr)
}

func TestDaemonUpButRCErrorReportsDegraded(t *testing.T) {
	t.Parallel()
	d := &fakeDaemon{addr: "127.0.0.1:5572", creds: daemon.Credentials{User: "u", Pass: "p"}}
	rc := &fakeRC{remotesErr: errors.New("connection refused")}
	svc := New(d, func(string, string, string) RC { return rc }, quietLogger())

	require.NoError(t, svc.Start(context.Background(), noopEmitter{}))
	t.Cleanup(func() { _ = svc.Stop(context.Background()) })

	st := svc.Status(context.Background())
	assert.True(t, st.DaemonRunning)
	assert.Error(t, st.Err)
}
