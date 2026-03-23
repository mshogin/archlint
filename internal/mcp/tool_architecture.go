package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/mshogin/archlint/internal/model"
)

// handleGetArchitecture implements the get_architecture tool.
func handleGetArchitecture(state *State, args json.RawMessage) (*model.Graph, error) {
	var params struct {
		Package string `json:"package"`
	}

	if args != nil {
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
	}

	graph := state.GetGraph()

	if params.Package == "" {
		return graph, nil
	}

	return filterGraph(graph, params.Package), nil
}
