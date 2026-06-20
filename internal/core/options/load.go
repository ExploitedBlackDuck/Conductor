package options

import (
	"fmt"
	"io/fs"

	"github.com/BurntSushi/toml"

	"github.com/conductor-app/conductor/catalogs"
	"github.com/conductor-app/conductor/internal/core/rclonebin"
)

// Load parses and validates the embedded catalog for the pinned rclone version.
func Load() (*Catalog, error) {
	name := "rclone@" + rclonebin.PinnedVersion + ".toml"
	data, err := fs.ReadFile(catalogs.FS(), name)
	if err != nil {
		return nil, fmt.Errorf("reading catalog %s: %w", name, err)
	}

	var c Catalog
	if err := toml.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("parsing catalog %s: %w", name, err)
	}
	if err := c.index(); err != nil {
		return nil, fmt.Errorf("indexing catalog %s: %w", name, err)
	}
	if err := c.validate(); err != nil {
		return nil, fmt.Errorf("validating catalog %s: %w", name, err)
	}
	return &c, nil
}

var (
	validTypes = map[ValueType]bool{
		TypeBool: true, TypeInt: true, TypeSize: true, TypeDuration: true,
		TypeEnum: true, TypeString: true, TypeList: true,
	}
	validRisks    = map[Risk]bool{RiskPassive: true, RiskMutating: true, RiskDestructive: true}
	validSections = map[RCSection]bool{RCConfig: true, RCFilter: true, RCNone: true}
)

// validate enforces internal consistency: known enum values, enum options that
// list their choices, and conflicts_with/requires that reference real flags.
func (c *Catalog) validate() error {
	for _, opt := range c.Options {
		if !validTypes[opt.Type] {
			return fmt.Errorf("option %s has unknown type %q", opt.Flag, opt.Type)
		}
		if !validRisks[opt.Risk] {
			return fmt.Errorf("option %s has unknown risk %q", opt.Flag, opt.Risk)
		}
		if !validSections[opt.RCSection] {
			return fmt.Errorf("option %s has unknown rc_section %q", opt.Flag, opt.RCSection)
		}
		if opt.Type == TypeEnum && len(opt.Enum) == 0 {
			return fmt.Errorf("enum option %s lists no values", opt.Flag)
		}
		if opt.RCSection != RCNone && opt.RCParam == "" {
			return fmt.Errorf("option %s has rc_section %q but no rc_param", opt.Flag, opt.RCSection)
		}
		for _, ref := range opt.ConflictsWith {
			if _, ok := c.byFlag[ref]; !ok {
				return fmt.Errorf("option %s conflicts_with unknown flag %q", opt.Flag, ref)
			}
		}
		for _, ref := range opt.Requires {
			if _, ok := c.byFlag[ref]; !ok {
				return fmt.Errorf("option %s requires unknown flag %q", opt.Flag, ref)
			}
		}
	}
	return nil
}
