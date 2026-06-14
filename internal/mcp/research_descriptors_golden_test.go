package mcp

import (
	"reflect"
	"testing"

	"github.com/mshogin/archlint/internal/model"
)

// Golden исследовательских дескрипторов: эталон снят с РЕАЛЬНЫХ Python-валидаторов
// (validator/structure/research, networkx 3.5) на тех же фикстурах.
//
// Фикстура refGraph (A-F, с циклами {A,B,C} и {D,E}) — ветка конденсации.
// Фикстура dagFixture (P,Q,R,S, ацикличная, P->S избыточно) — ветка DAG/редукции.

// dagFixture — ацикличный граф: P->Q,P->R,P->S,Q->S,R->S (P->S транзитивно избыточно).
func dagFixture() *model.Graph {
	n := func(id string) model.Node { return model.Node{ID: id, Entity: "package"} }
	e := func(f, t string) model.Edge { return model.Edge{From: f, To: t, Type: "calls"} }

	return &model.Graph{
		Nodes: []model.Node{n("P"), n("Q"), n("R"), n("S")},
		Edges: []model.Edge{e("P", "Q"), e("P", "R"), e("P", "S"), e("Q", "S"), e("R", "S")},
	}
}

func TestResearch_TransitiveClosure_Cyclic(t *testing.T) {
	tc := ComputeResearchDescriptors(refGraph()).TransitiveClosure
	if tc == nil {
		t.Fatal("transitiveClosure nil")
	}

	if tc.OriginalEdges != 8 {
		t.Errorf("originalEdges: got %d, want 8", tc.OriginalEdges)
	}
	if tc.ClosureEdges != 24 {
		t.Errorf("closureEdges: got %d, want 24 (Python)", tc.ClosureEdges)
	}
	if tc.ImplicitDependencies != 16 {
		t.Errorf("implicit: got %d, want 16", tc.ImplicitDependencies)
	}

	almost(t, "closureRatio", tc.ClosureRatio, 3.0)
	almost(t, "closureDensity", tc.ClosureDensity, 0.8)
	almost(t, "avgReachability", tc.AvgReachability, 0.6333)

	// Граф с циклами -> nx.transitive_reduction падает -> fallback = исходные рёбра.
	if tc.ReductionEdges != 8 || tc.RedundantEdges != 0 {
		t.Errorf("reduction fallback: got (%d,%d), want (8,0)", tc.ReductionEdges, tc.RedundantEdges)
	}
}

func TestResearch_TransitiveClosure_DAG(t *testing.T) {
	tc := ComputeResearchDescriptors(dagFixture()).TransitiveClosure
	if tc == nil {
		t.Fatal("transitiveClosure nil")
	}

	if tc.OriginalEdges != 5 || tc.ClosureEdges != 5 || tc.ImplicitDependencies != 0 {
		t.Errorf("edges: got orig=%d closure=%d implicit=%d, want 5/5/0",
			tc.OriginalEdges, tc.ClosureEdges, tc.ImplicitDependencies)
	}

	almost(t, "closureRatio", tc.ClosureRatio, 1.0)
	almost(t, "closureDensity", tc.ClosureDensity, 0.4167)
	almost(t, "avgReachability", tc.AvgReachability, 0.4167)

	// P->S избыточно (путь P->Q->S) -> редукция 4 ребра, 1 избыточное.
	if tc.ReductionEdges != 4 || tc.RedundantEdges != 1 {
		t.Errorf("reduction: got (%d,%d), want (4,1) (Python)", tc.ReductionEdges, tc.RedundantEdges)
	}
}

func TestResearch_PartialOrder_Cyclic(t *testing.T) {
	po := ComputeResearchDescriptors(refGraph()).PartialOrder
	if po == nil {
		t.Fatal("partialOrder nil")
	}

	if po.IsDAG {
		t.Error("isDAG: got true, want false (граф с циклами)")
	}
	if po.MinimalElements != 1 || po.MaximalElements != 1 {
		t.Errorf("min/max: got %d/%d, want 1/1", po.MinimalElements, po.MaximalElements)
	}
	if po.Height != 3 || po.Width != 1 {
		t.Errorf("height/width: got %d/%d, want 3/1 (Python)", po.Height, po.Width)
	}

	almost(t, "comparability", po.ComparabilityRatio, 1.0)
}

