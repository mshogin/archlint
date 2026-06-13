package mcp

import (
	"testing"

	"github.com/mshogin/archlint/internal/model"
)

// fakeView — произвольный DirectedView НЕ из model.Graph: доказывает graph-agnostic
// потребление (метрика гоняется на любом виде, не зная конкретного типа).
type fakeView struct {
	nodes []string
	adj   map[string][]string
}

func (f fakeView) NodeIDs() []string             { return f.nodes }
func (f fakeView) Successors(id string) []string { return f.adj[id] }

// (1) computeSCC работает на ПРОИЗВОЛЬНОМ DirectedView (не model.Graph).
func TestComputeSCC_GraphAgnostic(t *testing.T) {
	v := fakeView{
		nodes: []string{"a", "b", "c"},
		adj:   map[string][]string{"a": {"b"}, "b": {"a"}}, // цикл a<->b, c отдельно
	}
	r := computeSCC(v)
	if !r.cyclic["a"] || !r.cyclic["b"] {
		t.Fatalf("a,b должны быть в цикле; %v", r.cyclic)
	}
	if r.cyclic["c"] {
		t.Fatal("c не в цикле")
	}
	if len(r.members["a"]) != 2 {
		t.Fatalf("SCC{a,b} размера 2; got %v", r.members["a"])
	}
}

// (2) Мемоизация (DR-0011): тот же граф -> ТОТ ЖЕ индекс (один расчёт на граф,
// не P раз). Проверяем равенство указателей *sccResult.
func TestCyclicSCC_Memoized(t *testing.T) {
	g := &model.Graph{Edges: []model.Edge{
		{From: "x", To: "y", Type: model.EdgeImport},
		{From: "y", To: "x", Type: model.EdgeImport},
	}}
	r1 := cyclicSCC(g)
	r2 := cyclicSCC(g)
	if r1 != r2 {
		t.Fatal("мемоизация: повторный вызов на том же графе должен вернуть тот же индекс")
	}
	if !r1.cyclic["x"] || !r1.cyclic["y"] {
		t.Fatalf("x,y в цикле; %v", r1.cyclic)
	}
}

// (3) Self-loop через DirectedView -> cyclic, members пусто (узел сам).
func TestComputeSCC_SelfLoop(t *testing.T) {
	v := fakeView{nodes: []string{"s"}, adj: map[string][]string{"s": {"s"}}}
	r := computeSCC(v)
	if !r.cyclic["s"] {
		t.Fatal("self-loop узел должен быть cyclic")
	}
	if len(r.members["s"]) != 0 {
		t.Fatalf("self-loop: members пусто (SCC размера 1); got %v", r.members["s"])
	}
}
