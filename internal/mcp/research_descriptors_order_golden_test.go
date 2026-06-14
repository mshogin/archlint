package mcp

import (
	"reflect"
	"testing"
)

// Golden batch-3 (тонкие): теория множеств/порядка/информации + спектральная близость.
// Эталон — РЕАЛЬНЫЕ Python-валидаторы (set_theory / information_theory /
// integral_calculus, networkx 3.5) на refGraph (A-F, циклы) и dagFixture (P,Q,R,S).
// Спектральные (resistance-closeness/commute) опираются на pinv лапласиана —
// gauge-инвариантны, детерминированы.

func TestResearchOrder_EquivalenceClasses(t *testing.T) {
	af := ComputeResearchDescriptors(refGraph()).EquivalenceClasses
	if af.NumClasses != 3 || af.MaxClassSize != 3 || af.MinClassSize != 1 ||
		af.SingletonClasses != 1 || af.NonTrivialClasses != 2 ||
		af.QuotientNodes != 3 || af.QuotientEdges != 2 {
		t.Errorf("AF: got %+v, want num=3/max=3/min=1/singleton=1/nonTrivial=2/qN=3/qE=2 (Python)", af)
	}
	almost(t, "AF meanClassSize", af.MeanClassSize, 2.0)
	almost(t, "AF partitionEntropy", af.PartitionEntropy, 1.4591)

	dag := ComputeResearchDescriptors(dagFixture()).EquivalenceClasses
	if dag.NumClasses != 4 || dag.SingletonClasses != 4 || dag.NonTrivialClasses != 0 ||
		dag.QuotientNodes != 4 || dag.QuotientEdges != 5 {
		t.Errorf("DAG: got %+v, want num=4/singleton=4/nonTrivial=0/qN=4/qE=5 (Python)", dag)
	}
	almost(t, "DAG partitionEntropy", dag.PartitionEntropy, 2.0)
}

func TestResearchOrder_Lattice(t *testing.T) {
	for _, name := range []string{"AF", "DAG"} {
		var l *LatticeSignal
		if name == "AF" {
			l = ComputeResearchDescriptors(refGraph()).Lattice
		} else {
			l = ComputeResearchDescriptors(dagFixture()).Lattice
		}

		if l == nil {
			t.Fatalf("%s lattice nil", name)
		}
		if !l.IsLattice || !l.IsJoinSemilattice || !l.IsMeetSemilattice || !l.HasTop || !l.HasBottom {
			t.Errorf("%s: got %+v, want все true (Python)", name, l)
		}
		almost(t, name+" joinRatio", l.JoinRatio, 1.0)
		almost(t, name+" meetRatio", l.MeetRatio, 1.0)
	}
}

func TestResearchOrder_JoinMeet(t *testing.T) {
	af := ComputeResearchDescriptors(refGraph()).JoinMeet
	if af.PairsWithMeet != 15 || af.PairsWithJoin != 15 {
		t.Errorf("AF pairs: got meet=%d join=%d, want 15/15 (Python)", af.PairsWithMeet, af.PairsWithJoin)
	}
	almost(t, "AF avgMeet", af.AvgMeetCandidates, 3.13)
	almost(t, "AF avgJoin", af.AvgJoinCandidates, 3.2)

	dag := ComputeResearchDescriptors(dagFixture()).JoinMeet
	if dag.PairsWithMeet != 6 || dag.PairsWithJoin != 6 {
		t.Errorf("DAG pairs: got meet=%d join=%d, want 6/6 (Python)", dag.PairsWithMeet, dag.PairsWithJoin)
	}
	almost(t, "DAG avgMeet", dag.AvgMeetCandidates, 1.33)
	almost(t, "DAG avgJoin", dag.AvgJoinCandidates, 1.33)
}

