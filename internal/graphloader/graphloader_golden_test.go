package graphloader

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/mshogin/archlint/internal/model"
)

// Golden порта graph_loader.py -> Go (направление A, предусловие выпила Python).
// Критерий приёмки: тот же YAML на вход -> ИДЕНТИЧНЫЙ граф (узлы/рёбра/атрибуты/источник)
// в Go и в networkx. Эталон снят с настоящего validator.GraphLoader,
// лежит в testdata/<fmt>.expected.json. Сравнение — каноническая JSON-форма (ключи
// сортируются encoding/json, как Python sort_keys=True).

// canonical строит каноническую форму графа: ядро-поля вынесены (title/entity у узла,
// type/method у ребра), остальные nx-атрибуты — в "extra". Узлы сортируются по id,
// рёбра по (from,to) — как Python sorted(g.nodes())/sorted(g.edges()).
func canonical(g *model.Graph, source string) map[string]any {
	graphAttrs := map[string]any(g.Attrs)
	if graphAttrs == nil {
		graphAttrs = map[string]any{}
	}

	nodes := append([]model.Node(nil), g.Nodes...)
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })

	nodeList := make([]any, 0, len(nodes))
	for _, n := range nodes {
		nodeList = append(nodeList, map[string]any{
			"id":     n.ID,
			"title":  n.Title,
			"entity": n.Entity,
			"extra":  extraOrEmpty(n.Attrs),
		})
	}

	edges := append([]model.Edge(nil), g.Edges...)
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].From != edges[j].From {
			return edges[i].From < edges[j].From
		}

		return edges[i].To < edges[j].To
	})

	edgeList := make([]any, 0, len(edges))
	for _, e := range edges {
		edgeList = append(edgeList, map[string]any{
			"from":   e.From,
			"to":     e.To,
			"type":   e.Type,
			"method": e.Method,
			"extra":  extraOrEmpty(e.Attrs),
		})
	}

	return map[string]any{
		"source":      source,
		"graph_attrs": graphAttrs,
		"nodes":       nodeList,
		"edges":       edgeList,
	}
}

func extraOrEmpty(a map[string]any) map[string]any {
	if a == nil {
		return map[string]any{}
	}

	return a
}

func canonicalJSON(t *testing.T, v any) string {
	t.Helper()

	var buf bytes.Buffer

	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false) // как Python json.dumps (без экранирования <>&)

	if err := enc.Encode(v); err != nil {
		t.Fatalf("marshal: %v", err)
	}

	return strings.TrimSpace(buf.String())
}

func runGoldenFormat(t *testing.T, name string) {
	t.Helper()

	yamlPath := filepath.Join("testdata", name+".yaml")
	expPath := filepath.Join("testdata", name+".expected.json")

	g, source, err := LoadYAMLWithSource(yamlPath)
	if err != nil {
		t.Fatalf("LoadYAMLWithSource(%s): %v", yamlPath, err)
	}

	got := canonicalJSON(t, canonical(g, source))

	expBytes, err := os.ReadFile(expPath)
	if err != nil {
		t.Fatalf("read expected %s: %v", expPath, err)
	}

	want := strings.TrimSpace(string(expBytes))

	if got != want {
		t.Errorf("граф НЕ идентичен Python для формата %s\n--- got ---\n%s\n--- want (Python) ---\n%s", name, got, want)
	}
}

func TestGraphLoader_Golden_Archlint(t *testing.T)  { runGoldenFormat(t, "archlint") }
func TestGraphLoader_Golden_Dochub(t *testing.T)    { runGoldenFormat(t, "dochub") }
func TestGraphLoader_Golden_Callgraph(t *testing.T) { runGoldenFormat(t, "callgraph") }

// detect_source отдельно: archlint/dochub -> structure, callgraph -> behavior.
func TestGraphLoader_DetectSource(t *testing.T) {
	cases := map[string]string{"archlint": "structure", "dochub": "structure", "callgraph": "behavior"}
	for name, want := range cases {
		data, err := os.ReadFile(filepath.Join("testdata", name+".yaml"))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}

		_, src, err := ParseYAMLWithSource(data)
		if err != nil {
			t.Fatalf("parse %s: %v", name, err)
		}

		if src != want {
			t.Errorf("detect_source(%s): got %q, want %q", name, src, want)
		}
	}
}
