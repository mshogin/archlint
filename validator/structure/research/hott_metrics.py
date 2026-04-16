"""
Homotopy Type Theory (HoTT) metrics for architecture analysis.

Based on the univalent foundations of mathematics developed by Vladimir Voevodsky
and the homotopy type theory community (2006-2013).

Reference: https://homotopytypetheory.org/book/

Suggested by Ярослав Черкашин at Стачка 2026 conference (10 April 2026).
"""

from typing import Any, Dict, List, Optional, Tuple
import networkx as nx
from collections import defaultdict
import itertools


def _is_excluded(node: str, exclude: List[str]) -> bool:
    for pattern in exclude:
        if pattern in node:
            return True
    return False


def validate_univalence_property(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Univalence Property - аксиома унивалентности.

    В HoTT: эквивалентные типы идентичны (A ≃ B → A = B).
    Для архитектуры: изоморфные подграфы зависимостей представляют
    "эквивалентные" модули - они могут быть кандидатами для унификации.

    Использует networkx is_isomorphic для сравнения небольших подграфов.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 4:
            return {
                'name': 'univalence_property',
                'status': 'SKIP',
                'description': 'Univalence axiom: equivalent types are identical',
                'reason': 'Insufficient nodes',
                'details': {},
                'violations': [],
            }

        # Build ego subgraphs for each node (1-hop neighbourhood)
        def ego_subgraph(node):
            neighbors = set(subgraph.predecessors(node)) | set(subgraph.successors(node)) | {node}
            return subgraph.subgraph(neighbors)

        # Compare ego subgraphs pairwise (limit to first 30 nodes for performance)
        sample_nodes = list(nodes)[:30]
        isomorphic_pairs = []

        for i, a in enumerate(sample_nodes):
            for b in sample_nodes[i + 1:]:
                ego_a = ego_subgraph(a)
                ego_b = ego_subgraph(b)
                # Only compare if same number of nodes to speed up
                if len(ego_a) == len(ego_b) and len(ego_a) >= 2:
                    try:
                        if nx.is_isomorphic(ego_a, ego_b):
                            isomorphic_pairs.append((str(a), str(b)))
                    except Exception:
                        pass

        status = 'PASSED' if isomorphic_pairs else 'INFO'

        return {
            'name': 'univalence_property',
            'status': status,
            'description': 'Univalence axiom: equivalent types are identical',
            'details': {
                'nodes_analyzed': len(sample_nodes),
                'isomorphic_pairs_found': len(isomorphic_pairs),
                'sample_pairs': isomorphic_pairs[:5],
                'interpretation': (
                    'Isomorphic ego-subgraphs = structurally equivalent modules '
                    '(univalence holds - candidates for unification or shared abstraction)'
                ),
            },
            'violations': [],
        }
    except Exception as e:
        return {
            'name': 'univalence_property',
            'status': 'ERROR',
            'description': 'Univalence axiom: equivalent types are identical',
            'details': {},
            'violations': [str(e)],
        }


def validate_path_space_structure(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Path Space Structure - структура пространства путей.

    В HoTT: пространство путей Path(A, B) между двумя типами A и B
    само является типом. Размерность этого пространства отражает
    сложность связи.

    Для архитектуры: большое число различных простых путей между
    парой компонентов указывает на запутанную связность (высокую
    "размерность пространства путей").
    """
    exclude = config.exclude if config else []
    threshold = getattr(config, 'path_space_threshold', 5) if config else 5

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 3:
            return {
                'name': 'path_space_structure',
                'status': 'SKIP',
                'description': 'Path space dimension: number of distinct paths between components',
                'reason': 'Insufficient nodes',
                'details': {},
                'violations': [],
            }

        # Sample pairs and compute simple paths (limit depth to 5 to stay tractable)
        sample_nodes = list(nodes)[:20]
        high_dimensional = []
        path_dimensions = {}

        for source in sample_nodes:
            for target in sample_nodes:
                if source == target:
                    continue
                try:
                    paths = list(itertools.islice(
                        nx.all_simple_paths(subgraph, source, target, cutoff=5), 20
                    ))
                    dim = len(paths)
                    if dim > 0:
                        key = f'{source} -> {target}'
                        path_dimensions[key] = dim
                        if dim >= threshold:
                            high_dimensional.append({
                                'source': str(source),
                                'target': str(target),
                                'path_space_dimension': dim,
                            })
                except (nx.NetworkXNoPath, nx.NodeNotFound):
                    pass

        high_dimensional.sort(key=lambda x: -x['path_space_dimension'])
        status = 'WARNING' if high_dimensional else 'INFO'
        max_dim = max(path_dimensions.values()) if path_dimensions else 0

        return {
            'name': 'path_space_structure',
            'status': status,
            'description': 'Path space dimension: number of distinct paths between components',
            'details': {
                'pairs_analyzed': len(path_dimensions),
                'high_dimensional_pairs': len(high_dimensional),
                'max_path_space_dimension': max_dim,
                'threshold': threshold,
                'top_tangles': high_dimensional[:5],
                'interpretation': (
                    'High path-space dimension = tangled connectivity. '
                    'In HoTT terms: a rich higher-dimensional type structure.'
                ),
            },
            'violations': [
                f'{e["source"]} -> {e["target"]}: {e["path_space_dimension"]} paths'
                for e in high_dimensional[:10]
            ],
        }
    except Exception as e:
        return {
            'name': 'path_space_structure',
            'status': 'ERROR',
            'description': 'Path space dimension: number of distinct paths between components',
            'details': {},
            'violations': [str(e)],
        }


def validate_identity_types(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Identity Types - типы тождественности.

    В HoTT: тип тождественности Id_A(a, b) населён тогда и только тогда,
    когда a и b равны. Для отношения это означает:
    - Рефлексивность: a = a  (петли)
    - Симметрия:      a = b → b = a  (двунаправленные рёбра)
    - Транзитивность: a = b, b = c → a = c  (транзитивные замыкания)

    Для архитектуры: какие зависимости образуют корректные типы
    тождественности (= отношения эквивалентности).
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 2:
            return {
                'name': 'identity_types',
                'status': 'SKIP',
                'description': 'Identity types: reflexivity, symmetry, transitivity of dependencies',
                'reason': 'Insufficient nodes',
                'details': {},
                'violations': [],
            }

        # Reflexivity: self-loops
        reflexive = [str(node) for node in nodes if subgraph.has_edge(node, node)]

        # Symmetry: bidirectional edges (A->B and B->A)
        symmetric_pairs = []
        for u, v in subgraph.edges():
            if u != v and subgraph.has_edge(v, u):
                pair = tuple(sorted([str(u), str(v)]))
                if pair not in symmetric_pairs:
                    symmetric_pairs.append(pair)

        # Transitivity check on a sample: if A->B and B->C then A->C?
        transitive_closures = []
        violations_transitivity = []
        sample = list(nodes)[:15]
        for a in sample:
            for b in subgraph.successors(a):
                if b == a:
                    continue
                for c in subgraph.successors(b):
                    if c == a or c == b:
                        continue
                    if subgraph.has_edge(a, c):
                        transitive_closures.append((str(a), str(b), str(c)))
                    else:
                        violations_transitivity.append((str(a), str(b), str(c)))

        proper_identity_relations = len(symmetric_pairs)

        return {
            'name': 'identity_types',
            'status': 'INFO',
            'description': 'Identity types: reflexivity, symmetry, transitivity of dependencies',
            'details': {
                'reflexive_nodes': len(reflexive),
                'reflexive_samples': reflexive[:5],
                'symmetric_pairs': len(symmetric_pairs),
                'symmetric_samples': [list(p) for p in symmetric_pairs[:5]],
                'transitive_closures_present': len(transitive_closures),
                'transitive_gaps': len(violations_transitivity),
                'proper_identity_relations': proper_identity_relations,
                'interpretation': (
                    'Symmetric bidirectional dependencies form proper identity types. '
                    'Transitive gaps = implicit dependencies not yet explicit in the graph.'
                ),
            },
            'violations': [],
        }
    except Exception as e:
        return {
            'name': 'identity_types',
            'status': 'ERROR',
            'description': 'Identity types: reflexivity, symmetry, transitivity of dependencies',
            'details': {},
            'violations': [str(e)],
        }


def validate_higher_inductive_types(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Higher Inductive Types (HITs) - высшие индуктивные типы.

    В HoTT: HITs - типы, определённые не только конструкторами точек,
    но и конструкторами путей (и путей между путями).

    Для архитектуры: компоненты, образующие циклы длиной > 2,
    представляют собой "высшие" рекурсивные структуры -
    они самореференциальны через N переходов.
    """
    exclude = config.exclude if config else []
    min_cycle_length = getattr(config, 'min_hit_cycle_length', 3) if config else 3

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 3:
            return {
                'name': 'higher_inductive_types',
                'status': 'SKIP',
                'description': 'Higher inductive types: cycles of length > 2 as recursive structures',
                'reason': 'Insufficient nodes',
                'details': {},
                'violations': [],
            }

        # Find all simple cycles (bounded to avoid explosion on large graphs)
        from validator.utils.cycles import simple_cycles_bounded
        try:
            all_cycles = list(simple_cycles_bounded(subgraph, max_length=10))
        except Exception:
            all_cycles = []

        # Separate by length
        self_loops = [c for c in all_cycles if len(c) == 1]
        mutual_cycles = [c for c in all_cycles if len(c) == 2]
        higher_cycles = [c for c in all_cycles if len(c) >= min_cycle_length]

        # Group higher cycles by length
        by_length: Dict[int, List] = defaultdict(list)
        for cycle in higher_cycles:
            by_length[len(cycle)].append([str(node) for node in cycle])

        length_distribution = {str(k): len(v) for k, v in sorted(by_length.items())}
        violations = []
        for cycle in higher_cycles[:10]:
            violations.append(' -> '.join(str(n) for n in cycle) + ' -> ' + str(cycle[0]))

        status = 'WARNING' if higher_cycles else 'INFO'

        return {
            'name': 'higher_inductive_types',
            'status': status,
            'description': 'Higher inductive types: cycles of length > 2 as recursive structures',
            'details': {
                'self_loops': len(self_loops),
                'mutual_cycles_length_2': len(mutual_cycles),
                'higher_inductive_types_found': len(higher_cycles),
                'min_cycle_length_threshold': min_cycle_length,
                'cycle_length_distribution': length_distribution,
                'samples': [
                    [str(node) for node in c]
                    for c in higher_cycles[:5]
                ],
                'interpretation': (
                    'Cycles of length >= 3 are higher inductive types: '
                    'components that reference themselves through N hops. '
                    'These represent recursive architectural entanglement.'
                ),
            },
            'violations': violations,
        }
    except Exception as e:
        return {
            'name': 'higher_inductive_types',
            'status': 'ERROR',
            'description': 'Higher inductive types: cycles of length > 2 as recursive structures',
            'details': {},
            'violations': [str(e)],
        }


def validate_type_equivalence_coherence(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Type Equivalence Coherence - когерентность эквивалентности типов.

    В HoTT: если A ≃ B и A ≃ C, то B ≃ C (транзитивность эквивалентности).
    Для подстановок типов (реализаций интерфейсов) это означает:
    если несколько узлов "эквивалентно" зависят от одного узла,
    они должны согласовываться друг с другом.

    Для архитектуры: проверяем, что для каждого узла с несколькими
    входящими «эквивалентными» зависимостями они согласованы
    (имеют схожие профили зависимостей).
    """
    exclude = config.exclude if config else []
    coherence_threshold = getattr(config, 'coherence_threshold', 0.5) if config else 0.5

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 4:
            return {
                'name': 'type_equivalence_coherence',
                'status': 'SKIP',
                'description': 'Type substitution coherence: interface implementations agree',
                'reason': 'Insufficient nodes',
                'details': {},
                'violations': [],
            }

        # For each node, find nodes with multiple predecessors that share the same successors
        # (they "implement" the same "interface" if they have the same outgoing dependencies)
        incoherent_nodes = []
        coherent_nodes = []

        for target in list(nodes)[:30]:
            predecessors = list(subgraph.predecessors(target))
            if len(predecessors) < 2:
                continue

            # Build outgoing dependency sets for each predecessor
            pred_deps = {}
            for pred in predecessors:
                dep_set = frozenset(subgraph.successors(pred))
                pred_deps[str(pred)] = dep_set

            # Compute pairwise Jaccard similarity
            pred_list = list(pred_deps.items())
            similarities = []
            for i, (pa, deps_a) in enumerate(pred_list):
                for pb, deps_b in pred_list[i + 1:]:
                    union = deps_a | deps_b
                    inter = deps_a & deps_b
                    if union:
                        jaccard = len(inter) / len(union)
                        similarities.append(jaccard)

            if not similarities:
                continue

            avg_similarity = sum(similarities) / len(similarities)

            entry = {
                'node': str(target),
                'predecessors': len(predecessors),
                'avg_coherence': round(avg_similarity, 4),
            }

            if avg_similarity < coherence_threshold and len(predecessors) >= 2:
                incoherent_nodes.append(entry)
            else:
                coherent_nodes.append(entry)

        incoherent_nodes.sort(key=lambda x: x['avg_coherence'])
        status = 'WARNING' if incoherent_nodes else 'INFO'

        return {
            'name': 'type_equivalence_coherence',
            'status': status,
            'description': 'Type substitution coherence: interface implementations agree',
            'details': {
                'nodes_with_multiple_predecessors': len(incoherent_nodes) + len(coherent_nodes),
                'coherent_nodes': len(coherent_nodes),
                'incoherent_nodes': len(incoherent_nodes),
                'coherence_threshold': coherence_threshold,
                'incoherent_samples': incoherent_nodes[:5],
                'interpretation': (
                    'Nodes whose predecessors have dissimilar dependency profiles '
                    'represent incoherent type substitutions - the "implementors" '
                    'do not agree on their own dependencies.'
                ),
            },
            'violations': [
                f'{e["node"]}: coherence={e["avg_coherence"]} < {coherence_threshold}'
                for e in incoherent_nodes[:10]
            ],
        }
    except Exception as e:
        return {
            'name': 'type_equivalence_coherence',
            'status': 'ERROR',
            'description': 'Type substitution coherence: interface implementations agree',
            'details': {},
            'violations': [str(e)],
        }


def validate_n_truncation(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    n-Truncation - n-усечение.

    В HoTT:
    - (-1)-truncated (propositions): каждые два элемента равны
    - 0-truncated  (sets):          нет нетривиальных путей = нет циклов
    - 1-truncated  (groupoids):     все пути обратимы (симметричные рёбра)
    - n-truncated:                  все структуры выше уровня n тривиальны

    Для архитектуры:
    - 0-truncated (DAG): нет циклов - идеальная иерархия
    - 1-truncated: есть двунаправленные ребра, но нет длинных циклов
    - n >= 2: есть длинные (n-hop) циклы - запутанность

    Возвращаем минимальный уровень n, при котором граф является
    n-усечённым.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 2:
            return {
                'name': 'n_truncation',
                'status': 'SKIP',
                'description': 'n-truncation level: minimum n for which the graph is n-truncated',
                'reason': 'Insufficient nodes',
                'details': {},
                'violations': [],
            }

        # Find all simple cycles (bounded to avoid explosion on large graphs)
        from validator.utils.cycles import simple_cycles_bounded
        try:
            all_cycles = list(simple_cycles_bounded(subgraph, max_length=10))
        except Exception:
            all_cycles = []

        has_cycles = len(all_cycles) > 0
        cycle_lengths = [len(c) for c in all_cycles] if all_cycles else []
        max_cycle_length = max(cycle_lengths) if cycle_lengths else 0

        # Determine truncation level
        if not has_cycles:
            # No cycles at all -> 0-truncated (sets / DAG)
            truncation_level = 0
            level_name = '0-truncated (sets / DAG)'
            status = 'PASSED'
        else:
            # Check for mutual (length-2) cycles only
            long_cycles = [c for c in all_cycles if len(c) > 2]
            if not long_cycles:
                # Only length-1 (self-loops) and length-2 (bidirectional) cycles
                truncation_level = 1
                level_name = '1-truncated (groupoid): only reversible / bidirectional paths'
                status = 'INFO'
            else:
                # Higher cycles exist; level = max cycle length - 1 (homotopy dimension)
                truncation_level = max_cycle_length - 1
                level_name = f'{truncation_level}-truncated: has cycles up to length {max_cycle_length}'
                status = 'WARNING'

        # Collect longest cycle examples
        longest_cycles = sorted(all_cycles, key=len, reverse=True)[:3]

        return {
            'name': 'n_truncation',
            'status': status,
            'description': 'n-truncation level: minimum n for which the graph is n-truncated',
            'details': {
                'truncation_level': truncation_level,
                'level_name': level_name,
                'total_cycles': len(all_cycles),
                'max_cycle_length': max_cycle_length,
                'cycle_length_distribution': {
                    str(length): cycle_lengths.count(length)
                    for length in sorted(set(cycle_lengths))
                },
                'longest_cycle_examples': [
                    [str(node) for node in c]
                    for c in longest_cycles
                ],
                'interpretation': (
                    '0-truncated = clean DAG (ideal). '
                    '1-truncated = groupoid (bidirectional deps only). '
                    'n>=2 = higher entanglement (n = max_cycle_length - 1).'
                ),
            },
            'violations': [
                ' -> '.join(str(node) for node in c) + f' (length {len(c)})'
                for c in longest_cycles
            ],
        }
    except Exception as e:
        return {
            'name': 'n_truncation',
            'status': 'ERROR',
            'description': 'n-truncation level: minimum n for which the graph is n-truncated',
            'details': {},
            'violations': [str(e)],
        }
