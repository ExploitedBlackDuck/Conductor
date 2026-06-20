package app

import (
	"context"
	"time"

	"github.com/conductor-app/conductor/internal/core/domain"
)

// PairDTO is one saved sync/bisync pair for the frontend (§7.11.6). HasRun lets
// the UI default a never-run pair's first run to dry-run.
type PairDTO struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	Path1     string `json:"path1"`
	Path2     string `json:"path2"`
	ProfileID string `json:"profileId"`
	LastRun   string `json:"lastRun"`
	HasRun    bool   `json:"hasRun"`
}

// ProfileDTO is a named, reusable option set (§7.5).
type ProfileDTO struct {
	ID      string             `json:"id"`
	Name    string             `json:"name"`
	Kind    string             `json:"kind"`
	Options []ProfileOptionDTO `json:"options"`
}

// ProfileOptionDTO is one flag+value within a profile.
type ProfileOptionDTO struct {
	Flag  string `json:"flag"`
	Value string `json:"value"`
}

// CeilingDTO is a per-remote governance ceiling (§7.6, ADR-0013).
type CeilingDTO struct {
	Remote    string `json:"remote"`
	Transfers int    `json:"transfers"`
	Checkers  int    `json:"checkers"`
	Bwlimit   string `json:"bwlimit"`
	Tpslimit  int    `json:"tpslimit"`
}

// PairsResultDTO is the list of saved pairs or a typed error.
type PairsResultDTO struct {
	Pairs []PairDTO `json:"pairs"`
	Error *ErrorDTO `json:"error"`
}

// ProfilesResultDTO is the list of profiles or a typed error.
type ProfilesResultDTO struct {
	Profiles []ProfileDTO `json:"profiles"`
	Error    *ErrorDTO    `json:"error"`
}

// CeilingsResultDTO is the list of per-remote ceilings or a typed error.
type CeilingsResultDTO struct {
	Ceilings []CeilingDTO `json:"ceilings"`
	Error    *ErrorDTO    `json:"error"`
}

// ListPairs returns the saved sync/bisync pairs (§7.11.6).
func (a *App) ListPairs() PairsResultDTO {
	ps, err := a.pairs.Pairs(context.Background())
	if err != nil {
		return PairsResultDTO{Error: errorToDTO(err)}
	}
	out := make([]PairDTO, 0, len(ps))
	for _, p := range ps {
		out = append(out, toPairDTO(p))
	}
	return PairsResultDTO{Pairs: out}
}

// SavePair creates or updates a saved pair.
func (a *App) SavePair(p PairDTO) *ErrorDTO {
	if err := a.pairs.SavePair(context.Background(), fromPairDTO(p)); err != nil {
		return errorToDTO(err)
	}
	return nil
}

// DeletePair removes a saved pair.
func (a *App) DeletePair(id string) *ErrorDTO {
	if err := a.pairs.DeletePair(context.Background(), id); err != nil {
		return errorToDTO(err)
	}
	return nil
}

// RunPair runs a saved pair. A never-run pair runs as a dry-run regardless of
// acknowledged; destructive selections still require acknowledged=true (§7.4).
func (a *App) RunPair(id string, acknowledged bool) RunResultDTO {
	h, err := a.pairs.Run(context.Background(), id, acknowledged)
	if err != nil {
		return RunResultDTO{Error: errorToDTO(err)}
	}
	return RunResultDTO{OperationID: h.OperationID, JobID: h.JobID}
}

// ListProfiles returns the named option profiles (§7.5).
func (a *App) ListProfiles() ProfilesResultDTO {
	ps, err := a.pairs.Profiles(context.Background())
	if err != nil {
		return ProfilesResultDTO{Error: errorToDTO(err)}
	}
	out := make([]ProfileDTO, 0, len(ps))
	for _, p := range ps {
		out = append(out, toProfileDTO(p))
	}
	return ProfilesResultDTO{Profiles: out}
}

// SaveProfile creates or updates a named option profile.
func (a *App) SaveProfile(p ProfileDTO) *ErrorDTO {
	if err := a.pairs.SaveProfile(context.Background(), fromProfileDTO(p)); err != nil {
		return errorToDTO(err)
	}
	return nil
}

// DeleteProfile removes a profile.
func (a *App) DeleteProfile(id string) *ErrorDTO {
	if err := a.pairs.DeleteProfile(context.Background(), id); err != nil {
		return errorToDTO(err)
	}
	return nil
}

// ListCeilings returns the per-remote governance ceilings (§7.6).
func (a *App) ListCeilings() CeilingsResultDTO {
	cs, err := a.pairs.Ceilings(context.Background())
	if err != nil {
		return CeilingsResultDTO{Error: errorToDTO(err)}
	}
	out := make([]CeilingDTO, 0, len(cs))
	for _, c := range cs {
		out = append(out, toCeilingDTO(c))
	}
	return CeilingsResultDTO{Ceilings: out}
}

// SetCeiling saves a per-remote governance ceiling; the change is audited (§7.8).
func (a *App) SetCeiling(c CeilingDTO) *ErrorDTO {
	if err := a.pairs.SetCeiling(context.Background(), fromCeilingDTO(c)); err != nil {
		return errorToDTO(err)
	}
	return nil
}

func toPairDTO(p domain.SavedPair) PairDTO {
	dto := PairDTO{
		ID: p.ID, Name: p.Name, Kind: string(p.Kind),
		Path1: p.Path1, Path2: p.Path2, ProfileID: p.ProfileID, HasRun: p.HasRun(),
	}
	if !p.LastRun.IsZero() {
		dto.LastRun = p.LastRun.Format(time.RFC3339)
	}
	return dto
}

func fromPairDTO(p PairDTO) domain.SavedPair {
	return domain.SavedPair{
		ID: p.ID, Name: p.Name, Kind: domain.PairKind(p.Kind),
		Path1: p.Path1, Path2: p.Path2, ProfileID: p.ProfileID,
		// LastRun is owned by the core (TouchPairRun); the frontend never sets it.
	}
}

func toProfileDTO(p domain.Profile) ProfileDTO {
	opts := make([]ProfileOptionDTO, 0, len(p.Options))
	for _, o := range p.Options {
		opts = append(opts, ProfileOptionDTO{Flag: o.Flag, Value: o.Value})
	}
	return ProfileDTO{ID: p.ID, Name: p.Name, Kind: string(p.Kind), Options: opts}
}

func fromProfileDTO(p ProfileDTO) domain.Profile {
	opts := make([]domain.ProfileOption, 0, len(p.Options))
	for _, o := range p.Options {
		opts = append(opts, domain.ProfileOption{Flag: o.Flag, Value: o.Value})
	}
	return domain.Profile{ID: p.ID, Name: p.Name, Kind: domain.OperationKind(p.Kind), Options: opts}
}

func toCeilingDTO(c domain.RemoteCeiling) CeilingDTO {
	return CeilingDTO{
		Remote: c.Remote, Transfers: c.Transfers, Checkers: c.Checkers,
		Bwlimit: c.Bwlimit, Tpslimit: c.Tpslimit,
	}
}

func fromCeilingDTO(c CeilingDTO) domain.RemoteCeiling {
	return domain.RemoteCeiling{
		Remote: c.Remote, Transfers: c.Transfers, Checkers: c.Checkers,
		Bwlimit: c.Bwlimit, Tpslimit: c.Tpslimit,
	}
}
