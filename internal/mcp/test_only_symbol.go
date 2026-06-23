package mcp

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mshogin/archlint/internal/analyzer"
	"github.com/mshogin/archlint/internal/model"
)

// test_only_symbol.go — детектор test-only-prod-symbol (под-класс dead-code-семьи).
//
// СИМВОЛ (func/type/method), ОБЪЯВЛЕННЫЙ в prod-файле (НЕ _test.go), у которого
// ВСЕ входящие reference/call рёбра идут ТОЛЬКО из _test.go-файлов (ноль
// использований из prod-кода) = quasi-dead в проде (живёт лишь ради тестов, но
// лежит в prod). 2D-критерий доказуемости: граница test/prod = суффикс _test.go
// (ИЗМ.1 — дискретный языковой контракт Go), reference-граф точен (ИЗМ.2).
//
// СОУНДНОСТЬ (расщепление по видимости, как open/closed-world dead-code):
//   - UNEXPORTED символ used-only-from-_test -> CLOSED-WORLD: не виден вне пакета,
//     в пакете юзают только тесты => quasi-dead. Kind "test-only-prod-symbol":
//     ERROR-кандидат В ДЕЛЬТЕ (RequiresDelta) + HumanInLoop (тест может быть
//     временно единственным легальным юзером — человек решает).
//   - EXPORTED -> OPEN-WORLD (может юзаться внешними репо, не видно): НЕ ERROR.
//     Kind "test-only-prod-symbol-exported": WARNING, не блок.
//
// Защита от false-fire (как dead-code): требуем НЕПУСТОЙ набор входящих (символ
// без юзеров вообще = dead-code, не наш под-класс) И источник file из analyzer
// (без file -> пропуск).

const (
	kindTestOnlyProdSymbol         = "test-only-prod-symbol"          // unexported -> ERROR
	kindTestOnlyProdSymbolExported = "test-only-prod-symbol-exported" // exported  -> WARNING
)

// TestOnlyProdSymbol находит prod-символы, используемые ТОЛЬКО из _test.go.
func TestOnlyProdSymbol(g *model.Graph, a *analyzer.GoAnalyzer) []Violation {
	if g == nil || a == nil {
		return nil
	}

	// fileOf: qname -> файл объявления (для границы prod/_test.go).
	fileOf := make(map[string]string)
	for id, f := range a.AllFunctions() {
		fileOf[id] = f.File
	}
	for id, m := range a.AllMethods() {
		fileOf[id] = m.File
	}
	for id, t := range a.AllTypes() {
		fileOf[id] = t.File
	}

	// incoming: To -> [From] по рёбрам использования (calls/references/uses/returns).
	incoming := make(map[string][]string)
	for _, e := range g.Edges {
		switch e.Type {
		case model.EdgeCalls, model.EdgeReferences, model.EdgeUses, model.EdgeReturns:
			incoming[e.To] = append(incoming[e.To], e.From)
		}
	}

	var out []Violation
	for _, n := range g.Nodes {
		if n.Entity != "function" && n.Entity != "method" && n.Entity != "type" {
			continue
		}
		decl, ok := fileOf[n.ID]
		if !ok || isTestFile(decl) {
			continue // объявлен НЕ в prod-файле (или file неизвестен) — не наш случай
		}

		froms := incoming[n.ID]
		if len(froms) == 0 {
			continue // ноль использований = dead-code (другой детектор), не test-only
		}

		// Все ли входящие — из _test.go? (хоть один prod-юзер => символ живой в проде)
		allFromTest := true
		for _, from := range froms {
			ff, ok := fileOf[from]
			if !ok || !isTestFile(ff) {
				allFromTest = false
				break
			}
		}
		if !allFromTest {
			continue
		}

		kind := kindTestOnlyProdSymbol
		if isExportedSymbol(n.ID) {
			kind = kindTestOnlyProdSymbolExported
		}
		out = append(out, Violation{
			Kind:    kind,
			Message: fmt.Sprintf("prod-символ %s используется только из _test.go (quasi-dead в проде)", n.ID),
			Target:  n.ID,
			Anchor:  "test-only:" + n.ID,
		})
	}

	sort.Slice(out, func(i, j int) bool { return out[i].Target < out[j].Target })
	return out
}

// isTestFile — граница prod/тест: дискретный языковой контракт Go (_test.go).
func isTestFile(path string) bool {
	return strings.HasSuffix(path, "_test.go")
}

// isExportedSymbol — символ exported (видимость для расщепления соундности).
// qname вида "pkg/path.Symbol" или "pkg.Type.Method": exported, если ПОСЛЕДНИЙ
// сегмент имени начинается с заглавной (Go-контракт видимости).
func isExportedSymbol(qname string) bool {
	name := qname
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		name = name[i+1:]
	}
	if name == "" {
		return false
	}
	r := rune(name[0])
	return r >= 'A' && r <= 'Z'
}
