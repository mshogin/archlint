// Package archlintcfg loads and parses the .archlint.yaml configuration file.
// The config schema mirrors the Rust archlint-rs config.rs so that both tools
// read the same .archlint.yaml file without modification.
package archlintcfg

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// Level controls how a rule violation is reported.
type Level string

const (
	// LevelTaboo blocks CI: exit code 1.
	LevelTaboo Level = "taboo"
	// LevelTelemetry tracks only: exit code 0, shown in yellow.
	LevelTelemetry Level = "telemetry"
	// LevelPersonal is informational: exit code 0.
	LevelPersonal Level = "personal"
)

// RuleConfig is the configuration for a single rule.
type RuleConfig struct {
	// Enabled controls whether the rule is active (default: true).
	// Using *bool so we can distinguish "omitted" (nil -> default true) from
	// "explicitly false" (false pointer). This mirrors serde's default=true in
	// archlint-rs and fixes the bug where "enabled: false" was silently ignored.
	Enabled *bool `yaml:"enabled"`
	// ErrorOnViolation controls whether a violation causes a non-zero exit code.
	ErrorOnViolation bool `yaml:"error_on_violation"`
	// Level is the metric level: taboo, telemetry, or personal.
	Level Level `yaml:"level"`
	// Threshold is the numeric threshold for this rule (e.g. max fan-out).
	// nil means "use default".
	Threshold *int `yaml:"threshold"`
	// Exclude is a list of component IDs (or glob patterns) excluded from this rule.
	Exclude []string `yaml:"exclude"`
}

// IsEnabled returns whether the rule is active.
// When the Enabled field is nil (omitted from config), the rule defaults to active.
func (r *RuleConfig) IsEnabled() bool {
	if r.Enabled == nil {
		return true
	}
	return *r.Enabled
}

// LayerDef defines a logical architectural layer.
type LayerDef struct {
	// Name is the human-readable layer identifier used in allowed_dependencies.
	Name string `yaml:"name"`
	// Paths lists path prefixes (relative to project root) belonging to this layer.
	Paths []string `yaml:"paths"`
}

// Rules holds configuration for all supported rules.
type Rules struct {
	FanOut       RuleConfig `yaml:"fan_out"`
	FanIn        RuleConfig `yaml:"fan_in"`
	Cycles       RuleConfig `yaml:"cycles"`
	ISP          RuleConfig `yaml:"isp"`
	DIP          RuleConfig `yaml:"dip"`
	FeatureEnvy  RuleConfig `yaml:"feature_envy"`
	GodClass     RuleConfig `yaml:"god_class"`
	HubNode      RuleConfig `yaml:"hub_node"`
	SRP          RuleConfig `yaml:"srp"`
}

// Config is the top-level .archlint.yaml configuration.
type Config struct {
	// Rules section, keyed by rule name.
	Rules Rules `yaml:"rules"`
	// Layers defines optional logical architectural layers.
	Layers []LayerDef `yaml:"layers"`
	// AllowedDependencies maps source layer name -> allowed target layer names.
	// Any cross-layer dependency not listed here is a violation.
	AllowedDependencies map[string][]string `yaml:"allowed_dependencies"`
}

// Default thresholds matching archlint-rs defaults.
const (
	DefaultFanOutThreshold = 5
	DefaultFanInThreshold  = 10
	DefaultISPThreshold    = 5
)

func boolPtr(v bool) *bool { return &v }

func defaultRuleConfig(threshold *int) RuleConfig {
	return RuleConfig{
		Enabled:          boolPtr(true),
		ErrorOnViolation: false,
		Level:            LevelTelemetry,
		Threshold:        threshold,
		Exclude:          nil,
	}
}

func intPtr(v int) *int { return &v }

