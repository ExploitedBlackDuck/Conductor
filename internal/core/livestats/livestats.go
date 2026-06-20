// Package livestats polls the rc daemon for live transfer statistics on a
// context-owned ticker and emits typed snapshots as they change (§7.2, §7.10
// P4). Live stats are ephemeral runtime state — they are never persisted
// (ADR-0007). The poll loop has a single, owned goroutine that stops on context
// cancellation, and it coalesces redundant updates so an idle daemon does not
// emit a stream of identical snapshots.
package livestats

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/conductor-app/conductor/internal/core/domain"
	"github.com/conductor-app/conductor/internal/core/ports"
)

// Snapshot is one sampled view of live activity.
type Snapshot struct {
	Stats         domain.TransferStats
	ActiveJobs    int
	RunningJobIDs []int64
}

// Source produces a snapshot. It is implemented by the control service over the
// rc client.
type Source interface {
	Poll(ctx context.Context) (Snapshot, error)
}

// Emitter receives snapshots worth delivering to the frontend. The app layer
// adapts it to a typed Wails event (§2.8).
type Emitter interface {
	EmitStats(Snapshot)
}

// Poller samples a Source on a ticker and emits changed snapshots.
type Poller struct {
	source   Source
	emitter  Emitter
	log      *slog.Logger
	clock    ports.Clock
	interval time.Duration
}

// NewPoller constructs a Poller. A zero interval defaults to one second.
func NewPoller(source Source, emitter Emitter, log *slog.Logger, clock ports.Clock, interval time.Duration) *Poller {
	if interval <= 0 {
		interval = time.Second
	}
	if clock == nil {
		clock = ports.SystemClock{}
	}
	return &Poller{source: source, emitter: emitter, log: log, clock: clock, interval: interval}
}

// Run polls until ctx is cancelled. It emits the first successful snapshot and
// thereafter only snapshots whose meaningful signature changed, so an idle
// daemon goes quiet. It is the single owner of its work and returns when ctx is
// done (§2.3).
func (p *Poller) Run(ctx context.Context) {
	ticker := time.NewTicker(p.interval)
	defer ticker.Stop()

	var lastSig string
	var emittedOnce bool

	emitIfChanged := func() {
		snap, err := p.source.Poll(ctx)
		if err != nil {
			// Transient rc errors are logged but do not stop the loop; the
			// daemon may be restarting (ADR-0005).
			p.log.Debug("live stats poll failed", "error", err)
			return
		}
		sig := signature(snap)
		if emittedOnce && sig == lastSig {
			return
		}
		lastSig = sig
		emittedOnce = true
		p.emitter.EmitStats(snap)
	}

	emitIfChanged() // emit an initial snapshot promptly
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			emitIfChanged()
		}
	}
}

// signature captures the fields that make a snapshot meaningfully different,
// deliberately excluding monotonic clocks (elapsed time) so an otherwise-idle
// daemon produces a stable signature and is coalesced.
func signature(s Snapshot) string {
	sig := fmt.Sprintf("b=%d tb=%d sp=%.0f er=%d ch=%d tr=%d aj=%d|",
		s.Stats.Bytes, s.Stats.TotalBytes, s.Stats.Speed, s.Stats.Errors,
		s.Stats.Checks, s.Stats.Transfers, s.ActiveJobs)
	for _, f := range s.Stats.Transferring {
		sig += fmt.Sprintf("%s:%d/%d:%d;", f.Name, f.Bytes, f.Size, f.Percentage)
	}
	return sig
}