func TestResearchOrder_PartitionRefinement(t *testing.T) {
	af := ComputeResearchDescriptors(refGraph()).PartitionRefinement
	if !reflect.DeepEqual(af.NumBlocks, map[string]int{"scc": 3, "in_degree": 3, "out_degree": 2}) {
		t.Errorf("AF numBlocks: got %v, want {scc:3,in_degree:3,out_degree:2}", af.NumBlocks)
	}
	almost(t, "AF entropy scc", af.Entropies["scc"], 1.4591)
	almost(t, "AF entropy out_degree", af.Entropies["out_degree"], 0.9183)
	if af.Finest != "out_degree" || af.Coarsest != "scc" {
		t.Errorf("AF finest/coarsest: got %s/%s, want out_degree/scc (Python)", af.Finest, af.Coarsest)
	}

	dag := ComputeResearchDescriptors(dagFixture()).PartitionRefinement
	if !reflect.DeepEqual(dag.NumBlocks, map[string]int{"scc": 4, "topological": 3, "in_degree": 3, "out_degree": 3}) {
		t.Errorf("DAG numBlocks: got %v, want {scc:4,topological:3,in_degree:3,out_degree:3}", dag.NumBlocks)
	}
	if dag.Finest != "topological" || dag.Coarsest != "scc" {
		t.Errorf("DAG finest/coarsest: got %s/%s, want topological/scc (Python)", dag.Finest, dag.Coarsest)
	}
}

func TestResearchOrder_MutualInformation(t *testing.T) {
	af := ComputeResearchDescriptors(refGraph()).MutualInformation
	if af.HighMICount != 2 || af.PackagesAnalyzed != 6 {
		t.Errorf("AF: got high=%d pkgs=%d, want 2/6 (Python)", af.HighMICount, af.PackagesAnalyzed)
	}

	dag := ComputeResearchDescriptors(dagFixture()).MutualInformation
	if dag.HighMICount != 0 || dag.PackagesAnalyzed != 4 {
		t.Errorf("DAG: got high=%d pkgs=%d, want 0/4 (Python)", dag.HighMICount, dag.PackagesAnalyzed)
	}
}

func TestResearchSpectral_ResistanceCloseness(t *testing.T) {
	af := ComputeResearchDescriptors(refGraph()).ResistanceCloseness
	if af == nil {
		t.Fatal("AF resistanceCloseness nil")
	}
	almost(t, "AF meanCloseness", af.MeanCloseness, 1.8415)
	almost(t, "AF stdCloseness", af.StdCloseness, 0.8919)
	if af.MostCentral[0] != "C" {
		t.Errorf("AF mostCentral[0]: got %s, want C (макс closeness)", af.MostCentral[0])
	}
	if af.MostPeripheral[0] != "E" {
		t.Errorf("AF mostPeripheral[0]: got %s, want E (мин closeness)", af.MostPeripheral[0])
	}

	dag := ComputeResearchDescriptors(dagFixture()).ResistanceCloseness
	almost(t, "DAG meanCloseness", dag.MeanCloseness, 4.2667)
	almost(t, "DAG stdCloseness", dag.StdCloseness, 1.0667)
}

func TestResearchSpectral_CommuteTime(t *testing.T) {
	af := ComputeResearchDescriptors(refGraph()).CommuteTime
	if af == nil {
		t.Fatal("AF commuteTime nil")
	}
	if af.NumEdges != 6 {
		t.Errorf("AF numEdges: got %d, want 6", af.NumEdges)
	}
	almost(t, "AF avgCommute", af.AvgCommuteTime, 20.27)
	almost(t, "AF maxCommute", af.MaxCommuteTime, 44.0)
	if af.MostDistantPair != "E--F" {
		t.Errorf("AF mostDistantPair: got %s, want E--F (Python)", af.MostDistantPair)
	}

	dag := ComputeResearchDescriptors(dagFixture()).CommuteTime
	if dag.NumEdges != 5 {
		t.Errorf("DAG numEdges: got %d, want 5", dag.NumEdges)
	}
	almost(t, "DAG avgCommute", dag.AvgCommuteTime, 6.67)
	almost(t, "DAG maxCommute", dag.MaxCommuteTime, 10.0)
}
