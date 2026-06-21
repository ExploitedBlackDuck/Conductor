//go:build unix

package singleinstance

import (
	"errors"
	"fmt"
	"os"
	"syscall"
)

// acquire takes a non-blocking exclusive flock on path. The descriptor is held
// open for the process lifetime so the lock persists until exit (when the OS
// drops it); release closes it, dropping the lock immediately.
func acquire(path string) (func() error, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("opening lockfile %s: %w", path, err)
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = f.Close()
		if errors.Is(err, syscall.EWOULDBLOCK) {
			return nil, ErrAlreadyRunning
		}
		return nil, fmt.Errorf("locking %s: %w", path, err)
	}
	// Record our PID for diagnostics; failure here is non-fatal, the lock holds.
	if err := f.Truncate(0); err == nil {
		_, _ = f.Seek(0, 0)
		_, _ = fmt.Fprintf(f, "%d\n", os.Getpid())
	}
	return func() error {
		// Closing the descriptor releases the flock.
		return f.Close()
	}, nil
}
