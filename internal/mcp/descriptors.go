package mcp

import (
	"math"
	"sort"
	"strings"

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

	// --- БАТЧ 3: связностно-дистанционная группа ---
	Closeness     map[string]float64 `json:"closeness"`     // nx.closeness_centrality (входящие, wf_improved)
	Harmonic      map[string]float64 `json:"harmonic"`      // nx.harmonic_centrality (входящие, Σ1/d)
	Eigenvector   map[string]float64 `json:"eigenvector"`   // nx.eigenvector_centrality (power, I+Aᵀ, L2)
	Diameter      int                `json:"diameter"`      // диаметр undirected-проекции; -1 если несвязно
	AvgPathLength float64            `json:"avgPathLength"` // средняя длина пути (undirected, наибольшая компонента)

	// --- БАТЧ 4: распределение/качество ---
	Abstractness         float64            `json:"abstractness"`         // доля «абстрактных» узлов (по имени)
	AbstractCount        int                `json:"abstractCount"`        //
	ConcreteCount        int                `json:"concreteCount"`        //
	DistanceMainSequence map[string]float64 `json:"distanceMainSequence"` // D=|A+I-1| по пакетам
	MaxKCore             int                `json:"maxKCore"`             // макс. k-ядро (undirected)
	MeanDegree           float64            `json:"meanDegree"`           // средняя total-степень
	StdDegree            float64            `json:"stdDegree"`            // СКО total-степени (population)
	MaxTotalDegree       int                `json:"maxTotalDegree"`       // макс. total-степень (hub-магнитуда)

	// --- БАТЧ 5: reachability/ripple ---
	ChangePropagation    map[string]int     `json:"changePropagation"`    // транзитивные зависимые (descendants(reverse))
	AvgImpact            float64            `json:"avgImpact"`            // средний impact
	MaxImpact            int                `json:"maxImpact"`            // макс. impact
	BlastRadius          map[string]float64 `json:"blastRadius"`          // (pagerank + norm_fan_in)/2
	MaxComponentDistance int                `json:"maxComponentDistance"` // макс. directed расстояние
}

// abstractPatterns — имя-эвристика «абстрактного» узла (1:1 с Python validator).
var abstractPatterns = []string{"interface", "abstract", "base", "contract", "port"}

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

	// --- БАТЧ 3 ---
	d.Closeness, d.Harmonic = closenessHarmonic(dg)
	d.Eigenvector = eigenvectorCentrality(dg)
	d.Diameter, d.AvgPathLength = diameterAvgPath(dg)

	// --- БАТЧ 4 ---
	d.Abstractness, d.AbstractCount, d.ConcreteCount = abstractness(dg)
	d.DistanceMainSequence = distanceMainSequence(dg)
	d.MaxKCore = maxKCore(dg)
	d.MeanDegree, d.StdDegree = degreeStats(dg)
	d.MaxTotalDegree = maxTotalDegree(dg)

	// --- БАТЧ 5 ---
	d.ChangePropagation, d.AvgImpact, d.MaxImpact = changePropagation(dg)
	d.BlastRadius = blastRadius(dg, d.ChangePropagation, d.PageRank)
	d.MaxComponentDistance = maxComponentDistance(dg)

	return d
}

// changePropagation — для каждого узла число ТРАНЗИТИВНЫХ зависимых
// (descendants(reverse_graph, node) = все, кто может достичь node) + avg/max.
func changePropagation(dg *descriptorGraph) (impact map[string]int, avg float64, maxImpact int) {
	in := inAdjacency(dg)
	impact = make(map[string]int, len(dg.nodes))

	sum := 0

	for _, v := range dg.nodes {
		imp := len(bfsIncoming(in, v)) - 1 // исключаем сам узел
		impact[v] = imp
		sum += imp

		if imp > maxImpact {
			maxImpact = imp
		}
	}

	if len(dg.nodes) > 0 {
		avg = float64(sum) / float64(len(dg.nodes))
	}

	return impact, avg, maxImpact
}

// blastRadius — (pagerank + нормированный fan-in)/2 на узел (1:1 с Python).
func blastRadius(dg *descriptorGraph, impact map[string]int, pagerank map[string]float64) map[string]float64 {
	n := len(dg.nodes)
	out := make(map[string]float64, n)

	for _, v := range dg.nodes {
		normFanIn := 0.0
		if n > 0 {
			normFanIn = float64(impact[v]) / float64(n)
		}

		out[v] = (pagerank[v] + normFanIn) / 2
	}

	return out
}

// maxComponentDistance — максимальное directed кратчайшее расстояние (по всем парам).
func maxComponentDistance(dg *descriptorGraph) int {
	maxd := 0

	for _, s := range dg.nodes {
		for t, dd := range bfsOutgoing(dg.outAdj, s) {
			if t != s && dd > maxd {
				maxd = dd
			}
		}
	}

	return maxd
}

