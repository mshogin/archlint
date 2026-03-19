package lsp

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mshogin/archlint/internal/model"
)

// AnalyzerBridge связывает LSP-сервер с существующим GoAnalyzer.
type AnalyzerBridge struct {
	state *State
}

// NewAnalyzerBridge создаёт новый мост между LSP и анализатором.
func NewAnalyzerBridge(state *State) *AnalyzerBridge {
	return &AnalyzerBridge{state: state}
}

// FileAnalysis содержит результат анализа одного файла.
type FileAnalysis struct {
	FilePath     string              `json:"filePath"`
	Package      string              `json:"package"`
	Types        []TypeSummary       `json:"types,omitempty"`
	Functions    []FunctionSummary   `json:"functions,omitempty"`
	Methods      []MethodSummary     `json:"methods,omitempty"`
	Dependencies []DependencySummary `json:"dependencies,omitempty"`
	Diagnostics  []Diagnostic        `json:"diagnostics,omitempty"`
}

// TypeSummary содержит краткую информацию о типе.
type TypeSummary struct {
	ID     string   `json:"id"`
	Name   string   `json:"name"`
	Kind   string   `json:"kind"`
	Line   int      `json:"line"`
	Fields int      `json:"fields"`
	Embeds []string `json:"embeds,omitempty"`
	UsedBy []string `json:"usedBy,omitempty"`
}

// FunctionSummary содержит краткую информацию о функции.
type FunctionSummary struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Line  int      `json:"line"`
	Calls []string `json:"calls,omitempty"`
}

// MethodSummary содержит краткую информацию о методе.
type MethodSummary struct {
	ID       string   `json:"id"`
	Name     string   `json:"name"`
	Receiver string   `json:"receiver"`
	Line     int      `json:"line"`
	Calls    []string `json:"calls,omitempty"`
}

// DependencySummary описывает зависимость.
type DependencySummary struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"`
}

// ChangeAnalysis содержит результат анализа изменений.
type ChangeAnalysis struct {
	FilePath        string              `json:"filePath"`
	AffectedNodes   []string            `json:"affectedNodes"`
	NewDependencies []DependencySummary `json:"newDependencies,omitempty"`
	Impact          string              `json:"impact"`
	Diagnostics     []Diagnostic        `json:"diagnostics,omitempty"`
}

// MetricsResult содержит метрики для файла или пакета.
type MetricsResult struct {
	Target           string  `json:"target"`
	IncomingEdges    int     `json:"incomingEdges"`
	OutgoingEdges    int     `json:"outgoingEdges"`
	AfferentCoupling int     `json:"afferentCoupling"`
	EfferentCoupling int     `json:"efferentCoupling"`
	Instability      float64 `json:"instability"`
	TypeCount        int     `json:"typeCount"`
	FunctionCount    int     `json:"functionCount"`
	MethodCount      int     `json:"methodCount"`
}

// AnalyzeFile выполняет полный анализ указанного файла.
func (b *AnalyzerBridge) AnalyzeFile(filePath string) (*FileAnalysis, error) {
	a := b.state.GetAnalyzer()
	if a == nil {
		return nil, fmt.Errorf("анализатор не инициализирован")
	}

	graph := b.state.GetGraph()

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}

	result := &FileAnalysis{
		FilePath: absPath,
	}

	// Собираем типы, определённые в этом файле.
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

		// Находим кто использует этот тип.
		for _, edge := range graph.Edges {
			if edge.To == typeID && (edge.Type == "uses" || edge.Type == "embeds") {
				ts.UsedBy = append(ts.UsedBy, edge.From)
			}
		}

		result.Types = append(result.Types, ts)
	}

	// Собираем функции.
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

	// Собираем методы.
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

	// Собираем зависимости пакета.
	result.Dependencies = b.getPackageDependencies(graph, result.Package)

	// Генерируем диагностики.
	result.Diagnostics = b.generateFileDiagnostics(graph, absPath, result.Package)

	return result, nil
}

// AnalyzeChange анализирует влияние изменения файла на архитектуру.
func (b *AnalyzerBridge) AnalyzeChange(filePath string) (*ChangeAnalysis, error) {
	a := b.state.GetAnalyzer()
	if a == nil {
		return nil, fmt.Errorf("анализатор не инициализирован")
	}

	graph := b.state.GetGraph()

	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}

	result := &ChangeAnalysis{
		FilePath: absPath,
	}

	// Находим все узлы, определённые в этом файле.
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

	// Находим зависимости, связанные с затронутыми узлами.
	affectedSet := make(map[string]bool)
	for _, nodeID := range result.AffectedNodes {
		affectedSet[nodeID] = true
	}

	for _, edge := range graph.Edges {
		if affectedSet[edge.From] || affectedSet[edge.To] {
			result.NewDependencies = append(result.NewDependencies, DependencySummary{
				From: edge.From,
				To:   edge.To,
				Type: edge.Type,
			})
		}
	}

	// Оценка влияния.
	switch {
	case len(result.AffectedNodes) == 0:
		result.Impact = "none"
	case len(result.NewDependencies) > 20:
		result.Impact = "high"
	case len(result.NewDependencies) > 5:
		result.Impact = "medium"
	default:
		result.Impact = "low"
	}

	// Диагностики для изменённого файла.
	pkg := ""
	for _, typeInfo := range a.AllTypes() {
		if matchesFile(typeInfo.File, absPath) {
			pkg = typeInfo.Package

			break
		}
	}

	result.Diagnostics = b.generateFileDiagnostics(graph, absPath, pkg)

	return result, nil
}

