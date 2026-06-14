package mcp

import (
	"sort"
	"strconv"

	"github.com/mshogin/archlint/internal/model"
	"gonum.org/v1/gonum/graph/topo"
)

// Исследовательские (research) дескрипторы — порт «медленных» структурных метрик
// Python validator (NetworkX) -> Go. В отличие от батч-1..6 (быстрые магнитуды),
// эта группа — порядково-теоретические и дистанционные инварианты (частичный
// порядок, цепи/антицепи, транзитивное замыкание, геодезические расстояния).
//
// СЕВЕРИТИ: ВСЕ — СИГНАЛЫ под --signals (research/slow-режим), НЕ ERROR, НЕ часть
// быстрого гейта. Алгоритмы O(n^2..n^3); как в Python, граф > 200 узлов пропускается
// (nil). Эквивалентность гарантируется golden против реальных Python-валидаторов.
//
// ПОРТ batch-1 (fast-группа): transitive_closure, partial_order_analysis,
// chain_antichain, geodesic_distance. betti_numbers (β₁=E-V+C) уже считается в
// батч-2 как Descriptors.Betti1.

const researchNodeLimit = 200 // как Python: > limit -> SKIP (алгоритмы O(n^3)/NP)

// ResearchDescriptors — снимок медленных структурных дескрипторов. Любое поле nil,
// если соответствующая метрика пропущена (мало узлов / граф больше лимита).
type ResearchDescriptors struct {
	// batch-1 (fast-группа)
	TransitiveClosure *TransitiveClosureSignal `json:"transitiveClosure,omitempty"`
	PartialOrder      *PartialOrderSignal      `json:"partialOrder,omitempty"`
	ChainAntichain    *ChainAntichainSignal    `json:"chainAntichain,omitempty"`
	Geodesic          *GeodesicSignal          `json:"geodesic,omitempty"`

	// batch-2 (медленные граф-NP инварианты + спектральная топология)
	Treewidth          *TreewidthSignal          `json:"treewidth,omitempty"`
	Chromatic          *ChromaticSignal          `json:"chromatic,omitempty"`
	DominatingSet      *DominatingSetSignal      `json:"dominatingSet,omitempty"`
	IndependenceNumber *IndependenceSignal       `json:"independenceNumber,omitempty"`
	VertexCover        *VertexCoverSignal        `json:"vertexCover,omitempty"`
	GraphCliques       *GraphCliquesSignal       `json:"graphCliques,omitempty"`
	PersistentHomology *PersistentHomologySignal `json:"persistentHomology,omitempty"`
	HodgeDecomposition *HodgeSignal              `json:"hodgeDecomposition,omitempty"`
	ShapleyValue       *ShapleySignal            `json:"shapleyValue,omitempty"`

	// batch-3 (тонкие: теория множеств/порядка/информации + спектральная близость)
	EquivalenceClasses   *EquivalenceClassesSignal   `json:"equivalenceClasses,omitempty"`
	Lattice              *LatticeSignal              `json:"lattice,omitempty"`
	JoinMeet             *JoinMeetSignal             `json:"joinMeet,omitempty"`
	PartitionRefinement  *PartitionRefinementSignal  `json:"partitionRefinement,omitempty"`
	MutualInformation    *MutualInformationSignal    `json:"mutualInformation,omitempty"`
	ResistanceCloseness  *ResistanceClosenessSignal  `json:"resistanceCloseness,omitempty"`
	CommuteTime          *CommuteTimeSignal          `json:"commuteTime,omitempty"`
}

// TransitiveClosureSignal — порт validate_transitive_closure. R⁺ = R ∪ R² ∪ ...;
// closure_ratio = |R⁺|/|R| показывает долю неявных транзитивных зависимостей.
type TransitiveClosureSignal struct {
	OriginalEdges        int     `json:"originalEdges"`        // |R| (дедуплицированные рёбра)
	ClosureEdges         int     `json:"closureEdges"`         // |R⁺|
	ImplicitDependencies int     `json:"implicitDependencies"` // |R⁺| - |R|
	ClosureRatio         float64 `json:"closureRatio"`         // |R⁺|/|R|
	ClosureDensity       float64 `json:"closureDensity"`       // |R⁺|/(V*(V-1))
	ReductionEdges       int     `json:"reductionEdges"`       // |рёбер транзитивной редукции| (DAG; иначе = |R|)
	RedundantEdges       int     `json:"redundantEdges"`       // |R| - reduction (DAG; иначе 0)
	AvgReachability      float64 `json:"avgReachability"`      // средняя доля достижимых узлов
}

