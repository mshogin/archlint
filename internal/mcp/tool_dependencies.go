package mcp

import (
	"encoding/json"
	"fmt"
	"path/filepath"
)

// handleGetDependencies implements the get_dependencies tool.
func handleGetDependencies(state *State, args json.RawMessage) (*DependencyResult, error) {
	var params struct {
		Path string `json:"path"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if params.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	graph := state.GetGraph()
	a := state.GetAnalyzer()

	if a == nil {
		return nil, fmt.Errorf("analyzer not initialized")
	}

	// Determine the package for the given path.
	target := params.Path

	absPath, err := filepath.Abs(params.Path)
	if err == nil {
		// Try to find the package for this file.
		for _, typeInfo := range a.AllTypes() {
			if matchesFile(typeInfo.File, absPath) {
				target = typeInfo.Package

				break
			}
		}

		for _, funcInfo := range a.AllFunctions() {
			if matchesFile(funcInfo.File, absPath) {
				target = funcInfo.Package

				break
			}
		}
	}

	result := &DependencyResult{
		Target: target,
	}

	for _, edge := range graph.Edges {
		if edge.Type != "import" {
			continue
		}

		if matchesTarget(edge.From, target) {
			result.DependsOn = append(result.DependsOn, DependencySummary{
				From: edge.From,
				To:   edge.To,
				Type: edge.Type,
			})
		}

		if matchesTarget(edge.To, target) {
			result.UsedBy = append(result.UsedBy, DependencySummary{
				From: edge.From,
				To:   edge.To,
				Type: edge.Type,
			})
		}
	}

	return result, nil
}
