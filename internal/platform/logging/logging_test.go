package logging

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// knownToken is a sensitive value that must never survive to log output. The
// charter (§2.4) requires a test proving exactly this.
const knownToken = "s3cr3t-rc-token-do-not-leak"

func TestSensitiveAttributesAreRedacted(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		log  func(l *slog.Logger)
	}{
		{
			name: "top-level sensitive attribute",
			log:  func(l *slog.Logger) { l.Info("starting daemon", "rc_pass", knownToken) },
		},
		{
			name: "sensitive attribute via With",
			log: func(l *slog.Logger) {
				l.With("token", knownToken).Info("authenticated")
			},
		},
		{
			name: "sensitive attribute nested in a group",
			log: func(l *slog.Logger) {
				l.Info("session", slog.Group("rc", slog.String("password", knownToken)))
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var buf bytes.Buffer
			l := New(&buf, Options{Level: slog.LevelDebug, JSON: true})

			tc.log(l)

			out := buf.String()
			assert.NotContains(t, out, knownToken, "secret leaked into log output")
			assert.Contains(t, out, redacted, "redaction placeholder absent")
		})
	}
}

func TestNonSensitiveAttributesPassThrough(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	l := New(&buf, Options{Level: slog.LevelInfo, JSON: true})

	l.Info("listing remotes", "remote", "example-s3", "count", 3)

	out := buf.String()
	assert.Contains(t, out, "example-s3")
	assert.Contains(t, out, `"count":3`)
}

func TestRedactMasksURLUserinfo(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "basic auth in url",
			in:   "http://conductor:" + knownToken + "@127.0.0.1:5572/",
			want: "http://[REDACTED]@127.0.0.1:5572/",
		},
		{
			name: "no userinfo is unchanged",
			in:   "http://127.0.0.1:5572/",
			want: "http://127.0.0.1:5572/",
		},
		{
			name: "plain string is unchanged",
			in:   "core/stats",
			want: "core/stats",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := Redact(tc.in)
			assert.Equal(t, tc.want, got)
			assert.False(t, strings.Contains(got, knownToken), "token survived Redact")
		})
	}
}

func TestEnabledDelegates(t *testing.T) {
	t.Parallel()

	var buf bytes.Buffer
	l := New(&buf, Options{Level: slog.LevelWarn})

	require.False(t, l.Enabled(context.Background(), slog.LevelInfo))
	require.True(t, l.Enabled(context.Background(), slog.LevelError))
}
