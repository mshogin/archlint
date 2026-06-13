// Package graphloader загружает архитектурный граф из YAML (3 формата: archlint,
// DocHub, callgraph) — порт validator/graph_loader.py (NetworkX -> Go), направление A.
//
// Это ПРЕДУСЛОВИЕ полного выпила Python: Go-движок (analyzer) парсит только Go-код,
// а УНИВЕРСАЛЬНЫЙ вход (готовый граф из YAML любого формата) грузил Python. Здесь Go
// учится грузить произвольный граф из YAML. Эквивалентность гарантируется golden'ом
// против networkx (тот же YAML -> идентичные узлы/рёбра/атрибуты).
//
// Семантика 1:1 с nx.DiGraph: параллельные (from,to) СХЛОПЫВАЮТСЯ (last-wins по
// атрибутам, как nx.add_edge на существующем ребре); дефолты полей повторяют Python.
package graphloader

import (
	"fmt"
	"os"

	"github.com/mshogin/archlint/internal/model"
	"gopkg.in/yaml.v3"
)

// Тип источника графа (как в Python: structure=architecture, behavior=callgraph).
const (
	SourceStructure = "structure"
	SourceBehavior  = "behavior"
)

// LoadYAML грузит граф из YAML-файла (автоопределение формата).
func LoadYAML(filename string) (*model.Graph, error) {
	g, _, err := LoadYAMLWithSource(filename)

	return g, err
}

// LoadYAMLWithSource грузит граф и возвращает тип источника (structure|behavior).
func LoadYAMLWithSource(filename string) (*model.Graph, string, error) {
	data, err := os.ReadFile(filename) //nolint:gosec // путь — пользовательский вход CLI
	if err != nil {
		return nil, "", fmt.Errorf("read %s: %w", filename, err)
	}

	return ParseYAMLWithSource(data)
}

// ParseYAML парсит YAML-байты в граф (автоопределение формата).
func ParseYAML(data []byte) (*model.Graph, error) {
	g, _, err := ParseYAMLWithSource(data)

	return g, err
}

// ParseYAMLWithSource парсит YAML-байты в граф + тип источника.
func ParseYAMLWithSource(data []byte) (*model.Graph, string, error) {
	var raw map[string]any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, "", fmt.Errorf("parse yaml: %w", err)
	}

	if raw == nil {
		raw = map[string]any{}
	}

	source := DetectSource(raw)

	return parseData(raw, source), source, nil
}

// DetectSource определяет источник: behavior (callgraph) при наличии nodes+edges и
// признаков callgraph (package/function в первом узле или entry_point); иначе structure.
// 1:1 с Python detect_source (проверка entry_point — только при непустых nodes).
func DetectSource(raw map[string]any) string {
	_, hasNodes := raw["nodes"]
	_, hasEdges := raw["edges"]

	if hasNodes && hasEdges {
		nodes := asSlice(raw["nodes"])
		if len(nodes) > 0 {
			first := asMap(nodes[0])
			_, hasEntry := raw["entry_point"]

			if hasKey(first, "package") || hasKey(first, "function") || hasEntry {
				return SourceBehavior
			}
		}
	}

	return SourceStructure
}

// parseData диспетчеризует по формату (как Python parse_yaml).
func parseData(raw map[string]any, source string) *model.Graph {
	if source == SourceBehavior {
		return parseCallgraph(raw)
	}

	// structure: components-список -> archlint; components-словарь -> DocHub.
	if comp, ok := raw["components"]; ok {
		if _, isMap := comp.(map[string]any); isMap {
			return parseDochub(raw)
		}
	}

	return parseArchlint(raw)
}

// parseArchlint: components:[{id,title,entity,properties}], links:[{from,to,type,method,properties}].
func parseArchlint(raw map[string]any) *model.Graph {
	g := &model.Graph{}
	idx := newEdgeIndex()

	for _, c := range asSlice(raw["components"]) {
		comp := asMap(c)
		id := asString(comp, "id")

		if id == "" {
			continue
		}

		g.Nodes = append(g.Nodes, model.Node{
			ID:     id,
			Title:  asString(comp, "title"),
			Entity: asString(comp, "entity"),
			Attrs: map[string]any{
				"properties": asMapDefault(comp, "properties"),
			},
		})
	}

	for _, l := range asSlice(raw["links"]) {
		link := asMap(l)
		from := asString(link, "from")
		to := asString(link, "to")

		if from == "" || to == "" {
			continue
		}

		idx.set(g, model.Edge{
			From:   from,
			To:     to,
			Type:   asString(link, "type"),
			Method: asString(link, "method"),
			Attrs: map[string]any{
				"properties": asMapDefault(link, "properties"),
			},
		})
	}

	return g
}

