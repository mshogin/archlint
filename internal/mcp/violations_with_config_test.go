package mcp

import (
	"testing"

	"github.com/mshogin/archlint/internal/archlintcfg"
	"github.com/mshogin/archlint/internal/model"
)

// buildLayerConfig is a helper that returns a config with a standard 3-layer setup.
func buildLayerConfig(layerPaths map[string]string, allowed map[string][]string) *archlintcfg.Config {
	layers := make([]archlintcfg.LayerDef, 0, len(layerPaths))
	for name, path := range layerPaths {
		layers = append(layers, archlintcfg.LayerDef{Name: name, Paths: []string{path}})
	}
	return &archlintcfg.Config{
		Layers:              layers,
		AllowedDependencies: allowed,
	}
}

// TestDetectLayerViolations_WithModulePrefix verifies that layer violations are
// detected when package IDs include a Go module name prefix (e.g.
// "mymodule/internal/handler") while the config paths are relative (e.g.
// "internal/handler"). This is the canonical output of the GoAnalyzer.
func TestDetectLayerViolations_WithModulePrefix(t *testing.T) {
	cfg := buildLayerConfig(
		map[string]string{
			"handler":    "internal/handler",
			"service":    "internal/service",
			"repository": "internal/repo",
			"model":      "internal/model",
		},
		map[string][]string{
			"handler":    {"service", "model"},
			"service":    {"repository", "model"},
			"repository": {"model"},
		},
	)

	// Simulate module-prefixed package IDs (as produced by GoAnalyzer).
	nodes := []model.Node{
		{ID: "mymodule/internal/handler", Entity: "package"},
		{ID: "mymodule/internal/service", Entity: "package"},
		{ID: "mymodule/internal/repo", Entity: "package"},
		{ID: "mymodule/internal/model", Entity: "package"},
	}

	t.Run("forbidden: handler -> repo (must go through service)", func(t *testing.T) {
		edges := []model.Edge{
			{From: "mymodule/internal/handler", To: "mymodule/internal/repo", Type: "import"},
		}
		graph := &model.Graph{Nodes: nodes, Edges: edges}
		violations := DetectAllViolationsWithConfig(graph, cfg)

		var layerViolations []Violation
		for _, v := range violations {
			if v.Kind == "layer-violation" {
				layerViolations = append(layerViolations, v)
			}
		}

		if len(layerViolations) != 1 {
			t.Fatalf("expected 1 layer-violation, got %d: %+v", len(layerViolations), violations)
		}
		v := layerViolations[0]
		if v.Target != "mymodule/internal/handler" {
			t.Errorf("violation target: want %q, got %q", "mymodule/internal/handler", v.Target)
		}
	})

	t.Run("allowed: handler -> service", func(t *testing.T) {
		edges := []model.Edge{
			{From: "mymodule/internal/handler", To: "mymodule/internal/service", Type: "import"},
		}
		graph := &model.Graph{Nodes: nodes, Edges: edges}
		violations := DetectAllViolationsWithConfig(graph, cfg)

		for _, v := range violations {
			if v.Kind == "layer-violation" {
				t.Errorf("unexpected layer-violation: %+v", v)
			}
		}
	})

	t.Run("allowed: handler -> model", func(t *testing.T) {
		edges := []model.Edge{
			{From: "mymodule/internal/handler", To: "mymodule/internal/model", Type: "import"},
		}
		graph := &model.Graph{Nodes: nodes, Edges: edges}
		violations := DetectAllViolationsWithConfig(graph, cfg)

		for _, v := range violations {
			if v.Kind == "layer-violation" {
				t.Errorf("unexpected layer-violation: %+v", v)
			}
		}
	})

	t.Run("allowed: service -> repo", func(t *testing.T) {
		edges := []model.Edge{
			{From: "mymodule/internal/service", To: "mymodule/internal/repo", Type: "import"},
		}
		graph := &model.Graph{Nodes: nodes, Edges: edges}
		violations := DetectAllViolationsWithConfig(graph, cfg)

		for _, v := range violations {
			if v.Kind == "layer-violation" {
				t.Errorf("unexpected layer-violation: %+v", v)
			}
		}
	})

	t.Run("forbidden: service -> handler (reverse dependency)", func(t *testing.T) {
		edges := []model.Edge{
			{From: "mymodule/internal/service", To: "mymodule/internal/handler", Type: "import"},
		}
		graph := &model.Graph{Nodes: nodes, Edges: edges}
		violations := DetectAllViolationsWithConfig(graph, cfg)

		var layerViolations []Violation
		for _, v := range violations {
			if v.Kind == "layer-violation" {
				layerViolations = append(layerViolations, v)
			}
		}
		if len(layerViolations) != 1 {
			t.Fatalf("expected 1 layer-violation (service->handler), got %d: %+v", len(layerViolations), violations)
		}
	})
}

// TestDetectLayerViolations_ExactPaths checks that the original exact/prefix
// matching (without module prefix) still works after the suffix-match fix.
func TestDetectLayerViolations_ExactPaths(t *testing.T) {
	cfg := buildLayerConfig(
		map[string]string{
			"handler": "internal/handler",
			"service": "internal/service",
			"model":   "internal/model",
		},
		map[string][]string{
			"handler": {"service", "model"},
			"service": {"model"},
		},
	)

	nodes := []model.Node{
		{ID: "internal/handler", Entity: "package"},
		{ID: "internal/service", Entity: "package"},
		{ID: "internal/model", Entity: "package"},
	}

	t.Run("forbidden: handler -> (no service path)", func(t *testing.T) {
		// handler is only allowed to depend on service and model.
		// service is allowed to depend on model only.
		// Here: service -> handler is forbidden.
		edges := []model.Edge{
			{From: "internal/service", To: "internal/handler", Type: "import"},
		}
		graph := &model.Graph{Nodes: nodes, Edges: edges}
		violations := DetectAllViolationsWithConfig(graph, cfg)

		var layerViolations []Violation
		for _, v := range violations {
			if v.Kind == "layer-violation" {
				layerViolations = append(layerViolations, v)
			}
		}
		if len(layerViolations) != 1 {
			t.Fatalf("expected 1 layer-violation, got %d: %+v", len(layerViolations), violations)
		}
	})

	t.Run("allowed: handler -> service (exact paths)", func(t *testing.T) {
		edges := []model.Edge{
			{From: "internal/handler", To: "internal/service", Type: "import"},
		}
		graph := &model.Graph{Nodes: nodes, Edges: edges}
		violations := DetectAllViolationsWithConfig(graph, cfg)

		for _, v := range violations {
			if v.Kind == "layer-violation" {
				t.Errorf("unexpected layer-violation: %+v", v)
			}
		}
	})
}
