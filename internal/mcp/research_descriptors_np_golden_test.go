package mcp

import (
	"reflect"
	"testing"
)

// Golden batch-2 research-дескрипторов (граф-NP + спектральная топология). Эталон —
// РЕАЛЬНЫЕ Python-валидаторы (advanced_graph_metrics / advanced_metrics /
// advanced_topology_metrics / game_theory_metrics, networkx 3.5) на refGraph (A-F,
// циклы) и dagFixture (P,Q,R,S). Set-greedy метрики size-инвариантны к порядку
// (проверено на 3 hash-seed Python); vertex_cover канонизирован детерминированным
// порядком рёбер (Python 2-аппрокс нестабилен по hash-порядку).

func TestResearchNP_Treewidth(t *testing.T) {
	for _, tc := range []struct {
		name              string
		tw                *TreewidthSignal
		lower, upper, deg int
	}{
		{"AF", ComputeResearchDescriptors(refGraph()).Treewidth, 2, 4, 2},
		{"DAG", ComputeResearchDescriptors(dagFixture()).Treewidth, 2, 4, 2},
	} {
		if tc.tw == nil {
			t.Fatalf("%s: treewidth nil", tc.name)
		}
		if tc.tw.TreewidthLowerBound != tc.lower || tc.tw.TreewidthUpperBound != tc.upper || tc.tw.Degeneracy != tc.deg {
			t.Errorf("%s: got lower=%d upper=%d deg=%d, want %d/%d/%d (Python)",
				tc.name, tc.tw.TreewidthLowerBound, tc.tw.TreewidthUpperBound, tc.tw.Degeneracy, tc.lower, tc.upper, tc.deg)
		}
	}
}

func TestResearchNP_Chromatic(t *testing.T) {
	af := ComputeResearchDescriptors(refGraph()).Chromatic
	if af == nil {
		t.Fatal("AF chromatic nil")
	}
	if af.ChromaticLowerBound != 3 || af.ChromaticUpperBound != 3 {
		t.Errorf("AF bounds: got %d/%d, want 3/3 (Python)", af.ChromaticLowerBound, af.ChromaticUpperBound)
	}
	if !reflect.DeepEqual(af.ColorDistribution, map[int]int{0: 2, 1: 3, 2: 1}) {
		t.Errorf("AF color_distribution: got %v, want {0:2,1:3,2:1}", af.ColorDistribution)
	}

	dag := ComputeResearchDescriptors(dagFixture()).Chromatic
	if dag.ChromaticLowerBound != 3 || dag.ChromaticUpperBound != 3 {
		t.Errorf("DAG bounds: got %d/%d, want 3/3", dag.ChromaticLowerBound, dag.ChromaticUpperBound)
	}
	if !reflect.DeepEqual(dag.ColorDistribution, map[int]int{0: 1, 1: 1, 2: 2}) {
		t.Errorf("DAG color_distribution: got %v, want {0:1,1:1,2:2}", dag.ColorDistribution)
	}
}

func TestResearchNP_DominatingSet(t *testing.T) {
	af := ComputeResearchDescriptors(refGraph()).DominatingSet
	if af.DominationNumber != 2 || af.TotalNodes != 6 {
		t.Errorf("AF: got num=%d total=%d, want 2/6 (Python)", af.DominationNumber, af.TotalNodes)
	}
	almost(t, "AF dominationRatio", af.DominationRatio, 0.333)

	dag := ComputeResearchDescriptors(dagFixture()).DominatingSet
	if dag.DominationNumber != 1 {
		t.Errorf("DAG dominationNumber: got %d, want 1 (Python)", dag.DominationNumber)
	}
	almost(t, "DAG dominationRatio", dag.DominationRatio, 0.25)
}

func TestResearchNP_IndependenceNumber(t *testing.T) {
	af := ComputeResearchDescriptors(refGraph()).IndependenceNumber
	if af.IndependenceNumber != 3 {
		t.Errorf("AF independenceNumber: got %d, want 3 (Python)", af.IndependenceNumber)
	}
	almost(t, "AF independenceRatio", af.IndependenceRatio, 0.5)

	dag := ComputeResearchDescriptors(dagFixture()).IndependenceNumber
	if dag.IndependenceNumber != 2 {
		t.Errorf("DAG independenceNumber: got %d, want 2 (Python)", dag.IndependenceNumber)
	}
	almost(t, "DAG independenceRatio", dag.IndependenceRatio, 0.5)
}

