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
