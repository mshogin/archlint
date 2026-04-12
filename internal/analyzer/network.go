// Package analyzer contains graph analysis algorithms.
package analyzer

import (
	"math"

	"github.com/mshogin/archlint/internal/model"
)

// NetworkMetrics holds all computed network/graph metrics for an architecture graph.
type NetworkMetrics struct {
	// BetweennessCentrality maps node ID to its betweenness centrality score (Brandes algorithm).
	BetweennessCentrality map[string]float64

	// PageRank maps node ID to its PageRank score (damping 0.85, 100 iterations).
	PageRank map[string]float64

	// Modularity is the Q score of the community partition (higher = more modular).
	Modularity float64

	// Communities maps node ID to community label.
	Communities map[string]int

	// ClusteringCoefficient maps node ID to local clustering coefficient.
	ClusteringCoefficient map[string]float64

	// GlobalClusteringCoefficient is the average of all local clustering coefficients.
	GlobalClusteringCoefficient float64

	// AverageShortestPath is the mean shortest path length across all reachable pairs.
	AverageShortestPath float64

	// Diameter is the longest shortest path in the graph (eccentricity of the graph).
	Diameter int

	// DegreeHistogram maps degree value -> count of nodes with that degree.
	// Uses total degree (in + out) for directed graphs.
	DegreeHistogram map[int]int

	// DegreeEntropy is Shannon entropy of the degree distribution H = -Σ p(k) log p(k).
	DegreeEntropy float64

	// SmallWorldCoefficient sigma = (C/C_rand) / (L/L_rand). Values > 1 indicate small-world.
	SmallWorldCoefficient float64
}

// ComputeNetworkMetrics computes all network metrics for the given architecture graph.
func ComputeNetworkMetrics(graph *model.Graph) *NetworkMetrics {
	nm := &NetworkMetrics{}

	if len(graph.Nodes) == 0 {
		nm.BetweennessCentrality = map[string]float64{}
		nm.PageRank = map[string]float64{}
		nm.Communities = map[string]int{}
		nm.ClusteringCoefficient = map[string]float64{}
		nm.DegreeHistogram = map[int]int{}
		return nm
	}

	// Build adjacency structures.
	nodes, adj, radj, degrees := buildAdjacency(graph)

	nm.BetweennessCentrality = computeBetweenness(nodes, adj)
	nm.PageRank = computePageRank(nodes, adj, radj, 0.85, 100)
	nm.Communities, nm.Modularity = detectCommunities(nodes, adj, degrees)
	nm.ClusteringCoefficient, nm.GlobalClusteringCoefficient = computeClusteringCoefficient(nodes, adj, radj)
	nm.AverageShortestPath, nm.Diameter = computeShortestPathStats(nodes, adj)
	nm.DegreeHistogram = computeDegreeHistogram(degrees)
	nm.DegreeEntropy = computeEntropy(nm.DegreeHistogram, len(nodes))
	nm.SmallWorldCoefficient = computeSmallWorld(
		len(nodes),
		nm.GlobalClusteringCoefficient,
		nm.AverageShortestPath,
		averageDegree(degrees),
	)

	return nm
}

// buildAdjacency constructs adjacency lists from the graph.
// Returns: sorted node IDs, outgoing adj, incoming radj, total degree per node.
func buildAdjacency(graph *model.Graph) ([]string, map[string]map[string]bool, map[string]map[string]bool, map[string]int) {
	nodeSet := make(map[string]bool)
	for _, n := range graph.Nodes {
		nodeSet[n.ID] = true
	}

	adj := make(map[string]map[string]bool)
	radj := make(map[string]map[string]bool)

	for id := range nodeSet {
		adj[id] = make(map[string]bool)
		radj[id] = make(map[string]bool)
	}

	for _, e := range graph.Edges {
		if !nodeSet[e.From] || !nodeSet[e.To] {
			continue
		}
		if e.From == e.To {
			continue // skip self-loops
		}
		adj[e.From][e.To] = true
		radj[e.To][e.From] = true
	}

	// Collect sorted node list for determinism.
	nodes := make([]string, 0, len(nodeSet))
	for id := range nodeSet {
		nodes = append(nodes, id)
	}
	sortStrings(nodes)

	// Total degree = out-degree + in-degree (for undirected view).
	degrees := make(map[string]int, len(nodes))
	for _, id := range nodes {
		degrees[id] = len(adj[id]) + len(radj[id])
	}

	return nodes, adj, radj, degrees
}

