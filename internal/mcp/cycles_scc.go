package mcp

import (
	"sync"

	"github.com/mshogin/archlint/internal/model"
	"gonum.org/v1/gonum/graph/simple"
	"gonum.org/v1/gonum/graph/topo"
)

// DirectedView — graph-agnostic вид направленного графа для метрик (по образцу
// gonum graph.Directed): множество узлов + соседи. Метрика принимает ИНТЕРФЕЙС,
// не конкретный model.Graph -> одна метрика гоняется на исходном графе, quotient-
// графе и т.д., не зная конкретного типа (ADR-0002 Этап 1).
type DirectedView interface {
	NodeIDs() []string
	Successors(id string) []string
}

// importView — адаптер model.Graph по import-рёбрам к DirectedView.
type importView struct {
	nodes []string
	adj   map[string][]string
}

func newImportView(g *model.Graph) *importView {
	adj := make(map[string][]string)
	set := make(map[string]bool)

	for _, e := range g.Edges {
		if e.Type == model.EdgeImport {
			adj[e.From] = append(adj[e.From], e.To)
			set[e.From] = true
			set[e.To] = true
		}
	}

	nodes := make([]string, 0, len(set))
	for n := range set {
		nodes = append(nodes, n)
	}

	return &importView{nodes: nodes, adj: adj}
}

func (v *importView) NodeIDs() []string            { return v.nodes }
func (v *importView) Successors(id string) []string { return v.adj[id] }

// sccResult — индекс SCC: cyclic-членство + члены SCC каждого узла. Считается
// ОДИН раз на граф (SCC не зависит от стартового узла —).
type sccResult struct {
	cyclic  map[string]bool
	members map[string][]string // node -> члены его SCC (для SCC>1); пусто для self-loop
}

// computeSCC строит SCC-индекс по graph-agnostic виду через Tarjan (gonum).
// Узел в цикле ⟺ SCC>1 (взаимно достижимы) ИЛИ петля (self-loop). Соундно+полно.
func computeSCC(view DirectedView) *sccResult {
	g := simple.NewDirectedGraph()
	ids := make(map[string]int64)
	names := make(map[int64]string)
	self := make(map[string]bool)

	var next int64
	idOf := func(s string) int64 {
		if v, ok := ids[s]; ok {
			return v
		}
		v := next
		next++
		ids[s] = v
		names[v] = s
		g.AddNode(simple.Node(v))
		return v
	}

	for _, n := range view.NodeIDs() {
		idOf(n)
	}
	for _, from := range view.NodeIDs() {
		for _, to := range view.Successors(from) {
			if from == to {
				self[from] = true
				continue
			}
			g.SetEdge(simple.Edge{F: simple.Node(idOf(from)), T: simple.Node(idOf(to))})
		}
	}

	res := &sccResult{cyclic: make(map[string]bool), members: make(map[string][]string)}

	for _, comp := range topo.TarjanSCC(g) {
		if len(comp) <= 1 {
			continue
		}

		mem := make([]string, 0, len(comp))
		for _, n := range comp {
			mem = append(mem, names[n.ID()])
		}
		for _, n := range comp {
			res.cyclic[names[n.ID()]] = true
			res.members[names[n.ID()]] = mem
		}
	}

	for n := range self {
		res.cyclic[n] = true // петля = цикл (members пусто -> сам узел)
	}

	return res
}

// sccMemo мемоизирует SCC-индекс по указателю графа: detectCycles зовётся по
// каждому пакету (P раз), но SCC одинаков -> считаем ОДИН раз на граф.
// Допущение: граф в рамках анализа неизменен (строится один раз, читается).
var sccMemo sync.Map // *model.Graph -> *sccResult

func cyclicSCC(g *model.Graph) *sccResult {
	if r, ok := sccMemo.Load(g); ok {
		return r.(*sccResult)
	}

	r := computeSCC(newImportView(g))
	sccMemo.Store(g, r)

	return r
}