func TestResearch_PartialOrder_DAG(t *testing.T) {
	po := ComputeResearchDescriptors(dagFixture()).PartialOrder
	if po == nil {
		t.Fatal("partialOrder nil")
	}

	if !po.IsDAG {
		t.Error("isDAG: got false, want true")
	}
	if po.MinimalElements != 1 || po.MaximalElements != 1 {
		t.Errorf("min/max: got %d/%d, want 1/1", po.MinimalElements, po.MaximalElements)
	}
	if po.Height != 3 || po.Width != 2 {
		t.Errorf("height/width: got %d/%d, want 3/2 (Python)", po.Height, po.Width)
	}

	almost(t, "comparability", po.ComparabilityRatio, 0.8333)
}

func TestResearch_ChainAntichain_Cyclic(t *testing.T) {
	ca := ComputeResearchDescriptors(refGraph()).ChainAntichain
	if ca == nil {
		t.Fatal("chainAntichain nil")
	}

	if ca.MaxChainLength != 3 || ca.MaxAntichainSize != 1 {
		t.Errorf("chain/antichain: got %d/%d, want 3/1 (Python)", ca.MaxChainLength, ca.MaxAntichainSize)
	}
	if ca.DilworthMinChainCover != 1 || ca.MirskyMinAntichainCover != 3 {
		t.Errorf("dilworth/mirsky: got %d/%d, want 1/3", ca.DilworthMinChainCover, ca.MirskyMinAntichainCover)
	}
}

func TestResearch_ChainAntichain_DAG(t *testing.T) {
	ca := ComputeResearchDescriptors(dagFixture()).ChainAntichain
	if ca == nil {
		t.Fatal("chainAntichain nil")
	}

	if ca.MaxChainLength != 3 || ca.MaxAntichainSize != 2 {
		t.Errorf("chain/antichain: got %d/%d, want 3/2 (Python)", ca.MaxChainLength, ca.MaxAntichainSize)
	}
	if ca.DilworthMinChainCover != 2 || ca.MirskyMinAntichainCover != 3 {
		t.Errorf("dilworth/mirsky: got %d/%d, want 2/3", ca.DilworthMinChainCover, ca.MirskyMinAntichainCover)
	}
}

func TestResearch_Geodesic_Cyclic(t *testing.T) {
	gd := ComputeResearchDescriptors(refGraph()).Geodesic
	if gd == nil {
		t.Fatal("geodesic nil")
	}

	if gd.Diameter != 4 || gd.Radius != 2 || gd.WienerIndex != 29 {
		t.Errorf("diameter/radius/wiener: got %d/%d/%d, want 4/2/29 (Python)",
			gd.Diameter, gd.Radius, gd.WienerIndex)
	}

	almost(t, "avgDistance", gd.AvgDistance, 1.9333)

	if !reflect.DeepEqual(gd.Center, []string{"C"}) {
		t.Errorf("center: got %v, want [C]", gd.Center)
	}
	if !reflect.DeepEqual(gd.Periphery, []string{"E", "F"}) {
		t.Errorf("periphery: got %v, want [E F]", gd.Periphery)
	}
}

func TestResearch_Geodesic_DAG(t *testing.T) {
	gd := ComputeResearchDescriptors(dagFixture()).Geodesic
	if gd == nil {
		t.Fatal("geodesic nil")
	}

	if gd.Diameter != 2 || gd.Radius != 1 || gd.WienerIndex != 7 {
		t.Errorf("diameter/radius/wiener: got %d/%d/%d, want 2/1/7 (Python)",
			gd.Diameter, gd.Radius, gd.WienerIndex)
	}

	almost(t, "avgDistance", gd.AvgDistance, 1.1667)

	if !reflect.DeepEqual(gd.Center, []string{"P", "S"}) {
		t.Errorf("center: got %v, want [P S]", gd.Center)
	}
	if !reflect.DeepEqual(gd.Periphery, []string{"Q", "R"}) {
		t.Errorf("periphery: got %v, want [Q R]", gd.Periphery)
	}
}

func TestResearch_SkipSmallGraph(t *testing.T) {
	// < 2 узлов -> transitiveClosure/partialOrder/chainAntichain nil; < 3 -> geodesic nil.
	one := &model.Graph{Nodes: []model.Node{{ID: "X"}}}
	rd := ComputeResearchDescriptors(one)

	if rd.TransitiveClosure != nil || rd.PartialOrder != nil || rd.ChainAntichain != nil || rd.Geodesic != nil {
		t.Errorf("граф из 1 узла -> все research-сигналы nil, got %+v", rd)
	}
}
