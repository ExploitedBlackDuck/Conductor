package app

import (
	"context"
	"encoding/base64"
	"time"

	"github.com/conductor-app/conductor/internal/core/domain"
	"github.com/conductor-app/conductor/internal/core/history"
)

// OperationDTO is one history row for the frontend (§7.11.7).
type OperationDTO struct {
	ID            string `json:"id"`
	Kind          string `json:"kind"`
	Src           string `json:"src"`
	Dst           string `json:"dst"`
	RcloneVersion string `json:"rcloneVersion"`
	StartedAt     string `json:"startedAt"`
	EndedAt       string `json:"endedAt"`
	BytesMoved    int64  `json:"bytesMoved"`
	FilesMoved    int64  `json:"filesMoved"`
	Result        string `json:"result"`
	Destructive   bool   `json:"destructive"`
}

// OperationOptionDTO is one resolved option used by an operation.
type OperationOptionDTO struct {
	Flag         string `json:"flag"`
	Value        string `json:"value"`
	Risk         string `json:"risk"`
	Acknowledged bool   `json:"acknowledged"`
}

// OperationsResultDTO is a list of operations or a typed error.
type OperationsResultDTO struct {
	Operations []OperationDTO `json:"operations"`
	Error      *ErrorDTO      `json:"error"`
}

// OperationDetailDTO is one operation with the exact options it used.
type OperationDetailDTO struct {
	Operation OperationDTO         `json:"operation"`
	Options   []OperationOptionDTO `json:"options"`
	Found     bool                 `json:"found"`
	Error     *ErrorDTO            `json:"error"`
}

// AuditEntryDTO is one audit-chain entry for the viewer (§7.11.8).
type AuditEntryDTO struct {
	Seq     int64  `json:"seq"`
	At      string `json:"at"`
	Action  string `json:"action"`
	Subject string `json:"subject"`
	Detail  string `json:"detail"`
	Hash    string `json:"hash"`
}

// AuditViewDTO is the audit chain with its verification status. Intact drives
// the green/red chain-verification indicator (§7.11.8).
type AuditViewDTO struct {
	Entries     []AuditEntryDTO `json:"entries"`
	Intact      bool            `json:"intact"`
	BrokenAtSeq int64           `json:"brokenAtSeq"`
	Reason      string          `json:"reason"`
	Error       *ErrorDTO       `json:"error"`
}

// ExportResultDTO carries an export's bytes (base64) and suggested filename.
type ExportResultDTO struct {
	Filename string    `json:"filename"`
	Base64   string    `json:"base64"`
	Error    *ErrorDTO `json:"error"`
}

// RecentHistory returns the most recent operations (§7.11.7).
func (a *App) RecentHistory(limit int) OperationsResultDTO {
	return a.operationsResult(a.history.Recent(context.Background(), limit))
}

// HistoryByRemote returns operations touching a remote.
func (a *App) HistoryByRemote(remote string) OperationsResultDTO {
	return a.operationsResult(a.history.ByRemote(context.Background(), remote))
}

// DestructiveHistory returns operations that removed or could overwrite data.
func (a *App) DestructiveHistory() OperationsResultDTO {
	return a.operationsResult(a.history.Destructive(context.Background()))
}

// OperationDetail returns one operation and the exact options it used.
func (a *App) OperationDetail(id string) OperationDetailDTO {
	op, opts, found, err := a.history.Detail(context.Background(), id)
	if err != nil {
		return OperationDetailDTO{Error: errorToDTO(err)}
	}
	out := OperationDetailDTO{Found: found}
	if found {
		out.Operation = toOperationDTO(op)
		for _, o := range opts {
			out.Options = append(out.Options, OperationOptionDTO{
				Flag: o.Flag, Value: o.Value, Risk: o.Risk, Acknowledged: o.Acknowledged,
			})
		}
	}
	return out
}

// AuditView returns the audit chain and whether it verifies (§7.11.8).
func (a *App) AuditView() AuditViewDTO {
	ctx := context.Background()
	res, err := a.history.VerifyAudit(ctx)
	if err != nil {
		return AuditViewDTO{Error: errorToDTO(err)}
	}
	entries, err := a.history.AuditEntries(ctx)
	if err != nil {
		return AuditViewDTO{Error: errorToDTO(err)}
	}
	out := AuditViewDTO{Intact: res.Intact, BrokenAtSeq: res.BrokenAtSeq, Reason: res.Reason}
	for _, e := range entries {
		out.Entries = append(out.Entries, AuditEntryDTO{
			Seq: e.Seq, At: e.At.Format(time.RFC3339), Action: string(e.Action),
			Subject: e.Subject, Detail: string(e.Detail), Hash: e.Hash,
		})
	}
	return out
}

// ExportHistory produces a "what was moved" export in the given format ("json"
// or "csv"), optionally scoped to a remote. The bytes are base64-encoded so the
// frontend can offer them as a download (§7.11.7).
func (a *App) ExportHistory(format, remote string) ExportResultDTO {
	raw, err := a.history.Export(context.Background(), history.ExportRequest{
		Format: history.Format(format), Remote: remote,
	})
	if err != nil {
		return ExportResultDTO{Error: errorToDTO(err)}
	}
	ext := "json"
	if history.Format(format) == history.FormatCSV {
		ext = "csv"
	}
	return ExportResultDTO{
		Filename: "conductor-history." + ext,
		Base64:   base64.StdEncoding.EncodeToString(raw),
	}
}

// ClearHistory deletes operation rows and their sealed logs (§7.11.7).
func (a *App) ClearHistory() *ErrorDTO {
	if _, err := a.history.ClearHistory(context.Background()); err != nil {
		return errorToDTO(err)
	}
	return nil
}

func (a *App) operationsResult(ops []domain.Operation, err error) OperationsResultDTO {
	if err != nil {
		return OperationsResultDTO{Error: errorToDTO(err)}
	}
	out := make([]OperationDTO, 0, len(ops))
	for _, op := range ops {
		out = append(out, toOperationDTO(op))
	}
	return OperationsResultDTO{Operations: out}
}

func toOperationDTO(op domain.Operation) OperationDTO {
	dto := OperationDTO{
		ID: op.ID, Kind: string(op.Kind), Src: op.Src, Dst: op.Dst,
		RcloneVersion: op.RcloneVersion, BytesMoved: op.BytesMoved, FilesMoved: op.FilesMoved,
		Result: string(op.Result), Destructive: op.Kind.IsDestructive(),
	}
	if !op.StartedAt.IsZero() {
		dto.StartedAt = op.StartedAt.Format(time.RFC3339)
	}
	if !op.EndedAt.IsZero() {
		dto.EndedAt = op.EndedAt.Format(time.RFC3339)
	}
	return dto
}
