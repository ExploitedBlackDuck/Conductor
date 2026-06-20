// Package transfers runs copy/move operations through the rc daemon and records
// them as durable history (§7.7, §7.10 P5). It builds the validated command from
// the option catalog (ADR-0011), starts the job, writes an audit entry, and —
// when the job finishes or is cancelled — captures the job's stats, seals them
// at rest (ADR-0009), and persists an immutable Operation row with its options.
// Cancellation propagates through job/stop and the watcher's context (§2.3).
package transfers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/conductor-app/conductor/internal/core/coreerr"
	"github.com/conductor-app/conductor/internal/core/domain"
	"github.com/conductor-app/conductor/internal/core/options"
	"github.com/conductor-app/conductor/internal/core/ports"
	"github.com/conductor-app/conductor/internal/core/secrets"
)

// RC is the subset of the rc client the transfers service needs.
type RC interface {
	SyncCopy(ctx context.Context, srcFs, dstFs string, config map[string]any, filter map[string][]string, async bool) (int64, error)
	SyncMove(ctx context.Context, srcFs, dstFs string, config map[string]any, filter map[string][]string, deleteEmptySrcDirs, async bool) (int64, error)
	SyncSync(ctx context.Context, srcFs, dstFs string, config map[string]any, filter map[string][]string, async bool) (int64, error)
	SyncBisync(ctx context.Context, path1, path2 string, resync, dryRun bool, config map[string]any, async bool) (int64, error)
	JobStop(ctx context.Context, id int64) error
	JobStatus(ctx context.Context, id int64) (domain.JobStatus, error)
	CoreStatsForGroup(ctx context.Context, group string) (domain.TransferStats, error)
}

// Provider builds an RC client bound to the live daemon session.
type Provider func() (RC, error)

// Store persists completed operations (the consumer-defined port, §2.1).
type Store interface {
	InsertOperation(ctx context.Context, op domain.Operation, opts []domain.OperationOption, log *domain.CapturedLog) error
}

// Auditor records audit entries.
type Auditor interface {
	Record(ctx context.Context, action domain.AuditAction, subject string, detail any) (domain.AuditEntry, error)
}

// Sealer seals captured logs at rest (ADR-0009).
type Sealer interface {
	Seal(plaintext, additionalData []byte) (secrets.Sealed, error)
}

// RunRequest describes an operation to run.
type RunRequest struct {
	Kind         domain.OperationKind
	Src          domain.Endpoint
	Dst          domain.Endpoint
	Selection    options.Selection
	Ceilings     options.Ceilings
	Acknowledged bool
}

// RunHandle identifies a started operation.
type RunHandle struct {
	OperationID string
	JobID       int64
}

// Config configures the Service.
type Config struct {
	RC        Provider
	Store     Store
	Audit     Auditor
	Sealer    Sealer
	Catalog   *options.Catalog
	Version   string
	Logger    *slog.Logger
	Clock     ports.Clock
	PollEvery time.Duration
	// NewID generates operation ids; defaults to a random hex id.
	NewID func() string
}

// Service starts, watches, and records transfer operations.
type Service struct {
	cfg Config

	mu     sync.Mutex
	active map[string]*run
	wg     sync.WaitGroup
}

type run struct {
	op        domain.Operation
	jobID     int64
	group     string
	opts      []domain.OperationOption
	cancel    context.CancelFunc
	cancelled bool
}

// New constructs the Service, applying defaults.
func New(cfg Config) *Service {
	if cfg.Clock == nil {
		cfg.Clock = ports.SystemClock{}
	}
	if cfg.PollEvery <= 0 {
		cfg.PollEvery = 500 * time.Millisecond
	}
	if cfg.NewID == nil {
		cfg.NewID = randomID
	}
	return &Service{cfg: cfg, active: map[string]*run{}}
}

