"""
Topology & Algebraic Topology Validations

Based on:
- Betti Numbers
- Euler Characteristic
- Simplicial Complexes
- Persistent Homology concepts
"""

import networkx as nx
import numpy as np
from typing import Dict, List, Any, Optional, Set, TYPE_CHECKING
from collections import defaultdict
from itertools import combinations

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
            if len(parts) == 2:
                if parts[0] and parts[1]:
                    if node_lower.startswith(parts[0]) and node_lower.endswith(parts[1]):
                        return True
                elif parts[0]:
                    if node_lower.startswith(parts[0]):
                        return True
                elif parts[1]:
                    if node_lower.endswith(parts[1]):
                        return True
        elif pattern_lower in node_lower:
            return True
    return False


def validate_betti_numbers(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Betti Numbers - топологические инварианты графа.

    β₀ = количество связных компонент
    β₁ = количество независимых циклов (cyclomatic complexity)
    β₂ = количество "полостей" (для 2D симплициального комплекса)
    """
    max_beta1 = 10
    if config and config.threshold is not None:
        max_beta1 = int(config.threshold)
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    try:
        # Фильтруем граф
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        # β₀ = количество компонент связности
        beta_0 = nx.number_connected_components(subgraph)

        # β₁ = E - V + β₀ (по формуле Эйлера для графов)
        # Это количество независимых циклов
        V = subgraph.number_of_nodes()
        E = subgraph.number_of_edges()
        beta_1 = E - V + beta_0

        # β₂ для графа обычно 0 (нет 2-симплексов), но можно посчитать клики
        # как приближение к 2-симплексам
        triangles = sum(nx.triangles(subgraph).values()) // 3
        beta_2_approx = triangles  # Приближение

        if beta_1 > max_beta1:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'betti_numbers',
            'description': f'Топологические инварианты: β₁ <= {max_beta1}',
            'status': status,
            'beta_0': beta_0,
            'beta_1': beta_1,
            'beta_2_approx': beta_2_approx,
            'vertices': V,
            'edges': E,
            'threshold': max_beta1,
            'interpretation': {
                'beta_0': 'Количество изолированных компонент',
                'beta_1': 'Количество независимых циклов (сложность)',
                'beta_2': 'Количество "полостей" (тесно связанные группы)'
            }
        }
    except Exception as e:
        return {'name': 'betti_numbers', 'status': 'ERROR', 'error': str(e)}


def validate_euler_characteristic(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Euler Characteristic - χ = V - E + F

    Для графа: χ = V - E + C (где C - количество компонент)
    Показывает "простоту" топологии.
    """
    min_chi = -5
    if config and config.threshold is not None:
        min_chi = int(config.threshold)
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        V = subgraph.number_of_nodes()
        E = subgraph.number_of_edges()
        C = nx.number_connected_components(subgraph)

        # Euler characteristic для графа
        chi = V - E + C

        # Для дерева χ = 1, для полного графа χ = 1 - (n-1)(n-2)/2
        # Отрицательные значения означают много циклов

        if chi < min_chi:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'euler_characteristic',
            'description': f'Характеристика Эйлера χ >= {min_chi}',
            'status': status,
            'euler_characteristic': chi,
            'vertices': V,
            'edges': E,
            'components': C,
            'threshold': min_chi,
            'interpretation': 'χ = 1 для дерева, отрицательные значения = много циклов'
        }
    except Exception as e:
        return {'name': 'euler_characteristic', 'status': 'ERROR', 'error': str(e)}


