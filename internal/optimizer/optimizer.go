// Package optimizer suggests dependency changes to help a codebase hit metric targets.
//
// Algorithm (v1):
//  1. Compute baseline metrics (max fan-out, spanning-tree coverage) from the graph.
//  2. For each import edge, simulate its removal and recompute affected metrics.
//  3. Score each edge by how many target constraints move closer to satisfied.
//  4. Return the top-N suggestions ranked by impact score.
package optimizer

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/mshogin/archlint/internal/model"
)

// ---- Target config -------------------------------------------------------

// Targets holds the user-defined metric goals parsed from .archlint.yaml.
type Targets struct {
	MaxFanOut             *Constraint // max_fan_out: "<= 7"
	SpanningTreeCoverage  *Constraint // spanning_tree_coverage: ">= 0.4"
}

// Constraint represents a comparison constraint like "<= 7" or ">= 0.4".
type Constraint struct {
	Op    string  // "<=" or ">="
	Value float64 // numeric threshold
}

// ParseConstraint parses a string like "<= 7" or ">= 0.4" into a Constraint.
func ParseConstraint(s string) (*Constraint, error) {
	s = strings.TrimSpace(s)
	for _, op := range []string{"<=", ">=", "<", ">"} {
		if strings.HasPrefix(s, op) {
			raw := strings.TrimSpace(s[len(op):])
			v, err := strconv.ParseFloat(raw, 64)
			if err != nil {
				return nil, fmt.Errorf("cannot parse numeric value %q in constraint %q: %w", raw, s, err)
			}
			return &Constraint{Op: op, Value: v}, nil
		}
	}
	return nil, fmt.Errorf("constraint must start with <=, >=, < or > (got %q)", s)
}

// Satisfied returns true when the measured value satisfies the constraint.
func (c *Constraint) Satisfied(v float64) bool {
	switch c.Op {
	case "<=":
		return v <= c.Value
	case ">=":
		return v >= c.Value
	case "<":
		return v < c.Value
	case ">":
		return v > c.Value
	}
	return false
}

// ---- Metrics ---------------------------------------------------------------

// Metrics holds the computed values for the metrics the optimizer tracks.
type Metrics struct {
	MaxFanOut            int
	SpanningTreeCoverage float64
}

// ComputeMetrics computes optimizer metrics from the graph.
// Only import edges are considered for fan-out and spanning-tree coverage.
func ComputeMetrics(g *model.Graph) Metrics {
	return Metrics{
		MaxFanOut:            computeMaxFanOut(g),
		SpanningTreeCoverage: computeSpanningTreeCoverage(g),
	}
}

// computeMaxFanOut returns the maximum import fan-out across all package nodes.
func computeMaxFanOut(g *model.Graph) int {
	fanOut := make(map[string]int)
	for _, e := range g.Edges {
		if e.Type == "import" {
			fanOut[e.From]++
		}
	}
	max := 0
	for _, v := range fanOut {
		if v > max {
			max = v
		}
	}
	return max
}

// computeSpanningTreeCoverage computes the ratio:
//   (number of unique import edges in the spanning tree) / (total import edges)
//
// We use a simple BFS spanning tree over the import graph.
// If there are no import edges the coverage is defined as 1.0.
func computeSpanningTreeCoverage(g *model.Graph) float64 {
	// Build adjacency list (directed, import only).
	adj := make(map[string][]string)
	nodeSet := make(map[string]bool)
	totalImports := 0

	for _, e := range g.Edges {
		if e.Type != "import" {
			continue
		}
		adj[e.From] = append(adj[e.From], e.To)
		nodeSet[e.From] = true
		nodeSet[e.To] = true
		totalImports++
	}

	if totalImports == 0 {
		return 1.0
	}

	// BFS from all roots (nodes with no incoming import edges).
	inDegree := make(map[string]int)
	for _, e := range g.Edges {
		if e.Type == "import" {
			inDegree[e.To]++
		}
	}

	queue := []string{}
	for n := range nodeSet {
		if inDegree[n] == 0 {
			queue = append(queue, n)
		}
	}
	// If every node has in-degree > 0 (cycle), start from all nodes.
	if len(queue) == 0 {
		for n := range nodeSet {
			queue = append(queue, n)
		}
	}

	visited := make(map[string]bool)
	queued := make(map[string]bool)
	treeEdges := 0

	for _, n := range queue {
		queued[n] = true
	}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if visited[cur] {
			continue
		}
		visited[cur] = true
		for _, next := range adj[cur] {
			if !visited[next] && !queued[next] {
				treeEdges++
				queued[next] = true
				queue = append(queue, next)
			}
		}
	}

	return float64(treeEdges) / float64(totalImports)
}

// ---- Suggestion ------------------------------------------------------------

// Suggestion is a recommended edge removal with its projected impact.
type Suggestion struct {
	From        string
	To          string
	ImpactScore int     // number of target constraints moved closer to satisfied
	FanOutDelta int     // change in max fan-out (negative = improvement)
	CoverageDelta float64 // change in spanning-tree coverage (positive = improvement)
	Reason      string
}

// ---- Optimizer -------------------------------------------------------------

// Optimizer analyses the graph and produces Suggestions.
type Optimizer struct {
	targets        Targets
	preservedPkgs  map[string]bool // packages that must not lose any edges
	topN           int
}

