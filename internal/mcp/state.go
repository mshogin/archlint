// Package mcp implements an MCP (Model Context Protocol) server for archlint.
package mcp

import (
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
}

// NewState creates a new State instance.
func NewState() *State {
	return &State{}
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

	return nil
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

	return nil
}

// GetGraph returns a copy of the current graph.
func (s *State) GetGraph() *model.Graph {
	s.mu.RLock()
	defer s.mu.RUnlock()

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
