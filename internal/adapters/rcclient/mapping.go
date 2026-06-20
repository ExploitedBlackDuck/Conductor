package rcclient

import (
	"fmt"
	"time"

	"github.com/conductor-app/conductor/internal/core/domain"
)

// coreStatsResponse mirrors the rc core/stats JSON shape (see
// testdata/core_stats_*.json).
type coreStatsResponse struct {
	Bytes        int64                  `json:"bytes"`
	TotalBytes   int64                  `json:"totalBytes"`
	Speed        float64                `json:"speed"`
	Errors       int64                  `json:"errors"`
	Checks       int64                  `json:"checks"`
	Transfers    int64                  `json:"transfers"`
	ElapsedTime  float64                `json:"elapsedTime"`
	ETA          *float64               `json:"eta"`
	Transferring []transferringResponse `json:"transferring"`
}

// transferringResponse is one entry of core/stats "transferring".
type transferringResponse struct {
	Name       string   `json:"name"`
	Bytes      int64    `json:"bytes"`
	Size       int64    `json:"size"`
	Speed      float64  `json:"speed"`
	Percentage int      `json:"percentage"`
	ETA        *float64 `json:"eta"`
}

func (r coreStatsResponse) toDomain() domain.TransferStats {
	stats := domain.TransferStats{
		Bytes:          r.Bytes,
		TotalBytes:     r.TotalBytes,
		Speed:          r.Speed,
		Errors:         r.Errors,
		Checks:         r.Checks,
		Transfers:      r.Transfers,
		ElapsedSeconds: r.ElapsedTime,
		ETASeconds:     r.ETA,
	}
	for _, t := range r.Transferring {
		stats.Transferring = append(stats.Transferring, domain.FileProgress{
			Name:       t.Name,
			Bytes:      t.Bytes,
			Size:       t.Size,
			Speed:      t.Speed,
			Percentage: t.Percentage,
			ETASeconds: t.ETA,
		})
	}
	return stats
}

// jobStatusResponse mirrors the rc job/status JSON shape (see
// testdata/job_status_finished.json).
type jobStatusResponse struct {
	ID        int64   `json:"id"`
	Group     string  `json:"group"`
	Finished  bool    `json:"finished"`
	Success   bool    `json:"success"`
	Error     string  `json:"error"`
	Duration  float64 `json:"duration"`
	StartTime string  `json:"startTime"`
	EndTime   string  `json:"endTime"`
}

func (r jobStatusResponse) toDomain() (domain.JobStatus, error) {
	status := domain.JobStatus{
		ID:              r.ID,
		Group:           r.Group,
		Finished:        r.Finished,
		Success:         r.Success,
		Error:           r.Error,
		DurationSeconds: r.Duration,
	}
	start, err := parseRCTime(r.StartTime)
	if err != nil {
		return domain.JobStatus{}, fmt.Errorf("parsing job start time: %w", err)
	}
	status.StartTime = start

	// A still-running job has a zero end time.
	if r.EndTime != "" && r.EndTime != "0001-01-01T00:00:00Z" {
		end, err := parseRCTime(r.EndTime)
		if err != nil {
			return domain.JobStatus{}, fmt.Errorf("parsing job end time: %w", err)
		}
		status.EndTime = end
	}
	return status, nil
}

// parseRCTime parses an rc timestamp (RFC3339 with offset).
func parseRCTime(s string) (time.Time, error) {
	return time.Parse(time.RFC3339Nano, s)
}
