package bpmn

import (
	"errors"
	"testing"
)

func TestBuildGraph_Adjacency(t *testing.T) {
	process, err := ParseFile("testdata/minimal.bpmn")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	graph, err := BuildGraph(process)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	successors := graph.Successors("Start_1")
	if len(successors) != 1 || successors[0] != "Task_1" {
		t.Errorf("expected Start_1 -> [Task_1], got %v", successors)
	}

	successors = graph.Successors("Task_1")
	if len(successors) != 1 || successors[0] != "End_1" {
		t.Errorf("expected Task_1 -> [End_1], got %v", successors)
	}

	successors = graph.Successors("End_1")
	if len(successors) != 0 {
		t.Errorf("expected End_1 -> [], got %v", successors)
	}
}

func TestBuildGraph_InDegree(t *testing.T) {
	process, err := ParseFile("testdata/minimal.bpmn")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	graph, err := BuildGraph(process)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if graph.InDegree["Start_1"] != 0 {
		t.Errorf("expected InDegree[Start_1] = 0, got %d", graph.InDegree["Start_1"])
	}

	if graph.InDegree["Task_1"] != 1 {
		t.Errorf("expected InDegree[Task_1] = 1, got %d", graph.InDegree["Task_1"])
	}

	if graph.InDegree["End_1"] != 1 {
		t.Errorf("expected InDegree[End_1] = 1, got %d", graph.InDegree["End_1"])
	}
}

func TestProcessGraph_Successors(t *testing.T) {
	process, err := ParseFile("testdata/gateway_exclusive.bpmn")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	graph, err := BuildGraph(process)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	successors := graph.Successors("Gateway_1")
	if len(successors) != 2 {
		t.Errorf("expected 2 successors for Gateway_1, got %d", len(successors))
	}

	targets := make(map[string]bool)
	for _, s := range successors {
		targets[s] = true
	}

	if !targets["Task_Process"] {
		t.Error("expected Task_Process as successor of Gateway_1")
	}

	if !targets["Task_Reject"] {
		t.Error("expected Task_Reject as successor of Gateway_1")
	}
}

func TestProcessGraph_Predecessors(t *testing.T) {
	process, err := ParseFile("testdata/gateway_parallel.bpmn")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	graph, err := BuildGraph(process)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	predecessors := graph.Predecessors("Join_1")
	if len(predecessors) != 2 {
		t.Errorf("expected 2 predecessors for Join_1, got %d", len(predecessors))
	}

	sources := make(map[string]bool)
	for _, p := range predecessors {
		sources[p] = true
	}

	if !sources["Task_A"] {
		t.Error("expected Task_A as predecessor of Join_1")
	}

	if !sources["Task_B"] {
		t.Error("expected Task_B as predecessor of Join_1")
	}
}

func TestProcessGraph_StartEndEvents(t *testing.T) {
	process, err := ParseFile("testdata/gateway_exclusive.bpmn")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	graph, err := BuildGraph(process)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	starts := graph.StartEvents()
	if len(starts) != 1 {
		t.Errorf("expected 1 start event, got %d", len(starts))
	}

	ends := graph.EndEvents()
	if len(ends) != 2 {
		t.Errorf("expected 2 end events, got %d", len(ends))
	}
}

func TestValidate_NoStartEvent(t *testing.T) {
	process, err := ParseFile("testdata/no_start.bpmn")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	graph, err := BuildGraph(process)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	errs := graph.Validate()

	found := false

	for _, e := range errs {
		if errors.Is(e, ErrNoStartEvent) {
			found = true

			break
		}
	}

	if !found {
		t.Error("expected validation error about missing startEvent")
	}
}

func TestValidate_NoEndEvent(t *testing.T) {
	process := &BPMNProcess{
		ID:   "test",
		Name: "Test",
		Elements: []BPMNElement{
			{ID: "s1", Type: StartEvent},
			{ID: "t1", Type: Task},
		},
		Flows: []BPMNFlow{
			{ID: "f1", SourceRef: "s1", TargetRef: "t1"},
		},
	}

	graph, err := BuildGraph(process)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	errs := graph.Validate()

	found := false

	for _, e := range errs {
		if errors.Is(e, ErrNoEndEvent) {
			found = true

			break
		}
	}

	if !found {
		t.Error("expected validation error about missing endEvent")
	}
}

func TestValidate_BrokenRef(t *testing.T) {
	process := &BPMNProcess{
		ID:   "test",
		Name: "Test",
		Elements: []BPMNElement{
			{ID: "s1", Type: StartEvent},
			{ID: "e1", Type: EndEvent},
		},
		Flows: []BPMNFlow{
			{ID: "f1", SourceRef: "s1", TargetRef: "nonexistent"},
		},
	}

	graph, err := BuildGraph(process)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	errs := graph.Validate()

	found := false

	for _, e := range errs {
		if errors.Is(e, ErrBrokenRef) {
			found = true

			break
		}
	}

	if !found {
		t.Error("expected validation error about broken ref")
	}
}

func TestValidate_ValidProcess(t *testing.T) {
	process, err := ParseFile("testdata/minimal.bpmn")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	graph, err := BuildGraph(process)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	errs := graph.Validate()
	if len(errs) != 0 {
		t.Errorf("expected no validation errors, got %d: %v", len(errs), errs)
	}
}

func TestBuildGraph_NilProcess(t *testing.T) {
	_, err := BuildGraph(nil)
	if err == nil {
		t.Error("expected error for nil process, got nil")
	}
}

