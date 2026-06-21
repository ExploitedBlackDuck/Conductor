// Package singleinstance enforces that exactly one Conductor process owns the
// data directory at a time (§2.3). This is not optional polish: the SQLite store
// is single-writer and the audit hash-chain assumes a single appender, so two
// processes would contend on the database and could fork the chain. The lock is
// an OS-level advisory lock held on a lockfile in the data dir for the process's
// lifetime; the OS releases it when the process exits, even on a crash.
package singleinstance

import "errors"

// ErrAlreadyRunning is returned when another process already holds the lock.
var ErrAlreadyRunning = errors.New("another Conductor instance is already running")

// Lock is a held single-instance lock. Release (or process exit) frees it.
type Lock struct {
	release func() error
}

// Acquire takes the exclusive lock at path, creating the file if needed. It
// returns ErrAlreadyRunning if another process holds it.
func Acquire(path string) (*Lock, error) {
	release, err := acquire(path)
	if err != nil {
		return nil, err
	}
	return &Lock{release: release}, nil
}

// Release frees the lock. It is safe to call once; the OS also releases the lock
// when the process exits.
func (l *Lock) Release() error {
	if l == nil || l.release == nil {
		return nil
	}
	r := l.release
	l.release = nil
	return r()
}
