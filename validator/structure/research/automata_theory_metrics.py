"""
Automata Theory Metrics for Software Architecture Validation.

Applies automata and formal language concepts:
1. State Machine Analysis - DFA/NFA properties
2. Regular Language Properties - closure and decidability
3. Pushdown Automata - context-free patterns
4. Turing Machine Concepts - computability bounds
5. Bisimulation - behavioral equivalence

Mathematical foundations:
- DFA minimization via Hopcroft algorithm
- Myhill-Nerode theorem for state equivalence
- Pumping lemma implications
- Bisimulation relations for process equivalence
"""

from typing import Any, Dict, List
import networkx as nx
from collections import defaultdict
import numpy as np


def _is_excluded(node: str, exclude: List[str]) -> bool:
    """Check if node matches any exclusion pattern."""
    for pattern in exclude:
        if pattern in node:
            return True
    return False


def validate_state_machine_properties(graph: nx.DiGraph, config: Any = None) -> Dict[str, Any]:
    """
    Analyze architecture as a state machine (DFA/NFA).

    Models:
    - Components as states
    - Dependencies as transitions
    - Entry points as initial states
    - Terminal components as accepting states

    Checks:
    - Determinism (unique transitions for inputs)
    - Completeness (handling all inputs)
    - Minimality (no redundant states)
    """
    components = [{"name": n, **graph.nodes[n]} for n in graph.nodes()]
    dependencies = [{"from": u, "to": v, **graph.edges[u,v]} for u,v in graph.edges()]

    if len(components) < 2:
        return {
            "name": "state_machine_properties",
            "status": "INFO",
            "reason": "Insufficient components for state machine analysis",
            
            "details": {"components": len(components)}
        }

    # Build transition graph
    G = nx.DiGraph()

    for comp in components:
        name = comp.get("name", "")
        comp_type = comp.get("type", "")
        if name:
            G.add_node(name, type=comp_type)

    # Group transitions by label (dependency type)
    transitions = defaultdict(list)

    for dep in dependencies:
        src = dep.get("from", dep.get("source", ""))
        tgt = dep.get("to", dep.get("target", ""))
        label = dep.get("type", "default")

        if src and tgt and src in G.nodes() and tgt in G.nodes():
            G.add_edge(src, tgt, label=label)
            transitions[(src, label)].append(tgt)

    # Check determinism: each (state, input) has at most one target
    non_deterministic = []
    for (state, label), targets in transitions.items():
        if len(set(targets)) > 1:
            non_deterministic.append({
                "state": state,
                "label": label,
                "targets": list(set(targets))
            })

    is_deterministic = len(non_deterministic) == 0

    # Find initial states (no incoming edges)
    initial_states = [n for n in G.nodes() if G.in_degree(n) == 0]

    # Find accepting/terminal states (no outgoing edges)
    accepting_states = [n for n in G.nodes() if G.out_degree(n) == 0]

    # Check reachability from initial states
    reachable = set()
    for init in initial_states:
        reachable.add(init)
        reachable.update(nx.descendants(G, init))

    unreachable = [n for n in G.nodes() if n not in reachable]

    # Check if all accepting states are reachable
    reachable_accepting = [s for s in accepting_states if s in reachable]

    # Calculate state equivalence classes (Myhill-Nerode)
    # Simplified: group by (in_degree, out_degree, labels) signature
    signatures = defaultdict(list)
    for node in G.nodes():
        in_labels = sorted([G.edges[e]['label'] for e in G.in_edges(node)] if G.in_edges(node) else [])
        out_labels = sorted([G.edges[e]['label'] for e in G.out_edges(node)] if G.out_edges(node) else [])
        sig = (G.in_degree(node), G.out_degree(node), tuple(in_labels[:5]), tuple(out_labels[:5]))
        signatures[sig].append(node)

    # Potential equivalent states (same signature)
    potential_redundant = [nodes for nodes in signatures.values() if len(nodes) > 1]

    valid = is_deterministic and len(unreachable) == 0

    return {
        "name": "state_machine_properties",
        "status": "WARNING" if not valid else "INFO",
        "reason": f"State machine: {'deterministic' if is_deterministic else 'non-deterministic'}, " +
                f"{len(initial_states)} initial, {len(accepting_states)} accepting, " +
                f"{len(unreachable)} unreachable" +
                ("" if valid else " - issues found"),
        
        "details": {
            "is_deterministic": is_deterministic,
            "non_deterministic_transitions": non_deterministic[:5],
            "initial_states": initial_states[:10],
            "accepting_states": accepting_states[:10],
            "unreachable_states": unreachable[:10],
            "equivalence_classes": len(signatures),
            "potential_redundant_groups": len(potential_redundant)
        }
    }


