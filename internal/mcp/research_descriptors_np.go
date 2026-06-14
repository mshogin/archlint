package mcp

import (
	"math"
	"sort"

	"github.com/mshogin/archlint/internal/model"
)

// batch-2 research-дескрипторы: медленные граф-NP инварианты и спектральная
// топология (порт validator advanced_graph_metrics / advanced_metrics /
// advanced_topology_metrics / game_theory_metrics). Все — СИГНАЛЫ под --signals,
// НЕ ERROR. NP-метрики используют те же эвристики, что Python; точные (shapley)
// ограничены по размеру (как Python). Эквивалентность — golden против Python.
//
// ДЕТЕРМИНИЗМ: жадные аппроксимации (dominating/independence/vertex_cover) и
// раскраска перебирают узлы/рёбра в ОТСОРТИРОВАННОМ порядке. Python использует
// порядок итерации set (hash-зависимый, нестабильный между прогонами); Go даёт
// каноническую воспроизводимую версию той же эвристики. Размеры на фикстурах
// совпадают (инвариантны к порядку).

// --- структуры сигналов -----------------------------------------------------

type TreewidthSignal struct {
	TreewidthLowerBound int `json:"treewidthLowerBound"` // = degeneracy (нижняя граница)
	TreewidthUpperBound int `json:"treewidthUpperBound"` // = degeneracy*2 (грубая верхняя)
	Degeneracy          int `json:"degeneracy"`          // макс. core number
}

type ChromaticSignal struct {
	ChromaticLowerBound int         `json:"chromaticLowerBound"` // макс. клика (n<=100), иначе degeneracy
	ChromaticUpperBound int         `json:"chromaticUpperBound"` // жадная раскраска largest-first
	ColorDistribution   map[int]int `json:"colorDistribution"`   // цвет -> число узлов
}

type DominatingSetSignal struct {
	DominationNumber int     `json:"dominationNumber"`
	TotalNodes       int     `json:"totalNodes"`
	DominationRatio  float64 `json:"dominationRatio"`
}

type IndependenceSignal struct {
	IndependenceNumber int     `json:"independenceNumber"`
	TotalNodes         int     `json:"totalNodes"`
	IndependenceRatio  float64 `json:"independenceRatio"`
}

type VertexCoverSignal struct {
	CoverSize  int     `json:"coverSize"`
	TotalNodes int     `json:"totalNodes"`
	CoverRatio float64 `json:"coverRatio"`
}

type GraphCliquesSignal struct {
	MaxCliqueSize     int `json:"maxCliqueSize"`
	TotalCliques      int `json:"totalCliques"`      // число максимальных клик
	LargeCliquesCount int `json:"largeCliquesCount"` // клики размером > 4 (порог Python)
}

type PersistentHomologySignal struct {
	BarcodeH0Count       int     `json:"barcodeH0Count"`       // компоненты (слияния + 1 финальная)
	BarcodeH1Count       int     `json:"barcodeH1Count"`       // циклы
	ShortLivedComponents int     `json:"shortLivedComponents"` // слияния с persistence <= 2
	AvgPersistenceH0     float64 `json:"avgPersistenceH0"`
	CyclesCreated        int     `json:"cyclesCreated"`
}

type HodgeSignal struct {
	Harmonic0     int     `json:"harmonic0"`     // dim H⁰ = число компонент
	Harmonic1     int     `json:"harmonic1"`     // dim H¹ = β₁
	ExpectedH0    int     `json:"expectedH0"`    // c (компоненты)
	ExpectedH1    int     `json:"expectedH1"`    // e - n + c
	SpectralGapL0 float64 `json:"spectralGapL0"` // мин. ненулевое СЗ лапласиана узлов
	SpectralGapL1 float64 `json:"spectralGapL1"` // = L0 (общий ненулевой спектр B·Bᵀ и Bᵀ·B)
}

type ShapleySignal struct {
	ShapleyValues       map[string]float64 `json:"shapleyValues"`
	TotalShapley        float64            `json:"totalShapley"`
	GrandCoalitionValue float64            `json:"grandCoalitionValue"`
	EfficiencyCheck     float64            `json:"efficiencyCheck"` // |total - v(N)|
}

// --- общие undirected-хелперы -----------------------------------------------

