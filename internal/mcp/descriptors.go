package mcp

import (
	"math"
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

	// --- БАТЧ 2: глобальные скаляры ---
	MaxPossibleEdges  int     `json:"maxPossibleEdges"`  // V*(V-1)
	MaxFanOut         int     `json:"maxFanOut"`         // max out-degree
	IsDAG             bool    `json:"isDAG"`             // ацикличность
	GraphDepth        int     `json:"graphDepth"`        // длина (рёбер) longest-path DAG; -1 если есть циклы
	DependencyEntropy float64 `json:"dependencyEntropy"` // H = -Σ p log2 p по out-degree
	MaxEntropy        float64 `json:"maxEntropy"`        // log2(V)
	NormalizedEntropy float64 `json:"normalizedEntropy"` // H / log2(V)
	Gini              float64 `json:"gini"`              // Джини по total-degree
	AvgClustering     float64 `json:"avgClustering"`     // средний clustering (undirected-проекция)
	Betti1            int     `json:"betti1"`            // β₁ = E - V + C (C = слабые компоненты)
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

	// --- БАТЧ 2 ---
	n := len(dg.nodes)
	d.MaxPossibleEdges = n * (n - 1)
	d.MaxFanOut = maxFanOut(dg)
	d.IsDAG, d.GraphDepth = dagDepth(dg)
	d.DependencyEntropy, d.MaxEntropy, d.NormalizedEntropy = dependencyEntropy(dg)
	d.Gini = giniCoefficient(dg)
	d.AvgClustering = avgClustering(dg)
	d.Betti1 = dg.edgeCount - n + weaklyConnectedComponents(dg)

	return d
}

// maxFanOut — максимальная out-степень (0 при пустом графе).
func maxFanOut(dg *descriptorGraph) int {
	maxv := 0
	for _, id := range dg.nodes {
		if dg.outDeg[id] > maxv {
			maxv = dg.outDeg[id]
		}
	}

	return maxv
}

// dagDepth: Kahn topo-sort. Если обработаны не все узлы -> цикл (isDAG=false, depth=-1).
// Иначе depth = длина (в рёбрах) самого длинного пути (nx.dag_longest_path - 1).
func dagDepth(dg *descriptorGraph) (isDAG bool, depth int) {
	indeg := make(map[string]int, len(dg.nodes))
	for _, id := range dg.nodes {
		indeg[id] = dg.inDeg[id]
	}

	queue := make([]string, 0, len(dg.nodes))
	dist := make(map[string]int, len(dg.nodes))

	for _, id := range dg.nodes {
		if indeg[id] == 0 {
			queue = append(queue, id)
		}
	}

	processed := 0

	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]
		processed++

		for v := range dg.outAdj[u] {
			if v == u {
				continue // петля учтена в indeg -> до 0 не дойдёт (цикл)
			}

			if dist[u]+1 > dist[v] {
				dist[v] = dist[u] + 1
			}

			indeg[v]--
			if indeg[v] == 0 {
				queue = append(queue, v)
			}
		}
	}

	if processed < len(dg.nodes) {
		return false, -1 // остался узел в цикле
	}

	maxDepth := 0
	for _, dval := range dist {
		if dval > maxDepth {
			maxDepth = dval
		}
	}

	return true, maxDepth
}

// dependencyEntropy: H = -Σ p log2 p по out-степеням (p только для d>0), max=log2(V),
// normalized=H/max. Пустые зависимости -> (0,max,0). 1:1 с Python.
func dependencyEntropy(dg *descriptorGraph) (entropy, maxEntropy, normalized float64) {
	n := len(dg.nodes)

	total := 0
	for _, id := range dg.nodes {
		total += dg.outDeg[id]
	}

	if n > 1 {
		maxEntropy = math.Log2(float64(n))
	} else {
		maxEntropy = 1
	}

	if total == 0 {
		return 0, maxEntropy, 0
	}

	for _, id := range dg.nodes {
		dgr := dg.outDeg[id]
		if dgr <= 0 {
			continue
		}

		p := float64(dgr) / float64(total)
		entropy -= p * math.Log2(p)
	}

	if maxEntropy > 0 {
		normalized = entropy / maxEntropy
	}

	return entropy, maxEntropy, normalized
}

// giniCoefficient по total-degree (in+out): degrees sorted -> cumsum ->
// 1 - 2*Σcumsum/(n*total) + 1/n. 1:1 с Python (np.cumsum).
func giniCoefficient(dg *descriptorGraph) float64 {
	n := len(dg.nodes)
	if n == 0 {
		return 0
	}

	degrees := make([]int, 0, n)
	for _, id := range dg.nodes {
		degrees = append(degrees, dg.inDeg[id]+dg.outDeg[id])
	}

	sort.Ints(degrees)

	cum := 0
	sumCum := 0

	for _, dgr := range degrees {
		cum += dgr
		sumCum += cum
	}

	total := cum
	if total == 0 {
		return 0
	}

	return 1 - 2*float64(sumCum)/(float64(n)*float64(total)) + 1/float64(n)
}

// avgClustering — средний коэффициент кластеризации по UNDIRECTED-проекции
// (как nx.clustering(graph.to_undirected())). C(v)=2L/(k(k-1)), k<2 -> 0.
func avgClustering(dg *descriptorGraph) float64 {
	if len(dg.nodes) == 0 {
		return 0
	}

	adj := undirectedAdj(dg)

	var sum float64

	for _, id := range dg.nodes {
		neigh := adj[id]
		k := len(neigh)

		if k < 2 {
			continue // C=0
		}

		links := 0
		for a := range neigh {
			for b := range neigh {
				if a < b && adj[a][b] {
					links++
				}
			}
		}

		sum += 2 * float64(links) / (float64(k) * float64(k-1))
	}

	return sum / float64(len(dg.nodes))
}

// undirectedAdj строит неориентированную смежность (без петель) из дедуп-графа.
func undirectedAdj(dg *descriptorGraph) map[string]map[string]bool {
	adj := make(map[string]map[string]bool, len(dg.nodes))
	for _, id := range dg.nodes {
		adj[id] = make(map[string]bool)
	}

	for from, tos := range dg.outAdj {
		for to := range tos {
			if from == to {
				continue
			}

			adj[from][to] = true
			adj[to][from] = true
		}
	}

	return adj
}

// weaklyConnectedComponents — число слабых компонент (union-find по undirected-рёбрам).
func weaklyConnectedComponents(dg *descriptorGraph) int {
	parent := make(map[string]string, len(dg.nodes))
	for _, id := range dg.nodes {
		parent[id] = id
	}

	var find func(string) string
	find = func(x string) string {
		if parent[x] != x {
			parent[x] = find(parent[x])
		}

		return parent[x]
	}

	union := func(a, b string) {
		ra, rb := find(a), find(b)
		if ra != rb {
			parent[ra] = rb
		}
	}

	for from, tos := range dg.outAdj {
		for to := range tos {
			union(from, to)
		}
	}

	roots := make(map[string]bool, len(dg.nodes))
	for _, id := range dg.nodes {
		roots[find(id)] = true
	}

	return len(roots)
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
