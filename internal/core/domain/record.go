package domain

import "time"

// Result is the terminal outcome of an operation (§7.11.4). A stopped job is
// recorded as cancelled, never failed.
type Result string

// Operation results.
const (
	ResultRunning   Result = "running"
	ResultSuccess   Result = "success"
	ResultFailed    Result = "failed"
	ResultCancelled Result = "cancelled"
)

// Operation is the persisted history record of one copy/sync/move/etc. (§7.7).
// History rows are immutable; a re-run is a new row.
type Operation struct {
	// ID is a unique identifier assigned at start.
	ID string
	// Kind is the operation kind.
	Kind OperationKind
	// Src and Dst are the resolved endpoints (remote:path).
	Src string
	Dst string
	// RcloneVersion is the pinned rclone version the operation ran against.
	RcloneVersion string
	// Intensity is the JSON-encoded effective governance caps applied (§7.6).
	Intensity string
	// StartedAt / EndedAt bound the run; EndedAt is zero while running.
	StartedAt time.Time
	EndedAt   time.Time
	// BytesMoved / FilesMoved are captured from the job's final stats.
	BytesMoved int64
	FilesMoved int64
	// Result is the terminal outcome.
	Result Result
	// LogRef is the id of the sealed captured log, or "" if none.
	LogRef string
}

// OperationOption is one resolved option recorded with an operation (§7.7),
// including whether a destructive selection was acknowledged.
type OperationOption struct {
	Flag         string
	Value        string
	Risk         string
	Acknowledged bool
}

// CapturedLog is an operation's captured rclone job log/stats, sealed at rest
// with AEAD (ADR-0009). The plaintext SHA-256 and length are stored alongside
// for integrity verification on read.
type CapturedLog struct {
	ID              string
	OperationID     string
	Nonce           []byte
	SealedBytes     []byte
	SHA256Plaintext string
	BytesLen        int
}
