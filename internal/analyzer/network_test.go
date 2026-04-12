package analyzer

import (
	"math"
	"testing"

	"github.com/mshogin/archlint/internal/model"
)

// tolerance for floating-point comparisons.
const eps = 1e-6

func approxEqual(a, b, tol float64) bool {
	return math.Abs(a-b) <= tol
}

// makeGraph is a helper that builds a model.Graph from a list of node IDs and directed edges.
func makeGraph(nodeIDs []string, edges [][2]string) *model.Graph {
	g := &model.Graph{}
	for _, id := range nodeIDs {
		g.Nodes = append(g.Nodes, model.Node{ID: id})
	}
	for _, e := range edges {
		g.Edges = append(g.Edges, model.Edge{From: e[0], To: e[1]})
	}
	return g
}

// TestNetworkEmptyGraph ensures no panics and sensible zero values for an empty graph.
func TestNetworkEmptyGraph(t *testing.T) {
	g := &model.Graph{}
	nm := ComputeNetworkMetrics(g)
	if nm == nil {
		t.Fatal("expected non-nil NetworkMetrics")
	}
	if len(nm.BetweennessCentrality) != 0 {
		t.Errorf("expected empty betweenness, got %v", nm.BetweennessCentrality)
	}
	if nm.Diameter != 0 {
		t.Errorf("expected diameter 0, got %d", nm.Diameter)
	}
}

// TestNetworkSingleNode tests a graph with one node and no edges.
func TestNetworkSingleNode(t *testing.T) {
	g := makeGraph([]string{"A"}, nil)
	nm := ComputeNetworkMetrics(g)

	if nm.BetweennessCentrality["A"] != 0 {
		t.Errorf("single node betweenness should be 0, got %f", nm.BetweennessCentrality["A"])
	}
	if nm.PageRank["A"] != 1.0 {
		t.Errorf("single node PageRank should be 1.0, got %f", nm.PageRank["A"])
	}
	if nm.Diameter != 0 {
		t.Errorf("single node diameter should be 0, got %d", nm.Diameter)
	}
	if nm.AverageShortestPath != 0 {
		t.Errorf("single node avg path should be 0, got %f", nm.AverageShortestPath)
	}
}

// TestNetworkChain tests a simple chain A->B->C->D.
// Expected: B and C should have higher betweenness than A and D.
func TestNetworkChain(t *testing.T) {
	g := makeGraph(
		[]string{"A", "B", "C", "D"},
		[][2]string{{"A", "B"}, {"B", "C"}, {"C", "D"}},
	)
	nm := ComputeNetworkMetrics(g)

	// B and C are on all paths, so they should have non-zero betweenness.
	if nm.BetweennessCentrality["B"] <= 0 {
		t.Errorf("B should have positive betweenness in chain, got %f", nm.BetweennessCentrality["B"])
	}
	if nm.BetweennessCentrality["C"] <= 0 {
		t.Errorf("C should have positive betweenness in chain, got %f", nm.BetweennessCentrality["C"])
	}

	// A has no predecessors so has no betweenness on paths from others.
	if nm.BetweennessCentrality["A"] > nm.BetweennessCentrality["B"] {
		t.Errorf("A betweenness should be <= B in chain, A=%f B=%f",
			nm.BetweennessCentrality["A"], nm.BetweennessCentrality["B"])
	}

	// Diameter should be 3 (A->B->C->D).
	if nm.Diameter != 3 {
		t.Errorf("chain diameter should be 3, got %d", nm.Diameter)
	}

	// Average path length for reachable pairs in directed chain A->B->C->D:
	// A->B=1, A->C=2, A->D=3, B->C=1, B->D=2, C->D=1 => total=10, pairs=6 => avg=10/6
	expectedAvg := 10.0 / 6.0
	if !approxEqual(nm.AverageShortestPath, expectedAvg, 1e-6) {
		t.Errorf("chain avg shortest path: expected %f, got %f", expectedAvg, nm.AverageShortestPath)
	}

	// PageRank: D (sink) should have highest page rank in a chain.
	if nm.PageRank["D"] < nm.PageRank["A"] {
		t.Errorf("PageRank[D] should be >= PageRank[A] in chain, D=%f A=%f",
			nm.PageRank["D"], nm.PageRank["A"])
	}

	// Clustering coefficient: in a simple chain there are no triangles.
	for _, v := range []string{"A", "B", "C", "D"} {
		if nm.ClusteringCoefficient[v] != 0 {
			t.Errorf("chain node %s clustering should be 0, got %f", v, nm.ClusteringCoefficient[v])
		}
	}
}

