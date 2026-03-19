package mcp

import (
	"path/filepath"
	"strings"

	"github.com/mshogin/archlint/internal/analyzer"
	"github.com/mshogin/archlint/internal/model"
)

// FileMetrics holds rich per-file architecture metrics.
type FileMetrics struct {
	FilePath string `json:"filePath"`
	Package  string `json:"package"`

	// Coupling metrics.
	AfferentCoupling int     `json:"afferentCoupling"` // Ca: incoming dependencies
	EfferentCoupling int     `json:"efferentCoupling"` // Ce: outgoing dependencies
	Instability      float64 `json:"instability"`      // I = Ce / (Ca + Ce)
	Abstractness     float64 `json:"abstractness"`     // ratio of interfaces to total types
	MainSeqDistance  float64 `json:"mainSeqDistance"`   // |A + I - 1|

	// Size metrics.
	Types     int `json:"types"`
	Functions int `json:"functions"`
	Methods   int `json:"methods"`
	Fields    int `json:"fields"` // total across all types

	// SOLID violations.
	SRPViolations []Violation `json:"srpViolations,omitempty"`
	DIPViolations []Violation `json:"dipViolations,omitempty"`
	ISPViolations []Violation `json:"ispViolations,omitempty"`

	// Code smells.
	GodClasses     []string `json:"godClasses,omitempty"`
	HubNodes       []string `json:"hubNodes,omitempty"`
	OrphanNodes    []string `json:"orphanNodes,omitempty"`
	FeatureEnvy    []string `json:"featureEnvy,omitempty"`
	ShotgunSurgery []string `json:"shotgunSurgery,omitempty"`

	// Structural metrics.
	CyclicDeps   []string `json:"cyclicDeps,omitempty"`
	MaxCallDepth int      `json:"maxCallDepth"`
	FanIn        int      `json:"fanIn"`
	FanOut       int      `json:"fanOut"`

	// Overall health score (0-100, higher is better).
	HealthScore int `json:"healthScore"`
}

// SOLID and smell thresholds.
const (
	srpMethodThreshold = 7
	srpFieldThreshold  = 10
	ispMethodThreshold = 5
	godMethodThreshold = 15
	godFieldThreshold  = 20
	godFanOutThreshold = 10
	hubThreshold       = 15
	shotgunThreshold   = 5
)

// ComputeFileMetrics computes full metrics for a single file.
func ComputeFileMetrics(filePath string, a *analyzer.GoAnalyzer, graph *model.Graph) *FileMetrics {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		absPath = filePath
	}

	m := &FileMetrics{
		FilePath: absPath,
	}

	// Collect all node IDs belonging to this file.
	fileNodeIDs := collectFileNodeIDs(absPath, a)

	// Determine package.
	m.Package = determinePackage(absPath, a)

	// Size metrics.
	computeSizeMetrics(m, absPath, a)

	// Coupling metrics (package-level).
	computeCouplingMetrics(m, graph)

	// Fan-in and fan-out (file-level, based on call edges).
	computeFanMetrics(m, fileNodeIDs, graph)

	// SOLID checks.
	computeSRPViolations(m, absPath, a)
	computeDIPViolations(m, absPath, a)
	computeISPViolations(m, absPath, a)

	// Code smell detection.
	computeGodClasses(m, absPath, a, fileNodeIDs, graph)
	computeHubNodes(m, fileNodeIDs, graph)
	computeOrphanNodes(m, fileNodeIDs, graph)
	computeFeatureEnvy(m, absPath, a)
	computeShotgunSurgery(m, absPath, a, graph)

	// Structural metrics.
	computeCyclicDeps(m, graph)
	computeMaxCallDepth(m, fileNodeIDs, graph)

	// Health score.
	m.HealthScore = computeHealthScore(m)

	return m
}

// ComputeAllFileMetrics computes metrics for every file in the project.
func ComputeAllFileMetrics(a *analyzer.GoAnalyzer, graph *model.Graph) map[string]*FileMetrics {
	files := collectAllFiles(a)
	result := make(map[string]*FileMetrics, len(files))

	for _, f := range files {
		result[f] = ComputeFileMetrics(f, a, graph)
	}

	return result
}

func collectAllFiles(a *analyzer.GoAnalyzer) []string {
	seen := make(map[string]bool)

	for _, t := range a.AllTypes() {
		abs, err := filepath.Abs(t.File)
		if err == nil {
			seen[abs] = true
		}
	}

	for _, f := range a.AllFunctions() {
		abs, err := filepath.Abs(f.File)
		if err == nil {
			seen[abs] = true
		}
	}

	for _, m := range a.AllMethods() {
		abs, err := filepath.Abs(m.File)
		if err == nil {
			seen[abs] = true
		}
	}

	result := make([]string, 0, len(seen))
	for f := range seen {
		result = append(result, f)
	}

	return result
}

