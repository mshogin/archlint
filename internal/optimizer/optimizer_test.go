package optimizer_test

import (
	"testing"

	"github.com/mshogin/archlint/internal/model"
	"github.com/mshogin/archlint/internal/optimizer"
)

// ---- helpers ---------------------------------------------------------------

func importEdge(from, to string) model.Edge {
	return model.Edge{From: from, To: to, Type: "import"}
}

func pkgNode(id string) model.Node {
	return model.Node{ID: id, Title: id, Entity: "package"}
}

// buildGraph constructs a minimal graph with the given import edges.
func buildGraph(edges ...model.Edge) *model.Graph {
	nodeSet := make(map[string]bool)
	for _, e := range edges {
		nodeSet[e.From] = true
		nodeSet[e.To] = true
	}
	nodes := make([]model.Node, 0, len(nodeSet))
	for id := range nodeSet {
		nodes = append(nodes, pkgNode(id))
	}
	return &model.Graph{Nodes: nodes, Edges: edges}
}

// ---- ParseConstraint -------------------------------------------------------

func TestParseConstraint_LessEqual(t *testing.T) {
	c, err := optimizer.ParseConstraint("<= 7")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Op != "<=" || c.Value != 7 {
		t.Fatalf("got op=%s val=%v", c.Op, c.Value)
	}
}

func TestParseConstraint_GreaterEqual(t *testing.T) {
	c, err := optimizer.ParseConstraint(">= 0.4")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.Op != ">=" || c.Value != 0.4 {
		t.Fatalf("got op=%s val=%v", c.Op, c.Value)
	}
}

func TestParseConstraint_Invalid(t *testing.T) {
	_, err := optimizer.ParseConstraint("7")
	if err == nil {
		t.Fatal("expected error for constraint without operator")
	}
}

// ---- Constraint.Satisfied --------------------------------------------------

func TestConstraint_Satisfied(t *testing.T) {
	tests := []struct {
		expr  string
		value float64
		want  bool
	}{
		{"<= 7", 7, true},
		{"<= 7", 8, false},
		{">= 0.4", 0.4, true},
		{">= 0.4", 0.3, false},
		{"< 5", 4, true},
		{"< 5", 5, false},
		{"> 0.3", 0.4, true},
		{"> 0.3", 0.3, false},
	}
	for _, tt := range tests {
		c, err := optimizer.ParseConstraint(tt.expr)
		if err != nil {
			t.Fatalf("parse %q: %v", tt.expr, err)
		}
		if got := c.Satisfied(tt.value); got != tt.want {
			t.Errorf("constraint %q .Satisfied(%v) = %v, want %v", tt.expr, tt.value, got, tt.want)
		}
	}
}

// ---- ComputeMetrics --------------------------------------------------------

func TestComputeMetrics_MaxFanOut(t *testing.T) {
	// handler imports service and repo (fan_out=2), service imports repo (fan_out=1).
	g := buildGraph(
		importEdge("handler", "service"),
		importEdge("handler", "repo"),
		importEdge("service", "repo"),
	)
	m := optimizer.ComputeMetrics(g)
	if m.MaxFanOut != 2 {
		t.Errorf("MaxFanOut = %d, want 2", m.MaxFanOut)
	}
}

func TestComputeMetrics_EmptyGraph(t *testing.T) {
	g := &model.Graph{}
	m := optimizer.ComputeMetrics(g)
	if m.MaxFanOut != 0 {
		t.Errorf("MaxFanOut = %d, want 0", m.MaxFanOut)
	}
	if m.SpanningTreeCoverage != 1.0 {
		t.Errorf("SpanningTreeCoverage = %v, want 1.0", m.SpanningTreeCoverage)
	}
}

func TestComputeMetrics_SpanningTreeCoverage_Linear(t *testing.T) {
	// Linear chain: A -> B -> C. All edges are tree edges. Coverage = 1.0.
	g := buildGraph(
		importEdge("A", "B"),
		importEdge("B", "C"),
	)
	m := optimizer.ComputeMetrics(g)
	if m.SpanningTreeCoverage != 1.0 {
		t.Errorf("SpanningTreeCoverage = %v, want 1.0 for linear chain", m.SpanningTreeCoverage)
	}
}

