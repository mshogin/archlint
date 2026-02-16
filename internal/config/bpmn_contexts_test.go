package config

import (
	"errors"
	"testing"

	"github.com/mshogin/archlint/pkg/bpmn"
)

func TestLoadBPMNContexts_Valid(t *testing.T) {
	config, warnings, err := LoadBPMNContexts("testdata/valid.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d: %v", len(warnings), warnings)
	}

	if len(config.Contexts) != 2 {
		t.Fatalf("expected 2 contexts, got %d", len(config.Contexts))
	}

	orderCtx, ok := config.Contexts["order-processing"]
	if !ok {
		t.Fatal("expected context 'order-processing' not found")
	}

	if orderCtx.BPMNFile != "minimal.bpmn" {
		t.Errorf("expected bpmn_file 'minimal.bpmn', got %q", orderCtx.BPMNFile)
	}

	if len(orderCtx.Events) != 2 {
		t.Fatalf("expected 2 events in order-processing, got %d", len(orderCtx.Events))
	}

	if orderCtx.Events[0].EventID != "Start_1" {
		t.Errorf("expected event_id 'Start_1', got %q", orderCtx.Events[0].EventID)
	}

	if orderCtx.Events[0].EventName != "Create Order" {
		t.Errorf("expected event_name 'Create Order', got %q", orderCtx.Events[0].EventName)
	}

	if orderCtx.Events[0].EntryPoint.Package != "internal/api/handler" {
		t.Errorf("expected package 'internal/api/handler', got %q", orderCtx.Events[0].EntryPoint.Package)
	}

	if orderCtx.Events[0].EntryPoint.Function != "OrderHandler.Create" {
		t.Errorf("expected function 'OrderHandler.Create', got %q", orderCtx.Events[0].EntryPoint.Function)
	}

	if orderCtx.Events[0].EntryPoint.Type != EntryPointHTTP {
		t.Errorf("expected type 'http', got %q", orderCtx.Events[0].EntryPoint.Type)
	}
}

func TestLoadBPMNContexts_MultipleContexts(t *testing.T) {
	config, _, err := LoadBPMNContexts("testdata/valid.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, ok := config.Contexts["order-processing"]; !ok {
		t.Error("expected context 'order-processing' not found")
	}

	if _, ok := config.Contexts["reporting"]; !ok {
		t.Error("expected context 'reporting' not found")
	}

	reportingCtx := config.Contexts["reporting"]
	if reportingCtx.Events[0].EntryPoint.Type != EntryPointCron {
		t.Errorf("expected type 'cron' for reporting, got %q", reportingCtx.Events[0].EntryPoint.Type)
	}
}

func TestLoadBPMNContexts_EmptyContexts(t *testing.T) {
	_, _, err := LoadBPMNContexts("testdata/empty_contexts.yaml")
	if err == nil {
		t.Fatal("expected error for empty contexts, got nil")
	}

	if !errors.Is(err, ErrEmptyContexts) {
		t.Errorf("expected ErrEmptyContexts, got %v", err)
	}
}

func TestLoadBPMNContexts_MissingBPMNFile(t *testing.T) {
	_, _, err := LoadBPMNContexts("testdata/missing_bpmn_file.yaml")
	if err == nil {
		t.Fatal("expected error for missing bpmn_file, got nil")
	}

	if !errors.Is(err, ErrMissingBPMNFile) {
		t.Errorf("expected ErrMissingBPMNFile, got %v", err)
	}
}

func TestLoadBPMNContexts_EmptyEvents(t *testing.T) {
	_, _, err := LoadBPMNContexts("testdata/empty_events.yaml")
	if err == nil {
		t.Fatal("expected error for empty events, got nil")
	}

	if !errors.Is(err, ErrEmptyEvents) {
		t.Errorf("expected ErrEmptyEvents, got %v", err)
	}
}

func TestLoadBPMNContexts_MissingEventID(t *testing.T) {
	_, _, err := LoadBPMNContexts("testdata/missing_event_id.yaml")
	if err == nil {
		t.Fatal("expected error for missing event_id, got nil")
	}

	if !errors.Is(err, ErrMissingEventID) {
		t.Errorf("expected ErrMissingEventID, got %v", err)
	}
}

func TestLoadBPMNContexts_InvalidType(t *testing.T) {
	_, _, err := LoadBPMNContexts("testdata/invalid_type.yaml")
	if err == nil {
		t.Fatal("expected error for invalid type, got nil")
	}

	if !errors.Is(err, ErrInvalidType) {
		t.Errorf("expected ErrInvalidType, got %v", err)
	}
}

