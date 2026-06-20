package livestats

import (
	"context"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/conductor-app/conductor/internal/core/domain"
)

func idle() Snapshot { return Snapshot{} }
func active(bytes int64) Snapshot {
	return Snapshot{Stats: domain.TransferStats{Bytes: bytes, TotalBytes: 100, Speed: 1000, Transfers: 0}, ActiveJobs: 1}
}

func TestSignatureCoalescesIdleButReactsToChange(t *testing.T) {
	t.Parallel()

	// Two idle snapshots differing only in elapsed time share a signature.
	a := idle()
	a.Stats.ElapsedSeconds = 1
	b := idle()
	b.Stats.ElapsedSeconds = 99
	assert.Equal(t, signature(a), signature(b), "elapsed time alone must not change the signature")

	// A change in transferred bytes changes the signature.
	assert.NotEqual(t, signature(idle()), signature(active(10)))
	assert.NotEqual(t, signature(active(10)), signature(active(20)))
}

// scriptedSource returns each snapshot in turn, repeating the last, counting
// calls.
type scriptedSource struct {
	mu     sync.Mutex
	script []Snapshot
	calls  int
}

func (s *scriptedSource) Poll(context.Context) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	i := s.calls
	if i >= len(s.script) {
		i = len(s.script) - 1
	}
	s.calls++
	return s.script[i], nil
}

func (s *scriptedSource) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

// recorder captures emitted snapshots.
type recorder struct {
	mu    sync.Mutex
	snaps []Snapshot
}

func (r *recorder) EmitStats(s Snapshot) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.snaps = append(r.snaps, s)
}

func (r *recorder) emitted() []Snapshot {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Snapshot, len(r.snaps))
	copy(out, r.snaps)
	return out
}

func quietLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func TestPollerEmitsOnlyOnChange(t *testing.T) {
	t.Parallel()

	src := &scriptedSource{script: []Snapshot{
		idle(),     // emit (initial)
		idle(),     // coalesced
		active(50), // emit
		active(50), // coalesced
		idle(),     // emit
	}}
	rec := &recorder{}
	p := NewPoller(src, rec, quietLogger(), nil, time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { p.Run(ctx); close(done) }()

	// Let the script play out fully, then stop.
	require.Eventually(t, func() bool { return src.count() >= len(src.script)+2 }, 2*time.Second, time.Millisecond)
	cancel()
	<-done

	emitted := rec.emitted()
	require.Len(t, emitted, 3, "should emit only on the three distinct states")
	assert.Equal(t, int64(0), emitted[0].Stats.Bytes)
	assert.Equal(t, int64(50), emitted[1].Stats.Bytes)
	assert.Equal(t, int64(0), emitted[2].Stats.Bytes)
}

func TestPollerStopsOnContextCancel(t *testing.T) {
	t.Parallel()
	src := &scriptedSource{script: []Snapshot{idle()}}
	p := NewPoller(src, &recorder{}, quietLogger(), nil, time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { p.Run(ctx); close(done) }()

	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("poller did not stop on context cancellation")
	}
}
