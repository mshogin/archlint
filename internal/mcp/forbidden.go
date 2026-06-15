package mcp

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mshogin/archlint/internal/archlintcfg"
	"github.com/mshogin/archlint/internal/model"
)

// ForbiddenDependencies — ERROR-детектор запрещённых зависимостей (порт Python
// validate_forbidden_dependencies). Ребро (source,target), у которого source
// содержит rule.From И target содержит rule.To (case-insensitive подстрока), —
// нарушение объявленного запрета.
//
// СОУНДНОСТЬ: запрещённое объявлено пользователем в .archlint -> ПАТТЕРН ПО
// ОПРЕДЕЛЕНИЮ (односторонняя импликация срабатывание⟹дефект, порог не нужен).
// closed-world относительно конфига. Без правил (Forbidden пуст) -> детектор
// НЕАКТИВЕН (0 нарушений). Параллельные (source,target) схлопнуты (как nx.DiGraph).
func ForbiddenDependencies(graph *model.Graph, cfg *archlintcfg.Config) []Violation {
	if graph == nil || cfg == nil || len(cfg.Forbidden) == 0 {
		return nil
	}

	// Дедуп рёбер по (from,to) в детерминированном порядке.
	seen := make(map[[2]string]bool)

	edges := make([][2]string, 0, len(graph.Edges))

	for _, e := range graph.Edges {
		key := [2]string{e.From, e.To}
		if seen[key] {
			continue
		}

		seen[key] = true

		edges = append(edges, key)
	}

	sort.Slice(edges, func(i, j int) bool {
		if edges[i][0] != edges[j][0] {
			return edges[i][0] < edges[j][0]
		}

		return edges[i][1] < edges[j][1]
	})

	var out []Violation

	for _, e := range edges {
		src, dst := e[0], e[1]
		srcLow := strings.ToLower(src)
		dstLow := strings.ToLower(dst)

		for _, rule := range cfg.Forbidden {
			from := strings.ToLower(rule.From)
			to := strings.ToLower(rule.To)

			if from == "" || to == "" {
				continue
			}

			if strings.Contains(srcLow, from) && strings.Contains(dstLow, to) {
				out = append(out, Violation{
					Kind: "forbidden-dependency",
					Message: fmt.Sprintf(
						"Forbidden dependency: %s -> %s (rule: %s -> %s)",
						src, dst, rule.From, rule.To,
					),
					Target: src,
					Anchor: "forbidden:" + src + "->" + dst, // структурный якорь = пара src->dst
				})
			}
		}
	}

	return out
}