// Start validates the selection, enforces destructive acknowledgement, starts
// the rc job, records the start in the audit log, and begins watching the job
// to capture and persist it on completion.
func (s *Service) Start(ctx context.Context, req RunRequest) (RunHandle, error) {
	switch req.Kind {
	case domain.KindCopy, domain.KindMove, domain.KindSync, domain.KindBisync:
	default:
		return RunHandle{}, coreerr.New(coreerr.CodeOptionInvalid, "unsupported operation kind "+string(req.Kind), nil)
	}

	built, err := s.cfg.Catalog.Build(req.Selection, req.Kind, req.Ceilings)
	if err != nil {
		return RunHandle{}, err
	}

	// Central safety property (§7.4): any selection the impact engine flags as
	// requiring acknowledgement must carry it.
	impacts := s.cfg.Catalog.Evaluate(options.EvalInput{
		Selection: req.Selection, Kind: req.Kind, Src: req.Src, Dst: req.Dst, Ceilings: req.Ceilings,
	})
	// A dry-run makes no changes, so it never requires destructive
	// acknowledgement (§7.4 — dry-run is always one click away).
	dryRun := selectedBool(req.Selection, "--dry-run")
	if requiresAck(impacts) && !req.Acknowledged && !dryRun {
		return RunHandle{}, coreerr.New(coreerr.CodeDestructiveNotConfirmed,
			"this operation requires explicit acknowledgement before it can run", nil)
	}

	rc, err := s.cfg.RC()
	if err != nil {
		return RunHandle{}, err
	}

	opID := s.cfg.NewID()
	startedAt := s.cfg.Clock.Now().UTC()
	src, dst := req.Src.String(), req.Dst.String()

	if _, err := s.cfg.Audit.Record(ctx, domain.ActionOperationStart, opID, map[string]any{
		"kind": req.Kind, "src": src, "dst": dst, "argv": built.Argv,
	}); err != nil {
		return RunHandle{}, fmt.Errorf("recording operation start: %w", err)
	}
	if req.Acknowledged && requiresAck(impacts) {
		if _, err := s.cfg.Audit.Record(ctx, domain.ActionRiskAcknowledged, opID, map[string]any{"kind": req.Kind}); err != nil {
			return RunHandle{}, fmt.Errorf("recording risk acknowledgement: %w", err)
		}
	}

	jobID, err := s.startJob(ctx, rc, req, built)
	if err != nil {
		return RunHandle{}, err
	}

	op := domain.Operation{
		ID:            opID,
		Kind:          req.Kind,
		Src:           src,
		Dst:           dst,
		RcloneVersion: s.cfg.Version,
		Intensity:     intensityJSON(req.Ceilings),
		StartedAt:     startedAt,
		Result:        domain.ResultRunning,
	}
	r := &run{op: op, jobID: jobID, group: fmt.Sprintf("job/%d", jobID), opts: toOperationOptions(built.Effective, req.Acknowledged)}

	// The watcher must outlive this Start call's ctx: the run continues after
	// Start returns, and Close (not the request) owns its shutdown (§2.3).
	watchCtx, cancel := context.WithCancel(context.Background())
	r.cancel = cancel

	s.mu.Lock()
	s.active[opID] = r
	s.mu.Unlock()

	s.wg.Add(1)
	go s.watch(watchCtx, r) //nolint:gosec,contextcheck // intentional: watcher lifetime is bound to Close, not the start ctx

	return RunHandle{OperationID: opID, JobID: jobID}, nil
}

// Cancel requests cancellation of a running operation. The job is stopped and
// the watcher records the operation as cancelled, not failed (§7.11.4).
func (s *Service) Cancel(ctx context.Context, operationID string) error {
	s.mu.Lock()
	r, ok := s.active[operationID]
	if ok {
		r.cancelled = true
	}
	s.mu.Unlock()
	if !ok {
		return coreerr.New(coreerr.CodeJobCancelled, "no active operation "+operationID, nil)
	}

	rc, err := s.cfg.RC()
	if err != nil {
		return err
	}
	if err := rc.JobStop(ctx, r.jobID); err != nil {
		return err
	}
	return nil
}

// Close cancels all watchers and waits for them to finish, persisting whatever
// they captured. It makes the service safe to shut down without orphaning work.
func (s *Service) Close() {
	s.mu.Lock()
	for _, r := range s.active {
		r.cancel()
	}
	s.mu.Unlock()
	s.wg.Wait()
}

func (s *Service) startJob(ctx context.Context, rc RC, req RunRequest, built options.Built) (int64, error) {
	src, dst := req.Src.String(), req.Dst.String()
	switch req.Kind {
	case domain.KindCopy:
		return rc.SyncCopy(ctx, src, dst, built.ConfigParams, built.FilterParams, true)
	case domain.KindMove:
		return rc.SyncMove(ctx, src, dst, built.ConfigParams, built.FilterParams, false, true)
	case domain.KindSync:
		return rc.SyncSync(ctx, src, dst, built.ConfigParams, built.FilterParams, true)
	case domain.KindBisync:
		resync := selectedBool(req.Selection, "--resync")
		dryRun := selectedBool(req.Selection, "--dry-run")
		return rc.SyncBisync(ctx, src, dst, resync, dryRun, built.ConfigParams, true)
	default:
		return 0, coreerr.New(coreerr.CodeOptionInvalid, "unsupported transfer kind", nil)
	}
}

// selectedBool reports whether a boolean flag is set true in the selection.
func selectedBool(sel options.Selection, flag string) bool {
	v := sel.Single[flag]
	return v == "true" || v == "1"
}

