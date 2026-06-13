package mcp

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mshogin/archlint/internal/archlintcfg"
	"github.com/mshogin/archlint/internal/model"
	"gonum.org/v1/gonum/graph/network"
)

// GhostComponents — ERROR-детектор «призрачных» компонентов: компонент, объявленный
// в контексте (.archlint contexts), которого НЕТ в архитектурном графе = устаревшая
// декларация (порт behavior ghost_components).
//
// СОУНДНОСТЬ: ссылка на несуществующий компонент = дефект по определению
// (односторонняя импликация: ghost ⟹ устаревшая/опечатанная декларация). closed-world
// относительно объявленных контекстов. CONDITIONAL: неактивен без cfg.Contexts (self=0).
//
// Матчинг компонент<->узел — fuzzy (как Python _find_matching_node): нормализация =
// последний сегмент после '/', + двунаправленный substring. Консервативно: чем легче
// матч, тем МЕНЬШЕ ложных ghost (безопасная сторона для ERROR).
func GhostComponents(graph *model.Graph, cfg *archlintcfg.Config) []Violation {
	if graph == nil || cfg == nil || !cfg.HasContexts() {
		return nil
	}

	nodes := make([]string, 0, len(graph.Nodes))

	seen := make(map[string]bool)
	for _, n := range graph.Nodes {
		if !seen[n.ID] {
			seen[n.ID] = true

			nodes = append(nodes, n.ID)
		}
	}

	sort.Strings(nodes)

	comps := make([]string, 0)
	for c := range cfg.ContextComponents() {
		comps = append(comps, c)
	}

	sort.Strings(comps)

	var out []Violation

	for _, comp := range comps {
		if findMatchingNode(comp, nodes) == "" {
			out = append(out, Violation{
				Kind:    "ghost-component",
				Message: fmt.Sprintf("Ghost component: %q declared in a context but absent from the graph", comp),
				Target:  comp,
			})
		}
	}

	return out
}

// defaultLayerPatterns — хардкод имя-паттерны слоёв (порт validate_layer_traversal).
// Порядок = приоритет матча (первый substring выигрывает), как Python dict insertion.
var defaultLayerPatterns = []struct {
	pat   string
	layer int
}{
	{"cmd", 0}, {"api", 1}, {"handler", 1}, {"controller", 1},
	{"service", 2}, {"usecase", 2},
	{"domain", 3}, {"entity", 3}, {"model", 3},
	{"repository", 4}, {"storage", 4},
	{"infrastructure", 5}, {"pkg", 6},
}

func layerOf(comp string) (int, bool) {
	low := strings.ToLower(comp)
	for _, p := range defaultLayerPatterns {
		if strings.Contains(low, p.pat) {
			return p.layer, true
		}
	}

	return 0, false
}

// LayerTraversalResult — пара компонентов в контексте, нарушающая порядок слоёв
// (next_layer < curr_layer = «вызов вверх по слоям»).
type LayerTraversalResult struct {
	Context   string
	From      string
	To        string
	FromLayer int
	ToLayer   int
}

// ComputeLayerTraversal — порт validate_layer_traversal: для каждого контекста
// анализирует ПОСЛЕДОВАТЕЛЬНОСТЬ его компонентов; consecutive пара (i,i+1) с обоими
// слоями и next_layer<curr_layer = подъём вверх. nil без contexts.
//
// ★INTENT-LADEN (не в severity_class до горнила): опирается на (а) ПОРЯДОК компонентов
// в context-списке как «execution flow» (декларация != поток), (б) хардкод имя-паттерны
// слоёв. Слабее соундного layer-backedge (тот — на ОБЪЯВЛЕННОМ порядке + реальных рёбрах).
func ComputeLayerTraversal(cfg *archlintcfg.Config) []LayerTraversalResult {
	if cfg == nil || !cfg.HasContexts() {
		return nil
	}

	ctxs := append([]archlintcfg.ContextDef(nil), cfg.Contexts...)
	sort.Slice(ctxs, func(i, j int) bool { return ctxs[i].Name < ctxs[j].Name })

	var out []LayerTraversalResult

	for _, ctx := range ctxs {
		comps := ctx.Components
		for i := 0; i+1 < len(comps); i++ {
			cl, okC := layerOf(comps[i])
			nl, okN := layerOf(comps[i+1])

			if okC && okN && nl < cl {
				out = append(out, LayerTraversalResult{
					Context: ctx.Name, From: comps[i], To: comps[i+1], FromLayer: cl, ToLayer: nl,
				})
			}
		}
	}

	return out
}

