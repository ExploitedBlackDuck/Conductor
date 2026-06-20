package mounts

import (
	"context"
	"errors"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/conductor-app/conductor/internal/core/domain"
)

type fakeRC struct {
	mu          sync.Mutex
	mounted     map[string]string // mountPoint -> fs
	mountErr    error
	unmountErr  error
	mountCalls  int
	unmountCall int
}

func newFakeRC() *fakeRC { return &fakeRC{mounted: map[string]string{}} }

func (f *fakeRC) MountMount(_ context.Context, fs, mountPoint, _ string) error {
	if f.mountErr != nil {
		return f.mountErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.mounted[mountPoint] = fs
	f.mountCalls++
	return nil
}

func (f *fakeRC) MountUnmount(_ context.Context, mountPoint string) error {
	if f.unmountErr != nil {
		return f.unmountErr
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	delete(f.mounted, mountPoint)
	f.unmountCall++
	return nil
}

func (f *fakeRC) ListMounts(context.Context) ([]domain.Mount, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []domain.Mount
	for mp, fs := range f.mounted {
		out = append(out, domain.Mount{Fs: fs, MountPoint: mp})
	}
	return out, nil
}

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

func TestMountUnmountRoundTripWithAudit(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rc := newFakeRC()
	audit := &fakeAudit{}
	svc := New(func() (RC, error) { return rc, nil }, audit)

	require.NoError(t, svc.Mount(ctx, "mem:bucket", "/mnt/bucket", ""))
	mounts, err := svc.List(ctx)
	require.NoError(t, err)
	require.Len(t, mounts, 1)
	assert.Equal(t, "mem:bucket", mounts[0].Fs)

	require.NoError(t, svc.Unmount(ctx, "/mnt/bucket"))
	mounts, err = svc.List(ctx)
	require.NoError(t, err)
	assert.Empty(t, mounts)

	assert.Equal(t, []domain.AuditAction{domain.ActionMount, domain.ActionUnmount}, audit.actions)
}

func TestMountErrorIsNotAudited(t *testing.T) {
	t.Parallel()
	rc := newFakeRC()
	rc.mountErr = errors.New("mount failed")
	audit := &fakeAudit{}
	svc := New(func() (RC, error) { return rc, nil }, audit)

	require.Error(t, svc.Mount(context.Background(), "mem:bucket", "/mnt/x", ""))
	assert.Empty(t, audit.actions, "a failed mount must not be recorded as having happened")
}
