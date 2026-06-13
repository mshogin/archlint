package mcp

import (
	"math"
	"testing"

	"github.com/mshogin/archlint/internal/model"
)

// Golden порта структурных дескрипторов Python(NetworkX)->Go (направление A).
// Эталонные числа сняты с networkx 3.5 на ТОМ ЖЕ графе (см. /tmp/refgraph_truth.py):
// узлы A-F, рёбра [A->B,A->C,B->C,C->A,C->D,D->E,E->D,F->A]. Числовое совпадение
// (epsilon 1e-9) = доказательство эквивалентности порта.

func refGraph() *model.Graph {
	n := func(id string) model.Node { return model.Node{ID: id, Entity: "package"} }
	e := func(f, t, typ string) model.Edge { return model.Edge{From: f, To: t, Type: typ} }

	return &model.Graph{
		Nodes: []model.Node{n("A"), n("B"), n("C"), n("D"), n("E"), n("F")},
		Edges: []model.Edge{
			e("A", "B", "calls"),
			e("A", "B", "uses"), // ПАРАЛЛЕЛЬНОЕ ребро -> должно схлопнуться (как nx.DiGraph)
			e("A", "C", "calls"),
			e("B", "C", "calls"),
			e("C", "A", "calls"),
			e("C", "D", "calls"),
			e("D", "E", "calls"),
			e("E", "D", "calls"),
			e("F", "A", "calls"),
		},
	}
}

func almost(t *testing.T, name string, got, want float64) {
	t.Helper()

	if math.Abs(got-want) > 1e-9 {
		t.Errorf("%s: got %.12f, want %.12f (Python)", name, got, want)
	}
}

func TestDescriptors_Counts_Density(t *testing.T) {
	d := ComputeDescriptors(refGraph())

	if d.NodeCount != 6 {
		t.Errorf("nodeCount: got %d, want 6", d.NodeCount)
	}
	// 9 рёбер в модели, но параллельное A->B схлопнуто -> 8 (nx).
	if d.EdgeCount != 8 {
		t.Errorf("edgeCount (dedup): got %d, want 8", d.EdgeCount)
	}

	almost(t, "density", d.Density, 0.26666666666666666)
}

func TestDescriptors_Coupling_Instability(t *testing.T) {
	d := ComputeDescriptors(refGraph())

	wantCa := map[string]int{"A": 2, "B": 1, "C": 2, "D": 2, "E": 1, "F": 0}
	wantCe := map[string]int{"A": 2, "B": 1, "C": 2, "D": 1, "E": 1, "F": 1}
	for id, w := range wantCa {
		if d.AfferentCoupling[id] != w {
			t.Errorf("Ca[%s]: got %d, want %d", id, d.AfferentCoupling[id], w)
		}
	}
	for id, w := range wantCe {
		if d.EfferentCoupling[id] != w {
			t.Errorf("Ce[%s]: got %d, want %d", id, d.EfferentCoupling[id], w)
		}
	}

	wantI := map[string]float64{
		"A": 0.5, "B": 0.5, "C": 0.5, "D": 0.3333333333333333, "E": 0.5, "F": 1.0,
	}
	for id, w := range wantI {
		almost(t, "instability["+id+"]", d.Instability[id], w)
	}
}

func TestDescriptors_PageRank_vsPython(t *testing.T) {
	d := ComputeDescriptors(refGraph())

	want := map[string]float64{
		"A": 0.098981555474, "B": 0.067067161103, "C": 0.124074248077,
		"D": 0.356690289416, "E": 0.328186745929, "F": 0.025,
	}
	for id, w := range want {
		almost(t, "pagerank["+id+"]", d.PageRank[id], w)
	}

	// nx.pagerank нормирован: sum = 1.
	var sum float64
	for _, v := range d.PageRank {
		sum += v
	}
	almost(t, "pagerank sum", sum, 1.0)
}

func TestDescriptors_Betweenness_vsPython(t *testing.T) {
	d := ComputeDescriptors(refGraph())

	want := map[string]float64{
		"A": 0.25, "B": 0.0, "C": 0.35, "D": 0.2, "E": 0.0, "F": 0.0,
	}
	for id, w := range want {
		almost(t, "betweenness["+id+"]", d.Betweenness[id], w)
	}
}

// --- БАТЧ 2: глобальные скаляры (эталон networkx на том же графе A-F) ---

func TestDescriptors_Batch2_vsPython(t *testing.T) {
	d := ComputeDescriptors(refGraph())

	if d.MaxPossibleEdges != 30 {
		t.Errorf("maxPossibleEdges: got %d, want 30", d.MaxPossibleEdges)
	}
	if d.MaxFanOut != 2 {
		t.Errorf("maxFanOut: got %d, want 2", d.MaxFanOut)
	}
	if d.IsDAG {
		t.Errorf("isDAG: got true, want false (граф A-F цикличен)")
	}
	if d.GraphDepth != -1 {
		t.Errorf("graphDepth: got %d, want -1 (циклы)", d.GraphDepth)
	}
	if d.Betti1 != 3 {
		t.Errorf("betti1: got %d, want 3 (E-V+C = 8-6+1)", d.Betti1)
	}

	almost(t, "dependencyEntropy", d.DependencyEntropy, 2.5)
	almost(t, "maxEntropy", d.MaxEntropy, 2.584962501)
	almost(t, "normalizedEntropy", d.NormalizedEntropy, 0.967132018)
	almost(t, "gini", d.Gini, 0.229166667)
	almost(t, "avgClustering", d.AvgClustering, 0.277777778)
}

// graph_depth на АЦИКЛИЧЕСКОМ графе: A->B->C->D -> depth=3 (рёбер в longest-path), isDAG.
func TestDescriptors_GraphDepth_DAG(t *testing.T) {
	n := func(id string) model.Node { return model.Node{ID: id, Entity: "package"} }
	e := func(f, t string) model.Edge { return model.Edge{From: f, To: t, Type: "calls"} }
	g := &model.Graph{
		Nodes: []model.Node{n("A"), n("B"), n("C"), n("D")},
		Edges: []model.Edge{e("A", "B"), e("B", "C"), e("C", "D")},
	}

	d := ComputeDescriptors(g)
	if !d.IsDAG {
		t.Errorf("isDAG: got false, want true")
	}
	if d.GraphDepth != 3 {
		t.Errorf("graphDepth: got %d, want 3", d.GraphDepth)
	}
}
