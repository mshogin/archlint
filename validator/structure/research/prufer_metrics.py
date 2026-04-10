"""
Prüfer sequence / theorem metrics for architecture analysis.

Based on Heinz Prüfer's 1918 theorem encoding labeled trees as sequences.
Cayley's formula: there are n^(n-2) labeled trees on n vertices.

Reference: https://en.wikipedia.org/wiki/Pr%C3%BCfer_sequence

Suggested by Ярослав Черкашин (@Yaroslam, https://github.com/Yaroslam)
at Стачка 2026 conference (10 April 2026).
"""

from typing import Any, Dict, List, Optional
import math
import networkx as nx


def _is_excluded(node: str, exclude: List[str]) -> bool:
    for pattern in exclude:
        if pattern in node:
            return True
    return False


def _compute_prufer_code(tree: nx.Graph) -> List[int]:
    """
    Compute the Prüfer sequence of a labeled tree.

    Nodes are relabeled to integers 1..n before encoding.
    Algorithm: repeatedly remove the smallest-labeled leaf and record
    the label of its unique neighbor, until exactly 2 nodes remain.
    """
    n = tree.number_of_nodes()
    if n < 2:
        return []

    # Relabel nodes to integers 1..n
    mapping = {old: i + 1 for i, old in enumerate(sorted(tree.nodes(), key=str))}
    t = nx.relabel_nodes(tree, mapping)

    # Work on a mutable copy
    t = t.copy()
    code = []

    for _ in range(n - 2):
        # Find the smallest leaf (degree == 1)
        leaves = sorted([v for v in t.nodes() if t.degree(v) == 1])
        if not leaves:
            break
        leaf = leaves[0]
        neighbor = list(t.neighbors(leaf))[0]
        code.append(neighbor)
        t.remove_node(leaf)

    return code


def validate_prufer_canonical_form(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Prüfer Canonical Form - каноническая форма Прюфера.

    Вычисляет остовное дерево неориентированной версии графа
    (nx.minimum_spanning_tree), перенумеровывает вершины в 1..n
    и кодирует дерево последовательностью Прюфера.

    Результат носит описательный характер: это уникальная
    "подпись" остовного дерева архитектуры в виде числовой
    последовательности длиной n-2.

    Статус: PASSED (всегда - дескриптивная метрика).
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)
        n = len(nodes)

        if n < 3:
            return {
                'name': 'prufer_canonical_form',
                'status': 'SKIP',
                'description': 'Prüfer canonical form: spanning tree encoded as integer sequence',
                'reason': 'Insufficient nodes (need >= 3)',
                'details': {},
                'violations': [],
            }

        # Build undirected version and compute minimum spanning tree
        undirected = subgraph.to_undirected()
        spanning_tree = nx.minimum_spanning_tree(undirected)

        code = _compute_prufer_code(spanning_tree)

        return {
            'name': 'prufer_canonical_form',
            'status': 'PASSED',
            'description': 'Prüfer canonical form: spanning tree encoded as integer sequence',
            'details': {
                'nodes': n,
                'spanning_tree_edges': spanning_tree.number_of_edges(),
                'prufer_code_length': len(code),
                'prufer_code': code,
                'interpretation': (
                    'The Prüfer sequence uniquely encodes the minimum spanning tree '
                    'of the architecture graph. Length = n-2. '
                    'Same code = isomorphic spanning tree structure.'
                ),
            },
            'violations': [],
        }
    except Exception as e:
        return {
            'name': 'prufer_canonical_form',
            'status': 'ERROR',
            'description': 'Prüfer canonical form: spanning tree encoded as integer sequence',
            'details': {},
            'violations': [str(e)],
        }


