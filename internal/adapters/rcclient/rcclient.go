// Package rcclient is the HTTP client for the rclone rcd rc API (ADR-0003). The
// rc surface is small, so it uses stdlib net/http with a typed client rather
// than a framework. Every response shape has a typed Go struct mapped to a
// domain type, and is covered by a captured fixture under testdata/ (§2.5).
package rcclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/conductor-app/conductor/internal/core/coreerr"
	"github.com/conductor-app/conductor/internal/core/domain"
)

// Client talks to a single rcd instance over loopback with basic auth.
type Client struct {
	base  string
	user  string
	pass  string
	httpc *http.Client
}

// New constructs a Client for the daemon at addr (host:port) using the given
// per-session credentials.
func New(addr, user, pass string) *Client {
	return &Client{
		base:  "http://" + addr,
		user:  user,
		pass:  pass,
		httpc: &http.Client{Timeout: 30 * time.Second},
	}
}

// Ping calls rc/noop; a nil return means the daemon is reachable and
// authenticating.
func (c *Client) Ping(ctx context.Context) error {
	return c.call(ctx, "rc/noop", nil, nil)
}

// CorePID returns the daemon's operating-system process id (core/pid). It lets
// the supervisor and tests confirm a stopped daemon leaves no orphan.
func (c *Client) CorePID(ctx context.Context) (int, error) {
	var resp struct {
		PID int `json:"pid"`
	}
	if err := c.call(ctx, "core/pid", nil, &resp); err != nil {
		return 0, err
	}
	return resp.PID, nil
}

// CoreStats fetches aggregate live transfer stats (core/stats).
func (c *Client) CoreStats(ctx context.Context) (domain.TransferStats, error) {
	var resp coreStatsResponse
	if err := c.call(ctx, "core/stats", nil, &resp); err != nil {
		return domain.TransferStats{}, err
	}
	return resp.toDomain(), nil
}

// ConfigListRemotes returns the configured remote names (config/listremotes).
func (c *Client) ConfigListRemotes(ctx context.Context) ([]string, error) {
	var resp struct {
		Remotes []string `json:"remotes"`
	}
	if err := c.call(ctx, "config/listremotes", nil, &resp); err != nil {
		return nil, err
	}
	return resp.Remotes, nil
}

// JobList is the set of job identifiers known to the daemon (job/list).
type JobList struct {
	// ExecuteID identifies the daemon's execution session.
	ExecuteID string
	// JobIDs are all known job ids.
	JobIDs []int64
	// RunningIDs are currently-running job ids.
	RunningIDs []int64
	// FinishedIDs are completed job ids.
	FinishedIDs []int64
}

// JobList fetches the current job identifiers (job/list).
func (c *Client) JobList(ctx context.Context) (JobList, error) {
	var resp struct {
		ExecuteID   string  `json:"executeId"`
		JobIDs      []int64 `json:"jobids"`
		RunningIDs  []int64 `json:"runningIds"`
		FinishedIDs []int64 `json:"finishedIds"`
	}
	if err := c.call(ctx, "job/list", nil, &resp); err != nil {
		return JobList{}, err
	}
	return JobList{
		ExecuteID:   resp.ExecuteID,
		JobIDs:      resp.JobIDs,
		RunningIDs:  resp.RunningIDs,
		FinishedIDs: resp.FinishedIDs,
	}, nil
}

// SyncCopy starts a sync/copy on the daemon. config populates the rc _config
// block and filter the _filter block (both may be nil). When async is true the
// daemon runs the job in the background and the returned id is its job id;
// otherwise the call blocks until completion and returns 0.
func (c *Client) SyncCopy(ctx context.Context, srcFs, dstFs string, config map[string]any, filter map[string][]string, async bool) (int64, error) {
	body := map[string]any{"srcFs": srcFs, "dstFs": dstFs}
	if len(config) > 0 {
		body["_config"] = config
	}
	if len(filter) > 0 {
		body["_filter"] = filter
	}
	if async {
		body["_async"] = true
	}
	var resp struct {
		JobID int64 `json:"jobid"`
	}
	if err := c.call(ctx, "sync/copy", body, &resp); err != nil {
		return 0, err
	}
	return resp.JobID, nil
}