def validate_regular_language_properties(graph: nx.DiGraph, config: Any = None) -> Dict[str, Any]:
    """
    Analyze regular language properties of dependency patterns.

    Regular languages are closed under:
    - Union, intersection, complement
    - Concatenation, Kleene star
    - Homomorphism

    For architecture:
    - Dependency patterns should form regular structures
    - Non-regular patterns indicate complex coupling
    """
    components = [{"name": n, **graph.nodes[n]} for n in graph.nodes()]
    dependencies = [{"from": u, "to": v, **graph.edges[u,v]} for u,v in graph.edges()]

    if len(components) < 3:
        return {
            "name": "regular_language_properties",
            "status": "INFO",
            "reason": "Insufficient components for language analysis",
            
            "details": {"components": len(components)}
        }

    # Build graph
    G = nx.DiGraph()

    for comp in components:
        name = comp.get("name", "")
        if name:
            G.add_node(name)

    for dep in dependencies:
        src = dep.get("from", dep.get("source", ""))
        tgt = dep.get("to", dep.get("target", ""))
        if src and tgt and src in G.nodes() and tgt in G.nodes():
            G.add_edge(src, tgt)

    # Analyze cycle structure (related to pumping lemma)
    from validator.utils.cycles import simple_cycles_bounded
    try:
        cycles = list(simple_cycles_bounded(G, max_length=10))
    except:
        cycles = []

    # Cycle length distribution
    cycle_lengths = [len(c) for c in cycles]

    # Long cycles indicate potential non-regular complexity
    long_cycles = [c for c in cycles if len(c) > 5]

    # Nested cycles (cycles within cycles)
    nodes_in_cycles = set()
    for cycle in cycles:
        nodes_in_cycles.update(cycle)

    # Check for "center of mass" - nodes participating in many cycles
    cycle_participation = defaultdict(int)
    for cycle in cycles:
        for node in cycle:
            cycle_participation[node] += 1

    high_participation = [n for n, count in cycle_participation.items() if count > 3]

    # Analyze strongly connected components (analogous to loop structures)
    sccs = list(nx.strongly_connected_components(G))
    non_trivial_sccs = [scc for scc in sccs if len(scc) > 1]

    # Check for nested SCCs (hierarchical loop structure)
    condensation = nx.condensation(G)

    # Depth of DAG of SCCs
    if condensation.number_of_nodes() > 0:
        try:
            scc_dag_length = nx.dag_longest_path_length(condensation)
        except:
            scc_dag_length = 0
    else:
        scc_dag_length = 0

    # Regularity score based on structural patterns
    regularity_issues = []

    if len(long_cycles) > len(cycles) * 0.2:
        regularity_issues.append(f"Many long cycles ({len(long_cycles)})")

    if len(high_participation) > len(components) * 0.1:
        regularity_issues.append(f"High cycle participation ({len(high_participation)} nodes)")

    if scc_dag_length > 5:
        regularity_issues.append(f"Deep SCC hierarchy (depth {scc_dag_length})")

    max_issues = getattr(config, 'max_regularity_issues', 2)
    valid = len(regularity_issues) <= max_issues

    return {
        "name": "regular_language_properties",
        "status": "WARNING" if not valid else "INFO",
        "reason": f"Language analysis: {len(cycles)} cycles, {len(non_trivial_sccs)} SCCs, " +
                f"SCC depth={scc_dag_length}" +
                ("" if valid else f", {len(regularity_issues)} issues"),
        
        "details": {
            "total_cycles": len(cycles),
            "long_cycles": len(long_cycles),
            "cycle_length_distribution": dict(zip(*np.unique(cycle_lengths, return_counts=True))) if cycle_lengths else {},
            "non_trivial_sccs": len(non_trivial_sccs),
            "scc_dag_depth": scc_dag_length,
            "high_cycle_participation": high_participation[:10],
            "regularity_issues": regularity_issues
        }
    }


