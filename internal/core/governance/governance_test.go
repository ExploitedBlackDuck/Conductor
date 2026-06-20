package governance

import (
	"testing"

	"github.com/conductor-app/conductor/internal/core/domain"
	"github.com/conductor-app/conductor/internal/core/options"
)

// lookup builds a CeilingLookup from a fixed set of per-remote ceilings.
func lookup(ceilings ...domain.RemoteCeiling) CeilingLookup {
	byRemote := map[string]domain.RemoteCeiling{}
	for _, c := range ceilings {
		byRemote[c.Remote] = c
	}
	return func(remote string) (domain.RemoteCeiling, bool) {
		c, ok := byRemote[remote]
		return c, ok
	}
}

func TestResolveTakesMostRestrictive(t *testing.T) {
	t.Parallel()
	global := options.Ceilings{Transfers: 4, Checkers: 8, Bwlimit: "10M", Tpslimit: 10}

	cases := []struct {
		name    string
		remotes []string
		look    CeilingLookup
		want    options.Ceilings
	}{
		{
			name:    "no per-remote ceilings keeps the global defaults",
			remotes: []string{"local", ""},
			look:    lookup(),
			want:    global,
		},
		{
			name:    "a tighter per-remote cap wins each dimension",
			remotes: []string{"s3"},
			look:    lookup(domain.RemoteCeiling{Remote: "s3", Transfers: 2, Checkers: 4, Tpslimit: 5}),
			want:    options.Ceilings{Transfers: 2, Checkers: 4, Bwlimit: "10M", Tpslimit: 5},
		},
		{
			name:    "a looser per-remote cap never relaxes the global",
			remotes: []string{"fast"},
			look:    lookup(domain.RemoteCeiling{Remote: "fast", Transfers: 64, Checkers: 64, Tpslimit: 100}),
			want:    global,
		},
		{
			name:    "a per-remote bwlimit is that remote's authoritative cap",
			remotes: []string{"capped"},
			look:    lookup(domain.RemoteCeiling{Remote: "capped", Bwlimit: "1M"}),
			want:    options.Ceilings{Transfers: 4, Checkers: 8, Bwlimit: "1M", Tpslimit: 10},
		},
		{
			name:    "the tightest cap across multiple remotes wins",
			remotes: []string{"s3", "b2"},
			look: lookup(
				domain.RemoteCeiling{Remote: "s3", Transfers: 3, Checkers: 16},
				domain.RemoteCeiling{Remote: "b2", Transfers: 2, Checkers: 2, Tpslimit: 4},
			),
			want: options.Ceilings{Transfers: 2, Checkers: 2, Bwlimit: "10M", Tpslimit: 4},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()
			got := Resolve(global, c.remotes, c.look)
			if got != c.want {
				t.Errorf("Resolve() = %+v, want %+v", got, c.want)
			}
		})
	}
}

// TestResolveTreatsZeroAsNoCap proves a global of zero (no cap) is overridden by
// any real per-remote cap, not treated as the smallest value.
func TestResolveTreatsZeroAsNoCap(t *testing.T) {
	t.Parallel()
	global := options.Ceilings{} // no caps at all
	got := Resolve(global, []string{"s3"}, lookup(domain.RemoteCeiling{Remote: "s3", Transfers: 4}))
	if got.Transfers != 4 {
		t.Errorf("Transfers = %d, want 4 (a real cap must win over an absent one)", got.Transfers)
	}
}
