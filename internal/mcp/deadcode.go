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
// IMPLEMENTS-DISPATCH (2-й вход проверки соундности, destruction-критично): при достижении
// интерфейса I раскрываем на ВСЕ реализующие типы T (обратно по implements T->I)
// и далее их методы (via contains). Иначе реализация, вызываемая только через
// i.Foo() (без прямого calls-ребра, т.к. var-тип не резолвится), была бы
// ложно-мёртвой -> удаление живого. implements over-approx по имени -> dispatch
// тоже over-approx -> ложно-живой (дёшево), ложно-мёртвый невозможен.
//
// SEVERITY: ПРОМОТИРОВАН в severity_class=ERROR (open-world условно-соундный, обязательны
// дельта+human-in-loop). Self-проверка соундности пройдена: исторически 89->5 (0 false-dead),
// свежий self-прогон 2026-06-15 -> 0 находок (5 реальных мёртвых вычищены). Защита от
// false-dead (destruction) — три over-approx (references / implements-dispatch / exported-entry),
// доказаны юнит-тестами (TestDeadCode_ImplementsDispatch и др.). Разметка: golden id:dead-code.
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
				Message: fmt.Sprintf("dead code: %s is unreachable from entry points R", n.ID),
				Target:  n.ID,
			})
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Target < out[j].Target })

	return out
}