func TestResearchNP_VertexCover(t *testing.T) {
	af := ComputeResearchDescriptors(refGraph()).VertexCover
	if af.CoverSize != 4 || af.TotalNodes != 6 {
		t.Errorf("AF: got size=%d total=%d, want 4/6 (детерминированный 2-аппрокс)", af.CoverSize, af.TotalNodes)
	}
	almost(t, "AF coverRatio", af.CoverRatio, 0.667)

	dag := ComputeResearchDescriptors(dagFixture()).VertexCover
	if dag.CoverSize != 4 {
		t.Errorf("DAG coverSize: got %d, want 4", dag.CoverSize)
	}
	almost(t, "DAG coverRatio", dag.CoverRatio, 1.0)
}

func TestResearchNP_GraphCliques(t *testing.T) {
	af := ComputeResearchDescriptors(refGraph()).GraphCliques
	if af.MaxCliqueSize != 3 || af.TotalCliques != 4 || af.LargeCliquesCount != 0 {
		t.Errorf("AF: got max=%d total=%d large=%d, want 3/4/0 (Python)",
			af.MaxCliqueSize, af.TotalCliques, af.LargeCliquesCount)
	}

	dag := ComputeResearchDescriptors(dagFixture()).GraphCliques
	if dag.MaxCliqueSize != 3 || dag.TotalCliques != 2 {
		t.Errorf("DAG: got max=%d total=%d, want 3/2 (Python)", dag.MaxCliqueSize, dag.TotalCliques)
	}
}

func TestResearchNP_PersistentHomology(t *testing.T) {
	af := ComputeResearchDescriptors(refGraph()).PersistentHomology
	if af.BarcodeH0Count != 6 || af.BarcodeH1Count != 1 || af.ShortLivedComponents != 2 || af.CyclesCreated != 1 {
		t.Errorf("AF: got h0=%d h1=%d short=%d cycles=%d, want 6/1/2/1 (Python)",
			af.BarcodeH0Count, af.BarcodeH1Count, af.ShortLivedComponents, af.CyclesCreated)
	}
	almost(t, "AF avgPersistenceH0", af.AvgPersistenceH0, 3.4)

	dag := ComputeResearchDescriptors(dagFixture()).PersistentHomology
	if dag.BarcodeH0Count != 4 || dag.BarcodeH1Count != 2 || dag.ShortLivedComponents != 2 || dag.CyclesCreated != 2 {
		t.Errorf("DAG: got h0=%d h1=%d short=%d cycles=%d, want 4/2/2/2 (Python)",
			dag.BarcodeH0Count, dag.BarcodeH1Count, dag.ShortLivedComponents, dag.CyclesCreated)
	}
	almost(t, "DAG avgPersistenceH0", dag.AvgPersistenceH0, 2.0)
}

func TestResearchNP_Hodge(t *testing.T) {
	af := ComputeResearchDescriptors(refGraph()).HodgeDecomposition
	if af.Harmonic0 != 1 || af.Harmonic1 != 1 || af.ExpectedH0 != 1 || af.ExpectedH1 != 1 {
		t.Errorf("AF harmonics: got h0=%d h1=%d exp0=%d exp1=%d, want 1/1/1/1 (Python)",
			af.Harmonic0, af.Harmonic1, af.ExpectedH0, af.ExpectedH1)
	}
	almost(t, "AF spectralGapL0", af.SpectralGapL0, 0.4131)
	almost(t, "AF spectralGapL1", af.SpectralGapL1, 0.4131)

	dag := ComputeResearchDescriptors(dagFixture()).HodgeDecomposition
	if dag.Harmonic1 != 2 || dag.ExpectedH1 != 2 {
		t.Errorf("DAG h1/exp1: got %d/%d, want 2/2 (Python)", dag.Harmonic1, dag.ExpectedH1)
	}
	almost(t, "DAG spectralGapL0", dag.SpectralGapL0, 2.0)
}

func TestResearchNP_Shapley(t *testing.T) {
	af := ComputeResearchDescriptors(refGraph()).ShapleyValue
	if af == nil {
		t.Fatal("AF shapley nil")
	}

	want := map[string]float64{"A": 2.0, "C": 2.0, "D": 1.25, "B": 1.0, "E": 1.0, "F": 0.75}
	for k, v := range want {
		almost(t, "AF shapley["+k+"]", af.ShapleyValues[k], v)
	}
	almost(t, "AF totalShapley", af.TotalShapley, 8.0)
	almost(t, "AF grandCoalition", af.GrandCoalitionValue, 8.0)
	almost(t, "AF efficiency", af.EfficiencyCheck, 0.0)

	dag := ComputeResearchDescriptors(dagFixture()).ShapleyValue
	wantD := map[string]float64{"P": 2.25, "Q": 1.0, "R": 1.0, "S": 0.75}
	for k, v := range wantD {
		almost(t, "DAG shapley["+k+"]", dag.ShapleyValues[k], v)
	}
	almost(t, "DAG totalShapley", dag.TotalShapley, 5.0)
	almost(t, "DAG efficiency", dag.EfficiencyCheck, 0.0)
}
