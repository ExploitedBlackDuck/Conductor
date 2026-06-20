// Package coreerr defines Conductor's enumerated error-code catalog (§8.4) and
// the typed error that carries a code across the application. Codes are the
// stable contract the frontend switches on; the Wails binding layer maps an
// *Error to the transport DTO {code, message, retryable} (§2.2). Keeping every
// code in one place prevents stringly-typed drift.
package coreerr

import "errors"

// Code is a stable, enumerated error identifier. The frontend switches on it
// and never parses a message string.
type Code string

// The error-code catalog (§8.4). Each maps to a stable UI message and a
// retryable flag in the binding layer.
const (
	CodeDaemonNotRunning        Code = "ERR_DAEMON_NOT_RUNNING"
	CodeDaemonStart             Code = "ERR_DAEMON_START"
	CodeRcloneBinaryMissing     Code = "ERR_RCLONE_BINARY_MISSING"
	CodeRcloneChecksum          Code = "ERR_RCLONE_BINARY_CHECKSUM"
	CodeRCRequest               Code = "ERR_RC_REQUEST"
	CodeOptionConflict          Code = "ERR_OPTION_CONFLICT"
	CodeOptionOverCeiling       Code = "ERR_OPTION_OVER_CEILING"
	CodeDestructiveNotConfirmed Code = "ERR_DESTRUCTIVE_NOT_CONFIRMED"
	CodeJobCancelled            Code = "ERR_JOB_CANCELLED"
	CodeStoreMigration          Code = "ERR_STORE_MIGRATION"
	CodeSecretUnavailable       Code = "ERR_SECRET_UNAVAILABLE" //nolint:gosec // G101 false positive: an error code, not a credential
	CodeAuditChainBroken        Code = "ERR_AUDIT_CHAIN_BROKEN"
)

// Error is a coded application error. It wraps an underlying cause (preserved
// for errors.Is/As) and carries the catalog Code plus a retryable hint for the
// boundary DTO.
type Error struct {
	// Code is the catalog identifier the frontend branches on.
	Code Code
	// Message is a human-readable, secret-free summary safe to surface.
	Message string
	// Retryable indicates whether retrying the same call may succeed.
	Retryable bool
	// Err is the wrapped cause, or nil.
	Err error
}

// Error implements the error interface.
func (e *Error) Error() string {
	if e.Err != nil {
		return string(e.Code) + ": " + e.Message + ": " + e.Err.Error()
	}
	return string(e.Code) + ": " + e.Message
}

// Unwrap exposes the wrapped cause to errors.Is/As.
func (e *Error) Unwrap() error { return e.Err }

// New builds a coded error wrapping cause (which may be nil).
func New(code Code, message string, cause error) *Error {
	return &Error{Code: code, Message: message, Err: cause}
}

// Retryable builds a coded error flagged as retryable.
func Retryable(code Code, message string, cause error) *Error {
	return &Error{Code: code, Message: message, Retryable: true, Err: cause}
}

// CodeOf extracts the catalog Code from err, walking the chain. It returns
// ("", false) when no coded error is present.
func CodeOf(err error) (Code, bool) {
	var e *Error
	if errors.As(err, &e) {
		return e.Code, true
	}
	return "", false
}
