package app

import (
	"github.com/conductor-app/conductor/internal/core/domain"
	"github.com/conductor-app/conductor/internal/core/options"
)

// categoryOrder is the order the option builder presents categories (§7.11.5).
var categoryOrder = []string{"performance", "checking", "deletion", "filters", "output", "bisync"}

// OptionDTO is one catalog option for the frontend builder.
type OptionDTO struct {
	Flag        string   `json:"flag"`
	Type        string   `json:"type"`
	Default     string   `json:"default"`
	Category    string   `json:"category"`
	Summary     string   `json:"summary"`
	Description string   `json:"description"`
	Risk        string   `json:"risk"`
	AffectsData bool     `json:"affectsData"`
	Enum        []string `json:"enum"`
	Governed    string   `json:"governed"`
	Kinds       []string `json:"kinds"`
}

// CategoryDTO groups options for display.
type CategoryDTO struct {
	Name    string      `json:"name"`
	Options []OptionDTO `json:"options"`
}

// CatalogDTO is the full catalog presented to the builder.
type CatalogDTO struct {
	RcloneVersion string        `json:"rcloneVersion"`
	Categories    []CategoryDTO `json:"categories"`
}

// GetCatalog returns the option catalog for the pinned rclone, grouped and
// ordered for the builder UI (§7.11.5).
func (a *App) GetCatalog() CatalogDTO {
	grouped := a.catalog.Categories()
	dto := CatalogDTO{RcloneVersion: a.catalog.RcloneVersion}

	seen := map[string]bool{}
	emit := func(name string, opts []options.Option) {
		cat := CategoryDTO{Name: name}
		for _, o := range opts {
			cat.Options = append(cat.Options, toOptionDTO(o))
		}
		dto.Categories = append(dto.Categories, cat)
	}
	for _, name := range categoryOrder {
		if opts, ok := grouped[name]; ok {
			emit(name, opts)
			seen[name] = true
		}
	}
	// Any categories not in the preferred order are appended.
	for name, opts := range grouped {
		if !seen[name] {
			emit(name, opts)
		}
	}
	return dto
}

func toOptionDTO(o options.Option) OptionDTO {
	return OptionDTO{
		Flag:        o.Flag,
		Type:        string(o.Type),
		Default:     o.Default,
		Category:    o.Category,
		Summary:     o.Summary,
		Description: o.Description,
		Risk:        string(o.Risk),
		AffectsData: o.AffectsData,
		// Emit [] rather than null for list fields: a nil Go slice marshals to
		// JSON null, and the frontend reads these as arrays (opt.kinds.length).
		// A null there crashed the option builder into a blank panel.
		Enum:     orEmpty(o.Enum),
		Governed: string(o.Governed),
		Kinds:    orEmpty(o.Kinds),
	}
}

// EndpointDTO is a remote+path endpoint from the frontend.
type EndpointDTO struct {
	Remote string `json:"remote"`
	Path   string `json:"path"`
}

// CeilingsDTO carries governance ceilings from the frontend.
type CeilingsDTO struct {
	Transfers int    `json:"transfers"`
	Checkers  int    `json:"checkers"`
	Bwlimit   string `json:"bwlimit"`
	Tpslimit  int    `json:"tpslimit"`
}

// PreviewRequest is a selection to validate, evaluate, and preview.
type PreviewRequest struct {
	Kind     string              `json:"kind"`
	Single   map[string]string   `json:"single"`
	Multi    map[string][]string `json:"multi"`
	Ceilings CeilingsDTO         `json:"ceilings"`
	Src      EndpointDTO         `json:"src"`
	Dst      EndpointDTO         `json:"dst"`
}

// ImpactDTO is one impact finding for the impact panel.
type ImpactDTO struct {
	Level  string `json:"level"`
	Flag   string `json:"flag"`
	Title  string `json:"title"`
	Detail string `json:"detail"`
}

// EffectiveDTO is a resolved option in the effective command.
type EffectiveDTO struct {
	Flag        string `json:"flag"`
	Value       string `json:"value"`
	Risk        string `json:"risk"`
	AffectsData bool   `json:"affectsData"`
}

// ClampDTO is a governance clamp applied to the selection.
type ClampDTO struct {
	Flag      string `json:"flag"`
	Requested string `json:"requested"`
	Applied   string `json:"applied"`
	Reason    string `json:"reason"`
}

