package mcp

import (
	"fmt"
	"sort"

	"github.com/mshogin/archlint/internal/model"
)

// Горнило-кандидаты: articulation_points / bridge_edges / stability_violations.
// INTENT-LADEN: срабатывание НЕ всегда дефект (легитимный единый вход/фасад = articulation
// point; мост часто = здоровая модульность). DIP-РИСК.
//
// ★ВЕРДИКТ ГОРНИЛА (парный, соундность): ВСЕ ТРИ ДЕМОТИРОВАНЫ, НИ ОДИН не ERROR. На
// здоровом archlint 199 срабатываний на легитимной структуре (DIP-класс конфаунда).
// Поэтому НЕ в severity_class (никогда не блок), а СИГНАЛЫ-ДЕСКРИПТОРЫ под --signals:
//   - articulation_points -> WARNING-сигнал (фасад=articulation по определению; интент
//     синтаксически неотделим — класс DIP);
//   - bridge_edges -> INFO/WARNING loose-coupling дескриптор (мост чаще = здоровье);
//   - stability_violations -> INFO-дескриптор (чистая магнитуда SDP: I=ратио+порог, класс
//     instability/coupling), + регрессия-WARNING в дельте.
// Наблюдаемость (бутылочные горла + тренд связанности), не гейт.

// StabilityViolations — нарушения SDP Мартина (порт validate_stability_violations):
// стабильный компонент (низкая I) зависит от нестабильного (высокая I). I=Ce/(Ca+Ce).
// Нарушение: I(source) < I(target) - 0.2.
func StabilityViolations(graph *model.Graph) []Violation {
	dg := buildDescriptorGraph(graph)

	inst := make(map[string]float64, len(dg.nodes))
	for _, id := range dg.nodes {
		total := dg.inDeg[id] + dg.outDeg[id]
		if total > 0 {
			inst[id] = float64(dg.outDeg[id]) / float64(total)
		} else {
			inst[id] = 0.5
		}
	}

	// Дедуплицированные рёбра в детерминированном порядке.
	pairs := sortedEdgePairs(dg)

	var out []Violation

	for _, p := range pairs {
		si := inst[p[0]]

		ti, ok := inst[p[1]]
		if !ok {
			ti = 0.5
		}

		if si < ti-0.2 {
			out = append(out, Violation{
				Kind: "stability-violation",
				Message: fmt.Sprintf(
					"Stable-depends-on-unstable (SDP): %s (I=%.2f) -> %s (I=%.2f)",
					p[0], si, p[1], ti,
				),
				Target: p[0],
			})
		}
	}

	return out
}

// ArticulationPoints — узлы, удаление которых разбивает граф (nx.articulation_points
// на undirected-проекции, алгоритм Тарьяна).
func ArticulationPoints(graph *model.Graph) []Violation {
	dg := buildDescriptorGraph(graph)
	adj := undirectedAdj(dg)
	aps := articulationSet(dg.nodes, adj)

	out := make([]Violation, 0, len(aps))

	for _, id := range aps {
		out = append(out, Violation{
			Kind:    "articulation-point",
			Message: fmt.Sprintf("Articulation point (removal disconnects the graph): %s", id),
			Target:  id,
		})
	}

	return out
}

// BridgeEdges — рёбра, удаление которых разбивает граф (nx.bridges на undirected).
func BridgeEdges(graph *model.Graph) []Violation {
	dg := buildDescriptorGraph(graph)
	adj := undirectedAdj(dg)
	bridges := bridgeSet(dg.nodes, adj)

	out := make([]Violation, 0, len(bridges))

	for _, b := range bridges {
		out = append(out, Violation{
			Kind:    "bridge-edge",
			Message: fmt.Sprintf("Bridge edge (removal disconnects the graph): %s -- %s", b[0], b[1]),
			Target:  b[0],
		})
	}

	return out
}

// sortedEdgePairs — дедуп (from,to) в детерминированном порядке.
func sortedEdgePairs(dg *descriptorGraph) [][2]string {
	var pairs [][2]string

	for from, tos := range dg.outAdj {
		for to := range tos {
			pairs = append(pairs, [2]string{from, to})
		}
	}

	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i][0] != pairs[j][0] {
			return pairs[i][0] < pairs[j][0]
		}

		return pairs[i][1] < pairs[j][1]
	})

	return pairs
}

// articulationSet — точки сочленения undirected-графа (Тарьян, disc/low DFS).
func articulationSet(nodes []string, adj map[string]map[string]bool) []string {
	disc := map[string]int{}
	low := map[string]int{}
	isAP := map[string]bool{}
	timer := 0

	// Соседи в детерминированном порядке.
	neighbors := func(u string) []string {
		ns := make([]string, 0, len(adj[u]))
		for v := range adj[u] {
			ns = append(ns, v)
		}

		sort.Strings(ns)

		return ns
	}

	var dfs func(u, parent string)
	dfs = func(u, parent string) {
		disc[u] = timer
		low[u] = timer
		timer++
		children := 0

		for _, v := range neighbors(u) {
			if v == parent {
				continue
			}

			if _, seen := disc[v]; !seen {
				children++

				dfs(v, u)

				if low[v] < low[u] {
					low[u] = low[v]
				}

				if parent != "" && low[v] >= disc[u] {
					isAP[u] = true
				}
			} else if disc[v] < low[u] {
				low[u] = disc[v]
			}
		}

		if parent == "" && children > 1 {
			isAP[u] = true
		}
	}

	for _, n := range nodes {
		if _, seen := disc[n]; !seen {
			dfs(n, "")
		}
	}

	var out []string

	for _, n := range nodes {
		if isAP[n] {
			out = append(out, n)
		}
	}

	return out
}

// bridgeSet — мосты undirected-графа (Тарьян: ребро-дерева (u,v) с low[v] > disc[u]).
// Каждый мост возвращается как отсортированная пара.
func bridgeSet(nodes []string, adj map[string]map[string]bool) [][2]string {
	disc := map[string]int{}
	low := map[string]int{}
	timer := 0

	var bridges [][2]string

	neighbors := func(u string) []string {
		ns := make([]string, 0, len(adj[u]))
		for v := range adj[u] {
			ns = append(ns, v)
		}

		sort.Strings(ns)

		return ns
	}

	var dfs func(u, parent string)
	dfs = func(u, parent string) {
		disc[u] = timer
		low[u] = timer
		timer++

		for _, v := range neighbors(u) {
			if v == parent {
				continue
			}

			if _, seen := disc[v]; !seen {
				dfs(v, u)

				if low[v] < low[u] {
					low[u] = low[v]
				}

				if low[v] > disc[u] {
					a, b := u, v
					if b < a {
						a, b = b, a
					}

					bridges = append(bridges, [2]string{a, b})
				}
			} else if disc[v] < low[u] {
				low[u] = disc[v]
			}
		}
	}

	for _, n := range nodes {
		if _, seen := disc[n]; !seen {
			dfs(n, "")
		}
	}

	sort.Slice(bridges, func(i, j int) bool {
		if bridges[i][0] != bridges[j][0] {
			return bridges[i][0] < bridges[j][0]
		}

		return bridges[i][1] < bridges[j][1]
	})

	return bridges
}
