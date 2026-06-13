package mcp

import (
	"testing"

	"github.com/mshogin/archlint/internal/archlintcfg"
	"github.com/mshogin/archlint/internal/model"
)

// Golden layer-backedge (DR-0009 Уровень B). Слои по ПОРЯДКУ списка (верх->низ):
// presentation(#0) -> domain(#1) -> infra(#2). Разрешено вниз (верх зависит от низа);
// back-edge = ребро снизу вверх (infra->presentation) -> ERROR.

func layeredCfg() *archlintcfg.Config {
	return &archlintcfg.Config{
		Layers: []archlintcfg.LayerDef{
			{Name: "presentation", Paths: []string{"presentation"}},
			{Name: "domain", Paths: []string{"domain"}},
			{Name: "infra", Paths: []string{"infra"}},
		},
	}
}

func layeredGraph() *model.Graph {
	e := func(f, t string) model.Edge { return model.Edge{From: f, To: t, Type: "import"} }

	return &model.Graph{
		Nodes: []model.Node{
			{ID: "app/presentation/Handler"},
			{ID: "app/domain/Svc"},
			{ID: "app/infra/Repo"},
		},
		Edges: []model.Edge{
			e("app/presentation/Handler", "app/domain/Svc"), // вниз 0->1: ok
			e("app/domain/Svc", "app/infra/Repo"),           // вниз 1->2: ok
			e("app/infra/Repo", "app/presentation/Handler"), // back-edge 2->0: ERROR
		},
	}
}

func TestLayerBackedge_Detect(t *testing.T) {
	v := LayerBackedge(layeredGraph(), layeredCfg())
	if len(v) != 1 {
		t.Fatalf("ожидался 1 layer-backedge, got %d: %+v", len(v), v)
	}
	if v[0].Kind != "layer-backedge" || v[0].Target != "app/infra/Repo" {
		t.Fatalf("неверное нарушение: %+v", v[0])
	}

	c, ok := ClassOf("layer-backedge")
	if !ok || c.Class != "ERROR" {
		t.Fatalf("layer-backedge должен быть ERROR-class; got %+v ok=%v", c, ok)
	}
}

// Без layers-конфига -> детектор НЕАКТИВЕН.
func TestLayerBackedge_Inactive_NoLayers(t *testing.T) {
	if v := LayerBackedge(layeredGraph(), &archlintcfg.Config{}); len(v) != 0 {
		t.Fatalf("без layers детектор неактивен, ожидался 0, got %d", len(v))
	}
}

// Только разрешённые (вниз) рёбра -> 0 нарушений.
func TestLayerBackedge_ForwardOnly(t *testing.T) {
	e := func(f, t string) model.Edge { return model.Edge{From: f, To: t, Type: "import"} }
	g := &model.Graph{
		Nodes: []model.Node{{ID: "app/presentation/H"}, {ID: "app/infra/R"}},
		Edges: []model.Edge{e("app/presentation/H", "app/infra/R")}, // 0->2: вниз, ok
	}

	if v := LayerBackedge(g, layeredCfg()); len(v) != 0 {
		t.Fatalf("разрешённое ребро вниз -> 0, got %d: %+v", len(v), v)
	}
}

// Дельта-гейт: NEW back-edge -> Taboo; без baseline -> Telemetry.
func TestLayerBackedge_DeltaGate(t *testing.T) {
	v := LayerBackedge(layeredGraph(), layeredCfg())[0]
	acfg := &archlintcfg.Config{}

	if lvl := EffectiveLevel(v, acfg, nil); lvl != archlintcfg.LevelTelemetry {
		t.Errorf("без baseline -> Telemetry, got %v", lvl)
	}

	empty := &Baseline{Version: 1, Patterns: map[string][]string{}}
	if lvl := EffectiveLevel(v, acfg, empty); lvl != archlintcfg.LevelTaboo {
		t.Errorf("NEW vs baseline -> Taboo, got %v", lvl)
	}
}
