package mcp

import (
	"testing"

	"github.com/mshogin/archlint/internal/archlintcfg"
	"github.com/mshogin/archlint/internal/model"
)

// Golden deprecated_usage (порт validate_deprecated_usage). Эталон сверен с реальным
// Python-валидатором: patterns=['deprecated'], ребро в deprecated-узел -> 1 нарушение.

func deprecatedFixture() *model.Graph {
	e := func(f, t string) model.Edge { return model.Edge{From: f, To: t, Type: "import"} }

	return &model.Graph{
		Nodes: []model.Node{{ID: "app.Svc"}, {ID: "app.DeprecatedRepo"}, {ID: "app.Other"}},
		Edges: []model.Edge{
			e("app.Svc", "app.DeprecatedRepo"), // использование deprecated
			e("app.Other", "app.Svc"),          // Svc не deprecated -> ok
		},
	}
}

func TestDeprecated_Pattern_vsPython(t *testing.T) {
	cfg := &archlintcfg.Config{DeprecatedPatterns: []string{"deprecated"}}

	v := DeprecatedUsage(deprecatedFixture(), cfg)
	if len(v) != 1 {
		t.Fatalf("ожидался 1 deprecated-usage, got %d: %+v", len(v), v)
	}
	if v[0].Kind != "deprecated-usage" || v[0].Target != "app.Svc" {
		t.Fatalf("неверное нарушение: %+v", v[0])
	}

	c, ok := ClassOf("deprecated-usage")
	if !ok || c.Class != "ERROR" {
		t.Fatalf("deprecated-usage должен быть ERROR-class; got %+v ok=%v", c, ok)
	}
}

// Маркер через атрибут узла `deprecated` (без паттернов в конфиге).
func TestDeprecated_AttrMarker(t *testing.T) {
	g := &model.Graph{
		Nodes: []model.Node{
			{ID: "app.Client"},
			{ID: "app.LegacyApi", Attrs: map[string]any{"deprecated": true}},
		},
		Edges: []model.Edge{{From: "app.Client", To: "app.LegacyApi", Type: "import"}},
	}

	if v := DeprecatedUsage(g, &archlintcfg.Config{}); len(v) != 1 {
		t.Fatalf("attr-маркер deprecated -> 1 нарушение, got %d: %+v", len(v), v)
	}
}

// Без явных маркеров (нет паттернов, нет атрибутов) -> детектор НЕАКТИВЕН.
func TestDeprecated_Inactive_NoMarkers(t *testing.T) {
	if v := DeprecatedUsage(deprecatedFixture(), &archlintcfg.Config{}); len(v) != 0 {
		t.Fatalf("без маркеров детектор неактивен, ожидался 0, got %d", len(v))
	}
}

// Дельта-гейт: NEW deprecated-usage -> Taboo; без baseline -> Telemetry.
func TestDeprecated_DeltaGate(t *testing.T) {
	cfg := &archlintcfg.Config{DeprecatedPatterns: []string{"deprecated"}}
	v := DeprecatedUsage(deprecatedFixture(), cfg)[0]
	acfg := &archlintcfg.Config{}

	if lvl := EffectiveLevel(v, acfg, nil); lvl != archlintcfg.LevelTelemetry {
		t.Errorf("без baseline -> Telemetry, got %v", lvl)
	}

	empty := &Baseline{Version: 1, Patterns: map[string][]string{}}
	if lvl := EffectiveLevel(v, acfg, empty); lvl != archlintcfg.LevelTaboo {
		t.Errorf("NEW vs baseline -> Taboo, got %v", lvl)
	}
}
