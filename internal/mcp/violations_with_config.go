package mcp

import (
	"fmt"
	"strings"

	"github.com/mshogin/archlint/internal/archlintcfg"
	"github.com/mshogin/archlint/internal/model"
)

// DetectAllViolationsWithConfig checks all packages for violations using thresholds
// and exclusions from .archlint.yaml. It is additive on top of DetectAllViolations:
// if config is nil or is the zero value, behaviour is identical to DetectAllViolations.
func DetectAllViolationsWithConfig(graph *model.Graph, cfg *archlintcfg.Config) []Violation {
	if cfg == nil {
		return DetectAllViolations(graph)
	}

	var violations []Violation

	packages := make(map[string]bool)
	for _, node := range graph.Nodes {
		if node.Entity == "package" {
			packages[node.ID] = true
		}
	}

	for pkgID := range packages {
		violations = append(violations, detectViolationsForPackageWithConfig(graph, pkgID, cfg)...)
	}

	// Layer dependency violations (only when layers are configured).
	if cfg.HasLayerRules() {
		violations = append(violations, detectLayerViolations(graph, cfg)...)
	}

	return violations
}

// detectViolationsForPackageWithConfig runs per-package checks with config thresholds.
func detectViolationsForPackageWithConfig(graph *model.Graph, pkgID string, cfg *archlintcfg.Config) []Violation {
	if pkgID == "" {
		return nil
	}

	var violations []Violation

	// --- Fan-out (efferent coupling) ---
	if cfg.Rules.FanOut.IsEnabled() && !cfg.IsFanOutExcluded(pkgID) {
		importCount := 0
		for _, edge := range graph.Edges {
			if edge.From == pkgID && edge.Type == "import" {
				importCount++
			}
		}
		threshold := cfg.FanOutThreshold()
		if importCount > threshold {
			violations = append(violations, Violation{
				Kind:    "high-efferent-coupling",
				Message: fmt.Sprintf("Package %s has %d dependencies (threshold: %d). Consider decomposition.", pkgID, importCount, threshold),
				Target:  pkgID,
			})
		}
	}

	// --- Cycles ---
	if cfg.Rules.Cycles.IsEnabled() && !cfg.IsCyclesExcluded(pkgID) {
		violations = append(violations, detectCycles(graph, pkgID)...)
	}

	return violations
}

// detectLayerViolations checks cross-layer dependency rules defined in .archlint.yaml.
func detectLayerViolations(graph *model.Graph, cfg *archlintcfg.Config) []Violation {
	var violations []Violation

	for _, edge := range graph.Edges {
		if edge.Type != "import" {
			continue
		}

		fromLayer := cfg.LayerForModule(edge.From)
		toLayer := cfg.LayerForModule(edge.To)

		// Only check dependencies when both ends have a known layer.
		if fromLayer == "" || toLayer == "" || fromLayer == toLayer {
			continue
		}

		allowed, exists := cfg.AllowedDependencies[fromLayer]
		if !exists {
			// No rule defined for fromLayer -> allow by default.
			continue
		}

		if !containsString(allowed, toLayer) {
			violations = append(violations, Violation{
				Kind:    "layer-violation",
				Message: fmt.Sprintf("Forbidden dependency: %s (%s) -> %s (%s)", edge.From, fromLayer, edge.To, toLayer),
				Target:  edge.From,
			})
		}
	}

	return violations
}

// ValidateGraphWithConfig applies config rules to an already-parsed graph export
// and returns any violations. Used by the validate command.
func ValidateGraphWithConfig(components []string, edges [][2]string, cfg *archlintcfg.Config) []Violation {
	if cfg == nil {
		return nil
	}

	var violations []Violation

	// Build a minimal model.Graph from the exported data for cycle detection.
	nodes := make([]model.Node, 0, len(components))
	for _, c := range components {
		nodes = append(nodes, model.Node{ID: c, Entity: "package"})
	}

	graphEdges := make([]model.Edge, 0, len(edges))
	for _, e := range edges {
		graphEdges = append(graphEdges, model.Edge{From: e[0], To: e[1], Type: "import"})
	}

	g := &model.Graph{Nodes: nodes, Edges: graphEdges}

	violations = append(violations, DetectAllViolationsWithConfig(g, cfg)...)

	return violations
}

func containsString(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

// levelPrefix returns a human-readable severity prefix for the given level.
func levelPrefix(level archlintcfg.Level) string {
	switch level {
	case archlintcfg.LevelTaboo:
		return "[ERROR]"
	case archlintcfg.LevelPersonal:
		return "[INFO]"
	default:
		return "[WARN]"
	}
}

// ViolationLevel returns the config level for a given violation kind.
// Used by scan to decide whether a violation should block CI.
func ViolationLevel(v Violation, cfg *archlintcfg.Config) archlintcfg.Level {
	if cfg == nil {
		return archlintcfg.LevelTelemetry
	}

	switch {
	case v.Kind == "high-efferent-coupling":
		return cfg.Rules.FanOut.Level
	case v.Kind == "circular-dependency":
		return cfg.Rules.Cycles.Level
	case v.Kind == "isp-fat-interface":
		return cfg.Rules.ISP.Level
	case v.Kind == "dip-concrete-dependency" || strings.HasPrefix(v.Kind, "dip-"):
		return cfg.Rules.DIP.Level
	case v.Kind == "layer-violation":
		return archlintcfg.LevelTaboo // layer violations always block
	default:
		return archlintcfg.LevelTelemetry
	}
}

// LevelPrefix is exported for use in CLI output.
func LevelPrefix(level archlintcfg.Level) string {
	return levelPrefix(level)
}