func TestLoadBPMNContexts_DuplicateEventID(t *testing.T) {
	_, _, err := LoadBPMNContexts("testdata/duplicate_event_id.yaml")
	if err == nil {
		t.Fatal("expected error for duplicate event_id, got nil")
	}

	if !errors.Is(err, ErrDuplicateEventID) {
		t.Errorf("expected ErrDuplicateEventID, got %v", err)
	}
}

func TestLoadBPMNContexts_NonexistentBPMN(t *testing.T) {
	_, _, err := LoadBPMNContexts("testdata/nonexistent_bpmn.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent bpmn file, got nil")
	}

	if !errors.Is(err, ErrBPMNFileNotFound) {
		t.Errorf("expected ErrBPMNFileNotFound, got %v", err)
	}
}

func TestLoadBPMNContexts_MissingPackage(t *testing.T) {
	_, _, err := LoadBPMNContexts("testdata/missing_package.yaml")
	if err == nil {
		t.Fatal("expected error for missing package, got nil")
	}

	if !errors.Is(err, ErrMissingPackage) {
		t.Errorf("expected ErrMissingPackage, got %v", err)
	}
}

func TestLoadBPMNContexts_MissingFunction(t *testing.T) {
	_, _, err := LoadBPMNContexts("testdata/missing_function.yaml")
	if err == nil {
		t.Fatal("expected error for missing function, got nil")
	}

	if !errors.Is(err, ErrMissingFunction) {
		t.Errorf("expected ErrMissingFunction, got %v", err)
	}
}

func TestLoadBPMNContexts_AllEntryPointTypes(t *testing.T) {
	config, _, err := LoadBPMNContexts("testdata/all_types.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := config.Contexts["all-types"]
	if len(ctx.Events) != 5 {
		t.Fatalf("expected 5 events, got %d", len(ctx.Events))
	}

	expectedTypes := []EntryPointType{
		EntryPointHTTP, EntryPointKafka, EntryPointGRPC, EntryPointCron, EntryPointCustom,
	}

	for i, expected := range expectedTypes {
		if ctx.Events[i].EntryPoint.Type != expected {
			t.Errorf("event #%d: expected type %q, got %q", i, expected, ctx.Events[i].EntryPoint.Type)
		}
	}
}

func TestLoadBPMNContexts_NonexistentFile(t *testing.T) {
	_, _, err := LoadBPMNContexts("testdata/nonexistent.yaml")
	if err == nil {
		t.Fatal("expected error for nonexistent config file, got nil")
	}

	if !errors.Is(err, ErrConfigRead) {
		t.Errorf("expected ErrConfigRead, got %v", err)
	}
}

func TestLoadBPMNContexts_InvalidYAML(t *testing.T) {
	_, _, err := LoadBPMNContexts("testdata/minimal.bpmn")
	if err == nil {
		t.Fatal("expected error for invalid YAML, got nil")
	}
}

func TestValidateAgainstBPMN_ValidEvents(t *testing.T) {
	ctx := &BPMNContext{
		BPMNFile: "minimal.bpmn",
		Events: []EventMapping{
			{EventID: "Start_1"},
			{EventID: "Task_1"},
			{EventID: "End_1"},
		},
	}

	process := &bpmn.BPMNProcess{
		Elements: []bpmn.BPMNElement{
			{ID: "Start_1", Type: bpmn.StartEvent},
			{ID: "Task_1", Type: bpmn.Task},
			{ID: "End_1", Type: bpmn.EndEvent},
		},
	}

	warnings := ValidateAgainstBPMN("test", ctx, process)
	if len(warnings) != 0 {
		t.Errorf("expected 0 warnings, got %d: %v", len(warnings), warnings)
	}
}

func TestValidateAgainstBPMN_MissingEvent(t *testing.T) {
	ctx := &BPMNContext{
		BPMNFile: "minimal.bpmn",
		Events: []EventMapping{
			{EventID: "Start_1"},
			{EventID: "NonExistent_1"},
		},
	}

	process := &bpmn.BPMNProcess{
		Elements: []bpmn.BPMNElement{
			{ID: "Start_1", Type: bpmn.StartEvent},
			{ID: "End_1", Type: bpmn.EndEvent},
		},
	}

	warnings := ValidateAgainstBPMN("test", ctx, process)
	if len(warnings) != 1 {
		t.Fatalf("expected 1 warning, got %d: %v", len(warnings), warnings)
	}

	if warnings[0].Context != "test" {
		t.Errorf("expected context 'test', got %q", warnings[0].Context)
	}
}

func TestWarning_String(t *testing.T) {
	w := Warning{Context: "order", Message: "event not found"}
	expected := "[order] event not found"

	if w.String() != expected {
		t.Errorf("expected %q, got %q", expected, w.String())
	}

	w2 := Warning{Message: "global warning"}
	if w2.String() != "global warning" {
		t.Errorf("expected 'global warning', got %q", w2.String())
	}
}