def validate_simplicial_complexity(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Simplicial Complexity - анализ симплициальных комплексов.

    0-симплексы = узлы
    1-симплексы = рёбра
    2-симплексы = треугольники
    n-симплексы = полные подграфы на n+1 вершинах
    """
    max_dimension = 4
    if config and config.threshold is not None:
        max_dimension = int(config.threshold)
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        # Считаем симплексы разных размерностей
        simplices = {0: len(nodes), 1: subgraph.number_of_edges()}

        # Находим клики (полные подграфы) - это и есть симплексы
        cliques = list(nx.find_cliques(subgraph))

        max_dim = 0
        for clique in cliques:
            dim = len(clique) - 1  # Размерность симплекса
            max_dim = max(max_dim, dim)
            if dim not in simplices:
                simplices[dim] = 0
            simplices[dim] += 1

        violations = []
        if max_dim > max_dimension:
            # Находим большие клики
            for clique in cliques:
                if len(clique) - 1 > max_dimension:
                    violations.append({
                        'dimension': len(clique) - 1,
                        'nodes': list(clique)[:10]
                    })

        if violations:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'simplicial_complexity',
            'description': f'Максимальная размерность симплексов <= {max_dimension}',
            'status': status,
            'max_dimension': max_dim,
            'threshold': max_dimension,
            'simplices_by_dimension': dict(sorted(simplices.items())),
            'violations': violations[:5],
            'interpretation': 'Высокая размерность = тесно связанные группы компонентов'
        }
    except Exception as e:
        return {'name': 'simplicial_complexity', 'status': 'ERROR', 'error': str(e)}


def validate_topological_persistence(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Topological Persistence - устойчивость топологических свойств.

    Симуляция удаления рёбер и отслеживание изменения β₀, β₁.
    Показывает, насколько архитектура устойчива к изменениям.
    """
    threshold = 0.3
    if config and config.threshold is not None:
        threshold = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        edges = list(subgraph.edges())
        n_edges = len(edges)

        if n_edges == 0:
            return {
                'name': 'topological_persistence',
                'status': 'SKIP',
                'reason': 'Нет рёбер для анализа'
            }

        # Начальные топологические характеристики
        initial_components = nx.number_connected_components(subgraph)

        # Симулируем удаление рёбер и отслеживаем изменения
        fragility_scores = []

        # Проверяем каждое ребро на критичность
        critical_edges = []
        for edge in edges:
            test_graph = subgraph.copy()
            test_graph.remove_edge(*edge)
            new_components = nx.number_connected_components(test_graph)

            if new_components > initial_components:
                critical_edges.append({
                    'edge': edge,
                    'components_added': new_components - initial_components
                })

        fragility = len(critical_edges) / n_edges if n_edges > 0 else 0

        if fragility > threshold:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'topological_persistence',
            'description': f'Топологическая хрупкость <= {threshold:.0%}',
            'status': status,
            'fragility': round(fragility, 3),
            'threshold': threshold,
            'total_edges': n_edges,
            'critical_edges': len(critical_edges),
            'critical_edges_sample': critical_edges[:5],
            'interpretation': 'Высокая хрупкость = много критических зависимостей'
        }
    except Exception as e:
        return {'name': 'topological_persistence', 'status': 'ERROR', 'error': str(e)}


def validate_homological_density(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Homological Density - плотность гомологических групп.

    Отношение циклов к возможным циклам.
    """
    max_density = 0.5
    if config and config.threshold is not None:
        max_density = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        V = subgraph.number_of_nodes()
        E = subgraph.number_of_edges()

        if V <= 1:
            return {
                'name': 'homological_density',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов'
            }

        # Максимально возможное количество рёбер
        max_edges = V * (V - 1) // 2

        # β₁ = E - V + C
        C = nx.number_connected_components(subgraph)
        beta_1 = E - V + C

        # Максимально возможный β₁ для полного графа
        max_beta_1 = max_edges - V + 1 if max_edges > V else 0

        homological_density = beta_1 / max_beta_1 if max_beta_1 > 0 else 0

        if homological_density > max_density:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'homological_density',
            'description': f'Гомологическая плотность <= {max_density:.0%}',
            'status': status,
            'density': round(homological_density, 3),
            'beta_1': beta_1,
            'max_beta_1': max_beta_1,
            'threshold': max_density
        }
    except Exception as e:
        return {'name': 'homological_density', 'status': 'ERROR', 'error': str(e)}
