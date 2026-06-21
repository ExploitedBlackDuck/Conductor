package verify

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"

	"github.com/conductor-app/conductor/internal/core/coreerr"
	"github.com/conductor-app/conductor/internal/core/daemon"
	"github.com/conductor-app/conductor/internal/core/domain"
	"github.com/conductor-app/conductor/internal/core/ports"
)

// RC is the subset of the rc client the verify service needs: a structured
// check whose combined report is returned directly (operations/check).
type RC interface {
	OperationsCheck(ctx context.Context, src, dst string, oneway bool) (combined []string, err error)
}

// Provider builds an RC client bound to the live daemon session.
type Provider func() (RC, error)

// Runner runs a short-lived command to completion and returns its combined
// output, used for cryptcheck (which rclone does not expose over rc).
type Runner interface {
	Output(ctx context.Context, spec daemon.Spec) ([]byte, error)
}

// Store persists verification results (§7.7).
type Store interface {
	InsertVerification(ctx context.Context, v domain.Verification) error
	Verifications(ctx context.Context, limit int) ([]domain.Verification, error)
}

// Auditor hash-chains the verification result into the audit log (§7.8).
type Auditor interface {
	Record(ctx context.Context, action domain.AuditAction, subject string, detail any) (domain.AuditEntry, error)
}

// Config configures the Service.
type Config struct {
	RC         Provider
	Runner     Runner
	BinaryPath string
	ConfigPath string
	Store      Store
	Audit      Auditor
	Clock      ports.Clock
	// NewID generates verification ids; defaults to a random hex id.
	NewID func() string
}

// Service runs and records integrity verifications.
type Service struct {
	cfg Config
}

// New constructs the Service, applying defaults.
func New(cfg Config) *Service {
	if cfg.Clock == nil {
		cfg.Clock = ports.SystemClock{}
	}
	if cfg.NewID == nil {
		cfg.NewID = randomID
	}
	return &Service{cfg: cfg}
}

// Result is a completed verification with the offending paths for live display.
// The paths are returned to the UI but not persisted (§7.12).
type Result struct {
	Verification domain.Verification
	Differ       []string
	MissingOnSrc []string
	MissingOnDst []string
	Errors       []string
}

// Run performs an integrity check of src against dst, records it (persisted and
// hash-chained into the audit log), and returns the verdict with the offending
// paths. It mutates nothing. oneway limits the comparison to files on the source.
func (s *Service) Run(ctx context.Context, kind domain.VerificationKind, src, dst domain.Endpoint, oneway bool) (Result, error) {
	startedAt := s.cfg.Clock.Now().UTC()

	report, err := s.compare(ctx, kind, src, dst, oneway)
	if err != nil {
		return Result{}, err
	}

	v := domain.Verification{
		ID:         s.cfg.NewID(),
		Kind:       kind,
		Src:        src.String(),
		Dst:        dst.String(),
		StartedAt:  startedAt,
		EndedAt:    s.cfg.Clock.Now().UTC(),
		Match:      report.Match,
		Differ:     report.DifferCount(),
		Missing:    report.Missing(),
		ErrorCount: report.ErrorCount(),
		Result:     domain.ResultFor(report.DifferCount(), report.Missing(), report.ErrorCount()),
	}

	if err := s.cfg.Store.InsertVerification(ctx, v); err != nil {
		return Result{}, err
	}
	if _, err := s.cfg.Audit.Record(ctx, domain.ActionVerification, v.ID, map[string]any{
		"kind": v.Kind, "src": v.Src, "dst": v.Dst, "result": v.Result,
		"match": v.Match, "differ": v.Differ, "missing": v.Missing, "errors": v.ErrorCount,
	}); err != nil {
		return Result{}, fmt.Errorf("recording verification: %w", err)
	}

	return Result{
		Verification: v,
		Differ:       report.Differ,
		MissingOnSrc: report.MissingOnSrc,
		MissingOnDst: report.MissingOnDst,
		Errors:       report.Errors,
	}, nil
}

// Recent returns the most recent verifications for the history/Verify view.
func (s *Service) Recent(ctx context.Context, limit int) ([]domain.Verification, error) {
	return s.cfg.Store.Verifications(ctx, limit)
}

// compare obtains the combined report for the given kind: check over rc,
// cryptcheck as a one-shot CLI subprocess.
func (s *Service) compare(ctx context.Context, kind domain.VerificationKind, src, dst domain.Endpoint, oneway bool) (Report, error) {
	switch kind {
	case domain.VerifyCheck:
		rc, err := s.cfg.RC()
		if err != nil {
			return Report{}, err
		}
		combined, err := rc.OperationsCheck(ctx, src.String(), dst.String(), oneway)
		if err != nil {
			return Report{}, coreerr.New(coreerr.CodeRCRequest, "running integrity check", err)
		}
		return ParseCombined(combined), nil
	case domain.VerifyCryptcheck:
		return s.cryptcheckCLI(ctx, src, dst, oneway)
	default:
		return Report{}, coreerr.New(coreerr.CodeOptionInvalid, "unknown verification kind "+string(kind), nil)
	}
}

// cryptcheckCLI runs `rclone cryptcheck` as a one-shot subprocess and parses its
// --combined report. cryptcheck exits non-zero when files differ — that is a
// result, not a run failure — so the output is parsed regardless of exit, and
// the error is only fatal when nothing parseable was produced.
func (s *Service) cryptcheckCLI(ctx context.Context, src, dst domain.Endpoint, oneway bool) (Report, error) {
	args := []string{"cryptcheck", src.String(), dst.String(), "--combined", "-"}
	if oneway {
		args = append(args, "--one-way")
	}
	if s.cfg.ConfigPath != "" {
		args = append(args, "--config", s.cfg.ConfigPath)
	}
	out, runErr := s.cfg.Runner.Output(ctx, daemon.Spec{Path: s.cfg.BinaryPath, Args: args})
	report := ParseCombinedBytes(out)
	// A non-zero exit accompanies any mismatch; only treat it as a failure when
	// the command produced nothing to classify.
	if runErr != nil && report.Match == 0 && report.Missing() == 0 && report.DifferCount() == 0 && report.ErrorCount() == 0 {
		return Report{}, coreerr.New(coreerr.CodeRCRequest, "running cryptcheck", runErr)
	}
	return report, nil
}

func randomID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
