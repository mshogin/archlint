"""
Optimization Theory Metrics for Software Architecture Validation.

Applies optimization theory concepts:
1. Network Flow - max-flow/min-cut analysis
2. Linear Programming - capacity and resource optimization
3. Convex Optimization - structural convexity properties
4. Submodular Functions - diminishing returns analysis
5. Facility Location - optimal placement problems
6. Matching Theory - optimal pairing and allocation

Mathematical foundations:
- Max-Flow Min-Cut Theorem: max flow = min cut capacity
- LP Duality: primal-dual relationships
- Submodularity: f(A ∪ {x}) - f(A) ≥ f(B ∪ {x}) - f(B) for A ⊆ B
- Hungarian algorithm for optimal matching
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


def validate_max_flow_min_cut(graph: nx.DiGraph, config: Any = None) -> Dict[str, Any]:
    """
    Analyze max-flow/min-cut properties of architecture.

    Max-Flow Min-Cut Theorem states that the maximum flow from source to sink
    equals the minimum cut capacity separating them.

    For architecture:
    - Identifies bottlenecks (min-cut edges)
    - Measures data flow capacity
    - Detects congestion points
    """
    components = [{"name": n, **graph.nodes[n]} for n in graph.nodes()]
    dependencies = [{"from": u, "to": v, **graph.edges[u,v]} for u,v in graph.edges()]

    if len(components) < 2:
        return {
            "name": "max_flow_min_cut",
            "status": "INFO",
            "reason": "Insufficient components for flow analysis",
            
            "details": {"components": len(components)}
        }

    # Build directed graph with capacities
    G = nx.DiGraph()

    for comp in components:
        name = comp.get("name", "")
        if name:
            G.add_node(name)

    for dep in dependencies:
        src = dep.get("from", dep.get("source", ""))
        tgt = dep.get("to", dep.get("target", ""))
        if src and tgt and src in G.nodes() and tgt in G.nodes():
            # Capacity based on dependency type
            dep_type = dep.get("type", "uses")
            capacity = {"extends": 10, "implements": 10, "uses": 5, "calls": 3}.get(dep_type, 1)
            G.add_edge(src, tgt, capacity=capacity)

    if G.number_of_edges() == 0:
        return {
            "name": "max_flow_min_cut",
            "status": "INFO",
            "reason": "No dependencies for flow analysis",
            
            "details": {}
        }

    # Find potential sources (no incoming) and sinks (no outgoing)
    sources = [n for n in G.nodes() if G.in_degree(n) == 0]
    sinks = [n for n in G.nodes() if G.out_degree(n) == 0]

    flow_analysis = []
    bottlenecks = []

    # Analyze flows between source-sink pairs
    for source in sources[:3]:  # Limit analysis
        for sink in sinks[:3]:
            if source != sink and nx.has_path(G, source, sink):
                try:
                    flow_value, flow_dict = nx.maximum_flow(G, source, sink)
                    cut_value, partition = nx.minimum_cut(G, source, sink)

                    # Find cut edges (bottlenecks)
                    reachable, non_reachable = partition
                    cut_edges = []
                    for u in reachable:
                        for v in G.successors(u):
                            if v in non_reachable:
                                cut_edges.append((u, v))

                    flow_analysis.append({
                        "source": source,
                        "sink": sink,
                        "max_flow": flow_value,
                        "min_cut": cut_value,
                        "cut_edges": cut_edges[:5]
                    })

                    bottlenecks.extend(cut_edges)
                except nx.NetworkXError:
                    pass

    # Count bottleneck frequency
    bottleneck_freq = defaultdict(int)
    for edge in bottlenecks:
        bottleneck_freq[edge] += 1

    critical_bottlenecks = [edge for edge, count in bottleneck_freq.items() if count > 1]

    threshold = getattr(config, 'max_critical_bottlenecks', 3)
    valid = len(critical_bottlenecks) <= threshold

    return {
        "name": "max_flow_min_cut",
        "status": "WARNING" if not valid else "INFO",
        "reason": f"Flow analysis: {len(flow_analysis)} paths, {len(critical_bottlenecks)} critical bottlenecks" +
                ("" if valid else f" (exceeds threshold {threshold})"),
        
        "details": {
            "flow_paths": flow_analysis[:5],
            "critical_bottlenecks": critical_bottlenecks[:10],
            "bottleneck_frequency": dict(list(bottleneck_freq.items())[:10])
        }
    }


def validate_resource_allocation(graph: nx.DiGraph, config: Any = None) -> Dict[str, Any]:
    """
    Analyze resource allocation as linear programming problem.

    Models architecture as resource allocation:
    - Components as consumers
    - Dependencies as resource flows
    - Optimizes for balanced distribution

    Uses LP concepts:
    - Objective: minimize max load
    - Constraints: capacity limits
    """
    components = [{"name": n, **graph.nodes[n]} for n in graph.nodes()]
    dependencies = [{"from": u, "to": v, **graph.edges[u,v]} for u,v in graph.edges()]

    if not components:
        return {
            "name": "resource_allocation",
            "status": "INFO",
            "reason": "No components for resource analysis",
            
            "details": {}
        }

    # Calculate load on each component
    component_load = defaultdict(float)

    for dep in dependencies:
        tgt = dep.get("to", dep.get("target", ""))
        if tgt:
            # Weight by dependency type
            dep_type = dep.get("type", "uses")
            weight = {"extends": 3.0, "implements": 2.5, "uses": 1.0, "calls": 1.5}.get(dep_type, 1.0)
            component_load[tgt] += weight

    # Initialize all components
    for comp in components:
        name = comp.get("name", "")
        if name and name not in component_load:
            component_load[name] = 0

    loads = list(component_load.values())

    if not loads:
        return {
            "name": "resource_allocation",
            "status": "INFO",
            "reason": "No load data available",
            
            "details": {}
        }

    max_load = max(loads)
    min_load = min(loads)
    avg_load = sum(loads) / len(loads)

    # Load imbalance ratio
    if avg_load > 0:
        imbalance_ratio = max_load / avg_load
    else:
        imbalance_ratio = 1.0

    # Identify overloaded components
    threshold_multiplier = getattr(config, 'load_threshold_multiplier', 3.0)
    overloaded = [name for name, load in component_load.items()
                  if avg_load > 0 and load > avg_load * threshold_multiplier]

    # Calculate Gini coefficient for load distribution
    sorted_loads = sorted(loads)
    n = len(sorted_loads)
    if n > 1 and sum(sorted_loads) > 0:
        cumulative = np.cumsum(sorted_loads)
        gini = (2 * np.sum((np.arange(1, n + 1) * sorted_loads))) / (n * np.sum(sorted_loads)) - (n + 1) / n
        gini = max(0, min(1, gini))
    else:
        gini = 0

    max_imbalance = getattr(config, 'max_load_imbalance', 5.0)
    valid = imbalance_ratio <= max_imbalance and len(overloaded) == 0

    return {
        "name": "resource_allocation",
        "status": "WARNING" if not valid else "INFO",
        "reason": f"Load analysis: max={max_load:.1f}, avg={avg_load:.1f}, imbalance={imbalance_ratio:.2f}, " +
                f"Gini={gini:.3f}" + ("" if valid else f", {len(overloaded)} overloaded"),
        
        "details": {
            "max_load": max_load,
            "min_load": min_load,
            "avg_load": avg_load,
            "imbalance_ratio": imbalance_ratio,
            "gini_coefficient": gini,
            "overloaded_components": overloaded[:10],
            "load_distribution": dict(sorted(component_load.items(), key=lambda x: -x[1])[:15])
        }
    }


def validate_convex_structure(graph: nx.DiGraph, config: Any = None) -> Dict[str, Any]:
    """
    Analyze convex structure properties of architecture graph.

    A subgraph is convex if for any two vertices in it, all shortest paths
    between them lie entirely within the subgraph.

    Convex modules indicate:
    - Well-encapsulated functionality
    - Clean interfaces
    - No leaky abstractions
    """
    components = [{"name": n, **graph.nodes[n]} for n in graph.nodes()]
    dependencies = [{"from": u, "to": v, **graph.edges[u,v]} for u,v in graph.edges()]

    if len(components) < 3:
        return {
            "name": "convex_structure",
            "status": "INFO",
            "reason": "Insufficient components for convexity analysis",
            
            "details": {"components": len(components)}
        }

    # Build undirected graph
    G = nx.Graph()

    for comp in components:
        name = comp.get("name", "")
        layer = comp.get("layer", "")
        if name:
            G.add_node(name, layer=layer)

    for dep in dependencies:
        src = dep.get("from", dep.get("source", ""))
        tgt = dep.get("to", dep.get("target", ""))
        if src and tgt and src in G.nodes() and tgt in G.nodes():
            G.add_edge(src, tgt)

    # Group by layers
    layers = defaultdict(set)
    for node in G.nodes():
        layer = G.nodes[node].get('layer', 'default')
        layers[layer].add(node)

    # Check convexity of each layer
    convexity_results = {}
    non_convex_layers = []

    for layer_name, layer_nodes in layers.items():
        if len(layer_nodes) < 2:
            convexity_results[layer_name] = {"convex": True, "size": len(layer_nodes)}
            continue

        # Check if subgraph induced by layer is convex
        layer_list = list(layer_nodes)
        is_convex = True
        violations = []

        for i, u in enumerate(layer_list):
            for v in layer_list[i+1:]:
                if nx.has_path(G, u, v):
                    # Get all shortest paths
                    try:
                        for path in nx.all_shortest_paths(G, u, v):
                            path_set = set(path)
                            outside = path_set - layer_nodes
                            if outside:
                                is_convex = False
                                violations.append({
                                    "from": u,
                                    "to": v,
                                    "outside_nodes": list(outside)
                                })
                                break
                    except nx.NetworkXNoPath:
                        pass
                if not is_convex:
                    break
            if not is_convex:
                break

        convexity_results[layer_name] = {
            "convex": is_convex,
            "size": len(layer_nodes),
            "violations": violations[:3] if violations else []
        }

        if not is_convex:
            non_convex_layers.append(layer_name)

    # Calculate convexity ratio
    total_layers = len(layers)
    convex_layers = sum(1 for r in convexity_results.values() if r["convex"])
    convexity_ratio = convex_layers / total_layers if total_layers > 0 else 1.0

    min_ratio = getattr(config, 'min_convexity_ratio', 0.7)
    valid = convexity_ratio >= min_ratio

    return {
        "name": "convex_structure",
        "status": "WARNING" if not valid else "INFO",
        "reason": f"Convexity: {convex_layers}/{total_layers} layers convex ({convexity_ratio:.1%})" +
                ("" if valid else f" (below {min_ratio:.1%})"),
        
        "details": {
            "convexity_ratio": convexity_ratio,
            "convex_layers": convex_layers,
            "total_layers": total_layers,
            "non_convex_layers": non_convex_layers,
            "layer_analysis": convexity_results
        }
    }


def validate_submodular_functions(graph: nx.DiGraph, config: Any = None) -> Dict[str, Any]:
    """
    Analyze submodular function properties in architecture.

    A function f is submodular if: f(A ∪ {x}) - f(A) ≥ f(B ∪ {x}) - f(B) for A ⊆ B

    This captures "diminishing returns" property.

    For architecture:
    - Adding components has diminishing marginal benefit
    - Indicates good modularity
    - Helps identify optimal module boundaries
    """
    components = [{"name": n, **graph.nodes[n]} for n in graph.nodes()]
    dependencies = [{"from": u, "to": v, **graph.edges[u,v]} for u,v in graph.edges()]

    if len(components) < 3:
        return {
            "name": "submodular_functions",
            "status": "INFO",
            "reason": "Insufficient components for submodularity analysis",
            
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

    nodes = list(G.nodes())

    # Define coverage function: f(S) = number of nodes reachable from S
    def coverage(node_set):
        reachable = set()
        for node in node_set:
            reachable.add(node)
            reachable.update(nx.descendants(G, node))
        return len(reachable)

    # Test submodularity by sampling
    submodular_violations = []
    tests_performed = 0

    # Sample pairs A ⊂ B and elements x
    import random
    random.seed(42)

    sample_size = min(50, len(nodes) * 5)

    for _ in range(sample_size):
        if len(nodes) < 3:
            break

        # Choose random A, B where A ⊂ B
        a_size = random.randint(1, max(1, len(nodes) // 3))
        b_size = random.randint(a_size + 1, max(a_size + 1, len(nodes) - 1))

        A = set(random.sample(nodes, a_size))
        remaining = [n for n in nodes if n not in A]
        if len(remaining) < b_size - a_size + 1:
            continue

        B_extra = set(random.sample(remaining, b_size - a_size))
        B = A | B_extra

        remaining_for_x = [n for n in nodes if n not in B]
        if not remaining_for_x:
            continue

        x = random.choice(remaining_for_x)

        tests_performed += 1

        # Check submodularity: f(A ∪ {x}) - f(A) ≥ f(B ∪ {x}) - f(B)
        f_A = coverage(A)
        f_A_x = coverage(A | {x})
        f_B = coverage(B)
        f_B_x = coverage(B | {x})

        marginal_A = f_A_x - f_A
        marginal_B = f_B_x - f_B

        if marginal_A < marginal_B:
            submodular_violations.append({
                "A_size": len(A),
                "B_size": len(B),
                "x": x,
                "marginal_A": marginal_A,
                "marginal_B": marginal_B
            })

    # Calculate submodularity score
    if tests_performed > 0:
        submodularity_ratio = 1 - len(submodular_violations) / tests_performed
    else:
        submodularity_ratio = 1.0

    min_ratio = getattr(config, 'min_submodularity_ratio', 0.8)
    valid = submodularity_ratio >= min_ratio

    return {
        "name": "submodular_functions",
        "status": "WARNING" if not valid else "INFO",
        "reason": f"Submodularity: {submodularity_ratio:.1%} ({tests_performed - len(submodular_violations)}/{tests_performed} tests passed)" +
                ("" if valid else f" (below {min_ratio:.1%})"),
        
        "details": {
            "submodularity_ratio": submodularity_ratio,
            "tests_performed": tests_performed,
            "violations": len(submodular_violations),
            "sample_violations": submodular_violations[:5]
        }
    }


def validate_facility_location(graph: nx.DiGraph, config: Any = None) -> Dict[str, Any]:
    """
    Analyze architecture as facility location problem.

    Models:
    - "Facilities" as core/service components
    - "Clients" as dependent components
    - Optimal placement minimizes total distance

    Helps identify:
    - Central services that should exist
    - Suboptimal placement of shared functionality
    - Redundant facilities
    """
    components = [{"name": n, **graph.nodes[n]} for n in graph.nodes()]
    dependencies = [{"from": u, "to": v, **graph.edges[u,v]} for u,v in graph.edges()]

    if len(components) < 3:
        return {
            "name": "facility_location",
            "status": "INFO",
            "reason": "Insufficient components for facility location analysis",
            
            "details": {"components": len(components)}
        }

    # Build graph
    G = nx.DiGraph()

    for comp in components:
        name = comp.get("name", "")
        comp_type = comp.get("type", "")
        if name:
            G.add_node(name, type=comp_type)

    for dep in dependencies:
        src = dep.get("from", dep.get("source", ""))
        tgt = dep.get("to", dep.get("target", ""))
        if src and tgt and src in G.nodes() and tgt in G.nodes():
            G.add_edge(src, tgt)

    # Identify "facilities" (highly used components)
    in_degrees = dict(G.in_degree())
    avg_in_degree = sum(in_degrees.values()) / len(in_degrees) if in_degrees else 0

    facilities = [n for n, d in in_degrees.items() if d >= avg_in_degree * 2]
    clients = [n for n in G.nodes() if n not in facilities]

    if not facilities:
        # Use top quartile as facilities
        sorted_by_degree = sorted(in_degrees.items(), key=lambda x: -x[1])
        k = max(1, len(sorted_by_degree) // 4)
        facilities = [n for n, _ in sorted_by_degree[:k]]
        clients = [n for n in G.nodes() if n not in facilities]

    # Calculate "assignment" - each client to nearest facility
    G_undirected = G.to_undirected()

    assignments = {}
    total_distance = 0
    unassigned = []

    for client in clients:
        min_dist = float('inf')
        nearest_facility = None

        for facility in facilities:
            try:
                dist = nx.shortest_path_length(G_undirected, client, facility)
                if dist < min_dist:
                    min_dist = dist
                    nearest_facility = facility
            except nx.NetworkXNoPath:
                pass

        if nearest_facility:
            assignments[client] = {
                "facility": nearest_facility,
                "distance": min_dist
            }
            total_distance += min_dist
        else:
            unassigned.append(client)

    # Calculate metrics
    avg_distance = total_distance / len(assignments) if assignments else 0

    # Calculate facility load (how many clients each serves)
    facility_load = defaultdict(int)
    for assignment in assignments.values():
        facility_load[assignment["facility"]] += 1

    # Check for load imbalance
    if facility_load:
        max_facility_load = max(facility_load.values())
        min_facility_load = min(facility_load.values())
        avg_facility_load = sum(facility_load.values()) / len(facility_load)
    else:
        max_facility_load = min_facility_load = avg_facility_load = 0

    max_avg_distance = getattr(config, 'max_avg_facility_distance', 3.0)
    valid = avg_distance <= max_avg_distance and len(unassigned) == 0

    return {
        "name": "facility_location",
        "status": "WARNING" if not valid else "INFO",
        "reason": f"Facility location: {len(facilities)} facilities serving {len(clients)} clients, " +
                f"avg distance={avg_distance:.2f}" +
                ("" if valid else f" (exceeds {max_avg_distance})"),
        
        "details": {
            "num_facilities": len(facilities),
            "num_clients": len(clients),
            "avg_distance": avg_distance,
            "total_distance": total_distance,
            "facilities": facilities[:10],
            "facility_load": dict(facility_load),
            "unassigned_clients": unassigned[:10]
        }
    }


def validate_optimal_matching(graph: nx.DiGraph, config: Any = None) -> Dict[str, Any]:
    """
    Analyze optimal matching properties in architecture.

    Uses matching theory to find:
    - Optimal pairing of interfaces to implementations
    - Balanced producer-consumer relationships
    - Maximum cardinality matching

    Based on Hall's theorem and Hungarian algorithm concepts.
    """
    components = [{"name": n, **graph.nodes[n]} for n in graph.nodes()]
    dependencies = [{"from": u, "to": v, **graph.edges[u,v]} for u,v in graph.edges()]

    if len(components) < 2:
        return {
            "name": "optimal_matching",
            "status": "INFO",
            "reason": "Insufficient components for matching analysis",
            
            "details": {"components": len(components)}
        }

    # Build bipartite graph from implements/uses relationships
    G = nx.Graph()

    interfaces = set()
    implementations = set()

    for comp in components:
        name = comp.get("name", "")
        comp_type = comp.get("type", "").lower()
        if name:
            if "interface" in comp_type or "abstract" in comp_type or "contract" in comp_type:
                interfaces.add(name)
                G.add_node(name, bipartite=0)
            else:
                implementations.add(name)
                G.add_node(name, bipartite=1)

    # If no clear separation, use dependency direction
    if not interfaces:
        for dep in dependencies:
            src = dep.get("from", dep.get("source", ""))
            tgt = dep.get("to", dep.get("target", ""))
            dep_type = dep.get("type", "")

            if dep_type in ["implements", "extends"]:
                interfaces.add(tgt)
                implementations.discard(tgt)
                if src not in interfaces:
                    implementations.add(src)

    # Add edges
    for dep in dependencies:
        src = dep.get("from", dep.get("source", ""))
        tgt = dep.get("to", dep.get("target", ""))

        if src in implementations and tgt in interfaces:
            G.add_edge(src, tgt)
        elif src in interfaces and tgt in implementations:
            G.add_edge(tgt, src)

    # Find maximum matching
    if interfaces and implementations:
        try:
            matching = nx.max_weight_matching(G, maxcardinality=True)
            matching_size = len(matching)
        except:
            matching = set()
            matching_size = 0
    else:
        matching = set()
        matching_size = 0

    # Calculate metrics
    max_possible = min(len(interfaces), len(implementations))
    matching_ratio = matching_size / max_possible if max_possible > 0 else 1.0

    # Find unmatched nodes
    matched_nodes = set()
    for edge in matching:
        matched_nodes.add(edge[0])
        matched_nodes.add(edge[1])

    unmatched_interfaces = [n for n in interfaces if n not in matched_nodes and n in G.nodes()]
    unmatched_implementations = [n for n in implementations if n not in matched_nodes and n in G.nodes()]

    # Check Hall's condition (existence of perfect matching)
    # For each subset S of interfaces, |N(S)| ≥ |S|
    halls_violations = []
    if len(interfaces) <= 10:  # Only check for small sets
        from itertools import combinations
        for size in range(1, min(4, len(interfaces) + 1)):
            for subset in combinations(list(interfaces)[:10], size):
                subset_set = set(subset)
                neighbors = set()
                for node in subset_set:
                    if node in G:
                        neighbors.update(G.neighbors(node))
                if len(neighbors) < len(subset_set):
                    halls_violations.append({
                        "subset": list(subset_set),
                        "neighbors": len(neighbors)
                    })

    min_ratio = getattr(config, 'min_matching_ratio', 0.8)
    valid = matching_ratio >= min_ratio

    return {
        "name": "optimal_matching",
        "status": "WARNING" if not valid else "INFO",
        "reason": f"Matching: {matching_size}/{max_possible} " +
                f"({matching_ratio:.1%} coverage)" +
                ("" if valid else f" (below {min_ratio:.1%})"),
        
        "details": {
            "matching_size": matching_size,
            "max_possible": max_possible,
            "matching_ratio": matching_ratio,
            "interfaces": len(interfaces),
            "implementations": len(implementations),
            "unmatched_interfaces": unmatched_interfaces[:5],
            "unmatched_implementations": unmatched_implementations[:5],
            "halls_violations": halls_violations[:3]
        }
    }
