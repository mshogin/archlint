package mcp

// DeltaResult — разбиение текущих нарушений относительно baseline (Фаза 5).
type DeltaResult struct {
	New      []Violation // отсутствуют в baseline (регрессия для ERROR-class)
	Existing []Violation // присутствуют в baseline (давние)
}

// Delta классифицирует текущие нарушения относительно baseline (generic по Kind).
// Чистая классификация: NEW = не в baseline, Existing = в baseline. Решение о
// блокировке принимает гейт (EffectiveLevel), не Delta. nil-baseline -> всё в New
// (но гейт трактует nil как audit-fallback, см. EffectiveLevel).
func Delta(current []Violation, baseline *Baseline) DeltaResult {
	var r DeltaResult
	for _, v := range current {
		if baseline.Contains(v) {
			r.Existing = append(r.Existing, v)
		} else {
			r.New = append(r.New, v)
		}
	}
	return r
}
