package mcp

import (
	"strings"
	"unicode"

	"github.com/mshogin/archlint/internal/model"
)

// EntryPoints строит множество R — корни достижимости для dead-code (Фаза 3).
// R = АВТО-ДЕФОЛТ (детерминированный, НЕ эвристика) ∪ конфиг-паттерны.
//
// Дефолт ЩЕДРЫЙ по асимметрии цены (критерий 3 соундности): пропущенный entry ->
// функция без пути от R -> ложно-мёртвая -> удалили живое (destruction). Лишний
// entry -> ложно-живой (дёшево). Поэтому скупой R опаснее — берём щедро:
//   - функция main / init (рантайм-входы бинарей и пакетов);
//   - функция Test*/Benchmark*/Example* (тест ссылается на код -> код живой);
//   - ЛЮБОЙ exported символ (func/method/type): публичный API = entry по
//     определению (детерминированно по экспортируемости имени).
//
// configPatterns (.archlint entrypoints) ДОБАВЛЯЮТ узлы по подстроке ID — для
// того, что авто-дефолт не видит: framework-хендлеры, рефлексия/DI/codegen.
func EntryPoints(g *model.Graph, configPatterns []string) map[string]bool {
	r := make(map[string]bool)

	for _, n := range g.Nodes {
		if isDefaultEntry(n) {
			r[n.ID] = true
		}
	}

	for _, n := range g.Nodes {
		for _, p := range configPatterns {
			if p != "" && strings.Contains(n.ID, p) {
				r[n.ID] = true

				break
			}
		}
	}

	return r
}

// isDefaultEntry — авто-дефолтный entry: main/init/Test*/Benchmark*/Example* или
// любой exported func/method/type. Детерминированно по Entity + имени (Title).
func isDefaultEntry(n model.Node) bool {
	switch n.Entity {
	case "function":
		switch {
		case n.Title == "main" || n.Title == "init":
			return true
		case strings.HasPrefix(n.Title, "Test"),
			strings.HasPrefix(n.Title, "Benchmark"),
			strings.HasPrefix(n.Title, "Example"):
			return true
		default:
			return isExported(n.Title)
		}
	case "method", "struct", "interface":
		return isExported(n.Title)
	}

	return false
}

// isExported — Go-правило: имя экспортируемо, если начинается с заглавной.
func isExported(name string) bool {
	if name == "" {
		return false
	}

	return unicode.IsUpper([]rune(name)[0])
}
