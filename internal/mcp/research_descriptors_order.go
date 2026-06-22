package mcp

import (
	"math"
	"sort"

	"gonum.org/v1/gonum/graph/topo"
)

// batch-3 (тонкие) research-дескрипторы теории множеств/порядка/информации: классы
// эквивалентности (SCC), структура решётки (poset join/meet), анализ join/meet,
// уточнение разбиений, взаимная информация между пакетами. Все — СИГНАЛЫ под
// --signals (slow-режим), НЕ ERROR. Golden против реальных Python-валидаторов.
//
// ДЕТЕРМИНИЗМ: выборки пар берутся в отсортированном порядке узлов; finest/coarsest
// разрешают ничьи по фиксированному порядку (scc, topological, in_degree, out_degree).

// --- структуры сигналов -----------------------------------------------------

type EquivalenceClassesSignal struct {
	NumClasses        int     `json:"numClasses"`        // число SCC
	MaxClassSize      int     `json:"maxClassSize"`      //
	MinClassSize      int     `json:"minClassSize"`      //
	MeanClassSize     float64 `json:"meanClassSize"`     //
	SingletonClasses  int     `json:"singletonClasses"`  // классы размера 1
	NonTrivialClasses int     `json:"nonTrivialClasses"` // классы размера > 1 (циклы)
	QuotientNodes     int     `json:"quotientNodes"`     // узлы фактор-графа (= NumClasses)
	QuotientEdges     int     `json:"quotientEdges"`     // рёбра конденсации
	PartitionEntropy  float64 `json:"partitionEntropy"`  // -Σ p log2 p по размерам классов
}

type LatticeSignal struct {
	IsLattice         bool    `json:"isLattice"`
	IsJoinSemilattice bool    `json:"isJoinSemilattice"`
	IsMeetSemilattice bool    `json:"isMeetSemilattice"`
	JoinRatio         float64 `json:"joinRatio"` // доля пар с существующим join
	MeetRatio         float64 `json:"meetRatio"` // доля пар с существующим meet
	HasTop            bool    `json:"hasTop"`    // ровно один сток (out-degree 0)
	HasBottom         bool    `json:"hasBottom"` // ровно один исток (in-degree 0)
}

type JoinMeetSignal struct {
	PairsWithMeet     int     `json:"pairsWithMeet"`     // пары с общим предком
	PairsWithJoin     int     `json:"pairsWithJoin"`     // пары с общим потомком
	AvgMeetCandidates float64 `json:"avgMeetCandidates"` // среднее число общих предков
	AvgJoinCandidates float64 `json:"avgJoinCandidates"` // среднее число общих потомков
}

type PartitionRefinementSignal struct {
	NumBlocks map[string]int     `json:"numBlocks"` // scc/topological/in_degree/out_degree
	Entropies map[string]float64 `json:"entropies"`
	Finest    string             `json:"finest"`   // разбиение с мин. энтропией
	Coarsest  string             `json:"coarsest"` // разбиение с макс. энтропией
}

type MutualInformationSignal struct {
	HighMICount      int `json:"highMICount"`      // пары пакетов с MI > 0.7
	PackagesAnalyzed int `json:"packagesAnalyzed"` // число пакетов (по префиксу пути)
}

// --- классы эквивалентности (SCC) -------------------------------------------