// SyncMove starts a sync/move on the daemon. See SyncCopy for the parameter
// semantics; deleteEmptySrcDirs removes emptied source directories.
func (c *Client) SyncMove(ctx context.Context, srcFs, dstFs string, config map[string]any, filter map[string][]string, deleteEmptySrcDirs, async bool) (int64, error) {
	body := map[string]any{"srcFs": srcFs, "dstFs": dstFs, "deleteEmptySrcDirs": deleteEmptySrcDirs}
	if len(config) > 0 {
		body["_config"] = config
	}
	if len(filter) > 0 {
		body["_filter"] = filter
	}
	if async {
		body["_async"] = true
	}
	var resp struct {
		JobID int64 `json:"jobid"`
	}
	if err := c.call(ctx, "sync/move", body, &resp); err != nil {
		return 0, err
	}
	return resp.JobID, nil
}

// JobStop requests cancellation of a running job (job/stop).
func (c *Client) JobStop(ctx context.Context, id int64) error {
	return c.call(ctx, "job/stop", map[string]any{"jobid": id}, nil)
}

// CoreStatsForGroup fetches stats scoped to a single job's stats group
// (core/stats with a group filter), used to capture a finished job's totals.
func (c *Client) CoreStatsForGroup(ctx context.Context, group string) (domain.TransferStats, error) {
	var resp coreStatsResponse
	if err := c.call(ctx, "core/stats", map[string]any{"group": group}, &resp); err != nil {
		return domain.TransferStats{}, err
	}
	return resp.toDomain(), nil
}

// JobStatus fetches the status of a single job (job/status).
func (c *Client) JobStatus(ctx context.Context, id int64) (domain.JobStatus, error) {
	var resp jobStatusResponse
	if err := c.call(ctx, "job/status", map[string]any{"jobid": id}, &resp); err != nil {
		return domain.JobStatus{}, err
	}
	return resp.toDomain()
}

// call POSTs in (JSON, or {} when nil) to the rc endpoint at path with basic
// auth, and decodes a successful JSON body into out (ignored when nil). A
// non-200 response is decoded as an rc error and returned with ERR_RC_REQUEST.
func (c *Client) call(ctx context.Context, path string, in, out any) error {
	body := []byte("{}")
	if in != nil {
		encoded, err := json.Marshal(in)
		if err != nil {
			return fmt.Errorf("encoding %s request: %w", path, err)
		}
		body = encoded
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.base+"/"+path, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("building %s request: %w", path, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(c.user, c.pass)

	resp, err := c.httpc.Do(req)
	if err != nil {
		return coreerr.Retryable(coreerr.CodeRCRequest, "rc request failed: "+path, err)
	}
	defer func() { _ = resp.Body.Close() }()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return coreerr.Retryable(coreerr.CodeRCRequest, "reading rc response: "+path, err)
	}

	if resp.StatusCode != http.StatusOK {
		return rcError(path, resp.StatusCode, data)
	}

	if out == nil {
		return nil
	}
	if err := json.Unmarshal(data, out); err != nil {
		return coreerr.New(coreerr.CodeRCRequest, "decoding rc response: "+path, err)
	}
	return nil
}

// rcError maps a non-200 rc response (which carries a JSON {error,...} body) to
// a coded error.
func rcError(path string, status int, body []byte) error {
	var payload struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(body, &payload)
	msg := payload.Error
	if msg == "" {
		msg = fmt.Sprintf("rc %s returned status %d", path, status)
	}
	return coreerr.New(coreerr.CodeRCRequest, msg, nil)
}
