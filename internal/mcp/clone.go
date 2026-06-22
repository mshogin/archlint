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
//
// =5 (поднят с 3): фрагменты в 3-4 вызова — тривиальные мелкие формы (короткие хелперы/
// getter-цепочки одинакового профиля), массовый FP без архитектурной ценности (на self c3/c4
// = 128 ложных «клонов» из 478). Право качественного паттерна сохраняется: это сдвиг отсечки
// ШУМА, не порог величины. W2-проверка держится для k=5 (больной эталон с фрагментами >=5 fire).
const cloneMinSize = 5

// StructuralClone ищет структурные клоны через КАНОНИЧЕСКИЙ FINGERPRINT формы фрагмента
// (Тир1: хеш-группировка O(n log n), НЕ полный graph-isomorphism — GI дорог). Конкретные
// ИМЕНА целей/типов АБСТРАГИРОВАНЫ -> fingerprint кодирует изоморфизм ФОРМЫ (число/виды вызовов,
// арность сигнатуры, профиль доступа к полям). Коллизия формы при разной СЕМАНТИКЕ = legal FP
// (precision<1 -> WARNING, не ERROR). Детерминирован: сортировка членов и компонент fingerprint.
func StructuralClone(a *analyzer.GoAnalyzer) []Violation {
	if a == nil {
		return nil
	}

	groups := make(map[string][]string) // fingerprint -> qname
	sizeOf := make(map[string]int)      // fingerprint -> размер фрагмента (число вызовов)

	add := func(qname, fp string, size int) {
		if size < cloneMinSize {
			return
		}

		groups[fp] = append(groups[fp], qname)
		sizeOf[fp] = size
	}

	for id, f := range a.AllFunctions() {
		fp, size := cloneFingerprint(len(f.Params), len(f.Results), f.Calls, nil)
		add(id, fp, size)
	}

	for id, m := range a.AllMethods() {
		fp, size := cloneFingerprint(len(m.Params), len(m.Results), m.Calls, m.FieldAccess)
		add(id, fp, size)
	}

	// Группы клонов, РАНЖИРОВАННЫЕ по значимости: крупнейшие формы первыми (UX — самые
	// весомые клоны вверху вывода/PR-коммента), тай-брейк по fp для детерминизма.
	type cloneGroup struct {
		fp      string
		members []string
		size    int
	}

	var gs []cloneGroup

	for fp, members := range groups {
		if len(members) < 2 {
			continue
		}

		sort.Strings(members)
		gs = append(gs, cloneGroup{fp: fp, members: members, size: sizeOf[fp]})
	}

	sort.Slice(gs, func(i, j int) bool {
		if gs[i].size != gs[j].size {
			return gs[i].size > gs[j].size
		}

		return gs[i].fp < gs[j].fp
	})

	var out []Violation

	for _, g := range gs {
		for _, qn := range g.members {
			out = append(out, Violation{
				Kind: KindStructuralClone,
				Message: fmt.Sprintf(
					"structural clone: %s изоморфен ещё %d фрагмент(ам) формы [%s] (размер %d) — рассмотреть extract common",
					qn, len(g.members)-1, g.fp, g.size,
				),
				Target: qn,
			})
		}
	}

	return out
}

// cloneFingerprint строит каноническую форму фрагмента, АБСТРАГИРУЯ конкретные имена
// (изоморфизм формы, не совпадение имён), но РАЗЛИЧАЯ структуру потока вызовов.
//
// Усиление precision (меньше ложных склеек разных функций одной грубой формы):
//   - КАНОНИЧЕСКАЯ НУМЕРАЦИЯ RECEIVER'ов: имена переменных-приёмников -> позиционные индексы
//     по первому появлению (a.x();a.y();b.z() -> R0,R0,R1). Сохраняет изоморфизм (переименование
//     var a->foo НЕ меняет форму -> копипаста с переименованием всё ещё ловится), но различает
//     СТРУКТУРУ: вызовы на одном объекте vs на разных -> две функции с разным receiver-паттерном
//     не склеиваются. Это canonical labeling (стандарт для изоморфизма графов).
//   - ПОРЯДОК вызовов (последовательность, НЕ мультисет): точная копипаста сохраняет порядок;
//     разный порядок одинаковых видов -> разные формы. Устойчив к переименованию (имён в seq нет).
//   - арность сигнатуры (params/results), число уникальных receiver'ов, профиль доступа к полям.
//
// size = число вызовов (мера размера подграфа для cloneMinSize).
func cloneFingerprint(
	numParams, numResults int,
	calls []model.CallInfo,
	fields []model.FieldAccessInfo,
) (fingerprint string, size int) {
	// Каноническая нумерация receiver-переменных (имена -> позиционные индексы).
	recvIdx := make(map[string]int)
	nextRecv := 0

	canonRecv := func(r string) int {
		if r == "" {
			return -1 // не-method / свободный вызов
		}

		if i, ok := recvIdx[r]; ok {
			return i
		}

		recvIdx[r] = nextRecv
		nextRecv++

		return recvIdx[r]
	}

	// Каноническая нумерация ЦЕЛЕЙ вызова (Target -> позиционный индекс по первому появлению).
	// Различает СТРУКТУРУ повторов вызовов (дважды одна функция vs две разные), сохраняя
	// rename-robustness (конкретное имя цели не хранится — только позиция). Ключевой различитель
	// для СВОБОДНЫХ вызовов (receiver="" -> R-1 у всех): раньше N свободных вызовов = N×«-1»
	// и любые две такие функции склеивались; теперь структура повторов целей их разводит.
	targetIdx := make(map[string]int)
	nextTarget := 0

	canonTarget := func(tg string) int {
		if i, ok := targetIdx[tg]; ok {
			return i
		}

		targetIdx[tg] = nextTarget
		nextTarget++

		return targetIdx[tg]
	}

	// Последовательность видов вызовов В ПОРЯДКЕ тела (порядок различает; имён нет -> устойчиво
	// к переименованию). НЕ сортируется — порядок есть структурный признак точной копипасты.
	callSeq := make([]string, 0, len(calls))
	for _, c := range calls {
		callSeq = append(callSeq, fmt.Sprintf("%d:%d/%t/%t/%t", canonRecv(c.Receiver), canonTarget(c.Target), c.IsMethod, c.IsGoroutine, c.IsDeferred))
	}

	fieldSigs := make([]string, 0, len(fields))
	for _, fa := range fields {
		fieldSigs = append(fieldSigs, fmt.Sprintf("%t", fa.IsWrite))
	}

	sort.Strings(fieldSigs)

	fp := fmt.Sprintf(
		"p%d|r%d|c%d:%s|R%d|T%d|f%d:%s",
		numParams, numResults,
		len(calls), strings.Join(callSeq, ","),
		nextRecv, nextTarget,
		len(fields), strings.Join(fieldSigs, ","),
	)

	return fp, len(calls)
}
