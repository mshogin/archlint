package mcp

import (
	"fmt"
	"strings"
	"testing"

	"github.com/mshogin/archlint/internal/model"
)

// importGraph строит model.Graph только из import-рёбер (как видит detectCycles).
func importGraph(edges [][2]string) *model.Graph {
	g := &model.Graph{}
	seen := map[string]bool{}
	add := func(id string) {
		if !seen[id] {
			seen[id] = true
			g.Nodes = append(g.Nodes, model.Node{ID: id, Entity: "package"})
		}
	}
	for _, e := range edges {
		add(e[0])
		add(e[1])
		g.Edges = append(g.Edges, model.Edge{From: e[0], To: e[1], Type: "import"})
	}
	return g
}

// (1) Ацикличная цепочка — НЕ должно срабатывать (как archlint-на-себе, если ацикличен).
func TestDetectCycles_AcyclicChain(t *testing.T) {
	g := importGraph([][2]string{{"n0", "n1"}, {"n1", "n2"}, {"n2", "n3"}, {"n3", "n4"}})
	for _, n := range []string{"n0", "n1", "n2", "n3", "n4"} {
		if v := detectCycles(g, n); len(v) != 0 {
			t.Fatalf("ацикличный граф: узел %s ложно-флагнут: %v", n, v)
		}
	}
}

// (2) Цикл длины 12 — Tarjan SCC ловит. Старый simple_cycles(max_length=10) пропустил бы
// (демонстрация закрытия дыры полноты).
func TestDetectCycles_LongCycle12(t *testing.T) {
	var edges [][2]string
	for i := 0; i < 12; i++ {
		edges = append(edges, [2]string{fmt.Sprintf("n%d", i), fmt.Sprintf("n%d", (i+1)%12)})
	}
	g := importGraph(edges)
	v := detectCycles(g, "n0")
	if len(v) == 0 {
		t.Fatal("ДЫРА: цикл длины 12 НЕ пойман (старый max_length=10 пропустил бы — это и чиним)")
	}
	if !strings.Contains(v[0].Message, "SCC size 12") {
		t.Fatalf("ожидал SCC size 12 (вся петля), got: %s", v[0].Message)
	}
}

// (3) Ромбовый DAG A->B,A->C,B->D,C->D — 0 циклов, НЕ ложно-срабатывает.
func TestDetectCycles_DiamondNoCycle(t *testing.T) {
	g := importGraph([][2]string{{"A", "B"}, {"A", "C"}, {"B", "D"}, {"C", "D"}})
	for _, n := range []string{"A", "B", "C", "D"} {
		if v := detectCycles(g, n); len(v) != 0 {
			t.Fatalf("ромб DAG: узел %s ложно-флагнут (β₁=1, но 0 циклов): %v", n, v)
		}
	}
}

// (4) Петля X->X — цикл размера 1 через self-loop (SCC>1 его не ловит, отдельная ветка).
func TestDetectCycles_SelfLoop(t *testing.T) {
	g := importGraph([][2]string{{"X", "X"}})
	if v := detectCycles(g, "X"); len(v) == 0 {
		t.Fatal("self-loop X->X НЕ пойман")
	}
}

// (5) Два независимых цикла + ацикличный хвост — флагается только циклический.
func TestDetectCycles_MixedComponents(t *testing.T) {
	g := importGraph([][2]string{
		{"a", "b"}, {"b", "a"}, // цикл {a,b}
		{"c", "d"}, {"d", "c"}, // цикл {c,d}
		{"e", "f"}, // ацикличное ребро
	})
	for _, n := range []string{"a", "b", "c", "d"} {
		if v := detectCycles(g, n); len(v) == 0 {
			t.Fatalf("циклический узел %s НЕ пойман", n)
		}
	}
	for _, n := range []string{"e", "f"} {
		if v := detectCycles(g, n); len(v) != 0 {
			t.Fatalf("ацикличный узел %s ложно-флагнут: %v", n, v)
		}
	}
}