// undirectedEdgePairs — уникальные неориентированные рёбра (a<b) в порядке (a,b).
func undirectedEdgePairs(adj map[string]map[string]bool, nodes []string) [][2]string {
	var pairs [][2]string

	for _, a := range nodes {
		neigh := make([]string, 0, len(adj[a]))
		for b := range adj[a] {
			if a < b {
				neigh = append(neigh, b)
			}
		}

		sort.Strings(neigh)

		for _, b := range neigh {
			pairs = append(pairs, [2]string{a, b})
		}
	}

	return pairs
}

// --- treewidth / chromatic --------------------------------------------------

func computeTreewidth(dg *descriptorGraph) *TreewidthSignal {
	if len(dg.nodes) < 2 {
		return nil
	}

	deg := maxKCore(dg) // degeneracy = макс. core number (как nx.core_number max)

	return &TreewidthSignal{
		TreewidthLowerBound: deg,
		TreewidthUpperBound: deg * 2,
		Degeneracy:          deg,
	}
}

func computeChromatic(dg *descriptorGraph) *ChromaticSignal {
	n := len(dg.nodes)
	if n < 2 {
		return nil
	}

	adj := undirectedAdj(dg)

	// Жадная раскраска largest-first: порядок = степень убыв., имя возр. (детерминизм).
	order := make([]string, len(dg.nodes))
	copy(order, dg.nodes)

	sort.SliceStable(order, func(i, j int) bool {
		di, dj := len(adj[order[i]]), len(adj[order[j]])
		if di != dj {
			return di > dj
		}

		return order[i] < order[j]
	})

	color := make(map[string]int, n)
	dist := make(map[int]int)
	maxColor := 0

	for _, u := range order {
		used := make(map[int]bool)
		for v := range adj[u] {
			if c, ok := color[v]; ok {
				used[c] = true
			}
		}

		c := 0
		for used[c] {
			c++
		}

		color[u] = c
		dist[c]++

		if c > maxColor {
			maxColor = c
		}
	}

	upper := maxColor + 1

	// Нижняя граница: макс. клика (n<=100), иначе degeneracy.
	lower := maxKCore(dg)
	if n <= 100 {
		lower = maxCliqueSize(adj, dg.nodes)
	}

	return &ChromaticSignal{
		ChromaticLowerBound: lower,
		ChromaticUpperBound: upper,
		ColorDistribution:   dist,
	}
}

// --- dominating set / independence / vertex cover (жадные аппроксимации) -----

func computeDominatingSet(dg *descriptorGraph) *DominatingSetSignal {
	n := len(dg.nodes)
	if n < 2 {
		return nil
	}

	adj := undirectedAdj(dg)

	closed := func(x string) map[string]bool {
		s := map[string]bool{x: true}
		for v := range adj[x] {
			s[v] = true
		}

		return s
	}

	covered := make(map[string]bool)
	inSet := make(map[string]bool)
	size := 0

	for len(covered) < n {
		best := ""
		bestCov := 0

		for _, node := range dg.nodes {
			if inSet[node] {
				continue
			}

			cov := 0
			for v := range closed(node) {
				if !covered[v] {
					cov++
				}
			}

			if cov > bestCov {
				bestCov = cov
				best = node
			}
		}

		if best == "" {
			break
		}

		inSet[best] = true
		size++

		for v := range closed(best) {
			covered[v] = true
		}
	}

	ratio := float64(size) / float64(n)

	return &DominatingSetSignal{
		DominationNumber: size,
		TotalNodes:       n,
		DominationRatio:  roundTo(ratio, 1e3),
	}
}

func computeIndependenceNumber(dg *descriptorGraph) *IndependenceSignal {
	n := len(dg.nodes)
	if n < 2 {
		return nil
	}

	adj := undirectedAdj(dg)

	available := make(map[string]bool, n)
	for _, id := range dg.nodes {
		available[id] = true
	}

	size := 0

	for len(available) > 0 {
		// Узел минимальной (полной) степени; tie-break по имени.
		best := ""
		bestDeg := -1

		for _, id := range dg.nodes {
			if !available[id] {
				continue
			}

			d := len(adj[id])
			if bestDeg == -1 || d < bestDeg {
				bestDeg = d
				best = id
			}
		}

		size++

		delete(available, best)
		for v := range adj[best] {
			delete(available, v)
		}
	}

	ratio := float64(size) / float64(n)

	return &IndependenceSignal{
		IndependenceNumber: size,
		TotalNodes:         n,
		IndependenceRatio:  roundTo(ratio, 1e3),
	}
}