// computeBetweenness implements Brandes' algorithm for betweenness centrality on a directed graph.
// Normalized by (n-1)*(n-2) for directed graphs.
func computeBetweenness(nodes []string, adj map[string]map[string]bool) map[string]float64 {
	n := len(nodes)
	cb := make(map[string]float64, n)
	for _, v := range nodes {
		cb[v] = 0
	}

	for _, s := range nodes {
		// BFS from s.
		stack := make([]string, 0, n)
		pred := make(map[string][]string, n)
		sigma := make(map[string]float64, n)
		dist := make(map[string]int, n)
		for _, v := range nodes {
			dist[v] = -1
			sigma[v] = 0
			pred[v] = []string{}
		}
		sigma[s] = 1
		dist[s] = 0

		queue := []string{s}
		for len(queue) > 0 {
			v := queue[0]
			queue = queue[1:]
			stack = append(stack, v)
			for w := range adj[v] {
				if dist[w] < 0 {
					queue = append(queue, w)
					dist[w] = dist[v] + 1
				}
				if dist[w] == dist[v]+1 {
					sigma[w] += sigma[v]
					pred[w] = append(pred[w], v)
				}
			}
		}

		// Accumulation.
		delta := make(map[string]float64, n)
		for _, v := range nodes {
			delta[v] = 0
		}
		for len(stack) > 0 {
			w := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			for _, v := range pred[w] {
				if sigma[w] > 0 {
					delta[v] += (sigma[v] / sigma[w]) * (1 + delta[w])
				}
			}
			if w != s {
				cb[w] += delta[w]
			}
		}
	}

	// Normalize.
	if n > 2 {
		norm := float64((n - 1) * (n - 2))
		for k := range cb {
			cb[k] /= norm
		}
	}

	return cb
}

// computePageRank runs iterative PageRank with the given damping factor and iterations.
func computePageRank(nodes []string, adj, radj map[string]map[string]bool, damping float64, iterations int) map[string]float64 {
	n := len(nodes)
	rank := make(map[string]float64, n)
	initial := 1.0 / float64(n)
	for _, v := range nodes {
		rank[v] = initial
	}

	// Identify dangling nodes (no outgoing edges).
	for iter := 0; iter < iterations; iter++ {
		newRank := make(map[string]float64, n)
		danglingSum := 0.0
		for _, v := range nodes {
			if len(adj[v]) == 0 {
				danglingSum += rank[v]
			}
		}
		base := (1.0 - damping + damping*danglingSum) / float64(n)
		for _, v := range nodes {
			sum := 0.0
			for u := range radj[v] {
				out := float64(len(adj[u]))
				if out > 0 {
					sum += rank[u] / out
				}
			}
			newRank[v] = base + damping*sum
		}
		rank = newRank
	}

	return rank
}

// detectCommunities implements greedy label-propagation community detection
// and computes the modularity Q of the resulting partition.
func detectCommunities(nodes []string, adj map[string]map[string]bool, degrees map[string]int) (map[string]int, float64) {
	// Initialize each node in its own community.
	community := make(map[string]int, len(nodes))
	for i, v := range nodes {
		community[v] = i
	}

	// Undirected edge count (count each undirected pair once).
	edgeCount := 0
	for _, v := range nodes {
		edgeCount += len(adj[v])
	}
	// edgeCount is now sum of out-degrees = total directed edges = m (directed).
	// For modularity on undirected graph we use m = number of undirected edges.
	// We'll keep directed interpretation for simplicity (standard for directed modularity).
	m := edgeCount

	if m == 0 {
		return community, 0
	}

	// Label propagation: iterate until stable.
	maxIter := 30
	for iter := 0; iter < maxIter; iter++ {
		changed := false
		// Shuffle-like iteration using index order for determinism.
		for _, v := range nodes {
			// Count community membership of neighbors (treating edges as undirected).
			counts := make(map[int]int)
			for w := range adj[v] {
				counts[community[w]]++
			}
			for w := range adj {
				if adj[w][v] {
					counts[community[w]]++
				}
			}
			if len(counts) == 0 {
				continue
			}
			bestCom := community[v]
			bestCount := 0
			for c, cnt := range counts {
				if cnt > bestCount || (cnt == bestCount && c < bestCom) {
					bestCom = c
					bestCount = cnt
				}
			}
			if bestCom != community[v] {
				community[v] = bestCom
				changed = true
			}
		}
		if !changed {
			break
		}
	}

	// Compute modularity Q for directed graph:
	// Q = (1/m) * Σ_{ij} [A_ij - k_i_out * k_j_in / m] * δ(c_i, c_j)
	// where k_i_out = out-degree, k_j_in = in-degree.
	q := 0.0
	mf := float64(m)
	for _, u := range nodes {
		kOutU := float64(len(adj[u]))
		for _, v := range nodes {
			kInV := 0.0
			// Build in-degree inline.
			inDeg := 0
			for _, w := range nodes {
				if adj[w][v] {
					inDeg++
				}
			}
			kInV = float64(inDeg)
			aij := 0.0
			if adj[u][v] {
				aij = 1.0
			}
			expected := kOutU * kInV / mf
			if community[u] == community[v] {
				q += aij - expected
			}
		}
	}
	q /= mf

	return community, q
}

