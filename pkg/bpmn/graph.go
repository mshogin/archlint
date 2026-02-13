package bpmn

import (
	"fmt"

	"github.com/mshogin/archlint/pkg/tracer"
)

// ProcessGraph - направленный граф бизнес-процесса.
type ProcessGraph struct {
	Process     *BPMNProcess
	Adjacency   map[string][]string
	InDegree    map[string]int
	ElementByID map[string]*BPMNElement
}

// BuildGraph строит направленный граф из BPMNProcess.
func BuildGraph(process *BPMNProcess) (*ProcessGraph, error) {
	tracer.Enter("BuildGraph")

	if process == nil {
		tracer.ExitError("BuildGraph", ErrNilProcess)

		return nil, ErrNilProcess
	}

	g := &ProcessGraph{
		Process:     process,
		Adjacency:   make(map[string][]string),
		InDegree:    make(map[string]int),
		ElementByID: make(map[string]*BPMNElement),
	}

	for i := range process.Elements {
		elem := &process.Elements[i]
		g.ElementByID[elem.ID] = elem
		g.InDegree[elem.ID] = 0
	}

	for _, flow := range process.Flows {
		g.Adjacency[flow.SourceRef] = append(g.Adjacency[flow.SourceRef], flow.TargetRef)

		if _, exists := g.ElementByID[flow.TargetRef]; exists {
			g.InDegree[flow.TargetRef]++
		}
	}

	tracer.ExitSuccess("BuildGraph")

	return g, nil
}

// Successors возвращает список ID элементов-потомков.
func (g *ProcessGraph) Successors(elementID string) []string {
	tracer.Enter("ProcessGraph.Successors")
	tracer.ExitSuccess("ProcessGraph.Successors")

	return g.Adjacency[elementID]
}

// Predecessors возвращает список ID элементов-предков.
func (g *ProcessGraph) Predecessors(elementID string) []string {
	tracer.Enter("ProcessGraph.Predecessors")

	var result []string

	for src, targets := range g.Adjacency {
		for _, tgt := range targets {
			if tgt == elementID {
				result = append(result, src)
			}
		}
	}

	tracer.ExitSuccess("ProcessGraph.Predecessors")

	return result
}

// StartEvents возвращает все стартовые события процесса.
func (g *ProcessGraph) StartEvents() []*BPMNElement {
	tracer.Enter("ProcessGraph.StartEvents")

	var result []*BPMNElement

	for _, elem := range g.ElementByID {
		if elem.Type == StartEvent {
			result = append(result, elem)
		}
	}

	tracer.ExitSuccess("ProcessGraph.StartEvents")

	return result
}

// EndEvents возвращает все конечные события процесса.
func (g *ProcessGraph) EndEvents() []*BPMNElement {
	tracer.Enter("ProcessGraph.EndEvents")

	var result []*BPMNElement

	for _, elem := range g.ElementByID {
		if elem.Type == EndEvent {
			result = append(result, elem)
		}
	}

	tracer.ExitSuccess("ProcessGraph.EndEvents")

	return result
}

// Validate проверяет корректность графа.
func (g *ProcessGraph) Validate() []error {
	tracer.Enter("ProcessGraph.Validate")

	var errs []error

	if len(g.StartEvents()) == 0 {
		errs = append(errs, ErrNoStartEvent)
	}

	if len(g.EndEvents()) == 0 {
		errs = append(errs, ErrNoEndEvent)
	}

	for _, flow := range g.Process.Flows {
		if _, exists := g.ElementByID[flow.SourceRef]; !exists {
			errs = append(errs, fmt.Errorf("%w: flow %s sourceRef %q", ErrBrokenRef, flow.ID, flow.SourceRef))
		}

		if _, exists := g.ElementByID[flow.TargetRef]; !exists {
			errs = append(errs, fmt.Errorf("%w: flow %s targetRef %q", ErrBrokenRef, flow.ID, flow.TargetRef))
		}
	}

	connected := g.reachableFrom()
	for id := range g.ElementByID {
		if !connected[id] {
			errs = append(errs, fmt.Errorf("%w: %q", ErrIsolatedElement, id))
		}
	}

	tracer.ExitSuccess("ProcessGraph.Validate")

	return errs
}

// reachableFrom возвращает множество элементов, достижимых через потоки
// (в обоих направлениях).
func (g *ProcessGraph) reachableFrom() map[string]bool {
	tracer.Enter("ProcessGraph.reachableFrom")

	visited := make(map[string]bool)

	for _, flow := range g.Process.Flows {
		visited[flow.SourceRef] = true
		visited[flow.TargetRef] = true
	}

	tracer.ExitSuccess("ProcessGraph.reachableFrom")

	return visited
}