func computeEquivalenceClasses(dg *descriptorGraph) *EquivalenceClassesSignal {
	n := len(dg.nodes)
	if n < 2 {
		return nil
	}

	comp, sccCount := sccPartition(dg)

	sizes := make([]int, sccCount)
	for _, k := range comp {
		sizes[k]++
	}

	maxSz, minSz, sum := sizes[0], sizes[0], 0
	singleton, nonTrivial := 0, 0

	for _, s := range sizes {
		if s > maxSz {
			maxSz = s
		}

		if s < minSz {
			minSz = s
		}

		sum += s

		if s == 1 {
			singleton++
		} else {
			nonTrivial++
		}
	}

	// Рёбра конденсации: различные пары (comp[from], comp[to]).
	qEdges := map[[2]int]bool{}
	for from, tos := range dg.outAdj {
		for to := range tos {
			if comp[from] != comp[to] {
				qEdges[[2]int{comp[from], comp[to]}] = true
			}
		}
	}

	// Энтропия разбиения по размерам классов.
	ent := 0.0
	for _, s := range sizes {
		p := float64(s) / float64(n)
		ent -= p * math.Log2(p+1e-10)
	}

	return &EquivalenceClassesSignal{
		NumClasses:        sccCount,
		MaxClassSize:      maxSz,
		MinClassSize:      minSz,
		MeanClassSize:     roundTo(float64(sum)/float64(sccCount), 1e2),
		SingletonClasses:  singleton,
		NonTrivialClasses: nonTrivial,
		QuotientNodes:     sccCount,
		QuotientEdges:     len(qEdges),
		PartitionEntropy:  roundTo(ent, 1e4),
	}
}

// sccPartition — полная SCC-разбивка: node -> id класса, и число классов.
func sccPartition(dg *descriptorGraph) (map[string]int, int) {
	gg := dg.gonumGraph()
	comp := make(map[string]int, len(dg.nodes))

	sccs := topo.TarjanSCC(gg)
	for k, scc := range sccs {
		for _, nd := range scc {
			comp[dg.nodes[int(nd.ID())]] = k
		}
	}

	return comp, len(sccs)
}

// --- структура решётки (poset join/meet) ------------------------------------

func computeLattice(dg *descriptorGraph) *LatticeSignal {
	n := len(dg.nodes)
	if n < 3 || n > researchNodeLimit {
		return nil
	}

	w := buildWorkDAG(dg)
	reach := w.descendantsAll() // reach[x] = строго достижимые из x

	// le(x,y): y достижим из x ИЛИ x==y (рефлексивная достижимость).
	le := func(x, y string) bool { return x == y || reach[x][y] }

	m := len(w.nodes)
	sample := 20
	if pairs := m * (m - 1) / 2; pairs < sample {
		sample = pairs
	}

	joinExists, meetExists, total := 0, 0, 0

	for i := 0; i < m && total < sample; i++ {
		for j := i + 1; j < m && total < sample; j++ {
			a, b := w.nodes[i], w.nodes[j]
			total++

			if latticeJoin(w.nodes, le, a, b) {
				joinExists++
			}

			if latticeMeet(w.nodes, le, a, b) {
				meetExists++
			}
		}
	}

	isJoin := total > 0 && joinExists == total
	isMeet := total > 0 && meetExists == total

	top, bottom := 0, 0
	for _, id := range w.nodes {
		if len(w.out[id]) == 0 {
			top++
		}

		if w.inDeg[id] == 0 {
			bottom++
		}
	}

	jr, mr := 0.0, 0.0
	if total > 0 {
		jr = float64(joinExists) / float64(total)
		mr = float64(meetExists) / float64(total)
	}

	return &LatticeSignal{
		IsLattice:         isJoin && isMeet,
		IsJoinSemilattice: isJoin,
		IsMeetSemilattice: isMeet,
		JoinRatio:         roundTo(jr, 1e4),
		MeetRatio:         roundTo(mr, 1e4),
		HasTop:            top == 1,
		HasBottom:         bottom == 1,
	}
}

// latticeJoin — существует ли наименьшая верхняя грань (join) пары a,b.
func latticeJoin(nodes []string, le func(x, y string) bool, a, b string) bool {
	var ub []string

	for _, c := range nodes {
		if le(a, c) && le(b, c) {
			ub = append(ub, c)
		}
	}

	if len(ub) == 0 {
		return false
	}

	for _, cand := range ub {
		all := true

		for _, other := range ub {
			if !le(cand, other) {
				all = false

				break
			}
		}

		if all {
			return true
		}
	}

	return false
}

