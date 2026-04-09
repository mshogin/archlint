package archlintcfg_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mshogin/archlint/internal/archlintcfg"
)

func writeConfig(t *testing.T, dir, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, ".archlint.yaml"), []byte(content), 0o600); err != nil {
		t.Fatalf("writeConfig: %v", err)
	}
}

func TestDefaultsWhenNoFile(t *testing.T) {
	dir := t.TempDir()
	cfg := archlintcfg.Load(dir)
	if cfg.FanOutThreshold() != 5 {
		t.Errorf("FanOutThreshold: got %d, want 5", cfg.FanOutThreshold())
	}
	if cfg.FanInThreshold() != 10 {
		t.Errorf("FanInThreshold: got %d, want 10", cfg.FanInThreshold())
	}
	if cfg.ISPThreshold() != 5 {
		t.Errorf("ISPThreshold: got %d, want 5", cfg.ISPThreshold())
	}
	if cfg.Rules.FanOut.Level != archlintcfg.LevelTelemetry {
		t.Errorf("FanOut.Level: got %q, want %q", cfg.Rules.FanOut.Level, archlintcfg.LevelTelemetry)
	}
}

func TestCustomThresholds(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
rules:
  fan_out:
    threshold: 3
  fan_in:
    threshold: 7
`)
	cfg := archlintcfg.Load(dir)
	if cfg.FanOutThreshold() != 3 {
		t.Errorf("FanOutThreshold: got %d, want 3", cfg.FanOutThreshold())
	}
	if cfg.FanInThreshold() != 7 {
		t.Errorf("FanInThreshold: got %d, want 7", cfg.FanInThreshold())
	}
}

func TestRuleDisabled(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
rules:
  fan_out:
    enabled: false
    threshold: 3
  fan_in:
    enabled: true
    threshold: 8
`)
	cfg := archlintcfg.Load(dir)
	if cfg.Rules.FanOut.IsEnabled() {
		t.Error("FanOut should be disabled")
	}
	if !cfg.Rules.FanIn.IsEnabled() {
		t.Error("FanIn should be enabled")
	}
	if cfg.FanInThreshold() != 8 {
		t.Errorf("FanInThreshold: got %d, want 8", cfg.FanInThreshold())
	}
}

func TestExcludeList(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
rules:
  fan_out:
    threshold: 5
    exclude:
      - main
      - lib::utils
`)
	cfg := archlintcfg.Load(dir)
	if len(cfg.Rules.FanOut.Exclude) != 2 {
		t.Errorf("exclude length: got %d, want 2", len(cfg.Rules.FanOut.Exclude))
	}
	if cfg.IsFanOutExcluded("main") == false {
		t.Error("main should be excluded from fan_out")
	}
	if cfg.IsFanOutExcluded("lib::utils") == false {
		t.Error("lib::utils should be excluded from fan_out")
	}
}

// TestRuleDisabledOnlyFlag checks that "enabled: false" alone (without threshold or
// level) is honoured. This was the core bug: the old heuristic treated such a struct
// as "omitted" and silently applied the default (enabled: true).
func TestRuleDisabledOnlyFlag(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
rules:
  feature_envy:
    enabled: false
  god_class:
    enabled: false
  srp:
    enabled: false
  hub_node:
    enabled: false
  cycles:
    enabled: false
  dip:
    enabled: false
`)
	cfg := archlintcfg.Load(dir)
	if cfg.Rules.FeatureEnvy.IsEnabled() {
		t.Error("feature_envy should be disabled")
	}
	if cfg.Rules.GodClass.IsEnabled() {
		t.Error("god_class should be disabled")
	}
	if cfg.Rules.SRP.IsEnabled() {
		t.Error("srp should be disabled")
	}
	if cfg.Rules.HubNode.IsEnabled() {
		t.Error("hub_node should be disabled")
	}
	if cfg.Rules.Cycles.IsEnabled() {
		t.Error("cycles should be disabled")
	}
	if cfg.Rules.DIP.IsEnabled() {
		t.Error("dip should be disabled")
	}
	// Rules not mentioned should default to enabled.
	if !cfg.Rules.FanOut.IsEnabled() {
		t.Error("fan_out should default to enabled when not mentioned")
	}
	if !cfg.Rules.FanIn.IsEnabled() {
		t.Error("fan_in should default to enabled when not mentioned")
	}
}