// parseCallgraph: entry_point, nodes:[{id,package,function,receiver,type,file,line,depth}],
// edges:[{from,to,call_type,line}], stats. Узлы из рёбер, не объявленные явно, создаются
// (title=id, entity=unknown). 1:1 с Python _parse_callgraph_format.
func parseCallgraph(raw map[string]any) *model.Graph {
	g := &model.Graph{}
	nodeIdx := map[string]int{} // id -> позиция в g.Nodes

	if ep := asString(raw, "entry_point"); ep != "" {
		ensureAttrs(g)
		g.Attrs["entry_point"] = ep
	}

	addNode := func(n model.Node) {
		nodeIdx[n.ID] = len(g.Nodes)
		g.Nodes = append(g.Nodes, n)
	}

	for _, nd := range asSlice(raw["nodes"]) {
		node := asMap(nd)
		id := asString(node, "id")

		if id == "" {
			continue
		}

		typ := asString(node, "type")
		title := asStringDefault(node, "function", id)
		addNode(model.Node{
			ID:     id,
			Title:  title,
			Entity: typ,
			Attrs: map[string]any{
				"package":  asString(node, "package"),
				"function": asString(node, "function"),
				"receiver": asString(node, "receiver"),
				"type":     typ,
				"file":     asString(node, "file"),
				"line":     asInt(node, "line"),
				"depth":    asInt(node, "depth"),
			},
		})
	}

	idx := newEdgeIndex()

	for _, ed := range asSlice(raw["edges"]) {
		edge := asMap(ed)
		from := asString(edge, "from")
		to := asString(edge, "to")

		if from == "" || to == "" {
			continue
		}

		// Узлы из рёбер, не объявленные в nodes -> создаём (как Python).
		for _, end := range []string{from, to} {
			if _, ok := nodeIdx[end]; !ok {
				addNode(model.Node{ID: end, Title: end, Entity: "unknown"})
			}
		}

		callType := asStringDefault(edge, "call_type", "direct")
		idx.set(g, model.Edge{
			From: from,
			To:   to,
			Type: callType,
			Attrs: map[string]any{
				"call_type": callType,
				"line":      asInt(edge, "line"),
			},
		})
	}

	if stats := asMapRaw(raw["stats"]); len(stats) > 0 {
		ensureAttrs(g)
		g.Attrs["stats"] = stats
	}

	return g
}

// parseDochub: components:{id:{title,entity,properties}}, links:{from:[{to,type,method,properties}]}.
func parseDochub(raw map[string]any) *model.Graph {
	g := &model.Graph{}

	comps := asMap(raw["components"])
	for id, cv := range comps {
		comp := asMap(cv)
		if comp == nil {
			continue
		}

		g.Nodes = append(g.Nodes, model.Node{
			ID:     id,
			Title:  asString(comp, "title"),
			Entity: asString(comp, "entity"),
			Attrs: map[string]any{
				"properties": asMapDefault(comp, "properties"),
			},
		})
	}

	idx := newEdgeIndex()

	links := asMap(raw["links"])
	for from, lv := range links {
		for _, l := range asSlice(lv) {
			link := asMap(l)
			to := asString(link, "to")

			if to == "" {
				continue
			}

			idx.set(g, model.Edge{
				From:   from,
				To:     to,
				Type:   asString(link, "type"),
				Method: asString(link, "method"),
				Attrs: map[string]any{
					"properties": asMapDefault(link, "properties"),
				},
			})
		}
	}

	return g
}

// edgeIndex обеспечивает семантику nx.add_edge: одно ребро на (from,to), повторный
// (from,to) ПЕРЕЗАПИСЫВАЕТ атрибуты (last-wins).
type edgeIndex struct {
	pos map[[2]string]int
}

func newEdgeIndex() *edgeIndex { return &edgeIndex{pos: map[[2]string]int{}} }

func (ei *edgeIndex) set(g *model.Graph, e model.Edge) {
	key := [2]string{e.From, e.To}
	if i, ok := ei.pos[key]; ok {
		g.Edges[i] = e // last-wins

		return
	}

	ei.pos[key] = len(g.Edges)
	g.Edges = append(g.Edges, e)
}

func ensureAttrs(g *model.Graph) {
	if g.Attrs == nil {
		g.Attrs = map[string]any{}
	}
}

// --- accessors над map[string]any (yaml.v3) ---

func asString(m map[string]any, key string) string {
	if m == nil {
		return ""
	}

	if v, ok := m[key].(string); ok {
		return v
	}

	return ""
}

func asStringDefault(m map[string]any, key, def string) string {
	if v := asString(m, key); v != "" {
		return v
	}

	return def
}

func asInt(m map[string]any, key string) int {
	if m == nil {
		return 0
	}

	switch v := m[key].(type) {
	case int:
		return v
	case int64:
		return int(v)
	case float64:
		return int(v)
	}

	return 0
}

func asSlice(v any) []any {
	if s, ok := v.([]any); ok {
		return s
	}

	return nil
}

func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}

	return nil
}

func asMapRaw(v any) map[string]any { return asMap(v) }

// asMapDefault возвращает вложенный словарь поля или ПУСТОЙ {} (как Python default {}).
func asMapDefault(m map[string]any, key string) map[string]any {
	if m != nil {
		if mm, ok := m[key].(map[string]any); ok {
			return mm
		}
	}

	return map[string]any{}
}

func hasKey(m map[string]any, key string) bool {
	if m == nil {
		return false
	}

	_, ok := m[key]

	return ok
}