// latticeMeet — существует ли наибольшая нижняя грань (meet) пары a,b.
func latticeMeet(nodes []string, le func(x, y string) bool, a, b string) bool {
	var lb []string

	for _, c := range nodes {
		if le(c, a) && le(c, b) {
			lb = append(lb, c)
		}
	}

	if len(lb) == 0 {
		return false
	}

	for _, cand := range lb {
		all := true

		for _, other := range lb {
			if !le(other, cand) {
				all = false

				break
			}
		}

		if all {
			return true
		}
	}

	return false
}

// --- анализ join/meet (общие предки/потомки исходного графа) ----------------

func computeJoinMeet(dg *descriptorGraph) *JoinMeetSignal {
	n := len(dg.nodes)
	if n < 3 {
		return nil
	}

	// Обратная смежность для предков.
	rev := make(map[string]map[string]bool, n)
	for _, id := range dg.nodes {
		rev[id] = map[string]bool{}
	}

	for from, tos := range dg.outAdj {
		for to := range tos {
			rev[to][from] = true
		}
	}

	anc := make(map[string]map[string]bool, n)
	desc := make(map[string]map[string]bool, n)

	for _, id := range dg.nodes {
		a := reachableFromSuccessors(rev, id)
		a[id] = true
		anc[id] = a

		d := reachableFromSuccessors(dg.outAdj, id)
		d[id] = true
		desc[id] = d
	}

	intersectSize := func(x, y map[string]bool) int {
		c := 0

		small, big := x, y
		if len(y) < len(x) {
			small, big = y, x
		}

		for k := range small {
			if big[k] {
				c++
			}
		}

		return c
	}

	pairsWithMeet, pairsWithJoin := 0, 0
	sumMeet, sumJoin := 0, 0

	const sampleLimit = 15

	count := 0

	for i := 0; i < n && count < sampleLimit; i++ {
		for j := i + 1; j < n && count < sampleLimit; j++ {
			a, b := dg.nodes[i], dg.nodes[j]
			count++

			if m := intersectSize(anc[a], anc[b]); m > 0 {
				pairsWithMeet++
				sumMeet += m
			}

			if jn := intersectSize(desc[a], desc[b]); jn > 0 {
				pairsWithJoin++
				sumJoin += jn
			}
		}
	}

	avgMeet, avgJoin := 0.0, 0.0
	if pairsWithMeet > 0 {
		avgMeet = float64(sumMeet) / float64(pairsWithMeet)
	}

	if pairsWithJoin > 0 {
		avgJoin = float64(sumJoin) / float64(pairsWithJoin)
	}

	return &JoinMeetSignal{
		PairsWithMeet:     pairsWithMeet,
		PairsWithJoin:     pairsWithJoin,
		AvgMeetCandidates: roundTo(avgMeet, 1e2),
		AvgJoinCandidates: roundTo(avgJoin, 1e2),
	}
}

// --- уточнение разбиений ----------------------------------------------------

func computePartitionRefinement(dg *descriptorGraph) *PartitionRefinementSignal {
	n := len(dg.nodes)
	if n < 3 {
		return nil
	}

	numBlocks := map[string]int{}
	entropies := map[string]float64{}

	// Фиксированный порядок для детерминированного finest/coarsest.
	order := []string{"scc", "topological", "in_degree", "out_degree"}

	// SCC.
	_, sccCount := sccPartition(dg)
	sccSizes := sccBlockSizes(dg, sccCount)
	numBlocks["scc"] = sccCount
	entropies["scc"] = roundTo(blockEntropy(topK(sccSizes, 5)), 1e4)

	// Топологические уровни (только для DAG).
	isDAG, _ := dagDepth(dg)
	if isDAG {
		w := buildWorkDAG(dg)
		levels := w.topologicalGenerations()
		numBlocks["topological"] = len(levels)
		entropies["topological"] = roundTo(blockEntropy(topK(levels, 10)), 1e4)
	}

	// In-degree / out-degree разбиения.
	inSizes := degreeBlockSizes(dg.nodes, dg.inDeg)
	numBlocks["in_degree"] = len(inSizes)
	entropies["in_degree"] = roundTo(blockEntropy(topK(inSizes, 5)), 1e4)

	outSizes := degreeBlockSizes(dg.nodes, dg.outDeg)
	numBlocks["out_degree"] = len(outSizes)
	entropies["out_degree"] = roundTo(blockEntropy(topK(outSizes, 5)), 1e4)

	finest, coarsest := "", ""
	minE, maxE := 0.0, 0.0

	for _, name := range order {
		e, ok := entropies[name]
		if !ok {
			continue
		}

		if finest == "" || e < minE {
			minE = e
			finest = name
		}

		if coarsest == "" || e > maxE {
			maxE = e
			coarsest = name
		}
	}

	return &PartitionRefinementSignal{
		NumBlocks: numBlocks,
		Entropies: entropies,
		Finest:    finest,
		Coarsest:  coarsest,
	}
}