func computeVertexCover(dg *descriptorGraph) *VertexCoverSignal {
	n := len(dg.nodes)
	adj := undirectedAdj(dg)
	edges := undirectedEdgePairs(adj, dg.nodes)

	if len(edges) == 0 {
		return nil
	}

	// 2-приближение: проходим рёбра в отсортированном порядке, непокрытое ребро ->
	// в покрытие оба конца.
	cover := make(map[string]bool)

	for _, e := range edges {
		if cover[e[0]] || cover[e[1]] {
			continue
		}

		cover[e[0]] = true
		cover[e[1]] = true
	}

	size := len(cover)

	ratio := 0.0
	if n > 0 {
		ratio = float64(size) / float64(n)
	}

	return &VertexCoverSignal{
		CoverSize:  size,
		TotalNodes: n,
		CoverRatio: roundTo(ratio, 1e3),
	}
}

// --- клики (Bron-Kerbosch) --------------------------------------------------

// maximalCliques — все максимальные клики неориентированного графа (Bron-Kerbosch
// без поворота; изолированный узел = клика размера 1, как nx.find_cliques).
func maximalCliques(adj map[string]map[string]bool, nodes []string) [][]string {
	var out [][]string

	var bk func(r, p, x []string)
	bk = func(r, p, x []string) {
		if len(p) == 0 && len(x) == 0 {
			clique := append([]string(nil), r...)
			out = append(out, clique)

			return
		}

		// Итерируем копию p в детерминированном порядке.
		pIter := append([]string(nil), p...)
		sort.Strings(pIter)

		for _, v := range pIter {
			var nr, np, nx []string

			nr = append(append([]string(nil), r...), v)

			for _, w := range p {
				if adj[v][w] {
					np = append(np, w)
				}
			}

			for _, w := range x {
				if adj[v][w] {
					nx = append(nx, w)
				}
			}

			bk(nr, np, nx)

			// Перенос v из P в X.
			p = removeStr(p, v)
			x = append(x, v)
		}
	}

	all := append([]string(nil), nodes...)
	sort.Strings(all)
	bk(nil, all, nil)

	return out
}

func removeStr(s []string, v string) []string {
	out := s[:0]
	for _, x := range s {
		if x != v {
			out = append(out, x)
		}
	}

	return out
}

func maxCliqueSize(adj map[string]map[string]bool, nodes []string) int {
	maxSz := 0
	for _, c := range maximalCliques(adj, nodes) {
		if len(c) > maxSz {
			maxSz = len(c)
		}
	}

	return maxSz
}

func computeGraphCliques(dg *descriptorGraph) *GraphCliquesSignal {
	if len(dg.nodes) < 1 {
		return nil
	}

	adj := undirectedAdj(dg)
	cliques := maximalCliques(adj, dg.nodes)

	maxSz, large := 0, 0
	for _, c := range cliques {
		if len(c) > maxSz {
			maxSz = len(c)
		}

		if len(c) > 4 { // порог Python (max_clique_size=4)
			large++
		}
	}

	return &GraphCliquesSignal{
		MaxCliqueSize:     maxSz,
		TotalCliques:      len(cliques),
		LargeCliquesCount: large,
	}
}

// --- persistent homology (фильтрация по степени) ----------------------------

func computePersistentHomology(dg *descriptorGraph) *PersistentHomologySignal {
	n := len(dg.nodes)
	if n < 3 {
		return nil
	}

	adj := undirectedAdj(dg)
	edges := undirectedEdgePairs(adj, dg.nodes)

	type wedge struct {
		w    int
		a, b string
	}

	we := make([]wedge, 0, len(edges))
	for _, e := range edges {
		w := len(adj[e[0]])
		if len(adj[e[1]]) < w {
			w = len(adj[e[1]])
		}

		we = append(we, wedge{w, e[0], e[1]})
	}

	// Фильтрация: рёбра по убыванию (weight, a, b) — как Python sort(reverse=True).
	sort.Slice(we, func(i, j int) bool {
		if we[i].w != we[j].w {
			return we[i].w > we[j].w
		}

		if we[i].a != we[j].a {
			return we[i].a > we[j].a
		}

		return we[i].b > we[j].b
	})

	parent := make(map[string]string, n)
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

	var merges []int

	cycles := 0

	for idx, e := range we {
		fval := idx + 1
		ra, rb := find(e.a), find(e.b)

		if ra != rb {
			parent[ra] = rb
			merges = append(merges, fval)
		} else {
			cycles++
		}
	}

	short := 0
	sum := 0

	for _, p := range merges {
		if p <= 2 {
			short++
		}

		sum += p
	}

	avg := 0.0
	if len(merges) > 0 {
		avg = float64(sum) / float64(len(merges))
	}

	return &PersistentHomologySignal{
		BarcodeH0Count:       len(merges) + 1, // + финальная бесконечная компонента
		BarcodeH1Count:       cycles,
		ShortLivedComponents: short,
		AvgPersistenceH0:     roundTo(avg, 1e3),
		CyclesCreated:        cycles,
	}
}