def validate_prufer_entropy(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Prüfer Entropy - энтропия последовательности Прюфера.

    Вычисляет энтропию Шеннона над значениями кода Прюфера.
    Высокая энтропия означает, что метки вершин в коде
    распределены почти равномерно - признак сложной, "случайной"
    топологии остовного дерева.

    Статус: WARNING если entropy > log(n) * 0.9
    (почти максимальная энтропия = случайно-сложная архитектура).
    """
    exclude = config.exclude if config else []
    entropy_ratio_threshold = getattr(config, 'prufer_entropy_ratio_threshold', 0.9) if config else 0.9

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)
        n = len(nodes)

        if n < 4:
            return {
                'name': 'prufer_entropy',
                'status': 'SKIP',
                'description': 'Prüfer entropy: Shannon entropy of the Prüfer code',
                'reason': 'Insufficient nodes (need >= 4)',
                'details': {},
                'violations': [],
            }

        undirected = subgraph.to_undirected()
        spanning_tree = nx.minimum_spanning_tree(undirected)
        code = _compute_prufer_code(spanning_tree)

        if not code:
            return {
                'name': 'prufer_entropy',
                'status': 'SKIP',
                'description': 'Prüfer entropy: Shannon entropy of the Prüfer code',
                'reason': 'Empty Prüfer code',
                'details': {},
                'violations': [],
            }

        # Shannon entropy of code value distribution
        from collections import Counter
        counts = Counter(code)
        total = len(code)
        entropy = -sum((c / total) * math.log2(c / total) for c in counts.values())

        # Maximum possible entropy = log2(n) (uniform over all n labels)
        max_entropy = math.log2(n) if n > 1 else 1.0
        entropy_ratio = entropy / max_entropy if max_entropy > 0 else 0.0

        threshold = max_entropy * entropy_ratio_threshold
        status = 'WARNING' if entropy > threshold else 'INFO'

        return {
            'name': 'prufer_entropy',
            'status': status,
            'description': 'Prüfer entropy: Shannon entropy of the Prüfer code',
            'details': {
                'nodes': n,
                'prufer_code_length': len(code),
                'unique_values_in_code': len(counts),
                'entropy': round(entropy, 4),
                'max_entropy': round(max_entropy, 4),
                'entropy_ratio': round(entropy_ratio, 4),
                'threshold': round(threshold, 4),
                'interpretation': (
                    'High entropy (> 0.9 * log(n)) means the spanning tree hub '
                    'structure is nearly random - no dominant hub nodes, '
                    'evenly distributed connectivity = complex architecture topology.'
                ),
            },
            'violations': (
                [f'Prüfer entropy {entropy:.4f} > threshold {threshold:.4f} '
                 f'(ratio {entropy_ratio:.4f} > {entropy_ratio_threshold})']
                if status == 'WARNING' else []
            ),
        }
    except Exception as e:
        return {
            'name': 'prufer_entropy',
            'status': 'ERROR',
            'description': 'Prüfer entropy: Shannon entropy of the Prüfer code',
            'details': {},
            'violations': [str(e)],
        }


def validate_prufer_similarity(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Prüfer Similarity - схожесть с базовой архитектурой.

    Сравнение кода Прюфера с эталонным (baseline) требует двух графов.
    В одиночном режиме валидатор сообщает об ограничении.

    Статус: INFO (baseline comparison requires two graphs).
    """
    return {
        'name': 'prufer_similarity',
        'status': 'INFO',
        'description': 'Prüfer similarity: baseline comparison of spanning tree structure',
        'details': {
            'interpretation': (
                'Prüfer sequence similarity compares two architecture graphs by '
                'encoding their spanning trees and measuring edit distance between '
                'the resulting sequences. Baseline comparison requires two graphs - '
                'run with a reference architecture to enable this check.'
            ),
            'baseline_required': True,
        },
        'violations': [],
    }


def validate_tree_isomorphism_class(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Tree Isomorphism Class - класс изоморфизма дерева.

    Два дерева с одинаковым отсортированным кодом Прюфера
    являются изоморфными (одинаковая степенная последовательность
    вершин как мультиграф).

    Вычисляет отсортированный код Прюфера как "отпечаток"
    класса изоморфизма остовного дерева.

    Статус: PASSED (дескриптивная метрика).
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)
        n = len(nodes)

        if n < 3:
            return {
                'name': 'tree_isomorphism_class',
                'status': 'SKIP',
                'description': 'Tree isomorphism class: sorted Prüfer code as structural fingerprint',
                'reason': 'Insufficient nodes (need >= 3)',
                'details': {},
                'violations': [],
            }

        undirected = subgraph.to_undirected()
        spanning_tree = nx.minimum_spanning_tree(undirected)
        code = _compute_prufer_code(spanning_tree)
        sorted_code = tuple(sorted(code))

        # The sorted code encodes how many times each node appears as a
        # "parent" in the Prüfer sequence, which equals (degree - 1).
        # This is equivalent to the degree sequence of the spanning tree.
        degree_seq = sorted(dict(spanning_tree.degree()).values(), reverse=True)

        return {
            'name': 'tree_isomorphism_class',
            'status': 'PASSED',
            'description': 'Tree isomorphism class: sorted Prüfer code as structural fingerprint',
            'details': {
                'nodes': n,
                'isomorphism_fingerprint': list(sorted_code),
                'degree_sequence': degree_seq,
                'interpretation': (
                    'Two spanning trees with the same sorted Prüfer code are isomorphic. '
                    'The fingerprint captures the degree distribution of the spanning tree: '
                    'high repeated values indicate hub-and-spoke topology.'
                ),
            },
            'violations': [],
        }
    except Exception as e:
        return {
            'name': 'tree_isomorphism_class',
            'status': 'ERROR',
            'description': 'Tree isomorphism class: sorted Prüfer code as structural fingerprint',
            'details': {},
            'violations': [str(e)],
        }


def validate_spanning_tree_coverage(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Spanning Tree Coverage - покрытие остовным деревом.

    Вычисляет отношение (n-1) / total_edges, где n - число вершин.
    Значение 1.0 означает, что граф является деревом.
    Значение < 1.0 означает наличие циклов / дополнительных рёбер.

    Статус: WARNING если coverage < 0.5
    (много лишних рёбер = нетривиальная, не-древовидная структура).
    """
    exclude = config.exclude if config else []
    coverage_threshold = getattr(config, 'spanning_tree_coverage_threshold', 0.5) if config else 0.5

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)
        n = len(nodes)
        total_edges = subgraph.number_of_edges()

        if n < 2:
            return {
                'name': 'spanning_tree_coverage',
                'status': 'SKIP',
                'description': 'Spanning tree coverage: ratio of tree edges to total edges',
                'reason': 'Insufficient nodes (need >= 2)',
                'details': {},
                'violations': [],
            }

        if total_edges == 0:
            return {
                'name': 'spanning_tree_coverage',
                'status': 'SKIP',
                'description': 'Spanning tree coverage: ratio of tree edges to total edges',
                'reason': 'No edges in graph',
                'details': {},
                'violations': [],
            }

        tree_edges = n - 1  # A spanning tree on n nodes has exactly n-1 edges
        coverage = tree_edges / total_edges

        status = 'WARNING' if coverage < coverage_threshold else 'INFO'

        return {
            'name': 'spanning_tree_coverage',
            'status': status,
            'description': 'Spanning tree coverage: ratio of tree edges to total edges',
            'details': {
                'nodes': n,
                'total_edges': total_edges,
                'spanning_tree_edges': tree_edges,
                'extra_edges': total_edges - tree_edges,
                'coverage': round(coverage, 4),
                'threshold': coverage_threshold,
                'interpretation': (
                    '1.0 = pure tree (no cycles). '
                    '< 0.5 = more than double the minimum edges required, '
                    'indicating dense, non-tree-like dependency graph. '
                    'Extra edges introduce potential for cyclic complexity.'
                ),
            },
            'violations': (
                [f'Spanning tree coverage {coverage:.4f} < threshold {coverage_threshold} '
                 f'({total_edges - tree_edges} extra edges beyond tree minimum)']
                if status == 'WARNING' else []
            ),
        }
    except Exception as e:
        return {
            'name': 'spanning_tree_coverage',
            'status': 'ERROR',
            'description': 'Spanning tree coverage: ratio of tree edges to total edges',
            'details': {},
            'violations': [str(e)],
        }


def validate_cayley_complexity_bound(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Cayley Complexity Bound - граница сложности по формуле Кэли.

    По формуле Кэли существует n^(n-2) различных помеченных деревьев
    на n вершинах. Это число выражает "архитектурную гибкость":
    сколько различных древовидных топологий принципиально возможно
    для данного числа компонентов.

    Статус: INFO (дескриптивная метрика).
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        n = len(nodes)

        if n < 2:
            return {
                'name': 'cayley_complexity_bound',
                'status': 'SKIP',
                'description': "Cayley complexity bound: n^(n-2) labeled trees on n vertices",
                'reason': 'Insufficient nodes (need >= 2)',
                'details': {},
                'violations': [],
            }

        if n == 2:
            cayley_bound = 1
            log_bound = 0.0
        else:
            # n^(n-2)
            # For large n, use logarithm to avoid overflow in display
            log_bound = (n - 2) * math.log10(n)
            # Only compute exact value for small n to avoid memory issues
            if n <= 20:
                cayley_bound = n ** (n - 2)
                cayley_str = str(cayley_bound)
            else:
                cayley_bound = None
                cayley_str = f'10^{log_bound:.1f} (too large to display exactly)'

        details: Dict[str, Any] = {
            'nodes': n,
            'cayley_exponent': n - 2,
            'log10_cayley_bound': round(log_bound, 2) if n > 2 else 0,
            'interpretation': (
                f'There are n^(n-2) = {n}^{n-2} possible labeled spanning trees '
                f'on {n} vertices (Cayley\'s formula). '
                'This is the "architectural flexibility bound": '
                'the number of structurally distinct tree topologies available '
                'for this set of components. Larger = more freedom, more complexity.'
            ),
        }

        if n <= 20:
            details['cayley_bound_exact'] = cayley_bound
        else:
            details['cayley_bound_approx'] = cayley_str

        return {
            'name': 'cayley_complexity_bound',
            'status': 'INFO',
            'description': "Cayley complexity bound: n^(n-2) labeled trees on n vertices",
            'details': details,
            'violations': [],
        }
    except Exception as e:
        return {
            'name': 'cayley_complexity_bound',
            'status': 'ERROR',
            'description': "Cayley complexity bound: n^(n-2) labeled trees on n vertices",
            'details': {},
            'violations': [str(e)],
        }
