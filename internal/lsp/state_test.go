package lsp

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
		t.Error("состояние не должно быть инициализированным до Initialize()")
	}

	if err := state.Initialize(tmpDir); err != nil {
		t.Fatalf("ошибка инициализации: %v", err)
	}

	if !state.IsInitialized() {
		t.Error("состояние должно быть инициализированным после Initialize()")
	}

	graph := state.GetGraph()
	if graph == nil {
		t.Fatal("граф не должен быть nil")
	}

	if len(graph.Nodes) == 0 {
		t.Error("граф должен содержать узлы")
	}

	stats := state.Stats()
	if stats.TotalNodes == 0 {
		t.Error("статистика должна показывать узлы")
	}

	if stats.Packages == 0 {
		t.Error("статистика должна показывать пакеты")
	}
}

func TestStateReparseFile(t *testing.T) {
	tmpDir := t.TempDir()
	mainFile := filepath.Join(tmpDir, "main.go")

	if err := os.WriteFile(mainFile, []byte(`package main

func main() {}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	state := NewState()

	if err := state.Initialize(tmpDir); err != nil {
		t.Fatalf("ошибка инициализации: %v", err)
	}

	statsBefore := state.Stats()

	// Добавляем новый тип.
	if err := os.WriteFile(mainFile, []byte(`package main

type NewType struct{}

func main() {}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := state.ReparseFile(mainFile); err != nil {
		t.Fatalf("ошибка перепарсинга: %v", err)
	}

	statsAfter := state.Stats()

	if statsAfter.Structs <= statsBefore.Structs {
		t.Error("после добавления типа количество структур должно увеличиться")
	}
}

func TestStateFileVersions(t *testing.T) {
	state := NewState()

	uri := "file:///tmp/test.go"

	if v := state.GetFileVersion(uri); v != 0 {
		t.Errorf("ожидалась версия 0 для нового файла, получено %d", v)
	}

	state.SetFileVersion(uri, 5)

	if v := state.GetFileVersion(uri); v != 5 {
		t.Errorf("ожидалась версия 5, получено %d", v)
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
		t.Fatalf("ошибка инициализации: %v", err)
	}

	if state.RootDir() != tmpDir {
		t.Errorf("ожидался rootDir=%s, получено %s", tmpDir, state.RootDir())
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
		t.Fatalf("ошибка инициализации: %v", err)
	}

	graph1 := state.GetGraph()
	graph2 := state.GetGraph()

	// Изменяем graph1 — не должно влиять на graph2.
	if len(graph1.Nodes) > 0 {
		graph1.Nodes[0].ID = "modified"

		if graph2.Nodes[0].ID == "modified" {
			t.Error("GetGraph() должен возвращать копию, а не ссылку")
		}
	}
}
