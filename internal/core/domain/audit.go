// Package domain holds Conductor's core types and their validation. It depends
// on nothing outside the standard library: no rc wire formats, no SQL, no
// keyring (§2.1). Adapters translate to and from these types at the edges.
package domain

import (
	"encoding/json"
	"time"
)

// AuditAction enumerates the consequential actions recorded in the audit log
// (§7.8). Using typed constants keeps action names from drifting into magic
// strings across the codebase (§2.7).
type AuditAction string

// The audit action vocabulary. Destructive confirmations and risk
// acknowledgements are recorded with particular emphasis (ADR-0010).
const (
	ActionOperationStart       AuditAction = "operation.start"
	ActionOperationStop        AuditAction = "operation.stop"
	ActionOperationInterrupted AuditAction = "operation.interrupted"
	ActionDestructiveConfirmed AuditAction = "operation.destructive_confirmed"
	ActionRiskAcknowledged     AuditAction = "operation.risk_acknowledged"
	ActionMount                AuditAction = "mount.mount"
	ActionUnmount              AuditAction = "mount.unmount"
	ActionGovernanceCeilingSet AuditAction = "governance.ceiling_set"
	ActionExport               AuditAction = "history.export"
)

// AuditEntry is one record in the append-only, hash-chained audit log (§7.8).
// Entries are immutable once written; Hash chains to the previous entry's Hash
// so any tampering breaks verification (ADR-0010).
type AuditEntry struct {
	// Seq is the 1-based monotonic position in the chain.
	Seq int64
	// At is the time the action was recorded (stored in UTC).
	At time.Time
	// Action is the recorded action kind.
	Action AuditAction
	// Subject identifies what the action concerned (e.g. an operation ID).
	Subject string
	// Detail is canonical, compact JSON describing the action. It is the exact
	// byte sequence hashed into the chain, so verification is reproducible.
	Detail json.RawMessage
	// PrevHash is the hex-encoded Hash of the preceding entry, or "" for the
	// first entry (genesis).
	PrevHash string
	// Hash is the hex-encoded SHA-256 chain hash of this entry.
	Hash string
}

// AuditAnchor is a periodically-signed chain head (ADR-0010, §7.8). Hash-chaining
// alone detects partial or naive edits; an anchor signs the head with a separate
// keyring-held key so a *full recompute* of the chain — which would re-hash every
// entry consistently — is still detectable by anyone without that key.
type AuditAnchor struct {
	// Seq is the chain position whose head this anchor signs.
	Seq int64
	// HeadHash is the hex-encoded Hash of the entry at Seq at signing time.
	HeadHash string
	// Signature is the MAC over (Seq, HeadHash) under the audit-signing key.
	Signature []byte
	// SignedAt is when the head was signed (stored in UTC).
	SignedAt time.Time
}
