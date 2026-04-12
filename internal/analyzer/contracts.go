// Package analyzer provides architecture analysis utilities.
// contracts.go implements cross-graph contract analysis.
package analyzer

import (
	"github.com/mshogin/archlint/internal/archlintcfg"
	"github.com/mshogin/archlint/internal/model"
)

// ContractInfo holds the resolved metadata for a single external contract.
type ContractInfo struct {
	// Name is the human-readable contract identifier from config.
	Name string
	// Module is the module ID from config (may not exist in graph).
	Module string
	// Type is the protocol style (query, stream, rpc, rest, event).
	Type string
	// Schema is the optional schema reference.
	Schema string
	// Dependents is the count of internal nodes that have an edge pointing TO
	// the contract's module, i.e. nodes that depend on it.
	Dependents int
	// Dependencies is the count of nodes that the contract's module depends on,
	// i.e. edges going OUT from the module.
	Dependencies int
}

// ContractAnalysis is the result of AnalyzeContracts.
type ContractAnalysis struct {
	// Contracts contains resolved info for every contract in the config.
	Contracts []ContractInfo
	// OrphanContracts lists contract names whose module was not found in the graph.
	OrphanContracts []string
	// UnusedContracts lists contract names whose module exists but has no
	// internal nodes depending on it.
	UnusedContracts []string
}

// AnalyzeContracts resolves each ExternalContract against the graph and returns
// dependency counts, orphans, and unused entries.
//
// A contract is "orphan" when its Module ID does not match any node in the graph.
// A contract is "unused" when its module exists but no other node has an edge
// pointing to it (dependents == 0).
func AnalyzeContracts(graph *model.Graph, contracts []archlintcfg.ExternalContract) *ContractAnalysis {
	// Build a set of existing node IDs for O(1) lookup.
	nodeSet := make(map[string]struct{}, len(graph.Nodes))
	for _, n := range graph.Nodes {
		nodeSet[n.ID] = struct{}{}
	}

	// Count inbound edges (dependents) and outbound edges (dependencies) per node.
	inbound := make(map[string]int, len(graph.Nodes))
	outbound := make(map[string]int, len(graph.Nodes))
	for _, e := range graph.Edges {
		outbound[e.From]++
		inbound[e.To]++
	}

	result := &ContractAnalysis{
		Contracts: make([]ContractInfo, 0, len(contracts)),
	}

	for _, c := range contracts {
		info := ContractInfo{
			Name:   c.Name,
			Module: c.Module,
			Type:   c.Type,
			Schema: c.Schema,
		}

		if _, exists := nodeSet[c.Module]; !exists {
			// Module not found in the graph.
			result.OrphanContracts = append(result.OrphanContracts, c.Name)
		} else {
			info.Dependents = inbound[c.Module]
			info.Dependencies = outbound[c.Module]
			if info.Dependents == 0 {
				result.UnusedContracts = append(result.UnusedContracts, c.Name)
			}
		}

		result.Contracts = append(result.Contracts, info)
	}

	return result
}

// InjectContractNodes returns a shallow copy of the graph with additional
// "external_contract" nodes and "implements" edges for each contract whose
// module exists in the original graph.  The original graph is not modified.
func InjectContractNodes(graph *model.Graph, contracts []archlintcfg.ExternalContract) *model.Graph {
	nodeSet := make(map[string]struct{}, len(graph.Nodes))
	for _, n := range graph.Nodes {
		nodeSet[n.ID] = struct{}{}
	}

	newNodes := make([]model.Node, len(graph.Nodes))
	copy(newNodes, graph.Nodes)

	newEdges := make([]model.Edge, len(graph.Edges))
	copy(newEdges, graph.Edges)

	for _, c := range contracts {
		contractID := "contract::" + c.Name
		newNodes = append(newNodes, model.Node{
			ID:     contractID,
			Title:  c.Name,
			Entity: "external_contract",
		})
		// Only add the implements edge when the module is present in the graph.
		if _, ok := nodeSet[c.Module]; ok {
			newEdges = append(newEdges, model.Edge{
				From: c.Module,
				To:   contractID,
				Type: "implements",
			})
		}
	}

	return &model.Graph{
		Nodes: newNodes,
		Edges: newEdges,
	}
}
