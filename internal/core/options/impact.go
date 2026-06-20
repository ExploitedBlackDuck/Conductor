package options

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/conductor-app/conductor/internal/core/domain"
)

// ImpactLevel is the severity of an impact finding (§7.5).
type ImpactLevel string

const (
	// ImpactWarn surfaces a caution; the run may proceed.
	ImpactWarn ImpactLevel = "warn"
	// ImpactRequireAck demands an explicit acknowledgement, recorded in the
	// audit log, before the run is allowed.
	ImpactRequireAck ImpactLevel = "require_ack"
	// ImpactBlock hard-stops the run.
	ImpactBlock ImpactLevel = "block"
)

// Impact is one finding from the impact engine.
type Impact struct {
	Level  ImpactLevel
	Flag   string // related flag, or "" for an operation-kind finding
	Title  string
	Detail string
}

// EvalInput is the full context the impact engine evaluates: the selection, the
// operation kind, the resolved endpoints, and the governance ceilings (§7.5).
type EvalInput struct {
	Selection Selection
	Kind      domain.OperationKind
	Src       domain.Endpoint
	Dst       domain.Endpoint
	Ceilings  Ceilings
}

// concurrency thresholds above which high-concurrency warnings fire.
const (
	transfersWarnAbove = 8
	checkersWarnAbove  = 16
)

// Evaluate runs the declarative impact rules over the input and returns all
// findings (§7.5). It is pure and order-stable, and is exercised by table tests.
func (c *Catalog) Evaluate(in EvalInput) []Impact {
	var impacts []Impact

	// Operation-kind findings: a sync makes the destination match the source and
	// so deletes extra files; delete and purge remove data outright (§7.4).
	switch in.Kind {
	case domain.KindSync:
		impacts = append(impacts, Impact{
			Level: ImpactRequireAck, Title: "Sync deletes extra files at the destination",
			Detail: "Sync makes the destination match the source; files at the destination not present at the source will be deleted. Run a dry-run first.",
		})
	case domain.KindDelete, domain.KindPurge:
		impacts = append(impacts, Impact{
			Level: ImpactRequireAck, Title: "This operation removes data",
			Detail: fmt.Sprintf("A %s deletes data at the destination. Confirm the resolved target before running.", in.Kind),
		})
	}

	// Per-option findings.
	for i := range c.Options {
		opt := &c.Options[i]
		if !c.isOn(in.Selection, opt) {
			continue
		}
		impacts = append(impacts, c.optionImpacts(opt, in)...)
	}

	// Bandwidth cap warning: a transfer-type operation with no bwlimit and no
	// governance ceiling can saturate the link (§7.5, ADR-0013).
	if isTransferKind(in.Kind) && !c.isFlagOn(in.Selection, "--bwlimit") && in.Ceilings.Bwlimit == "" {
		impacts = append(impacts, Impact{
			Level: ImpactWarn, Flag: "--bwlimit", Title: "No bandwidth cap set",
			Detail: "Without --bwlimit a transfer can saturate your connection. Set a limit or a per-remote ceiling.",
		})
	}

	return impacts
}

// optionImpacts returns the findings for a single enabled option.
func (c *Catalog) optionImpacts(opt *Option, in EvalInput) []Impact {
	var out []Impact

	// Any destructive option requires acknowledgement (delete-timing,
	// delete-excluded, bisync resync).
	if opt.Risk == RiskDestructive {
		out = append(out, Impact{
			Level: ImpactRequireAck, Flag: opt.Flag, Title: opt.Summary, Detail: opt.Description,
		})
	}

	switch opt.Flag {
	case "--no-check-dest", "--ignore-existing", "--size-only", "--no-traverse":
		out = append(out, Impact{
			Level: ImpactWarn, Flag: opt.Flag, Title: "Weakens change detection",
			Detail: opt.Description + " May skip or overwrite files unexpectedly.",
		})
	case "--include", "--exclude", "--max-size", "--min-size":
		out = append(out, Impact{
			Level: ImpactWarn, Flag: opt.Flag, Title: "Filter in effect",
			Detail: "Verify the filter matches the intended set of files; preview with a dry-run before running.",
		})
	case "--transfers":
		if n, err := strconv.Atoi(c.value(in.Selection, opt.Flag)); err == nil && n > transfersWarnAbove {
			out = append(out, Impact{
				Level: ImpactWarn, Flag: opt.Flag, Title: "High parallelism",
				Detail: fmt.Sprintf("--transfers %d is high; many object stores rate-limit or ban high concurrency.", n),
			})
		}
	case "--checkers":
		if n, err := strconv.Atoi(c.value(in.Selection, opt.Flag)); err == nil && n > checkersWarnAbove {
			out = append(out, Impact{
				Level: ImpactWarn, Flag: opt.Flag, Title: "High parallelism",
				Detail: fmt.Sprintf("--checkers %d is high; it may trip provider rate limits.", n),
			})
		}
	}
	return out
}

// isOn reports whether an option is enabled in the selection (truthy scalar or
// non-empty list).
func (c *Catalog) isOn(sel Selection, opt *Option) bool {
	if opt.Type == TypeList {
		for _, v := range sel.Multi[opt.Flag] {
			if strings.TrimSpace(v) != "" {
				return true
			}
		}
		return false
	}
	raw, ok := sel.Single[opt.Flag]
	if !ok {
		return false
	}
	raw = strings.TrimSpace(raw)
	if opt.Type == TypeBool {
		return raw == "true" || raw == "1"
	}
	return raw != "" && raw != "off"
}

// isFlagOn looks up an option by flag and reports whether it is enabled.
func (c *Catalog) isFlagOn(sel Selection, flag string) bool {
	opt, ok := c.Lookup(flag)
	if !ok {
		return false
	}
	return c.isOn(sel, opt)
}

// value returns the scalar value selected for a flag.
func (c *Catalog) value(sel Selection, flag string) string {
	return strings.TrimSpace(sel.Single[flag])
}

func isTransferKind(kind domain.OperationKind) bool {
	switch kind {
	case domain.KindCopy, domain.KindSync, domain.KindMove, domain.KindBisync:
		return true
	default:
		return false
	}
}