def validate_pushdown_patterns(graph: nx.DiGraph, config: Any = None) -> Dict[str, Any]:
    """
    Analyze pushdown automata patterns (context-free structures).

    Context-free patterns in architecture:
    - Nested call structures (balanced parentheses)
    - Recursive module dependencies
    - Stack-based processing flows

    Detects:
    - Unbounded recursion depth
    - Missing base cases
    - Unbalanced structures
    """
    components = [{"name": n, **graph.nodes[n]} for n in graph.nodes()]
    dependencies = [{"from": u, "to": v, **graph.edges[u,v]} for u,v in graph.edges()]

    if len(components) < 3:
        return {
            "name": "pushdown_patterns",
            "status": "INFO",
            "reason": "Insufficient components for pushdown analysis",
            
            "details": {"components": len(components)}
        }

    # Build graph
    G = nx.DiGraph()

    for comp in components:
        name = comp.get("name", "")
        layer = comp.get("layer", "")
        if name:
            G.add_node(name, layer=layer)

    for dep in dependencies:
        src = dep.get("from", dep.get("source", ""))
        tgt = dep.get("to", dep.get("target", ""))
        dep_type = dep.get("type", "")
        if src and tgt and src in G.nodes() and tgt in G.nodes():
            G.add_edge(src, tgt, type=dep_type)

    # Find recursive patterns (self-loops and mutual recursion)
    self_recursive = [n for n in G.nodes() if G.has_edge(n, n)]

    # Mutual recursion via cycles of length 2
    mutual_recursive = []
    for node in G.nodes():
        for succ in G.successors(node):
            if G.has_edge(succ, node) and node < succ:  # Avoid duplicates
                mutual_recursive.append((node, succ))

    # Analyze call depth (stack depth analog)
    # Find longest paths (potential maximum stack depth)
    try:
        if nx.is_directed_acyclic_graph(G):
            longest_path_length = nx.dag_longest_path_length(G)
            longest_path = nx.dag_longest_path(G)
        else:
            # For cyclic graphs, find longest simple path (approximation)
            longest_path_length = 0
            longest_path = []
            for source in [n for n in G.nodes() if G.in_degree(n) == 0][:10]:
                for target in [n for n in G.nodes() if G.out_degree(n) == 0][:10]:
                    try:
                        for path in nx.all_simple_paths(G, source, target, cutoff=5):
                            if len(path) > longest_path_length:
                                longest_path_length = len(path)
                                longest_path = path
                    except:
                        pass
    except:
        longest_path_length = 0
        longest_path = []

    # Check for "balanced" structures
    # In layered architecture, calls should go down layers and return up
    layer_violations = []
    layers_by_node = {n: G.nodes[n].get('layer', '') for n in G.nodes()}

    layer_order = {}
    for i, layer in enumerate(sorted(set(layers_by_node.values()))):
        layer_order[layer] = i

    for u, v in G.edges():
        u_layer = layers_by_node.get(u, '')
        v_layer = layers_by_node.get(v, '')

        if u_layer and v_layer and u_layer in layer_order and v_layer in layer_order:
            if layer_order[u_layer] > layer_order[v_layer]:
                layer_violations.append({
                    "from": u,
                    "to": v,
                    "from_layer": u_layer,
                    "to_layer": v_layer
                })

    # Analyze "nesting depth" via dominator tree
    entry_points = [n for n in G.nodes() if G.in_degree(n) == 0]
    max_dominator_depth = 0

    if entry_points:
        try:
            for entry in entry_points[:3]:
                dominators = nx.immediate_dominators(G, entry)
                # Build dominator tree
                dom_tree = nx.DiGraph()
                for node, dom in dominators.items():
                    if node != dom:
                        dom_tree.add_edge(dom, node)

                if dom_tree.number_of_nodes() > 0:
                    depths = nx.single_source_shortest_path_length(dom_tree, entry)
                    max_depth = max(depths.values()) if depths else 0
                    max_dominator_depth = max(max_dominator_depth, max_depth)
        except:
            pass

    max_depth_threshold = getattr(config, 'max_call_depth', 10)
    max_recursion = getattr(config, 'max_recursive_patterns', 5)

    total_recursive = len(self_recursive) + len(mutual_recursive)
    valid = longest_path_length <= max_depth_threshold and total_recursive <= max_recursion

    return {
        "name": "pushdown_patterns",
        "status": "WARNING" if not valid else "INFO",
        "reason": f"Pushdown analysis: depth={longest_path_length}, " +
                f"recursive={total_recursive}, layer_violations={len(layer_violations)}" +
                ("" if valid else " - complexity concerns"),
        
        "details": {
            "max_call_depth": longest_path_length,
            "longest_path": longest_path[:10],
            "self_recursive": self_recursive[:10],
            "mutual_recursive": mutual_recursive[:10],
            "dominator_depth": max_dominator_depth,
            "layer_violations": layer_violations[:10]
        }
    }


