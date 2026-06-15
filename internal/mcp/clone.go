package mcp

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mshogin/archlint/internal/analyzer"
	"github.com/mshogin/archlint/internal/model"
)

// KindStructuralClone — точная структурная копипаста: >=2 фрагмента (функции/методы) с
// ИЗОМОРФНОЙ канонической формой размера >= cloneMinSize. DRY-принцип (карта ROADMAP).
const KindStructuralClone = "structural-clone"

// cloneMinSize — порог РАЗМЕРА (число вызовов фрагмента) = отсечка ШУМА (тривиальные мелкие
// функции одинаковой формы — не дубль). Это НЕ магнитуда-θ (в отличие от near-clone%):
// паттерн «∃ изоморфная пара >= k узлов» КАЧЕСТВЕННЫЙ, k фиксирован как анти-шум, не настройка.
const cloneMinSize = 3

// StructuralClone ищет структурные клоны через КАНОНИЧЕСКИЙ FINGERPRINT формы фрагмента
// (Тир1: хеш-группировка O(n log n), НЕ полный graph-isomorphism — GI дорог). Конкретные
// ИМЕНА целей/типов АБСТРАГИРОВАНЫ -> fingerprint кодирует изоморфизм ФОРМЫ (число/виды вызовов,
// арность сигнатуры, профиль доступа к полям). Коллизия формы при разной СЕМАНТИКЕ = legal FP
// (precision<1 -> WARNING, не ERROR). Детерминирован: сортировка членов и компонент fingerprint.
func StructuralClone(a *analyzer.GoAnalyzer) []Violation {
	if a == nil {
		return nil
	}

	groups := make(map[string][]string) // fingerprint -> отсортированные qname

	add := func(qname, fp string, size int) {
		if size < cloneMinSize {
			return
		}

		groups[fp] = append(groups[fp], qname)
	}

	for id, f := range a.AllFunctions() {
		fp, size := cloneFingerprint(len(f.Params), len(f.Results), f.Calls, nil)
		add(id, fp, size)
	}

	for id, m := range a.AllMethods() {
		fp, size := cloneFingerprint(len(m.Params), len(m.Results), m.Calls, m.FieldAccess)
		add(id, fp, size)
	}

	var out []Violation

	for fp, members := range groups {
		if len(members) < 2 {
			continue
		}

		sort.Strings(members)

		for _, qn := range members {
			out = append(out, Violation{
				Kind: KindStructuralClone,
				Message: fmt.Sprintf(
					"structural clone: %s изоморфен ещё %d фрагмент(ам) формы [%s] — рассмотреть extract common",
					qn, len(members)-1, fp,
				),
				Target: qn,
			})
		}
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Target < out[j].Target })

	return out
}

// cloneFingerprint строит каноническую форму фрагмента, АБСТРАГИРУЯ конкретные имена
// (изоморфизм формы, не совпадение имён). Признаки: арность сигнатуры (params/results),
// мультимножество структурных видов вызовов (method/goroutine/deferred), профиль доступа
// к полям (read/write). size = число вызовов (мера размера подграфа для cloneMinSize).
func cloneFingerprint(
	numParams, numResults int,
	calls []model.CallInfo,
	fields []model.FieldAccessInfo,
) (fingerprint string, size int) {
	callSigs := make([]string, 0, len(calls))
	for _, c := range calls {
		callSigs = append(callSigs, fmt.Sprintf("%t-%t-%t", c.IsMethod, c.IsGoroutine, c.IsDeferred))
	}

	sort.Strings(callSigs)

	fieldSigs := make([]string, 0, len(fields))
	for _, fa := range fields {
		fieldSigs = append(fieldSigs, fmt.Sprintf("%t", fa.IsWrite))
	}

	sort.Strings(fieldSigs)

	fp := fmt.Sprintf(
		"p%d|r%d|c%d:%s|f%d:%s",
		numParams, numResults,
		len(calls), strings.Join(callSigs, ","),
		len(fields), strings.Join(fieldSigs, ","),
	)

	return fp, len(calls)
}
