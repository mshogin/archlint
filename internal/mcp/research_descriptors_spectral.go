package mcp

import (
	"math"
	"sort"

	"gonum.org/v1/gonum/mat"
)

// batch-3 спектральная близость на псевдообратной лапласиана G = L⁺ (Moore-Penrose)
// неориентированной проекции. Оба — СИГНАЛЫ под --signals (slow), НЕ ERROR, НЕ дефект
// (спектр = анализ, не доказательство). G(a,a), G(a,b) и эффективное сопротивление
// R(a,b) GAUGE-ИНВАРИАНТНЫ (не зависят от обработки null-space L⁺) -> детерминизм.
//
// АРХ-ЧТЕНИЕ (подтверждено разбором семантики):
//   - 1/G(v,v) = current-flow closeness centrality (НЕ throughput/bottleneck — то
//     betweenness): ВЫСОКОЕ = модуль электрически централен (низкое сопротивление,
//     много путей, глубоко встроен); НИЗКОЕ = периферийный/слабо-прицепленный модуль.
//   - commute κ(a,b)=2|E|·R(a,b) = МУЛЬТИ-ПУТЕВАЯ структурная близость (в отличие от
//     geodesic, учитывает избыточность путей): низкое κ = тесно/избыточно связаны,
//     высокое κ = слабо связаны / near-separable (кандидаты на естественный разрез).

// ResistanceClosenessSignal — порт validate_node_capacity, честно переименованный:
// 1/diag(L⁺) = resistance closeness, НЕ «ёмкость»/«бутылочное горло».
type ResistanceClosenessSignal struct {
	MeanCloseness  float64  `json:"meanCloseness"`  // среднее 1/G(v,v)
	StdCloseness   float64  `json:"stdCloseness"`   // СКО (population)
	MostCentral    []string `json:"mostCentral"`    // макс. closeness — глубоко встроенные
	MostPeripheral []string `json:"mostPeripheral"` // мин. closeness — периферийные
}

// CommuteTimeSignal — порт validate_commute_time. κ = время random-walk туда-обратно
// = мульти-путевая близость модулей.
type CommuteTimeSignal struct {
	NumEdges        int     `json:"numEdges"`
	AvgCommuteTime  float64 `json:"avgCommuteTime"`
	MaxCommuteTime  float64 `json:"maxCommuteTime"`
	MostDistantPair string  `json:"mostDistantPair"` // макс. κ — слабейше связанная пара
}

// laplacianPinv — псевдообратная Мура-Пенроуза лапласиана неориентированной проекции:
// L⁺ = Σ_{λ_k > tol} (1/λ_k) v_k v_kᵀ. Возвращает матрицу G[i][j] в порядке dg.nodes.
func laplacianPinv(dg *descriptorGraph) [][]float64 {
	n := len(dg.nodes)
	if n < 2 {
		return nil
	}

	adj := undirectedAdj(dg)

	// Симметричный лапласиан L = D - A.
	sd := mat.NewSymDense(n, nil)

	for i, id := range dg.nodes {
		deg := len(adj[id])
		sd.SetSym(i, i, float64(deg))

		for j := i + 1; j < n; j++ {
			if adj[id][dg.nodes[j]] {
				sd.SetSym(i, j, -1)
			}
		}
	}

	var es mat.EigenSym
	if ok := es.Factorize(sd, true); !ok {
		return nil
	}

	vals := es.Values(nil)

	var vecs mat.Dense

	es.VectorsTo(&vecs)

	const tol = 1e-10

	g := make([][]float64, n)
	for i := range g {
		g[i] = make([]float64, n)
	}

	for k := 0; k < n; k++ {
		if vals[k] <= tol {
			continue // null-space (включая константный вектор связной компоненты)
		}

		inv := 1.0 / vals[k]

		for i := 0; i < n; i++ {
			vik := vecs.At(i, k)
			if vik == 0 {
				continue
			}

			for j := 0; j < n; j++ {
				g[i][j] += inv * vik * vecs.At(j, k)
			}
		}
	}

	return g
}

func computeResistanceCloseness(dg *descriptorGraph) *ResistanceClosenessSignal {
	n := len(dg.nodes)
	if n < 3 {
		return nil
	}

	g := laplacianPinv(dg)
	if g == nil {
		return nil
	}

	type nc struct {
		id  string
		cap float64
	}

	caps := make([]nc, 0, n)

	var sum, sumSq float64

	finite := 0

	for i, id := range dg.nodes {
		diag := g[i][i]
		if diag <= 1e-10 {
			continue // бесконечная closeness — изолированный узел; в статистику не идёт
		}

		c := 1.0 / diag
		caps = append(caps, nc{id, c})
		sum += c
		sumSq += c * c
		finite++
	}

	if finite == 0 {
		return nil
	}

	mean := sum / float64(finite)
	variance := sumSq/float64(finite) - mean*mean
	if variance < 0 {
		variance = 0
	}

	// Сортировка по closeness (cap); tie-break по имени.
	sort.Slice(caps, func(i, j int) bool {
		if caps[i].cap != caps[j].cap {
			return caps[i].cap > caps[j].cap // по убыванию: центральные первыми
		}

		return caps[i].id < caps[j].id
	})

	limit := 5
	if limit > len(caps) {
		limit = len(caps)
	}

	central := make([]string, 0, limit)
	for _, x := range caps[:limit] {
		central = append(central, x.id)
	}

	peripheral := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		peripheral = append(peripheral, caps[len(caps)-1-i].id)
	}

	return &ResistanceClosenessSignal{
		MeanCloseness:  roundTo(mean, 1e4),
		StdCloseness:   roundTo(math.Sqrt(variance), 1e4),
		MostCentral:    central,
		MostPeripheral: peripheral,
	}
}

func computeCommuteTime(dg *descriptorGraph) *CommuteTimeSignal {
	n := len(dg.nodes)
	if n < 3 {
		return nil
	}

	// Требуется связность неориентированной проекции (как Python).
	if weaklyConnectedComponents(dg) != 1 {
		return nil
	}

	adj := undirectedAdj(dg)
	e := len(undirectedEdgePairs(adj, dg.nodes))

	g := laplacianPinv(dg)
	if g == nil {
		return nil
	}

	var sum, maxK float64

	pairs := 0
	maxPair := ""

	for i := 0; i < n; i++ {
		for j := i + 1; j < n; j++ {
			r := g[i][i] + g[j][j] - 2*g[i][j]
			kappa := 2 * float64(e) * r
			sum += kappa
			pairs++

			if maxPair == "" || kappa > maxK {
				maxK = kappa
				maxPair = dg.nodes[i] + "--" + dg.nodes[j]
			}
		}
	}

	avg := 0.0
	if pairs > 0 {
		avg = sum / float64(pairs)
	}

	return &CommuteTimeSignal{
		NumEdges:        e,
		AvgCommuteTime:  roundTo(avg, 1e2),
		MaxCommuteTime:  roundTo(maxK, 1e2),
		MostDistantPair: maxPair,
	}
}
