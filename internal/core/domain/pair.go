package domain

import "time"

// PairKind is the kind of a saved pair: a one-way sync or a two-way bisync.
type PairKind string

// Saved-pair kinds.
const (
	PairSync   PairKind = "sync"
	PairBisync PairKind = "bisync"
)

// SavedPair is a reusable sync/bisync pair (§7.5, §7.7). A pair whose LastRun is
// zero has never run; its first run defaults to dry-run (§7.4).
type SavedPair struct {
	ID        string
	Name      string
	Kind      PairKind
	Path1     string
	Path2     string
	ProfileID string
	LastRun   time.Time
}

// HasRun reports whether the pair has completed at least one run.
func (p SavedPair) HasRun() bool { return !p.LastRun.IsZero() }

// Profile is a named, reusable option set (§7.5).
type Profile struct {
	ID      string
	Name    string
	Kind    OperationKind
	Options []ProfileOption
}

// ProfileOption is one flag+value within a profile.
type ProfileOption struct {
	Flag  string
	Value string
}

// RemoteCeiling holds saved governance caps for a remote (§7.6). A zero/empty
// field means no cap for that dimension.
type RemoteCeiling struct {
	Remote    string
	Transfers int
	Checkers  int
	Bwlimit   string
	Tpslimit  int
}
