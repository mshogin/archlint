package callgraph

import (
	"fmt"
	"time"

	"github.com/mshogin/archlint/internal/analyzer"
)

// BuildOptions настройки построения графа.
type BuildOptions struct {
	MaxDepth          int
	IncludeExternal   bool
	IncludeStdlib     bool
	ExcludePackages   []string
	ResolveInterfaces bool
	TrackGoroutines   bool
}

// DefaultBuildOptions возвращает настройки по умолчанию.
func DefaultBuildOptions() BuildOptions {
	return BuildOptions{
		MaxDepth:          10,
		ResolveInterfaces: true,
		TrackGoroutines:   true,
	}
}

// Builder строит граф вызовов от заданной точки входа.
type Builder struct {
	analyzer *analyzer.GoAnalyzer
	options  BuildOptions
}

// NewBuilder создает новый строитель.
func NewBuilder(a *analyzer.GoAnalyzer, opts BuildOptions) (*Builder, error) {
	if a == nil {
		return nil, ErrAnalyzerRequired
	}

	if opts.MaxDepth < 1 || opts.MaxDepth > 50 {
		return nil, fmt.Errorf("%w: %d", ErrInvalidMaxDepth, opts.MaxDepth)
	}

	return &Builder{
		analyzer: a,
		options:  opts,
	}, nil
}

// Build строит граф вызовов от указанной точки входа.
func (b *Builder) Build(entryPoint string) (*CallGraph, error) {
	return b.buildGraph("", "", entryPoint)
}

// BuildForEvent строит граф вызовов для BPMN-события.
func (b *Builder) BuildForEvent(eventID, eventName, entryPoint string) (*CallGraph, error) {
	return b.buildGraph(eventID, eventName, entryPoint)
}

func (b *Builder) buildGraph(eventID, eventName, entryPoint string) (*CallGraph, error) {
	start := time.Now()

	if b.analyzer.LookupFunction(entryPoint) == nil && b.analyzer.LookupMethod(entryPoint) == nil {
		err := fmt.Errorf("%w: %s", ErrEntryPointNotFound, entryPoint)

		return nil, err
	}

	resolver := NewCallResolver(b.analyzer)
	walker := NewCallWalker(b.analyzer, resolver, b.options.MaxDepth)

	walker.Walk(entryPoint)
	nodes, edges, stats, warnings := walker.Results()

	cg := &CallGraph{
		EventID:     eventID,
		EventName:   eventName,
		EntryPoint:  entryPoint,
		Nodes:       nodes,
		Edges:       edges,
		MaxDepth:    b.options.MaxDepth,
		ActualDepth: stats.MaxDepthReached,
		Stats:       stats,
		Warnings:    warnings,
		BuildTime:   time.Since(start),
	}

	return cg, nil
}
