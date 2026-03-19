package lsp

import (
	"sync"

	"github.com/mshogin/archlint/internal/analyzer"
	"github.com/mshogin/archlint/internal/model"
)

// State хранит in-memory состояние архитектурного графа.
// Все методы являются потокобезопасными.
type State struct {
	mu sync.RWMutex

	// graph содержит текущий архитектурный граф.
	graph *model.Graph

	// analyzer хранит текущий экземпляр GoAnalyzer с parsed AST.
	analyzer *analyzer.GoAnalyzer

	// rootDir — корневая директория проекта.
	rootDir string

	// fileVersions отслеживает версии открытых файлов.
	fileVersions map[string]int

	// initialized указывает, что начальный парсинг завершён.
	initialized bool
}

// NewState создаёт новый экземпляр State.
func NewState() *State {
	return &State{
		fileVersions: make(map[string]int),
	}
}

// Initialize выполняет первоначальный парсинг проекта.
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

// ReparseFile перепарсивает один файл и обновляет граф.
// Создаёт новый анализатор, парсит весь проект заново (для корректного
// разрешения зависимостей), и заменяет текущий граф.
func (s *State) ReparseFile(_ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.initialized {
		return nil
	}

	// Полный ребилд графа — простая и корректная стратегия.
	// Для проектов типичного размера (до ~1000 файлов) занимает < 100ms.
	newAnalyzer := analyzer.NewGoAnalyzer()

	graph, err := newAnalyzer.Analyze(s.rootDir)
	if err != nil {
		return err
	}

	s.analyzer = newAnalyzer
	s.graph = graph

	return nil
}

// ReparseFiles перепарсивает набор файлов (batch update).
func (s *State) ReparseFiles(_ []string) error {
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

// GetGraph возвращает копию текущего графа.
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

// GetAnalyzer возвращает текущий анализатор (read-only доступ).
// Вызывающий код не должен модифицировать анализатор.
func (s *State) GetAnalyzer() *analyzer.GoAnalyzer {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.analyzer
}

// SetFileVersion обновляет версию файла.
func (s *State) SetFileVersion(uri string, version int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.fileVersions[uri] = version
}

// GetFileVersion возвращает версию файла.
func (s *State) GetFileVersion(uri string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.fileVersions[uri]
}

// IsInitialized возвращает true, если начальный парсинг завершён.
func (s *State) IsInitialized() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.initialized
}

// RootDir возвращает корневую директорию проекта.
func (s *State) RootDir() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	return s.rootDir
}

// Stats возвращает статистику текущего графа.
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

// GraphStats содержит статистику графа.
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
