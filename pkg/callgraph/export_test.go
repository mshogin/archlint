package callgraph

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

func TestYAMLExporter_MarshalCallGraph(t *testing.T) {
	cg := &CallGraph{
		EventID:    "evt-1",
		EventName:  "Test",
		EntryPoint: "pkg.Func",
		Nodes: []CallNode{
			{ID: "pkg.Func", Package: "pkg", Function: "Func", Type: NodeFunction, Depth: 0},
		},
		Edges:       []CallEdge{},
		MaxDepth:    10,
		ActualDepth: 0,
		Stats:       Stats{TotalNodes: 1},
	}

	exporter := NewYAMLExporter()
	data, err := exporter.MarshalCallGraph(cg)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed CallGraph
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("YAML not parseable: %v", err)
	}

	if parsed.EventID != "evt-1" {
		t.Errorf("expected event_id 'evt-1', got %q", parsed.EventID)
	}

	if len(parsed.Nodes) != 1 {
		t.Errorf("expected 1 node, got %d", len(parsed.Nodes))
	}
}

func TestYAMLExporter_MarshalEventSet(t *testing.T) {
	set := &EventCallGraphSet{
		ProcessID:   "test-process",
		Graphs:      map[string]CallGraph{"evt-1": {EntryPoint: "pkg.Func"}},
		GeneratedAt: time.Now(),
		Stats:       SetStats{TotalEvents: 1, BuiltGraphs: 1},
	}

	exporter := NewYAMLExporter()
	data, err := exporter.MarshalEventSet(set)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed EventCallGraphSet
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("YAML not parseable: %v", err)
	}

	if parsed.ProcessID != "test-process" {
		t.Errorf("expected process_id 'test-process', got %q", parsed.ProcessID)
	}
}

func TestYAMLExporter_ExportCallGraph(t *testing.T) {
	tmpDir := t.TempDir()

	cg := &CallGraph{
		EntryPoint: "pkg.Func",
		Nodes:      []CallNode{{ID: "pkg.Func", Function: "Func", Type: NodeFunction}},
	}

	exporter := NewYAMLExporter()
	path := filepath.Join(tmpDir, "test.yaml")

	if err := exporter.ExportCallGraph(cg, path); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("cannot read exported file: %v", err)
	}

	if len(data) == 0 {
		t.Error("exported file is empty")
	}

	var parsed CallGraph
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("exported YAML not parseable: %v", err)
	}

	if parsed.EntryPoint != "pkg.Func" {
		t.Errorf("expected entry_point 'pkg.Func', got %q", parsed.EntryPoint)
	}
}

func TestYAMLExporter_ReferentialIntegrity(t *testing.T) {
	cg := &CallGraph{
		EntryPoint: "a.A",
		Nodes: []CallNode{
			{ID: "a.A", Function: "A", Type: NodeFunction},
			{ID: "a.B", Function: "B", Type: NodeFunction},
		},
		Edges: []CallEdge{
			{From: "a.A", To: "a.B", CallType: CallDirect},
		},
	}

	exporter := NewYAMLExporter()
	data, err := exporter.MarshalCallGraph(cg)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var parsed CallGraph
	if err := yaml.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("YAML not parseable: %v", err)
	}

	nodeIDs := make(map[string]bool)
	for _, node := range parsed.Nodes {
		nodeIDs[node.ID] = true
	}

	for _, edge := range parsed.Edges {
		if !nodeIDs[edge.From] {
			t.Errorf("edge.From %q references nonexistent node", edge.From)
		}

		if !nodeIDs[edge.To] {
			t.Errorf("edge.To %q references nonexistent node", edge.To)
		}
	}
}
