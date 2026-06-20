// Package options implements ADR-0011: rclone options are a typed, data-driven
// catalog with a validating flag builder and a declarative impact-rule engine,
// not free-text flags. The catalog is the single source of truth for which
// options Conductor exposes and how they map to rc parameters and argv; the
// flag builder turns a validated selection into the exact effective command,
// and the impact engine explains and gates risky combinations.
package options

import (
	"fmt"
	"sort"
)

// ValueType is the data type of an option's value, driving input rendering and
// validation.
type ValueType string

// Value types.
const (
	TypeBool     ValueType = "bool"
	TypeInt      ValueType = "int"
	TypeSize     ValueType = "size"     // rclone size, e.g. "1M", "10Gi", "off"
	TypeDuration ValueType = "duration" // rclone duration, e.g. "1s", "500ms"
	TypeEnum     ValueType = "enum"
	TypeString   ValueType = "string"
	TypeList     ValueType = "list" // repeatable; value is a list of strings
)

// Risk classifies an option's effect on the data being moved (§7.5 risk badges):
// passive is read-only/inspection, mutating can change which data is written or
// weaken change detection, and destructive can delete or unrecoverably
// overwrite data.
type Risk string

// Risk values.
const (
	RiskPassive     Risk = "passive"
	RiskMutating    Risk = "mutating"
	RiskDestructive Risk = "destructive"
)

// Governed names the governance dimension an option participates in, so the
// flag builder can clamp it to a ceiling (§7.6). Empty means ungoverned.
type Governed string

// Governance dimensions.
const (
	GovernedNone      Governed = ""
	GovernedTransfers Governed = "transfers"
	GovernedCheckers  Governed = "checkers"
	GovernedBwlimit   Governed = "bwlimit"
	GovernedTpslimit  Governed = "tpslimit"
)

// RCSection is where an option's value goes in an rc request: the global
// _config block, the _filter block, or neither (handled specially, e.g.
// dry-run, or CLI-only).
type RCSection string

// rc request sections.
const (
	RCConfig RCSection = "config"
	RCFilter RCSection = "filter"
	RCNone   RCSection = ""
)

// Option is the typed metadata describing one exposed rclone option (ADR-0011).
type Option struct {
	Flag          string    `toml:"flag"`
	Aliases       []string  `toml:"aliases"`
	Type          ValueType `toml:"type"`
	Default       string    `toml:"default"`
	Category      string    `toml:"category"`
	Summary       string    `toml:"summary"`
	Description   string    `toml:"description"`
	Risk          Risk      `toml:"risk"`
	AffectsData   bool      `toml:"affects_data"`
	ConflictsWith []string  `toml:"conflicts_with"`
	Requires      []string  `toml:"requires"`
	Enum          []string  `toml:"enum"`
	// RCParam is the rc parameter key (e.g. "Transfers") for this option.
	RCParam string `toml:"rc_param"`
	// RCSection selects which rc block RCParam belongs to.
	RCSection RCSection `toml:"rc_section"`
	// Governed names the governance dimension for ceiling clamping (§7.6).
	Governed Governed `toml:"governed"`
	// Kinds optionally restricts the option to specific operation kinds (e.g.
	// resync is bisync-only). Empty means all kinds.
	Kinds []string `toml:"kinds"`
}

// Catalog is the versioned set of exposed options for one pinned rclone.
type Catalog struct {
	RcloneVersion string   `toml:"rclone_version"`
	Options       []Option `toml:"option"`

	byFlag map[string]*Option
}

// index builds the flag lookup, validating there are no duplicate flags.
func (c *Catalog) index() error {
	c.byFlag = make(map[string]*Option, len(c.Options))
	for i := range c.Options {
		opt := &c.Options[i]
		if opt.Flag == "" {
			return fmt.Errorf("catalog contains an option with no flag")
		}
		if _, dup := c.byFlag[opt.Flag]; dup {
			return fmt.Errorf("catalog contains duplicate flag %q", opt.Flag)
		}
		c.byFlag[opt.Flag] = opt
	}
	return nil
}

// Lookup returns the option for a flag, or false if the catalog does not expose
// it. Unknown flags cannot be injected (ADR-0011).
func (c *Catalog) Lookup(flag string) (*Option, bool) {
	opt, ok := c.byFlag[flag]
	return opt, ok
}

// Flags returns the exposed flag names, sorted.
func (c *Catalog) Flags() []string {
	flags := make([]string, 0, len(c.byFlag))
	for f := range c.byFlag {
		flags = append(flags, f)
	}
	sort.Strings(flags)
	return flags
}

// Categories returns the options grouped by category, for the UI builder
// (§7.11.5).
func (c *Catalog) Categories() map[string][]Option {
	out := map[string][]Option{}
	for _, opt := range c.Options {
		out[opt.Category] = append(out[opt.Category], opt)
	}
	return out
}
