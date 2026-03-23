package mcp

import (
	"encoding/json"
	"fmt"
	"path/filepath"
)

// handleAnalyzeFile implements the analyze_file tool.
func handleAnalyzeFile(state *State, args json.RawMessage) (*FileAnalysis, error) {
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

	absPath, err := filepath.Abs(params.Path)
	if err != nil {
		absPath = params.Path
	}

	result := &FileAnalysis{
		FilePath: absPath,
	}

	// Collect types defined in this file.
	for typeID, typeInfo := range a.AllTypes() {
		if !matchesFile(typeInfo.File, absPath) {
			continue
		}

		result.Package = typeInfo.Package

		ts := TypeSummary{
			ID:     typeID,
			Name:   typeInfo.Name,
			Kind:   typeInfo.Kind,
			Line:   typeInfo.Line,
			Fields: len(typeInfo.Fields),
			Embeds: typeInfo.Embeds,
		}

		for _, edge := range graph.Edges {
			if edge.To == typeID && (edge.Type == "uses" || edge.Type == "embeds") {
				ts.UsedBy = append(ts.UsedBy, edge.From)
			}
		}

		result.Types = append(result.Types, ts)
	}

	// Collect functions.
	for funcID, funcInfo := range a.AllFunctions() {
		if !matchesFile(funcInfo.File, absPath) {
			continue
		}

		if result.Package == "" {
			result.Package = funcInfo.Package
		}

		fs := FunctionSummary{
			ID:   funcID,
			Name: funcInfo.Name,
			Line: funcInfo.Line,
		}

		for _, call := range funcInfo.Calls {
			target := a.ResolveCallTarget(call, funcInfo.Package)
			if target != "" {
				fs.Calls = append(fs.Calls, target)
			}
		}

		result.Functions = append(result.Functions, fs)
	}

	// Collect methods.
	for methodID, methodInfo := range a.AllMethods() {
		if !matchesFile(methodInfo.File, absPath) {
			continue
		}

		if result.Package == "" {
			result.Package = methodInfo.Package
		}

		ms := MethodSummary{
			ID:       methodID,
			Name:     methodInfo.Name,
			Receiver: methodInfo.Receiver,
			Line:     methodInfo.Line,
		}

		for _, call := range methodInfo.Calls {
			target := a.ResolveCallTarget(call, methodInfo.Package)
			if target != "" {
				ms.Calls = append(ms.Calls, target)
			}
		}

		result.Methods = append(result.Methods, ms)
	}

	// Collect package dependencies.
	result.Dependencies = getPackageDependencies(graph, result.Package)

	// Check for violations.
	result.Violations = DetectViolationsForPackage(graph, result.Package)

	return result, nil
}