// watch polls the job until it finishes or the service is shutting down, then
// finalizes it exactly once.
func (s *Service) watch(ctx context.Context, r *run) {
	defer s.wg.Done()

	ticker := time.NewTicker(s.cfg.PollEvery)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			// The watcher context is cancelled on shutdown; finalize with a
			// fresh context so the operation is still captured and persisted.
			s.finalize(context.Background(), r, true) //nolint:contextcheck // see comment
			return
		case <-ticker.C:
			rc, err := s.cfg.RC()
			if err != nil {
				continue
			}
			st, err := rc.JobStatus(ctx, r.jobID)
			if err != nil {
				continue
			}
			if st.Finished {
				s.finalize(ctx, r, false)
				return
			}
		}
	}
}

// finalize captures the job's stats, seals them, persists the operation, and
// records the stop in the audit log.
func (s *Service) finalize(ctx context.Context, r *run, stopping bool) {
	// Bound the finalize work so a shutdown cannot hang.
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	rc, err := s.cfg.RC()
	if err != nil {
		s.cfg.Logger.Error("finalize: rc unavailable", "op", r.op.ID, "error", err)
		s.remove(r.op.ID)
		return
	}

	stats, statsErr := rc.CoreStatsForGroup(ctx, r.group)
	status, statusErr := rc.JobStatus(ctx, r.jobID)

	r.op.EndedAt = s.cfg.Clock.Now().UTC()
	r.op.Result = resultFor(r, stopping, status, statusErr)
	if statsErr == nil {
		r.op.BytesMoved = stats.Bytes
		r.op.FilesMoved = stats.Transfers
	}

	captured := s.sealLog(r, stats, status, statsErr, statusErr)
	if captured != nil {
		r.op.LogRef = captured.ID
	}

	if err := s.cfg.Store.InsertOperation(ctx, r.op, r.opts, captured); err != nil {
		s.cfg.Logger.Error("finalize: persisting operation failed", "op", r.op.ID, "error", err)
	}
	if _, err := s.cfg.Audit.Record(ctx, domain.ActionOperationStop, r.op.ID, map[string]any{
		"result": r.op.Result, "bytes": r.op.BytesMoved, "files": r.op.FilesMoved,
	}); err != nil {
		s.cfg.Logger.Error("finalize: recording operation stop failed", "op", r.op.ID, "error", err)
	}

	s.remove(r.op.ID)
}

func (s *Service) sealLog(r *run, stats domain.TransferStats, status domain.JobStatus, statsErr, statusErr error) *domain.CapturedLog {
	payload := map[string]any{"stats": stats, "jobStatus": status}
	if statsErr != nil {
		payload["statsError"] = statsErr.Error()
	}
	if statusErr != nil {
		payload["jobStatusError"] = statusErr.Error()
	}
	plaintext, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	sealed, err := s.cfg.Sealer.Seal(plaintext, []byte(r.op.ID))
	if err != nil {
		s.cfg.Logger.Error("finalize: sealing captured log failed", "op", r.op.ID, "error", err)
		return nil
	}
	sum := sha256.Sum256(plaintext)
	return &domain.CapturedLog{
		ID:              s.cfg.NewID(),
		OperationID:     r.op.ID,
		Nonce:           sealed.Nonce,
		SealedBytes:     sealed.Ciphertext,
		SHA256Plaintext: hex.EncodeToString(sum[:]),
		BytesLen:        len(plaintext),
	}
}

func (s *Service) remove(opID string) {
	s.mu.Lock()
	delete(s.active, opID)
	s.mu.Unlock()
}

func resultFor(r *run, stopping bool, status domain.JobStatus, statusErr error) domain.Result {
	if r.cancelled || stopping {
		return domain.ResultCancelled
	}
	if statusErr != nil {
		return domain.ResultFailed
	}
	if status.Success {
		return domain.ResultSuccess
	}
	return domain.ResultFailed
}

func toOperationOptions(eff []options.EffectiveOption, acknowledged bool) []domain.OperationOption {
	out := make([]domain.OperationOption, 0, len(eff))
	for _, e := range eff {
		out = append(out, domain.OperationOption{
			Flag:         e.Flag,
			Value:        e.Value,
			Risk:         string(e.Risk),
			Acknowledged: acknowledged && e.AffectsData,
		})
	}
	return out
}

func requiresAck(impacts []options.Impact) bool {
	for _, im := range impacts {
		if im.Level == options.ImpactRequireAck || im.Level == options.ImpactBlock {
			return true
		}
	}
	return false
}

func intensityJSON(c options.Ceilings) string {
	b, err := json.Marshal(map[string]any{
		"transfers": c.Transfers, "checkers": c.Checkers, "bwlimit": c.Bwlimit, "tpslimit": c.Tpslimit,
	})
	if err != nil {
		return "{}"
	}
	return string(b)
}

func randomID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