// Default returns a Config with all default values (same as archlint-rs).
func Default() Config {
	return Config{
		Rules: Rules{
			FanOut:      defaultRuleConfig(intPtr(DefaultFanOutThreshold)),
			FanIn:       defaultRuleConfig(intPtr(DefaultFanInThreshold)),
			Cycles:      defaultRuleConfig(nil),
			ISP:         defaultRuleConfig(intPtr(DefaultISPThreshold)),
			DIP:         defaultRuleConfig(nil),
			FeatureEnvy: defaultRuleConfig(nil),
			GodClass:    defaultRuleConfig(nil),
			HubNode:     defaultRuleConfig(nil),
			SRP:         defaultRuleConfig(nil),
		},
	}
}

// Load reads .archlint.yaml from dir. Falls back to defaults if the file does
// not exist or cannot be parsed (matching archlint-rs behaviour).
func Load(dir string) Config {
	return LoadFile(filepath.Join(dir, ".archlint.yaml"))
}

// LoadFile reads the config from an explicit path. Falls back to defaults on
// any error.
func LoadFile(path string) Config {
	data, err := os.ReadFile(path) //nolint:gosec // user-provided path
	if err != nil {
		if !os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "Warning: could not read %s: %v\n", path, err)
		}
		return Default()
	}

	// Parse into a raw map first so we can apply per-field defaults for any
	// keys that were omitted from the YAML.
	var raw Config
	if err := yaml.Unmarshal(data, &raw); err != nil {
		fmt.Fprintf(os.Stderr, "Warning: could not parse %s: %v. Using defaults.\n", path, err)
		return Default()
	}

	def := Default()

	// Apply defaults for missing fields (mirrors archlint-rs fill-logic).
	applyRuleDefaults(&raw.Rules.FanOut, &def.Rules.FanOut)
	applyRuleDefaults(&raw.Rules.FanIn, &def.Rules.FanIn)
	applyRuleDefaults(&raw.Rules.Cycles, &def.Rules.Cycles)
	applyRuleDefaults(&raw.Rules.ISP, &def.Rules.ISP)
	applyRuleDefaults(&raw.Rules.DIP, &def.Rules.DIP)
	applyRuleDefaults(&raw.Rules.FeatureEnvy, &def.Rules.FeatureEnvy)
	applyRuleDefaults(&raw.Rules.GodClass, &def.Rules.GodClass)
	applyRuleDefaults(&raw.Rules.HubNode, &def.Rules.HubNode)
	applyRuleDefaults(&raw.Rules.SRP, &def.Rules.SRP)

	if raw.AllowedDependencies == nil {
		raw.AllowedDependencies = make(map[string][]string)
	}

	return raw
}

// applyRuleDefaults fills zero-value fields with defaults from def.
// With Enabled as *bool we can cleanly distinguish:
//   - nil:   not set by user -> apply default (true)
//   - true:  user explicitly enabled
//   - false: user explicitly disabled (the bug was here: previously treated
//            the same as nil because bool zero-value == false)
//
// For other fields (Level, Threshold) we still fill in the default when absent.
func applyRuleDefaults(r, def *RuleConfig) {
	// If the rule section was entirely absent from the YAML, all fields are
	// zero: Enabled=nil, Level="", Threshold=nil, Exclude=nil.
	// In that case apply the full default and return early.
	if r.Enabled == nil && r.Level == "" && r.Threshold == nil && len(r.Exclude) == 0 {
		*r = *def
		return
	}
	// Rule section was present (user specified at least one field).
	// Honour the user's Enabled value; for nil (omitted) default to true.
	if r.Enabled == nil {
		r.Enabled = def.Enabled
	}
	if r.Threshold == nil && def.Threshold != nil {
		r.Threshold = def.Threshold
	}
	if r.Level == "" {
		r.Level = def.Level
	}
}

// FanOutThreshold returns the effective fan-out threshold.
func (c *Config) FanOutThreshold() int {
	if c.Rules.FanOut.Threshold != nil {
		return *c.Rules.FanOut.Threshold
	}
	return DefaultFanOutThreshold
}

