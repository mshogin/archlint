"""
Advanced Graph Theory & Algebraic Structures Validations

Based on:
- Treewidth, Pathwidth
- Graph minors
- Chromatic number
- Dominating sets
- Lattice theory
"""

import networkx as nx
import numpy as np
from typing import Dict, List, Any, Optional, Set, TYPE_CHECKING
from collections import defaultdict

if TYPE_CHECKING:
    from validator.config import RuleConfig


def _get_violation_status(error_on_violation: bool) -> str:
    return 'ERROR' if error_on_violation else 'WARNING'


def _is_excluded(node: str, exclude_patterns: List[str]) -> bool:
    if not exclude_patterns:
        return False
    node_lower = node.lower()
    for pattern in exclude_patterns:
        pattern_lower = pattern.lower()
        if '*' in pattern_lower:
            parts = pattern_lower.split('*')
            if len(parts) == 2 and parts[0] and node_lower.startswith(parts[0]):
                return True
        elif pattern_lower in node_lower:
            return True
    return False


def validate_treewidth(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Treewidth - сложность декомпозиции графа в дерево.

    Низкий treewidth = граф близок к дереву = простая структура.
    """
    max_treewidth = 5
    if config and config.threshold is not None:
        max_treewidth = int(config.threshold)
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 2:
            return {
                'name': 'treewidth',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов'
            }

        # Приближение treewidth через дегенерируемость (degeneracy)
        # degeneracy <= treewidth <= degeneracy * something
        core_numbers = nx.core_number(subgraph)
        degeneracy = max(core_numbers.values()) if core_numbers else 0

        # Более точная оценка через минимальную степень заполнения
        # Используем degeneracy как нижнюю границу
        treewidth_lower = degeneracy
        treewidth_upper = degeneracy * 2  # Грубая верхняя оценка

        if treewidth_lower > max_treewidth:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'treewidth',
            'description': f'Treewidth <= {max_treewidth}',
            'status': status,
            'treewidth_lower_bound': treewidth_lower,
            'treewidth_upper_bound': treewidth_upper,
            'degeneracy': degeneracy,
            'threshold': max_treewidth,
            'interpretation': 'Низкий treewidth = древовидная структура'
        }
    except Exception as e:
        return {'name': 'treewidth', 'status': 'ERROR', 'error': str(e)}


def validate_chromatic_number(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Chromatic Number - минимальное число цветов для раскраски.

    χ(G) = минимальное разбиение на независимые множества.
    """
    max_chromatic = 5
    if config and config.threshold is not None:
        max_chromatic = int(config.threshold)
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 2:
            return {
                'name': 'chromatic_number',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов'
            }

        # Жадная раскраска как верхняя граница
        coloring = nx.greedy_color(subgraph, strategy='largest_first')
        chromatic_upper = max(coloring.values()) + 1 if coloring else 1

        # Максимальная клика как нижняя граница
        try:
            max_clique = max(len(c) for c in nx.find_cliques(subgraph))
        except Exception:
            max_clique = 1

        chromatic_lower = max_clique

        if chromatic_upper > max_chromatic:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        # Распределение по цветам
        color_counts = defaultdict(int)
        for node, color in coloring.items():
            color_counts[color] += 1

        return {
            'name': 'chromatic_number',
            'description': f'Хроматическое число <= {max_chromatic}',
            'status': status,
            'chromatic_lower_bound': chromatic_lower,
            'chromatic_upper_bound': chromatic_upper,
            'threshold': max_chromatic,
            'color_distribution': dict(sorted(color_counts.items())),
            'interpretation': 'Низкое χ = легко разбить на независимые группы'
        }
    except Exception as e:
        return {'name': 'chromatic_number', 'status': 'ERROR', 'error': str(e)}


def validate_dominating_set(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Dominating Set - минимальное множество узлов, покрывающих всех соседей.

    γ(G) = размер минимального доминирующего множества.
    """
    max_domination_ratio = 0.3
    if config and config.threshold is not None:
        max_domination_ratio = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 2:
            return {
                'name': 'dominating_set',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов'
            }

        # Жадное приближение доминирующего множества
        dominating = set()
        covered = set()
        remaining = set(nodes)

        while covered != set(nodes):
            # Выбираем узел, покрывающий больше всего непокрытых
            best_node = None
            best_coverage = 0

            for node in remaining:
                neighbors = set(subgraph.neighbors(node)) | {node}
                new_coverage = len(neighbors - covered)
                if new_coverage > best_coverage:
                    best_coverage = new_coverage
                    best_node = node

            if best_node is None:
                break

            dominating.add(best_node)
            covered |= set(subgraph.neighbors(best_node)) | {best_node}
            remaining.discard(best_node)

        domination_size = len(dominating)
        domination_ratio = domination_size / n if n > 0 else 0

        if domination_ratio > max_domination_ratio:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'dominating_set',
            'description': f'Доминирующее множество <= {max_domination_ratio:.0%}',
            'status': status,
            'domination_number': domination_size,
            'total_nodes': n,
            'domination_ratio': round(domination_ratio, 3),
            'threshold': max_domination_ratio,
            'dominating_nodes': list(dominating)[:10],
            'interpretation': 'Низкое γ = можно управлять через немногих'
        }
    except Exception as e:
        return {'name': 'dominating_set', 'status': 'ERROR', 'error': str(e)}


def validate_independence_number(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Independence Number - максимальное независимое множество.

    α(G) = максимальный набор узлов без связей между ними.
    """
    min_independence_ratio = 0.2
    if config and config.threshold is not None:
        min_independence_ratio = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 2:
            return {
                'name': 'independence_number',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов'
            }

        # Приближение максимального независимого множества
        # Используем жадный алгоритм
        independent = set()
        available = set(nodes)

        while available:
            # Выбираем узел с минимальной степенью
            min_degree_node = min(available, key=lambda x: subgraph.degree(x))
            independent.add(min_degree_node)

            # Удаляем узел и его соседей
            to_remove = {min_degree_node} | set(subgraph.neighbors(min_degree_node))
            available -= to_remove

        independence_size = len(independent)
        independence_ratio = independence_size / n if n > 0 else 0

        if independence_ratio < min_independence_ratio:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'independence_number',
            'description': f'Независимость >= {min_independence_ratio:.0%}',
            'status': status,
            'independence_number': independence_size,
            'total_nodes': n,
            'independence_ratio': round(independence_ratio, 3),
            'threshold': min_independence_ratio,
            'independent_nodes': list(independent)[:10],
            'interpretation': 'Высокое α = много несвязанных компонентов'
        }
    except Exception as e:
        return {'name': 'independence_number', 'status': 'ERROR', 'error': str(e)}


def validate_vertex_cover(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Vertex Cover - минимальное множество узлов, покрывающих все рёбра.

    τ(G) = минимальный размер вершинного покрытия.
    """
    max_cover_ratio = 0.7
    if config and config.threshold is not None:
        max_cover_ratio = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        e = subgraph.number_of_edges()

        if e == 0:
            return {
                'name': 'vertex_cover',
                'status': 'SKIP',
                'reason': 'Нет рёбер'
            }

        # 2-приближение вершинного покрытия
        cover = set()
        uncovered_edges = set(subgraph.edges())

        while uncovered_edges:
            # Берём произвольное ребро
            u, v = next(iter(uncovered_edges))
            cover.add(u)
            cover.add(v)

            # Удаляем покрытые рёбра
            uncovered_edges = {(a, b) for (a, b) in uncovered_edges
                               if a not in cover and b not in cover}

        cover_size = len(cover)
        cover_ratio = cover_size / n if n > 0 else 0

        if cover_ratio > max_cover_ratio:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'vertex_cover',
            'description': f'Вершинное покрытие <= {max_cover_ratio:.0%}',
            'status': status,
            'cover_size': cover_size,
            'total_nodes': n,
            'cover_ratio': round(cover_ratio, 3),
            'threshold': max_cover_ratio,
            'cover_nodes': list(cover)[:10],
            'interpretation': 'Низкое τ = рёбра сконцентрированы'
        }
    except Exception as e:
        return {'name': 'vertex_cover', 'status': 'ERROR', 'error': str(e)}


def validate_graph_density_distribution(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Graph Density Distribution - распределение плотности по подграфам.

    Анализ неравномерности плотности.
    """
    max_variance = 0.1
    if config and config.threshold is not None:
        max_variance = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 5:
            return {
                'name': 'graph_density_distribution',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов'
            }

        # Вычисляем локальную плотность для каждого узла
        local_densities = []

        for node in nodes:
            neighbors = list(subgraph.neighbors(node))
            k = len(neighbors)

            if k < 2:
                local_densities.append(0)
                continue

            # Подграф на соседях
            neighbor_subgraph = subgraph.subgraph(neighbors)
            local_edges = neighbor_subgraph.number_of_edges()
            max_edges = k * (k - 1) / 2

            local_density = local_edges / max_edges if max_edges > 0 else 0
            local_densities.append(local_density)

        if not local_densities:
            return {
                'name': 'graph_density_distribution',
                'status': 'SKIP',
                'reason': 'Нет данных'
            }

        avg_density = float(np.mean(local_densities))
        variance = float(np.var(local_densities))
        std_density = float(np.std(local_densities))

        if variance > max_variance:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'graph_density_distribution',
            'description': f'Дисперсия плотности <= {max_variance}',
            'status': status,
            'avg_local_density': round(avg_density, 3),
            'density_variance': round(variance, 4),
            'density_std': round(std_density, 3),
            'threshold': max_variance,
            'interpretation': 'Низкая дисперсия = равномерная структура'
        }
    except Exception as e:
        return {'name': 'graph_density_distribution', 'status': 'ERROR', 'error': str(e)}


def validate_graph_symmetry(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Graph Symmetry - симметрия графа через автоморфизмы.

    Высокая симметрия = регулярная структура.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 2:
            return {
                'name': 'graph_symmetry',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов'
            }

        # Приближение симметрии через degree sequence
        degrees = sorted([subgraph.degree(n) for n in nodes])
        degree_counts = defaultdict(int)
        for d in degrees:
            degree_counts[d] += 1

        # Количество классов эквивалентности
        equivalence_classes = len(degree_counts)

        # Симметрия = 1 - (classes / n)
        symmetry = 1 - (equivalence_classes / n) if n > 0 else 0

        # Анализ регулярности
        is_regular = len(degree_counts) == 1
        max_degree = max(degrees) if degrees else 0
        min_degree = min(degrees) if degrees else 0

        return {
            'name': 'graph_symmetry',
            'description': 'Анализ симметрии графа',
            'status': 'INFO',
            'symmetry_score': round(symmetry, 3),
            'equivalence_classes': equivalence_classes,
            'is_regular': is_regular,
            'degree_range': [min_degree, max_degree],
            'degree_distribution': dict(degree_counts),
            'interpretation': 'Высокая симметрия = регулярная структура'
        }
    except Exception as e:
        return {'name': 'graph_symmetry', 'status': 'ERROR', 'error': str(e)}
