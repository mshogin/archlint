package bpmn

import (
	"testing"
)

func TestParse_MinimalProcess(t *testing.T) {
	process, err := ParseFile("testdata/minimal.bpmn")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if process.ID != "Process_1" {
		t.Errorf("expected process ID %q, got %q", "Process_1", process.ID)
	}

	if process.Name != "Minimal Process" {
		t.Errorf("expected process name %q, got %q", "Minimal Process", process.Name)
	}

	if len(process.Elements) != 3 {
		t.Fatalf("expected 3 elements, got %d", len(process.Elements))
	}

	if len(process.Flows) != 2 {
		t.Fatalf("expected 2 flows, got %d", len(process.Flows))
	}

	elementTypes := make(map[ElementType]int)
	for _, e := range process.Elements {
		elementTypes[e.Type]++
	}

	if elementTypes[StartEvent] != 1 {
		t.Errorf("expected 1 startEvent, got %d", elementTypes[StartEvent])
	}

	if elementTypes[Task] != 1 {
		t.Errorf("expected 1 task, got %d", elementTypes[Task])
	}

	if elementTypes[EndEvent] != 1 {
		t.Errorf("expected 1 endEvent, got %d", elementTypes[EndEvent])
	}
}

func TestParse_AllElementTypes(t *testing.T) {
	process, err := ParseFile("testdata/gateway_exclusive.bpmn")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	elementTypes := make(map[ElementType]int)
	for _, e := range process.Elements {
		elementTypes[e.Type]++
	}

	want := map[ElementType]int{
		StartEvent:       1,
		EndEvent:         2,
		ServiceTask:      2,
		UserTask:         1,
		ExclusiveGateway: 1,
	}

	for typ, count := range want {
		if elementTypes[typ] != count {
			t.Errorf("expected %d %s, got %d", count, typ, elementTypes[typ])
		}
	}
}

func TestParse_EventSubtypes(t *testing.T) {
	process, err := ParseFile("testdata/intermediate_events.bpmn")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	eventTypes := make(map[string]EventType)
	for _, e := range process.Elements {
		eventTypes[e.ID] = e.EventType
	}

	tests := []struct {
		id       string
		wantType EventType
	}{
		{"Start_1", EventTimer},
		{"Catch_1", EventSignal},
		{"Throw_1", EventError},
		{"End_1", EventNone},
	}

	for _, tt := range tests {
		got := eventTypes[tt.id]
		if got != tt.wantType {
			t.Errorf("element %s: expected event type %q, got %q", tt.id, tt.wantType, got)
		}
	}
}

func TestParse_MessageEventDefinition(t *testing.T) {
	process, err := ParseFile("testdata/gateway_exclusive.bpmn")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, e := range process.Elements {
		if e.ID == "StartEvent_1" {
			if e.EventType != EventMessage {
				t.Errorf("expected message event type for StartEvent_1, got %q", e.EventType)
			}

			return
		}
	}

	t.Error("StartEvent_1 not found in elements")
}

func TestParse_MultipleFlows(t *testing.T) {
	process, err := ParseFile("testdata/gateway_exclusive.bpmn")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(process.Flows) != 6 {
		t.Fatalf("expected 6 flows, got %d", len(process.Flows))
	}

	hasNamedFlow := false

	for _, f := range process.Flows {
		if f.Name == "Valid" {
			hasNamedFlow = true

			if f.SourceRef != "Gateway_1" {
				t.Errorf("expected flow %q source %q, got %q", f.Name, "Gateway_1", f.SourceRef)
			}

			if f.TargetRef != "Task_Process" {
				t.Errorf("expected flow %q target %q, got %q", f.Name, "Task_Process", f.TargetRef)
			}
		}
	}

	if !hasNamedFlow {
		t.Error("expected named flow 'Valid' not found")
	}
}

func TestParse_InvalidXML(t *testing.T) {
	_, err := Parse([]byte("not xml at all"))
	if err == nil {
		t.Error("expected error for invalid XML, got nil")
	}
}

func TestParse_EmptyProcess(t *testing.T) {
	process, err := ParseFile("testdata/empty_process.bpmn")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(process.Elements) != 0 {
		t.Errorf("expected 0 elements, got %d", len(process.Elements))
	}

	if len(process.Flows) != 0 {
		t.Errorf("expected 0 flows, got %d", len(process.Flows))
	}
}

func TestParse_NoProcesses(t *testing.T) {
	xml := `<?xml version="1.0" encoding="UTF-8"?>
<definitions xmlns="http://www.omg.org/spec/BPMN/20100524/MODEL"
             id="Definitions_1"
             targetNamespace="http://example.com/bpmn">
</definitions>`

	_, err := Parse([]byte(xml))
	if err == nil {
		t.Error("expected error for definitions without processes, got nil")
	}
}

func TestParse_ParallelGateway(t *testing.T) {
	process, err := ParseFile("testdata/gateway_parallel.bpmn")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	elementTypes := make(map[ElementType]int)
	for _, e := range process.Elements {
		elementTypes[e.Type]++
	}

	if elementTypes[ParallelGateway] != 2 {
		t.Errorf("expected 2 parallelGateway, got %d", elementTypes[ParallelGateway])
	}

	if elementTypes[ServiceTask] != 2 {
		t.Errorf("expected 2 serviceTask, got %d", elementTypes[ServiceTask])
	}

	if len(process.Flows) != 6 {
		t.Errorf("expected 6 flows, got %d", len(process.Flows))
	}
}

func TestParseFile_NonExistent(t *testing.T) {
	_, err := ParseFile("testdata/nonexistent.bpmn")
	if err == nil {
		t.Error("expected error for non-existent file, got nil")
	}
}
