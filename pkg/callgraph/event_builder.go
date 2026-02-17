package callgraph

import (
	"errors"
	"fmt"
	"time"

	"github.com/mshogin/archlint/internal/analyzer"
	"github.com/mshogin/archlint/internal/config"
)

// ErrContextNotFound контекст не найден в конфигурации.
var ErrContextNotFound = errors.New("контекст не найден")

// EventBuilder строит графы вызовов для всех событий бизнес-процесса.
type EventBuilder struct {
	builder  *Builder
	contexts *config.BehavioralConfig
}

// NewEventBuilder создает новый строитель событийных графов.
func NewEventBuilder(
	a *analyzer.GoAnalyzer,
	contexts *config.BehavioralConfig,
	opts BuildOptions,
) (*EventBuilder, error) {
	builder, err := NewBuilder(a, opts)
	if err != nil {
		return nil, err
	}

	return &EventBuilder{
		builder:  builder,
		contexts: contexts,
	}, nil
}

// BuildAll строит графы вызовов для всех событий, имеющих маппинг.
func (e *EventBuilder) BuildAll() (*EventCallGraphSet, error) {
	set := &EventCallGraphSet{
		Graphs:      make(map[string]CallGraph),
		GeneratedAt: time.Now(),
	}

	for ctxName, ctx := range e.contexts.Contexts {
		e.buildContextEvents(set, ctxName, ctx.Events)
	}

	return set, nil
}

// BuildForContext строит графы вызовов для одного контекста.
func (e *EventBuilder) BuildForContext(ctxName string) (*EventCallGraphSet, error) {
	ctx, ok := e.contexts.Contexts[ctxName]
	if !ok {
		return nil, fmt.Errorf("%w: %q", ErrContextNotFound, ctxName)
	}

	set := &EventCallGraphSet{
		ProcessID:   ctxName,
		Graphs:      make(map[string]CallGraph),
		GeneratedAt: time.Now(),
	}

	e.buildContextEvents(set, ctxName, ctx.Events)

	return set, nil
}

func (e *EventBuilder) buildContextEvents(set *EventCallGraphSet, ctxName string, events []config.EventMapping) {
	for _, event := range events {
		set.Stats.TotalEvents++

		entryPoint := event.EntryPoint.Package + "." + event.EntryPoint.Function

		cg, err := e.builder.BuildForEvent(event.EventID, event.EventName, entryPoint)
		if err != nil {
			set.Warnings = append(set.Warnings,
				fmt.Sprintf("Контекст %q, событие %q: %v", ctxName, event.EventID, err))
			set.Stats.FailedGraphs++

			continue
		}

		set.Stats.MappedEvents++
		set.Stats.BuiltGraphs++
		set.Stats.TotalNodes += cg.Stats.TotalNodes
		set.Stats.TotalEdges += cg.Stats.TotalEdges

		set.Graphs[event.EventID] = *cg
	}
}
