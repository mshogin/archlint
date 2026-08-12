package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

// callgraphFixture: main -> helper -> leaf. Обе стороны нужны в одном графе,
// чтобы проверить, что направления не путаются местами.
func callgraphFixture(t *testing.T) (*Server, map[string]string) {
	t.Helper()

	tmpDir := t.TempDir()
	src := `package main

func leaf() {}

func helper() {
	leaf()
}

func main() {
	helper()
}
`

	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	server := createInitializedServer(t, tmpDir)

	ids := make(map[string]string)

	for _, node := range server.state.GetGraph().Nodes {
		if node.Entity == "function" {
			ids[node.Title] = node.ID
		}
	}

	for _, name := range []string{"main", "helper", "leaf"} {
		if ids[name] == "" {
			t.Fatalf("функция %q не найдена в графе", name)
		}
	}

	return server, ids
}

func callgraph(t *testing.T, server *Server, entry, direction string) *CallGraphResult {
	t.Helper()

	args := map[string]any{"entry": entry, "max_depth": 5}
	if direction != "" {
		args["direction"] = direction
	}

	raw, err := json.Marshal(args)
	if err != nil {
		t.Fatal(err)
	}

	res, err := handleGetCallgraph(server.state, raw)
	if err != nil {
		t.Fatalf("handleGetCallgraph(%s): %v", direction, err)
	}

	return res
}

func containsID(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}

	return false
}

func neighbours(res *CallGraphResult, id string) ([]string, bool) {
	for _, n := range res.Nodes {
		if n.ID == id {
			if len(n.CalledBy) > 0 {
				return n.CalledBy, true
			}

			return n.CallsTo, true
		}
	}

	return nil, false
}

// Главное свойство: direction=callers отвечает на вопрос «кого сломает правка
// внутри helper», то есть находит main. Прежнее направление на этот вопрос
// отвечать не умело.
func TestGetCallgraph_Callers(t *testing.T) {
	server, ids := callgraphFixture(t)

	res := callgraph(t, server, ids["helper"], callgraphCallers)

	if res.Direction != callgraphCallers {
		t.Fatalf("direction в ответе %q, ожидали %q", res.Direction, callgraphCallers)
	}

	got, ok := neighbours(res, ids["helper"])
	if !ok {
		t.Fatal("в ответе нет узла helper")
	}

	if !containsID(got, ids["main"]) {
		t.Fatalf("helper должен вызываться из main, получили %v", got)
	}

	// leaf вызывается helper'ом, а не наоборот: обход не должен уходить вниз.
	if containsID(got, ids["leaf"]) {
		t.Fatalf("leaf не вызывает helper, но попал в callers: %v", got)
	}

	for _, n := range res.Nodes {
		if len(n.CallsTo) > 0 {
			t.Fatalf("при direction=callers поле callsTo должно быть пустым, узел %s: %v", n.ID, n.CallsTo)
		}
	}
}

// Умолчание не меняется: у существующих потребителей параметра direction нет.
func TestGetCallgraph_DefaultIsCallees(t *testing.T) {
	server, ids := callgraphFixture(t)

	res := callgraph(t, server, ids["helper"], "")

	if res.Direction != callgraphCallees {
		t.Fatalf("умолчание %q, ожидали %q", res.Direction, callgraphCallees)
	}

	got, ok := neighbours(res, ids["helper"])
	if !ok {
		t.Fatal("в ответе нет узла helper")
	}

	if !containsID(got, ids["leaf"]) {
		t.Fatalf("helper зовёт leaf, получили %v", got)
	}

	if containsID(got, ids["main"]) {
		t.Fatalf("main зовёт helper, а не наоборот: %v", got)
	}
}

func TestGetCallgraph_RejectsUnknownDirection(t *testing.T) {
	server, ids := callgraphFixture(t)

	raw, err := json.Marshal(map[string]any{"entry": ids["helper"], "direction": "sideways"})
	if err != nil {
		t.Fatal(err)
	}

	// Молча свести неизвестное направление к умолчанию значило бы ответить на
	// другой вопрос, чем задали.
	if _, err := handleGetCallgraph(server.state, raw); err == nil {
		t.Fatal("неизвестное direction должно быть ошибкой")
	}
}

// Вызов через интерфейс: анализатор разрешает его в конкретный метод, но
// РЕБРОМ references (по имени), а не calls. По одним calls такой метод
// выглядит никем не вызываемым — ровно та слепая зона, из-за которой карточка
// влияния занижала радиус для контроллеров и клиентов.
func TestGetCallgraph_InterfaceDispatchInReferencedBy(t *testing.T) {
	tmpDir := t.TempDir()
	src := `package main

type Fetcher interface {
	Fetch(id int) string
}

type httpFetcher struct{}

func (h *httpFetcher) Fetch(id int) string {
	return "x"
}

func useIface(f Fetcher) string {
	return f.Fetch(1)
}

func main() {
	println(useIface(&httpFetcher{}))
}
`

	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	server := createInitializedServer(t, tmpDir)

	var implID, callerID string

	for _, n := range server.state.GetGraph().Nodes {
		switch {
		case n.Entity == "method" && n.Title == "Fetch":
			implID = n.ID
		case n.Entity == "function" && n.Title == "useIface":
			callerID = n.ID
		}
	}

	if implID == "" || callerID == "" {
		t.Fatalf("не нашёл узлы: impl=%q caller=%q", implID, callerID)
	}

	res := callgraph(t, server, implID, callgraphCallers)

	var node *CallGraphNode

	for i := range res.Nodes {
		if res.Nodes[i].ID == implID {
			node = &res.Nodes[i]

			break
		}
	}

	if node == nil {
		t.Fatal("в ответе нет узла реализации")
	}

	if !containsID(node.ReferencedBy, callerID) {
		t.Fatalf("вызов через интерфейс потерян: referencedBy=%v", node.ReferencedBy)
	}

	// Точность полей не должна смешиваться: приблизительная связь не имеет
	// права выглядеть как прямой вызов.
	if containsID(node.CalledBy, callerID) {
		t.Fatalf("приблизительная связь попала в calledBy: %v", node.CalledBy)
	}
}
