// Package pairs orchestrates reusable sync/bisync pairs and named option
// profiles (§7.5, §7.7). It turns a saved pair plus its profile into a
// validated transfer run: it resolves the effective governance ceilings for the
// endpoints' remotes (ADR-0013, §7.6) and forces a dry-run on a pair's first
// run, the safe default for never-before-run pairs (§7.4). The pair's options
// are still validated and its destructive impacts still acknowledged by the
// transfers service; this package only assembles the request.
package pairs

import (
	"context"
	"time"

	"github.com/conductor-app/conductor/internal/core/coreerr"
	"github.com/conductor-app/conductor/internal/core/domain"
	"github.com/conductor-app/conductor/internal/core/governance"
	"github.com/conductor-app/conductor/internal/core/options"
	"github.com/conductor-app/conductor/internal/core/ports"
	"github.com/conductor-app/conductor/internal/core/transfers"
)

// Store is the persistence port for saved pairs, profiles, and per-remote
// ceilings (satisfied by the sqlitestore adapter).
type Store interface {
	Pair(ctx context.Context, id string) (domain.SavedPair, bool, error)
	Pairs(ctx context.Context) ([]domain.SavedPair, error)
	SavePair(ctx context.Context, p domain.SavedPair) error
	DeletePair(ctx context.Context, id string) error
	TouchPairRun(ctx context.Context, id string, at time.Time) error

	Profile(ctx context.Context, id string) (domain.Profile, bool, error)
	Profiles(ctx context.Context) ([]domain.Profile, error)
	SaveProfile(ctx context.Context, p domain.Profile) error
	DeleteProfile(ctx context.Context, id string) error

	Ceiling(ctx context.Context, remote string) (domain.RemoteCeiling, bool, error)
	Ceilings(ctx context.Context) ([]domain.RemoteCeiling, error)
	SetCeiling(ctx context.Context, c domain.RemoteCeiling) error
}

// Runner starts a validated operation; satisfied by *transfers.Service.
type Runner interface {
	Start(ctx context.Context, req transfers.RunRequest) (transfers.RunHandle, error)
}

// Auditor records governance-ceiling changes (§7.8).
type Auditor interface {
	Record(ctx context.Context, action domain.AuditAction, subject string, detail any) (domain.AuditEntry, error)
}

// Config configures the Service.
type Config struct {
	Store   Store
	Runner  Runner
	Catalog *options.Catalog
	Audit   Auditor
	// Defaults are the global conservative governance ceilings (ADR-0013); the
	// per-remote ceilings of a run's endpoints tighten these further.
	Defaults options.Ceilings
	Clock    ports.Clock
}

// Service assembles and starts saved-pair runs and manages profiles/ceilings.
type Service struct {
	cfg Config
}

// New constructs the Service, applying defaults.
func New(cfg Config) *Service {
	if cfg.Clock == nil {
		cfg.Clock = ports.SystemClock{}
	}
	return &Service{cfg: cfg}
}

// Run starts the saved pair identified by id. A pair that has never run is
// forced to dry-run (§7.4); once started, the pair is stamped as run so its next
// run is live by default. Destructive acknowledgement is enforced downstream by
// the transfers service, so acknowledged is threaded through unchanged.
func (s *Service) Run(ctx context.Context, pairID string, acknowledged bool) (transfers.RunHandle, error) {
	req, pair, err := s.buildRun(ctx, pairID, acknowledged)
	if err != nil {
		return transfers.RunHandle{}, err
	}

	handle, err := s.cfg.Runner.Start(ctx, req)
	if err != nil {
		return transfers.RunHandle{}, err
	}

	// The pair has now run at least once; only stamp the first time so we don't
	// rewrite the timestamp on every subsequent run.
	if !pair.HasRun() {
		if err := s.cfg.Store.TouchPairRun(ctx, pair.ID, s.cfg.Clock.Now().UTC()); err != nil {
			return transfers.RunHandle{}, err
		}
	}
	return handle, nil
}

// BuildRun assembles the transfers request for a saved pair without starting it,
// so a caller can preview the resolved endpoints, options, and ceilings (§7.4).
func (s *Service) BuildRun(ctx context.Context, pairID string, acknowledged bool) (transfers.RunRequest, error) {
	req, _, err := s.buildRun(ctx, pairID, acknowledged)
	return req, err
}

