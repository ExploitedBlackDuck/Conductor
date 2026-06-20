package options

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/conductor-app/conductor/internal/core/coreerr"
	"github.com/conductor-app/conductor/internal/core/domain"
)

// Selection is an operator's chosen options. Single holds scalar values keyed by
// flag (a bool option carries "true"/"false"); Multi holds list-option values.
type Selection struct {
	Single map[string]string
	Multi  map[string][]string
}

// Ceilings are governance caps the builder clamps to (§7.6). A zero/empty field
// means no ceiling for that dimension.
type Ceilings struct {
	Transfers int
	Checkers  int
	Bwlimit   string // rclone size, e.g. "10M"; "" means no cap
	Tpslimit  int
}

// EffectiveOption is a resolved selection entry for display and audit.
type EffectiveOption struct {
	Flag        string
	Value       string
	Risk        Risk
	AffectsData bool
}

// Clamp records a governance reduction applied by the builder (§7.6).
type Clamp struct {
	Flag      string
	Requested string
	Applied   string
	Reason    string
}

// Built is the validated, assembled command: the exact effective argv preview
// (§7.11.3), the rc parameters for execution, the resolved options, and any
// governance clamps applied.
type Built struct {
	Argv         []string
	ConfigParams map[string]any
	FilterParams map[string][]string
	Effective    []EffectiveOption
	Clamps       []Clamp
}

// Build validates the selection against the catalog for the given operation kind
// and assembles the effective command, clamping governed values to ceilings. It
// rejects unknown flags, type errors, kind mismatches, conflicts, and unmet
// requirements (ADR-0011); it never lets a free-text flag through.
func (c *Catalog) Build(sel Selection, kind domain.OperationKind, ceilings Ceilings) (Built, error) {
	enabled, err := c.resolveEnabled(sel, kind)
	if err != nil {
		return Built{}, err
	}
	if err := c.checkConflicts(enabled); err != nil {
		return Built{}, err
	}
	if err := c.checkRequires(enabled); err != nil {
		return Built{}, err
	}

	built := Built{
		ConfigParams: map[string]any{},
		FilterParams: map[string][]string{},
	}

	// Iterate in catalog order for a stable, previewable command.
	for i := range c.Options {
		opt := &c.Options[i]
		val, ok := enabled[opt.Flag]
		if !ok {
			continue
		}
		val = c.applyCeiling(opt, val, ceilings, &built)
		c.appendEffective(opt, val, &built)
		c.appendArgv(opt, val, sel, &built)
		c.appendRC(opt, val, sel, &built)
	}
	return built, nil
}

// resolveEnabled returns the map of enabled flag -> resolved scalar value,
// validating existence, kind applicability, and value types.
func (c *Catalog) resolveEnabled(sel Selection, kind domain.OperationKind) (map[string]string, error) {
	enabled := map[string]string{}

	for flag, raw := range sel.Single {
		opt, ok := c.Lookup(flag)
		if !ok {
			return nil, coreerr.New(coreerr.CodeOptionInvalid, "unknown option "+flag, nil)
		}
		if !opt.appliesTo(kind) {
			return nil, coreerr.New(coreerr.CodeOptionInvalid,
				fmt.Sprintf("option %s does not apply to a %s operation", flag, kind), nil)
		}
		if opt.Type == TypeList {
			return nil, coreerr.New(coreerr.CodeOptionInvalid, "option "+flag+" is a list option; provide list values", nil)
		}
		on, value, err := validateScalar(opt, raw)
		if err != nil {
			return nil, err
		}
		if on {
			enabled[flag] = value
		}
	}

	for flag, values := range sel.Multi {
		opt, ok := c.Lookup(flag)
		if !ok {
			return nil, coreerr.New(coreerr.CodeOptionInvalid, "unknown option "+flag, nil)
		}
		if !opt.appliesTo(kind) {
			return nil, coreerr.New(coreerr.CodeOptionInvalid,
				fmt.Sprintf("option %s does not apply to a %s operation", flag, kind), nil)
		}
		if opt.Type != TypeList {
			return nil, coreerr.New(coreerr.CodeOptionInvalid, "option "+flag+" is not a list option", nil)
		}
		nonEmpty := false
		for _, v := range values {
			if strings.TrimSpace(v) != "" {
				nonEmpty = true
				break
			}
		}
		if nonEmpty {
			enabled[flag] = "" // presence marker; values live in sel.Multi
		}
	}
	return enabled, nil
}

// validateScalar checks a scalar value against its type. It returns whether the
// option is enabled and the normalised value.
func validateScalar(opt *Option, raw string) (bool, string, error) {
	raw = strings.TrimSpace(raw)
	switch opt.Type {
	case TypeBool:
		switch raw {
		case "", "false", "0":
			return false, "", nil
		case "true", "1":
			return true, "true", nil
		default:
			return false, "", coreerr.New(coreerr.CodeOptionInvalid, "option "+opt.Flag+" expects a boolean", nil)
		}
	case TypeInt:
		if raw == "" {
			return false, "", nil
		}
		if _, err := strconv.Atoi(raw); err != nil {
			return false, "", coreerr.New(coreerr.CodeOptionInvalid, "option "+opt.Flag+" expects an integer", err)
		}
		return true, raw, nil
	case TypeSize:
		if raw == "" {
			return false, "", nil
		}
		if _, err := parseSize(raw); err != nil {
			return false, "", coreerr.New(coreerr.CodeOptionInvalid, "option "+opt.Flag+" expects a size like 10M or off", err)
		}
		return true, raw, nil
	case TypeDuration:
		if raw == "" {
			return false, "", nil
		}
		if !looksLikeDuration(raw) {
			return false, "", coreerr.New(coreerr.CodeOptionInvalid, "option "+opt.Flag+" expects a duration like 1s or 500ms", nil)
		}
		return true, raw, nil
	case TypeEnum:
		if raw == "" {
			return false, "", nil
		}
		for _, e := range opt.Enum {
			if raw == e {
				return true, raw, nil
			}
		}
		return false, "", coreerr.New(coreerr.CodeOptionInvalid,
			fmt.Sprintf("option %s expects one of %s", opt.Flag, strings.Join(opt.Enum, ", ")), nil)
	case TypeString:
		if raw == "" {
			return false, "", nil
		}
		return true, raw, nil
	default:
		return false, "", coreerr.New(coreerr.CodeOptionInvalid, "option "+opt.Flag+" has an unsupported type", nil)
	}
}

