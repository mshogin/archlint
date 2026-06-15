package mcp

import (
	"fmt"
	"sort"

	"github.com/mshogin/archlint/internal/archlintcfg"
	"github.com/mshogin/archlint/internal/model"
)

// LayerBackedge — ERROR-детектор обратных рёбер против ПОРЯДКА слоёв (Уровень B).
// Ранг слоя = ПОЗИЦИЯ в списке cfg.Layers (single source of truth; список читается
// сверху-вниз: верхние слои -> нижние). Разрешённое направление зависимостей — вниз
// (верхний слой зависит от нижнего, fromRank < toRank). BACK-EDGE — ребро из нижнего
// слоя в верхний (fromRank > toRank), против объявленного порядка, — нарушение.
//
// СОУНДНОСТЬ: относительно ОБЪЯВЛЕННОГО порядка слоёв (closed-world на конфиге L);
// обратное ребро = паттерн по определению. CONDITIONAL: неактивен без layers-конфига.
func LayerBackedge(graph *model.Graph, cfg *archlintcfg.Config) []Violation {
	if graph == nil || cfg == nil || len(cfg.Layers) == 0 {
		return nil
	}

	rank := make(map[string]int, len(cfg.Layers))
	for i, l := range cfg.Layers {
		rank[l.Name] = i
	}

	seen := make(map[[2]string]bool)

	var pairs [][2]string

	for _, e := range graph.Edges {
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

	var out []Violation

	for _, p := range pairs {
		fromLayer := cfg.LayerForModule(p[0])
		toLayer := cfg.LayerForModule(p[1])

		if fromLayer == "" || toLayer == "" || fromLayer == toLayer {
			continue
		}

		fr, okF := rank[fromLayer]
		tr, okT := rank[toLayer]

		if !okF || !okT {
			continue
		}

		// back-edge: нижний слой (больший ранг) зависит от верхнего (меньший ранг).
		if fr > tr {
			out = append(out, Violation{
				Kind: "layer-backedge",
				Message: fmt.Sprintf(
					"Layer back-edge against declared order: %s (%s #%d) -> %s (%s #%d)",
					p[0], fromLayer, fr, p[1], toLayer, tr,
				),
				Target: p[0],
				Anchor: "backedge:" + p[0] + "->" + p[1], // структурный якорь = пара from->to (без рангов/слоёв)
			})
		}
	}

	return out
}
