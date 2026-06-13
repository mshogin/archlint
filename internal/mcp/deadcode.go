package mcp

import (
	"fmt"
	"sort"

	"github.com/mshogin/archlint/internal/model"
)

// DeadCode — метрика мёртвого кода (Фаза 3): узлы func/method, недостижимые от
// множества entry points R по рёбрам calls ∪ references ∪ usesType ∪ returns ∪
// contains, С РАСКРЫТИЕМ IMPLEMENTS-DISPATCH.
//
// ★IMPLEMENTS-DISPATCH (вход 2 горнила, destruction-критично): при достижении
// интерфейса I раскрываем на ВСЕ реализующие типы T (обратно по implements T->I)
// и далее их методы (via contains). Иначе реализация, вызываемая только через
// i.Foo() (без прямого calls-ребра, т.к. var-тип не резолвится), была бы
// ложно-мёртвой -> удаление живого. implements over-approx по имени -> dispatch
// тоже over-approx -> ложно-живой (дёшево), ложно-мёртвый невозможен.
//
// SEVERITY: метрика ТОЛЬКО считает; класс WARNING пока (: ERROR после
// прохождения self-горнила соундности).
func DeadCode(g *model.Graph, configPatterns []string) []Violation {
	r := EntryPoints(g, configPatterns)

	kind := make(map[string]string)
	for _, n := range g.Nodes {
		if n.Attrs != nil {
			if k, ok := n.Attrs["kind"].(string); ok {
				kind[n.ID] = k
			}
		}
	}

	// Прямые рёбра достижимости + обратный индекс implements для dispatch.
	fwd := make(map[string][]string)
	implementers := make(map[string][]string) // interface -> [concrete types]
	for _, e := range g.Edges {
		switch e.Type {
		case model.EdgeCalls, model.EdgeReferences, model.EdgeUses, model.EdgeReturns, model.EdgeContains:
			fwd[e.From] = append(fwd[e.From], e.To)
		case model.EdgeImplements: // T -> I; для dispatch нужен обратный обход
			implementers[e.To] = append(implementers[e.To], e.From)
		}
	}

	reached := make(map[string]bool)
	var queue []string
	push := func(id string) {
		if !reached[id] {
			reached[id] = true
			queue = append(queue, id)
		}
	}

	for id := range r {
		push(id)
	}

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]

		for _, to := range fwd[cur] {
			push(to)
		}

		// dispatch: достигнут интерфейс -> все его реализации (и их методы via contains).
		if kind[cur] == model.KindInterface {
			for _, t := range implementers[cur] {
				push(t)
			}
		}
	}

	var out []Violation
	for _, n := range g.Nodes {
		if (n.Entity == "function" || n.Entity == "method") && !reached[n.ID] {
			out = append(out, Violation{
				Kind:    "dead-code",
				Message: fmt.Sprintf("dead code: %s недостижим от entry points R", n.ID),
				Target:  n.ID,
			})
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Target < out[j].Target })

	return out
}
