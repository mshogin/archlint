package mcp

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mshogin/archlint/internal/archlintcfg"
	"github.com/mshogin/archlint/internal/model"
)

// DeprecatedUsage — ERROR-детектор использования deprecated-компонентов (порт
// validate_deprecated_usage). Узел DEPRECATED, если: (а) его имя содержит явный
// настроенный паттерн (cfg.DeprecatedPatterns, case-insensitive), ИЛИ (б) у узла
// атрибут графа `deprecated` истинен. Любое РЕБРО в deprecated-узел от
// не-deprecated источника — нарушение (использование устаревшего = дефект).
//
// СОУНДНОСТЬ: маркер ЯВНЫЙ (паттерн в конфиге или атрибут) -> срабатывание⟹дефект
// (односторонняя импликация). ОТЛИЧИЕ ОТ Python: НЕ берём широкие дефолтные паттерны
// (legacy/old/v1 ложно-стреляют на здоровом коде); без явных маркеров детектор
// НЕАКТИВЕН (0 ложных, как forbidden). closed-world относительно объявленных маркеров.
func DeprecatedUsage(graph *model.Graph, cfg *archlintcfg.Config) []Violation {
	if graph == nil || cfg == nil {
		return nil
	}

	patterns := make([]string, 0, len(cfg.DeprecatedPatterns))
	for _, p := range cfg.DeprecatedPatterns {
		if p != "" {
			patterns = append(patterns, strings.ToLower(p))
		}
	}

	// Множество deprecated-узлов.
	deprecated := make(map[string]bool)

	for _, n := range graph.Nodes {
		if isDeprecatedNode(n, patterns) {
			deprecated[n.ID] = true
		}
	}

	if len(deprecated) == 0 {
		return nil // нет явных маркеров -> детектор неактивен
	}

	// Рёбра в deprecated-узлы от не-deprecated источников (дедуп (from,to)).
	seen := make(map[[2]string]bool)

	var pairs [][2]string

	for _, e := range graph.Edges {
		if !deprecated[e.To] || deprecated[e.From] {
			continue
		}

		key := [2]string{e.From, e.To}
		if seen[key] {
			continue
		}

		seen[key] = true

		pairs = append(pairs, key)
	}

	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i][0] != pairs[j][0] {
			return pairs[i][0] < pairs[j][0]
		}

		return pairs[i][1] < pairs[j][1]
	})

	out := make([]Violation, 0, len(pairs))
	for _, p := range pairs {
		out = append(out, Violation{
			Kind:    "deprecated-usage",
			Message: fmt.Sprintf("Use of deprecated component: %s -> %s", p[0], p[1]),
			Target:  p[0],
		})
	}

	return out
}

// isDeprecatedNode — узел помечен deprecated по имени-паттерну или атрибуту `deprecated`.
func isDeprecatedNode(n model.Node, patterns []string) bool {
	low := strings.ToLower(n.ID)
	for _, p := range patterns {
		if strings.Contains(low, p) {
			return true
		}
	}

	if n.Attrs != nil {
		if v, ok := n.Attrs["deprecated"]; ok && truthy(v) {
			return true
		}
	}

	return false
}

// truthy — истинность атрибута (bool true / непустая строка кроме "false" / ненулевое число).
func truthy(v any) bool {
	switch x := v.(type) {
	case bool:
		return x
	case string:
		return x != "" && strings.ToLower(x) != "false"
	case int:
		return x != 0
	case float64:
		return x != 0
	}

	return v != nil
}
