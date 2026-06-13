package mcp

import (
	"testing"

	"github.com/mshogin/archlint/internal/archlintcfg"
	"github.com/mshogin/archlint/internal/model"
)

// Golden ghost_components (порт validate_ghost_components). Эталон сверен с реальным
// Python-валидатором: контекст с app/Real (в графе) + app/Ghost (нет) -> 1 ghost.

func ghostFixture() (*model.Graph, *archlintcfg.Config) {
	g := &model.Graph{Nodes: []model.Node{{ID: "app/Real"}, {ID: "app/Other"}}}
	cfg := &archlintcfg.Config{Contexts: []archlintcfg.ContextDef{
		{Name: "c1", Components: []string{"app/Real", "app/Ghost"}},
	}}

	return g, cfg
}

func TestGhost_Detect_vsPython(t *testing.T) {
	g, cfg := ghostFixture()

	v := GhostComponents(g, cfg)
	if len(v) != 1 {
		t.Fatalf("ожидался 1 ghost (app/Ghost), got %d: %+v", len(v), v)
	}
	if v[0].Kind != "ghost-component" || v[0].Target != "app/Ghost" {
		t.Fatalf("неверное нарушение: %+v", v[0])
	}

	c, ok := ClassOf("ghost-component")
	if !ok || c.Class != "ERROR" {
		t.Fatalf("ghost-component должен быть ERROR-class; got %+v ok=%v", c, ok)
	}
}

// Без контекстов -> детектор НЕАКТИВЕН (self-горнило: self=0 как forbidden).
func TestGhost_Inactive_NoContexts(t *testing.T) {
	g, _ := ghostFixture()
	if v := GhostComponents(g, &archlintcfg.Config{}); len(v) != 0 {
		t.Fatalf("без contexts неактивен, ожидался 0, got %d", len(v))
	}
}

// Все компоненты контекста есть в графе (fuzzy-матч) -> 0 ghost.
func TestGhost_AllPresent(t *testing.T) {
	g := &model.Graph{Nodes: []model.Node{{ID: "app/cart/Service"}, {ID: "app/pay/Gateway"}}}
	cfg := &archlintcfg.Config{Contexts: []archlintcfg.ContextDef{
		// "Service"/"Gateway" матчатся по нормализованному последнему сегменту.
		{Name: "c", Components: []string{"Service", "Gateway"}},
	}}

	if v := GhostComponents(g, cfg); len(v) != 0 {
		t.Fatalf("все компоненты матчатся -> 0 ghost, got %d: %+v", len(v), v)
	}
}

// Дельта-гейт: NEW ghost -> Taboo; без baseline -> Telemetry.
func TestGhost_DeltaGate(t *testing.T) {
	g, cfg := ghostFixture()
	v := GhostComponents(g, cfg)[0]
	acfg := &archlintcfg.Config{}

	if lvl := EffectiveLevel(v, acfg, nil); lvl != archlintcfg.LevelTelemetry {
		t.Errorf("без baseline -> Telemetry, got %v", lvl)
	}

	empty := &Baseline{Version: 1, Patterns: map[string][]string{}}
	if lvl := EffectiveLevel(v, acfg, empty); lvl != archlintcfg.LevelTaboo {
		t.Errorf("NEW ghost vs baseline -> Taboo, got %v", lvl)
	}
}

// Golden context_coverage (порт validate_context_coverage). Эталон vs Python:
// триугольник A->B->C->A (равный PageRank), контекст {B} -> coverage 0.333,
// covered=[B], uncovered=[A,C]. SEVERITY НЕ решён (горнило: intent-laden false-fire
// на легитимных uncovered-критических -> парный вердикт).
func TestCoverage_vsPython(t *testing.T) {
	e := func(f, t string) model.Edge { return model.Edge{From: f, To: t, Type: "import"} }
	g := &model.Graph{
		Nodes: []model.Node{{ID: "A"}, {ID: "B"}, {ID: "C"}},
		Edges: []model.Edge{e("A", "B"), e("B", "C"), e("C", "A")},
	}
	cfg := &archlintcfg.Config{Contexts: []archlintcfg.ContextDef{{Name: "c", Components: []string{"B"}}}}

	r := ComputeContextCoverage(g, cfg)
	if !r.Active {
		t.Fatal("ожидался active coverage")
	}
	if r.Coverage < 0.332 || r.Coverage > 0.334 {
		t.Errorf("coverage: got %.4f, want ~0.333 (Python)", r.Coverage)
	}
	if len(r.Covered) != 1 || r.Covered[0] != "B" {
		t.Errorf("covered: got %v, want [B]", r.Covered)
	}
	if len(r.Uncovered) != 2 {
		t.Errorf("uncovered: got %v, want [A C]", r.Uncovered)
	}
}

// Без контекстов -> неактивен.
func TestCoverage_Inactive(t *testing.T) {
	e := func(f, t string) model.Edge { return model.Edge{From: f, To: t, Type: "import"} }
	g := &model.Graph{Nodes: []model.Node{{ID: "A"}, {ID: "B"}}, Edges: []model.Edge{e("A", "B")}}
	if r := ComputeContextCoverage(g, &archlintcfg.Config{}); r.Active {
		t.Errorf("без contexts -> неактивен, got %+v", r)
	}
}
