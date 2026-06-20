// Package logging configures Conductor's structured operational logger
// (log/slog, §2.4). This logger is deliberately distinct from the
// tamper-evident audit log (§7.8): operational logs serve the operator and
// developer, are rotatable and disposable, and must never contain secrets.
// A redacting handler wraps the chosen slog handler so that sensitive
// attribute values are masked before they reach any output sink.
package logging

import (
	"context"
	"io"
	"log/slog"
	"net/url"
	"strings"
)

// redacted is the placeholder substituted for any sensitive value.
const redacted = "[REDACTED]"

// sensitiveKeys are attribute keys whose values are masked at the logging
// boundary regardless of where they originate. Matching is case-insensitive.
// rc session credentials and the per-install data key (ADR-0009) are the
// primary concern.
var sensitiveKeys = map[string]struct{}{
	"password":      {},
	"passwd":        {},
	"pass":          {},
	"rc_user":       {},
	"rc_pass":       {},
	"token":         {},
	"secret":        {},
	"authorization": {},
	"auth":          {},
	"data_key":      {},
	"apikey":        {},
	"api_key":       {},
	"credential":    {},
	"credentials":   {},
}

// Options configures the operational logger.
type Options struct {
	// Level is the minimum level emitted.
	Level slog.Level
	// JSON selects machine-readable JSON output (production) over the
	// human-readable text handler (development).
	JSON bool
}

// New builds a *slog.Logger writing to w. The returned logger redacts the
// values of known-sensitive attributes (see Redact for free-form strings), so
// callers may safely attach structured fields without auditing every call site.
func New(w io.Writer, opts Options) *slog.Logger {
	ho := &slog.HandlerOptions{Level: opts.Level}
	var h slog.Handler
	if opts.JSON {
		h = slog.NewJSONHandler(w, ho)
	} else {
		h = slog.NewTextHandler(w, ho)
	}
	return slog.New(&redactHandler{inner: h})
}

// Redact masks credentials embedded in a free-form string, currently the
// userinfo component of any URL it contains (for example an rc address written
// as http://user:pass@host). Prefer passing secrets as keyed attributes, which
// the handler redacts automatically; Redact exists for the rare value that must
// be logged whole.
func Redact(s string) string {
	u, err := url.Parse(s)
	if err != nil || u.User == nil {
		return s
	}
	// Drop the userinfo, then splice the placeholder back in literally. Using
	// url.User would percent-encode the placeholder's brackets; rebuilding by
	// hand keeps the masked output readable.
	u.User = nil
	rest := u.String()
	if i := strings.Index(rest, "://"); i >= 0 {
		return rest[:i+3] + redacted + "@" + rest[i+3:]
	}
	return rest
}

// redactHandler is a slog.Handler middleware that masks the values of
// sensitive attributes before delegating to the wrapped handler.
type redactHandler struct {
	inner slog.Handler
}

func (h *redactHandler) Enabled(ctx context.Context, level slog.Level) bool {
	return h.inner.Enabled(ctx, level)
}

func (h *redactHandler) Handle(ctx context.Context, r slog.Record) error {
	clone := slog.NewRecord(r.Time, r.Level, r.Message, r.PC)
	r.Attrs(func(a slog.Attr) bool {
		clone.AddAttrs(redactAttr(a))
		return true
	})
	return h.inner.Handle(ctx, clone)
}

func (h *redactHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	red := make([]slog.Attr, len(attrs))
	for i, a := range attrs {
		red[i] = redactAttr(a)
	}
	return &redactHandler{inner: h.inner.WithAttrs(red)}
}

func (h *redactHandler) WithGroup(name string) slog.Handler {
	return &redactHandler{inner: h.inner.WithGroup(name)}
}

// redactAttr masks a single attribute, recursing into groups so a sensitive key
// nested under a group is still caught.
func redactAttr(a slog.Attr) slog.Attr {
	if _, ok := sensitiveKeys[strings.ToLower(a.Key)]; ok {
		return slog.String(a.Key, redacted)
	}
	if a.Value.Kind() == slog.KindGroup {
		group := a.Value.Group()
		nested := make([]any, 0, len(group))
		for _, ga := range group {
			nested = append(nested, redactAttr(ga))
		}
		return slog.Group(a.Key, nested...)
	}
	return a
}