// GetMetrics возвращает метрики для указанного пакета или файла.
func (b *AnalyzerBridge) GetMetrics(target string) (*MetricsResult, error) {
	graph := b.state.GetGraph()
	if graph == nil {
		return nil, fmt.Errorf("граф не инициализирован")
	}

	result := &MetricsResult{
		Target: target,
	}

	a := b.state.GetAnalyzer()
	if a == nil {
		return result, nil
	}

	// Считаем рёбра.
	for _, edge := range graph.Edges {
		if matchesTarget(edge.From, target) {
			result.OutgoingEdges++
		}

		if matchesTarget(edge.To, target) {
			result.IncomingEdges++
		}
	}

	// Afferent coupling — кто зависит от target.
	// Efferent coupling — от кого зависит target.
	uniqueAfferent := make(map[string]bool)
	uniqueEfferent := make(map[string]bool)

	for _, edge := range graph.Edges {
		if edge.Type == "import" {
			if matchesTarget(edge.To, target) {
				uniqueAfferent[edge.From] = true
			}

			if matchesTarget(edge.From, target) {
				uniqueEfferent[edge.To] = true
			}
		}
	}

	result.AfferentCoupling = len(uniqueAfferent)
	result.EfferentCoupling = len(uniqueEfferent)

	total := result.AfferentCoupling + result.EfferentCoupling
	if total > 0 {
		result.Instability = float64(result.EfferentCoupling) / float64(total)
	}

	// Считаем элементы.
	for _, typeInfo := range a.AllTypes() {
		if matchesTarget(typeInfo.Package, target) {
			result.TypeCount++
		}
	}

	for _, funcInfo := range a.AllFunctions() {
		if matchesTarget(funcInfo.Package, target) {
			result.FunctionCount++
		}
	}

	for _, methodInfo := range a.AllMethods() {
		if matchesTarget(methodInfo.Package, target) {
			result.MethodCount++
		}
	}

	return result, nil
}

// getPackageDependencies собирает зависимости пакета.
func (b *AnalyzerBridge) getPackageDependencies(graph *model.Graph, pkgID string) []DependencySummary {
	if pkgID == "" {
		return nil
	}

	var deps []DependencySummary

	seen := make(map[string]bool)

	for _, edge := range graph.Edges {
		if edge.Type != "import" {
			continue
		}

		if edge.From != pkgID {
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

// generateFileDiagnostics генерирует диагностики для файла.
func (b *AnalyzerBridge) generateFileDiagnostics(graph *model.Graph, _ string, pkgID string) []Diagnostic {
	if pkgID == "" {
		return nil
	}

	var diags []Diagnostic

	// Проверка на высокое количество зависимостей (efferent coupling).
	importCount := 0

	for _, edge := range graph.Edges {
		if edge.From == pkgID && edge.Type == "import" {
			importCount++
		}
	}

	const highCouplingThreshold = 10

	if importCount > highCouplingThreshold {
		diags = append(diags, Diagnostic{
			Range: Range{
				Start: Position{Line: 0, Character: 0},
				End:   Position{Line: 0, Character: 1},
			},
			Severity: SeverityWarning,
			Source:   "archlint",
			Code:     "high-efferent-coupling",
			Message:  fmt.Sprintf("Пакет %s имеет %d зависимостей (порог: %d). Рассмотрите декомпозицию.", pkgID, importCount, highCouplingThreshold),
		})
	}

	// Проверка на циклические зависимости (простой DFS для пакетов).
	diags = append(diags, b.detectCycles(graph, pkgID)...)

	return diags
}

// detectCycles ищет циклические зависимости через import-рёбра.
func (b *AnalyzerBridge) detectCycles(graph *model.Graph, startPkg string) []Diagnostic {
	// Строим adjacency list только из import-рёбер.
	adj := make(map[string][]string)

	for _, edge := range graph.Edges {
		if edge.Type == "import" {
			adj[edge.From] = append(adj[edge.From], edge.To)
		}
	}

	// DFS для поиска циклов от startPkg.
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

		return []Diagnostic{{
			Range: Range{
				Start: Position{Line: 0, Character: 0},
				End:   Position{Line: 0, Character: 1},
			},
			Severity: SeverityError,
			Source:   "archlint",
			Code:     "circular-dependency",
			Message:  fmt.Sprintf("Обнаружена циклическая зависимость: %s", cycle),
		}}
	}

	return nil
}

// matchesFile проверяет, соответствует ли путь файла запрашиваемому.
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

// matchesTarget проверяет, соответствует ли ID запрашиваемому target.
func matchesTarget(id, target string) bool {
	return id == target || strings.HasPrefix(id, target+".") || strings.HasSuffix(id, "/"+target)
}
