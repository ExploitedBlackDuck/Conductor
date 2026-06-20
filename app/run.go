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

// StartRun starts a copy/move from the current builder selection. Destructive
// selections must carry acknowledged=true or the core refuses them (§7.4); the
// failure is surfaced as a typed error, never a silent run.
func (a *App) StartRun(req PreviewRequest, acknowledged bool) RunResultDTO {
	h, err := a.transfers.Start(context.Background(), transfers.RunRequest{
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
	})
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
