// Package mcp implements an MCP (Model Context Protocol) server for archlint.
package mcp

import (
	"path/filepath"
	"sync"

	"github.com/mshogin/archlint/internal/analyzer"
	"github.com/mshogin/archlint/internal/model"
)

// State holds the in-memory architecture graph with thread-safe access.
type State struct {
	mu sync.RWMutex

	// graph holds the current architecture graph.
	graph *model.Graph

	// analyzer holds the current GoAnalyzer instance with parsed AST.
	analyzer *analyzer.GoAnalyzer

	// rootDir is the project root directory.
	rootDir string

	// initialized indicates that initial parsing is complete.
	initialized bool

	// fileNodes maps absolute file paths to node IDs defined in that file.
	fileNodes map[string][]string

	// degradation tracks metrics baselines and detects degradation.
	degradation *DegradationDetector
}

// NewState creates a new State instance.
func NewState() *State {
	return &State{
		fileNodes:   make(map[string][]string),
		degradation: NewDegradationDetector(),
	}
}

// Initialize performs initial project parsing.
func (s *State) Initialize(rootDir string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.rootDir = rootDir
	s.analyzer = analyzer.NewGoAnalyzer()

	graph, err := s.analyzer.Analyze(rootDir)
	if err != nil {
		return err
	}

	s.graph = graph
	s.initialized = true
	s.buildFileNodeIndex()

	return nil
}

// InitializeMetricsBaseline computes initial metrics for all files and stores as baseline.
// Must be called after Initialize.
func (s *State) InitializeMetricsBaseline() {
	s.mu.RLock()
	a := s.analyzer
	graph := s.copyGraphLocked()
	s.mu.RUnlock()

	if a == nil {
		return
	}

	metrics := ComputeAllFileMetrics(a, graph)
	s.degradation.SetBaselines(metrics)
}

// Reparse re-parses the entire project and rebuilds the graph.
// For projects up to ~1000 files this takes < 100ms.
func (s *State) Reparse() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.initialized {
		return nil
	}

	newAnalyzer := analyzer.NewGoAnalyzer()

	graph, err := newAnalyzer.Analyze(s.rootDir)
	if err != nil {
		return err
	}

	s.analyzer = newAnalyzer
	s.graph = graph
	s.buildFileNodeIndex()

	return nil
}

// ReparseFile re-parses the entire project (atomically) in response to a
// single file change. This is fast for projects with <1000 files and
// guarantees consistency. After reparsing, it computes fresh metrics for the
// changed file and checks for degradation.
func (s *State) ReparseFile(path string) (*DegradationReport, error) {
	// Full re-analyze and atomic swap.
	if err := s.Reparse(); err != nil {
		return nil, err
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	// Compute degradation for the changed file.
	s.mu.RLock()
	a := s.analyzer
	graph := s.copyGraphLocked()
	s.mu.RUnlock()

	report := s.degradation.Check(absPath, a, graph)

	return report, nil
}

// GetGraph returns a copy of the current graph.
func (s *State) GetGraph() *model.Graph {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.copyGraphLocked()
}

// copyGraphLocked returns a copy of the graph. Caller must hold at least s.mu.RLock().
func (s *State) copyGraphLocked() *model.Graph {
	if s.graph == nil {
		return &model.Graph{}
	}

	nodes := make([]model.Node, len(s.graph.Nodes))
	copy(nodes, s.graph.Nodes)

	edges := make([]model.Edge, len(s.graph.Edges))
	copy(edges, s.graph.Edges)

	return &model.Graph{
		Nodes: nodes,
		Edges: edges,
	}
}

// GetAnalyzer returns the current analyzer (read-only access).
func (s *State) GetAnalyzer() *analyzer.GoAnalyzer {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.analyzer
}

// GetDegradationDetector returns the degradation detector.
func (s *State) GetDegradationDetector() *DegradationDetector {
	return s.degradation
}

// IsInitialized returns true if initial parsing is complete.
func (s *State) IsInitialized() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.initialized
}

// RootDir returns the project root directory.
func (s *State) RootDir() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.rootDir
}

// FileNodeIDs returns node IDs for a given file path.
func (s *State) FileNodeIDs(path string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	absPath, err := filepath.Abs(path)
	if err != nil {
		absPath = path
	}

	ids := s.fileNodes[absPath]
	result := make([]string, len(ids))
	copy(result, ids)

	return result
}

// buildFileNodeIndex rebuilds the fileNodes map from the analyzer data.
// Caller must hold s.mu write lock.
func (s *State) buildFileNodeIndex() {
	s.fileNodes = make(map[string][]string)

	if s.analyzer == nil {
		return
	}

	for id, t := range s.analyzer.AllTypes() {
		absPath, err := filepath.Abs(t.File)
		if err != nil {
			absPath = t.File
		}

		s.fileNodes[absPath] = append(s.fileNodes[absPath], id)
	}

	for id, f := range s.analyzer.AllFunctions() {
		absPath, err := filepath.Abs(f.File)
		if err != nil {
			absPath = f.File
		}

		s.fileNodes[absPath] = append(s.fileNodes[absPath], id)
	}

	for id, m := range s.analyzer.AllMethods() {
		absPath, err := filepath.Abs(m.File)
		if err != nil {
			absPath = m.File
		}

		s.fileNodes[absPath] = append(s.fileNodes[absPath], id)
	}
}

// GraphStats holds graph statistics.
type GraphStats struct {
	TotalNodes   int `json:"totalNodes"`
	TotalEdges   int `json:"totalEdges"`
	Packages     int `json:"packages"`
	Structs      int `json:"structs"`
	Interfaces   int `json:"interfaces"`
	Functions    int `json:"functions"`
	Methods      int `json:"methods"`
	ExternalDeps int `json:"externalDeps"`
}

// Stats returns statistics about the current graph.
func (s *State) Stats() GraphStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.graph == nil {
		return GraphStats{}
	}

	stats := GraphStats{
		TotalNodes: len(s.graph.Nodes),
		TotalEdges: len(s.graph.Edges),
	}

	for _, node := range s.graph.Nodes {
		switch node.Entity {
		case "package":
			stats.Packages++
		case "struct":
			stats.Structs++
		case "interface":
			stats.Interfaces++
		case "function":
			stats.Functions++
		case "method":
			stats.Methods++
		case "external":
			stats.ExternalDeps++
		}
	}

	return stats
}
