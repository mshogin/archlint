package mcp

import (
	"strings"

	"github.com/mshogin/archlint/internal/analyzer"
	"github.com/mshogin/archlint/internal/model"
)

// ReachSRPResult holds the reach-based SRP metric (ρ) for a single type.
//
// Reach-SRP improves on LCOM4 by considering the external reach of methods
// (calls to nodes outside the type's package) in addition to shared field
// access (internal reach). Two methods are cohesive if they share at least
// one resource (field or external call target), if one calls the other, or
// if both are "pure" (no external reach at all).
//
// Interpretation:
//   - ρ = 0: no methods (trivial / skip)
//   - ρ = 1: single responsibility (good)
//   - ρ >= 2: multiple responsibilities detected (SRP violation)
type ReachSRPResult struct {
	// Type is the fully-qualified type ID (pkgID.TypeName).
	Type string `json:"type"`

	// Rho is the responsibility count (number of equivalence classes).
	Rho int `json:"rho"`

	// LCOM4 is the classic LCOM4 metric computed for comparison.
	LCOM4 int `json:"lcom4"`

	// Classes contains each equivalence class as a slice of method names.
	// len(Classes) == Rho.
	Classes [][]string `json:"classes"`

	// PureMethodCount is the number of methods with empty external reach
	// (no calls to nodes outside the type's package).
	PureMethodCount int `json:"pureMethodCount"`
}

// ComputeReachSRP computes the reach-based SRP metric (ρ) for the given type.
//
// Algorithm:
//  1. Collect all methods of type T.
//  2. For each method m compute:
//     - R_int(m): field node IDs reachable via field_read/field_write edges.
//     - R_ext(m): node IDs reachable via "calls" edges where the target's
//     package differs from T's package (external calls).
//     - R(m) = R_int(m) ∪ R_ext(m).
//  3. Build an undirected equivalence graph on methods:
//     - Edge between m_i and m_j if R(m_i) ∩ R(m_j) ≠ ∅ (shared resource).
//     - Edge if m_i calls m_j or vice versa (internal delegation).
//     - Edge if both are "pure" (R_ext = ∅) — data-carrier unification rule.
//  4. Count connected components via BFS → ρ(T).
//  5. Also compute LCOM4 for side-by-side comparison.
func ComputeReachSRP(a *analyzer.GoAnalyzer, typeID string, graph *model.Graph) *ReachSRPResult {
	t := a.LookupType(typeID)
	if t == nil {
		return &ReachSRPResult{Type: typeID}
	}

	// Collect methods that belong to this type.
	typeMethods := collectTypeMethods(a, t.Package, t.Name)

	if len(typeMethods) == 0 {
		return &ReachSRPResult{Type: typeID, Rho: 0}
	}

	// Build per-method reach sets from the graph.
	rInt := computeRInt(typeMethods, graph)
	rExt := computeRExt(typeMethods, t.Package, graph, a)

	// Merged reach: R(m) = R_int(m) ∪ R_ext(m).
	reach := make(map[string]map[string]bool, len(typeMethods))
	for name := range typeMethods {
		r := make(map[string]bool)
		for id := range rInt[name] {
			r[id] = true
		}
		for id := range rExt[name] {
			r[id] = true
		}
		reach[name] = r
	}

	// Count pure methods (no external reach).
	pureCount := 0
	for name := range typeMethods {
		if len(rExt[name]) == 0 {
			pureCount++
		}
	}

	// Build adjacency list for the equivalence graph.
	adj := buildReachGraph(typeMethods, reach, rExt, a)

	// Count connected components.
	components := countConnectedComponents(typeMethods, adj)

	// Compute LCOM4 for comparison.
	lcom := ComputeLCOM4(a, typeID, graph)

	return &ReachSRPResult{
		Type:            typeID,
		Rho:             len(components),
		LCOM4:           lcom.LCOM,
		Classes:         components,
		PureMethodCount: pureCount,
	}
}

// computeRInt computes the internal reach for each method: the set of field
// node IDs that the method accesses via field_read or field_write edges.
func computeRInt(
	typeMethods map[string]string, // methodName -> methodID
	graph *model.Graph,
) map[string]map[string]bool { // methodName -> set of field node IDs
	result := make(map[string]map[string]bool, len(typeMethods))
	for name := range typeMethods {
		result[name] = make(map[string]bool)
	}

	// Build reverse index: methodID -> methodName for quick lookup.
	idToName := make(map[string]string, len(typeMethods))
	for name, id := range typeMethods {
		idToName[id] = name
	}

	for _, edge := range graph.Edges {
		if edge.Type != model.EdgeFieldRead && edge.Type != model.EdgeFieldWrite {
			continue
		}
		name, ok := idToName[edge.From]
		if !ok {
			continue
		}
		result[name][edge.To] = true
	}

	return result
}

