package mcp

import (
	"encoding/json"
	"fmt"
)

// handleGetCallgraph implements the get_callgraph tool.
func handleGetCallgraph(state *State, args json.RawMessage) (*CallGraphResult, error) {
	var params struct {
		Entry    string `json:"entry"`
		MaxDepth int    `json:"max_depth"`
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

	graph := state.GetGraph()

	// Build adjacency list from "calls" edges.
	callAdj := make(map[string][]string)
	nodeNames := make(map[string]string)

	for _, edge := range graph.Edges {
		if edge.Type == "calls" {
			callAdj[edge.From] = append(callAdj[edge.From], edge.To)
		}
	}

	for _, node := range graph.Nodes {
		nodeNames[node.ID] = node.Title
	}

	result := &CallGraphResult{
		Entry:    params.Entry,
		MaxDepth: params.MaxDepth,
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

		var callsTo []string

		if item.depth < params.MaxDepth {
			for _, target := range callAdj[item.id] {
				callsTo = append(callsTo, target)

				if !visited[target] {
					visited[target] = true
					queue = append(queue, queueItem{id: target, depth: item.depth + 1})
				}
			}
		}

		result.Nodes = append(result.Nodes, CallGraphNode{
			ID:      item.id,
			Name:    name,
			Depth:   item.depth,
			CallsTo: callsTo,
		})
	}

	return result, nil
}
