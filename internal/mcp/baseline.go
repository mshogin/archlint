package mcp

import "sort"

// Baseline — снимок ERROR-class паттерн-фактов кода (Фаза 5, дельта-инфраструктура,
// DR-0034). Дельта-гейт сравнивает ТЕКУЩИЕ паттерны с этим снимком: появившийся
// (NEW) ERROR-паттерн = регрессия -> блок; уже бывший в baseline -> аудит (telemetry).
//
// Назначение — соундность-СОХРАНЯЮЩАЯ активация уже-соундных детекторов (SCC,
// dead-code, layer-backedge) в боевом гейте: на легаси с N давними нарушениями
// абсолютный режим бесполезен (всё красное), дельта блокирует только НОВОЕ (DR-0007).
//
// ★ДЕТЕРМИНИЗМ (DR-0034): два снимка одного кода обязаны быть БАЙТ-идентичны.
// Поэтому Patterns сериализуется как map[Kind][]fingerprint, где каждый список
// ОТСОРТИРОВАН и дедуплицирован, а encoding/json маршалит ключи map в
// лексикографическом порядке. Никакой зависимости от порядка обхода map.
type Baseline struct {
	Version int `json:"version"`
	// Patterns: Kind нарушения -> отсортированный уникальный набор fingerprint'ов.
	Patterns map[string][]string `json:"patterns"`
}

// Fingerprint — СТРОГАЯ идентичность экземпляра паттерна для дельты (DR-0034 п.3).
// НЕ fuzzy / НЕ rename-tracking (Ось-1 запрещает магнитудный матч на гейте);
// переименование -> ложный-NEW -> irritation (приемлемо, fail-safe в безопасную
// сторону, НЕ чиним). Ключ строится из ДЕТЕРМИНИРОВАННЫХ полей Violation:
//   - circular-dependency: отсортированное множество member-qname SCC (несётся
//     в Message детерминированно: detectCycles сортирует членов). Это коллапсирует
//     P per-package дубликатов одного цикла в ОДНУ идентичность (идентичность SCC =
//     множество членов, а не отдельный пакет).
//   - layer-violation: пара (From -> To) — несётся в Message детерминированно.
//   - прочие (dead-code и др.): строгий qname-key = Target.
func Fingerprint(v Violation) string {
	switch v.Kind {
	case "circular-dependency", "layer-violation", "forbidden-dependency":
		// Message построен из отсортированных/стабильных полей -> детерминирован
		// и кодирует строгую структурную идентичность (член-множество / пару).
		return v.Message
	default:
		return v.Target
	}
}

// errorClass сообщает, относится ли Kind к ERROR-классу (реестр severity_class).
// Только ERROR-class паттерны участвуют в дельта-гейте; WARNING/INFO (магнитуды,
// DIP/SRP/coupling) НИКОГДА не блокируют (Ось-1, DR-0009).
func errorClass(kind string) bool {
	c, ok := ClassOf(kind)
	return ok && c.Class == "ERROR"
}

// BuildBaseline собирает снимок из ERROR-class нарушений. Не-ERROR игнорируются
// (дельта-гейт оперирует только блокирующими паттернами). Результат детерминирован:
// каждый список отсортирован и дедуплицирован.
func BuildBaseline(violations []Violation) *Baseline {
	b := &Baseline{Version: 1, Patterns: make(map[string][]string)}
	seen := make(map[string]map[string]bool)

	for _, v := range violations {
		if !errorClass(v.Kind) {
			continue
		}
		fp := Fingerprint(v)
		if seen[v.Kind] == nil {
			seen[v.Kind] = make(map[string]bool)
		}
		if seen[v.Kind][fp] {
			continue
		}
		seen[v.Kind][fp] = true
		b.Patterns[v.Kind] = append(b.Patterns[v.Kind], fp)
	}

	for k := range b.Patterns {
		sort.Strings(b.Patterns[k])
	}

	return b
}

// Contains сообщает, присутствовал ли паттерн v в baseline (по строгому fingerprint).
// nil-baseline -> false (никакой паттерн не "существующий" -> на гейте обрабатывается
// как audit-fallback в EffectiveLevel, НЕ как блок).
func (b *Baseline) Contains(v Violation) bool {
	if b == nil {
		return false
	}
	fp := Fingerprint(v)
	for _, x := range b.Patterns[v.Kind] {
		if x == fp {
			return true
		}
	}
	return false
}