func collectFileNodeIDs(absPath string, a *analyzer.GoAnalyzer) map[string]bool {
	ids := make(map[string]bool)

	for id, t := range a.AllTypes() {
		if matchesFile(t.File, absPath) {
			ids[id] = true
		}
	}

	for id, f := range a.AllFunctions() {
		if matchesFile(f.File, absPath) {
			ids[id] = true
		}
	}

	for id, m := range a.AllMethods() {
		if matchesFile(m.File, absPath) {
			ids[id] = true
		}
	}

	return ids
}

func determinePackage(absPath string, a *analyzer.GoAnalyzer) string {
	for _, t := range a.AllTypes() {
		if matchesFile(t.File, absPath) {
			return t.Package
		}
	}

	for _, f := range a.AllFunctions() {
		if matchesFile(f.File, absPath) {
			return f.Package
		}
	}

	for _, m := range a.AllMethods() {
		if matchesFile(m.File, absPath) {
			return m.Package
		}
	}

	return ""
}

func computeSizeMetrics(m *FileMetrics, absPath string, a *analyzer.GoAnalyzer) {
	for _, t := range a.AllTypes() {
		if !matchesFile(t.File, absPath) {
			continue
		}

		m.Types++
		m.Fields += len(t.Fields)
	}

	for _, f := range a.AllFunctions() {
		if matchesFile(f.File, absPath) {
			m.Functions++
		}
	}

	for _, method := range a.AllMethods() {
		if matchesFile(method.File, absPath) {
			m.Methods++
		}
	}
}

func computeCouplingMetrics(m *FileMetrics, graph *model.Graph) {
	if m.Package == "" {
		return
	}

	// Afferent coupling: who depends on this package.
	// Efferent coupling: what this package depends on.
	seenCa := make(map[string]bool)
	seenCe := make(map[string]bool)

	for _, edge := range graph.Edges {
		if edge.Type != "import" {
			continue
		}

		if edge.To == m.Package && edge.From != m.Package {
			if !seenCa[edge.From] {
				seenCa[edge.From] = true
				m.AfferentCoupling++
			}
		}

		if edge.From == m.Package && edge.To != m.Package {
			if !seenCe[edge.To] {
				seenCe[edge.To] = true
				m.EfferentCoupling++
			}
		}
	}

	total := m.AfferentCoupling + m.EfferentCoupling
	if total > 0 {
		m.Instability = float64(m.EfferentCoupling) / float64(total)
	}

	// Abstractness: ratio of interfaces to total types in the package.
	totalTypes := 0
	interfaces := 0

	for _, t := range graph.Nodes {
		if !matchesTarget(t.ID, m.Package) {
			continue
		}

		switch t.Entity {
		case "struct", "interface", "type":
			totalTypes++
			if t.Entity == "interface" {
				interfaces++
			}
		}
	}

	if totalTypes > 0 {
		m.Abstractness = float64(interfaces) / float64(totalTypes)
	}

	// Distance from main sequence: |A + I - 1|.
	d := m.Abstractness + m.Instability - 1.0
	if d < 0 {
		d = -d
	}

	m.MainSeqDistance = d
}

func computeFanMetrics(m *FileMetrics, nodeIDs map[string]bool, graph *model.Graph) {
	for _, edge := range graph.Edges {
		if edge.Type != "calls" {
			continue
		}

		if nodeIDs[edge.To] && !nodeIDs[edge.From] {
			m.FanIn++
		}

		if nodeIDs[edge.From] && !nodeIDs[edge.To] {
			m.FanOut++
		}
	}
}

func computeSRPViolations(m *FileMetrics, absPath string, a *analyzer.GoAnalyzer) {
	for typeID, t := range a.AllTypes() {
		if !matchesFile(t.File, absPath) || t.Kind != "struct" {
			continue
		}

		// Count methods on this type.
		methodCount := 0
		pkgsUsed := make(map[string]bool)

		for _, method := range a.AllMethods() {
			if method.Package == t.Package && method.Receiver == t.Name {
				methodCount++

				for _, call := range method.Calls {
					if call.IsMethod && call.Receiver != "" {
						pkgsUsed[call.Receiver] = true
					}
				}
			}
		}

		if methodCount > srpMethodThreshold {
			m.SRPViolations = append(m.SRPViolations, Violation{
				Kind:    "srp-too-many-methods",
				Message: strings.Join([]string{"Type ", t.Name, " has too many methods (", intToStr(methodCount), " > ", intToStr(srpMethodThreshold), ")"}, ""),
				Target:  typeID,
			})
		}

		if len(t.Fields) > srpFieldThreshold {
			m.SRPViolations = append(m.SRPViolations, Violation{
				Kind:    "srp-too-many-fields",
				Message: strings.Join([]string{"Type ", t.Name, " has too many fields (", intToStr(len(t.Fields)), " > ", intToStr(srpFieldThreshold), ")"}, ""),
				Target:  typeID,
			})
		}
	}
}

