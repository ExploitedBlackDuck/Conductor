// Package history serves the operation-history browser and the "what was moved"
// export (§7.7, §7.11.7). It exposes the intention-revealing queries the UI
// needs over the persisted operations, and produces a CSV/JSON export that
// bundles the operations with the tamper-evident audit trail and its
// verification status (§7.8). Every export is itself recorded in the audit log.
package history

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/conductor-app/conductor/internal/core/audit"
	"github.com/conductor-app/conductor/internal/core/domain"
)

// Store is the persistence port for history queries (satisfied by sqlitestore).
type Store interface {
	RecentOperations(ctx context.Context, limit int) ([]domain.Operation, error)
	OperationsByRemote(ctx context.Context, remote string) ([]domain.Operation, error)
	OperationsInRange(ctx context.Context, from, to time.Time) ([]domain.Operation, error)
	DestructiveOperations(ctx context.Context) ([]domain.Operation, error)
	OperationByID(ctx context.Context, id string) (domain.Operation, []domain.OperationOption, bool, error)
	ClearHistory(ctx context.Context) (int64, error)
}

// AuditLog verifies and lists the audit chain, records new entries, and signs
// the chain head (§7.8, ADR-0010).
type AuditLog interface {
	Verify(ctx context.Context) (audit.Result, error)
	Entries(ctx context.Context) ([]domain.AuditEntry, error)
	Record(ctx context.Context, action domain.AuditAction, subject string, detail any) (domain.AuditEntry, error)
	SignHead(ctx context.Context) (domain.AuditAnchor, error)
}

// Config configures the Service.
type Config struct {
	Store Store
	Audit AuditLog
}

// Service answers history queries and produces exports.
type Service struct {
	cfg Config
}

// New constructs the Service.
func New(cfg Config) *Service { return &Service{cfg: cfg} }

// Recent returns the most recent operations, newest first.
func (s *Service) Recent(ctx context.Context, limit int) ([]domain.Operation, error) {
	return s.cfg.Store.RecentOperations(ctx, limit)
}

// ByRemote returns operations touching a remote, newest first.
func (s *Service) ByRemote(ctx context.Context, remote string) ([]domain.Operation, error) {
	return s.cfg.Store.OperationsByRemote(ctx, remote)
}

// InRange returns operations started within [from, to), newest first.
func (s *Service) InRange(ctx context.Context, from, to time.Time) ([]domain.Operation, error) {
	return s.cfg.Store.OperationsInRange(ctx, from, to)
}

// Destructive returns operations that removed or could overwrite data (§7.4).
func (s *Service) Destructive(ctx context.Context) ([]domain.Operation, error) {
	return s.cfg.Store.DestructiveOperations(ctx)
}

// Detail returns one operation and the exact options it used. The bool is false
// when no operation has that id.
func (s *Service) Detail(ctx context.Context, id string) (domain.Operation, []domain.OperationOption, bool, error) {
	return s.cfg.Store.OperationByID(ctx, id)
}

// VerifyAudit reports whether the audit chain is intact (§7.11.8).
func (s *Service) VerifyAudit(ctx context.Context) (audit.Result, error) {
	return s.cfg.Audit.Verify(ctx)
}

// AuditEntries returns the full audit chain in order (§7.11.8).
func (s *Service) AuditEntries(ctx context.Context) ([]domain.AuditEntry, error) {
	return s.cfg.Audit.Entries(ctx)
}

// SignAuditHead signs the current audit chain head (ADR-0010), called
// periodically and on clean shutdown so the head is anchored.
func (s *Service) SignAuditHead(ctx context.Context) error {
	_, err := s.cfg.Audit.SignHead(ctx)
	return err
}

// ClearHistory deletes operation rows and their sealed logs; the append-only
// audit log is preserved (§7.11.7).
func (s *Service) ClearHistory(ctx context.Context) (int64, error) {
	return s.cfg.Store.ClearHistory(ctx)
}

// Format selects the export serialization.
type Format string

const (
	// FormatJSON bundles operations, the audit trail, and chain status.
	FormatJSON Format = "json"
	// FormatCSV is a flat operations table for spreadsheets.
	FormatCSV Format = "csv"
)

// ExportRequest scopes and formats an export. A zero From/To means unbounded;
// an empty Remote means all remotes.
type ExportRequest struct {
	Format Format
	Remote string
	From   time.Time
	To     time.Time
}

