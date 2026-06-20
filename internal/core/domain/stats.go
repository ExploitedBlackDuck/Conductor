package domain

// TransferStats is the aggregate live picture of in-flight work, mapped from the
// rc core/stats response (§7.1). It is ephemeral runtime state, never persisted
// (ADR-0007); only completed operations are recorded.
type TransferStats struct {
	// Bytes transferred so far across all active transfers.
	Bytes int64
	// TotalBytes expected across all active transfers (0 when unknown).
	TotalBytes int64
	// Speed is the current aggregate throughput in bytes/second.
	Speed float64
	// Errors counts errors seen so far.
	Errors int64
	// Checks counts files checked (compared) so far.
	Checks int64
	// Transfers counts files transferred so far.
	Transfers int64
	// ElapsedSeconds is wall-clock time since stats were reset.
	ElapsedSeconds float64
	// ETASeconds is the estimated seconds remaining, or nil when unknown.
	ETASeconds *float64
	// Transferring holds per-file progress for currently active transfers.
	Transferring []FileProgress
}

// FileProgress is the progress of a single in-flight file (§7.11.4).
type FileProgress struct {
	// Name is the file's path relative to its transfer root.
	Name string
	// Bytes transferred so far for this file.
	Bytes int64
	// Size is the file's total size in bytes (0 when unknown).
	Size int64
	// Speed is this file's current throughput in bytes/second.
	Speed float64
	// Percentage complete, 0–100.
	Percentage int
	// ETASeconds is the estimated seconds remaining for this file, or nil.
	ETASeconds *float64
}
