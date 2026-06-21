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
	// started records which start method fired and the bisync flags it carried,
	// so tests can assert the correct rc endpoint and arguments were used.
	startedKind  string
	bisyncResync bool
	bisyncDryRun bool
}

func (f *fakeRC) SyncCopy(context.Context, string, string, map[string]any, map[string][]string, bool) (int64, error) {
	f.mu.Lock()
	f.startedKind = "copy"
	f.mu.Unlock()
	return 7, nil
}

func (f *fakeRC) SyncMove(context.Context, string, string, map[string]any, map[string][]string, bool, bool) (int64, error) {
	f.mu.Lock()
	f.startedKind = "move"
	f.mu.Unlock()
	return 7, nil
}

func (f *fakeRC) SyncSync(context.Context, string, string, map[string]any, map[string][]string, bool) (int64, error) {
	f.mu.Lock()
	f.startedKind = "sync"
	f.mu.Unlock()
	return 7, nil
}

func (f *fakeRC) SyncBisync(_ context.Context, _, _ string, resync, dryRun bool, _ map[string]any, _ bool) (int64, error) {
	f.mu.Lock()
	f.startedKind = "bisync"
	f.bisyncResync = resync
	f.bisyncDryRun = dryRun
	f.mu.Unlock()
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

// helper: a bisync/sync request with the given selection.
func runReq(kind domain.OperationKind, sel map[string]string, ack bool) RunRequest {
	return RunRequest{
		Kind:         kind,
		Src:          domain.Endpoint{Remote: "a", Path: "x"},
		Dst:          domain.Endpoint{Remote: "b", Path: "y"},
		Selection:    options.Selection{Single: sel},
		Acknowledged: ack,
	}
}

// previewed is a sample shown change set with a delete, standing in for the
// dry-run preview the operator confirmed against (ADR-0015).
func previewed() *domain.ChangeSet {
	return &domain.ChangeSet{
		Deletes:     []domain.FileChange{{Kind: domain.ChangeDelete, Path: "gone.txt"}},
		CreateCount: 1, DeleteCount: 1,
		Creates: []domain.FileChange{{Kind: domain.ChangeCreate, Path: "new.txt"}},
	}
}

// TestDestructiveRefusedWithoutPreview is the ADR-0015 gate: a destructive sync
// must not run until it has been previewed with a dry-run change set — even if
// the operator tries to acknowledge in the abstract.
func TestDestructiveRefusedWithoutPreview(t *testing.T) {
	t.Parallel()
	svc := newService(t, &fakeRC{pollsToDone: 1}, &fakeStore{}, &fakeAudit{})

	req := runReq(domain.KindSync, nil, true) // acknowledged, but never previewed
	_, err := svc.Start(context.Background(), req)
	require.Error(t, err)
	code, _ := coreerr.CodeOf(err)
	assert.Equal(t, coreerr.CodeDryRunPreviewRequired, code)
}

// TestDestructiveRefusedWithoutAckAfterPreview proves that a shown change set is
// necessary but not sufficient: the operator must still acknowledge it.
func TestDestructiveRefusedWithoutAckAfterPreview(t *testing.T) {
	t.Parallel()
	svc := newService(t, &fakeRC{pollsToDone: 1}, &fakeStore{}, &fakeAudit{})

	req := runReq(domain.KindSync, nil, false)
	req.ShownChangeSet = previewed()
	_, err := svc.Start(context.Background(), req)
	require.Error(t, err)
	code, _ := coreerr.CodeOf(err)
	assert.Equal(t, coreerr.CodeDestructiveNotConfirmed, code)
}

// TestSyncWithAcknowledgedPreviewRuns proves the sync proceeds once previewed
// and acknowledged, hits the sync/sync endpoint, and records the risk ack.
func TestSyncWithAcknowledgedPreviewRuns(t *testing.T) {
	t.Parallel()
	rc := &fakeRC{pollsToDone: 1}
	audit := &fakeAudit{}
	svc := newService(t, rc, &fakeStore{}, audit)

	req := runReq(domain.KindSync, nil, true)
	req.ShownChangeSet = previewed()
	_, err := svc.Start(context.Background(), req)
	require.NoError(t, err)

	rc.mu.Lock()
	assert.Equal(t, "sync", rc.startedKind)
	rc.mu.Unlock()
	assert.Contains(t, audit.recorded(), domain.ActionRiskAcknowledged)
}

// TestBisyncResyncRefusedWithoutPreview is the ADR-0015 gate for the destructive
// bisync baseline reset: --resync must not run until previewed.
func TestBisyncResyncRefusedWithoutPreview(t *testing.T) {
	t.Parallel()
	svc := newService(t, &fakeRC{pollsToDone: 1}, &fakeStore{}, &fakeAudit{})

	_, err := svc.Start(context.Background(), runReq(domain.KindBisync, map[string]string{"--resync": "true"}, true))
	require.Error(t, err)
	code, _ := coreerr.CodeOf(err)
	assert.Equal(t, coreerr.CodeDryRunPreviewRequired, code)
}

// TestDryRunNeedsNoAcknowledgement proves dry-run is always one click away
// (§7.4): a sync simulated with --dry-run runs without acknowledgement.
func TestDryRunNeedsNoAcknowledgement(t *testing.T) {
	t.Parallel()
	rc := &fakeRC{pollsToDone: 1}
	svc := newService(t, rc, &fakeStore{}, &fakeAudit{})

	_, err := svc.Start(context.Background(), runReq(domain.KindSync, map[string]string{"--dry-run": "true"}, false))
	require.NoError(t, err)
	rc.mu.Lock()
	assert.Equal(t, "sync", rc.startedKind)
	rc.mu.Unlock()
}

// TestBisyncPassesResyncAndDryRunFlags proves the selected --resync/--dry-run
// booleans reach the sync/bisync rc call.
func TestBisyncPassesResyncAndDryRunFlags(t *testing.T) {
	t.Parallel()
	rc := &fakeRC{pollsToDone: 1}
	svc := newService(t, rc, &fakeStore{}, &fakeAudit{})

	_, err := svc.Start(context.Background(), runReq(domain.KindBisync, map[string]string{"--resync": "true", "--dry-run": "true"}, true))
	require.NoError(t, err)

	rc.mu.Lock()
	defer rc.mu.Unlock()
	assert.Equal(t, "bisync", rc.startedKind)
	assert.True(t, rc.bisyncResync, "--resync must reach sync/bisync")
	assert.True(t, rc.bisyncDryRun, "--dry-run must reach sync/bisync")
}

// fakePreviewer returns a canned change set and records the kind it saw.
type fakePreviewer struct {
	cs   domain.ChangeSet
	kind domain.OperationKind
}

func (p *fakePreviewer) Preview(_ context.Context, kind domain.OperationKind, _, _ domain.Endpoint, _ options.Built) (domain.ChangeSet, error) {
	p.kind = kind
	return p.cs, nil
}

// TestPreviewRunsThroughPreviewer proves the service validates the selection and
// delegates the dry-run to the previewer, returning its change set (ADR-0015).
func TestPreviewRunsThroughPreviewer(t *testing.T) {
	t.Parallel()
	cat, err := options.Load()
	require.NoError(t, err)
	pv := &fakePreviewer{cs: *previewed()}
	svc := New(Config{
		RC:        func() (RC, error) { return &fakeRC{}, nil },
		Catalog:   cat,
		Previewer: pv,
		Logger:    slog.New(slog.NewTextHandler(io.Discard, nil)),
	})

	cs, err := svc.Preview(context.Background(), runReq(domain.KindSync, nil, false))
	require.NoError(t, err)
	assert.Equal(t, 1, cs.DeleteCount)
	assert.Equal(t, domain.KindSync, pv.kind)
}

// TestPreviewUnavailableIsCoded proves a missing previewer is a typed failure,
// never a silent skip of the gate.
func TestPreviewUnavailableIsCoded(t *testing.T) {
	t.Parallel()
	svc := newService(t, &fakeRC{}, &fakeStore{}, &fakeAudit{}) // no previewer
	_, err := svc.Preview(context.Background(), runReq(domain.KindSync, nil, false))
	require.Error(t, err)
	code, _ := coreerr.CodeOf(err)
	assert.Equal(t, coreerr.CodeDryRunPreviewFailed, code)
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
