package app

import (
	"sync"

	"github.com/conductor-app/conductor/internal/core/livestats"
	"github.com/conductor-app/conductor/shell"
)

// Event names emitted to the frontend (§2.8). Typed constants in one place so a
// name cannot drift between Go and the subscriber.
const (
	// EventStatsUpdate carries a StatsEventDTO with the live transfer snapshot.
	EventStatsUpdate = "stats:update"
)

// FileProgressDTO is per-file progress in the live stats event.
type FileProgressDTO struct {
	Name       string   `json:"name"`
	Bytes      int64    `json:"bytes"`
	Size       int64    `json:"size"`
	Speed      float64  `json:"speed"`
	Percentage int      `json:"percentage"`
	ETASeconds *float64 `json:"etaSeconds"`
}

// StatsEventDTO is the live transfer snapshot delivered to the frontend, both as
// the EventStatsUpdate payload and as the return of StatsSnapshot (which also
// makes this type appear in the generated bindings, §2.8).
type StatsEventDTO struct {
	Bytes        int64             `json:"bytes"`
	TotalBytes   int64             `json:"totalBytes"`
	Speed        float64           `json:"speed"`
	Errors       int64             `json:"errors"`
	Checks       int64             `json:"checks"`
	Transfers    int64             `json:"transfers"`
	ETASeconds   *float64          `json:"etaSeconds"`
	ActiveJobs   int               `json:"activeJobs"`
	Transferring []FileProgressDTO `json:"transferring"`
}

// statsEmitter adapts livestats.Emitter to a typed Wails event and retains the
// last snapshot so StatsSnapshot can serve an initial value before the first
// event arrives.
type statsEmitter struct {
	rt shell.Runtime

	mu   sync.Mutex
	last StatsEventDTO
}

// EmitStats implements livestats.Emitter.
func (e *statsEmitter) EmitStats(snap livestats.Snapshot) {
	dto := toStatsEventDTO(snap)
	e.mu.Lock()
	e.last = dto
	e.mu.Unlock()
	e.rt.EmitEvent(EventStatsUpdate, dto)
}

func (e *statsEmitter) snapshot() StatsEventDTO {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.last
}

func toStatsEventDTO(snap livestats.Snapshot) StatsEventDTO {
	s := snap.Stats
	dto := StatsEventDTO{
		Bytes:      s.Bytes,
		TotalBytes: s.TotalBytes,
		Speed:      s.Speed,
		Errors:     s.Errors,
		Checks:     s.Checks,
		Transfers:  s.Transfers,
		ETASeconds: s.ETASeconds,
		ActiveJobs: snap.ActiveJobs,
	}
	for _, f := range s.Transferring {
		dto.Transferring = append(dto.Transferring, FileProgressDTO{
			Name: f.Name, Bytes: f.Bytes, Size: f.Size, Speed: f.Speed,
			Percentage: f.Percentage, ETASeconds: f.ETASeconds,
		})
	}
	return dto
}
