// Package preview produces the parsed dry-run change set that gates a
// destructive operation (ADR-0015, §7.4). It runs the *same* operation the
// operator is about to confirm with --dry-run, as a sanctioned one-shot CLI
// subprocess (argv-only, no shell — ADR-0004; §7.2.1's "equivalent argv for the
// rare one-shot CLI path"), and parses the result with the changeset package.
//
// The CLI path is used deliberately: the rc `core/command` endpoint does not
// cleanly accept sync's positional source/dest, whereas the one-shot CLI gives a
// deterministic structured report (validated against the pinned rclone). For
// sync/copy/move that report is `--combined` (creates/updates/deletes); bisync,
// which has no --combined, uses `--use-json-log` skip events (deletes exact,
// writes as creates).
package preview

import (
	"context"

	"github.com/conductor-app/conductor/internal/core/changeset"
	"github.com/conductor-app/conductor/internal/core/coreerr"
	"github.com/conductor-app/conductor/internal/core/daemon"
	"github.com/conductor-app/conductor/internal/core/domain"
	"github.com/conductor-app/conductor/internal/core/options"
)

// Runner runs a short-lived command to completion and returns its combined
// output (satisfied by the procrunner adapter / daemon.Runner).
type Runner interface {
	Output(ctx context.Context, spec daemon.Spec) ([]byte, error)
}

// Config configures the Service.
type Config struct {
	// BinaryPath is the pinned, checksum-verified rclone (ADR-0008).
	BinaryPath string
	// ConfigPath is the rclone config file; "" lets rclone use its default.
	ConfigPath string
	Runner     Runner
}

// Service runs dry-run previews.
type Service struct {
	cfg Config
}

// New constructs the Service.
func New(cfg Config) *Service { return &Service{cfg: cfg} }

// parseMode selects which parser reads the captured output.
type parseMode int

const (
	modeCombined parseMode = iota // --combined report (sync/copy/move)
	modeJSONLog                   // --use-json-log skip events (bisync)
)

// Preview runs the operation with --dry-run and parses the result into a
// ChangeSet (ADR-0015). built carries the same validated flags as the real run,
// so filters and options that change what is created/deleted are reflected in
// the preview. A failure to run or capture the dry-run is reported as
// ERR_DRYRUN_PREVIEW_FAILED — the operation must not proceed without a preview.
func (s *Service) Preview(ctx context.Context, kind domain.OperationKind, src, dst domain.Endpoint, built options.Built) (domain.ChangeSet, error) {
	args, mode, err := s.dryRunArgs(kind, src, dst, built)
	if err != nil {
		return domain.ChangeSet{}, err
	}

	out, err := s.cfg.Runner.Output(ctx, daemon.Spec{Path: s.cfg.BinaryPath, Args: args})
	if err != nil {
		return domain.ChangeSet{}, coreerr.New(coreerr.CodeDryRunPreviewFailed,
			"could not run the dry-run preview for this operation", err)
	}

	if mode == modeJSONLog {
		return changeset.ParseJSONLog(out)
	}
	return changeset.ParseCombined(out)
}

// dryRunArgs assembles the argv for the dry-run of kind, mirroring the real
// run's flags (built.Argv) and choosing the capture mechanism per kind.
func (s *Service) dryRunArgs(kind domain.OperationKind, src, dst domain.Endpoint, built options.Built) ([]string, parseMode, error) {
	var (
		args []string
		mode parseMode
	)
	switch kind {
	case domain.KindSync, domain.KindCopy, domain.KindMove:
		args = []string{string(kind), src.String(), dst.String()}
		args = append(args, built.Argv...)
		args = append(args, "--dry-run", "--combined", "-")
		mode = modeCombined
	case domain.KindBisync:
		args = []string{"bisync", src.String(), dst.String()}
		args = append(args, built.Argv...)
		args = append(args, "--dry-run", "--use-json-log")
		mode = modeJSONLog
	default:
		return nil, 0, coreerr.New(coreerr.CodeOptionInvalid,
			"no dry-run preview for operation kind "+string(kind), nil)
	}

	if s.cfg.ConfigPath != "" {
		args = append(args, "--config", s.cfg.ConfigPath)
	}
	return args, mode, nil
}
