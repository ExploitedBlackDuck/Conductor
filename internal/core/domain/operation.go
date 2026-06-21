package domain

import "strings"

// OperationKind enumerates the kinds of operation Conductor can run (§7.1). The
// kind drives both the rc endpoint used and the impact rules applied.
type OperationKind string

// The operation kinds Conductor can run.
const (
	KindCopy   OperationKind = "copy"
	KindSync   OperationKind = "sync"
	KindMove   OperationKind = "move"
	KindDelete OperationKind = "delete"
	KindPurge  OperationKind = "purge"
	KindBisync OperationKind = "bisync"
	KindMount  OperationKind = "mount"
)

// IsDestructive reports whether the kind itself (independent of options) deletes
// or can overwrite data at the destination. A sync makes the destination match
// the source and so deletes extra files; delete and purge remove data
// outright. These always require explicit confirmation (§7.4, ADR-0011).
func (k OperationKind) IsDestructive() bool {
	switch k {
	case KindSync, KindDelete, KindPurge:
		return true
	default:
		return false
	}
}

// Endpoint is a resolved operation endpoint: a remote name plus a path within
// it. Endpoints are shown resolved before execution; there is no silent path
// interpolation (§7.4).
type Endpoint struct {
	// Remote is the rclone remote name, or "" for a local path.
	Remote string
	// Path is the path within the remote (or a local filesystem path).
	Path string
}

// String renders the endpoint in rclone's remote:path form.
func (e Endpoint) String() string {
	if e.Remote == "" {
		return e.Path
	}
	return e.Remote + ":" + e.Path
}

// ServerSideEligible reports whether a copy/move between src and dst could run
// server-side (§7.3): both must be on the same configured remote, the
// conservative subset of "same backend identity" Conductor can determine without
// inspecting remote config. A local endpoint is never server-side eligible.
func ServerSideEligible(src, dst Endpoint) bool {
	return src.Remote != "" && src.Remote == dst.Remote
}

// ParseEndpoint splits an rclone-style "remote:path" string into an Endpoint. A
// string with no colon — or whose colon falls after a path separator (a local
// path that merely contains a colon) — is treated as a local path with no
// remote. It is the inverse of String for the values Conductor produces.
func ParseEndpoint(s string) Endpoint {
	i := strings.IndexByte(s, ':')
	if i < 0 {
		return Endpoint{Path: s}
	}
	// A separator before the colon means the colon belongs to the path, not a
	// remote name (e.g. a local path), so there is no remote.
	if strings.ContainsAny(s[:i], `/\`) {
		return Endpoint{Path: s}
	}
	return Endpoint{Remote: s[:i], Path: s[i+1:]}
}