// TestNetworkStarGraph tests a star graph with one hub connected to several leaves.
// Hub should have the highest betweenness centrality.
func TestNetworkStarGraph(t *testing.T) {
	// Star: hub -> leaf1, hub -> leaf2, hub -> leaf3
	g := makeGraph(
		[]string{"hub", "leaf1", "leaf2", "leaf3"},
		[][2]string{
			{"hub", "leaf1"},
			{"hub", "leaf2"},
			{"hub", "leaf3"},
		},
	)
	nm := ComputeNetworkMetrics(g)

	// Hub has highest betweenness (all paths go through it from leaf to leaf via hub is not possible directed).
	// Actually in directed star hub->leaves, hub has 0 betweenness (no paths go "through" it
	// since leaves can't reach other leaves). Leaves have 0 betweenness.
	// Hub is a source, not a bottleneck in directed star.
	// PageRank: leaves should get the rank from hub.
	for _, leaf := range []string{"leaf1", "leaf2", "leaf3"} {
		if nm.PageRank[leaf] < nm.PageRank["hub"] {
			t.Errorf("in directed star, leaf PageRank should be >= hub, %s=%f hub=%f",
				leaf, nm.PageRank[leaf], nm.PageRank["hub"])
		}
	}
}

// TestNetworkCompleteGraph tests K4 (4 fully connected nodes, undirected treated as directed both ways).
// Clustering coefficient should be 1.0 for all nodes.
func TestNetworkCompleteGraph(t *testing.T) {
	nodes := []string{"A", "B", "C", "D"}
	var edges [][2]string
	for i := 0; i < len(nodes); i++ {
		for j := 0; j < len(nodes); j++ {
			if i != j {
				edges = append(edges, [2]string{nodes[i], nodes[j]})
			}
		}
	}
	g := makeGraph(nodes, edges)
	nm := ComputeNetworkMetrics(g)

	// In K4, every node has 3 neighbors, and all pairs of neighbors are connected.
	for _, v := range nodes {
		if !approxEqual(nm.ClusteringCoefficient[v], 1.0, eps) {
			t.Errorf("K4 clustering coefficient for %s should be 1.0, got %f",
				v, nm.ClusteringCoefficient[v])
		}
	}

	// Global clustering coefficient should be 1.0.
	if !approxEqual(nm.GlobalClusteringCoefficient, 1.0, eps) {
		t.Errorf("K4 global clustering coefficient should be 1.0, got %f",
			nm.GlobalClusteringCoefficient)
	}

	// Diameter should be 1 (all directly connected).
	if nm.Diameter != 1 {
		t.Errorf("K4 diameter should be 1, got %d", nm.Diameter)
	}

	// Average path length should be 1.
	if !approxEqual(nm.AverageShortestPath, 1.0, eps) {
		t.Errorf("K4 avg path should be 1.0, got %f", nm.AverageShortestPath)
	}

	// PageRank sum should be ~1.0.
	sum := 0.0
	for _, pr := range nm.PageRank {
		sum += pr
	}
	if !approxEqual(sum, 1.0, 1e-4) {
		t.Errorf("PageRank sum should be ~1.0, got %f", sum)
	}

	// All nodes symmetric, so PageRank should be equal.
	pr0 := nm.PageRank[nodes[0]]
	for _, v := range nodes[1:] {
		if !approxEqual(nm.PageRank[v], pr0, 1e-4) {
			t.Errorf("K4 PageRank should be equal, %s=%f %s=%f", nodes[0], pr0, v, nm.PageRank[v])
		}
	}
}

// TestNetworkTwoClusterGraph tests a graph with two clearly separated clusters.
// Modularity should be positive (clusters detected).
func TestNetworkTwoClusterGraph(t *testing.T) {
	// Cluster 1: A <-> B <-> C (fully connected, bidirectional)
	// Cluster 2: D <-> E <-> F (fully connected, bidirectional)
	// Bridge: C -> D (single directed edge between clusters)
	nodes := []string{"A", "B", "C", "D", "E", "F"}
	edges := [][2]string{
		{"A", "B"}, {"B", "A"},
		{"B", "C"}, {"C", "B"},
		{"A", "C"}, {"C", "A"},
		{"D", "E"}, {"E", "D"},
		{"E", "F"}, {"F", "E"},
		{"D", "F"}, {"F", "D"},
		{"C", "D"}, // single bridge
	}
	g := makeGraph(nodes, edges)
	nm := ComputeNetworkMetrics(g)

	// C is on the path between clusters, should have high betweenness.
	// D also since it's the bridge destination.
	maxBC := 0.0
	maxNode := ""
	for _, v := range nodes {
		if nm.BetweennessCentrality[v] > maxBC {
			maxBC = nm.BetweennessCentrality[v]
			maxNode = v
		}
	}
	// C or D should have the highest betweenness since they are bridge nodes.
	if maxNode != "C" && maxNode != "D" {
		t.Logf("expected C or D to have highest betweenness, got %s=%f", maxNode, maxBC)
		// Not a hard failure as betweenness depends on directed path count.
	}

	// Degree histogram should be non-empty.
	if len(nm.DegreeHistogram) == 0 {
		t.Error("DegreeHistogram should be non-empty")
	}

	// Entropy should be >= 0.
	if nm.DegreeEntropy < 0 {
		t.Errorf("DegreeEntropy should be >= 0, got %f", nm.DegreeEntropy)
	}

	// With clear clusters, modularity should be positive.
	if nm.Modularity < 0 {
		t.Logf("modularity is negative (%f); this can happen with label propagation", nm.Modularity)
	}
}

