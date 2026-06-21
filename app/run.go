package app

import (
	"context"

	"github.com/conductor-app/conductor/internal/core/domain"
	"github.com/conductor-app/conductor/internal/core/options"
	"github.com/conductor-app/conductor/internal/core/transfers"
)

// RunResultDTO is the outcome of starting an operation.
type RunResultDTO struct {
	OperationID string    `json:"operationId"`
	JobID       int64     `json:"jobId"`
	Error       *ErrorDTO `json:"error"`
}

// FileChangeDTO is one path a dry-run would change.
type FileChangeDTO struct {
	Kind string `json:"kind"`
	Path string `json:"path"`
}

// ChangeSetDTO is the dry-run preview the operator confirms against (ADR-0015).
// Deletes are always complete; creates/updates may be summarised by count when
// Truncated, for very large trees.
type ChangeSetDTO struct {
	Creates     []FileChangeDTO `json:"creates"`
	Updates     []FileChangeDTO `json:"updates"`
	Deletes     []FileChangeDTO `json:"deletes"`
	CreateCount int             `json:"createCount"`
	UpdateCount int             `json:"updateCount"`
	DeleteCount int             `json:"deleteCount"`
	Truncated   bool            `json:"truncated"`
}

// ChangeSetResultDTO is a dry-run preview or a typed error.
type ChangeSetResultDTO struct {
	ChangeSet ChangeSetDTO `json:"changeSet"`
	Error     *ErrorDTO    `json:"error"`
}

func toChangeSetDTO(cs domain.ChangeSet) ChangeSetDTO {
	conv := func(in []domain.FileChange) []FileChangeDTO {
		out := make([]FileChangeDTO, 0, len(in))
		for _, c := range in {
			out = append(out, FileChangeDTO{Kind: string(c.Kind), Path: c.Path})
		}
		return out
	}
	return ChangeSetDTO{
		Creates: conv(cs.Creates), Updates: conv(cs.Updates), Deletes: conv(cs.Deletes),
		CreateCount: cs.CreateCount, UpdateCount: cs.UpdateCount, DeleteCount: cs.DeleteCount,
		Truncated: cs.Truncated,
	}
}

// runRequest maps a PreviewRequest DTO to the core RunRequest.
func runRequest(req PreviewRequest, acknowledged bool) transfers.RunRequest {
	return transfers.RunRequest{
		Kind:      domain.OperationKind(req.Kind),
		Src:       domain.Endpoint{Remote: req.Src.Remote, Path: req.Src.Path},
		Dst:       domain.Endpoint{Remote: req.Dst.Remote, Path: req.Dst.Path},
		Selection: options.Selection{Single: req.Single, Multi: req.Multi},
		Ceilings: options.Ceilings{
			Transfers: req.Ceilings.Transfers,
			Checkers:  req.Ceilings.Checkers,
			Bwlimit:   req.Ceilings.Bwlimit,
			Tpslimit:  req.Ceilings.Tpslimit,
		},
		Acknowledged: acknowledged,
	}
}

// PreviewRun runs the operation's dry-run and returns the change set the UI
// shows before a destructive confirm (ADR-0015). It mutates nothing.
func (a *App) PreviewRun(req PreviewRequest) ChangeSetResultDTO {
	cs, err := a.transfers.Preview(context.Background(), runRequest(req, false))
	if err != nil {
		return ChangeSetResultDTO{Error: errorToDTO(err)}
	}
	return ChangeSetResultDTO{ChangeSet: toChangeSetDTO(cs)}
}

// StartRun starts an operation from the current builder selection. A destructive
// selection must be previewed and carry acknowledged=true or the core refuses it
// (ADR-0015, §7.4); the failure is surfaced as a typed error, never a silent
// run. When acknowledged, the dry-run preview is attached so the gate is
// satisfied against the concrete change set.
func (a *App) StartRun(req PreviewRequest, acknowledged bool) RunResultDTO {
	run := runRequest(req, acknowledged)
	if acknowledged {
		cs, err := a.transfers.Preview(context.Background(), run)
		if err != nil {
			return RunResultDTO{Error: errorToDTO(err)}
		}
		run.ShownChangeSet = &cs
	}
	h, err := a.transfers.Start(context.Background(), run)
	if err != nil {
		return RunResultDTO{Error: errorToDTO(err)}
	}
	return RunResultDTO{OperationID: h.OperationID, JobID: h.JobID}
}

// CancelRun requests cancellation of a running operation. A stopped job is
// recorded as cancelled, not failed (§7.11.4).
func (a *App) CancelRun(operationID string) *ErrorDTO {
	if err := a.transfers.Cancel(context.Background(), operationID); err != nil {
		return errorToDTO(err)
	}
	return nil
}