func computeDIPViolations(m *FileMetrics, absPath string, a *analyzer.GoAnalyzer) {
	// Flag functions/methods that accept concrete struct types as parameters.
	// This is a simplified check: look for function calls that directly reference struct types.
	for _, t := range a.AllTypes() {
		if !matchesFile(t.File, absPath) || t.Kind != "struct" {
			continue
		}

		// Check fields for concrete dependencies where interfaces might be expected.
		for _, field := range t.Fields {
			if field.TypePkg != "" {
				// External concrete type dependency — potential DIP violation.
				resolvedType := ""

				for tID, tInfo := range a.AllTypes() {
					if tInfo.Name == strings.Split(field.TypeName, ".")[len(strings.Split(field.TypeName, "."))-1] &&
						tInfo.Kind == "struct" && tInfo.Package != t.Package {
						resolvedType = tID

						break
					}
				}

				if resolvedType != "" {
					m.DIPViolations = append(m.DIPViolations, Violation{
						Kind:    "dip-concrete-dependency",
						Message: strings.Join([]string{"Type ", t.Name, " depends on concrete type ", field.TypeName, " — consider depending on an interface"}, ""),
						Target:  t.Package + "." + t.Name,
					})
				}
			}
		}
	}
}

func computeISPViolations(m *FileMetrics, absPath string, a *analyzer.GoAnalyzer) {
	for typeID, t := range a.AllTypes() {
		if !matchesFile(t.File, absPath) || t.Kind != "interface" {
			continue
		}

		// Count methods on the interface by counting method nodes contained by it.
		methodCount := 0

		for _, method := range a.AllMethods() {
			if method.Package == t.Package && method.Receiver == t.Name {
				methodCount++
			}
		}

		// Also count interface methods from fields (embedded methods appear as fields in AST).
		// For interfaces parsed by the analyzer, we count the fields as method signatures.
		methodCount += len(t.Fields)

		if methodCount > ispMethodThreshold {
			m.ISPViolations = append(m.ISPViolations, Violation{
				Kind:    "isp-fat-interface",
				Message: strings.Join([]string{"Interface ", t.Name, " has too many methods (", intToStr(methodCount), " > ", intToStr(ispMethodThreshold), ")"}, ""),
				Target:  typeID,
			})
		}
	}
}

func computeGodClasses(m *FileMetrics, absPath string, a *analyzer.GoAnalyzer, nodeIDs map[string]bool, graph *model.Graph) {
	for typeID, t := range a.AllTypes() {
		if !matchesFile(t.File, absPath) || t.Kind != "struct" {
			continue
		}

		methodCount := 0

		for _, method := range a.AllMethods() {
			if method.Package == t.Package && method.Receiver == t.Name {
				methodCount++
			}
		}

		// Fan-out for this type's methods.
		typeMethodIDs := make(map[string]bool)

		for mID, method := range a.AllMethods() {
			if method.Package == t.Package && method.Receiver == t.Name {
				typeMethodIDs[mID] = true
			}
		}

		fanOut := 0

		for _, edge := range graph.Edges {
			if edge.Type == "calls" && typeMethodIDs[edge.From] && !typeMethodIDs[edge.To] {
				fanOut++
			}
		}

		if methodCount > godMethodThreshold || len(t.Fields) > godFieldThreshold || fanOut > godFanOutThreshold {
			m.GodClasses = append(m.GodClasses, typeID)
		}
	}
}

func computeHubNodes(m *FileMetrics, nodeIDs map[string]bool, graph *model.Graph) {
	for nodeID := range nodeIDs {
		fanIn := 0
		fanOut := 0

		for _, edge := range graph.Edges {
			if edge.Type != "calls" {
				continue
			}

			if edge.To == nodeID {
				fanIn++
			}

			if edge.From == nodeID {
				fanOut++
			}
		}

		if fanIn+fanOut > hubThreshold {
			m.HubNodes = append(m.HubNodes, nodeID)
		}
	}
}

