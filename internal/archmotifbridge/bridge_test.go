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

// TestArchmotifProvider_NamedSignals — ComputeMetricsNamed выводит
// modularity+motif_redundancy как graph-метрики; modularity численно совпадает с
// archmotif-движком на 3-цикле p1->p2->p3->p1 (эталон -1/3). Сигналы, не гейт.
func TestArchmotifProvider_NamedSignals(t *testing.T) {
	pkg := func(id string) model.Node { return model.Node{ID: id, Title: id, Entity: model.EntityPackage} }
	imp := func(a, b string) model.Edge { return model.Edge{From: a, To: b, Type: model.EdgeImport} }
	g := &model.Graph{
		Nodes: []model.Node{pkg("p1"), pkg("p2"), pkg("p3")},
		Edges: []model.Edge{imp("p1", "p2"), imp("p2", "p3"), imp("p3", "p1")},
	}

	rep := archmotifbridge.Compute(g)
	if rep.Source != "archmotif" {
		t.Fatalf("provider=%s, want archmotif; notes=%v", rep.Source, rep.Notes)
	}
	if _, ok := rep.GraphMetrics["modularity"]; !ok {
		t.Fatalf("modularity not in GraphMetrics: %v", rep.GraphMetrics)
	}
	if _, ok := rep.GraphMetrics["motif_redundancy"]; !ok {
		t.Fatalf("motif_redundancy not in GraphMetrics: %v", rep.GraphMetrics)
	}
	// Эталон archmotif для одного 3-цикла: Q = -1/3.
	q := rep.GraphMetrics["modularity"]
	if q < -0.3334 || q > -0.3332 {
		t.Errorf("modularity Q=%.6f, want ~-0.3333 (archmotif reference)", q)
	}
}

// TestArchmotifProvider_NewMetrics — новые форк-метрики через bridge на
// triangle(p1,p2,p3)+isolated(p4). Эталон archmotif-движка: components=2, scc=2,
// radius(λ_max)=3 (триугольник), curvature(mean Forman)=3, eigenvalues(count)=4.
func TestArchmotifProvider_NewMetrics(t *testing.T) {
	pkg := func(id string) model.Node { return model.Node{ID: id, Title: id, Entity: model.EntityPackage} }
	imp := func(a, b string) model.Edge { return model.Edge{From: a, To: b, Type: model.EdgeImport} }
	g := &model.Graph{
		Nodes: []model.Node{pkg("p1"), pkg("p2"), pkg("p3"), pkg("p4")},
		Edges: []model.Edge{imp("p1", "p2"), imp("p2", "p3"), imp("p3", "p1"), imp("p1", "p3")},
	}

	gm := archmotifbridge.Compute(g).GraphMetrics
	checks := map[string]float64{"components": 2, "scc": 2, "radius": 3, "curvature": 3, "eigenvalues": 4}
	for name, want := range checks {
		got, ok := gm[name]
		if !ok {
			t.Errorf("%s not in GraphMetrics: %v", name, gm)
			continue
		}
		if got != want {
			t.Errorf("%s: got %v, want %v (archmotif reference)", name, got, want)
		}
	}
}