func TestComputeMetrics_SpanningTreeCoverage_Redundant(t *testing.T) {
	// A -> B, A -> C, B -> C (redundant path A-B-C and direct A-C).
	// Tree: A->B and A->C are tree edges; B->C is a back edge.
	// Coverage = 2/3 ~= 0.67
	g := buildGraph(
		importEdge("A", "B"),
		importEdge("A", "C"),
		importEdge("B", "C"),
	)
	m := optimizer.ComputeMetrics(g)
	if m.SpanningTreeCoverage >= 1.0 {
		t.Errorf("SpanningTreeCoverage = %v, want < 1.0 for graph with redundant edge", m.SpanningTreeCoverage)
	}
}

// ---- Optimizer.Optimize ----------------------------------------------------

func TestOptimizer_SuggestsRemovalToHitFanOutTarget(t *testing.T) {
	// handler has fan_out=3 (imports service, repo, util).
	// Target: max_fan_out <= 2.
	// Removing any handler import should be suggested.
	g := buildGraph(
		importEdge("handler", "service"),
		importEdge("handler", "repo"),
		importEdge("handler", "util"),
		importEdge("service", "repo"),
	)

	maxFO, _ := optimizer.ParseConstraint("<= 2")
	targets := optimizer.Targets{MaxFanOut: maxFO}
	opt := optimizer.New(targets, nil, 10)
	suggestions := opt.Optimize(g)

	if len(suggestions) == 0 {
		t.Fatal("expected at least one suggestion, got none")
	}
	for _, s := range suggestions {
		if s.ImpactScore <= 0 {
			t.Errorf("suggestion %s->%s has non-positive impact score %d", s.From, s.To, s.ImpactScore)
		}
	}
}

func TestOptimizer_NoSuggestionsWhenTargetAlreadySatisfied(t *testing.T) {
	// handler fan_out=1. Target: max_fan_out <= 5.  Already satisfied.
	g := buildGraph(
		importEdge("handler", "service"),
	)
	maxFO, _ := optimizer.ParseConstraint("<= 5")
	targets := optimizer.Targets{MaxFanOut: maxFO}
	opt := optimizer.New(targets, nil, 10)
	suggestions := opt.Optimize(g)

	if len(suggestions) != 0 {
		t.Errorf("expected no suggestions when targets are already met, got %d", len(suggestions))
	}
}

func TestOptimizer_PreservedComponentsSkipped(t *testing.T) {
	// handler -> repo violates fan-out, but repo is preserved.
	g := buildGraph(
		importEdge("handler", "service"),
		importEdge("handler", "repo"),
		importEdge("handler", "util"),
	)
	maxFO, _ := optimizer.ParseConstraint("<= 1")
	targets := optimizer.Targets{MaxFanOut: maxFO}
	opt := optimizer.New(targets, []string{"repo"}, 10)
	suggestions := opt.Optimize(g)

	for _, s := range suggestions {
		if s.To == "repo" || s.From == "repo" {
			t.Errorf("suggestion involves preserved package 'repo': %s -> %s", s.From, s.To)
		}
	}
}

func TestOptimizer_RankedByImpactDesc(t *testing.T) {
	// Build a graph where removing one edge has higher impact than another.
	// A -> B (fan_out=3), A -> C, A -> D, B -> C.
	// Target: max_fan_out <= 2.
	g := buildGraph(
		importEdge("A", "B"),
		importEdge("A", "C"),
		importEdge("A", "D"),
		importEdge("B", "C"),
	)
	maxFO, _ := optimizer.ParseConstraint("<= 2")
	targets := optimizer.Targets{MaxFanOut: maxFO}
	opt := optimizer.New(targets, nil, 10)
	suggestions := opt.Optimize(g)

	for i := 1; i < len(suggestions); i++ {
		if suggestions[i].ImpactScore > suggestions[i-1].ImpactScore {
			t.Errorf("suggestions not sorted by impact: [%d].Score=%d > [%d].Score=%d",
				i, suggestions[i].ImpactScore, i-1, suggestions[i-1].ImpactScore)
		}
	}
}

func TestOptimizer_TopNLimit(t *testing.T) {
	// Create many edges from one node; limit to top 2.
	edges := make([]model.Edge, 10)
	for i := 0; i < 10; i++ {
		edges[i] = importEdge("hub", "dep"+string(rune('a'+i)))
	}
	g := buildGraph(edges...)

	maxFO, _ := optimizer.ParseConstraint("<= 1")
	targets := optimizer.Targets{MaxFanOut: maxFO}
	opt := optimizer.New(targets, nil, 2)
	suggestions := opt.Optimize(g)

	if len(suggestions) > 2 {
		t.Errorf("expected at most 2 suggestions (topN=2), got %d", len(suggestions))
	}
}
