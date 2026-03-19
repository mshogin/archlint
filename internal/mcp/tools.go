package mcp

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mshogin/archlint/internal/model"
)

// ToolDefinition describes an MCP tool for the tools/list response.
type ToolDefinition struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// toolDefinitions returns the list of all available tools.
func toolDefinitions() []ToolDefinition {
	return []ToolDefinition{
		{
			Name:        "analyze_file",
			Description: "Analyze a Go file's architecture: structs, interfaces, functions, dependencies, and diagnostics",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "File path to analyze"}
				},
				"required": ["path"]
			}`),
		},
		{
			Name:        "analyze_change",
			Description: "Analyze the architectural impact of a file change: affected nodes, related edges, impact level, degradation report",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "File path that changed"},
					"diff": {"type": "string", "description": "Optional diff content (unused, analysis is AST-based)"}
				},
				"required": ["path"]
			}`),
		},
		{
			Name:        "get_dependencies",
			Description: "Get dependency graph for a file or package: what it depends on and what depends on it",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "File path or package ID to get dependencies for"}
				},
				"required": ["path"]
			}`),
		},
		{
			Name:        "get_architecture",
			Description: "Get the full architecture graph or a filtered subset for a package",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"package": {"type": "string", "description": "Optional package filter (returns full graph if empty)"}
				}
			}`),
		},
		{
			Name:        "check_violations",
			Description: "Check for architecture violations: SOLID, god classes, circular dependencies, high coupling, and code smells",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "Optional file path or package to check (checks all if empty)"}
				}
			}`),
		},
		{
			Name:        "get_callgraph",
			Description: "Get call graph from a function or method entry point",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"entry": {"type": "string", "description": "Entry point function/method ID (e.g. 'internal/service.OrderService.ProcessOrder')"},
					"max_depth": {"type": "number", "description": "Maximum depth to traverse (default: 10)"}
				},
				"required": ["entry"]
			}`),
		},
		{
			Name:        "get_file_metrics",
			Description: "Get rich per-file architecture metrics: coupling, SOLID violations, code smells, health score (0-100)",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "File path to compute metrics for"}
				},
				"required": ["path"]
			}`),
		},
		{
			Name:        "get_degradation_report",
			Description: "Get degradation report: compare current file health against baseline, detect new/fixed violations",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {
					"path": {"type": "string", "description": "File path to check for degradation"}
				},
				"required": ["path"]
			}`),
		},
	}
}

// FileAnalysis is the result of analyzing a single file.
type FileAnalysis struct {
	FilePath     string              `json:"filePath"`
	Package      string              `json:"package"`
	Types        []TypeSummary       `json:"types,omitempty"`
	Functions    []FunctionSummary   `json:"functions,omitempty"`
	Methods      []MethodSummary     `json:"methods,omitempty"`
	Dependencies []DependencySummary `json:"dependencies,omitempty"`
	Violations   []Violation         `json:"violations,omitempty"`
}

// TypeSummary holds brief info about a type.
type TypeSummary struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Kind   string   `json:"kind"`
	Line   int      `json:"line"`
	Fields int      `json:"fields"`
	Embeds []string `json:"embeds,omitempty"`
	UsedBy []string `json:"usedBy,omitempty"`
}

// FunctionSummary holds brief info about a function.
type FunctionSummary struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Line  int      `json:"line"`
	Calls []string `json:"calls,omitempty"`
}

// MethodSummary holds brief info about a method.
type MethodSummary struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Receiver string   `json:"receiver"`
	Line     int      `json:"line"`
	Calls    []string `json:"calls,omitempty"`
}

// DependencySummary describes a dependency edge.
type DependencySummary struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"`
}

// ChangeAnalysis is the result of analyzing a file change.
type ChangeAnalysis struct {
	FilePath      string              `json:"filePath"`
	AffectedNodes []string            `json:"affectedNodes"`
	RelatedEdges  []DependencySummary `json:"relatedEdges,omitempty"`
	Impact        string              `json:"impact"`
	Violations    []Violation         `json:"violations,omitempty"`
	Degradation   *DegradationReport  `json:"degradation,omitempty"`
}

