// Package governance resolves the effective bandwidth/concurrency ceilings for
// an operation from the global defaults and any per-remote ceilings (ADR-0013,
// §7.6). The flag builder then clamps the operator's selections to these
// ceilings. Resolution takes the most restrictive applicable cap.
package governance

import (
	"github.com/conductor-app/conductor/internal/core/domain"
	"github.com/conductor-app/conductor/internal/core/options"
)

// CeilingLookup returns the saved ceiling for a remote, if any.
type CeilingLookup func(remote string) (domain.RemoteCeiling, bool)

// Resolve combines the global ceilings with the per-remote ceilings of the given
// remotes, taking the most restrictive (smallest non-zero) value per dimension.
// A per-remote bandwidth cap takes precedence as that remote's authoritative
// limit.
func Resolve(global options.Ceilings, remotes []string, lookup CeilingLookup) options.Ceilings {
	eff := global
	for _, r := range remotes {
		if r == "" {
			continue
		}
		c, ok := lookup(r)
		if !ok {
			continue
		}
		eff.Transfers = minNonZero(eff.Transfers, c.Transfers)
		eff.Checkers = minNonZero(eff.Checkers, c.Checkers)
		eff.Tpslimit = minNonZero(eff.Tpslimit, c.Tpslimit)
		if c.Bwlimit != "" {
			eff.Bwlimit = c.Bwlimit
		}
	}
	return eff
}

// minNonZero returns the smaller of two caps, treating zero as "no cap" so a
// real cap always wins over an absent one.
func minNonZero(a, b int) int {
	switch {
	case a == 0:
		return b
	case b == 0:
		return a
	case a < b:
		return a
	default:
		return b
	}
}