// bfsOutgoing — directed расстояния от s по исходящим рёбрам.
func bfsOutgoing(out map[string]map[string]bool, s string) map[string]int {
	dist := map[string]int{s: 0}
	queue := []string{s}

	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]

		for v := range out[u] {
			if _, seen := dist[v]; !seen {
				dist[v] = dist[u] + 1
				queue = append(queue, v)
			}
		}
	}

	return dist
}

// isAbstractName — имя-эвристика абстрактности (lowercase contains pattern).
func isAbstractName(id string) bool {
	low := strings.ToLower(id)
	for _, p := range abstractPatterns {
		if strings.Contains(low, p) {
			return true
		}
	}

	return false
}

// abstractness — доля «абстрактных» узлов по имени + счётчики.
func abstractness(dg *descriptorGraph) (ratio float64, abstractCount, concreteCount int) {
	for _, id := range dg.nodes {
		if isAbstractName(id) {
			abstractCount++
		}
	}

	total := len(dg.nodes)
	concreteCount = total - abstractCount

	if total > 0 {
		ratio = float64(abstractCount) / float64(total)
	}

	return ratio, abstractCount, concreteCount
}

// distanceMainSequence — D=|A+I-1| по ПАКЕТАМ (pkg = первая часть id по '/' или '.').
// A — доля абстрактных в пакете, I = Σce/(Σca+Σce) (0.5 если степени 0). 1:1 с Python.
func distanceMainSequence(dg *descriptorGraph) map[string]float64 {
	pkgNodes := map[string][]string{}

	for _, id := range dg.nodes {
		pkg := packageOf(id)
		pkgNodes[pkg] = append(pkgNodes[pkg], id)
	}

	out := make(map[string]float64, len(pkgNodes))

	for pkg, nodes := range pkgNodes {
		abs := 0
		totalCa, totalCe := 0, 0

		for _, id := range nodes {
			if isAbstractName(id) {
				abs++
			}

			totalCa += dg.inDeg[id]
			totalCe += dg.outDeg[id]
		}

		a := float64(abs) / float64(len(nodes))

		var inst float64
		if totalCa+totalCe > 0 {
			inst = float64(totalCe) / float64(totalCa+totalCe)
		} else {
			inst = 0.5
		}

		out[pkg] = math.Abs(a + inst - 1)
	}

	return out
}

// packageOf — первая часть имени узла (id.replace('.', '/').split('/')[0]), как Python.
func packageOf(id string) string {
	norm := strings.ReplaceAll(id, ".", "/")

	if i := strings.IndexByte(norm, '/'); i >= 0 {
		return norm[:i]
	}

	return norm
}

// maxKCore — максимальное k-ядро undirected-проекции (Batagelj-Zaversnik core_number).
func maxKCore(dg *descriptorGraph) int {
	adj := undirectedAdj(dg)
	core := make(map[string]int, len(dg.nodes))

	for _, id := range dg.nodes {
		core[id] = len(adj[id])
	}

	processed := make(map[string]bool, len(dg.nodes))

	for len(processed) < len(dg.nodes) {
		// узел с минимальным текущим core среди необработанных
		var v string

		first := true

		for _, id := range dg.nodes {
			if processed[id] {
				continue
			}

			if first || core[id] < core[v] {
				v = id
				first = false
			}
		}

		processed[v] = true

		for u := range adj[v] {
			if !processed[u] && core[u] > core[v] {
				core[u]--
			}
		}
	}

	maxK := 0
	for _, k := range core {
		if k > maxK {
			maxK = k
		}
	}

	return maxK
}

// degreeStats — среднее и популяционное СКО total-степени (как np.mean/np.std).
func degreeStats(dg *descriptorGraph) (mean, std float64) {
	n := len(dg.nodes)
	if n == 0 {
		return 0, 0
	}

	degs := make([]int, 0, n)
	sum := 0

	for _, id := range dg.nodes {
		d := dg.inDeg[id] + dg.outDeg[id]
		degs = append(degs, d)
		sum += d
	}

	mean = float64(sum) / float64(n)

	var sq float64
	for _, d := range degs {
		diff := float64(d) - mean
		sq += diff * diff
	}

	std = math.Sqrt(sq / float64(n))

	return mean, std
}

// maxTotalDegree — макс. total-степень (in+out).
func maxTotalDegree(dg *descriptorGraph) int {
	maxv := 0
	for _, id := range dg.nodes {
		if d := dg.inDeg[id] + dg.outDeg[id]; d > maxv {
			maxv = d
		}
	}

	return maxv
}