// DependencyResult is the result of a dependency query.
type DependencyResult struct {
	Target    string              `json:"target"`
	DependsOn []DependencySummary `json:"dependsOn,omitempty"`
	UsedBy    []DependencySummary `json:"usedBy,omitempty"`
}

// Violation describes an architecture violation.
type Violation struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
	Target  string `json:"target,omitempty"`
}

// CallGraphNode represents a node in the call graph result.
type CallGraphNode struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	Depth   int      `json:"depth"`
	CallsTo []string `json:"callsTo,omitempty"`
}

// CallGraphResult is the result of a call graph query.
type CallGraphResult struct {
	Entry    string          `json:"entry"`
	MaxDepth int             `json:"maxDepth"`
	Nodes    []CallGraphNode `json:"nodes"`
}

// ToolExecutor executes MCP tools using the in-memory state.
type ToolExecutor struct {
	state *State
}

// NewToolExecutor creates a new tool executor.
func NewToolExecutor(state *State) *ToolExecutor {
	return &ToolExecutor{state: state}
}

// Execute runs a tool by name with the given arguments.
func (e *ToolExecutor) Execute(toolName string, args json.RawMessage) (interface{}, error) {
	switch toolName {
	case "analyze_file":
		return e.analyzeFile(args)
	case "analyze_change":
		return e.analyzeChange(args)
	case "get_dependencies":
		return e.getDependencies(args)
	case "get_architecture":
		return e.getArchitecture(args)
	case "check_violations":
		return e.checkViolations(args)
	case "get_callgraph":
		return e.getCallgraph(args)
	case "get_file_metrics":
		return e.getFileMetrics(args)
	case "get_degradation_report":
		return e.getDegradationReport(args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

func (e *ToolExecutor) analyzeFile(args json.RawMessage) (*FileAnalysis, error) {
	var params struct {
		Path string `json:"path"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if params.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	a := e.state.GetAnalyzer()
	if a == nil {
		return nil, fmt.Errorf("analyzer not initialized")
	}

	graph := e.state.GetGraph()

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
	result.Violations = detectViolationsForPackage(graph, result.Package)

	return result, nil
}

func (e *ToolExecutor) analyzeChange(args json.RawMessage) (*ChangeAnalysis, error) {
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

	a := e.state.GetAnalyzer()
	if a == nil {
		return nil, fmt.Errorf("analyzer not initialized")
	}

	graph := e.state.GetGraph()

	absPath, err := filepath.Abs(params.Path)
	if err != nil {
		absPath = params.Path
	}

	result := &ChangeAnalysis{
		FilePath: absPath,
	}

	// Find all nodes defined in this file.
	for typeID, typeInfo := range a.AllTypes() {
		if matchesFile(typeInfo.File, absPath) {
			result.AffectedNodes = append(result.AffectedNodes, typeID)
		}
	}

	for funcID, funcInfo := range a.AllFunctions() {
		if matchesFile(funcInfo.File, absPath) {
			result.AffectedNodes = append(result.AffectedNodes, funcID)
		}
	}

	for methodID, methodInfo := range a.AllMethods() {
		if matchesFile(methodInfo.File, absPath) {
			result.AffectedNodes = append(result.AffectedNodes, methodID)
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

	result.Violations = detectViolationsForPackage(graph, pkg)

	// Include degradation report.
	result.Degradation = e.state.GetDegradationDetector().CheckWithoutUpdate(absPath, a, graph)

	return result, nil
}

func (e *ToolExecutor) getDependencies(args json.RawMessage) (*DependencyResult, error) {
	var params struct {
		Path string `json:"path"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if params.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	graph := e.state.GetGraph()
	a := e.state.GetAnalyzer()

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

func (e *ToolExecutor) getArchitecture(args json.RawMessage) (*model.Graph, error) {
	var params struct {
		Package string `json:"package"`
	}

	if args != nil {
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
	}

	graph := e.state.GetGraph()

	if params.Package == "" {
		return graph, nil
	}

	return filterGraph(graph, params.Package), nil
}

// ViolationReport is the result of check_violations with rich metrics.
type ViolationReport struct {
	Violations  []Violation `json:"violations"`
	FileMetrics *FileMetrics `json:"fileMetrics,omitempty"`
}

func (e *ToolExecutor) checkViolations(args json.RawMessage) (*ViolationReport, error) {
	var params struct {
		Path string `json:"path"`
	}

	if args != nil {
		if err := json.Unmarshal(args, &params); err != nil {
			return nil, fmt.Errorf("invalid arguments: %w", err)
		}
	}

	graph := e.state.GetGraph()
	a := e.state.GetAnalyzer()

	report := &ViolationReport{}

	if params.Path != "" {
		if a == nil {
			return nil, fmt.Errorf("analyzer not initialized")
		}

		// Determine package from path.
		target := params.Path

		absPath, err := filepath.Abs(params.Path)
		if err == nil {
			for _, typeInfo := range a.AllTypes() {
				if matchesFile(typeInfo.File, absPath) {
					target = typeInfo.Package

					break
				}
			}
		}

		// Classic violations (coupling, cycles).
		report.Violations = detectViolationsForPackage(graph, target)

		// Rich per-file metrics including SOLID, smells.
		metrics := ComputeFileMetrics(params.Path, a, graph)
		report.FileMetrics = metrics

		// Merge metrics-derived violations into the report.
		report.Violations = append(report.Violations, metrics.SRPViolations...)
		report.Violations = append(report.Violations, metrics.DIPViolations...)
		report.Violations = append(report.Violations, metrics.ISPViolations...)

		for _, gc := range metrics.GodClasses {
			report.Violations = append(report.Violations, Violation{
				Kind:    "god-class",
				Message: fmt.Sprintf("God class detected: %s", gc),
				Target:  gc,
			})
		}

		for _, hub := range metrics.HubNodes {
			report.Violations = append(report.Violations, Violation{
				Kind:    "hub-node",
				Message: fmt.Sprintf("Hub node detected (fan-in + fan-out > %d): %s", hubThreshold, hub),
				Target:  hub,
			})
		}

		for _, fe := range metrics.FeatureEnvy {
			report.Violations = append(report.Violations, Violation{
				Kind:    "feature-envy",
				Message: fmt.Sprintf("Feature envy detected: %s calls more methods on other types than its own receiver", fe),
				Target:  fe,
			})
		}

		for _, ss := range metrics.ShotgunSurgery {
			report.Violations = append(report.Violations, Violation{
				Kind:    "shotgun-surgery",
				Message: fmt.Sprintf("Shotgun surgery risk: changes to %s would affect >%d files", ss, shotgunThreshold),
				Target:  ss,
			})
		}

		return report, nil
	}

	// Check all packages.
	report.Violations = detectAllViolations(graph)

	return report, nil
}

func (e *ToolExecutor) getCallgraph(args json.RawMessage) (*CallGraphResult, error) {
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

	graph := e.state.GetGraph()

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

func (e *ToolExecutor) getFileMetrics(args json.RawMessage) (*FileMetrics, error) {
	var params struct {
		Path string `json:"path"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if params.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	a := e.state.GetAnalyzer()
	if a == nil {
		return nil, fmt.Errorf("analyzer not initialized")
	}

	graph := e.state.GetGraph()

	return ComputeFileMetrics(params.Path, a, graph), nil
}

func (e *ToolExecutor) getDegradationReport(args json.RawMessage) (*DegradationReport, error) {
	var params struct {
		Path string `json:"path"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if params.Path == "" {
		return nil, fmt.Errorf("path is required")
	}

	a := e.state.GetAnalyzer()
	if a == nil {
		return nil, fmt.Errorf("analyzer not initialized")
	}

	graph := e.state.GetGraph()

	return e.state.GetDegradationDetector().CheckWithoutUpdate(params.Path, a, graph), nil
}

// getPackageDependencies collects import dependencies for a package.
func getPackageDependencies(graph *model.Graph, pkgID string) []DependencySummary {
	if pkgID == "" {
		return nil
	}

	var deps []DependencySummary

	seen := make(map[string]bool)

	for _, edge := range graph.Edges {
		if edge.Type != "import" || edge.From != pkgID {
			continue
		}

		key := edge.From + "->" + edge.To
		if seen[key] {
			continue
		}

		seen[key] = true

		deps = append(deps, DependencySummary{
			From: edge.From,
			To:   edge.To,
			Type: edge.Type,
		})
	}

	return deps
}

// detectViolationsForPackage checks for violations in a specific package.
func detectViolationsForPackage(graph *model.Graph, pkgID string) []Violation {
	if pkgID == "" {
		return nil
	}

	var violations []Violation

	// High efferent coupling check.
	importCount := 0

	for _, edge := range graph.Edges {
		if edge.From == pkgID && edge.Type == "import" {
			importCount++
		}
	}

	const highCouplingThreshold = 10

	if importCount > highCouplingThreshold {
		violations = append(violations, Violation{
			Kind:    "high-efferent-coupling",
			Message: fmt.Sprintf("Package %s has %d dependencies (threshold: %d). Consider decomposition.", pkgID, importCount, highCouplingThreshold),
			Target:  pkgID,
		})
	}

	// Circular dependency check.
	violations = append(violations, detectCycles(graph, pkgID)...)

	return violations
}

// detectAllViolations checks all packages for violations.
func detectAllViolations(graph *model.Graph) []Violation {
	var violations []Violation

	packages := make(map[string]bool)

	for _, node := range graph.Nodes {
		if node.Entity == "package" {
			packages[node.ID] = true
		}
	}

	for pkgID := range packages {
		violations = append(violations, detectViolationsForPackage(graph, pkgID)...)
	}

	return violations
}

// detectCycles searches for circular dependencies via import edges using DFS.
func detectCycles(graph *model.Graph, startPkg string) []Violation {
	adj := make(map[string][]string)

	for _, edge := range graph.Edges {
		if edge.Type == "import" {
			adj[edge.From] = append(adj[edge.From], edge.To)
		}
	}

	visited := make(map[string]bool)
	inStack := make(map[string]bool)
	var cyclePath []string

	var dfs func(node string) bool

	dfs = func(node string) bool {
		visited[node] = true
		inStack[node] = true
		cyclePath = append(cyclePath, node)

		for _, next := range adj[node] {
			if next == startPkg && inStack[next] {
				return true
			}

			if !visited[next] {
				if dfs(next) {
					return true
				}
			}
		}

		inStack[node] = false
		cyclePath = cyclePath[:len(cyclePath)-1]

		return false
	}

	if dfs(startPkg) {
		cycle := strings.Join(cyclePath, " -> ") + " -> " + startPkg

		return []Violation{{
			Kind:    "circular-dependency",
			Message: fmt.Sprintf("Circular dependency detected: %s", cycle),
			Target:  startPkg,
		}}
	}

	return nil
}

// filterGraph filters the graph to only include nodes and edges related to filter.
func filterGraph(graph *model.Graph, filter string) *model.Graph {
	nodeSet := make(map[string]bool)

	for _, node := range graph.Nodes {
		if matchesTarget(node.ID, filter) {
			nodeSet[node.ID] = true
		}
	}

	var filteredNodes []model.Node

	for _, node := range graph.Nodes {
		if nodeSet[node.ID] {
			filteredNodes = append(filteredNodes, node)
		}
	}

	var filteredEdges []model.Edge

	for _, edge := range graph.Edges {
		if nodeSet[edge.From] || nodeSet[edge.To] {
			filteredEdges = append(filteredEdges, edge)
		}
	}

	return &model.Graph{
		Nodes: filteredNodes,
		Edges: filteredEdges,
	}
}

// matchesFile checks if a file path matches the requested path.
func matchesFile(actual, requested string) bool {
	if actual == requested {
		return true
	}

	absActual, err := filepath.Abs(actual)
	if err != nil {
		return false
	}

	return absActual == requested
}

// matchesTarget checks if an ID matches the target.
func matchesTarget(id, target string) bool {
	return id == target || strings.HasPrefix(id, target+".") || strings.HasSuffix(id, "/"+target)
}
