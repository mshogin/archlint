"""
Number Theory Metrics for Software Architecture Validation.

Applies number theory concepts:
1. Divisibility and Factorization - structural decomposition
2. Prime Analysis - fundamental building blocks
3. Modular Arithmetic - cyclic patterns
4. Chinese Remainder Theorem - independent constraints
5. Diophantine Analysis - integer solutions

Mathematical foundations:
- Fundamental Theorem of Arithmetic: unique prime factorization
- Euler's totient function φ(n)
- GCD/LCM relationships
- Modular congruences
"""

from typing import Any, Dict, List
import networkx as nx
from collections import defaultdict
import math


def _is_excluded(node: str, exclude: List[str]) -> bool:
    """Check if node matches any exclusion pattern."""
    for pattern in exclude:
        if pattern in node:
            return True
    return False


def validate_prime_factorization(graph: nx.DiGraph, config: Any = None) -> Dict[str, Any]:
    """
    Analyze architecture through prime factorization lens.

    Just as every integer has a unique prime factorization,
    architecture should decompose into "prime" (atomic) components.

    Checks:
    - Identifies atomic components (cannot be decomposed further)
    - Measures how well architecture factors into primes
    - Detects "composite" components that should be split
    """
    components = [{"name": n, **graph.nodes[n]} for n in graph.nodes()]
    dependencies = [{"from": u, "to": v, **graph.edges[u,v]} for u,v in graph.edges()]

    if len(components) < 2:
        return {
            "name": "prime_factorization",
            "status": "INFO",
            "reason": "Insufficient components for factorization analysis",
            
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

    # "Prime" components: minimal, cannot be decomposed
    # Characteristics: low internal complexity, single responsibility
    # Proxy: out_degree (what it depends on) is low

    degrees = {n: (G.in_degree(n), G.out_degree(n)) for n in G.nodes()}

    # Prime candidates: out_degree <= 2 (depends on few things)
    prime_candidates = [n for n, (in_d, out_d) in degrees.items() if out_d <= 2]

    # Composite candidates: high out_degree (depends on many things)
    composite_candidates = [n for n, (in_d, out_d) in degrees.items() if out_d > 5]

    # Check if composites can be "factored" into primes
    # A composite should ideally depend only on primes or through primes
    factorization_quality = []

    for composite in composite_candidates:
        direct_deps = list(G.successors(composite))
        prime_deps = [d for d in direct_deps if d in prime_candidates]
        composite_deps = [d for d in direct_deps if d in composite_candidates and d != composite]

        factor_ratio = len(prime_deps) / len(direct_deps) if direct_deps else 1.0
        factorization_quality.append({
            "component": composite,
            "total_deps": len(direct_deps),
            "prime_deps": len(prime_deps),
            "composite_deps": len(composite_deps),
            "factor_ratio": factor_ratio
        })

    # Calculate overall factorization score
    if factorization_quality:
        avg_factor_ratio = sum(f["factor_ratio"] for f in factorization_quality) / len(factorization_quality)
    else:
        avg_factor_ratio = 1.0

    # Prime ratio in architecture
    prime_ratio = len(prime_candidates) / len(components) if components else 0

    min_prime_ratio = getattr(config, 'min_prime_ratio', 0.3)
    min_factor_ratio = getattr(config, 'min_factorization_ratio', 0.5)

    valid = prime_ratio >= min_prime_ratio and avg_factor_ratio >= min_factor_ratio

    return {
        "name": "prime_factorization",
        "status": "WARNING" if not valid else "INFO",
        "reason": f"Factorization: {len(prime_candidates)} primes ({prime_ratio:.1%}), " +
                f"{len(composite_candidates)} composites, avg factor ratio={avg_factor_ratio:.2f}" +
                ("" if valid else " - needs better decomposition"),
        
        "details": {
            "prime_count": len(prime_candidates),
            "composite_count": len(composite_candidates),
            "prime_ratio": prime_ratio,
            "avg_factorization_ratio": avg_factor_ratio,
            "prime_components": prime_candidates[:15],
            "composite_analysis": factorization_quality[:10]
        }
    }


def validate_modular_arithmetic(graph: nx.DiGraph, config: Any = None) -> Dict[str, Any]:
    """
    Analyze cyclic patterns using modular arithmetic.

    Modular arithmetic deals with cycles: a ≡ b (mod n)

    For architecture:
    - Identify cyclic patterns in dependencies
    - Measure cycle lengths and their relationships
    - GCD/LCM of cycle lengths reveals structure
    """
    components = [{"name": n, **graph.nodes[n]} for n in graph.nodes()]
    dependencies = [{"from": u, "to": v, **graph.edges[u,v]} for u,v in graph.edges()]

    if len(components) < 3:
        return {
            "name": "modular_arithmetic",
            "status": "INFO",
            "reason": "Insufficient components for modular analysis",
            
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

    # Find all cycles
    try:
        cycles = list(nx.simple_cycles(G))
    except:
        cycles = []

    if not cycles:
        return {
            "name": "modular_arithmetic",
            "status": "INFO",
            "reason": "No cycles found - acyclic architecture",
            
            "details": {"cycles": 0}
        }

    # Cycle lengths
    cycle_lengths = [len(c) for c in cycles]

    # Calculate GCD of all cycle lengths
    if len(cycle_lengths) >= 2:
        overall_gcd = cycle_lengths[0]
        for length in cycle_lengths[1:]:
            overall_gcd = math.gcd(overall_gcd, length)
    else:
        overall_gcd = cycle_lengths[0] if cycle_lengths else 1

    # Calculate LCM (careful with overflow)
    def safe_lcm(a, b):
        return min(a * b // math.gcd(a, b), 10000)

    if len(cycle_lengths) >= 2:
        overall_lcm = cycle_lengths[0]
        for length in cycle_lengths[1:]:
            overall_lcm = safe_lcm(overall_lcm, length)
    else:
        overall_lcm = cycle_lengths[0] if cycle_lengths else 1

    # Analyze residue classes
    # Group cycles by length mod GCD
    residue_classes = defaultdict(list)
    for i, cycle in enumerate(cycles):
        residue = len(cycle) % overall_gcd if overall_gcd > 0 else 0
        residue_classes[residue].append(i)

    # Euler's totient analog: count of cycles coprime to total
    n = len(cycles)
    coprime_count = sum(1 for length in cycle_lengths if math.gcd(length, n) == 1) if n > 0 else 0
    totient_ratio = coprime_count / n if n > 0 else 1.0

    # Check for "harmonic" cycle structure (lengths are multiples of each other)
    harmonic_pairs = 0
    total_pairs = 0
    for i, l1 in enumerate(cycle_lengths):
        for l2 in cycle_lengths[i+1:]:
            total_pairs += 1
            if l1 % l2 == 0 or l2 % l1 == 0:
                harmonic_pairs += 1

    harmonic_ratio = harmonic_pairs / total_pairs if total_pairs > 0 else 1.0

    # High GCD indicates regular cycle structure
    # High harmonic ratio indicates nested/hierarchical cycles
    max_cycle_length = getattr(config, 'max_cycle_length', 10)
    long_cycles = [c for c in cycles if len(c) > max_cycle_length]

    valid = len(long_cycles) == 0 and overall_gcd <= 5

    return {
        "name": "modular_arithmetic",
        "status": "WARNING" if not valid else "INFO",
        "reason": f"Modular: {len(cycles)} cycles, GCD={overall_gcd}, LCM={overall_lcm}, " +
                f"harmonic={harmonic_ratio:.1%}" +
                ("" if valid else f", {len(long_cycles)} long cycles"),
        
        "details": {
            "cycle_count": len(cycles),
            "cycle_lengths": sorted(set(cycle_lengths)),
            "gcd": overall_gcd,
            "lcm": overall_lcm,
            "residue_classes": len(residue_classes),
            "totient_ratio": totient_ratio,
            "harmonic_ratio": harmonic_ratio,
            "long_cycles": len(long_cycles)
        }
    }


def validate_chinese_remainder(graph: nx.DiGraph, config: Any = None) -> Dict[str, Any]:
    """
    Apply Chinese Remainder Theorem concepts to architecture.

    CRT: System of congruences has unique solution if moduli are coprime.

    For architecture:
    - Independent subsystems should have coprime "periods"
    - Constraints from different modules should be compatible
    - Detects conflicting requirements
    """
    components = [{"name": n, **graph.nodes[n]} for n in graph.nodes()]
    dependencies = [{"from": u, "to": v, **graph.edges[u,v]} for u,v in graph.edges()]

    if len(components) < 3:
        return {
            "name": "chinese_remainder",
            "status": "INFO",
            "reason": "Insufficient components for CRT analysis",
            
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
        if src and tgt and src in G.nodes() and tgt in G.nodes():
            G.add_edge(src, tgt)

    # Find independent subsystems (weakly connected components)
    wccs = list(nx.weakly_connected_components(G))

    # For each subsystem, calculate a "characteristic number"
    # Based on structure: nodes * edges + cycles
    subsystem_chars = []

    for i, wcc in enumerate(wccs):
        subgraph = G.subgraph(wcc)
        n = subgraph.number_of_nodes()
        m = subgraph.number_of_edges()

        try:
            cycle_count = len(list(nx.simple_cycles(subgraph)))
        except:
            cycle_count = 0

        # Characteristic: prime factorization analog
        char = n + m * 2 + cycle_count * 3
        char = max(1, char)  # Ensure positive

        subsystem_chars.append({
            "id": i,
            "nodes": n,
            "edges": m,
            "cycles": cycle_count,
            "characteristic": char
        })

    # Check pairwise coprimality
    coprime_pairs = 0
    non_coprime_pairs = []
    total_pairs = 0

    chars = [s["characteristic"] for s in subsystem_chars]
    for i, c1 in enumerate(chars):
        for j, c2 in enumerate(chars[i+1:], i+1):
            total_pairs += 1
            if math.gcd(c1, c2) == 1:
                coprime_pairs += 1
            else:
                non_coprime_pairs.append({
                    "subsystem1": i,
                    "subsystem2": j,
                    "char1": c1,
                    "char2": c2,
                    "gcd": math.gcd(c1, c2)
                })

    coprime_ratio = coprime_pairs / total_pairs if total_pairs > 0 else 1.0

    # CRT solvability: system is solvable if all moduli are pairwise coprime
    # High coprime ratio = more independent subsystems
    # Low coprime ratio = potential constraint conflicts

    # Calculate overall "modulus" (LCM of all characteristics)
    if chars:
        overall_modulus = chars[0]
        for c in chars[1:]:
            overall_modulus = min(overall_modulus * c // math.gcd(overall_modulus, c), 1000000)
    else:
        overall_modulus = 1

    min_coprime_ratio = getattr(config, 'min_coprime_ratio', 0.5)
    valid = coprime_ratio >= min_coprime_ratio

    return {
        "name": "chinese_remainder",
        "status": "WARNING" if not valid else "INFO",
        "reason": f"CRT analysis: {len(wccs)} subsystems, " +
                f"{coprime_pairs}/{total_pairs} coprime pairs ({coprime_ratio:.1%})" +
                ("" if valid else " - potential constraint conflicts"),
        
        "details": {
            "subsystem_count": len(wccs),
            "coprime_ratio": coprime_ratio,
            "coprime_pairs": coprime_pairs,
            "total_pairs": total_pairs,
            "overall_modulus": overall_modulus,
            "subsystem_characteristics": subsystem_chars[:10],
            "non_coprime_pairs": non_coprime_pairs[:5]
        }
    }


def validate_divisibility_lattice(graph: nx.DiGraph, config: Any = None) -> Dict[str, Any]:
    """
    Analyze architecture using divisibility lattice structure.

    Divisibility forms a lattice where:
    - Meet (∧) = GCD
    - Join (∨) = LCM
    - 1 is bottom, 0 is top (formally)

    For architecture:
    - Component "sizes" form divisibility relationships
    - Well-structured hierarchy follows divisibility patterns
    """
    components = [{"name": n, **graph.nodes[n]} for n in graph.nodes()]
    dependencies = [{"from": u, "to": v, **graph.edges[u,v]} for u,v in graph.edges()]

    if len(components) < 2:
        return {
            "name": "divisibility_lattice",
            "status": "INFO",
            "reason": "Insufficient components for divisibility analysis",
            
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

    # Assign "weight" to each component based on its structural importance
    # Weight = 1 + in_degree + out_degree * 2 + descendants count
    weights = {}
    for node in G.nodes():
        descendants = len(nx.descendants(G, node))
        weight = 1 + G.in_degree(node) + G.out_degree(node) * 2 + descendants
        weights[node] = weight

    # Build divisibility relationships
    # A "divides" B if there's a path from A to B and weight(A) | weight(B)
    divisibility_edges = []
    non_divisibility_violations = []

    for u, v in G.edges():
        w_u = weights[u]
        w_v = weights[v]

        # Check if larger divides smaller or vice versa
        if w_u > 0 and w_v > 0:
            if w_v % w_u == 0 or w_u % w_v == 0:
                divisibility_edges.append((u, v, math.gcd(w_u, w_v)))
            else:
                non_divisibility_violations.append({
                    "from": u,
                    "to": v,
                    "weight_from": w_u,
                    "weight_to": w_v,
                    "gcd": math.gcd(w_u, w_v)
                })

    # Calculate divisibility ratio
    total_edges = G.number_of_edges()
    divisibility_ratio = len(divisibility_edges) / total_edges if total_edges > 0 else 1.0

    # Find "prime" weights (weights that are actually prime numbers)
    def is_prime(n):
        if n < 2:
            return False
        for i in range(2, int(n**0.5) + 1):
            if n % i == 0:
                return False
        return True

    prime_weighted = [n for n, w in weights.items() if is_prime(w)]

    # Calculate lattice properties
    weight_values = list(weights.values())
    if len(weight_values) >= 2:
        overall_gcd = weight_values[0]
        overall_lcm = weight_values[0]
        for w in weight_values[1:]:
            overall_gcd = math.gcd(overall_gcd, w)
            overall_lcm = min(overall_lcm * w // math.gcd(overall_lcm, w), 1000000)
    else:
        overall_gcd = weight_values[0] if weight_values else 1
        overall_lcm = weight_values[0] if weight_values else 1

    min_divisibility = getattr(config, 'min_divisibility_ratio', 0.3)
    valid = divisibility_ratio >= min_divisibility

    return {
        "name": "divisibility_lattice",
        "status": "WARNING" if not valid else "INFO",
        "reason": f"Divisibility: {len(divisibility_edges)}/{total_edges} edges follow divisibility " +
                f"({divisibility_ratio:.1%}), GCD={overall_gcd}" +
                ("" if valid else " - irregular weight structure"),
        
        "details": {
            "divisibility_ratio": divisibility_ratio,
            "divisibility_edges": len(divisibility_edges),
            "violations": len(non_divisibility_violations),
            "overall_gcd": overall_gcd,
            "overall_lcm": overall_lcm,
            "prime_weighted_components": prime_weighted[:10],
            "sample_violations": non_divisibility_violations[:5],
            "weight_distribution": dict(sorted(weights.items(), key=lambda x: -x[1])[:10])
        }
    }


def validate_diophantine_constraints(graph: nx.DiGraph, config: Any = None) -> Dict[str, Any]:
    """
    Analyze integer constraint satisfaction (Diophantine equations).

    Diophantine equations: polynomial equations seeking integer solutions.
    ax + by = c has solutions iff gcd(a,b) | c

    For architecture:
    - Resource constraints as linear Diophantine equations
    - Checks if integer solutions exist
    - Identifies over/under-constrained systems
    """
    components = [{"name": n, **graph.nodes[n]} for n in graph.nodes()]
    dependencies = [{"from": u, "to": v, **graph.edges[u,v]} for u,v in graph.edges()]

    if len(components) < 2:
        return {
            "name": "diophantine_constraints",
            "status": "INFO",
            "reason": "Insufficient components for Diophantine analysis",
            
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
        if src and tgt and src in G.nodes() and tgt in G.nodes():
            G.add_edge(src, tgt)

    # Model: each component has "capacity" and "demand"
    # Constraint: capacity >= demand (from dependencies)

    # Calculate capacity (what component can provide) = out_degree + 1
    # Calculate demand (what component needs) = in_degree
    constraints = []

    for node in G.nodes():
        capacity = G.out_degree(node) + 1  # Can serve out_degree + self
        demand = G.in_degree(node)  # Needs to handle in_degree

        constraints.append({
            "component": node,
            "capacity": capacity,
            "demand": demand,
            "satisfied": capacity >= demand,
            "slack": capacity - demand
        })

    # Check layer constraints
    # ax + by = c form: sum of capacities in layer = sum of demands from upper layers
    layers = defaultdict(list)
    for node in G.nodes():
        layer = G.nodes[node].get('layer', 'default')
        layers[layer].append(node)

    layer_equations = []
    for layer_name, layer_nodes in layers.items():
        layer_capacity = sum(G.out_degree(n) + 1 for n in layer_nodes)

        # Demand from components outside this layer
        external_demand = 0
        for node in layer_nodes:
            for pred in G.predecessors(node):
                if pred not in layer_nodes:
                    external_demand += 1

        # Check solvability: capacity should cover demand
        gcd = math.gcd(layer_capacity, external_demand) if external_demand > 0 else layer_capacity
        solvable = external_demand == 0 or layer_capacity >= external_demand

        layer_equations.append({
            "layer": layer_name,
            "capacity": layer_capacity,
            "external_demand": external_demand,
            "gcd": gcd,
            "solvable": solvable
        })

    # Count satisfied constraints
    satisfied = sum(1 for c in constraints if c["satisfied"])
    satisfaction_ratio = satisfied / len(constraints) if constraints else 1.0

    solvable_layers = sum(1 for eq in layer_equations if eq["solvable"])
    layer_solvability = solvable_layers / len(layer_equations) if layer_equations else 1.0

    # Identify over-constrained components (demand >> capacity)
    over_constrained = [c for c in constraints if c["slack"] < -2]

    min_satisfaction = getattr(config, 'min_constraint_satisfaction', 0.8)
    valid = satisfaction_ratio >= min_satisfaction and len(over_constrained) == 0

    return {
        "name": "diophantine_constraints",
        "status": "WARNING" if not valid else "INFO",
        "reason": f"Diophantine: {satisfied}/{len(constraints)} constraints satisfied " +
                f"({satisfaction_ratio:.1%}), {solvable_layers}/{len(layer_equations)} layers solvable" +
                ("" if valid else f", {len(over_constrained)} over-constrained"),
        
        "details": {
            "satisfaction_ratio": satisfaction_ratio,
            "satisfied_constraints": satisfied,
            "total_constraints": len(constraints),
            "layer_solvability": layer_solvability,
            "over_constrained": [c["component"] for c in over_constrained][:10],
            "layer_analysis": layer_equations,
            "constraint_samples": sorted(constraints, key=lambda x: x["slack"])[:10]
        }
    }