// CoverageResult — результат context_coverage: покрытие топ-N PageRank-критических
// узлов объявленными контекстами.
type CoverageResult struct {
	Active    bool     // false без contexts / <2 узлов
	Coverage  float64  // covered / critical
	Critical  []string // топ-N узлов по PageRank
	Covered   []string // критические, покрытые контекстом
	Uncovered []string // критические, НЕ покрытые
}

// ComputeContextCoverage — порт validate_context_coverage. Критические = топ-N
// (по умолчанию 10) узлов по PageRank; покрытый = существует context-компонент,
// fuzzy-матчащий узел. coverage = covered/critical.
//
// ★INTENT-LADEN (не зарегистрирован в severity_class до горнила): «критический узел
// вне объявленного контекста» != всегда дефект -> риск false-fire, вердикт ERROR vs
// WARNING — после репрезентативного горнила (парный, не один).
func ComputeContextCoverage(graph *model.Graph, cfg *archlintcfg.Config) CoverageResult {
	const topN = 10

	if graph == nil || cfg == nil || !cfg.HasContexts() || len(graph.Nodes) < 2 {
		return CoverageResult{Active: false}
	}

	pr := pageRankOf(graph)

	type nv struct {
		id string
		pr float64
	}

	ranked := make([]nv, 0, len(pr))
	for id, v := range pr {
		ranked = append(ranked, nv{id, v})
	}

	sort.Slice(ranked, func(i, j int) bool {
		if ranked[i].pr != ranked[j].pr {
			return ranked[i].pr > ranked[j].pr
		}

		return ranked[i].id < ranked[j].id // детерминированный tie-break
	})

	limit := topN
	if limit > len(ranked) {
		limit = len(ranked)
	}

	ctxComps := make([]string, 0)
	for c := range cfg.ContextComponents() {
		ctxComps = append(ctxComps, c)
	}

	sort.Strings(ctxComps)

	res := CoverageResult{Active: true}

	for _, n := range ranked[:limit] {
		res.Critical = append(res.Critical, n.id)

		if nodeCoveredByContext(n.id, ctxComps) {
			res.Covered = append(res.Covered, n.id)
		} else {
			res.Uncovered = append(res.Uncovered, n.id)
		}
	}

	if len(res.Critical) > 0 {
		res.Coverage = float64(len(res.Covered)) / float64(len(res.Critical))
	} else {
		res.Coverage = 1.0
	}

	return res
}

// nodeCoveredByContext — узел покрыт, если хоть один context-компонент fuzzy-матчит его
// (как Python: for comp in context_components: if _find_matching_node(comp, {node})).
func nodeCoveredByContext(node string, ctxComps []string) bool {
	one := []string{node}
	for _, comp := range ctxComps {
		if findMatchingNode(comp, one) != "" {
			return true
		}
	}

	return false
}

// pageRankOf — PageRank узлов графа (gonum, alpha=0.85), как в дескрипторах.
func pageRankOf(graph *model.Graph) map[string]float64 {
	dg := buildDescriptorGraph(graph)
	gg := dg.gonumGraph()

	out := make(map[string]float64, len(dg.nodes))
	for id64, v := range network.PageRank(gg, 0.85, 1e-13) {
		out[dg.nodes[int(id64)]] = v
	}

	return out
}

// normalizeComponent — последний сегмент после '/' (как Python _normalize_component_name).
func normalizeComponent(name string) string {
	if i := strings.LastIndexByte(name, '/'); i >= 0 {
		return name[i+1:]
	}

	return name
}

// findMatchingNode — fuzzy-матч компонента к узлу (1:1 с Python _find_matching_node):
// точная нормализованная пара ИЛИ двунаправленный substring. "" -> матча нет (ghost).
func findMatchingNode(component string, nodes []string) string {
	normComp := normalizeComponent(component)

	for _, node := range nodes {
		normNode := normalizeComponent(node)

		if normComp == normNode {
			return node
		}

		if strings.Contains(node, normComp) || strings.Contains(node, component) {
			return node
		}

		if strings.Contains(component, normNode) || strings.Contains(component, node) {
			return node
		}
	}

	return ""
}