// PartialOrderSignal — порт validate_partial_order_analysis. Для DAG poset = само
// транзитивное замыкание; для графа с циклами работаем с конденсацией (фактор по SCC).
type PartialOrderSignal struct {
	IsDAG              bool    `json:"isDAG"`
	MinimalElements    int     `json:"minimalElements"`    // узлы без входящих (work-graph)
	MaximalElements    int     `json:"maximalElements"`    // узлы без исходящих
	Height             int     `json:"height"`             // длина макс. цепи (узлов) = глубина иерархии
	Width              int     `json:"width"`              // макс. топологический уровень = параллелизм
	ComparabilityRatio float64 `json:"comparabilityRatio"` // доля сравнимых пар
}

// ChainAntichainSignal — порт validate_chain_antichain (Дилворт/Мирский). Цепь =
// линейно упорядоченное подмножество; антицепь = попарно несравнимые (независимые
// модули для параллельной разработки).
type ChainAntichainSignal struct {
	MaxChainLength          int `json:"maxChainLength"`          // длиннейшая цепь (узлов)
	MaxAntichainSize        int `json:"maxAntichainSize"`        // крупнейшая антицепь (эвристика)
	DilworthMinChainCover   int `json:"dilworthMinChainCover"`   // = MaxAntichainSize
	MirskyMinAntichainCover int `json:"mirskyMinAntichainCover"` // = MaxChainLength
}

// GeodesicSignal — порт validate_geodesic_distance на НЕориентированной проекции
// (наибольшая связная компонента). Малый диаметр = компактная архитектура.
type GeodesicSignal struct {
	Diameter    int      `json:"diameter"`    // макс. эксцентриситет
	Radius      int      `json:"radius"`      // мин. эксцентриситет
	AvgDistance float64  `json:"avgDistance"` // средняя геодезическая (пары u<v)
	WienerIndex int      `json:"wienerIndex"` // сумма всех геодезических (пары u<v)
	Center      []string `json:"center"`      // узлы с эксцентриситетом = радиусу (отсортированы)
	Periphery   []string `json:"periphery"`   // узлы с эксцентриситетом = диаметру (отсортированы)
}

// ComputeResearchDescriptors считает медленные структурные дескрипторы графа.
func ComputeResearchDescriptors(g *model.Graph) ResearchDescriptors {
	dg := buildDescriptorGraph(g)

	var rd ResearchDescriptors

	rd.TransitiveClosure = computeTransitiveClosure(dg)
	rd.PartialOrder = computePartialOrder(dg)
	rd.ChainAntichain = computeChainAntichain(dg)
	rd.Geodesic = computeGeodesic(dg)

	// batch-2: медленные граф-NP инварианты.
	rd.Treewidth = computeTreewidth(dg)
	rd.Chromatic = computeChromatic(dg)
	rd.DominatingSet = computeDominatingSet(dg)
	rd.IndependenceNumber = computeIndependenceNumber(dg)
	rd.VertexCover = computeVertexCover(dg)
	rd.GraphCliques = computeGraphCliques(dg)
	rd.PersistentHomology = computePersistentHomology(dg)
	rd.HodgeDecomposition = computeHodge(g, dg)
	rd.ShapleyValue = computeShapley(dg)

	// batch-3: теория множеств/порядка/информации + спектральная близость.
	rd.EquivalenceClasses = computeEquivalenceClasses(dg)
	rd.Lattice = computeLattice(dg)
	rd.JoinMeet = computeJoinMeet(dg)
	rd.PartitionRefinement = computePartitionRefinement(dg)
	rd.MutualInformation = computeMutualInformation(dg)
	rd.ResistanceCloseness = computeResistanceCloseness(dg)
	rd.CommuteTime = computeCommuteTime(dg)

	return rd
}

// --- транзитивное замыкание -------------------------------------------------