func sccBlockSizes(dg *descriptorGraph, sccCount int) []int {
	comp, _ := sccPartition(dg)
	sizes := make([]int, sccCount)

	for _, k := range comp {
		sizes[k]++
	}

	sort.Sort(sort.Reverse(sort.IntSlice(sizes)))

	return sizes
}

func degreeBlockSizes(nodes []string, deg map[string]int) []int {
	classes := map[int]int{}
	for _, id := range nodes {
		classes[deg[id]]++
	}

	sizes := make([]int, 0, len(classes))
	for _, c := range classes {
		sizes = append(sizes, c)
	}

	sort.Sort(sort.Reverse(sort.IntSlice(sizes)))

	return sizes
}

func topK(sizes []int, k int) []int {
	if len(sizes) > k {
		return sizes[:k]
	}

	return sizes
}

func blockEntropy(sizes []int) float64 {
	total := 0
	for _, s := range sizes {
		total += s
	}

	if total == 0 {
		return 0
	}

	ent := 0.0
	for _, s := range sizes {
		p := float64(s) / float64(total)
		ent -= p * math.Log2(p+1e-10)
	}

	return ent
}

// --- взаимная информация между пакетами -------------------------------------

func computeMutualInformation(dg *descriptorGraph) *MutualInformationSignal {
	pkgOf := func(id string) string {
		for i := 0; i < len(id); i++ {
			if id[i] == '/' {
				return id[:i]
			}
		}

		return id
	}

	pkgNodes := map[string][]string{}
	for _, id := range dg.nodes {
		p := pkgOf(id)
		pkgNodes[p] = append(pkgNodes[p], id)
	}

	if len(pkgNodes) < 2 {
		return nil
	}

	pkgOfNode := make(map[string]string, len(dg.nodes))
	for _, id := range dg.nodes {
		pkgOfNode[id] = pkgOf(id)
	}

	// Направленные рёбра между пакетами: cross(p,q) = e(p->q)+e(q->p).
	cross := map[[2]string]int{}
	for from, tos := range dg.outAdj {
		pf := pkgOfNode[from]

		for to := range tos {
			pt := pkgOfNode[to]
			if pf == pt {
				continue
			}

			a, b := pf, pt
			if b < a {
				a, b = b, a
			}

			cross[[2]string{a, b}]++
		}
	}

	pkgs := make([]string, 0, len(pkgNodes))
	for p := range pkgNodes {
		pkgs = append(pkgs, p)
	}

	sort.Strings(pkgs)

	high := 0

	for i := 0; i < len(pkgs); i++ {
		for j := i + 1; j < len(pkgs); j++ {
			p1, p2 := pkgs[i], pkgs[j]
			c := cross[[2]string{p1, p2}]
			maxEdges := len(pkgNodes[p1]) * len(pkgNodes[p2]) * 2

			if maxEdges > 0 {
				mi := float64(c) / float64(maxEdges)
				if mi > 0.7 {
					high++
				}
			}
		}
	}

	return &MutualInformationSignal{
		HighMICount:      high,
		PackagesAnalyzed: len(pkgNodes),
	}
}