// FanInThreshold returns the effective fan-in threshold.
func (c *Config) FanInThreshold() int {
	if c.Rules.FanIn.Threshold != nil {
		return *c.Rules.FanIn.Threshold
	}
	return DefaultFanInThreshold
}

// ISPThreshold returns the effective ISP (interface size) threshold.
func (c *Config) ISPThreshold() int {
	if c.Rules.ISP.Threshold != nil {
		return *c.Rules.ISP.Threshold
	}
	return DefaultISPThreshold
}

// HasLayerRules returns true when both layers and allowed_dependencies are set.
func (c *Config) HasLayerRules() bool {
	return len(c.Layers) > 0 && len(c.AllowedDependencies) > 0
}

// LayerForModule resolves which layer name a module path belongs to.
// moduleID uses "/" or "::" as separator (both are normalised to "/").
// Returns empty string when no layer matches.
//
// Matching rules (in order):
//  1. Exact match: moduleID == layerPath
//  2. Prefix match: moduleID starts with layerPath+"/" (sub-package)
//  3. Suffix match: moduleID ends with "/"+layerPath (module prefix in ID)
//  4. Suffix sub-package: moduleID ends with "/"+layerPath+"/" component
//
// Rule 3 and 4 handle the common case where the analyzer produces package IDs
// that include the Go module name as a prefix, e.g. "mymodule/internal/handler"
// while the config path is just "internal/handler".
func (c *Config) LayerForModule(moduleID string) string {
	asPath := strings.ReplaceAll(moduleID, "::", "/")
	for _, layer := range c.Layers {
		for _, prefix := range layer.Paths {
			norm := strings.ReplaceAll(prefix, "::", "/")
			norm = strings.TrimRight(norm, "/")
			// Exact or prefix match (layerPath is a prefix of moduleID).
			if asPath == norm || strings.HasPrefix(asPath, norm+"/") {
				return layer.Name
			}
			// Suffix match: moduleID has a module-name prefix before layerPath.
			// e.g. moduleID="mymodule/internal/handler", norm="internal/handler"
			if strings.HasSuffix(asPath, "/"+norm) || strings.Contains(asPath, "/"+norm+"/") {
				return layer.Name
			}
		}
	}
	return ""
}

// IsExcluded returns true when target matches any entry in the exclude list.
// Entries are compared as exact strings (no glob expansion for now).
func isExcluded(exclude []string, target string) bool {
	for _, e := range exclude {
		if e == target {
			return true
		}
	}
	return false
}

// IsFanOutExcluded checks whether a package/component is excluded from fan-out checks.
func (c *Config) IsFanOutExcluded(target string) bool {
	return isExcluded(c.Rules.FanOut.Exclude, target)
}

// IsFanInExcluded checks whether a package/component is excluded from fan-in checks.
func (c *Config) IsFanInExcluded(target string) bool {
	return isExcluded(c.Rules.FanIn.Exclude, target)
}

// IsCyclesExcluded checks whether a package/component is excluded from cycle checks.
func (c *Config) IsCyclesExcluded(target string) bool {
	return isExcluded(c.Rules.Cycles.Exclude, target)
}

// IsFeatureEnvyExcluded checks whether a component is excluded from feature-envy checks.
func (c *Config) IsFeatureEnvyExcluded(target string) bool {
	return isExcluded(c.Rules.FeatureEnvy.Exclude, target)
}

// IsGodClassExcluded checks whether a component is excluded from god-class checks.
func (c *Config) IsGodClassExcluded(target string) bool {
	return isExcluded(c.Rules.GodClass.Exclude, target)
}

// IsHubNodeExcluded checks whether a component is excluded from hub-node checks.
func (c *Config) IsHubNodeExcluded(target string) bool {
	return isExcluded(c.Rules.HubNode.Exclude, target)
}

// IsSRPExcluded checks whether a component is excluded from SRP checks.
func (c *Config) IsSRPExcluded(target string) bool {
	return isExcluded(c.Rules.SRP.Exclude, target)
}
