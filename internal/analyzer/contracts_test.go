package analyzer

import (
	"testing"

	"github.com/mshogin/archlint/internal/archlintcfg"
	"github.com/mshogin/archlint/internal/model"
)

// buildTestGraph builds a small graph for contract tests.
//
// Topology:
//
//	A -> B -> C
//	     B -> D
//
// i.e.:
//   - A depends on B (inbound[B]++)
//   - B depends on C (inbound[C]++)
//   - B depends on D (inbound[D]++)
func buildTestGraph() *model.Graph {
	return &model.Graph{
		Nodes: []model.Node{
			{ID: "A", Title: "A", Entity: "package"},
			{ID: "B", Title: "B", Entity: "package"},
			{ID: "C", Title: "C", Entity: "package"},
			{ID: "D", Title: "D", Entity: "package"},
		},
		Edges: []model.Edge{
			{From: "A", To: "B"},
			{From: "B", To: "C"},
			{From: "B", To: "D"},
		},
	}
}

// TestContractOK verifies that a contract whose module exists and has
// dependents is resolved as OK (not orphan, not unused).
func TestContractOK(t *testing.T) {
	g := buildTestGraph()
	contracts := []archlintcfg.ExternalContract{
		{Name: "b_contract", Module: "B", Type: "query"},
	}

	analysis := AnalyzeContracts(g, contracts)

	if len(analysis.OrphanContracts) != 0 {
		t.Errorf("expected 0 orphans, got %d: %v", len(analysis.OrphanContracts), analysis.OrphanContracts)
	}
	if len(analysis.UnusedContracts) != 0 {
		t.Errorf("expected 0 unused, got %d: %v", len(analysis.UnusedContracts), analysis.UnusedContracts)
	}
	if len(analysis.Contracts) != 1 {
		t.Fatalf("expected 1 contract info, got %d", len(analysis.Contracts))
	}
	info := analysis.Contracts[0]
	if info.Dependents != 1 {
		t.Errorf("expected 1 dependent (A->B), got %d", info.Dependents)
	}
	if info.Dependencies != 2 {
		t.Errorf("expected 2 dependencies (B->C, B->D), got %d", info.Dependencies)
	}
}

// TestContractUnused verifies that a contract whose module exists but has no
// inbound edges is reported as UNUSED.
func TestContractUnused(t *testing.T) {
	g := buildTestGraph()
	// Node "A" has no inbound edges in the test graph.
	contracts := []archlintcfg.ExternalContract{
		{Name: "a_contract", Module: "A", Type: "rest"},
	}

	analysis := AnalyzeContracts(g, contracts)

	if len(analysis.OrphanContracts) != 0 {
		t.Errorf("expected 0 orphans, got %v", analysis.OrphanContracts)
	}
	if len(analysis.UnusedContracts) != 1 || analysis.UnusedContracts[0] != "a_contract" {
		t.Errorf("expected [a_contract] in unused, got %v", analysis.UnusedContracts)
	}
	if analysis.Contracts[0].Dependents != 0 {
		t.Errorf("expected 0 dependents for A, got %d", analysis.Contracts[0].Dependents)
	}
}

// TestContractOrphan verifies that a contract whose module is missing from the
// graph is reported as ORPHAN.
func TestContractOrphan(t *testing.T) {
	g := buildTestGraph()
	contracts := []archlintcfg.ExternalContract{
		{Name: "old_endpoint", Module: "src::legacy::v1", Type: "rpc"},
	}

	analysis := AnalyzeContracts(g, contracts)

	if len(analysis.OrphanContracts) != 1 || analysis.OrphanContracts[0] != "old_endpoint" {
		t.Errorf("expected [old_endpoint] in orphans, got %v", analysis.OrphanContracts)
	}
	if len(analysis.UnusedContracts) != 0 {
		t.Errorf("expected 0 unused, got %v", analysis.UnusedContracts)
	}
}

