package mcp

import (
	"encoding/json"
	"fmt"
)

// handleGetDegradationReport implements the get_degradation_report tool.
func handleGetDegradationReport(state *State, args json.RawMessage) (*DegradationReport, error) {
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

	return state.GetDegradationDetector().CheckWithoutUpdate(params.Path, a, graph), nil
}