// PreviewDTO is the resolved-operation preview the UI renders before any run
// (§7.11.3): the exact effective command, the impact findings, governance
// clamps, and the overall risk level. A build error (invalid selection) is
// surfaced as a typed error without an argv.
type PreviewDTO struct {
	Kind        string         `json:"kind"`
	ResolvedSrc string         `json:"resolvedSrc"`
	ResolvedDst string         `json:"resolvedDst"`
	Argv        []string       `json:"argv"`
	Command     string         `json:"command"`
	Effective   []EffectiveDTO `json:"effective"`
	Clamps      []ClampDTO     `json:"clamps"`
	Impacts     []ImpactDTO    `json:"impacts"`
	RiskLevel   string         `json:"riskLevel"`
	RequiresAck bool           `json:"requiresAck"`
	Error       *ErrorDTO      `json:"error"`
}

// PreviewOperation validates and previews a selection: it always returns the
// impact findings (the impact engine is lenient), and either the assembled
// effective command or a typed build error. It performs no execution.
func (a *App) PreviewOperation(req PreviewRequest) PreviewDTO {
	kind := domain.OperationKind(req.Kind)
	sel := options.Selection{Single: req.Single, Multi: req.Multi}
	ceilings := options.Ceilings{
		Transfers: req.Ceilings.Transfers,
		Checkers:  req.Ceilings.Checkers,
		Bwlimit:   req.Ceilings.Bwlimit,
		Tpslimit:  req.Ceilings.Tpslimit,
	}
	src := domain.Endpoint{Remote: req.Src.Remote, Path: req.Src.Path}
	dst := domain.Endpoint{Remote: req.Dst.Remote, Path: req.Dst.Path}

	impacts := a.catalog.Evaluate(options.EvalInput{
		Selection: sel, Kind: kind, Src: src, Dst: dst, Ceilings: ceilings,
	})

	out := PreviewDTO{
		Kind:        req.Kind,
		ResolvedSrc: src.String(),
		ResolvedDst: dst.String(),
		Impacts:     toImpactDTOs(impacts),
		RequiresAck: requiresAck(impacts),
	}

	built, err := a.catalog.Build(sel, kind, ceilings)
	if err != nil {
		out.Error = errorToDTO(err)
		out.RiskLevel = string(options.RiskPassive)
		return out
	}

	out.Argv = built.Argv
	out.Command = buildCommand(kind, src, dst, built.Argv)
	out.Effective = toEffectiveDTOs(built.Effective)
	out.Clamps = toClampDTOs(built.Clamps)
	out.RiskLevel = overallRisk(kind, built.Effective)
	return out
}

func buildCommand(kind domain.OperationKind, src, dst domain.Endpoint, argv []string) string {
	parts := []string{"rclone", string(kind)}
	if s := src.String(); s != "" {
		parts = append(parts, s)
	}
	if d := dst.String(); d != "" {
		parts = append(parts, d)
	}
	parts = append(parts, argv...)
	out := ""
	for i, p := range parts {
		if i > 0 {
			out += " "
		}
		out += p
	}
	return out
}

func overallRisk(kind domain.OperationKind, eff []options.EffectiveOption) string {
	level := options.RiskPassive
	rank := map[options.Risk]int{options.RiskPassive: 0, options.RiskMutating: 1, options.RiskDestructive: 2}
	if kind.IsDestructive() {
		level = options.RiskDestructive
	}
	for _, e := range eff {
		if rank[e.Risk] > rank[level] {
			level = e.Risk
		}
	}
	return string(level)
}

func requiresAck(impacts []options.Impact) bool {
	for _, im := range impacts {
		if im.Level == options.ImpactRequireAck || im.Level == options.ImpactBlock {
			return true
		}
	}
	return false
}

func toImpactDTOs(impacts []options.Impact) []ImpactDTO {
	out := make([]ImpactDTO, 0, len(impacts))
	for _, im := range impacts {
		out = append(out, ImpactDTO{Level: string(im.Level), Flag: im.Flag, Title: im.Title, Detail: im.Detail})
	}
	return out
}

func toEffectiveDTOs(eff []options.EffectiveOption) []EffectiveDTO {
	out := make([]EffectiveDTO, 0, len(eff))
	for _, e := range eff {
		out = append(out, EffectiveDTO{Flag: e.Flag, Value: e.Value, Risk: string(e.Risk), AffectsData: e.AffectsData})
	}
	return out
}

func toClampDTOs(clamps []options.Clamp) []ClampDTO {
	out := make([]ClampDTO, 0, len(clamps))
	for _, cl := range clamps {
		out = append(out, ClampDTO{Flag: cl.Flag, Requested: cl.Requested, Applied: cl.Applied, Reason: cl.Reason})
	}
	return out
}
