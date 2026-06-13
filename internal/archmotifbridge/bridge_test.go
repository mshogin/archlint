package archmotifbridge_test

import (
	"testing"

	"github.com/mshogin/archlint/internal/analyzer"
	"github.com/mshogin/archlint/internal/archmotifbridge"
	"github.com/mshogin/archlint/internal/model"
)

// fixtureGraph builds a 4-package archlint graph with two import-communities
// ({a,b} and {c,d}) joined by a single bridge import b->c.
func fixtureGraph() *model.Graph {
	pkg := func(id string) model.Node { return model.Node{ID: id, Title: id, Entity: model.EntityPackage} }
	imp := func(a, b string) model.Edge { return model.Edge{From: a, To: b, Type: model.EdgeImport} }
	return &model.Graph{
		Nodes: []model.Node{pkg("a"), pkg("b"), pkg("c"), pkg("d")},
		Edges: []model.Edge{
			imp("a", "b"), imp("b", "a"),
			imp("c", "d"), imp("d", "c"),
			imp("b", "c"),
		},
	}
}

func TestArchmotifProvider_Fixture(t *testing.T) {
	rep, err := archmotifbridge.ArchmotifProvider{}.Compute(fixtureGraph())
	if err != nil {
		t.Fatalf("archmotif Compute: %v", err)
	}
	if rep.Source != "archmotif" {
		t.Fatalf("source = %q, want archmotif", rep.Source)
	}
	if !rep.HasModularity {
		t.Fatalf("modularity not computed; notes=%v", rep.Notes)
	}
	t.Logf("archmotif: Q=%.4f graphMetrics=%v anomalies=%d notes=%v",
		rep.Modularity, rep.GraphMetrics, len(rep.Anomalies), rep.Notes)
}

func TestFallbackProvider_Fixture(t *testing.T) {
	rep, err := archmotifbridge.FallbackProvider{}.Compute(fixtureGraph())
	if err != nil {
		t.Fatalf("fallback Compute: %v", err)
	}
	if rep.Source != "fallback" || !rep.HasModularity {
		t.Fatalf("fallback bad report: %+v", rep)
	}
	// Two clear communities -> positive modularity.
	if rep.Modularity <= 0 {
		t.Fatalf("fallback Q=%.4f, want >0 for two communities", rep.Modularity)
	}
	t.Logf("fallback: Q=%.4f notes=%v", rep.Modularity, rep.Notes)
}

func TestCompute_NilGraphFallsBack(t *testing.T) {
	rep := archmotifbridge.Compute(nil)
	if rep.Source != "fallback" {
		t.Fatalf("nil graph should fall back, got source=%q", rep.Source)
	}
}

// TestCompute_RealGraph proves the end-to-end path on a REAL project graph:
// archlint analyzes its own internal/model package, then the bridge computes
// archmotif metrics over it.
func TestCompute_RealGraph(t *testing.T) {
	g, err := analyzer.NewGoAnalyzer().Analyze("../model")
	if err != nil {
		t.Fatalf("analyze real project: %v", err)
	}
	if len(g.Nodes) == 0 {
		t.Skip("real graph empty (analyzer produced no nodes)")
	}
	rep := archmotifbridge.Compute(g)
	t.Logf("REAL graph: nodes=%d edges=%d -> source=%s Q=%.4f hasMod=%v graphMetrics=%v anomalies=%d",
		len(g.Nodes), len(g.Edges), rep.Source, rep.Modularity, rep.HasModularity, rep.GraphMetrics, len(rep.Anomalies))
	for _, n := range rep.Notes {
		t.Logf("  note: %s", n)
	}
}