func (s *Service) buildRun(ctx context.Context, pairID string, acknowledged bool) (transfers.RunRequest, domain.SavedPair, error) {
	pair, ok, err := s.cfg.Store.Pair(ctx, pairID)
	if err != nil {
		return transfers.RunRequest{}, domain.SavedPair{}, err
	}
	if !ok {
		return transfers.RunRequest{}, domain.SavedPair{},
			coreerr.New(coreerr.CodeOptionInvalid, "no saved pair "+pairID, nil)
	}

	kind, err := pairKind(pair.Kind)
	if err != nil {
		return transfers.RunRequest{}, domain.SavedPair{}, err
	}

	sel, err := s.selectionFor(ctx, pair)
	if err != nil {
		return transfers.RunRequest{}, domain.SavedPair{}, err
	}

	// A never-run pair defaults to dry-run: the operator reviews the simulated
	// result before a live run is offered (§7.4, §7.11.6).
	if !pair.HasRun() {
		sel.Single["--dry-run"] = "true"
	}

	src := domain.ParseEndpoint(pair.Path1)
	dst := domain.ParseEndpoint(pair.Path2)

	ceilings, err := s.resolveCeilings(ctx, src.Remote, dst.Remote)
	if err != nil {
		return transfers.RunRequest{}, domain.SavedPair{}, err
	}

	return transfers.RunRequest{
		Kind:         kind,
		Src:          src,
		Dst:          dst,
		Selection:    sel,
		Ceilings:     ceilings,
		Acknowledged: acknowledged,
	}, pair, nil
}

// selectionFor builds the option selection from the pair's profile, splitting
// list options into Multi and scalars into Single per the catalog.
func (s *Service) selectionFor(ctx context.Context, pair domain.SavedPair) (options.Selection, error) {
	sel := options.Selection{Single: map[string]string{}, Multi: map[string][]string{}}
	if pair.ProfileID == "" {
		return sel, nil
	}

	prof, ok, err := s.cfg.Store.Profile(ctx, pair.ProfileID)
	if err != nil {
		return options.Selection{}, err
	}
	if !ok {
		return options.Selection{}, coreerr.New(coreerr.CodeOptionInvalid,
			"saved pair references missing profile "+pair.ProfileID, nil)
	}

	for _, o := range prof.Options {
		if opt, found := s.cfg.Catalog.Lookup(o.Flag); found && opt.Type == options.TypeList {
			sel.Multi[o.Flag] = append(sel.Multi[o.Flag], o.Value)
			continue
		}
		sel.Single[o.Flag] = o.Value
	}
	return sel, nil
}

// resolveCeilings combines the global defaults with the per-remote ceilings of
// the run's endpoints, taking the most restrictive (§7.6).
func (s *Service) resolveCeilings(ctx context.Context, remotes ...string) (options.Ceilings, error) {
	all, err := s.cfg.Store.Ceilings(ctx)
	if err != nil {
		return options.Ceilings{}, err
	}
	byRemote := make(map[string]domain.RemoteCeiling, len(all))
	for _, c := range all {
		byRemote[c.Remote] = c
	}
	lookup := func(remote string) (domain.RemoteCeiling, bool) {
		c, ok := byRemote[remote]
		return c, ok
	}
	return governance.Resolve(s.cfg.Defaults, remotes, lookup), nil
}

// pairKind maps a saved-pair kind to its operation kind.
func pairKind(k domain.PairKind) (domain.OperationKind, error) {
	switch k {
	case domain.PairSync:
		return domain.KindSync, nil
	case domain.PairBisync:
		return domain.KindBisync, nil
	default:
		return "", coreerr.New(coreerr.CodeOptionInvalid, "unknown pair kind "+string(k), nil)
	}
}

// --- pass-through management of pairs, profiles, and ceilings ---------------

// SavePair persists a saved pair (create or edit).
func (s *Service) SavePair(ctx context.Context, p domain.SavedPair) error {
	return s.cfg.Store.SavePair(ctx, p)
}

// Pairs lists all saved pairs.
func (s *Service) Pairs(ctx context.Context) ([]domain.SavedPair, error) {
	return s.cfg.Store.Pairs(ctx)
}

// DeletePair removes a saved pair.
func (s *Service) DeletePair(ctx context.Context, id string) error {
	return s.cfg.Store.DeletePair(ctx, id)
}

// SaveProfile persists a named option profile.
func (s *Service) SaveProfile(ctx context.Context, p domain.Profile) error {
	return s.cfg.Store.SaveProfile(ctx, p)
}

// Profiles lists all profiles.
func (s *Service) Profiles(ctx context.Context) ([]domain.Profile, error) {
	return s.cfg.Store.Profiles(ctx)
}

// DeleteProfile removes a profile.
func (s *Service) DeleteProfile(ctx context.Context, id string) error {
	return s.cfg.Store.DeleteProfile(ctx, id)
}

// Ceilings lists all per-remote governance ceilings.
func (s *Service) Ceilings(ctx context.Context) ([]domain.RemoteCeiling, error) {
	return s.cfg.Store.Ceilings(ctx)
}

// SetCeiling saves a per-remote governance ceiling and records the change in the
// audit log: a ceiling can only be relaxed by an explicit, recorded edit (§7.6,
// §7.8).
func (s *Service) SetCeiling(ctx context.Context, c domain.RemoteCeiling) error {
	if err := s.cfg.Store.SetCeiling(ctx, c); err != nil {
		return err
	}
	if s.cfg.Audit != nil {
		if _, err := s.cfg.Audit.Record(ctx, domain.ActionGovernanceCeilingSet, c.Remote, map[string]any{
			"transfers": c.Transfers, "checkers": c.Checkers, "bwlimit": c.Bwlimit, "tpslimit": c.Tpslimit,
		}); err != nil {
			return err
		}
	}
	return nil
}
