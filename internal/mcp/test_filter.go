package mcp

import (
	"strings"

	"github.com/mshogin/archlint/internal/analyzer"
)

// DropTestTargets убирает находки, чей Target объявлен в _test.go (тест-функции, заглушки,
// сьюты) или принадлежит внешнему тест-пакету (<pkg>_test). Тесты по-прежнему участвуют в
// графе (test-reachability для dead-code, test-only-prod-symbol): фильтр работает только на
// выходе, по Target. Находки на прод-символах, пакетах и без Target (cycles по Anchor) остаются.
// a == nil (stdin/TS/Rust): файла символа не знаем -> убираем только внешние тест-пакеты.
func DropTestTargets(a *analyzer.GoAnalyzer, violations []Violation) []Violation {
	var idx map[string]string
	if a != nil {
		idx = buildLocationIndex(a)
	}

	kept := violations[:0:0]
	for _, v := range violations {
		if isTestTarget(idx, v.Target) {
			continue
		}
		kept = append(kept, v)
	}

	return kept
}

func isTestTarget(idx map[string]string, target string) bool {
	if target == "" {
		return false
	}

	pkg := target
	if i := strings.IndexByte(pkg, '.'); i >= 0 {
		pkg = pkg[:i]
	}
	if strings.HasSuffix(pkg, "_test") {
		return true
	}

	loc := resolveLocation(idx, target)
	if i := strings.LastIndexByte(loc, ':'); i >= 0 {
		loc = loc[:i]
	}

	return strings.HasSuffix(loc, "_test.go")
}
