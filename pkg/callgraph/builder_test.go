package callgraph

import (
	"errors"
	"testing"

	"github.com/mshogin/archlint/internal/analyzer"
)

const testdataDir = "testdata/sample"

func setupAnalyzer(t *testing.T) *analyzer.GoAnalyzer {
	t.Helper()

	a := analyzer.NewGoAnalyzer()

	if _, err := a.Analyze(testdataDir); err != nil {
		t.Fatalf("failed to analyze testdata: %v", err)
	}

	return a
}

func TestBuilder_SimpleChain(t *testing.T) {
	a := setupAnalyzer(t)

	builder, err := NewBuilder(a, BuildOptions{MaxDepth: 10, ResolveInterfaces: true, TrackGoroutines: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cg, err := builder.Build("testdata/sample.transform")
	if err != nil {
		t.Fatalf("unexpected error building graph: %v", err)
	}

	if cg.Stats.TotalNodes < 2 {
		t.Errorf("expected at least 2 nodes (transform->validate), got %d", cg.Stats.TotalNodes)
	}

	if cg.Stats.TotalEdges < 1 {
		t.Errorf("expected at least 1 edge, got %d", cg.Stats.TotalEdges)
	}

	if cg.EntryPoint != "testdata/sample.transform" {
		t.Errorf("expected entry point 'callgraph/testdata/sample.transform', got %q", cg.EntryPoint)
	}
}

func TestBuilder_MethodChain(t *testing.T) {
	a := setupAnalyzer(t)

	builder, err := NewBuilder(a, BuildOptions{MaxDepth: 10, ResolveInterfaces: true, TrackGoroutines: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cg, err := builder.Build("testdata/sample.Service.Process")
	if err != nil {
		t.Fatalf("unexpected error building graph: %v", err)
	}

	if cg.Stats.TotalNodes < 2 {
		t.Errorf("expected at least 2 nodes, got %d", cg.Stats.TotalNodes)
	}

	hasAsync := false
	for _, edge := range cg.Edges {
		if edge.Async {
			hasAsync = true

			break
		}
	}

	if !hasAsync {
		t.Error("expected at least one async edge (goroutine), but found none")
	}
}

func TestBuilder_CycleDetection(t *testing.T) {
	a := setupAnalyzer(t)

	builder, err := NewBuilder(a, BuildOptions{MaxDepth: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cg, err := builder.Build("testdata/sample.CycleA")
	if err != nil {
		t.Fatalf("unexpected error building graph: %v", err)
	}

	if cg.Stats.CyclesDetected == 0 {
		t.Error("expected at least 1 cycle detected, got 0")
	}

	hasCycleEdge := false
	for _, edge := range cg.Edges {
		if edge.Cycle {
			hasCycleEdge = true

			break
		}
	}

	if !hasCycleEdge {
		t.Error("expected at least one edge with Cycle=true")
	}
}

func TestBuilder_MaxDepth(t *testing.T) {
	a := setupAnalyzer(t)

	builder, err := NewBuilder(a, BuildOptions{MaxDepth: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cg, err := builder.Build("testdata/sample.transform")
	if err != nil {
		t.Fatalf("unexpected error building graph: %v", err)
	}

	if cg.MaxDepth != 1 {
		t.Errorf("expected MaxDepth=1, got %d", cg.MaxDepth)
	}

	if cg.ActualDepth > 1 {
		t.Errorf("expected ActualDepth<=1, got %d", cg.ActualDepth)
	}
}

func TestBuilder_GoroutineDetection(t *testing.T) {
	a := setupAnalyzer(t)

	builder, err := NewBuilder(a, BuildOptions{MaxDepth: 10, TrackGoroutines: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cg, err := builder.Build("testdata/sample.Service.Process")
	if err != nil {
		t.Fatalf("unexpected error building graph: %v", err)
	}

	if cg.Stats.GoroutineCalls == 0 {
		t.Error("expected at least 1 goroutine call, got 0")
	}
}

func TestBuilder_DeferredCall(t *testing.T) {
	a := setupAnalyzer(t)

	builder, err := NewBuilder(a, BuildOptions{MaxDepth: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cg, err := builder.Build("testdata/sample.Service.ProcessWithDefer")
	if err != nil {
		t.Fatalf("unexpected error building graph: %v", err)
	}

	hasDeferred := false
	for _, edge := range cg.Edges {
		if edge.CallType == CallDeferred {
			hasDeferred = true

			break
		}
	}

	if !hasDeferred {
		t.Error("expected at least one deferred edge, but found none")
	}
}

func TestBuilder_EntryPointNotFound(t *testing.T) {
	a := setupAnalyzer(t)

	builder, err := NewBuilder(a, BuildOptions{MaxDepth: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = builder.Build("nonexistent.Function")
	if err == nil {
		t.Fatal("expected error for nonexistent entry point, got nil")
	}

	if !errors.Is(err, ErrEntryPointNotFound) {
		t.Errorf("expected ErrEntryPointNotFound, got %v", err)
	}
}

func TestBuilder_EmptyFunction(t *testing.T) {
	a := setupAnalyzer(t)

	builder, err := NewBuilder(a, BuildOptions{MaxDepth: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cg, err := builder.Build("testdata/sample.EmptyFunc")
	if err != nil {
		t.Fatalf("unexpected error building graph: %v", err)
	}

	if cg.Stats.TotalNodes != 1 {
		t.Errorf("expected 1 node for empty function, got %d", cg.Stats.TotalNodes)
	}

	if cg.Stats.TotalEdges != 0 {
		t.Errorf("expected 0 edges for empty function, got %d", cg.Stats.TotalEdges)
	}
}

func TestBuilder_ForEvent(t *testing.T) {
	a := setupAnalyzer(t)

	builder, err := NewBuilder(a, BuildOptions{MaxDepth: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cg, err := builder.BuildForEvent("evt-1", "Test Event", "testdata/sample.EmptyFunc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cg.EventID != "evt-1" {
		t.Errorf("expected EventID 'evt-1', got %q", cg.EventID)
	}

	if cg.EventName != "Test Event" {
		t.Errorf("expected EventName 'Test Event', got %q", cg.EventName)
	}
}

func TestBuilder_InvalidMaxDepth(t *testing.T) {
	a := setupAnalyzer(t)

	_, err := NewBuilder(a, BuildOptions{MaxDepth: 0})
	if err == nil {
		t.Fatal("expected error for MaxDepth=0")
	}

	if !errors.Is(err, ErrInvalidMaxDepth) {
		t.Errorf("expected ErrInvalidMaxDepth, got %v", err)
	}

	_, err = NewBuilder(a, BuildOptions{MaxDepth: 51})
	if err == nil {
		t.Fatal("expected error for MaxDepth=51")
	}
}

func TestBuilder_NilAnalyzer(t *testing.T) {
	_, err := NewBuilder(nil, BuildOptions{MaxDepth: 10})
	if err == nil {
		t.Fatal("expected error for nil analyzer")
	}

	if !errors.Is(err, ErrAnalyzerRequired) {
		t.Errorf("expected ErrAnalyzerRequired, got %v", err)
	}
}

func TestBuilder_NodeIDsUnique(t *testing.T) {
	a := setupAnalyzer(t)

	builder, err := NewBuilder(a, BuildOptions{MaxDepth: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cg, err := builder.Build("testdata/sample.Service.Process")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	seen := make(map[string]bool)
	for _, node := range cg.Nodes {
		if seen[node.ID] {
			t.Errorf("duplicate node ID: %s", node.ID)
		}

		seen[node.ID] = true
	}
}

func TestBuilder_EdgeReferencesValid(t *testing.T) {
	a := setupAnalyzer(t)

	builder, err := NewBuilder(a, BuildOptions{MaxDepth: 10})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cg, err := builder.Build("testdata/sample.Service.Process")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	nodeIDs := make(map[string]bool)
	for _, node := range cg.Nodes {
		nodeIDs[node.ID] = true
	}

	for _, edge := range cg.Edges {
		if !nodeIDs[edge.From] {
			t.Errorf("edge.From %q references nonexistent node", edge.From)
		}

		if !nodeIDs[edge.To] {
			t.Errorf("edge.To %q references nonexistent node", edge.To)
		}
	}
}
