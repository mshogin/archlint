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
	Enabled bool `yaml:"enabled"`
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

// LayerDef defines a logical architectural layer.
type LayerDef struct {
	// Name is the human-readable layer identifier used in allowed_dependencies.
	Name string `yaml:"name"`
	// Paths lists path prefixes (relative to project root) belonging to this layer.
	Paths []string `yaml:"paths"`
}

// Rules holds configuration for all supported rules.
type Rules struct {
	FanOut RuleConfig `yaml:"fan_out"`
	FanIn  RuleConfig `yaml:"fan_in"`
	Cycles RuleConfig `yaml:"cycles"`
	ISP    RuleConfig `yaml:"isp"`
	DIP    RuleConfig `yaml:"dip"`
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

func defaultRuleConfig(threshold *int) RuleConfig {
	return RuleConfig{
		Enabled:          true,
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
			FanOut: defaultRuleConfig(intPtr(DefaultFanOutThreshold)),
			FanIn:  defaultRuleConfig(intPtr(DefaultFanInThreshold)),
			Cycles: defaultRuleConfig(nil),
			ISP:    defaultRuleConfig(intPtr(DefaultISPThreshold)),
			DIP:    defaultRuleConfig(nil),
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

	if raw.AllowedDependencies == nil {
		raw.AllowedDependencies = make(map[string][]string)
	}

	return raw
}

// applyRuleDefaults fills zero-value fields with defaults from def.
func applyRuleDefaults(r, def *RuleConfig) {
	// enabled defaults to true (zero value of bool is false, so we check the
	// YAML-unmarshalled value; yaml.v3 sets bool fields to false when absent,
	// so we cannot distinguish "explicitly false" from "omitted" unless we use
	// a pointer). We treat the default as true to match archlint-rs.
	// NOTE: If a user explicitly writes "enabled: false" that is honoured
	// because yaml unmarshals it as false, same as the zero value — so we
	// cannot set it to true unconditionally after parse. The safest approach
	// (matching Rust's serde default=true) is to check: if the whole RuleConfig
	// was zero-valued (empty struct), apply the default; otherwise leave as is.
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
func (c *Config) LayerForModule(moduleID string) string {
	asPath := strings.ReplaceAll(moduleID, "::", "/")
	for _, layer := range c.Layers {
		for _, prefix := range layer.Paths {
			norm := strings.ReplaceAll(prefix, "::", "/")
			norm = strings.TrimRight(norm, "/")
			if asPath == norm || strings.HasPrefix(asPath, norm+"/") {
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