func computeTransitiveClosure(dg *descriptorGraph) *TransitiveClosureSignal {
	n := len(dg.nodes)
	if n < 2 || n > researchNodeLimit {
		return nil
	}

	e := dg.edgeCount

	// |R⁺|: для каждого u число узлов, достижимых путём длины >= 1 (включая сам u,
	// если он лежит на цикле — как nx.transitive_closure).
	closureEdges := 0
	// avg_reachability считается по descendants (без self), как в Python.
	var reachSum float64
	sample := n
	if sample > 20 {
		sample = 20
	}

	for i, u := range dg.nodes {
		row := reachableFromSuccessors(dg.outAdj, u)
		closureEdges += len(row)

		if i < sample {
			desc := len(row)
			if row[u] {
				desc-- // descendants не считает сам узел
			}

			if n > 1 {
				reachSum += float64(desc) / float64(n-1)
			}
		}
	}

	ratio := 0.0
	if e > 0 {
		ratio = float64(closureEdges) / float64(e)
	}

	maxPossible := n * (n - 1)

	density := 0.0
	if maxPossible > 0 {
		density = float64(closureEdges) / float64(maxPossible)
	}

	reductionEdges, redundant := transitiveReduction(dg)

	avgReach := 0.0
	if sample > 0 {
		avgReach = reachSum / float64(sample)
	}

	return &TransitiveClosureSignal{
		OriginalEdges:        e,
		ClosureEdges:         closureEdges,
		ImplicitDependencies: closureEdges - e,
		ClosureRatio:         roundTo(ratio, 1e4),
		ClosureDensity:       roundTo(density, 1e4),
		ReductionEdges:       reductionEdges,
		RedundantEdges:       redundant,
		AvgReachability:      roundTo(avgReach, 1e4),
	}
}

// reachableFromSuccessors — множество узлов, достижимых из u путём длины >= 1
// (BFS стартует с прямых преемников, поэтому u входит ТОЛЬКО если есть путь назад
// по циклу). Включение self-через-цикл совпадает с nx.transitive_closure.
func reachableFromSuccessors(out map[string]map[string]bool, u string) map[string]bool {
	visited := make(map[string]bool)
	queue := make([]string, 0, len(out[u]))

	for v := range out[u] {
		if !visited[v] {
			visited[v] = true
			queue = append(queue, v)
		}
	}

	for len(queue) > 0 {
		x := queue[0]
		queue = queue[1:]

		for v := range out[x] {
			if !visited[v] {
				visited[v] = true
				queue = append(queue, v)
			}
		}
	}

	return visited
}

// transitiveReduction — рёбра транзитивной редукции. Определена только для DAG
// (как nx.transitive_reduction); на графе с циклами Python падает в except и
// возвращает исходное число рёбер с 0 избыточных — повторяем это поведение.
func transitiveReduction(dg *descriptorGraph) (reductionEdges, redundant int) {
	isDAG, _ := dagDepth(dg)
	if !isDAG {
		return dg.edgeCount, 0
	}

	// descendants каждого узла (DAG -> без self).
	desc := make(map[string]map[string]bool, len(dg.nodes))
	for _, u := range dg.nodes {
		desc[u] = reachableFromSuccessors(dg.outAdj, u)
	}

	kept := 0

	for _, u := range dg.nodes {
		succ := make([]string, 0, len(dg.outAdj[u]))
		for v := range dg.outAdj[u] {
			succ = append(succ, v)
		}

		for _, v := range succ {
			// (u,v) избыточно, если v достижим из другого преемника w.
			isRedundant := false

			for _, w := range succ {
				if w == v {
					continue
				}

				if desc[w][v] {
					isRedundant = true
					break
				}
			}

			if !isRedundant {
				kept++
			}
		}
	}

	return kept, dg.edgeCount - kept
}

// --- частичный порядок / цепи-антицепи --------------------------------------

// workDAG — рабочий DAG для порядково-теоретических метрик: либо сам граф (если он
// ацикличен), либо его конденсация (фактор по SCC) — оба гарантированно ацикличны.
type workDAG struct {
	nodes []string
	out   map[string]map[string]bool
	inDeg map[string]int
}

func newWorkDAG() *workDAG {
	return &workDAG{out: map[string]map[string]bool{}, inDeg: map[string]int{}}
}

func (w *workDAG) addNode(id string) {
	if _, ok := w.out[id]; !ok {
		w.out[id] = map[string]bool{}
		w.inDeg[id] = 0
		w.nodes = append(w.nodes, id)
	}
}

func (w *workDAG) addEdge(from, to string) {
	if w.out[from][to] {
		return
	}

	w.out[from][to] = true
	w.inDeg[to]++
}

// buildWorkDAG — рабочий DAG: исходный граф при ацикличности, иначе конденсация.
func buildWorkDAG(dg *descriptorGraph) *workDAG {
	isDAG, _ := dagDepth(dg)

	w := newWorkDAG()

	if isDAG {
		for _, id := range dg.nodes {
			w.addNode(id)
		}

		for from, tos := range dg.outAdj {
			for to := range tos {
				if from != to {
					w.addEdge(from, to)
				}
			}
		}

		sort.Strings(w.nodes)

		return w
	}

	// Конденсация: каждая SCC -> один узел SCC_k, рёбра между разными SCC.
	gg := dg.gonumGraph()
	comp := make(map[string]int, len(dg.nodes))

	for k, scc := range topo.TarjanSCC(gg) {
		for _, nd := range scc {
			comp[dg.nodes[int(nd.ID())]] = k
		}
	}

	sccName := func(k int) string { return "SCC_" + strconv.Itoa(k) }

	for _, id := range dg.nodes {
		w.addNode(sccName(comp[id]))
	}

	for from, tos := range dg.outAdj {
		for to := range tos {
			if comp[from] != comp[to] {
				w.addEdge(sccName(comp[from]), sccName(comp[to]))
			}
		}
	}

	sort.Strings(w.nodes)

	return w
}

