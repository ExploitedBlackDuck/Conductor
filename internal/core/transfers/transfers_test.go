package transfers

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/chacha20poly1305"

	"github.com/conductor-app/conductor/internal/core/coreerr"
	"github.com/conductor-app/conductor/internal/core/domain"
	"github.com/conductor-app/conductor/internal/core/options"
	"github.com/conductor-app/conductor/internal/core/secrets"
)

// fakeRC drives a scripted job: it reports running for a few polls, then
// finished, and records stop calls.
type fakeRC struct {
	mu          sync.Mutex
	pollsToDone int
	polls       int
	stopped     bool
	jobErr      bool
}

func (f *fakeRC) SyncCopy(context.Context, string, string, map[string]any, map[string][]string, bool) (int64, error) {
	return 7, nil
}

func (f *fakeRC) SyncMove(context.Context, string, string, map[string]any, map[string][]string, bool, bool) (int64, error) {
	return 7, nil
}

func (f *fakeRC) JobStop(context.Context, int64) error {
	f.mu.Lock()
	f.stopped = true
	f.mu.Unlock()
	return nil
}

func (f *fakeRC) JobStatus(context.Context, int64) (domain.JobStatus, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.polls++
	done := f.polls >= f.pollsToDone || f.stopped
	return domain.JobStatus{ID: 7, Finished: done, Success: done && !f.stopped && !f.jobErr}, nil
}

func (f *fakeRC) CoreStatsForGroup(context.Context, string) (domain.TransferStats, error) {
	return domain.TransferStats{Bytes: 2048, Transfers: 2}, nil
}

// fakeStore records inserted operations.
type fakeStore struct {
	mu  sync.Mutex
	ops []domain.Operation
	log *domain.CapturedLog
}

func (s *fakeStore) InsertOperation(_ context.Context, op domain.Operation, _ []domain.OperationOption, log *domain.CapturedLog) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ops = append(s.ops, op)
	s.log = log
	return nil
}

func (s *fakeStore) last() (domain.Operation, *domain.CapturedLog, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.ops) == 0 {
		return domain.Operation{}, nil, false
	}
	return s.ops[len(s.ops)-1], s.log, true
}

// fakeAudit records audit actions.
type fakeAudit struct {
	mu      sync.Mutex
	actions []domain.AuditAction
}

func (a *fakeAudit) Record(_ context.Context, action domain.AuditAction, _ string, _ any) (domain.AuditEntry, error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.actions = append(a.actions, action)
	return domain.AuditEntry{}, nil
}

func (a *fakeAudit) recorded() []domain.AuditAction {
	a.mu.Lock()
	defer a.mu.Unlock()
	out := make([]domain.AuditAction, len(a.actions))
	copy(out, a.actions)
	return out
}

func newSealer(t *testing.T) *secrets.Sealer {
	t.Helper()
	s, err := secrets.NewSealer(bytes.Repeat([]byte{0x11}, chacha20poly1305.KeySize))
	require.NoError(t, err)
	return s
}

func newService(t *testing.T, rc *fakeRC, store *fakeStore, audit *fakeAudit) *Service {
	t.Helper()
	cat, err := options.Load()
	require.NoError(t, err)
	return New(Config{
		RC:        func() (RC, error) { return rc, nil },
		Store:     store,
		Audit:     audit,
		Sealer:    newSealer(t),
		Catalog:   cat,
		Version:   "v1.74.3",
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
		PollEvery: time.Millisecond,
	})
}

func TestCopyPersistsOperationAndAudit(t *testing.T) {
	t.Parallel()
	rc := &fakeRC{pollsToDone: 3}
	store := &fakeStore{}
	audit := &fakeAudit{}
	svc := newService(t, rc, store, audit)

	h, err := svc.Start(context.Background(), RunRequest{
		Kind:      domain.KindCopy,
		Src:       domain.Endpoint{Remote: "example-s3", Path: "data"},
		Dst:       domain.Endpoint{Path: "/local/backup"},
		Selection: options.Selection{Single: map[string]string{"--transfers": "4"}},
	})
	require.NoError(t, err)
	assert.NotEmpty(t, h.OperationID)

	require.Eventually(t, func() bool { _, _, ok := store.last(); return ok }, 2*time.Second, 2*time.Millisecond)

	op, log, _ := store.last()
	assert.Equal(t, domain.ResultSuccess, op.Result)
	assert.Equal(t, int64(2048), op.BytesMoved)
	assert.Equal(t, int64(2), op.FilesMoved)
	require.NotNil(t, log, "a sealed captured log must be persisted")
	assert.NotEmpty(t, log.SealedBytes)
	assert.NotEmpty(t, log.Nonce)

	actions := audit.recorded()
	assert.Contains(t, actions, domain.ActionOperationStart)
	assert.Contains(t, actions, domain.ActionOperationStop)
}

func TestCancelRecordsCancelled(t *testing.T) {
	t.Parallel()
	rc := &fakeRC{pollsToDone: 1000} // would run long without cancel
	store := &fakeStore{}
	audit := &fakeAudit{}
	svc := newService(t, rc, store, audit)

	h, err := svc.Start(context.Background(), RunRequest{
		Kind: domain.KindCopy,
		Src:  domain.Endpoint{Path: "/a"},
		Dst:  domain.Endpoint{Path: "/b"},
	})
	require.NoError(t, err)

	require.NoError(t, svc.Cancel(context.Background(), h.OperationID))

	require.Eventually(t, func() bool { _, _, ok := store.last(); return ok }, 2*time.Second, 2*time.Millisecond)
	op, _, _ := store.last()
	assert.Equal(t, domain.ResultCancelled, op.Result, "a stopped job is cancelled, not failed")
}

func TestCancelUnknownOperation(t *testing.T) {
	t.Parallel()
	svc := newService(t, &fakeRC{pollsToDone: 1}, &fakeStore{}, &fakeAudit{})
	err := svc.Cancel(context.Background(), "does-not-exist")
	require.Error(t, err)
	code, _ := coreerr.CodeOf(err)
	assert.Equal(t, coreerr.CodeJobCancelled, code)
}

func TestCloseFinalizesActiveRuns(t *testing.T) {
	t.Parallel()
	rc := &fakeRC{pollsToDone: 1000}
	store := &fakeStore{}
	svc := newService(t, rc, store, &fakeAudit{})

	_, err := svc.Start(context.Background(), RunRequest{
		Kind: domain.KindCopy, Src: domain.Endpoint{Path: "/a"}, Dst: domain.Endpoint{Path: "/b"},
	})
	require.NoError(t, err)

	svc.Close() // must finalize and not hang

	_, _, ok := store.last()
	assert.True(t, ok, "shutdown should finalize active runs")
}