func computeOrphanNodes(m *FileMetrics, nodeIDs map[string]bool, graph *model.Graph) {
	for nodeID := range nodeIDs {
		connected := false

		for _, edge := range graph.Edges {
			if edge.Type == "contains" {
				continue // skip containment edges
			}

			if edge.From == nodeID || edge.To == nodeID {
				connected = true

				break
			}
		}

		if !connected {
			m.OrphanNodes = append(m.OrphanNodes, nodeID)
		}
	}
}

func computeFeatureEnvy(m *FileMetrics, absPath string, a *analyzer.GoAnalyzer) {
	for methodID, method := range a.AllMethods() {
		if !matchesFile(method.File, absPath) {
			continue
		}

		ownCalls := 0
		otherCalls := 0

		for _, call := range method.Calls {
			if call.IsMethod {
				if call.Receiver == method.Receiver || call.Receiver == "" {
					ownCalls++
				} else {
					otherCalls++
				}
			}
		}

		if otherCalls > ownCalls && otherCalls > 2 {
			m.FeatureEnvy = append(m.FeatureEnvy, methodID)
		}
	}
}

func computeShotgunSurgery(m *FileMetrics, absPath string, a *analyzer.GoAnalyzer, graph *model.Graph) {
	for typeID, t := range a.AllTypes() {
		if !matchesFile(t.File, absPath) || t.Kind != "struct" {
			continue
		}

		// Count how many distinct files have nodes that call methods of this type.
		typeMethodIDs := make(map[string]bool)

		for mID, method := range a.AllMethods() {
			if method.Package == t.Package && method.Receiver == t.Name {
				typeMethodIDs[mID] = true
			}
		}

		callerFiles := make(map[string]bool)

		for _, edge := range graph.Edges {
			if edge.Type != "calls" || !typeMethodIDs[edge.To] {
				continue
			}

			// Find file of the calling node.
			for _, f := range a.AllFunctions() {
				for fID := range a.AllFunctions() {
					if fID == edge.From {
						callerFiles[f.File] = true
					}
				}
			}

			for mID, method := range a.AllMethods() {
				if mID == edge.From {
					callerFiles[method.File] = true
				}
			}
		}

		// Exclude the file itself.
		delete(callerFiles, t.File)
		abs, _ := filepath.Abs(t.File)
		delete(callerFiles, abs)

		if len(callerFiles) > shotgunThreshold {
			m.ShotgunSurgery = append(m.ShotgunSurgery, typeID)
		}
	}
}

func computeCyclicDeps(m *FileMetrics, graph *model.Graph) {
	if m.Package == "" {
		return
	}

	violations := detectCycles(graph, m.Package)
	for _, v := range violations {
		m.CyclicDeps = append(m.CyclicDeps, v.Message)
	}
}

func computeMaxCallDepth(m *FileMetrics, nodeIDs map[string]bool, graph *model.Graph) {
	// Build call adjacency from graph edges.
	adj := make(map[string][]string)

	for _, edge := range graph.Edges {
		if edge.Type == "calls" {
			adj[edge.From] = append(adj[edge.From], edge.To)
		}
	}

	maxDepth := 0

	for nodeID := range nodeIDs {
		depth := dfsMaxDepth(nodeID, adj, make(map[string]bool), 0, 50)
		if depth > maxDepth {
			maxDepth = depth
		}
	}

	m.MaxCallDepth = maxDepth
}

func dfsMaxDepth(node string, adj map[string][]string, visited map[string]bool, current, limit int) int {
	if current >= limit || visited[node] {
		return current
	}

	visited[node] = true

	maxD := current

	for _, next := range adj[node] {
		d := dfsMaxDepth(next, adj, visited, current+1, limit)
		if d > maxD {
			maxD = d
		}
	}

	visited[node] = false

	return maxD
}

func computeHealthScore(m *FileMetrics) int {
	score := 100

	score -= 5 * len(m.SRPViolations)
	score -= 10 * len(m.GodClasses)
	score -= 10 * len(m.CyclicDeps)
	score -= 5 * len(m.HubNodes)
	score -= 3 * len(m.ISPViolations)
	score -= 3 * len(m.DIPViolations)
	score -= 2 * len(m.FeatureEnvy)

	if m.Instability > 0.8 {
		score -= 5
	}

	if m.MainSeqDistance > 0.5 {
		score -= 5
	}

	if score < 0 {
		score = 0
	}

	return score
}

// intToStr converts an int to a string without importing strconv (avoiding circular deps).
func intToStr(n int) string {
	if n == 0 {
		return "0"
	}

	neg := false
	if n < 0 {
		neg = true
		n = -n
	}

	var digits []byte

	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}

	if neg {
		digits = append([]byte{'-'}, digits...)
	}

	return string(digits)
}