// descendantsAll — descendants каждого узла DAG (для сравнимости/антицепи).
func (w *workDAG) descendantsAll() map[string]map[string]bool {
	desc := make(map[string]map[string]bool, len(w.nodes))
	for _, u := range w.nodes {
		desc[u] = reachableFromSuccessors(w.out, u)
	}

	return desc
}

// longestPathNodes — длина длиннейшего пути DAG в УЗЛАХ (= рёбра+1; 0 при пустом).
func (w *workDAG) longestPathNodes() int {
	indeg := make(map[string]int, len(w.nodes))
	for _, id := range w.nodes {
		indeg[id] = w.inDeg[id]
	}

	queue := make([]string, 0)
	dist := make(map[string]int, len(w.nodes))

	for _, id := range w.nodes {
		if indeg[id] == 0 {
			queue = append(queue, id)
		}
	}

	maxEdges := 0

	for len(queue) > 0 {
		u := queue[0]
		queue = queue[1:]

		for v := range w.out[u] {
			if dist[u]+1 > dist[v] {
				dist[v] = dist[u] + 1
				if dist[v] > maxEdges {
					maxEdges = dist[v]
				}
			}

			indeg[v]--
			if indeg[v] == 0 {
				queue = append(queue, v)
			}
		}
	}

	if len(w.nodes) == 0 {
		return 0
	}

	return maxEdges + 1 // узлов = рёбер + 1
}

// topologicalGenerations — топологические уровни (Kahn): уровень k = узлы, у которых
// все предшественники в более ранних уровнях. Возвращает размеры уровней.
func (w *workDAG) topologicalGenerations() []int {
	indeg := make(map[string]int, len(w.nodes))
	for _, id := range w.nodes {
		indeg[id] = w.inDeg[id]
	}

	current := make([]string, 0)
	for _, id := range w.nodes {
		if indeg[id] == 0 {
			current = append(current, id)
		}
	}

	sort.Strings(current)

	var sizes []int

	for len(current) > 0 {
		sizes = append(sizes, len(current))

		var next []string

		for _, u := range current {
			for v := range w.out[u] {
				indeg[v]--
				if indeg[v] == 0 {
					next = append(next, v)
				}
			}
		}

		sort.Strings(next)
		current = next
	}

	return sizes
}

func computePartialOrder(dg *descriptorGraph) *PartialOrderSignal {
	n := len(dg.nodes)
	if n < 2 || n > researchNodeLimit {
		return nil
	}

	isDAG, _ := dagDepth(dg)
	w := buildWorkDAG(dg)

	minimal, maximal := 0, 0
	for _, id := range w.nodes {
		if w.inDeg[id] == 0 {
			minimal++
		}

		if len(w.out[id]) == 0 {
			maximal++
		}
	}

	height := w.longestPathNodes()

	width := 0
	for _, s := range w.topologicalGenerations() {
		if s > width {
			width = s
		}
	}

	comparable, total := comparablePairs(w)

	ratio := 0.0
	if total > 0 {
		ratio = float64(comparable) / float64(total)
	}

	return &PartialOrderSignal{
		IsDAG:              isDAG,
		MinimalElements:    minimal,
		MaximalElements:    maximal,
		Height:             height,
		Width:              width,
		ComparabilityRatio: roundTo(ratio, 1e4),
	}
}

// comparablePairs — число сравнимых пар (x<=y или y<=x) и общее число пар work-DAG.
func comparablePairs(w *workDAG) (comparable, total int) {
	desc := w.descendantsAll()

	for i := 0; i < len(w.nodes); i++ {
		for j := i + 1; j < len(w.nodes); j++ {
			u, v := w.nodes[i], w.nodes[j]
			total++

			if desc[u][v] || desc[v][u] {
				comparable++
			}
		}
	}

	return comparable, total
}