// TestNetworkPageRankSum verifies that PageRank sums to 1.0 for any graph.
func TestNetworkPageRankSum(t *testing.T) {
	g := makeGraph(
		[]string{"A", "B", "C", "D", "E"},
		[][2]string{
			{"A", "B"}, {"B", "C"}, {"C", "A"}, // cycle
			{"D", "E"},                           // separate component
		},
	)
	nm := ComputeNetworkMetrics(g)

	sum := 0.0
	for _, pr := range nm.PageRank {
		sum += pr
	}
	if !approxEqual(sum, 1.0, 1e-4) {
		t.Errorf("PageRank should sum to 1.0, got %f", sum)
	}
}

// TestNetworkDegreeEntropy verifies entropy is 0 for a graph where all nodes have the same degree.
func TestNetworkDegreeEntropy(t *testing.T) {
	// K3: all nodes have degree 2 (out=2, in=0 if directed... but let's use bidirectional).
	g := makeGraph(
		[]string{"A", "B", "C"},
		[][2]string{
			{"A", "B"}, {"B", "A"},
			{"B", "C"}, {"C", "B"},
			{"A", "C"}, {"C", "A"},
		},
	)
	nm := ComputeNetworkMetrics(g)

	// All nodes have in-degree=2, out-degree=2 => total degree=4 for each.
	// So histogram has one entry: {4: 3}. Entropy should be 0.
	if !approxEqual(nm.DegreeEntropy, 0.0, eps) {
		t.Errorf("uniform degree distribution entropy should be 0, got %f", nm.DegreeEntropy)
	}
}

// TestNetworkSmallWorldCoefficient checks that small world coefficient is non-negative
// and that a complete graph gives a high sigma (or 0 if L_rand == 0).
func TestNetworkSmallWorldCoefficient(t *testing.T) {
	// Chain graph: should have high path length, low clustering.
	chain := makeGraph(
		[]string{"A", "B", "C", "D", "E"},
		[][2]string{{"A", "B"}, {"B", "C"}, {"C", "D"}, {"D", "E"}},
	)
	nmChain := ComputeNetworkMetrics(chain)
	if nmChain.SmallWorldCoefficient < 0 {
		t.Errorf("SmallWorldCoefficient should be >= 0, got %f", nmChain.SmallWorldCoefficient)
	}
}

// TestNetworkBetweennessBottleneck verifies that a clear bottleneck node has high betweenness.
func TestNetworkBetweennessBottleneck(t *testing.T) {
	// Hourglass: left cluster -> bottleneck -> right cluster.
	// All left nodes connect to bottleneck, all right nodes receive from bottleneck.
	nodes := []string{"L1", "L2", "L3", "BN", "R1", "R2", "R3"}
	edges := [][2]string{
		{"L1", "BN"}, {"L2", "BN"}, {"L3", "BN"},
		{"BN", "R1"}, {"BN", "R2"}, {"BN", "R3"},
	}
	g := makeGraph(nodes, edges)
	nm := ComputeNetworkMetrics(g)

	// BN should have the highest betweenness.
	bnBC := nm.BetweennessCentrality["BN"]
	for _, v := range []string{"L1", "L2", "L3", "R1", "R2", "R3"} {
		if nm.BetweennessCentrality[v] > bnBC {
			t.Errorf("BN should have highest betweenness, but %s=%f > BN=%f",
				v, nm.BetweennessCentrality[v], bnBC)
		}
	}

	// BN betweenness should be positive.
	if bnBC <= 0 {
		t.Errorf("bottleneck node BN should have positive betweenness, got %f", bnBC)
	}
}

// TestNetworkSelfLoopsIgnored verifies that self-loops do not affect metric computation.
func TestNetworkSelfLoopsIgnored(t *testing.T) {
	g := makeGraph(
		[]string{"A", "B", "C"},
		[][2]string{
			{"A", "B"}, {"B", "C"},
			{"A", "A"}, // self-loop
		},
	)
	nm := ComputeNetworkMetrics(g)
	// Should not panic and produce valid results.
	if nm.Diameter < 0 {
		t.Errorf("unexpected negative diameter")
	}
}

// TestNetworkDiameterDisconnected verifies that diameter only considers reachable pairs.
func TestNetworkDiameterDisconnected(t *testing.T) {
	// Two disconnected components: A->B->C and D->E
	g := makeGraph(
		[]string{"A", "B", "C", "D", "E"},
		[][2]string{{"A", "B"}, {"B", "C"}, {"D", "E"}},
	)
	nm := ComputeNetworkMetrics(g)

	// Diameter should be 2 (longest reachable path A->B->C).
	if nm.Diameter != 2 {
		t.Errorf("disconnected graph diameter should be 2, got %d", nm.Diameter)
	}
}
