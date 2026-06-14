package mcp

import (
	"math"
	"testing"

	"github.com/mshogin/archlint/internal/model"
)

// Golden порта структурных дескрипторов Python(NetworkX)->Go (направление A).
// Эталонные числа сняты с networkx 3.5 на ТОМ ЖЕ графе:
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

// --- БАТЧ 3: связностно-дистанционная группа (эталон networkx на A-F) ---

func TestDescriptors_Batch3_vsPython(t *testing.T) {
	d := ComputeDescriptors(refGraph())

	closeness := map[string]float64{
		"A": 0.45, "B": 0.36, "C": 0.45, "D": 0.555555556, "E": 0.384615385, "F": 0.0,
	}
	for id, w := range closeness {
		almost(t, "closeness["+id+"]", d.Closeness[id], w)
	}

	harmonic := map[string]float64{
		"A": 2.5, "B": 2.0, "C": 2.5, "D": 3.333333333, "E": 2.416666667, "F": 0.0,
	}
	for id, w := range harmonic {
		almost(t, "harmonic["+id+"]", d.Harmonic[id], w)
	}

	if d.Diameter != 4 {
		t.Errorf("diameter: got %d, want 4", d.Diameter)
	}
	almost(t, "avgPathLength", d.AvgPathLength, 1.933333333)

	// eigenvector: power-iteration (nx tol 1e-6) -> сравнение с epsilon 1e-6.
	eig := map[string]float64{
		"A": 0.291011543, "B": 0.219678115, "C": 0.385508217,
		"D": 0.676500517, "E": 0.510670415, "F": 0.0,
	}
	for id, w := range eig {
		if math.Abs(d.Eigenvector[id]-w) > 1e-6 {
			t.Errorf("eigenvector[%s]: got %.9f, want %.9f (Python)", id, d.Eigenvector[id], w)
		}
	}
}

// --- БАТЧ 4: распределение/качество (эталон networkx на A-F) ---

func TestDescriptors_Batch4_vsPython(t *testing.T) {
	d := ComputeDescriptors(refGraph())

	almost(t, "abstractness", d.Abstractness, 0.0)
	if d.AbstractCount != 0 || d.ConcreteCount != 6 {
		t.Errorf("abstract/concrete: got %d/%d, want 0/6", d.AbstractCount, d.ConcreteCount)
	}

	if d.MaxKCore != 2 {
		t.Errorf("maxKCore: got %d, want 2", d.MaxKCore)
	}
	if d.MaxTotalDegree != 4 {
		t.Errorf("maxTotalDegree: got %d, want 4", d.MaxTotalDegree)
	}

	almost(t, "meanDegree", d.MeanDegree, 2.666666667)
	almost(t, "stdDegree", d.StdDegree, 1.105541597)

	// D=|A+I-1| по пакетам (каждый узел A-F -> свой пакет).
	wantD := map[string]float64{
		"A": 0.5, "B": 0.5, "C": 0.5, "D": 0.666666667, "E": 0.5, "F": 0.0,
	}
	for pkg, w := range wantD {
		almost(t, "D["+pkg+"]", d.DistanceMainSequence[pkg], w)
	}
}

// --- БАТЧ 5: reachability/ripple (эталон networkx на A-F) ---

func TestDescriptors_Batch5_vsPython(t *testing.T) {
	d := ComputeDescriptors(refGraph())

	wantImpact := map[string]int{"A": 3, "B": 3, "C": 3, "D": 5, "E": 5, "F": 0}
	for id, w := range wantImpact {
		if d.ChangePropagation[id] != w {
			t.Errorf("changePropagation[%s]: got %d, want %d", id, d.ChangePropagation[id], w)
		}
	}
	if d.MaxImpact != 5 {
		t.Errorf("maxImpact: got %d, want 5", d.MaxImpact)
	}
	almost(t, "avgImpact", d.AvgImpact, 3.166666667)

	if d.MaxComponentDistance != 4 {
		t.Errorf("maxComponentDistance: got %d, want 4", d.MaxComponentDistance)
	}

	// blast = (pagerank + norm_fan_in)/2 на СОШЕДШЕМСЯ pagerank (tol 1e-10, как batch-1
	// golden; дефолтный nx tol 1e-6 даёт менее точный pagerank).
	wantBlast := map[string]float64{
		"A": 0.299490778, "B": 0.283533581, "C": 0.312037124,
		"D": 0.595011811, "E": 0.580760040, "F": 0.0125,
	}
	for id, w := range wantBlast {
		almost(t, "blastRadius["+id+"]", d.BlastRadius[id], w)
	}
}

