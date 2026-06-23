package mcp

import (
	"fmt"
	"sort"

	"github.com/mshogin/archlint/internal/model"
)

// OutEdge — типизированное исходящее ребро (To + Type) для DIP-вью.
type OutEdge struct {
	To   string
	Type string
}

// DIPView — факты, нужные DIP-метрике: узлы-интерфейсы, kind узла, типизированные
// исходящие рёбра. graph-agnostic (по духу ADR-0002): метрика гоняется на ЛЮБОМ
// виде графа (исходный / quotient), не зная конкретного типа. DirectedView (узлы+
// соседи, для SCC) для DIP недостаточен — нужны kind + типы рёбер, поэтому свой вид.
type DIPView interface {
	InterfaceNodes() []string
	KindOf(id string) string
	OutEdges(id string) []OutEdge
}

// modelDIPView — адаптер model.Graph -> DIPView.
type modelDIPView struct {
	kind   map[string]string
	out    map[string][]OutEdge
	ifaces []string
}

func newDIPView(g *model.Graph) *modelDIPView {
	v := &modelDIPView{kind: make(map[string]string), out: make(map[string][]OutEdge)}

	for _, n := range g.Nodes {
		if n.Attrs == nil {
			continue
		}
		if k, ok := n.Attrs["kind"].(string); ok {
			v.kind[n.ID] = k
			if k == model.KindInterface {
				v.ifaces = append(v.ifaces, n.ID)
			}
		}
	}

	for _, e := range g.Edges {
		v.out[e.From] = append(v.out[e.From], OutEdge{To: e.To, Type: e.Type})
	}

	sort.Strings(v.ifaces)

	return v
}

func (v *modelDIPView) InterfaceNodes() []string     { return v.ifaces }
func (v *modelDIPView) KindOf(id string) string      { return v.kind[id] }
func (v *modelDIPView) OutEdges(id string) []OutEdge { return v.out[id] }

// detectDIP — нарушения принципа инверсии зависимостей: интерфейс (абстракция)
// ссылается на СВОЙ конкретный тип (деталь) в сигнатуре своего метода — ребро
// usesType/returns от интерфейса к узлу kind=concrete.
//
// Направление ошибки (соундность > полнота): цель не-concrete (интерфейс/внешний/
// нерезолвимый — у внешних нет kind=concrete-узла, рёбра ведут только на резолвимые
// СВОИ типы) -> НЕ нарушение. Только свой concrete -> нарушение. Ложного firing на
// легальном (abstraction->abstraction, примитивы) нет.
//
// SEVERITY: WARNING (verified self-проверкой). Две оси держать РАЗДЕЛЬНО:
//   - W1 construct-validity [интенсионал/элементы]: term(m) ⊆ term(Def_DIP) — метрика читает ровно
//     язык принципа (рёбра abstract->concrete). ВЫПОЛНЕН.
//   - precision [экстенсионал/исходы]: fire(m) ⊇ viol_DIP, строго fire = viol ∪ {DTO-зависимости}
//     (DTO=concrete БЕЗ поведения, вне Def_DIP -> legal FP). precision<1 -> WARNING, не ERROR.
//
// Демотация обоснована ЭКСТЕНСИОНАЛОМ (precision<1), НЕ дефектом интенсионала (W1 ok).
func detectDIP(v DIPView) []Violation {
	var out []Violation

	seen := make(map[[2]string]bool)

	for _, iface := range v.InterfaceNodes() {
		for _, e := range v.OutEdges(iface) {
			if e.Type != model.EdgeUses && e.Type != model.EdgeReturns {
				continue
			}
			if e.To == iface || v.KindOf(e.To) != model.KindConcrete {
				continue
			}

			key := [2]string{iface, e.To}
			if seen[key] {
				continue
			}
			seen[key] = true

			out = append(out, Violation{
				Kind:    "dip-abstraction-to-detail",
				Message: fmt.Sprintf("DIP: interface %s references concrete type %s in a method signature (the abstraction depends on a detail)", iface, e.To),
				Target:  iface,
			})
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Message < out[j].Message })

	return out
}

// DetectDIP — порт DIP-метрики (Тир1, Фаза 3) на model.Graph через DIPView.
func DetectDIP(g *model.Graph) []Violation { return detectDIP(newDIPView(g)) }
