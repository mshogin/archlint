package callgraph

import (
	"strings"
	"testing"
)

func TestSequenceGenerator_BasicDiagram(t *testing.T) {
	cg := &CallGraph{
		EventName:  "TestEvent",
		EntryPoint: "pkg.Handler.Process",
		Nodes: []CallNode{
			{ID: "pkg.Handler.Process", Package: "pkg", Function: "Process", Receiver: "Handler", Type: NodeMethod, Depth: 0},
			{ID: "pkg.Service.Run", Package: "pkg", Function: "Run", Receiver: "Service", Type: NodeMethod, Depth: 1},
		},
		Edges: []CallEdge{
			{From: "pkg.Handler.Process", To: "pkg.Service.Run", CallType: CallDirect},
		},
	}

	gen := NewSequenceGenerator(SequenceOptions{
		MaxDepth:     5,
		ShowPackages: true,
		MarkAsync:    true,
	})

	puml, err := gen.Generate(cg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(puml, "@startuml") {
		t.Error("expected @startuml in output")
	}

	if !strings.Contains(puml, "@enduml") {
		t.Error("expected @enduml in output")
	}

	if !strings.Contains(puml, "title") {
		t.Error("expected title in output")
	}
}

func TestSequenceGenerator_AsyncCalls(t *testing.T) {
	cg := &CallGraph{
		Nodes: []CallNode{
			{ID: "a.Func", Package: "a", Function: "Func", Type: NodeFunction, Depth: 0},
			{ID: "a.AsyncFunc", Package: "a", Function: "AsyncFunc", Type: NodeFunction, Depth: 1},
		},
		Edges: []CallEdge{
			{From: "a.Func", To: "a.AsyncFunc", CallType: CallGoroutine, Async: true},
		},
	}

	gen := NewSequenceGenerator(SequenceOptions{MaxDepth: 5, MarkAsync: true})
	puml, err := gen.Generate(cg)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(puml, "->>") {
		t.Error("expected async arrow (->>) in output")
	}

	if !strings.Contains(puml, "**async**") {
		t.Error("expected async label in output")
	}
}

func TestSequenceGenerator_InterfaceCalls(t *testing.T) {
	cg := &CallGraph{
		Nodes: []CallNode{
			{ID: "a.Func", Package: "a", Function: "Func", Type: NodeFunction, Depth: 0},
			{ID: "a.Repo.Save", Package: "a", Function: "Save", Receiver: "Repo", Type: NodeInterfaceMethod, Depth: 1},
		},
		Edges: []CallEdge{
			{From: "a.Func", To: "a.Repo.Save", CallType: CallInterface},
		},
	}

	gen := NewSequenceGenerator(SequenceOptions{MaxDepth: 5, MarkInterface: true})
	puml, err := gen.Generate(cg)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(puml, "<<interface>>") {
		t.Error("expected <<interface>> marker in output")
	}
}

func TestSequenceGenerator_MaxDepthTruncation(t *testing.T) {
	cg := &CallGraph{
		Nodes: []CallNode{
			{ID: "a.A", Package: "a", Function: "A", Type: NodeFunction, Depth: 0},
			{ID: "a.B", Package: "a", Function: "B", Type: NodeFunction, Depth: 1},
			{ID: "a.C", Package: "a", Function: "C", Type: NodeFunction, Depth: 2},
			{ID: "a.D", Package: "a", Function: "D", Type: NodeFunction, Depth: 3},
		},
		Edges: []CallEdge{
			{From: "a.A", To: "a.B", CallType: CallDirect},
			{From: "a.B", To: "a.C", CallType: CallDirect},
			{From: "a.C", To: "a.D", CallType: CallDirect},
		},
	}

	gen := NewSequenceGenerator(SequenceOptions{MaxDepth: 1})
	puml, err := gen.Generate(cg)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if strings.Contains(puml, "a_D") {
		t.Error("expected D (depth 3) to be excluded at MaxDepth=1")
	}
}

func TestSequenceGenerator_EmptyGraph(t *testing.T) {
	gen := NewSequenceGenerator(DefaultSequenceOptions())
	puml, err := gen.Generate(nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if puml != "" {
		t.Errorf("expected empty string for nil graph, got %q", puml)
	}
}

func TestSequenceGenerator_GroupByPackage(t *testing.T) {
	cg := &CallGraph{
		Nodes: []CallNode{
			{ID: "handler.H", Package: "handler", Function: "H", Type: NodeFunction, Depth: 0},
			{ID: "service.S", Package: "service", Function: "S", Type: NodeFunction, Depth: 1},
		},
		Edges: []CallEdge{
			{From: "handler.H", To: "service.S", CallType: CallDirect},
		},
	}

	gen := NewSequenceGenerator(SequenceOptions{MaxDepth: 5, GroupByPackage: true})
	puml, err := gen.Generate(cg)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(puml, "box") {
		t.Error("expected box grouping in output")
	}

	if !strings.Contains(puml, "end box") {
		t.Error("expected end box in output")
	}
}
