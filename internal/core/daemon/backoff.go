package daemon

import "time"

// backoff produces an exponentially increasing delay, capped at max, for
// restart attempts. It is reset after a successful, healthy launch so a daemon
// that runs fine for a while restarts quickly the next time.
type backoff struct {
	base    time.Duration
	max     time.Duration
	current time.Duration
}

// next returns the next delay and advances the sequence.
func (b *backoff) next() time.Duration {
	if b.current == 0 {
		b.current = b.base
		return b.current
	}
	b.current *= 2
	if b.current > b.max {
		b.current = b.max
	}
	return b.current
}

// reset returns the sequence to its starting point.
func (b *backoff) reset() {
	b.current = 0
}