func computeChainAntichain(dg *descriptorGraph) *ChainAntichainSignal {
	n := len(dg.nodes)
	if n < 2 || n > researchNodeLimit {
		return nil
	}

	w := buildWorkDAG(dg)

	maxChain := w.longestPathNodes()

	width := 0
	for _, s := range w.topologicalGenerations() {
		if s > width {
			width = s
		}
	}

	// Эвристика антицепи: max(топологический уровень, жадная антицепь) — как Python.
	greedy := greedyAntichainSize(w)

	maxAntichain := width
	if greedy > maxAntichain {
		maxAntichain = greedy
	}

	return &ChainAntichainSignal{
		MaxChainLength:          maxChain,
		MaxAntichainSize:        maxAntichain,
		DilworthMinChainCover:   maxAntichain, // Дилворт: min покрытие цепями = max антицепь
		MirskyMinAntichainCover: maxChain,     // Мирский: min покрытие антицепями = max цепь
	}
}

// greedyAntichainSize — жадная антицепь: повторно берём узел с минимумом сравнимых
// среди оставшихся и убираем все сравнимые с ним (как Python greedy_antichain).
func greedyAntichainSize(w *workDAG) int {
	desc := w.descendantsAll()

	comparable := func(x, y string) bool { return desc[x][y] || desc[y][x] }

	remaining := make(map[string]bool, len(w.nodes))
	for _, id := range w.nodes {
		remaining[id] = true
	}

	size := 0

	for len(remaining) > 0 {
		// Узел с минимумом сравнимых; tie-break по имени для детерминизма.
		best := ""
		bestCount := -1

		ordered := make([]string, 0, len(remaining))
		for id := range remaining {
			ordered = append(ordered, id)
		}

		sort.Strings(ordered)

		for _, x := range ordered {
			cnt := 0

			for _, y := range ordered {
				if y != x && comparable(x, y) {
					cnt++
				}
			}

			if bestCount == -1 || cnt < bestCount {
				bestCount = cnt
				best = x
			}
		}

		size++

		next := make(map[string]bool)
		for y := range remaining {
			if y != best && !comparable(best, y) {
				next[y] = true
			}
		}

		remaining = next
	}

	return size
}

// --- геодезические расстояния -----------------------------------------------

func computeGeodesic(dg *descriptorGraph) *GeodesicSignal {
	if len(dg.nodes) < 3 {
		return nil
	}

	adj := undirectedAdj(dg)

	// Наибольшая связная компонента неориентированной проекции.
	largest := largestComponent(dg.nodes, adj)
	if len(largest) < 3 {
		return nil
	}

	inCC := make(map[string]bool, len(largest))
	for _, id := range largest {
		inCC[id] = true
	}

	ecc := make(map[string]int, len(largest))
	wiener := 0

	pairCount := 0

	for _, u := range largest {
		dist := bfsOutgoing(adj, u)

		e := 0
		for _, v := range largest {
			d, ok := dist[v]
			if !ok {
				continue
			}

			if d > e {
				e = d
			}

			if u < v {
				wiener += d
				pairCount++
			}
		}

		ecc[u] = e
	}

	radius, diameter := 0, 0
	first := true

	for _, u := range largest {
		e := ecc[u]
		if first {
			radius, diameter = e, e
			first = false

			continue
		}

		if e < radius {
			radius = e
		}

		if e > diameter {
			diameter = e
		}
	}

	var center, periphery []string

	for _, u := range largest {
		if ecc[u] == radius {
			center = append(center, u)
		}

		if ecc[u] == diameter {
			periphery = append(periphery, u)
		}
	}

	sort.Strings(center)
	sort.Strings(periphery)

	avg := 0.0
	if pairCount > 0 {
		avg = float64(wiener) / float64(pairCount)
	}

	return &GeodesicSignal{
		Diameter:    diameter,
		Radius:      radius,
		AvgDistance: roundTo(avg, 1e4),
		WienerIndex: wiener,
		Center:      center,
		Periphery:   periphery,
	}
}

// largestComponent — наибольшая связная компонента неориентированного adj
// (отсортированный список узлов).
func largestComponent(nodes []string, adj map[string]map[string]bool) []string {
	visited := make(map[string]bool, len(nodes))

	var best []string

	for _, s := range nodes {
		if visited[s] {
			continue
		}

		comp := []string{}
		queue := []string{s}
		visited[s] = true

		for len(queue) > 0 {
			u := queue[0]
			queue = queue[1:]
			comp = append(comp, u)

			neigh := make([]string, 0, len(adj[u]))
			for v := range adj[u] {
				neigh = append(neigh, v)
			}

			sort.Strings(neigh)

			for _, v := range neigh {
				if !visited[v] {
					visited[v] = true
					queue = append(queue, v)
				}
			}
		}

		if len(comp) > len(best) {
			best = comp
		}
	}

	sort.Strings(best)

	return best
}
