package rcclient_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/conductor-app/conductor/internal/adapters/rcclient"
	"github.com/conductor-app/conductor/internal/core/coreerr"
)

const (
	testUser = "conductor-test"
	testPass = "testpass123"
)

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	require.NoError(t, err)
	return data
}

// fixtureServer serves a mapping of rc path -> fixture file, enforcing basic
// auth and POST, mimicking rcd closely enough to exercise the client.
func fixtureServer(t *testing.T, routes map[string]string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()
	for path, file := range routes {
		body := fixture(t, file)
		mux.HandleFunc("/"+path, func(w http.ResponseWriter, r *http.Request) {
			assert.Equal(t, http.MethodPost, r.Method)
			u, p, ok := r.BasicAuth()
			if !ok || u != testUser || p != testPass {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write(body)
		})
	}
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

func clientFor(t *testing.T, srv *httptest.Server) *rcclient.Client {
	t.Helper()
	addr := strings.TrimPrefix(srv.URL, "http://")
	return rcclient.New(addr, testUser, testPass)
}

func TestCoreStatsIdle(t *testing.T) {
	t.Parallel()
	srv := fixtureServer(t, map[string]string{"core/stats": "core_stats_idle.json"})
	c := clientFor(t, srv)

	stats, err := c.CoreStats(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(0), stats.Bytes)
	assert.Equal(t, int64(0), stats.Transfers)
	assert.Nil(t, stats.ETASeconds)
	assert.Empty(t, stats.Transferring)
}

func TestCoreStatsActive(t *testing.T) {
	t.Parallel()
	srv := fixtureServer(t, map[string]string{"core/stats": "core_stats_active.json"})
	c := clientFor(t, srv)

	stats, err := c.CoreStats(context.Background())
	require.NoError(t, err)
	assert.Equal(t, int64(67108864), stats.Bytes)
	assert.Equal(t, int64(67108864), stats.TotalBytes)
	assert.Equal(t, int64(1), stats.Transfers)
}

func TestCoreStatsTransferring(t *testing.T) {
	t.Parallel()
	srv := fixtureServer(t, map[string]string{"core/stats": "core_stats_transferring.json"})
	c := clientFor(t, srv)

	stats, err := c.CoreStats(context.Background())
	require.NoError(t, err)
	require.Len(t, stats.Transferring, 1)
	f := stats.Transferring[0]
	assert.Equal(t, "big.bin", f.Name)
	assert.Equal(t, int64(24117248), f.Bytes)
	assert.Equal(t, int64(67108864), f.Size)
	assert.Equal(t, 35, f.Percentage)
	require.NotNil(t, stats.ETASeconds)
	assert.InDelta(t, 10, *stats.ETASeconds, 0.001)
}

func TestConfigListRemotes(t *testing.T) {
	t.Parallel()
	srv := fixtureServer(t, map[string]string{"config/listremotes": "config_listremotes.json"})
	c := clientFor(t, srv)

	remotes, err := c.ConfigListRemotes(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []string{"example-local", "example-mem"}, remotes)
}

func TestListMounts(t *testing.T) {
	t.Parallel()
	srv := fixtureServer(t, map[string]string{"mount/listmounts": "mount_listmounts.json"})
	c := clientFor(t, srv)

	mounts, err := c.ListMounts(context.Background())
	require.NoError(t, err)
	require.Len(t, mounts, 1)
	assert.Equal(t, "example-mem:bucket", mounts[0].Fs)
	assert.Equal(t, "/Users/example/mnt/bucket", mounts[0].MountPoint)
	assert.False(t, mounts[0].MountedOn.IsZero())
}

func TestOperationsCheck(t *testing.T) {
	t.Parallel()
	srv := fixtureServer(t, map[string]string{"operations/check": "operations_check.json"})
	c := clientFor(t, srv)

	res, err := c.OperationsCheck(context.Background(), "/src", "s3:dst", false)
	require.NoError(t, err)
	assert.False(t, res.Success)
	assert.Equal(t, "md5", res.HashType)
	assert.Equal(t, []string{"differ.txt"}, res.Differ)
	assert.Equal(t, []string{"onlysrc.txt"}, res.MissingOnDst)
	assert.Equal(t, []string{"onlydst.txt"}, res.MissingOnSrc)
	assert.Len(t, res.Combined, 4)
}

func TestJobList(t *testing.T) {
	t.Parallel()
	srv := fixtureServer(t, map[string]string{"job/list": "job_list.json"})
	c := clientFor(t, srv)

	jobs, err := c.JobList(context.Background())
	require.NoError(t, err)
	assert.Equal(t, []int64{1, 2, 3}, jobs.JobIDs)
	assert.Equal(t, []int64{3}, jobs.RunningIDs)
	assert.Equal(t, []int64{1, 2}, jobs.FinishedIDs)
}

func TestJobStatusFinished(t *testing.T) {
	t.Parallel()
	srv := fixtureServer(t, map[string]string{"job/status": "job_status_finished.json"})
	c := clientFor(t, srv)

	status, err := c.JobStatus(context.Background(), 1)
	require.NoError(t, err)
	assert.Equal(t, int64(1), status.ID)
	assert.True(t, status.Finished)
	assert.True(t, status.Success)
	assert.Empty(t, status.Error)
	assert.False(t, status.StartTime.IsZero())
	assert.False(t, status.EndTime.IsZero())
}

func TestRCErrorMapping(t *testing.T) {
	t.Parallel()
	mux := http.NewServeMux()
	mux.HandleFunc("/core/stats", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"job not found","status":500}`))
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	c := clientFor(t, srv)

	_, err := c.CoreStats(context.Background())
	require.Error(t, err)
	code, ok := coreerr.CodeOf(err)
	require.True(t, ok)
	assert.Equal(t, coreerr.CodeRCRequest, code)
	assert.Contains(t, err.Error(), "job not found")
}

func TestAuthFailureSurfaces(t *testing.T) {
	t.Parallel()
	srv := fixtureServer(t, map[string]string{"core/stats": "core_stats_idle.json"})
	addr := strings.TrimPrefix(srv.URL, "http://")
	c := rcclient.New(addr, testUser, "wrong-password")

	_, err := c.CoreStats(context.Background())
	require.Error(t, err)
	code, ok := coreerr.CodeOf(err)
	require.True(t, ok)
	assert.Equal(t, coreerr.CodeRCRequest, code)
}
