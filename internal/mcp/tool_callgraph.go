package mcp

import (
	"encoding/json"
	"fmt"
)

// Направления обхода графа вызовов.
const (
	callgraphCallees = "callees" // кого зовёт entry (обход по рёбрам calls как есть)
	callgraphCallers = "callers" // кто зовёт entry (обход по инвертированным рёбрам)
)

// handleGetCallgraph implements the get_callgraph tool.
//
// Направление важно для разбора последствий изменения: вопрос «кого зовёт эта
// функция» отвечает на «что она делает», а вопрос «кто зовёт эту функцию» - на
// «кого сломает правка внутри неё». Второй без инверсии рёбер недоступен, хотя
// рёбра calls в графе уже есть.
func handleGetCallgraph(state StateReader, args json.RawMessage) (*CallGraphResult, error) {
	var params struct {
		Entry     string `json:"entry"`
		MaxDepth  int    `json:"max_depth"`
		Direction string `json:"direction"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if params.Entry == "" {
		return nil, fmt.Errorf("entry is required")
	}

	if params.MaxDepth <= 0 {
		params.MaxDepth = 10
	}

	// Умолчание - прежнее поведение: у существующих потребителей direction нет.
	switch params.Direction {
	case "":
		params.Direction = callgraphCallees
	case callgraphCallees, callgraphCallers:
	default:
		return nil, fmt.Errorf("direction must be %q or %q, got %q",
			callgraphCallees, callgraphCallers, params.Direction)
	}

	graph := state.GetGraph()

	// Смежность строится по направлению обхода: для callers ребро A calls B
	// читается как «B вызывается из A».
	adj := make(map[string][]string)
	nodeNames := make(map[string]string)

	for _, edge := range graph.Edges {
		if edge.Type != "calls" {
			continue
		}

		if params.Direction == callgraphCallers {
			adj[edge.To] = append(adj[edge.To], edge.From)
		} else {
			adj[edge.From] = append(adj[edge.From], edge.To)
		}
	}

	for _, node := range graph.Nodes {
		nodeNames[node.ID] = node.Title
	}

	result := &CallGraphResult{
		Entry:     params.Entry,
		MaxDepth:  params.MaxDepth,
		Direction: params.Direction,
	}

	// BFS traversal from entry point.
	type queueItem struct {
		id    string
		depth int
	}

	visited := make(map[string]bool)
	queue := []queueItem{{id: params.Entry, depth: 0}}
	visited[params.Entry] = true

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		name := nodeNames[item.id]
		if name == "" {
			name = item.id
		}

		var neighbours []string

		if item.depth < params.MaxDepth {
			for _, target := range adj[item.id] {
				neighbours = append(neighbours, target)

				if !visited[target] {
					visited[target] = true
					queue = append(queue, queueItem{id: target, depth: item.depth + 1})
				}
			}
		}

		node := CallGraphNode{
			ID:    item.id,
			Name:  name,
			Depth: item.depth,
		}

		// Поля разные намеренно: одно и то же поле с разным смыслом читается
		// неверно и в отчёте, и моделью.
		if params.Direction == callgraphCallers {
			node.CalledBy = neighbours
		} else {
			node.CallsTo = neighbours
		}

		result.Nodes = append(result.Nodes, node)
	}

	return result, nil
}
