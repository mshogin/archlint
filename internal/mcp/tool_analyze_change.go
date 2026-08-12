package mcp

import (
	"encoding/json"
	"fmt"
	"path/filepath"
)

// handleAnalyzeChange implements the analyze_change tool.
func handleAnalyzeChange(state StateReader, args json.RawMessage) (*ChangeAnalysis, error) {
	var params struct {
		Path string `json:"path"`
		Diff string `json:"diff"`
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

	result := &ChangeAnalysis{
		FilePath: absPath,
		Scope:    scopeFile,
	}

	// Find all nodes defined in this file.
	var decls []decl

	for typeID, typeInfo := range a.AllTypes() {
		if matchesFile(typeInfo.File, absPath) {
			result.AffectedNodes = append(result.AffectedNodes, typeID)
			decls = append(decls, decl{id: typeID, line: typeInfo.Line})
		}
	}

	for funcID, funcInfo := range a.AllFunctions() {
		if matchesFile(funcInfo.File, absPath) {
			result.AffectedNodes = append(result.AffectedNodes, funcID)
			decls = append(decls, decl{id: funcID, line: funcInfo.Line})
		}
	}

	for methodID, methodInfo := range a.AllMethods() {
		if matchesFile(methodInfo.File, absPath) {
			result.AffectedNodes = append(result.AffectedNodes, methodID)
			decls = append(decls, decl{id: methodID, line: methodInfo.Line})
		}
	}

	// Дифф передан - сужаем «затронутое» до объявлений, в которые реально
	// попали изменённые строки. Если сузить не вышло (правка выше первого
	// объявления: импорты, package-level переменные), остаёмся на файле
	// целиком: такая правка architecturally значима, терять её нельзя.
	if changed := changedLines(params.Diff, absPath); len(changed) > 0 {
		result.ChangedLines = len(changed)

		if touched := declsTouched(decls, changed); len(touched) > 0 {
			result.AffectedNodes = touched
			result.Scope = scopeDecls
		}
	}

	// Find edges related to affected nodes.
	affectedSet := make(map[string]bool)
	for _, nodeID := range result.AffectedNodes {
		affectedSet[nodeID] = true
	}

	for _, edge := range graph.Edges {
		if affectedSet[edge.From] || affectedSet[edge.To] {
			result.RelatedEdges = append(result.RelatedEdges, DependencySummary{
				From: edge.From,
				To:   edge.To,
				Type: edge.Type,
			})
		}
	}

	// Assess impact.
	switch {
	case len(result.AffectedNodes) == 0:
		result.Impact = "none"
	case len(result.RelatedEdges) > 20:
		result.Impact = "high"
	case len(result.RelatedEdges) > 5:
		result.Impact = "medium"
	default:
		result.Impact = "low"
	}

	// Check for violations in the affected package.
	pkg := ""
	for _, typeInfo := range a.AllTypes() {
		if matchesFile(typeInfo.File, absPath) {
			pkg = typeInfo.Package

			break
		}
	}

	result.Violations = DetectViolationsForPackage(graph, pkg)

	// Объяснимость агентского гейта: severity-класс + флаги соундности из SSOT.
	ApplySeverity(result.Violations)

	// Include degradation report.
	result.Degradation = state.GetDegradationDetector().CheckWithoutUpdate(absPath, a, graph)

	return result, nil
}
