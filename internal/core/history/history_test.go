package history

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/conductor-app/conductor/internal/core/audit"
	"github.com/conductor-app/conductor/internal/core/domain"
)

// fakeStore is an in-memory history.Store.
type fakeStore struct {
	ops     []domain.Operation
	options map[string][]domain.OperationOption
}

func (f *fakeStore) RecentOperations(_ context.Context, _ int) ([]domain.Operation, error) {
	return f.ops, nil
}

func (f *fakeStore) OperationsByRemote(_ context.Context, remote string) ([]domain.Operation, error) {
	var out []domain.Operation
	for _, o := range f.ops {
		if strings.HasPrefix(o.Src, remote+":") || strings.HasPrefix(o.Dst, remote+":") {
			out = append(out, o)
		}
	}
	return out, nil
}

func (f *fakeStore) OperationsInRange(_ context.Context, from, to time.Time) ([]domain.Operation, error) {
	var out []domain.Operation
	for _, o := range f.ops {
		if !o.StartedAt.Before(from) && o.StartedAt.Before(to) {
			out = append(out, o)
		}
	}
	return out, nil
}

func (f *fakeStore) DestructiveOperations(_ context.Context) ([]domain.Operation, error) {
	var out []domain.Operation
	for _, o := range f.ops {
		if o.Kind.IsDestructive() {
			out = append(out, o)
		}
	}
	return out, nil
}

func (f *fakeStore) OperationByID(_ context.Context, id string) (domain.Operation, []domain.OperationOption, bool, error) {
	for _, o := range f.ops {
		if o.ID == id {
			return o, f.options[id], true, nil
		}
	}
	return domain.Operation{}, nil, false, nil
}

func (f *fakeStore) ClearHistory(_ context.Context) (int64, error) {
	n := int64(len(f.ops))
	f.ops = nil
	return n, nil
}

// fakeAudit is an in-memory AuditLog.
type fakeAudit struct {
	entries  []domain.AuditEntry
	intact   bool
	recorded []domain.AuditAction
}

func (a *fakeAudit) Verify(context.Context) (audit.Result, error) {
	return audit.Result{Intact: a.intact, Count: len(a.entries)}, nil
}
func (a *fakeAudit) Entries(context.Context) ([]domain.AuditEntry, error) { return a.entries, nil }
func (a *fakeAudit) Record(_ context.Context, action domain.AuditAction, _ string, _ any) (domain.AuditEntry, error) {
	a.recorded = append(a.recorded, action)
	return domain.AuditEntry{}, nil
}

func seed() (*fakeStore, *fakeAudit) {
	base := time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC)
	store := &fakeStore{
		ops: []domain.Operation{
			{ID: "op-1", Kind: domain.KindCopy, Src: "s3:data", Dst: "/local", StartedAt: base, BytesMoved: 1024, FilesMoved: 3, Result: domain.ResultSuccess},
			{ID: "op-2", Kind: domain.KindSync, Src: "/local", Dst: "s3:mirror", StartedAt: base.Add(time.Hour), Result: domain.ResultSuccess},
		},
		options: map[string][]domain.OperationOption{
			"op-1": {{Flag: "--checksum", Value: "true", Risk: "passive"}},
		},
	}
	auditLog := &fakeAudit{intact: true, entries: []domain.AuditEntry{{Seq: 1, Action: domain.ActionOperationStart}}}
	return store, auditLog
}

func TestExportJSONRoundTrip(t *testing.T) {
	t.Parallel()
	store, auditLog := seed()
	svc := New(Config{Store: store, Audit: auditLog})

	raw, err := svc.Export(context.Background(), ExportRequest{Format: FormatJSON})
	require.NoError(t, err)

	var bundle ExportBundle
	require.NoError(t, json.Unmarshal(raw, &bundle))

	require.Len(t, bundle.Operations, 2)
	assert.Equal(t, "op-1", bundle.Operations[0].ID)
	assert.Equal(t, domain.KindCopy, bundle.Operations[0].Kind)
	assert.Len(t, bundle.Operations[0].Options, 1, "the exact options used are exported")
	assert.True(t, bundle.AuditIntact)
	assert.Len(t, bundle.Audit, 1, "the audit trail is bundled")

	// The export is itself recorded in the audit log (§7.8).
	assert.Contains(t, auditLog.recorded, domain.ActionExport)
}

func TestExportCSV(t *testing.T) {
	t.Parallel()
	store, auditLog := seed()
	svc := New(Config{Store: store, Audit: auditLog})

	raw, err := svc.Export(context.Background(), ExportRequest{Format: FormatCSV})
	require.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(raw)), "\n")
	require.Len(t, lines, 3) // header + 2 operations
	assert.True(t, strings.HasPrefix(lines[0], "id,kind,src,dst"))
	assert.Contains(t, lines[1], "op-1")
	assert.Contains(t, lines[1], "1024")
	assert.Contains(t, auditLog.recorded, domain.ActionExport)
}

func TestExportByRemoteScope(t *testing.T) {
	t.Parallel()
	store, auditLog := seed()
	svc := New(Config{Store: store, Audit: auditLog})

	raw, err := svc.Export(context.Background(), ExportRequest{Format: FormatJSON, Remote: "s3"})
	require.NoError(t, err)

	var bundle ExportBundle
	require.NoError(t, json.Unmarshal(raw, &bundle))
	assert.Len(t, bundle.Operations, 2, "both ops touch s3")
}

func TestExportUnknownFormat(t *testing.T) {
	t.Parallel()
	store, auditLog := seed()
	svc := New(Config{Store: store, Audit: auditLog})

	_, err := svc.Export(context.Background(), ExportRequest{Format: "xml"})
	require.Error(t, err)
	assert.NotContains(t, auditLog.recorded, domain.ActionExport, "a failed export records nothing")
}

func TestQueryAndClearPassThrough(t *testing.T) {
	t.Parallel()
	store, auditLog := seed()
	svc := New(Config{Store: store, Audit: auditLog})
	ctx := context.Background()

	dest, err := svc.Destructive(ctx)
	require.NoError(t, err)
	require.Len(t, dest, 1)
	assert.Equal(t, "op-2", dest[0].ID)

	n, err := svc.ClearHistory(ctx)
	require.NoError(t, err)
	assert.Equal(t, int64(2), n)
}
