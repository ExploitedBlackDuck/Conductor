package domain

import "time"

// JobStatus is the status of an asynchronous rc job, mapped from job/status
// (§7.1). A stopped job is reported as cancelled by higher layers, not failed
// (§7.11.4).
type JobStatus struct {
	// ID is the rc job identifier.
	ID int64
	// Group is the rc stats group for the job (e.g. "job/1").
	Group string
	// Finished is true once the job has completed (successfully or not).
	Finished bool
	// Success is true when the job finished without error.
	Success bool
	// Error is the failure message, or "" when none.
	Error string
	// DurationSeconds is how long the job ran.
	DurationSeconds float64
	// StartTime is when the job started.
	StartTime time.Time
	// EndTime is when the job ended (zero if still running).
	EndTime time.Time
}
