package mcp

import (
	"math"
	"sort"

	"github.com/mshogin/archlint/internal/model"
	"gonum.org/v1/gonum/graph/simple"
	"gonum.org/v1/gonum/graph/spectral"
	"gonum.org/v1/gonum/mat"
)

// SpectralDistance — INFO-СКРИН/роутер на pattern-diff между двумя версиями графа.
// НИКОГДА не ERROR/блок (спектр != proof): это сигнал «структура изменилась
// спектрально» для роутинга к более дорогим проверкам, не приговор.
//
// Алгоритм (MVP, L2-падинг): спектр симметричного лапласиана обеих версий ->
// отсортировать -> zero-pad до общей длины -> ||λ_base − λ_curr||_2 -> нормировать.
// Wasserstein отложен.
//
// ★ДЕТЕРМИНИЗМ-ЗАЩИТА (обязательна): λ округляются до 1e-9 (гасим FP-шум
// эйгендекомпозиции), epsilon=1e-6 (выше noise-floor). ACCEPTANCE: дважды на ОДНОМ
// графе -> Distance≈0, Shifted=false (округление делает идентичные спектры точно
// равными -> Distance ровно 0).
type SpectralDistance struct {
	Distance float64 `json:"distance"`
	Shifted  bool    `json:"shifted"` // Distance > epsilon
}

// spectralEpsilon — порог сдвига: выше численного noise-floor округлённого спектра.
const spectralEpsilon = 1e-6

// ComputeSpectralDistance сравнивает спектры лапласианов base и curr.
func ComputeSpectralDistance(base, curr *model.Graph) SpectralDistance {
	b := laplacianSpectrumOf(base)
	c := laplacianSpectrumOf(curr)

	maxLen := len(b)
	if len(c) > maxLen {
		maxLen = len(c)
	}

	var ss, normB, normC float64

	for i := 0; i < maxLen; i++ {
		var bi, ci float64
		if i < len(b) {
			bi = b[i]
		}

		if i < len(c) {
			ci = c[i]
		}

		d := bi - ci
		ss += d * d
		normB += bi * bi
		normC += ci * ci
	}

	dist := math.Sqrt(ss)

	// Нормировка в [0,~1]: масштаб-инвариантность к размеру графа.
	denom := math.Sqrt(normB) + math.Sqrt(normC)
	if denom < 1e-12 {
		denom = 1
	}

	norm := roundTo(dist/denom, 1e9)

	return SpectralDistance{Distance: norm, Shifted: norm > spectralEpsilon}
}

// laplacianSpectrumOf — отсортированный спектр симметричного лапласиана undirected-
// проекции графа, округлённый до 1e-9 (детерминизм). nil при n<2 / сбое факторизации.
func laplacianSpectrumOf(g *model.Graph) []float64 {
	dg := buildDescriptorGraph(g)

	n := len(dg.nodes)
	if n < 2 {
		return nil
	}

	adj := undirectedAdj(dg)

	gg := simple.NewUndirectedGraph()
	for i := 0; i < n; i++ {
		gg.AddNode(simple.Node(i))
	}

	for from, tos := range adj {
		fi := dg.index[from]
		for to := range tos {
			ti := dg.index[to]
			if fi < ti { // ребро один раз, без петель
				gg.SetEdge(gg.NewEdge(simple.Node(fi), simple.Node(ti)))
			}
		}
	}

	lap := spectral.NewLaplacian(gg)

	sd := mat.NewSymDense(n, nil)
	for i := 0; i < n; i++ {
		for j := i; j < n; j++ {
			sd.SetSym(i, j, lap.At(i, j))
		}
	}

	var es mat.EigenSym
	if ok := es.Factorize(sd, false); !ok {
		return nil
	}

	values := es.Values(nil)
	sort.Float64s(values)

	for i, v := range values {
		if v < 0 {
			v = 0
		}

		values[i] = roundTo(v, 1e9) // детерминизм: гасим FP-шум
	}

	return values
}

// roundTo округляет x до 1/scale (напр. scale=1e9 -> 1e-9).
func roundTo(x, scale float64) float64 {
	return math.Round(x*scale) / scale
}