// New creates a new Optimizer.
func New(targets Targets, preservedComponents []string, topN int) *Optimizer {
	preserved := make(map[string]bool, len(preservedComponents))
	for _, p := range preservedComponents {
		// Strip trailing glob wildcard for prefix matching.
		key := strings.TrimSuffix(p, "*")
		preserved[key] = true
	}
	if topN <= 0 {
		topN = 10
	}
	return &Optimizer{
		targets:       targets,
		preservedPkgs: preserved,
		topN:          topN,
	}
}

// Optimize runs the impact simulation and returns ranked suggestions.
func (o *Optimizer) Optimize(g *model.Graph) []Suggestion {
	baseline := ComputeMetrics(g)

	// Collect candidate edges (import only, not involving preserved packages).
	candidates := o.importEdges(g)

	suggestions := make([]Suggestion, 0, len(candidates))

	for _, edge := range candidates {
		sim := o.simulateRemoval(g, edge, baseline)
		if sim.ImpactScore > 0 {
			suggestions = append(suggestions, sim)
		}
	}

	// Rank: primary = impact score (desc), secondary = fan-out delta (most negative first).
	sort.Slice(suggestions, func(i, j int) bool {
		if suggestions[i].ImpactScore != suggestions[j].ImpactScore {
			return suggestions[i].ImpactScore > suggestions[j].ImpactScore
		}
		return suggestions[i].FanOutDelta < suggestions[j].FanOutDelta
	})

	if len(suggestions) > o.topN {
		suggestions = suggestions[:o.topN]
	}
	return suggestions
}

// importEdges returns deduplicated import edges that are not protected by preserved-component rules.
// Deduplication is by (From, To) pair to avoid redundant suggestions from multi-file packages.
func (o *Optimizer) importEdges(g *model.Graph) []model.Edge {
	seen := make(map[string]bool)
	var result []model.Edge
	for _, e := range g.Edges {
		if e.Type != "import" {
			continue
		}
		if o.isPreserved(e.From) || o.isPreserved(e.To) {
			continue
		}
		key := e.From + "\x00" + e.To
		if seen[key] {
			continue
		}
		seen[key] = true
		result = append(result, e)
	}
	return result
}

// isPreserved returns true if the package matches any preserved prefix.
func (o *Optimizer) isPreserved(pkg string) bool {
	for prefix := range o.preservedPkgs {
		if strings.HasPrefix(pkg, prefix) || pkg == strings.TrimSuffix(prefix, "/") {
			return true
		}
	}
	return false
}

// simulateRemoval removes edge from g, recomputes metrics, and scores the impact.
func (o *Optimizer) simulateRemoval(g *model.Graph, edge model.Edge, baseline Metrics) Suggestion {
	// Build a temporary edge list without the target edge.
	simEdges := make([]model.Edge, 0, len(g.Edges)-1)
	for _, e := range g.Edges {
		if e.From == edge.From && e.To == edge.To && e.Type == edge.Type {
			continue
		}
		simEdges = append(simEdges, e)
	}
	simGraph := &model.Graph{Nodes: g.Nodes, Edges: simEdges}

	after := ComputeMetrics(simGraph)

	score := 0

	// Max fan-out: lower is better.
	if o.targets.MaxFanOut != nil {
		beforeSatisfied := o.targets.MaxFanOut.Satisfied(float64(baseline.MaxFanOut))
		afterSatisfied := o.targets.MaxFanOut.Satisfied(float64(after.MaxFanOut))
		if !beforeSatisfied && afterSatisfied {
			score += 2 // newly satisfied
		} else if !beforeSatisfied && after.MaxFanOut < baseline.MaxFanOut {
			score++ // moved closer
		}
	}

	// Spanning-tree coverage: higher is better.
	if o.targets.SpanningTreeCoverage != nil {
		beforeSatisfied := o.targets.SpanningTreeCoverage.Satisfied(baseline.SpanningTreeCoverage)
		afterSatisfied := o.targets.SpanningTreeCoverage.Satisfied(after.SpanningTreeCoverage)
		if !beforeSatisfied && afterSatisfied {
			score += 2
		} else if !beforeSatisfied && after.SpanningTreeCoverage > baseline.SpanningTreeCoverage {
			score++
		}
	}

	reason := buildReason(edge, baseline, after)

	return Suggestion{
		From:          edge.From,
		To:            edge.To,
		ImpactScore:   score,
		FanOutDelta:   after.MaxFanOut - baseline.MaxFanOut,
		CoverageDelta: after.SpanningTreeCoverage - baseline.SpanningTreeCoverage,
		Reason:        reason,
	}
}

// buildReason produces a human-readable explanation for the suggestion.
func buildReason(edge model.Edge, before, after Metrics) string {
	parts := []string{}
	if after.MaxFanOut < before.MaxFanOut {
		parts = append(parts, fmt.Sprintf(
			"%s imports too many packages (fan_out reduced by %d)",
			shortPkg(edge.From), before.MaxFanOut-after.MaxFanOut,
		))
	}
	if after.SpanningTreeCoverage > before.SpanningTreeCoverage {
		parts = append(parts, "removes a redundant dependency path")
	}
	if len(parts) == 0 {
		parts = append(parts, fmt.Sprintf("removing %s -> %s reduces complexity", shortPkg(edge.From), shortPkg(edge.To)))
	}
	return strings.Join(parts, "; ")
}

func shortPkg(pkg string) string {
	parts := strings.Split(pkg, "/")
	if len(parts) <= 2 {
		return pkg
	}
	return strings.Join(parts[len(parts)-2:], "/")
}