func (c *Catalog) checkConflicts(enabled map[string]string) error {
	for flag := range enabled {
		opt, _ := c.Lookup(flag)
		for _, other := range opt.ConflictsWith {
			if _, on := enabled[other]; on {
				return coreerr.New(coreerr.CodeOptionConflict,
					fmt.Sprintf("%s conflicts with %s; choose one", flag, other), nil)
			}
		}
	}
	return nil
}

func (c *Catalog) checkRequires(enabled map[string]string) error {
	for flag := range enabled {
		opt, _ := c.Lookup(flag)
		for _, req := range opt.Requires {
			if _, on := enabled[req]; !on {
				return coreerr.New(coreerr.CodeOptionInvalid,
					fmt.Sprintf("%s requires %s to also be set", flag, req), nil)
			}
		}
	}
	return nil
}

// applyCeiling clamps a governed value to its ceiling, recording any reduction.
func (c *Catalog) applyCeiling(opt *Option, val string, ceil Ceilings, built *Built) string {
	switch opt.Governed {
	case GovernedTransfers:
		return clampInt(opt.Flag, val, ceil.Transfers, built)
	case GovernedCheckers:
		return clampInt(opt.Flag, val, ceil.Checkers, built)
	case GovernedTpslimit:
		return clampInt(opt.Flag, val, ceil.Tpslimit, built)
	case GovernedBwlimit:
		return clampBwlimit(opt.Flag, val, ceil.Bwlimit, built)
	default:
		return val
	}
}

func clampInt(flag, val string, ceiling int, built *Built) string {
	if ceiling <= 0 {
		return val
	}
	n, err := strconv.Atoi(val)
	if err != nil || n <= ceiling {
		return val
	}
	applied := strconv.Itoa(ceiling)
	built.Clamps = append(built.Clamps, Clamp{
		Flag: flag, Requested: val, Applied: applied,
		Reason: fmt.Sprintf("clamped to governance ceiling %d", ceiling),
	})
	return applied
}

func clampBwlimit(flag, val, ceiling string, built *Built) string {
	if ceiling == "" {
		return val
	}
	ceilBytes, err := parseSize(ceiling)
	if err != nil {
		return val
	}
	valBytes, err := parseSize(val)
	// An unparseable or "off"/unlimited request is capped to the ceiling.
	if err != nil || valBytes < 0 || valBytes > ceilBytes {
		built.Clamps = append(built.Clamps, Clamp{
			Flag: flag, Requested: val, Applied: ceiling,
			Reason: "clamped to per-remote/global bandwidth ceiling " + ceiling,
		})
		return ceiling
	}
	return val
}

func (c *Catalog) appendEffective(opt *Option, val string, built *Built) {
	display := val
	switch opt.Type {
	case TypeBool:
		display = "true"
	case TypeList:
		display = "(list)"
	}
	built.Effective = append(built.Effective, EffectiveOption{
		Flag: opt.Flag, Value: display, Risk: opt.Risk, AffectsData: opt.AffectsData,
	})
}

func (c *Catalog) appendArgv(opt *Option, val string, sel Selection, built *Built) {
	switch opt.Type {
	case TypeBool:
		built.Argv = append(built.Argv, opt.Flag)
	case TypeList:
		for _, v := range sel.Multi[opt.Flag] {
			if strings.TrimSpace(v) == "" {
				continue
			}
			built.Argv = append(built.Argv, opt.Flag, v)
		}
	default:
		built.Argv = append(built.Argv, opt.Flag, val)
	}
}

func (c *Catalog) appendRC(opt *Option, val string, sel Selection, built *Built) {
	switch opt.RCSection {
	case RCConfig:
		built.ConfigParams[opt.RCParam] = rcValue(opt, val)
	case RCFilter:
		if opt.Type == TypeList {
			for _, v := range sel.Multi[opt.Flag] {
				if strings.TrimSpace(v) == "" {
					continue
				}
				built.FilterParams[opt.RCParam] = append(built.FilterParams[opt.RCParam], v)
			}
		} else {
			built.FilterParams[opt.RCParam] = append(built.FilterParams[opt.RCParam], val)
		}
	case RCNone:
		// argv-only (e.g. delete-timing, resync); handled by the run service.
	}
}

// rcValue converts a validated string value to the Go type the rc _config block
// expects.
func rcValue(opt *Option, val string) any {
	switch opt.Type {
	case TypeBool:
		return true
	case TypeInt:
		n, _ := strconv.Atoi(val)
		return n
	default:
		return val
	}
}

// appliesTo reports whether the option is valid for the given operation kind.
func (o *Option) appliesTo(kind domain.OperationKind) bool {
	if len(o.Kinds) == 0 {
		return true
	}
	for _, k := range o.Kinds {
		if domain.OperationKind(k) == kind {
			return true
		}
	}
	return false
}
