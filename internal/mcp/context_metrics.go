package mcp

import (
	"sort"

	"github.com/mshogin/archlint/internal/archlintcfg"
)

// ContextSignals — INFO-дескрипторы объявленных контекстов (порт behavior
// context_complexity/coupling/depth). СИГНАЛЫ/наблюдаемость под --signals, НЕ гейт
// (магнитуды, произвольный порог). Неактивны (nil) без объявленных контекстов.
//
// Соответствие Python: complexity = число компонентов в контексте; depth = то же
// (Python оценивает глубину как len(components)); coupling = доля общих компонентов
// между парой контекстов = |∩| / min(|A|,|B|).
type ContextSignals struct {
	PerContext    map[string]int `json:"perContext"`    // имя контекста -> число компонентов
	MaxComplexity int            `json:"maxComplexity"` // макс. число компонентов в контексте
	MaxDepth      int            `json:"maxDepth"`      // = maxComplexity (оценка Python)
	MaxCoupling   float64        `json:"maxCoupling"`   // макс. shared_ratio по парам контекстов
	// SinglePointsOfFailure — компоненты, присутствующие во ВСЕХ контекстах (вездесущая
	// зависимость = single point of failure). Вердикт горнила: WARNING-сигнал,
	// НЕ ERROR (вариант articulation, тот же DIP-класс конфаунда). Только при >=2 контекстах.
	SinglePointsOfFailure []string `json:"singlePointsOfFailure"`
	// NearSPOFCount — компоненты в >=80% контекстов (но не во всех).
	NearSPOFCount int `json:"nearSPOFCount"`
	// Coverage — покрытие топ-N PageRank-критических узлов контекстами. Вердикт горнила:
	// WARNING-сигнал, НЕ ERROR (intent-laden, data-конфаунд класса DIP). nil без графа.
	Coverage *CoverageResult `json:"coverage,omitempty"`
}

// ComputeContextSignals считает INFO-дескрипторы контекстов из конфига.
// nil -> контексты не объявлены (метрики неактивны).
func ComputeContextSignals(cfg *archlintcfg.Config) *ContextSignals {
	if cfg == nil || len(cfg.Contexts) == 0 {
		return nil
	}

	cs := &ContextSignals{PerContext: make(map[string]int, len(cfg.Contexts))}

	// Множества компонентов по контекстам (дедуп внутри контекста).
	sets := make([]map[string]bool, 0, len(cfg.Contexts))

	for _, ctx := range cfg.Contexts {
		set := make(map[string]bool, len(ctx.Components))
		for _, comp := range ctx.Components {
			if comp != "" {
				set[comp] = true
			}
		}

		cs.PerContext[ctx.Name] = len(set)
		sets = append(sets, set)

		if len(set) > cs.MaxComplexity {
			cs.MaxComplexity = len(set)
		}
	}

	cs.MaxDepth = cs.MaxComplexity // Python: estimated_depth = len(components)

	// Coupling: максимальная доля общих компонентов по всем парам контекстов.
	for i := 0; i < len(sets); i++ {
		for j := i + 1; j < len(sets); j++ {
			ratio := sharedRatio(sets[i], sets[j])
			if ratio > cs.MaxCoupling {
				cs.MaxCoupling = ratio
			}
		}
	}

	cs.SinglePointsOfFailure, cs.NearSPOFCount = singlePointsOfFailure(sets)

	return cs
}

// singlePointsOfFailure — компоненты во ВСЕХ контекстах (SPOF) + число near-SPOF
// (>=80% контекстов, но не все). Требует >=2 контекстов (иначе тривиально все = SPOF).
func singlePointsOfFailure(sets []map[string]bool) (spof []string, nearCount int) {
	total := len(sets)
	if total < 2 {
		return nil, 0
	}

	count := make(map[string]int)
	for _, set := range sets {
		for comp := range set {
			count[comp]++
		}
	}

	threshold := 0.8 * float64(total)

	for comp, n := range count {
		switch {
		case n == total:
			spof = append(spof, comp)
		case float64(n) >= threshold:
			nearCount++
		}
	}

	sort.Strings(spof)

	return spof, nearCount
}

// sharedRatio = |A ∩ B| / min(|A|,|B|); 0 если любой пуст.
func sharedRatio(a, b map[string]bool) float64 {
	minSize := len(a)
	if len(b) < minSize {
		minSize = len(b)
	}

	if minSize == 0 {
		return 0
	}

	shared := 0

	// Итерируем меньшее множество (детерминизм не важен для счётчика).
	small, large := a, b
	if len(b) < len(a) {
		small, large = b, a
	}

	keys := make([]string, 0, len(small))
	for k := range small {
		keys = append(keys, k)
	}

	sort.Strings(keys)

	for _, k := range keys {
		if large[k] {
			shared++
		}
	}

	return float64(shared) / float64(minSize)
}
