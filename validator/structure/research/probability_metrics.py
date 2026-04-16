"""
Probability Theory Metrics for Software Architecture Validation.

Applies probability and statistics concepts:
1. Random Walk Analysis - stochastic navigation patterns
2. Markov Chain Properties - stationary distributions
3. Entropy Measures - information content
4. Concentration Inequalities - deviation bounds
5. Bayesian Analysis - conditional dependencies
6. Stochastic Processes - temporal patterns

Mathematical foundations:
- Ergodic theorem for Markov chains
- Shannon entropy H = -Σp log p
- Chebyshev/Chernoff bounds
- Bayes' theorem P(A|B) = P(B|A)P(A)/P(B)
"""

from typing import Any, Dict, List
import networkx as nx
from collections import defaultdict
import numpy as np
import math


def _is_excluded(node: str, exclude: List[str]) -> bool:
    """Check if node matches any exclusion pattern."""
    for pattern in exclude:
        if pattern in node:
            return True
    return False


def validate_random_walk(graph: nx.DiGraph, config: Any = None) -> Dict[str, Any]:
    """
    Analyze random walk properties on architecture graph.

    Random walks model:
    - User navigation patterns
    - Request propagation
    - Fault cascading probability

    Metrics:
    - Hitting probabilities
    - Expected return times
    - Mixing time
    """
    components = [{"name": n, **graph.nodes[n]} for n in graph.nodes()]
    dependencies = [{"from": u, "to": v, **graph.edges[u,v]} for u,v in graph.edges()]

    if len(components) < 2:
        return {
            "name": "random_walk",
            "status": "INFO",
            "reason": "Insufficient components for random walk analysis",
            
            "details": {"components": len(components)}
        }

    # Build directed graph
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

    if G.number_of_edges() == 0:
        return {
            "name": "random_walk",
            "status": "INFO",
            "reason": "No edges for random walk",
            
            "details": {}
        }

    # Build transition matrix
    nodes = list(G.nodes())
    n = len(nodes)
    node_idx = {node: i for i, node in enumerate(nodes)}

    # Transition probabilities (uniform over outgoing edges)
    P = np.zeros((n, n))
    for node in nodes:
        out_degree = G.out_degree(node)
        if out_degree > 0:
            prob = 1.0 / out_degree
            for succ in G.successors(node):
                P[node_idx[node], node_idx[succ]] = prob
        else:
            # Absorbing state or self-loop
            P[node_idx[node], node_idx[node]] = 1.0

    # Calculate stationary distribution (for strongly connected components)
    # Use power iteration
    pi = np.ones(n) / n
    for _ in range(100):
        pi_new = pi @ P
        if np.allclose(pi, pi_new):
            break
        pi = pi_new

    # Identify high-probability nodes (likely destinations)
    stationary_dist = {nodes[i]: pi[i] for i in range(n)}
    sorted_by_prob = sorted(stationary_dist.items(), key=lambda x: -x[1])

    # Calculate expected return times
    return_times = {}
    for i, node in enumerate(nodes):
        if pi[i] > 1e-10:
            return_times[node] = 1.0 / pi[i]
        else:
            return_times[node] = float('inf')

    # Identify "trap" states (hard to leave)
    trap_candidates = [n for n, rt in return_times.items() if rt < 2]

    # Calculate mixing time (approximate via spectral gap)
    try:
        eigenvalues = np.linalg.eigvals(P)
        eigenvalues_sorted = sorted(np.abs(eigenvalues), reverse=True)
        if len(eigenvalues_sorted) > 1:
            spectral_gap = 1 - eigenvalues_sorted[1].real
            if spectral_gap > 0:
                mixing_time_approx = 1.0 / spectral_gap
            else:
                mixing_time_approx = float('inf')
        else:
            spectral_gap = 1.0
            mixing_time_approx = 1.0
    except:
        spectral_gap = 0
        mixing_time_approx = float('inf')

    # High mixing time indicates poor accessibility
    max_mixing_time = getattr(config, 'max_mixing_time', 100)
    valid = mixing_time_approx < max_mixing_time

    return {
        "name": "random_walk",
        "status": "WARNING" if not valid else "INFO",
        "reason": f"Random walk: mixing_time≈{mixing_time_approx:.1f}, " +
                f"spectral_gap={spectral_gap:.3f}, {len(trap_candidates)} potential traps" +
                ("" if valid else f" (mixing too slow)"),
        
        "details": {
            "mixing_time": mixing_time_approx if mixing_time_approx != float('inf') else "infinite",
            "spectral_gap": spectral_gap,
            "top_stationary": dict(sorted_by_prob[:10]),
            "trap_candidates": trap_candidates[:10],
            "return_times_sample": dict(sorted(return_times.items(), key=lambda x: x[1])[:10])
        }
    }


