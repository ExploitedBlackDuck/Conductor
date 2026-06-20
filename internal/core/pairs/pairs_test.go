package pairs

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/conductor-app/conductor/internal/core/coreerr"
	"github.com/conductor-app/conductor/internal/core/domain"
	"github.com/conductor-app/conductor/internal/core/options"
	"github.com/conductor-app/conductor/internal/core/transfers"
)

// memStore is an in-memory pairs.Store for the service tests.
type memStore struct {
	pairs    map[string]domain.SavedPair
	profiles map[string]domain.Profile
	ceilings map[string]domain.RemoteCeiling
}

func newMemStore() *memStore {
	return &memStore{
		pairs:    map[string]domain.SavedPair{},
		profiles: map[string]domain.Profile{},
		ceilings: map[string]domain.RemoteCeiling{},
	}
}

func (m *memStore) Pair(_ context.Context, id string) (domain.SavedPair, bool, error) {
	p, ok := m.pairs[id]
	return p, ok, nil
}
func (m *memStore) Pairs(context.Context) ([]domain.SavedPair, error) { return nil, nil }
func (m *memStore) SavePair(_ context.Context, p domain.SavedPair) error {
	m.pairs[p.ID] = p
	return nil
}

func (m *memStore) DeletePair(_ context.Context, id string) error { delete(m.pairs, id); return nil }

func (m *memStore) TouchPairRun(_ context.Context, id string, at time.Time) error {
	p := m.pairs[id]
	p.LastRun = at
	m.pairs[id] = p
	return nil
}

func (m *memStore) Profile(_ context.Context, id string) (domain.Profile, bool, error) {
	p, ok := m.profiles[id]
	return p, ok, nil
}
func (m *memStore) Profiles(context.Context) ([]domain.Profile, error) { return nil, nil }
func (m *memStore) SaveProfile(_ context.Context, p domain.Profile) error {
	m.profiles[p.ID] = p
	return nil
}

func (m *memStore) DeleteProfile(_ context.Context, id string) error {
	delete(m.profiles, id)
	return nil
}

func (m *memStore) Ceiling(_ context.Context, r string) (domain.RemoteCeiling, bool, error) {
	c, ok := m.ceilings[r]
	return c, ok, nil
}

func (m *memStore) Ceilings(context.Context) ([]domain.RemoteCeiling, error) {
	out := make([]domain.RemoteCeiling, 0, len(m.ceilings))
	for _, c := range m.ceilings {
		out = append(out, c)
	}
	return out, nil
}

func (m *memStore) SetCeiling(_ context.Context, c domain.RemoteCeiling) error {
	m.ceilings[c.Remote] = c
	return nil
}

// fakeRunner captures the request it was asked to start.
type fakeRunner struct {
	last    transfers.RunRequest
	started int
}

func (f *fakeRunner) Start(_ context.Context, req transfers.RunRequest) (transfers.RunHandle, error) {
	f.last = req
	f.started++
	return transfers.RunHandle{OperationID: "op-1", JobID: 1}, nil
}

// fakeAudit records the actions it was asked to record.
type fakeAudit struct{ actions []domain.AuditAction }

func (a *fakeAudit) Record(_ context.Context, action domain.AuditAction, _ string, _ any) (domain.AuditEntry, error) {
	a.actions = append(a.actions, action)
	return domain.AuditEntry{}, nil
}

func newService(t *testing.T, store Store, runner Runner, audit Auditor) *Service {
	t.Helper()
	cat, err := options.Load()
	require.NoError(t, err)
	return New(Config{
		Store:    store,
		Runner:   runner,
		Catalog:  cat,
		Audit:    audit,
		Defaults: options.Ceilings{Transfers: 4, Checkers: 8},
	})
}

// TestNewPairDefaultsToDryRun is the P7 gate: a bisync pair that has never run
// must run as a dry-run, and the run is recorded so the next run is live.
func TestNewPairDefaultsToDryRun(t *testing.T) {
	t.Parallel()
	store := newMemStore()
	store.pairs["pair-1"] = domain.SavedPair{
		ID: "pair-1", Name: "mirror", Kind: domain.PairBisync, Path1: "/home", Path2: "gdrive:backup",
	}
	runner := &fakeRunner{}
	svc := newService(t, store, runner, nil)

	_, err := svc.Run(context.Background(), "pair-1", false)
	require.NoError(t, err)

	assert.Equal(t, 1, runner.started, "the run is started exactly once")
	assert.Equal(t, "true", runner.last.Selection.Single["--dry-run"], "a new pair's first run is a dry-run")
	assert.Equal(t, domain.KindBisync, runner.last.Kind)

	// The pair is now marked as run.
	got, _, _ := store.Pair(context.Background(), "pair-1")
	assert.True(t, got.HasRun())
}