// computeClusteringCoefficient computes local clustering coefficients for each node
// using the undirected version: for each node u, count triangles among its neighbors
// (using both in and out neighbors as undirected neighbors).
// Returns per-node map and global average.
func computeClusteringCoefficient(nodes []string, adj, radj map[string]map[string]bool) (map[string]float64, float64) {
	cc := make(map[string]float64, len(nodes))

	for _, u := range nodes {
		// Undirected neighborhood: union of successors and predecessors.
		nbrs := make(map[string]bool)
		for w := range adj[u] {
			nbrs[w] = true
		}
		for w := range radj[u] {
			nbrs[w] = true
		}
		delete(nbrs, u)

		k := len(nbrs)
		if k < 2 {
			cc[u] = 0
			continue
		}

		// Count edges between neighbors (undirected: check both directions).
		triangles := 0
		nbList := make([]string, 0, k)
		for w := range nbrs {
			nbList = append(nbList, w)
		}
		for i := 0; i < len(nbList); i++ {
			for j := i + 1; j < len(nbList); j++ {
				wi, wj := nbList[i], nbList[j]
				if adj[wi][wj] || adj[wj][wi] {
					triangles++
				}
			}
		}

		maxPossible := k * (k - 1) / 2
		cc[u] = float64(triangles) / float64(maxPossible)
	}

	// Global average.
	if len(nodes) == 0 {
		return cc, 0
	}
	sum := 0.0
	for _, v := range cc {
		sum += v
	}
	return cc, sum / float64(len(nodes))
}

// computeShortestPathStats runs BFS from every node (directed) and computes
// average shortest path length and diameter across all reachable pairs.
func computeShortestPathStats(nodes []string, adj map[string]map[string]bool) (float64, int) {
	n := len(nodes)
	if n <= 1 {
		return 0, 0
	}

	totalDist := 0
	reachablePairs := 0
	diameter := 0

	for _, s := range nodes {
		dist := bfsDistances(s, nodes, adj)
		for _, t := range nodes {
			if t == s {
				continue
			}
			if d, ok := dist[t]; ok {
				totalDist += d
				reachablePairs++
				if d > diameter {
					diameter = d
				}
			}
		}
	}

	if reachablePairs == 0 {
		return 0, 0
	}

	return float64(totalDist) / float64(reachablePairs), diameter
}

// bfsDistances returns BFS distances from source s using directed edges.
func bfsDistances(s string, _ []string, adj map[string]map[string]bool) map[string]int {
	dist := map[string]int{s: 0}
	queue := []string{s}
	for len(queue) > 0 {
		v := queue[0]
		queue = queue[1:]
		for w := range adj[v] {
			if _, seen := dist[w]; !seen {
				dist[w] = dist[v] + 1
				queue = append(queue, w)
			}
		}
	}
	return dist
}

// computeDegreeHistogram builds a histogram of total degrees (in + out).
func computeDegreeHistogram(degrees map[string]int) map[int]int {
	hist := make(map[int]int)
	for _, d := range degrees {
		hist[d]++
	}
	return hist
}

// computeEntropy computes Shannon entropy of the degree distribution.
func computeEntropy(histogram map[int]int, n int) float64 {
	if n == 0 {
		return 0
	}
	h := 0.0
	nf := float64(n)
	for _, count := range histogram {
		p := float64(count) / nf
		if p > 0 {
			h -= p * math.Log(p)
		}
	}
	return h
}

// averageDegree returns the mean total degree across all nodes.
func averageDegree(degrees map[string]int) float64 {
	if len(degrees) == 0 {
		return 0
	}
	sum := 0
	for _, d := range degrees {
		sum += d
	}
	return float64(sum) / float64(len(degrees))
}

// computeSmallWorld computes the small-world coefficient sigma.
// sigma = (C/C_rand) / (L/L_rand)
// For an Erdős-Rényi random graph: C_rand = k/n, L_rand = ln(n)/ln(k)
// where k = average degree and n = number of nodes.
func computeSmallWorld(n int, globalCC, avgPathLen, k float64) float64 {
	if n <= 2 || k <= 1 || avgPathLen == 0 {
		return 0
	}

	cRand := k / float64(n)
	lRand := math.Log(float64(n)) / math.Log(k)

	if cRand == 0 || lRand == 0 {
		return 0
	}

	cRatio := globalCC / cRand
	lRatio := avgPathLen / lRand

	if lRatio == 0 {
		return 0
	}

	return cRatio / lRatio
}

// sortStrings sorts a string slice in-place (simple insertion sort for small slices).
func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		key := s[i]
		j := i - 1
		for j >= 0 && s[j] > key {
			s[j+1] = s[j]
			j--
		}
		s[j+1] = key
	}
}