def validate_computability_bounds(graph: nx.DiGraph, config: Any = None) -> Dict[str, Any]:
    """
    Analyze computability and complexity bounds.

    Based on Turing machine concepts:
    - Halting analysis (termination guarantees)
    - Resource bounds (space/time complexity)
    - Decidability of properties

    For architecture:
    - Identifies potentially non-terminating patterns
    - Bounds on complexity growth
    - Predictability of behavior
    """
    components = [{"name": n, **graph.nodes[n]} for n in graph.nodes()]
    dependencies = [{"from": u, "to": v, **graph.edges[u,v]} for u,v in graph.edges()]

    if len(components) < 2:
        return {
            "name": "computability_bounds",
            "status": "INFO",
            "reason": "Insufficient components for computability analysis",
            
            "details": {"components": len(components)}
        }

    # Build graph
    G = nx.DiGraph()

    for comp in components:
        name = comp.get("name", "")
        if name:
            G.add_node(name)

    for dep in dependencies:
        src = dep.get("from", dep.get("source", ""))
        tgt = dep.get("to", dep.get("target", ""))
        if src and tgt and src in G.nodes() and tgt in G.nodes():
            G.add_edge(src, tgt)

    # Halting analysis: cycles indicate potential non-termination
    from validator.utils.cycles import simple_cycles_bounded
    try:
        cycles = list(simple_cycles_bounded(G, max_length=10))
    except:
        cycles = []

    has_cycles = len(cycles) > 0

    # Classify cycles by "escape potential"
    cycles_with_exit = 0
    cycles_without_exit = 0

    for cycle in cycles[:20]:  # Sample
        cycle_set = set(cycle)
        has_exit = False

        for node in cycle:
            for succ in G.successors(node):
                if succ not in cycle_set:
                    has_exit = True
                    break
            if has_exit:
                break

        if has_exit:
            cycles_with_exit += 1
        else:
            cycles_without_exit += 1

    # Complexity analysis: graph density and growth
    n = G.number_of_nodes()
    m = G.number_of_edges()

    if n > 1:
        density = m / (n * (n - 1))
    else:
        density = 0

    # Check for "exponential blowup" patterns
    # Nodes with high fan-out create exponential paths
    fan_outs = dict(G.out_degree())
    high_fan_out = [n for n, d in fan_outs.items() if d > 5]

    # Calculate approximate path count growth
    # Simplified: product of fan-outs along longest path
    path_growth_factor = 1
    try:
        if nx.is_directed_acyclic_graph(G):
            longest_path = nx.dag_longest_path(G)
            for node in longest_path:
                path_growth_factor *= max(1, G.out_degree(node))
    except:
        pass

    # Resource bounds estimation
    # Based on structural complexity
    cyclomatic_complexity = m - n + 2 * len(list(nx.weakly_connected_components(G)))

    # Decidability concerns
    decidability_issues = []

    if cycles_without_exit > 0:
        decidability_issues.append(f"Cycles without exits ({cycles_without_exit})")

    if path_growth_factor > 1000:
        decidability_issues.append(f"High path growth factor ({path_growth_factor})")

    if cyclomatic_complexity > n * 2:
        decidability_issues.append(f"High cyclomatic complexity ({cyclomatic_complexity})")

    valid = len(decidability_issues) == 0

    return {
        "name": "computability_bounds",
        "status": "WARNING" if not valid else "INFO",
        "reason": f"Computability: cyclomatic={cyclomatic_complexity}, " +
                f"{'cyclic' if has_cycles else 'acyclic'}, " +
                f"density={density:.3f}" +
                ("" if valid else f", {len(decidability_issues)} concerns"),
        
        "details": {
            "has_cycles": has_cycles,
            "total_cycles": len(cycles),
            "cycles_with_exit": cycles_with_exit,
            "cycles_without_exit": cycles_without_exit,
            "cyclomatic_complexity": cyclomatic_complexity,
            "density": density,
            "path_growth_factor": path_growth_factor,
            "high_fan_out_nodes": high_fan_out[:10],
            "decidability_issues": decidability_issues
        }
    }