// inAdjacency — предшественники (для входящих расстояний и eigenvector).
func inAdjacency(dg *descriptorGraph) map[string]map[string]bool {
	in := make(map[string]map[string]bool, len(dg.nodes))
	for _, id := range dg.nodes {
		in[id] = make(map[string]bool)
	}

	for from, tos := range dg.outAdj {
		for to := range tos {
			in[to][from] = true
		}
	}

	return in
}

// bfsIncoming — расстояния ДО target (BFS по предшественникам); как distance(u,target)
// в исходном directed-графе. Включает сам target (0).
func bfsIncoming(in map[string]map[string]bool, target string) map[string]int {
	dist := map[string]int{target: 0}
	queue := []string{target}

	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]

		for p := range in[u] {
			if _, seen := dist[p]; !seen {
				dist[p] = dist[u] + 1
				queue = append(queue, p)
			}
		}
	}

	return dist
}

// closenessHarmonic — nx.closeness_centrality (wf_improved) и nx.harmonic_centrality
// (обе по ВХОДЯЩИМ расстояниям для directed).
func closenessHarmonic(dg *descriptorGraph) (closeness, harmonic map[string]float64) {
	in := inAdjacency(dg)
	n := len(dg.nodes)
	closeness = make(map[string]float64, n)
	harmonic = make(map[string]float64, n)

	for _, v := range dg.nodes {
		dist := bfsIncoming(in, v)

		totsp := 0
		var harm float64

		for u, dd := range dist {
			if u == v {
				continue
			}

			totsp += dd
			harm += 1.0 / float64(dd)
		}

		harmonic[v] = harm

		reachers := len(dist) // включая v
		if totsp > 0 && n > 1 {
			cl := float64(reachers-1) / float64(totsp)
			cl *= float64(reachers-1) / float64(n-1) // wf_improved
			closeness[v] = cl
		} else {
			closeness[v] = 0.0
		}
	}

	return closeness, harmonic
}

// eigenvectorCentrality — power iteration как nx.eigenvector_centrality:
// x_v <- x_v + Σ_{p->v} x_p  (т.е. (I+Aᵀ)x), L2-нормировка, tol=1e-6, max_iter=1000.
func eigenvectorCentrality(dg *descriptorGraph) map[string]float64 {
	n := len(dg.nodes)
	x := make(map[string]float64, n)

	if n == 0 {
		return x
	}

	in := inAdjacency(dg)
	for _, id := range dg.nodes {
		x[id] = 1.0 / float64(n)
	}

	const tol = 1e-6

	const maxIter = 1000

	for iter := 0; iter < maxIter; iter++ {
		xlast := make(map[string]float64, n)
		for k, v := range x {
			xlast[k] = v
		}

		// x = xlast.copy(); x[v] += Σ predecessors
		for _, v := range dg.nodes {
			sum := xlast[v]
			for p := range in[v] {
				sum += xlast[p]
			}

			x[v] = sum
		}

		norm := 0.0
		for _, v := range x {
			norm += v * v
		}

		norm = math.Sqrt(norm)
		if norm == 0 {
			norm = 1
		}

		for k := range x {
			x[k] /= norm
		}

		diff := 0.0
		for _, id := range dg.nodes {
			diff += math.Abs(x[id] - xlast[id])
		}

		if diff < float64(n)*tol {
			break
		}
	}

	return x
}

// diameterAvgPath — диаметр и средняя длина пути на UNDIRECTED-проекции.
// Диаметр: -1 если граф несвязен (как nx -> infinity). avg_path_length считается на
// наибольшей связной компоненте (как Python validate_avg_path_length).
func diameterAvgPath(dg *descriptorGraph) (diameter int, avgPath float64) {
	n := len(dg.nodes)
	if n < 2 {
		return 0, 0
	}

	adj := undirectedAdj(dg)

	ecc := make(map[string]int, n)
	connected := true
	diameter = 0

	var pathSum, pathPairs float64

	for _, s := range dg.nodes {
		dist := bfsUndirected(adj, s)
		if len(dist) < n {
			connected = false
		}

		maxd := 0
		for u, dd := range dist {
			if u == s {
				continue
			}

			pathSum += float64(dd)
			pathPairs++

			if dd > maxd {
				maxd = dd
			}
		}

		ecc[s] = maxd
		if maxd > diameter {
			diameter = maxd
		}
	}

	if !connected {
		diameter = -1 // несвязный граф: диаметр не определён (nx -> infinity)
	}

	if pathPairs > 0 {
		avgPath = pathSum / pathPairs
	}

	return diameter, avgPath
}

// bfsUndirected — расстояния от s по неориентированной смежности.
func bfsUndirected(adj map[string]map[string]bool, s string) map[string]int {
	dist := map[string]int{s: 0}
	queue := []string{s}

	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]

		for nb := range adj[u] {
			if _, seen := dist[nb]; !seen {
				dist[nb] = dist[u] + 1
				queue = append(queue, nb)
			}
		}
	}

	return dist
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
