package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStateInitialize(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(`package main

type App struct {
	Name string
}

func main() {}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	state := NewState()

	if state.IsInitialized() {
		t.Error("state should not be initialized before Initialize()")
	}

	if err := state.Initialize(tmpDir); err != nil {
		t.Fatalf("initialization error: %v", err)
	}

	if !state.IsInitialized() {
		t.Error("state should be initialized after Initialize()")
	}

	graph := state.GetGraph()
	if graph == nil {
		t.Fatal("graph should not be nil")
	}

	if len(graph.Nodes) == 0 {
		t.Error("graph should contain nodes")
	}

	stats := state.Stats()
	if stats.TotalNodes == 0 {
		t.Error("stats should show nodes")
	}

	if stats.Packages == 0 {
		t.Error("stats should show packages")
	}
}

func TestStateReparse(t *testing.T) {
	tmpDir := t.TempDir()
	mainFile := filepath.Join(tmpDir, "main.go")

	if err := os.WriteFile(mainFile, []byte(`package main

func main() {}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	state := NewState()

	if err := state.Initialize(tmpDir); err != nil {
		t.Fatalf("initialization error: %v", err)
	}

	statsBefore := state.Stats()

	// Add a new type.
	if err := os.WriteFile(mainFile, []byte(`package main

type NewType struct{}

func main() {}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := state.Reparse(); err != nil {
		t.Fatalf("reparse error: %v", err)
	}

	statsAfter := state.Stats()

	if statsAfter.Structs <= statsBefore.Structs {
		t.Error("struct count should increase after adding a type")
	}
}

func TestStateRootDir(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(`package main
func main() {}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	state := NewState()

	if err := state.Initialize(tmpDir); err != nil {
		t.Fatalf("initialization error: %v", err)
	}

	if state.RootDir() != tmpDir {
		t.Errorf("expected rootDir=%s, got %s", tmpDir, state.RootDir())
	}
}

func TestStateGetGraphReturnsACopy(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(`package main
func main() {}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	state := NewState()

	if err := state.Initialize(tmpDir); err != nil {
		t.Fatalf("initialization error: %v", err)
	}

	graph1 := state.GetGraph()
	graph2 := state.GetGraph()

	if len(graph1.Nodes) > 0 {
		graph1.Nodes[0].ID = "modified"

		if graph2.Nodes[0].ID == "modified" {
			t.Error("GetGraph() should return a copy, not a reference")
		}
	}
}