// abstractness на фикстуре с «абстрактными» именами: 2 из 4 -> 0.5.
func TestDescriptors_Abstractness_Named(t *testing.T) {
	n := func(id string) model.Node { return model.Node{ID: id} }
	g := &model.Graph{Nodes: []model.Node{
		n("core/UserInterface"), n("core/RepositoryPort"), n("svc/Handler"), n("svc/Worker"),
	}}

	d := ComputeDescriptors(g)
	almost(t, "abstractness", d.Abstractness, 0.5)
	if d.AbstractCount != 2 || d.ConcreteCount != 2 {
		t.Errorf("abstract/concrete: got %d/%d, want 2/2", d.AbstractCount, d.ConcreteCount)
	}
}

// --- БАТЧ 6: Фаулер-смеллы (эталон — РЕАЛЬНЫЕ Python-валидаторы) ---

func TestDescriptors_Batch6_FowlerSmells_vsPython(t *testing.T) {
	var nodes []model.Node
	add := func(id, ent string) { nodes = append(nodes, model.Node{ID: id, Entity: ent}) }

	// god_class: app.God с 21 методом (>20)
	add("app.God", "type")
	for i := 0; i < 21; i++ {
		add("app.God.M"+itoa(i), "method")
	}
	// lazy_class: app.Lazy с 1 методом, out_degree 0
	add("app.Lazy", "type")
	add("app.Lazy.M0", "method")
	// speculative_generality: интерфейс без реализаций, in_degree 0
	add("app.Iface", "interface")
	// shotgun_surgery: app.Hub с fan-in 11 (>10)
	add("app.Hub", "type")

	var edges []model.Edge
	for i := 0; i < 11; i++ {
		add("app.c"+itoa(i), "type")
		edges = append(edges, model.Edge{From: "app.c" + itoa(i), To: "app.Hub", Type: "calls"})
	}

	d := ComputeDescriptors(&model.Graph{Nodes: nodes, Edges: edges})

	checks := []struct {
		name string
		got  int
		want int
	}{
		{"godClass", d.GodClass, 1},
		{"lazyClass", d.LazyClass, 1},
		{"shotgunSurgery", d.ShotgunSurgery, 1},
		{"speculativeGenerality", d.SpeculativeGenerality, 1},
		{"featureEnvy", d.FeatureEnvy, 0},
		{"divergentChange", d.DivergentChange, 0},
		{"middleMan", d.MiddleMan, 0},
		{"dataClumps", d.DataClumps, 0},
		{"zigzagCoupling", d.ZigzagCoupling, 0},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %d, want %d (Python)", c.name, c.got, c.want)
		}
	}
}

// zigzag: caller X -> a.b.F1, a.c.G1, a.b.F2 -> компоненты [a.b, a.c, a.b] -> non-adjacent повтор.
func TestDescriptors_Zigzag_NonAdjacent(t *testing.T) {
	g := &model.Graph{
		Nodes: []model.Node{
			{ID: "X", Entity: "type"},
			{ID: "a.b.F1", Entity: "method"},
			{ID: "a.c.G1", Entity: "method"},
			{ID: "a.b.F2", Entity: "method"},
		},
		Edges: []model.Edge{
			{From: "X", To: "a.b.F1", Type: "calls"},
			{From: "X", To: "a.c.G1", Type: "calls"},
			{From: "X", To: "a.b.F2", Type: "calls"},
		},
	}

	if d := ComputeDescriptors(g); d.ZigzagCoupling != 1 {
		t.Errorf("zigzagCoupling: got %d, want 1 (a.b повторяется non-adjacent)", d.ZigzagCoupling)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}

	var b []byte

	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}

	return string(b)
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