// TestRunPairAfterFirstRunIsLive proves that once a pair has run, the dry-run
// default no longer applies.
func TestRunPairAfterFirstRunIsLive(t *testing.T) {
	t.Parallel()
	store := newMemStore()
	store.pairs["pair-1"] = domain.SavedPair{
		ID: "pair-1", Name: "mirror", Kind: domain.PairBisync,
		Path1: "/home", Path2: "gdrive:backup",
		LastRun: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	}
	runner := &fakeRunner{}
	svc := newService(t, store, runner, nil)

	_, err := svc.Run(context.Background(), "pair-1", true)
	require.NoError(t, err)

	assert.NotEqual(t, "true", runner.last.Selection.Single["--dry-run"],
		"a previously-run pair is not forced to dry-run")
}

// TestRunAppliesProfileAndResolvesCeilings proves profile options become the
// selection and per-remote ceilings tighten the global defaults.
func TestRunAppliesProfileAndResolvesCeilings(t *testing.T) {
	t.Parallel()
	store := newMemStore()
	store.profiles["prof-1"] = domain.Profile{
		ID: "prof-1", Name: "safe", Kind: domain.KindSync,
		Options: []domain.ProfileOption{{Flag: "--checksum", Value: "true"}},
	}
	store.ceilings["s3"] = domain.RemoteCeiling{Remote: "s3", Transfers: 2}
	store.pairs["pair-1"] = domain.SavedPair{
		ID: "pair-1", Name: "mirror", Kind: domain.PairSync,
		Path1: "/home", Path2: "s3:bucket", ProfileID: "prof-1",
		LastRun: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC), // already run, no forced dry-run
	}
	runner := &fakeRunner{}
	svc := newService(t, store, runner, nil)

	_, err := svc.Run(context.Background(), "pair-1", true)
	require.NoError(t, err)

	assert.Equal(t, "true", runner.last.Selection.Single["--checksum"], "profile options become the selection")
	assert.Equal(t, domain.Endpoint{Remote: "s3", Path: "bucket"}, runner.last.Dst)
	// Global default transfers is 4; the s3 ceiling of 2 wins.
	assert.Equal(t, 2, runner.last.Ceilings.Transfers)
	assert.Equal(t, 8, runner.last.Ceilings.Checkers, "the unconstrained dimension keeps the global default")
}

func TestRunMissingPair(t *testing.T) {
	t.Parallel()
	svc := newService(t, newMemStore(), &fakeRunner{}, nil)
	_, err := svc.Run(context.Background(), "nope", false)
	require.Error(t, err)
	code, _ := coreerr.CodeOf(err)
	assert.Equal(t, coreerr.CodeOptionInvalid, code)
}

// TestSetCeilingIsAudited proves a governance-ceiling change is recorded (§7.8).
func TestSetCeilingIsAudited(t *testing.T) {
	t.Parallel()
	audit := &fakeAudit{}
	svc := newService(t, newMemStore(), &fakeRunner{}, audit)

	require.NoError(t, svc.SetCeiling(context.Background(), domain.RemoteCeiling{Remote: "s3", Transfers: 2}))
	assert.Contains(t, audit.actions, domain.ActionGovernanceCeilingSet)
}

// TestPairReferencingMissingProfileFails proves a dangling profile reference is
// a clear, coded error rather than a silently empty selection.
func TestPairReferencingMissingProfileFails(t *testing.T) {
	t.Parallel()
	store := newMemStore()
	store.pairs["pair-1"] = domain.SavedPair{
		ID: "pair-1", Name: "n", Kind: domain.PairSync, Path1: "a:", Path2: "b:", ProfileID: "ghost",
		LastRun: time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC),
	}
	svc := newService(t, store, &fakeRunner{}, nil)

	_, err := svc.Run(context.Background(), "pair-1", true)
	require.Error(t, err)
	code, _ := coreerr.CodeOf(err)
	assert.Equal(t, coreerr.CodeOptionInvalid, code)
}
