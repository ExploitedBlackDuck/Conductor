package domain

import "time"

// VerificationKind is the kind of integrity check (§7.12). Both compare a source
// against a destination already present in rclone.conf and mutate nothing.
type VerificationKind string

// The verification kinds.
const (
	// VerifyCheck compares source and destination (hashes where the backends
	// support them, sizes otherwise).
	VerifyCheck VerificationKind = "check"
	// VerifyCryptcheck compares an encrypted remote against a plaintext source.
	VerifyCryptcheck VerificationKind = "cryptcheck"
)

// VerifyResult is the terminal outcome of a verification.
type VerifyResult string

// Verification results.
const (
	// VerifyMatch means every file matched: no differences, missing, or errors.
	VerifyMatch VerifyResult = "match"
	// VerifyMismatch means at least one file differed, was missing, or errored.
	VerifyMismatch VerifyResult = "mismatch"
)

// Verification is the persisted result of an integrity check (§7.7, §7.12). It
// is read-only evidence — a verification never mutates a remote — and is
// hash-chained into the audit log so "this sync was verified and matched" is a
// durable, tamper-evident claim, not a transient console line. Only the counts
// are stored; the offending paths are shown live but not persisted (they are as
// sensitive as the data and the table records the verdict, not the contents).
type Verification struct {
	ID         string
	Kind       VerificationKind
	Src        string
	Dst        string
	StartedAt  time.Time
	EndedAt    time.Time
	Match      int
	Differ     int
	Missing    int
	ErrorCount int
	Result     VerifyResult
}

// ResultFor classifies a verification from its counts: anything other than a
// clean all-match is a mismatch.
func ResultFor(differ, missing, errorCount int) VerifyResult {
	if differ == 0 && missing == 0 && errorCount == 0 {
		return VerifyMatch
	}
	return VerifyMismatch
}