def validate_bisimulation(graph: nx.DiGraph, config: Any = None) -> Dict[str, Any]:
    """
    Analyze bisimulation equivalence in architecture.

    Bisimulation is a relation R where:
    - If (p, q) ∈ R and p →a p', then ∃q' such that q →a q' and (p', q') ∈ R

    For architecture:
    - Components with same "behavior" (transitions)
    - Identifies redundant components
    - Checks behavioral consistency

    Used in process algebras and model checking.
    """
    components = [{"name": n, **graph.nodes[n]} for n in graph.nodes()]
    dependencies = [{"from": u, "to": v, **graph.edges[u,v]} for u,v in graph.edges()]

    if len(components) < 2:
        return {
            "name": "bisimulation",
            "status": "INFO",
            "reason": "Insufficient components for bisimulation analysis",
            
            "details": {"components": len(components)}
        }

    # Build labeled transition system
    G = nx.DiGraph()

    for comp in components:
        name = comp.get("name", "")
        comp_type = comp.get("type", "")
        if name:
            G.add_node(name, type=comp_type)

    for dep in dependencies:
        src = dep.get("from", dep.get("source", ""))
        tgt = dep.get("to", dep.get("target", ""))
        label = dep.get("type", "default")
        if src and tgt and src in G.nodes() and tgt in G.nodes():
            G.add_edge(src, tgt, label=label)

    nodes = list(G.nodes())

    # Compute behavioral signature for each node
    # Signature: (outgoing labels sorted, incoming labels sorted, successor types, predecessor types)
    def get_signature(node, depth=2):
        if depth == 0:
            return (G.nodes[node].get('type', ''),)

        out_labels = tuple(sorted([G.edges[node, succ]['label'] for succ in G.successors(node)]))
        in_labels = tuple(sorted([G.edges[pred, node]['label'] for pred in G.predecessors(node)]))

        # Recursive signature of neighbors (limited depth)
        succ_sigs = tuple(sorted([get_signature(s, depth-1) for s in G.successors(node)]))
        pred_sigs = tuple(sorted([get_signature(p, depth-1) for p in G.predecessors(node)]))

        return (G.nodes[node].get('type', ''), out_labels, in_labels, succ_sigs[:3], pred_sigs[:3])

    # Group by signature (bisimilar candidates)
    signatures = defaultdict(list)
    for node in nodes:
        sig = get_signature(node)
        signatures[sig].append(node)

    # Find bisimulation classes
    bisimulation_classes = {i: nodes for i, (sig, nodes) in enumerate(signatures.items())}

    # Classes with multiple members are potentially redundant
    redundant_classes = {k: v for k, v in bisimulation_classes.items() if len(v) > 1}

    # Verify bisimulation by checking transition matching
    verified_bisimilar = []

    for class_id, class_nodes in redundant_classes.items():
        if len(class_nodes) == 2:
            n1, n2 = class_nodes

            # Check if they have matching transitions
            n1_out = {(G.edges[n1, s]['label'], s) for s in G.successors(n1)}
            n2_out = {(G.edges[n2, s]['label'], s) for s in G.successors(n2)}

            # Check if labels match (targets may differ but should be bisimilar too)
            n1_labels = {label for label, _ in n1_out}
            n2_labels = {label for label, _ in n2_out}

            if n1_labels == n2_labels:
                verified_bisimilar.append((n1, n2))

    # Calculate bisimulation quotient size
    quotient_size = len(signatures)
    original_size = len(nodes)
    reduction_ratio = quotient_size / original_size if original_size > 0 else 1.0

    # Low reduction ratio indicates high redundancy
    min_reduction = getattr(config, 'min_bisimulation_reduction', 0.5)
    valid = reduction_ratio >= min_reduction or len(redundant_classes) == 0

    return {
        "name": "bisimulation",
        "status": "WARNING" if not valid else "INFO",
        "reason": f"Bisimulation: {quotient_size}/{original_size} equivalence classes " +
                f"(reduction {reduction_ratio:.1%}), {len(verified_bisimilar)} bisimilar pairs" +
                ("" if valid else " - high redundancy"),
        
        "details": {
            "original_size": original_size,
            "quotient_size": quotient_size,
            "reduction_ratio": reduction_ratio,
            "redundant_class_count": len(redundant_classes),
            "verified_bisimilar_pairs": verified_bisimilar[:10],
            "largest_equivalence_class": max((len(v) for v in signatures.values()), default=0)
        }
    }
