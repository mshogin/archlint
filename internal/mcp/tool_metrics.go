package mcp

import (
	"encoding/json"
	"fmt"
)

// handleGetFileMetrics implements the get_file_metrics tool.
func handleGetFileMetrics(state *State, args json.RawMessage) (*FileMetrics, error) {
	var params struct {
		Path string `json:"path"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if params.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	a := state.GetAnalyzer()
	if a == nil {
		return nil, fmt.Errorf("analyzer not initialized")
	}

	graph := state.GetGraph()

	return ComputeFileMetrics(params.Path, a, graph), nil
}
