package mcp

import (
	"fmt"
	"sort"
	"strings"

	"github.com/mshogin/archlint/internal/archlintcfg"
	"github.com/mshogin/archlint/internal/model"
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