// computeRExt computes the external reach for each method.
//
// External reach = set of stable, normalized call targets whose receiver is a
// different package than the type's own package.  Because the Go graph builder
// only adds resolved call edges to the graph (and cannot resolve stdlib/external
// package targets), we derive external reach directly from the method's
// CallInfo list rather than from graph edges.
//
// A call contributes to external reach when ALL of the following hold:
//   - call.IsMethod == true (method call, not a plain function call)
//   - call.Receiver is not empty
//   - call.Receiver does not equal the struct receiver variable name (not a
//     self-call via the struct receiver)
//   - The call target is not an intra-package identifier (i.e. contains a "."
//     separator so it looks like "pkg.Method" rather than just "Method")
//
// The representative key added to the set is "receiver.MethodName" (the
// call.Target as recorded by the parser), which uniquely identifies the
// combination of external package and method being invoked.
func computeRExt(
	typeMethods map[string]string, // methodName -> methodID
	_ string, // typePkg - reserved for future use
	_ *model.Graph, // graph - reserved for future use
	a *analyzer.GoAnalyzer,
) map[string]map[string]bool { // methodName -> set of external call keys
	result := make(map[string]map[string]bool, len(typeMethods))
	for name := range typeMethods {
		result[name] = make(map[string]bool)
	}

	// Determine the struct's own receiver variable names across all methods so
	// we can exclude self-calls (e.g. "s.B()" where s is the struct receiver).
	ownReceivers := make(map[string]bool)
	for name, methodID := range typeMethods {
		_ = name
		m := a.LookupMethod(methodID)
		if m == nil {
			continue
		}
		// The receiver variable used in this method (e.g. "s", "g", "w").
		// We infer it from the method's own calls: any intra-struct call will
		// have call.Receiver matching this variable.
		// Simpler: we can scan all calls and collect receivers that refer to
		// the same-type methods.
		for _, call := range m.Calls {
			if call.IsMethod && call.Receiver != "" {
				// If the callee method name is one of our own methods, the
				// receiver variable is an intra-struct receiver.
				calleeName := lastSegment(call.Target)
				if _, ours := typeMethods[calleeName]; ours {
					ownReceivers[call.Receiver] = true
				}
			}
		}
	}

	for methodName, methodID := range typeMethods {
		m := a.LookupMethod(methodID)
		if m == nil {
			continue
		}

		for _, call := range m.Calls {
			if !call.IsMethod || call.Receiver == "" {
				continue
			}
			// Skip self-calls (intra-struct delegation — handled elsewhere).
			if ownReceivers[call.Receiver] {
				continue
			}
			// The target must look like "pkg.Method" (contains a dot).
			if !strings.Contains(call.Target, ".") {
				continue
			}
			result[methodName][call.Target] = true
		}
	}

	return result
}

// isInPackage returns true when nodeID starts with pkgID followed by ".".
// This is the convention used throughout the archlint graph builder.
func isInPackage(nodeID, pkgID string) bool {
	if len(nodeID) <= len(pkgID) {
		return false
	}
	return nodeID[:len(pkgID)+1] == pkgID+"."
}

// buildReachGraph constructs the equivalence adjacency list for the method set.
// Three rules add edges:
//  1. Shared resource: R(m_i) ∩ R(m_j) ≠ ∅.
//  2. Internal delegation: m_i calls m_j or vice versa.
//  3. Both pure: R_ext(m_i) = ∅ AND R_ext(m_j) = ∅ (data-carrier unification).
func buildReachGraph(
	typeMethods map[string]string,
	reach map[string]map[string]bool,
	rExt map[string]map[string]bool,
	a *analyzer.GoAnalyzer,
) map[string]map[string]bool {
	adj := make(map[string]map[string]bool, len(typeMethods))
	for name := range typeMethods {
		adj[name] = make(map[string]bool)
	}

	// Collect method names as a sorted slice for deterministic pairwise iteration.
	names := make([]string, 0, len(typeMethods))
	for name := range typeMethods {
		names = append(names, name)
	}

	// Precompute internal delegation edges from LCOM4's call graph logic.
	callAdj := buildMethodGraphFromCalls(a, typeMethods) // name -> set of names

	for i := 0; i < len(names); i++ {
		for j := i + 1; j < len(names); j++ {
			ni, nj := names[i], names[j]

			// Rule 2: Internal delegation.
			if callAdj[ni][nj] || callAdj[nj][ni] {
				adj[ni][nj] = true
				adj[nj][ni] = true
				continue
			}

			// Rule 3: Both pure (no external reach).
			if len(rExt[ni]) == 0 && len(rExt[nj]) == 0 {
				adj[ni][nj] = true
				adj[nj][ni] = true
				continue
			}

			// Rule 1: Shared resource in merged reach sets.
			if setsIntersect(reach[ni], reach[nj]) {
				adj[ni][nj] = true
				adj[nj][ni] = true
			}
		}
	}

	return adj
}

// setsIntersect returns true when the two bool-valued maps share at least one key.
func setsIntersect(a, b map[string]bool) bool {
	// Iterate the smaller set for efficiency.
	if len(a) > len(b) {
		a, b = b, a
	}
	for k := range a {
		if b[k] {
			return true
		}
	}
	return false
}
