package mcp

import (
	"github.com/mshogin/archlint/internal/analyzer"
	"github.com/mshogin/archlint/internal/model"
)

// LCOMResult holds the LCOM4 computation result for a single type.
//
// LCOM4 (Lack of Cohesion of Methods, Hitz & Montazeri 1995) measures how
// cohesive a struct/class is by counting the number of connected components
// in the method graph.
//
// Interpretation:
//   - LCOM4 = 0: no methods (trivial / skip)
//   - LCOM4 = 1: fully cohesive, single responsibility (good)
//   - LCOM4 >= 2: multiple disjoint method clusters, SRP violation
//   - LCOM4 = N (N = method count): every method is isolated, max violation
type LCOMResult struct {
	// Type is the fully-qualified type ID (pkgID.TypeName).
	Type string

	// LCOM is the number of connected components (0 for empty types).
	LCOM int

	// Methods lists all method names belonging to this type.
	Methods []string

	// Components lists each connected component as a slice of method names.
	// len(Components) == LCOM.
	Components [][]string
}

// ComputeLCOM4 computes the LCOM4 metric for the given type using the
// architecture graph that was already built by the analyzer.
//
// Edge construction strategy (limitation note):
// The Go AST parser in this codebase does not track which struct fields each
// method accesses. True LCOM4 would connect methods that share a field access.
//
// Instead, we use the call graph edges already present in the model.Graph:
// two methods of the same struct are connected when one directly calls the
// other (i.e. there is a "calls" edge between their method IDs in the graph).
//
// This is a conservative heuristic: it detects structural cohesion via
// direct intra-struct method calls. Methods that only access the same field
// without calling each other will appear in separate components, producing
// higher (worse) LCOM4 values than true field-based LCOM4.
//
// TODO: Once the analyzer tracks field-access expressions per method body,
// replace or augment the call-based heuristic with proper shared-field edges.
func ComputeLCOM4(a *analyzer.GoAnalyzer, typeID string, graph *model.Graph) *LCOMResult {
	t := a.LookupType(typeID)
	if t == nil {
		return &LCOMResult{Type: typeID}
	}

	// Collect methods that belong to this type.
	// Method IDs have the form: pkgID.ReceiverName.MethodName
	typeMethods := collectTypeMethods(a, t.Package, t.Name)

	if len(typeMethods) == 0 {
		return &LCOMResult{Type: typeID, LCOM: 0, Methods: nil, Components: nil}
	}

	// Build adjacency list using "calls" edges from the pre-built graph.
	adj := buildMethodGraphFromEdges(typeMethods, graph)

	// Count connected components via BFS.
	components := countConnectedComponents(typeMethods, adj)

	methodNames := make([]string, 0, len(typeMethods))
	for name := range typeMethods {
		methodNames = append(methodNames, name)
	}

	return &LCOMResult{
		Type:       typeID,
		LCOM:       len(components),
		Methods:    methodNames,
		Components: components,
	}
}

// collectTypeMethods returns a set keyed by method name with method ID values
// for all methods whose receiver is receiverName in the given package.
func collectTypeMethods(a *analyzer.GoAnalyzer, pkg, receiverName string) map[string]string {
	result := make(map[string]string)

	for methodID, m := range a.AllMethods() {
		if m.Package == pkg && m.Receiver == receiverName {
			result[m.Name] = methodID
		}
	}

	return result
}

// buildMethodGraphFromEdges constructs an undirected adjacency list (by method
// name) from the pre-built graph's "calls" edges. Two methods of the same
// struct are adjacent when a "calls" edge exists between their method IDs.
func buildMethodGraphFromEdges(
	typeMethods map[string]string, // name -> methodID
	graph *model.Graph,
) map[string]map[string]bool {
	adj := make(map[string]map[string]bool, len(typeMethods))

	for name := range typeMethods {
		adj[name] = make(map[string]bool)
	}

	// Reverse index: methodID -> methodName (for this type only).
	idToName := make(map[string]string, len(typeMethods))
	for name, id := range typeMethods {
		idToName[id] = name
	}

	for _, edge := range graph.Edges {
		if edge.Type != "calls" {
			continue
		}

		fromName, fromOK := idToName[edge.From]
		toName, toOK := idToName[edge.To]

		if fromOK && toOK && fromName != toName {
			adj[fromName][toName] = true
			adj[toName][fromName] = true
		}
	}

	return adj
}

// countConnectedComponents performs BFS over the undirected method graph and
// returns each component as a slice of method names.
func countConnectedComponents(
	typeMethods map[string]string,
	adj map[string]map[string]bool,
) [][]string {
	visited := make(map[string]bool, len(typeMethods))
	var components [][]string

	for name := range typeMethods {
		if visited[name] {
			continue
		}

		// BFS from this node.
		component := []string{}
		queue := []string{name}
		visited[name] = true

		for len(queue) > 0 {
			current := queue[0]
			queue = queue[1:]
			component = append(component, current)

			for neighbor := range adj[current] {
				if !visited[neighbor] {
					visited[neighbor] = true
					queue = append(queue, neighbor)
				}
			}
		}

		components = append(components, component)
	}

	return components
}
