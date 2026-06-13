package mcp

import (
	"encoding/json"
	"fmt"
	"path/filepath"
	"sort"
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
		{
			Name:        "descriptors",
			Description: "Get structural magnitude descriptors of the architecture graph: centralities (pagerank, betweenness, closeness, harmonic, eigenvector), coupling (Ca/Ce, instability, abstractness, D), distribution (gini, entropy, clustering, k-core), reachability (blast radius, change propagation), and Fowler smell counts. Signals/magnitudes for analysis — not a gate.",
			InputSchema: json.RawMessage(`{
				"type": "object",
				"properties": {}
			}`),
		},
	}
}

// --- Result types ---

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

// ViolationReport is the result of check_violations with rich metrics.
type ViolationReport struct {
	Violations  []Violation  `json:"violations"`
	FileMetrics *FileMetrics `json:"fileMetrics,omitempty"`
}

// --- ToolExecutor: thin dispatcher ---

// ToolExecutor executes MCP tools using the in-memory state.
type ToolExecutor struct {
	state StateReader
}

// NewToolExecutor creates a new tool executor.
func NewToolExecutor(state StateReader) *ToolExecutor {
	return &ToolExecutor{state: state}
}

// Execute runs a tool by name with the given arguments.
func (e *ToolExecutor) Execute(toolName string, args json.RawMessage) (interface{}, error) {
	switch toolName {
	case "analyze_file":
		return handleAnalyzeFile(e.state, args)
	case "analyze_change":
		return handleAnalyzeChange(e.state, args)
	case "get_dependencies":
		return handleGetDependencies(e.state, args)
	case "get_architecture":
		return handleGetArchitecture(e.state, args)
	case "check_violations":
		return handleCheckViolations(e.state, args)
	case "get_callgraph":
		return handleGetCallgraph(e.state, args)
	case "get_file_metrics":
		return handleGetFileMetrics(e.state, args)
	case "get_degradation_report":
		return handleGetDegradationReport(e.state, args)
	case "descriptors":
		return handleDescriptors(e.state, args)
	default:
		return nil, fmt.Errorf("unknown tool: %s", toolName)
	}
}

// --- Shared helpers ---

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

// DetectViolationsForPackage checks for violations in a specific package.
func DetectViolationsForPackage(graph *model.Graph, pkgID string) []Violation {
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

// DetectAllViolations checks all packages for violations.
func DetectAllViolations(graph *model.Graph) []Violation {
	var violations []Violation

	packages := make(map[string]bool)

	for _, node := range graph.Nodes {
		if node.Entity == "package" {
			packages[node.ID] = true
		}
	}

	for pkgID := range packages {
		violations = append(violations, DetectViolationsForPackage(graph, pkgID)...)
	}

	return violations
}

// detectCycles reports a circular-dependency Violation for startPkg if it
// participates in a cycle of the import graph. Делегирует graph-agnostic
// SCC-индексу (cycles_scc.go), который МЕМОИЗИРОВАН на граф (DR-0011): SCC не
// зависит от startPkg, поэтому считается один раз, а не P раз на каждый пакет.
//
// Принцип (DR-0005, чистый iff): узел в цикле ⟺ SCC размера>1 ИЛИ петля.
// Соундно+полно (циклы любой длины). Та же SCC-машина обслуживает карточку
// "слоистость Уровень A" (SCC>1 среди модулей = цикл модульных зависимостей).
func detectCycles(graph *model.Graph, startPkg string) []Violation {
	scc := cyclicSCC(graph)
	if !scc.cyclic[startPkg] {
		return nil
	}

	members := scc.members[startPkg]
	if len(members) == 0 {
		members = []string{startPkg} // self-loop: SCC размера 1
	}

	sorted := append([]string(nil), members...)
	sort.Strings(sorted)

	return []Violation{{
		Kind:    "circular-dependency",
		Message: fmt.Sprintf("Circular dependency detected (SCC size %d): %s", len(sorted), strings.Join(sorted, " <-> ")),
		Target:  startPkg,
	}}
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
