package mcp

import (
	"testing"

	"github.com/mshogin/archlint/internal/archlintcfg"
	"github.com/mshogin/archlint/internal/model"
)

// Golden forbidden_dependencies (порт validate_forbidden_dependencies). Эталон
// сверен с реальным Python-валидатором: rule handler->repository, граф с одним
// запрещённым ребром -> 1 нарушение, status FAILED.

func forbiddenFixture() *model.Graph {
	e := func(f, t string) model.Edge { return model.Edge{From: f, To: t, Type: "import"} }

	return &model.Graph{
		Nodes: []model.Node{
			{ID: "app.handler.UserHandler"}, {ID: "app.repository.UserRepo"},
			{ID: "app.service.Svc"}, {ID: "app.repository.Repo"},
		},
		Edges: []model.Edge{
			e("app.handler.UserHandler", "app.repository.UserRepo"), // запрещено
			e("app.service.Svc", "app.repository.Repo"),             // service != handler -> ok
		},
	}
}

func TestForbidden_Match_vsPython(t *testing.T) {
	cfg := &archlintcfg.Config{Forbidden: []archlintcfg.ForbiddenRule{{From: "handler", To: "repository"}}}

	v := ForbiddenDependencies(forbiddenFixture(), cfg)
	if len(v) != 1 {
		t.Fatalf("ожидался 1 forbidden-dependency, got %d: %+v", len(v), v)
	}
	if v[0].Kind != "forbidden-dependency" || v[0].Target != "app.handler.UserHandler" {
		t.Fatalf("неверное нарушение: %+v", v[0])
	}

	// ERROR-класс: проходит через severity_class + дельта-гейт.
	c, ok := ClassOf("forbidden-dependency")
	if !ok || c.Class != "ERROR" {
		t.Fatalf("forbidden-dependency должен быть ERROR-class; got %+v ok=%v", c, ok)
	}
}

// Без правил -> детектор НЕАКТИВЕН (0 нарушений).
func TestForbidden_Inactive_NoConfig(t *testing.T) {
	if v := ForbiddenDependencies(forbiddenFixture(), &archlintcfg.Config{}); len(v) != 0 {
		t.Fatalf("без forbidden-правил детектор неактивен, ожидался 0, got %d", len(v))
	}
}

// Дельта-гейт: NEW forbidden -> Taboo (блок); без baseline -> Telemetry (аудит).
func TestForbidden_DeltaGate(t *testing.T) {
	cfg := &archlintcfg.Config{Forbidden: []archlintcfg.ForbiddenRule{{From: "handler", To: "repository"}}}
	v := ForbiddenDependencies(forbiddenFixture(), cfg)[0]

	acfg := &archlintcfg.Config{}
	if lvl := EffectiveLevel(v, acfg, nil); lvl != archlintcfg.LevelTelemetry {
		t.Errorf("без baseline forbidden -> Telemetry (audit), got %v", lvl)
	}

	empty := &Baseline{Version: 1, Patterns: map[string][]string{}}
	if lvl := EffectiveLevel(v, acfg, empty); lvl != archlintcfg.LevelTaboo {
		t.Errorf("NEW forbidden vs baseline -> Taboo (block), got %v", lvl)
	}
}
