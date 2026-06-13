package mcp

import (
	"sort"

	"github.com/mshogin/archlint/internal/model"
	"gonum.org/v1/gonum/graph/network"
	"gonum.org/v1/gonum/graph/simple"
)

// Структурные МАГНИТУДНЫЕ дескрипторы — порт Python validator (NetworkX) -> Go
// (направление A, «перенос вычислений»). Считаются на ВСЁМ архитектурном графе:
// узлы = все components, рёбра = все links, ПАРАЛЛЕЛЬНЫЕ (from,to) СХЛОПНУТЫ в одно
// (как nx.DiGraph). Эквивалентность гарантируется golden'ом против Python-выхода.
//
// ★СЕВЕРИТИ: ВСЕ дескрипторы — СИГНАЛЫ (INFO / регрессия-WARNING), НЕ ERROR. Это
// магнитуды (ось паттерн/магнитуда, DR-0009): порог произволен, абсолютное значение
// не блокирует. Они НЕ регистрируются в severity_class и НЕ участвуют в дельта-гейте
// как блокирующие. Назначение — наблюдаемость/тренд, не гейт.

// Descriptors — снимок магнитудных дескрипторов графа.
type Descriptors struct {
	NodeCount        int                `json:"nodeCount"`
	EdgeCount        int                `json:"edgeCount"`        // распараллеленные (from,to) схлопнуты
	Density          float64            `json:"density"`          // E / (V*(V-1)), directed
	AfferentCoupling map[string]int     `json:"afferentCoupling"` // Ca = in-degree
	EfferentCoupling map[string]int     `json:"efferentCoupling"` // Ce = out-degree
	Instability      map[string]float64 `json:"instability"`      // I = Ce / (Ca+Ce)
	PageRank         map[string]float64 `json:"pageRank"`         // nx.pagerank(alpha=0.85)
	Betweenness      map[string]float64 `json:"betweenness"`      // nx betweenness, normalized
}

// descriptorGraph — промежуточное представление: упорядоченные узлы + дедуплицированные
// directed-рёбра. Изолирует совпадение с nx.DiGraph (схлопывание параллельных рёбер).
type descriptorGraph struct {
	nodes     []string                   // отсортированные уникальные ID узлов
	index     map[string]int             // ID -> позиция (детерминированный node-id для gonum)
	outAdj    map[string]map[string]bool // from -> set(to), без петель и дублей
	inDeg     map[string]int
	outDeg    map[string]int
	edgeCount int
}

// buildDescriptorGraph собирает граф как Python graph_loader: все Nodes + конечные
// точки рёбер как узлы, рёбра дедуплицированы по (from,to). Петли (from==to) считаются
// в Ca/Ce (как nx in/out_degree), но в gonum-граф для центральностей не добавляются
// (simple.DirectedGraph запрещает self-loop; в арх-графах петли редки).
func buildDescriptorGraph(g *model.Graph) *descriptorGraph {
	dg := &descriptorGraph{
		index:  make(map[string]int),
		outAdj: make(map[string]map[string]bool),
		inDeg:  make(map[string]int),
		outDeg: make(map[string]int),
	}

	seen := make(map[string]bool)
	addNode := func(id string) {
		if !seen[id] {
			seen[id] = true
			dg.nodes = append(dg.nodes, id)
		}
	}

	if g != nil {
		for _, n := range g.Nodes {
			addNode(n.ID)
		}

		for _, e := range g.Edges {
			addNode(e.From)
			addNode(e.To)
		}

		// Дедуп (from,to) -> одно ребро; параллельные типы (calls/uses/import) схлопнуты.
		for _, e := range g.Edges {
			if dg.outAdj[e.From] == nil {
				dg.outAdj[e.From] = make(map[string]bool)
			}

			if dg.outAdj[e.From][e.To] {
				continue // уже учтено
			}

			dg.outAdj[e.From][e.To] = true
			dg.edgeCount++
			dg.outDeg[e.From]++
			dg.inDeg[e.To]++
		}
	}

	sort.Strings(dg.nodes)
	for i, id := range dg.nodes {
		dg.index[id] = i
	}

	return dg
}

// gonumGraph материализует gonum simple.DirectedGraph из дедуплицированного графа,
// ПРОПУСКАЯ петли (для центральностей). node-id = позиция в отсортированном dg.nodes.
func (dg *descriptorGraph) gonumGraph() *simple.DirectedGraph {
	gg := simple.NewDirectedGraph()

	for i := range dg.nodes {
		gg.AddNode(simple.Node(i))
	}

	for from, tos := range dg.outAdj {
		for to := range tos {
			if from == to {
				continue // петля: simple.DirectedGraph запрещает self-edge
			}

			gg.SetEdge(gg.NewEdge(simple.Node(dg.index[from]), simple.Node(dg.index[to])))
		}
	}

	return gg
}

// ComputeDescriptors считает все батч-1 дескрипторы на архитектурном графе.
func ComputeDescriptors(g *model.Graph) Descriptors {
	dg := buildDescriptorGraph(g)

	d := Descriptors{
		NodeCount:        len(dg.nodes),
		EdgeCount:        dg.edgeCount,
		Density:          density(len(dg.nodes), dg.edgeCount),
		AfferentCoupling: make(map[string]int, len(dg.nodes)),
		EfferentCoupling: make(map[string]int, len(dg.nodes)),
		Instability:      make(map[string]float64, len(dg.nodes)),
		PageRank:         make(map[string]float64, len(dg.nodes)),
		Betweenness:      make(map[string]float64, len(dg.nodes)),
	}

	for _, id := range dg.nodes {
		ca := dg.inDeg[id]
		ce := dg.outDeg[id]
		d.AfferentCoupling[id] = ca
		d.EfferentCoupling[id] = ce

		if ca+ce > 0 {
			d.Instability[id] = float64(ce) / float64(ca+ce)
		} else {
			d.Instability[id] = 0.0
		}
	}

	gg := dg.gonumGraph()

	// PageRank: gonum совпадает с nx.pagerank(alpha=0.85) на графах без dangling/петель
	// (нормировка sum=1 у обоих). tol тугой для эпсилон-совпадения.
	for id64, v := range network.PageRank(gg, 0.85, 1e-13) {
		d.PageRank[dg.nodes[int(id64)]] = v
	}

	// Betweenness: gonum даёт НЕнормированную; nx normalized=True (directed) делит на
	// (n-1)(n-2). Применяем ту же нормировку для совпадения.
	bnorm := betweennessNorm(len(dg.nodes))
	for id64, v := range network.Betweenness(gg) {
		d.Betweenness[dg.nodes[int(id64)]] = v * bnorm
	}

	// Узлы без центральности в map gonum (изолированные) -> явный 0.
	for _, id := range dg.nodes {
		if _, ok := d.PageRank[id]; !ok {
			d.PageRank[id] = 0.0
		}

		if _, ok := d.Betweenness[id]; !ok {
			d.Betweenness[id] = 0.0
		}
	}

	return d
}

// density — плотность directed-графа E/(V*(V-1)); 0 при V<2 (как nx.density).
func density(v, e int) float64 {
	if v < 2 {
		return 0.0
	}

	return float64(e) / (float64(v) * float64(v-1))
}

// betweennessNorm — нормировочный множитель nx normalized=True для directed:
// 1/((n-1)(n-2)); 1.0 при n<3 (нормировка не применяется).
func betweennessNorm(n int) float64 {
	if n < 3 {
		return 1.0
	}

	return 1.0 / (float64(n-1) * float64(n-2))
}
