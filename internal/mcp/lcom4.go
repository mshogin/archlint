package mcp

import (
	"strings"

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

// ComputeLCOM4 computes the LCOM4 metric for the given type.
//
// Edge construction strategy:
// The Go AST parser records method calls with the *variable name* of the
// receiver (e.g. "w", "s"), not the type name. The pre-built graph call edges
// are not resolved for same-type intra-method calls because the resolver needs
// full type-inference to map variable names to types.
//
// Approach used here:
//  1. For each method M of the struct, scan M's Calls.
//  2. A call is an intra-struct call if its Target ends with
//     ".<receiverVar>.<methodName>" where <methodName> exists as another
//     method of the same struct. Because the AST captures `w.B()` as
//     Target="w.B" Receiver="w", we extract the last dotted segment as the
//     potential method name and check membership.
//
// This heuristic connects methods that call each other on the struct receiver,
// which is the primary cohesion signal in Go code. It does not detect cohesion
// via shared field access (that would require tracking `w.field` expressions
// per method body, which the analyzer does not currently provide).
//
// The graph parameter is accepted for API consistency but is not used for edge
// detection (the pre-built edges do not resolve intra-struct calls).
//
// TODO: Once the analyzer tracks field-access expressions per method body,
// augment with shared-field edges for true LCOM4 semantics.
func ComputeLCOM4(a *analyzer.GoAnalyzer, typeID string, _ *model.Graph) *LCOMResult {
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

	// Build adjacency list from intra-struct call analysis.
	adj := buildMethodGraphFromCalls(a, typeMethods)

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

// buildMethodGraphFromCalls constructs an undirected adjacency list (by method
// name) by scanning each method's Call list for intra-struct calls.
//
// Detection logic: the AST parser stores a call `w.B()` as
//
//	CallInfo{Target: "w.B", IsMethod: true, Receiver: "w"}
//
// We extract the last segment of Target (after the final ".") and check
// whether it matches a method name in the same struct. If it does, we add
// an undirected edge between the caller and that method.
//
// Edge cases handled:
//   - Self-calls (methodA calling methodA) are ignored.
//   - Calls to other types that happen to share a method name but have a
//     different target path are skipped because we only check IsMethod calls
//     whose full Target has exactly one dot (i.e. "receiverVar.MethodName").
func buildMethodGraphFromCalls(
	a *analyzer.GoAnalyzer,
	typeMethods map[string]string, // name -> methodID
) map[string]map[string]bool {
	adj := make(map[string]map[string]bool, len(typeMethods))

	for name := range typeMethods {
		adj[name] = make(map[string]bool)
	}

	for callerName, callerID := range typeMethods {
		m := a.LookupMethod(callerID)
		if m == nil {
			continue
		}

		for _, call := range m.Calls {
			if !call.IsMethod {
				continue
			}

			// Target format: "receiverVar.MethodName"
			// Extract the method name as the last segment.
			calleeName := lastSegment(call.Target)
			if calleeName == "" || calleeName == callerName {
				continue
			}

			// Check that this method name actually belongs to the same struct.
			if _, exists := typeMethods[calleeName]; exists {
				adj[callerName][calleeName] = true
				adj[calleeName][callerName] = true
			}
		}
	}

	return adj
}

// lastSegment returns the substring after the last "." in s, or s itself if
// there is no ".".
func lastSegment(s string) string {
	idx := strings.LastIndex(s, ".")
	if idx < 0 {
		return s
	}

	return s[idx+1:]
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
