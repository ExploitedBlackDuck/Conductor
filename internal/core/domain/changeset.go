package domain

// ChangeKind classifies one entry in a dry-run change set (ADR-0015).
type ChangeKind string

// The change kinds a dry-run preview distinguishes. Delete is the dangerous one
// and is always enumerated; creates/updates may be summarised by count for very
// large trees (§7.4). The sync-family `--combined` report distinguishes all
// three; the bisync/delete/purge JSON-log path can only separate writes from
// deletes (rclone does not split create from update there), so it reports writes
// as creates — validated against rclone v1.74.3.
const (
	ChangeCreate ChangeKind = "create"
	ChangeUpdate ChangeKind = "update"
	ChangeDelete ChangeKind = "delete"
)

// FileChange is one path affected by an operation, as reported by a --dry-run
// pass (ADR-0015).
type FileChange struct {
	Kind ChangeKind
	Path string
}

// ChangeSet is the parsed result of a --dry-run pass: the concrete creates,
// updates, and deletes an operation would perform — the basis of the
// destructive-op preview gate (ADR-0015, §7.4). It is shown to the operator
// before a destructive run can be confirmed, and is persisted (path lists
// sealed) as evidence of what was acknowledged (§7.7).
//
// Creates/Updates may be capped for very large trees — Truncated then reports
// that the counts exceed the listed entries — but Deletes are never capped, so
// the dangerous changes are always fully enumerable.
type ChangeSet struct {
	Creates []FileChange
	Updates []FileChange
	Deletes []FileChange
	// Exact totals, accurate even when the Creates/Updates slices are truncated.
	CreateCount int
	UpdateCount int
	DeleteCount int
	// Truncated reports that Creates/Updates list fewer entries than their counts
	// (Deletes are always complete).
	Truncated bool
}

// HasDeletes reports whether the operation would delete any data — the property
// that makes a preview a destructive one (§7.4).
func (c ChangeSet) HasDeletes() bool { return c.DeleteCount > 0 }

// Total is the number of changes across all kinds.
func (c ChangeSet) Total() int { return c.CreateCount + c.UpdateCount + c.DeleteCount }

// Empty reports whether the operation would change nothing.
func (c ChangeSet) Empty() bool { return c.Total() == 0 }