// Export produces a "what was moved" export for the requested scope and records
// the export in the audit log (§7.7, §7.8). The returned bytes are the file
// contents; the caller writes them to disk or streams them to the UI.
func (s *Service) Export(ctx context.Context, req ExportRequest) ([]byte, error) {
	// Sign the current chain head before exporting so the tamper-evident record
	// carries a fresh signed head (ADR-0010, §7.8). A signing failure must not
	// block the export — the existing anchor and the chain still verify.
	if _, serr := s.cfg.Audit.SignHead(ctx); serr != nil {
		// best-effort: proceed with whatever anchor already exists.
		_ = serr
	}

	ops, err := s.scopedOperations(ctx, req)
	if err != nil {
		return nil, err
	}

	var out []byte
	switch req.Format {
	case FormatCSV:
		out, err = exportCSV(ops)
	case FormatJSON, "":
		out, err = s.exportJSON(ctx, ops)
	default:
		return nil, fmt.Errorf("unknown export format %q", req.Format)
	}
	if err != nil {
		return nil, err
	}

	// Record the export itself: an export is a consequential, audited action.
	if _, err := s.cfg.Audit.Record(ctx, domain.ActionExport, "history", map[string]any{
		"format": string(req.Format), "remote": req.Remote, "operations": len(ops),
	}); err != nil {
		return nil, fmt.Errorf("recording export: %w", err)
	}
	return out, nil
}

// scopedOperations resolves the operations selected by an ExportRequest.
func (s *Service) scopedOperations(ctx context.Context, req ExportRequest) ([]domain.Operation, error) {
	switch {
	case req.Remote != "":
		return s.cfg.Store.OperationsByRemote(ctx, req.Remote)
	case !req.From.IsZero() || !req.To.IsZero():
		to := req.To
		if to.IsZero() {
			to = time.Unix(1<<62, 0) // effectively unbounded upper edge
		}
		return s.cfg.Store.OperationsInRange(ctx, req.From, to)
	default:
		return s.cfg.Store.RecentOperations(ctx, 0)
	}
}

// ExportOperation is one operation in a JSON export, including the exact options
// it used so the record is self-describing (§7.7).
type ExportOperation struct {
	domain.Operation
	Options []domain.OperationOption `json:"options"`
}

// ExportBundle is the JSON export shape: the operations, the audit trail, and
// whether the chain verified at export time.
type ExportBundle struct {
	ExportedFormat string              `json:"format"`
	Operations     []ExportOperation   `json:"operations"`
	Audit          []domain.AuditEntry `json:"audit"`
	AuditIntact    bool                `json:"auditIntact"`
	AuditReason    string              `json:"auditReason,omitempty"`
}

func (s *Service) exportJSON(ctx context.Context, ops []domain.Operation) ([]byte, error) {
	bundle := ExportBundle{ExportedFormat: string(FormatJSON), Operations: make([]ExportOperation, 0, len(ops))}
	for _, op := range ops {
		_, opts, _, err := s.cfg.Store.OperationByID(ctx, op.ID)
		if err != nil {
			return nil, err
		}
		bundle.Operations = append(bundle.Operations, ExportOperation{Operation: op, Options: opts})
	}

	res, err := s.cfg.Audit.Verify(ctx)
	if err != nil {
		return nil, err
	}
	bundle.AuditIntact = res.Intact
	bundle.AuditReason = res.Reason

	entries, err := s.cfg.Audit.Entries(ctx)
	if err != nil {
		return nil, err
	}
	bundle.Audit = entries

	out, err := json.MarshalIndent(bundle, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshalling export: %w", err)
	}
	return out, nil
}

func exportCSV(ops []domain.Operation) ([]byte, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if err := w.Write([]string{
		"id", "kind", "src", "dst", "started_at", "ended_at",
		"bytes_moved", "files_moved", "result", "rclone_version",
	}); err != nil {
		return nil, fmt.Errorf("writing csv header: %w", err)
	}
	for _, op := range ops {
		ended := ""
		if !op.EndedAt.IsZero() {
			ended = op.EndedAt.UTC().Format(time.RFC3339)
		}
		if err := w.Write([]string{
			op.ID, string(op.Kind), op.Src, op.Dst,
			op.StartedAt.UTC().Format(time.RFC3339), ended,
			strconv.FormatInt(op.BytesMoved, 10), strconv.FormatInt(op.FilesMoved, 10),
			string(op.Result), op.RcloneVersion,
		}); err != nil {
			return nil, fmt.Errorf("writing csv row: %w", err)
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return nil, fmt.Errorf("flushing csv: %w", err)
	}
	return buf.Bytes(), nil
}