// --- разложение Ходжа -------------------------------------------------------

func computeHodge(g *model.Graph, dg *descriptorGraph) *HodgeSignal {
	n := len(dg.nodes)

	adj := undirectedAdj(dg)
	eu := len(undirectedEdgePairs(adj, dg.nodes))

	if n < 2 || eu < 1 || n > 300 {
		return nil
	}

	c := weaklyConnectedComponents(dg) // неориентированные компоненты

	// Гармонические числа = комбинаторные топологические инварианты (нулевое
	// пространство лапласианов Ходжа): dim H⁰ = c, dim H¹ = β₁ = e - n + c.
	betti1 := eu - n + c

	// Спектральный зазор = мин. ненулевое СЗ лапласиана узлов L₀. Ненулевой спектр
	// L₀=B·Bᵀ и L₁=Bᵀ·B совпадает -> spectral_gap_L0 == spectral_gap_L1.
	gap := minNonzeroEigenvalue(laplacianSpectrumOf(g))

	return &HodgeSignal{
		Harmonic0:     c,
		Harmonic1:     betti1,
		ExpectedH0:    c,
		ExpectedH1:    betti1,
		SpectralGapL0: roundTo(gap, 1e4),
		SpectralGapL1: roundTo(gap, 1e4),
	}
}

// minNonzeroEigenvalue — минимальное собственное значение >= 1e-10 (0 если нет).
func minNonzeroEigenvalue(spectrum []float64) float64 {
	const zero = 1e-10

	min := 0.0
	found := false

	for _, v := range spectrum {
		if v >= zero {
			if !found || v < min {
				min = v
				found = true
			}
		}
	}

	return min
}

// --- значение Шепли (точное, n <= 15) ---------------------------------------

func computeShapley(dg *descriptorGraph) *ShapleySignal {
	n := len(dg.nodes)
	if n < 2 || n > 15 {
		return nil
	}

	nodes := dg.nodes
	idx := make(map[string]int, n)
	for i, id := range nodes {
		idx[id] = i
	}

	// Ориентированные (дедуплицированные) рёбра для v(S).
	type edge struct{ u, v int }

	var edges []edge

	for from, tos := range dg.outAdj {
		for to := range tos {
			edges = append(edges, edge{idx[from], idx[to]})
		}
	}

	// v(S) = внутренние рёбра + 0.5 * исходящие (S задана битовой маской).
	value := func(mask uint) float64 {
		if mask == 0 {
			return 0
		}

		internal, outgoing := 0, 0

		for _, e := range edges {
			uin := mask&(1<<uint(e.u)) != 0
			vin := mask&(1<<uint(e.v)) != 0

			if uin && vin {
				internal++
			} else if uin && !vin {
				outgoing++
			}
		}

		return float64(internal) + 0.5*float64(outgoing)
	}

	fact := make([]float64, n+1)
	fact[0] = 1
	for i := 1; i <= n; i++ {
		fact[i] = fact[i-1] * float64(i)
	}

	shap := make(map[string]float64, n)
	total := 0.0

	for i := 0; i < n; i++ {
		// Игроки кроме i.
		others := make([]int, 0, n-1)
		for j := 0; j < n; j++ {
			if j != i {
				others = append(others, j)
			}
		}

		phi := 0.0

		// Перебор всех подмножеств S из others.
		for sub := uint(0); sub < (1 << uint(len(others))); sub++ {
			var mask uint

			size := 0

			for k, p := range others {
				if sub&(1<<uint(k)) != 0 {
					mask |= 1 << uint(p)
					size++
				}
			}

			withI := mask | (1 << uint(i))
			marginal := value(withI) - value(mask)
			weight := fact[size] * fact[n-size-1] / fact[n]
			phi += weight * marginal
		}

		v := roundTo(phi, 1e4)
		shap[nodes[i]] = v
		total += v
	}

	grand := value((1 << uint(n)) - 1)

	return &ShapleySignal{
		ShapleyValues:       shap,
		TotalShapley:        roundTo(total, 1e4),
		GrandCoalitionValue: roundTo(grand, 1e4),
		EfficiencyCheck:     roundTo(math.Abs(total-grand), 1e6),
	}
}