// TestContractMixedStatuses verifies multiple contracts with different statuses
// at once.
func TestContractMixedStatuses(t *testing.T) {
	g := buildTestGraph()
	contracts := []archlintcfg.ExternalContract{
		{Name: "ok_contract", Module: "B", Type: "query"},       // OK
		{Name: "unused_contract", Module: "A", Type: "stream"},  // UNUSED
		{Name: "orphan_contract", Module: "Z", Type: "event"},   // ORPHAN
		{Name: "ok_contract2", Module: "C", Type: "rest"},       // UNUSED (C has 1 dep but no inbound besides B->C which is inbound)
	}

	analysis := AnalyzeContracts(g, contracts)

	if len(analysis.Contracts) != 4 {
		t.Fatalf("expected 4 contracts, got %d", len(analysis.Contracts))
	}

	orphans := toSet(analysis.OrphanContracts)
	unused := toSet(analysis.UnusedContracts)

	if !orphans["orphan_contract"] {
		t.Errorf("expected orphan_contract in orphans, got %v", analysis.OrphanContracts)
	}
	if !unused["unused_contract"] {
		t.Errorf("expected unused_contract in unused, got %v", analysis.UnusedContracts)
	}
	if orphans["ok_contract"] || unused["ok_contract"] {
		t.Errorf("ok_contract should be neither orphan nor unused")
	}
	// C has 1 inbound edge (B->C), so it should NOT be unused.
	if unused["ok_contract2"] {
		t.Errorf("ok_contract2 (module C) has 1 inbound edge, should not be unused")
	}
}

// TestContractEmpty verifies that an empty contracts list returns empty analysis.
func TestContractEmpty(t *testing.T) {
	g := buildTestGraph()
	analysis := AnalyzeContracts(g, nil)

	if len(analysis.Contracts) != 0 {
		t.Errorf("expected 0 contracts, got %d", len(analysis.Contracts))
	}
	if len(analysis.OrphanContracts) != 0 {
		t.Errorf("expected 0 orphans, got %v", analysis.OrphanContracts)
	}
	if len(analysis.UnusedContracts) != 0 {
		t.Errorf("expected 0 unused, got %v", analysis.UnusedContracts)
	}
}

// TestInjectContractNodes verifies that InjectContractNodes adds the right
// nodes and edges without modifying the original graph.
func TestInjectContractNodes(t *testing.T) {
	g := buildTestGraph()
	originalNodeCount := len(g.Nodes)
	originalEdgeCount := len(g.Edges)

	contracts := []archlintcfg.ExternalContract{
		{Name: "b_contract", Module: "B", Type: "query"},          // exists -> implements edge
		{Name: "orphan_contract", Module: "Z", Type: "rpc"},       // orphan -> no implements edge
	}

	enriched := InjectContractNodes(g, contracts)

	// Original graph must not be modified.
	if len(g.Nodes) != originalNodeCount {
		t.Errorf("original graph nodes modified: want %d, got %d", originalNodeCount, len(g.Nodes))
	}
	if len(g.Edges) != originalEdgeCount {
		t.Errorf("original graph edges modified: want %d, got %d", originalEdgeCount, len(g.Edges))
	}

	// Enriched graph should have 2 extra nodes (one per contract).
	if len(enriched.Nodes) != originalNodeCount+2 {
		t.Errorf("expected %d nodes in enriched graph, got %d", originalNodeCount+2, len(enriched.Nodes))
	}

	// Only one implements edge (for the non-orphan contract).
	implementsEdges := 0
	for _, e := range enriched.Edges {
		if e.Type == "implements" {
			implementsEdges++
		}
	}
	if implementsEdges != 1 {
		t.Errorf("expected 1 implements edge, got %d", implementsEdges)
	}

	// Verify entity type of injected nodes.
	for _, n := range enriched.Nodes {
		if n.Entity == "external_contract" {
			if n.ID != "contract::b_contract" && n.ID != "contract::orphan_contract" {
				t.Errorf("unexpected contract node ID: %s", n.ID)
			}
		}
	}
}

// toSet converts a string slice to a set for easy membership testing.
func toSet(s []string) map[string]bool {
	m := make(map[string]bool, len(s))
	for _, v := range s {
		m[v] = true
	}
	return m
}