def validate_markov_properties(graph: nx.DiGraph, config: Any = None) -> Dict[str, Any]:
    """
    Analyze Markov chain properties of architecture.

    Key properties:
    - Irreducibility (all states reachable)
    - Aperiodicity (no fixed return cycles)
    - Ergodicity (unique stationary distribution)
    - Reversibility (detailed balance)
    """
    components = [{"name": n, **graph.nodes[n]} for n in graph.nodes()]
    dependencies = [{"from": u, "to": v, **graph.edges[u,v]} for u,v in graph.edges()]

    if len(components) < 2:
        return {
            "name": "markov_properties",
            "status": "INFO",
            "reason": "Insufficient components for Markov analysis",

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

    # Check irreducibility via strongly connected components
    sccs = list(nx.strongly_connected_components(G))
    is_irreducible = len(sccs) == 1

    # Find the largest SCC (recurrent class)
    largest_scc = max(sccs, key=len) if sccs else set()
    recurrent_ratio = len(largest_scc) / len(G.nodes()) if G.nodes() else 0

    # Check aperiodicity
    # A Markov chain is aperiodic if GCD of cycle lengths is 1
    from validator.utils.cycles import simple_cycles_bounded
    try:
        cycles = list(simple_cycles_bounded(G, max_length=10))
        if cycles:
            cycle_lengths = [len(c) for c in cycles]
            overall_gcd = cycle_lengths[0]
            for length in cycle_lengths[1:]:
                overall_gcd = math.gcd(overall_gcd, length)
            is_aperiodic = overall_gcd == 1
            period = overall_gcd
        else:
            is_aperiodic = True  # Acyclic is considered aperiodic
            period = 1
    except:
        is_aperiodic = True
        period = 1

    # Check ergodicity (irreducible + aperiodic)
    is_ergodic = is_irreducible and is_aperiodic

    # Check reversibility (detailed balance)
    # For undirected interpretation
    reversible_edges = 0
    total_edges = G.number_of_edges()

    for u, v in G.edges():
        if G.has_edge(v, u):
            reversible_edges += 1

    reversibility_ratio = reversible_edges / total_edges if total_edges > 0 else 1.0

    # Classify states
    transient_states = []
    recurrent_states = list(largest_scc)

    for scc in sccs:
        if scc != largest_scc:
            transient_states.extend(scc)

    # Absorbing states
    absorbing = [n for n in G.nodes() if G.out_degree(n) == 0]

    min_recurrent_ratio = getattr(config, 'min_recurrent_ratio', 0.5)
    valid = recurrent_ratio >= min_recurrent_ratio and is_aperiodic

    return {
        "name": "markov_properties",
        "status": "WARNING" if not valid else "INFO",
        "reason": f"Markov: {'ergodic' if is_ergodic else 'non-ergodic'}, " +
                f"period={period}, recurrent={recurrent_ratio:.1%}, " +
                f"reversibility={reversibility_ratio:.1%}" +
                ("" if valid else " - structural concerns"),
        
        "details": {
            "is_irreducible": is_irreducible,
            "is_aperiodic": is_aperiodic,
            "is_ergodic": is_ergodic,
            "period": period,
            "scc_count": len(sccs),
            "recurrent_ratio": recurrent_ratio,
            "recurrent_states": list(recurrent_states)[:15],
            "transient_states": transient_states[:10],
            "absorbing_states": absorbing[:10],
            "reversibility_ratio": reversibility_ratio
        }
    }


def validate_entropy_measures(graph: nx.DiGraph, config: Any = None) -> Dict[str, Any]:
    """
    Calculate entropy measures for architecture.

    Shannon entropy measures uncertainty/information content.
    H(X) = -Σ p(x) log p(x)

    For architecture:
    - Degree distribution entropy
    - Layer distribution entropy
    - Dependency type entropy
    - High entropy = diverse, Low entropy = concentrated
    """
    components = [{"name": n, **graph.nodes[n]} for n in graph.nodes()]
    dependencies = [{"from": u, "to": v, **graph.edges[u,v]} for u,v in graph.edges()]

    if not components:
        return {
            "name": "entropy_measures",
            "status": "INFO",
            "reason": "No components for entropy analysis",
            
            "details": {}
        }

    # Build graph
    G = nx.DiGraph()

    for comp in components:
        name = comp.get("name", "")
        layer = comp.get("layer", "")
        comp_type = comp.get("type", "")
        if name:
            G.add_node(name, layer=layer, type=comp_type)

    for dep in dependencies:
        src = dep.get("from", dep.get("source", ""))
        tgt = dep.get("to", dep.get("target", ""))
        dep_type = dep.get("type", "uses")
        if src and tgt and src in G.nodes() and tgt in G.nodes():
            G.add_edge(src, tgt, type=dep_type)

    def calc_entropy(values):
        """Calculate Shannon entropy from value list."""
        if not values:
            return 0.0
        counts = defaultdict(int)
        for v in values:
            counts[v] += 1
        total = len(values)
        entropy = 0.0
        for count in counts.values():
            p = count / total
            if p > 0:
                entropy -= p * math.log2(p)
        return entropy

    def calc_max_entropy(n_categories):
        """Maximum possible entropy for n categories."""
        if n_categories <= 1:
            return 0.0
        return math.log2(n_categories)

    # Degree distribution entropy
    in_degrees = [G.in_degree(n) for n in G.nodes()]
    out_degrees = [G.out_degree(n) for n in G.nodes()]

    in_degree_entropy = calc_entropy(in_degrees)
    out_degree_entropy = calc_entropy(out_degrees)

    # Layer distribution entropy
    layers = [G.nodes[n].get('layer', 'default') for n in G.nodes()]
    layer_entropy = calc_entropy(layers)
    max_layer_entropy = calc_max_entropy(len(set(layers)))
    layer_entropy_ratio = layer_entropy / max_layer_entropy if max_layer_entropy > 0 else 1.0

    # Component type entropy
    types = [G.nodes[n].get('type', 'default') for n in G.nodes()]
    type_entropy = calc_entropy(types)

    # Dependency type entropy
    dep_types = [G.edges[e].get('type', 'uses') for e in G.edges()]
    dep_entropy = calc_entropy(dep_types)
    max_dep_entropy = calc_max_entropy(len(set(dep_types)))
    dep_entropy_ratio = dep_entropy / max_dep_entropy if max_dep_entropy > 0 else 1.0

    # Joint entropy: (layer, type) pairs
    joint_pairs = [(G.nodes[n].get('layer', ''), G.nodes[n].get('type', ''))
                   for n in G.nodes()]
    joint_entropy = calc_entropy(joint_pairs)

    # Mutual information: I(layer; type) = H(layer) + H(type) - H(layer, type)
    mutual_info = layer_entropy + type_entropy - joint_entropy

    # Normalized mutual information
    nmi = 2 * mutual_info / (layer_entropy + type_entropy) if (layer_entropy + type_entropy) > 0 else 0

    # Entropy analysis
    min_entropy_ratio = getattr(config, 'min_entropy_ratio', 0.3)
    valid = layer_entropy_ratio >= min_entropy_ratio

    return {
        "name": "entropy_measures",
        "status": "WARNING" if not valid else "INFO",
        "reason": f"Entropy: layer={layer_entropy:.2f} ({layer_entropy_ratio:.1%}), " +
                f"deps={dep_entropy:.2f} ({dep_entropy_ratio:.1%}), NMI={nmi:.2f}" +
                ("" if valid else " - low diversity"),
        
        "details": {
            "in_degree_entropy": in_degree_entropy,
            "out_degree_entropy": out_degree_entropy,
            "layer_entropy": layer_entropy,
            "layer_entropy_ratio": layer_entropy_ratio,
            "type_entropy": type_entropy,
            "dependency_entropy": dep_entropy,
            "dependency_entropy_ratio": dep_entropy_ratio,
            "joint_entropy": joint_entropy,
            "mutual_information": mutual_info,
            "normalized_mutual_info": nmi
        }
    }


def validate_concentration_bounds(graph: nx.DiGraph, config: Any = None) -> Dict[str, Any]:
    """
    Apply concentration inequalities to architecture metrics.

    Concentration inequalities bound deviation from expected values:
    - Chebyshev: P(|X - μ| ≥ kσ) ≤ 1/k²
    - Chernoff: P(X ≥ (1+δ)μ) ≤ exp(-δ²μ/3)

    For architecture:
    - Detect outlier components
    - Bound worst-case scenarios
    - Identify abnormal patterns
    """
    components = [{"name": n, **graph.nodes[n]} for n in graph.nodes()]
    dependencies = [{"from": u, "to": v, **graph.edges[u,v]} for u,v in graph.edges()]

    if len(components) < 3:
        return {
            "name": "concentration_bounds",
            "status": "INFO",
            "reason": "Insufficient components for concentration analysis",
            
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

    # Analyze degree distribution
    in_degrees = np.array([G.in_degree(n) for n in G.nodes()])
    out_degrees = np.array([G.out_degree(n) for n in G.nodes()])
    total_degrees = in_degrees + out_degrees

    # Calculate statistics
    mean_degree = np.mean(total_degrees)
    std_degree = np.std(total_degrees)
    max_degree = np.max(total_degrees)

    # Chebyshev bound: components more than k std devs from mean
    k = 2
    chebyshev_bound = 1 / (k ** 2)  # Probability bound
    actual_outliers = []

    for i, node in enumerate(G.nodes()):
        if std_degree > 0:
            z_score = abs(total_degrees[i] - mean_degree) / std_degree
            if z_score > k:
                actual_outliers.append({
                    "node": node,
                    "degree": int(total_degrees[i]),
                    "z_score": z_score
                })

    outlier_ratio = len(actual_outliers) / len(components) if components else 0

    # Check if actual outliers exceed Chebyshev bound
    chebyshev_violated = outlier_ratio > chebyshev_bound

    # Chernoff-style analysis for high-degree nodes
    if mean_degree > 0:
        delta = (max_degree / mean_degree) - 1 if max_degree > mean_degree else 0
        # Chernoff bound: exp(-delta^2 * mu / 3)
        chernoff_bound = math.exp(-delta * delta * mean_degree / 3) if delta > 0 else 1.0
    else:
        delta = 0
        chernoff_bound = 1.0

    # Percentile analysis
    p95 = np.percentile(total_degrees, 95)
    p99 = np.percentile(total_degrees, 99)

    # Components in tail
    tail_components = [node for node, deg in zip(G.nodes(), total_degrees) if deg > p95]

    max_outlier_ratio = getattr(config, 'max_outlier_ratio', 0.1)
    valid = outlier_ratio <= max_outlier_ratio and not chebyshev_violated

    return {
        "name": "concentration_bounds",
        "status": "WARNING" if not valid else "INFO",
        "reason": f"Concentration: μ={mean_degree:.1f}, σ={std_degree:.1f}, " +
                f"{len(actual_outliers)} outliers ({outlier_ratio:.1%})" +
                ("" if valid else " - exceeds bounds"),
        
        "details": {
            "mean_degree": mean_degree,
            "std_degree": std_degree,
            "max_degree": int(max_degree),
            "chebyshev_bound": chebyshev_bound,
            "chernoff_bound": chernoff_bound,
            "outlier_ratio": outlier_ratio,
            "outliers": actual_outliers[:10],
            "p95_degree": p95,
            "p99_degree": p99,
            "tail_components": tail_components[:10]
        }
    }


def validate_bayesian_dependencies(graph: nx.DiGraph, config: Any = None) -> Dict[str, Any]:
    """
    Analyze conditional dependencies using Bayesian concepts.

    Bayes' theorem: P(A|B) = P(B|A)P(A)/P(B)

    For architecture:
    - Conditional independence (d-separation)
    - Dependency networks as Bayesian networks
    - Causal reasoning about changes
    """
    components = [{"name": n, **graph.nodes[n]} for n in graph.nodes()]
    dependencies = [{"from": u, "to": v, **graph.edges[u,v]} for u,v in graph.edges()]

    if len(components) < 3:
        return {
            "name": "bayesian_dependencies",
            "status": "INFO",
            "reason": "Insufficient components for Bayesian analysis",
            
            "details": {"components": len(components)}
        }

    # Build DAG (treat cycles as issues)
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

    # Check DAG property (required for Bayesian network)
    is_dag = nx.is_directed_acyclic_graph(G)

    if not is_dag:
        # Find cycles for reporting
        from validator.utils.cycles import simple_cycles_bounded
        try:
            cycles = list(simple_cycles_bounded(G, max_length=10))
            cycle_count = len(cycles)
        except:
            cycle_count = -1
    else:
        cycle_count = 0

    # Analyze as Bayesian network (if DAG)
    # Calculate "conditional dependencies" structure

    # Markov blanket: parents + children + parents of children
    markov_blankets = {}
    for node in G.nodes():
        parents = set(G.predecessors(node))
        children = set(G.successors(node))
        parents_of_children = set()
        for child in children:
            parents_of_children.update(G.predecessors(child))
        parents_of_children.discard(node)

        blanket = parents | children | parents_of_children
        markov_blankets[node] = blanket

    # Blanket size distribution
    blanket_sizes = [len(b) for b in markov_blankets.values()]
    avg_blanket_size = np.mean(blanket_sizes) if blanket_sizes else 0
    max_blanket_size = max(blanket_sizes) if blanket_sizes else 0

    # D-separation analysis (simplified)
    # Two nodes are d-separated given evidence if no active path
    # Check independence structure
    conditionally_independent = []

    # Sample pairs
    nodes = list(G.nodes())
    import random
    random.seed(42)

    sample_size = min(30, len(nodes) * 3)
    for _ in range(sample_size):
        if len(nodes) < 3:
            break
        a, b, c = random.sample(nodes, 3)

        # Check if a and b are d-separated given c
        # Simplified: check if c blocks all paths
        try:
            paths_exist = nx.has_path(G, a, b) or nx.has_path(G, b, a)
            if paths_exist:
                # Check if c is on all paths
                c_blocks = True
                G_temp = G.copy()
                G_temp.remove_node(c)
                if nx.has_path(G_temp, a, b) or nx.has_path(G_temp, b, a):
                    c_blocks = False

                if c_blocks:
                    conditionally_independent.append((a, b, c))
        except:
            pass

    # Calculate factorization structure
    # P(X1,...,Xn) = Π P(Xi | Parents(Xi))
    # Count average parent set size
    parent_sizes = [G.in_degree(n) for n in G.nodes()]
    avg_parents = np.mean(parent_sizes) if parent_sizes else 0

    # Sparsity: ratio of edges to fully connected
    n = G.number_of_nodes()
    max_edges = n * (n - 1)
    sparsity = 1 - G.number_of_edges() / max_edges if max_edges > 0 else 1.0

    valid = is_dag and avg_blanket_size <= n * 0.5

    return {
        "name": "bayesian_dependencies",
        "status": "WARNING" if not valid else "INFO",
        "reason": f"Bayesian: {'DAG' if is_dag else f'has {cycle_count} cycles'}, " +
                f"avg blanket={avg_blanket_size:.1f}, sparsity={sparsity:.1%}" +
                ("" if valid else " - structure issues"),
        
        "details": {
            "is_dag": is_dag,
            "cycle_count": cycle_count,
            "avg_markov_blanket_size": avg_blanket_size,
            "max_markov_blanket_size": max_blanket_size,
            "avg_parent_count": avg_parents,
            "sparsity": sparsity,
            "conditionally_independent_samples": conditionally_independent[:10],
            "largest_blankets": sorted(
                [(n, len(b)) for n, b in markov_blankets.items()],
                key=lambda x: -x[1]
            )[:10]
        }
    }


def validate_stochastic_processes(graph: nx.DiGraph, config: Any = None) -> Dict[str, Any]:
    """
    Analyze stochastic process properties of architecture.

    Models architecture behavior as stochastic process:
    - Birth-death processes (component creation/deletion)
    - Queuing patterns (request flow)
    - Renewal processes (periodic patterns)

    Metrics:
    - Steady-state analysis
    - Traffic intensity
    - Utilization patterns
    """
    components = [{"name": n, **graph.nodes[n]} for n in graph.nodes()]
    dependencies = [{"from": u, "to": v, **graph.edges[u,v]} for u,v in graph.edges()]

    if len(components) < 2:
        return {
            "name": "stochastic_processes",
            "status": "INFO",
            "reason": "Insufficient components for process analysis",
            
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

    # Model as queuing network
    # Arrival rate proxy: in-degree
    # Service rate proxy: out-degree + 1

    nodes = list(G.nodes())
    utilization = {}
    queue_metrics = []

    for node in nodes:
        arrival_rate = G.in_degree(node) + 1  # λ
        service_rate = G.out_degree(node) + 1  # μ

        # Traffic intensity ρ = λ/μ
        rho = arrival_rate / service_rate if service_rate > 0 else float('inf')
        utilization[node] = rho

        # M/M/1 queue metrics (if ρ < 1)
        if rho < 1:
            avg_queue_length = rho / (1 - rho)  # L = ρ/(1-ρ)
            avg_wait_time = 1 / (service_rate - arrival_rate)  # W = 1/(μ-λ)
        else:
            avg_queue_length = float('inf')
            avg_wait_time = float('inf')

        queue_metrics.append({
            "node": node,
            "arrival_rate": arrival_rate,
            "service_rate": service_rate,
            "utilization": rho,
            "avg_queue_length": avg_queue_length if avg_queue_length != float('inf') else "unbounded",
            "stable": rho < 1
        })

    # Count stable vs unstable nodes
    stable_nodes = [q for q in queue_metrics if q["stable"]]
    unstable_nodes = [q for q in queue_metrics if not q["stable"]]

    stability_ratio = len(stable_nodes) / len(nodes) if nodes else 1.0

    # Overall network metrics
    utilization_values = [u for u in utilization.values() if u != float('inf')]
    avg_utilization = np.mean(utilization_values) if utilization_values else 0
    max_utilization = max(utilization_values) if utilization_values else 0

    # Bottleneck identification
    bottlenecks = [q for q in queue_metrics if q["utilization"] > 0.9 and q["stable"]]

    # Birth-death balance
    # Net growth rate at each node
    growth_rates = {}
    for node in nodes:
        # Birth: incoming dependencies bring "work"
        birth = G.in_degree(node)
        # Death: outgoing dependencies consume "work"
        death = G.out_degree(node)
        growth_rates[node] = birth - death

    # Balanced nodes
    balanced = [n for n, rate in growth_rates.items() if abs(rate) <= 2]
    balance_ratio = len(balanced) / len(nodes) if nodes else 1.0

    min_stability = getattr(config, 'min_stability_ratio', 0.8)
    valid = stability_ratio >= min_stability and len(bottlenecks) <= 3

    return {
        "name": "stochastic_processes",
        "status": "WARNING" if not valid else "INFO",
        "reason": f"Stochastic: {len(stable_nodes)}/{len(nodes)} stable ({stability_ratio:.1%}), " +
                f"avg ρ={avg_utilization:.2f}, {len(bottlenecks)} bottlenecks" +
                ("" if valid else " - stability concerns"),
        
        "details": {
            "stability_ratio": stability_ratio,
            "stable_nodes": len(stable_nodes),
            "unstable_nodes": len(unstable_nodes),
            "avg_utilization": avg_utilization,
            "max_utilization": max_utilization,
            "bottlenecks": [b["node"] for b in bottlenecks][:10],
            "balance_ratio": balance_ratio,
            "queue_analysis": sorted(queue_metrics, key=lambda x: -x["utilization"] if x["utilization"] != float('inf') else 1000)[:10],
            "growth_rate_extremes": sorted(growth_rates.items(), key=lambda x: -abs(x[1]))[:10]
        }
    }
