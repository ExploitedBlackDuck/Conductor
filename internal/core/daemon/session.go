package daemon

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"
)

// Credentials are the rc session user/pass. They are generated per session and
// held only in memory — never written to disk or logged (ADR-0009, §2.4).
type Credentials struct {
	User string
	Pass string
}

// generateCredentials returns fresh random rc credentials.
func generateCredentials() (Credentials, error) {
	user, err := randomToken()
	if err != nil {
		return Credentials{}, fmt.Errorf("generating rc user: %w", err)
	}
	pass, err := randomToken()
	if err != nil {
		return Credentials{}, fmt.Errorf("generating rc pass: %w", err)
	}
	return Credentials{User: "conductor-" + user[:8], Pass: pass}, nil
}

// randomToken returns 32 hex characters of cryptographic randomness.
func randomToken() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// freeLoopbackAddr reserves and returns a free 127.0.0.1:<port> address. The
// listener is closed immediately; rclone binds the port on start. The brief
// window is acceptable for a single-operator local app.
func freeLoopbackAddr() (string, error) {
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return "", fmt.Errorf("reserving loopback port: %w", err)
	}
	addr := l.Addr().String()
	if err := l.Close(); err != nil {
		return "", fmt.Errorf("releasing reserved port: %w", err)
	}
	return addr, nil
}

// HealthProbe reports whether the rc daemon at addr is responding to
// authenticated requests. It returns nil when healthy.
type HealthProbe func(ctx context.Context, addr string, creds Credentials) error

// httpHealthProbe calls rc/noop over loopback with basic auth; a 200 means the
// daemon is up and authenticating. It is the default probe.
func httpHealthProbe(ctx context.Context, addr string, creds Credentials) error {
	url := "http://" + addr + "/rc/noop"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, strings.NewReader("{}"))
	if err != nil {
		return fmt.Errorf("building health request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.SetBasicAuth(creds.User, creds.Pass)

	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("health request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("health check returned status %d", resp.StatusCode)
	}
	return nil
}

// waitHealthy polls probe until it succeeds or the deadline/timeout elapses.
func waitHealthy(ctx context.Context, probe HealthProbe, addr string, creds Credentials, timeout, interval time.Duration) error {
	deadline := time.Now().Add(timeout)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	var lastErr error
	for {
		if err := probe(ctx, addr, creds); err != nil {
			lastErr = err
		} else {
			return nil
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if time.Now().After(deadline) {
				return fmt.Errorf("daemon did not become healthy within %s: %w", timeout, lastErr)
			}
		}
	}
}