func TestFallbackOnInvalidYAML(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, "this: [is: not: valid: yaml")
	cfg := archlintcfg.Load(dir)
	// Should return defaults without panicking.
	if cfg.FanOutThreshold() != 5 {
		t.Errorf("FanOutThreshold after invalid yaml: got %d, want 5", cfg.FanOutThreshold())
	}
}

func TestLayerConfigParsed(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
layers:
  - name: handler
    paths: ["internal/handler", "src/handler"]
  - name: service
    paths: ["internal/service", "src/service"]
  - name: repo
    paths: ["internal/repo"]
  - name: model
    paths: ["internal/model"]

allowed_dependencies:
  handler: [service, model]
  service: [repo, model]
  repo: [model]
  model: []
`)
	cfg := archlintcfg.Load(dir)
	if len(cfg.Layers) != 4 {
		t.Errorf("layers: got %d, want 4", len(cfg.Layers))
	}
	if !cfg.HasLayerRules() {
		t.Error("HasLayerRules should be true")
	}

	cases := []struct{ id, want string }{
		{"internal/handler/users", "handler"},
		{"src/service/orders", "service"},
		{"internal/repo/pg", "repo"},
		{"internal/model/user", "model"},
		{"pkg/utils", ""},
	}
	for _, tc := range cases {
		got := cfg.LayerForModule(tc.id)
		if got != tc.want {
			t.Errorf("LayerForModule(%q): got %q, want %q", tc.id, got, tc.want)
		}
	}

	allowed := cfg.AllowedDependencies["handler"]
	if len(allowed) != 2 {
		t.Errorf("handler allowed: got %v", allowed)
	}
}

func TestNoLayersHasLayerRulesFalse(t *testing.T) {
	dir := t.TempDir()
	cfg := archlintcfg.Load(dir)
	if cfg.HasLayerRules() {
		t.Error("HasLayerRules should be false with no config")
	}
}

func TestLevelParsed(t *testing.T) {
	dir := t.TempDir()
	writeConfig(t, dir, `
rules:
  fan_out:
    threshold: 5
    level: taboo
  cycles:
    level: personal
  isp:
    threshold: 5
    level: telemetry
`)
	cfg := archlintcfg.Load(dir)
	if cfg.Rules.FanOut.Level != archlintcfg.LevelTaboo {
		t.Errorf("fan_out.level: got %q, want taboo", cfg.Rules.FanOut.Level)
	}
	if cfg.Rules.Cycles.Level != archlintcfg.LevelPersonal {
		t.Errorf("cycles.level: got %q, want personal", cfg.Rules.Cycles.Level)
	}
	if cfg.Rules.ISP.Level != archlintcfg.LevelTelemetry {
		t.Errorf("isp.level: got %q, want telemetry", cfg.Rules.ISP.Level)
	}
	// fan_in not specified -> default telemetry
	if cfg.Rules.FanIn.Level != archlintcfg.LevelTelemetry {
		t.Errorf("fan_in.level: got %q, want telemetry", cfg.Rules.FanIn.Level)
	}
}

func TestLoadFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "custom.yaml")
	if err := os.WriteFile(path, []byte("rules:\n  fan_out:\n    threshold: 99\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	cfg := archlintcfg.LoadFile(path)
	if cfg.FanOutThreshold() != 99 {
		t.Errorf("FanOutThreshold: got %d, want 99", cfg.FanOutThreshold())
	}
}

func TestLoadFileMissing(t *testing.T) {
	cfg := archlintcfg.LoadFile("/nonexistent/.archlint.yaml")
	// Should return defaults.
	if cfg.FanOutThreshold() != 5 {
		t.Errorf("FanOutThreshold: got %d, want 5", cfg.FanOutThreshold())
	}
}
