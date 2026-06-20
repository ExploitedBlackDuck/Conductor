package domain

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
