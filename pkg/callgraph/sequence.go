package callgraph

import (
	"fmt"
	"strings"

	"github.com/mshogin/archlint/pkg/tracer"
)

// SequenceOptions настройки генерации диаграммы.
type SequenceOptions struct {
	MaxDepth       int
	ShowPackages   bool
	MarkAsync      bool
	MarkInterface  bool
	GroupByPackage bool
	Title          string
}

// DefaultSequenceOptions возвращает настройки по умолчанию.
func DefaultSequenceOptions() SequenceOptions {
	return SequenceOptions{
		MaxDepth:      5,
		ShowPackages:  true,
		MarkAsync:     true,
		MarkInterface: true,
	}
}

// SequenceGenerator генерирует PlantUML sequence диаграммы.
type SequenceGenerator struct {
	options SequenceOptions
}

// NewSequenceGenerator создает новый генератор.
func NewSequenceGenerator(opts SequenceOptions) *SequenceGenerator {
	return &SequenceGenerator{options: opts}
}

// Generate генерирует PlantUML код из графа вызовов.
func (g *SequenceGenerator) Generate(cg *CallGraph) (string, error) {
	tracer.Enter("SequenceGenerator.Generate")

	if cg == nil || len(cg.Nodes) == 0 {
		tracer.ExitSuccess("SequenceGenerator.Generate")

		return "", nil
	}

	var sb strings.Builder

	sb.WriteString("@startuml\n")

	title := g.options.Title
	if title == "" {
		title = fmt.Sprintf("Event: %s -> %s", cg.EventName, cg.EntryPoint)
	}

	fmt.Fprintf(&sb, "title %s\n\n", title)

	nodeByID := make(map[string]*CallNode)
	for i := range cg.Nodes {
		nodeByID[cg.Nodes[i].ID] = &cg.Nodes[i]
	}

	g.writeParticipants(&sb, cg)
	sb.WriteString("\n")
	g.writeEdges(&sb, cg, nodeByID)
	sb.WriteString("\n@enduml\n")

	tracer.ExitSuccess("SequenceGenerator.Generate")

	return sb.String(), nil
}

func (g *SequenceGenerator) writeParticipants(sb *strings.Builder, cg *CallGraph) {
	if g.options.GroupByPackage {
		g.writeGroupedParticipants(sb, cg)

		return
	}

	g.writeFlatParticipants(sb, cg)
}

func (g *SequenceGenerator) writeFlatParticipants(sb *strings.Builder, cg *CallGraph) {
	seen := make(map[string]bool)

	for _, node := range cg.Nodes {
		if node.Depth > g.options.MaxDepth {
			continue
		}

		alias := g.participantAlias(node.ID)
		if seen[alias] {
			continue
		}

		seen[alias] = true
		label := g.participantLabel(&node)
		fmt.Fprintf(sb, "participant \"%s\" as %s\n", label, alias)
	}
}

func (g *SequenceGenerator) writeGroupedParticipants(sb *strings.Builder, cg *CallGraph) {
	packages, pkgOrder := g.collectPackages(cg)
	colors := []string{"#LightBlue", "#LightGreen", "#LightYellow", "#LightCoral", "#LightCyan"}

	for i, pkg := range pkgOrder {
		color := colors[i%len(colors)]
		fmt.Fprintf(sb, "box \"%s\" %s\n", pkg, color)

		aliasSeen := make(map[string]bool)

		for _, node := range packages[pkg] {
			alias := g.participantAlias(node.ID)
			if aliasSeen[alias] {
				continue
			}

			aliasSeen[alias] = true
			label := g.participantLabel(&node)
			fmt.Fprintf(sb, "    participant \"%s\" as %s\n", label, alias)
		}

		sb.WriteString("end box\n")
	}
}

func (g *SequenceGenerator) collectPackages(cg *CallGraph) (packages map[string][]CallNode, order []string) {
	packages = make(map[string][]CallNode)
	seen := make(map[string]bool)

	for _, node := range cg.Nodes {
		if node.Depth > g.options.MaxDepth {
			continue
		}

		pkg := node.Package
		if pkg == "" {
			pkg = "external"
		}

		if !seen[pkg] {
			order = append(order, pkg)
			seen[pkg] = true
		}

		packages[pkg] = append(packages[pkg], node)
	}

	return packages, order
}

func (g *SequenceGenerator) writeEdges(sb *strings.Builder, cg *CallGraph, nodeByID map[string]*CallNode) {
	for _, edge := range cg.Edges {
		fromNode := nodeByID[edge.From]
		toNode := nodeByID[edge.To]

		if fromNode == nil || toNode == nil {
			continue
		}

		if fromNode.Depth > g.options.MaxDepth {
			continue
		}

		from := g.participantAlias(edge.From)
		to := g.participantAlias(edge.To)
		label := g.edgeLabel(&edge, toNode)
		arrow := g.arrowStyle(&edge)

		fmt.Fprintf(sb, "%s %s %s : %s\n", from, arrow, to, label)

		if edge.Cycle {
			sb.WriteString("note right: cycle detected\n")
		}

		if edge.Async {
			sb.WriteString("note right: goroutine\n")
		}
	}
}

func (g *SequenceGenerator) participantAlias(id string) string {
	r := strings.NewReplacer("/", "_", ".", "_", "-", "_", "<", "_", ">", "_")

	return r.Replace(id)
}

func (g *SequenceGenerator) participantLabel(node *CallNode) string {
	if node.Receiver != "" {
		if g.options.ShowPackages && node.Package != "" {
			return fmt.Sprintf("%s.%s", node.Package, node.Receiver)
		}

		return node.Receiver
	}

	if g.options.ShowPackages && node.Package != "" {
		return fmt.Sprintf("%s.%s", node.Package, node.Function)
	}

	return node.Function
}

func (g *SequenceGenerator) edgeLabel(edge *CallEdge, toNode *CallNode) string {
	label := toNode.Function

	if g.options.MarkAsync && edge.Async {
		label = "**async** " + label
	}

	if g.options.MarkInterface && edge.CallType == CallInterface {
		label += " <<interface>>"
	}

	if edge.CallType == CallDeferred {
		label = "defer " + label
	}

	return label
}

func (g *SequenceGenerator) arrowStyle(edge *CallEdge) string {
	if edge.Async {
		return "->>"
	}

	return "->"
}
