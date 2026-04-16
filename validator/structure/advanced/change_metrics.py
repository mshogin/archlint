"""
Change Impact Analysis & Technical Debt Validations

Analyzes impact of changes and technical debt indicators.
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
        if pattern_lower.endswith('*'):
            if node_lower.startswith(pattern_lower[:-1]):
                return True
        elif pattern_lower.startswith('*'):
            if node_lower.endswith(pattern_lower[1:]):
                return True
        elif '*' in pattern_lower:
            parts = pattern_lower.split('*', 1)
            if node_lower.startswith(parts[0]) and node_lower.endswith(parts[1]):
                return True
        else:
            if pattern_lower in node_lower:
                return True
    return False


def validate_change_propagation(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Change Propagation - оценка распространения изменений (ripple effect).

    Метрика: для каждого узла считаем количество транзитивно зависящих узлов.
    """
    max_impact = 20
    if config and config.threshold is not None:
        max_impact = int(config.threshold)
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    try:
        violations = []
        impact_scores = []

        # Инвертируем граф для поиска зависящих узлов
        reverse_graph = graph.reverse()

        for node in graph.nodes():
            if _is_excluded(node, exclude):
                continue

            # Находим все узлы, которые транзитивно зависят от данного
            dependents = set(nx.descendants(reverse_graph, node))
            impact = len(dependents)

            impact_scores.append({'node': node, 'impact': impact})

            if impact > max_impact:
                violations.append({
                    'component': node,
                    'impact_radius': impact,
                    'sample_dependents': list(dependents)[:5]
                })

        violations.sort(key=lambda x: x['impact_radius'], reverse=True)
        impact_scores.sort(key=lambda x: x['impact'], reverse=True)

        avg_impact = float(np.mean([s['impact'] for s in impact_scores])) if impact_scores else 0

        if violations:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'change_propagation',
            'description': f'Радиус изменений <= {max_impact} компонентов',
            'status': status,
            'threshold': max_impact,
            'avg_impact': round(avg_impact, 2),
            'max_impact': impact_scores[0]['impact'] if impact_scores else 0,
            'top_impact': impact_scores[:5],
            'violations': violations[:10],
            'violations_count': len(violations)
        }
    except Exception as e:
        return {'name': 'change_propagation', 'status': 'ERROR', 'error': str(e)}


def validate_blast_radius(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Blast Radius - радиус поражения при сбое компонента.

    Комбинирует fan-in (кто зависит) и критичность (PageRank).
    """
    max_radius = 0.3
    if config and config.threshold is not None:
        max_radius = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    try:
        violations = []

        # Вычисляем PageRank для оценки критичности
        try:
            pagerank = nx.pagerank(graph)
        except Exception:
            pagerank = {n: 1.0 / len(graph.nodes()) for n in graph.nodes()}

        reverse_graph = graph.reverse()

        for node in graph.nodes():
            if _is_excluded(node, exclude):
                continue

            # Dependents count
            dependents = set(nx.descendants(reverse_graph, node))
            fan_in = len(dependents)

            # Blast radius = PageRank * normalized fan-in
            total_nodes = len(graph.nodes())
            normalized_fan_in = fan_in / total_nodes if total_nodes > 0 else 0
            pr = pagerank.get(node, 0)

            blast_radius = float((pr + normalized_fan_in) / 2)

            if blast_radius > max_radius:
                violations.append({
                    'component': node,
                    'blast_radius': round(blast_radius, 3),
                    'pagerank': round(pr, 3),
                    'dependents_count': fan_in
                })

        violations.sort(key=lambda x: x['blast_radius'], reverse=True)

        if violations:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'blast_radius',
            'description': f'Радиус поражения <= {max_radius}',
            'status': status,
            'threshold': max_radius,
            'violations': violations[:10],
            'violations_count': len(violations)
        }
    except Exception as e:
        return {'name': 'blast_radius', 'status': 'ERROR', 'error': str(e)}


def validate_hotspot_detection(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Hotspot Detection - компоненты с высокой связностью (потенциальные проблемные зоны).

    Hotspot = высокий fan-in + высокий fan-out.
    """
    threshold = 10
    if config and config.threshold is not None:
        threshold = int(config.threshold)
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    try:
        hotspots = []

        for node in graph.nodes():
            if _is_excluded(node, exclude):
                continue

            in_degree = graph.in_degree(node)
            out_degree = graph.out_degree(node)
            total_degree = in_degree + out_degree

            if total_degree > threshold * 2:
                hotspots.append({
                    'component': node,
                    'in_degree': in_degree,
                    'out_degree': out_degree,
                    'total_degree': total_degree,
                    'hotspot_score': round(total_degree / (threshold * 2), 2)
                })

        hotspots.sort(key=lambda x: x['total_degree'], reverse=True)

        if hotspots:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'hotspot_detection',
            'description': f'Компоненты с суммарной степенью <= {threshold * 2}',
            'status': status,
            'threshold': threshold * 2,
            'hotspots': hotspots[:10],
            'hotspots_count': len(hotspots)
        }
    except Exception as e:
        return {'name': 'hotspot_detection', 'status': 'ERROR', 'error': str(e)}


def validate_deprecated_usage(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Deprecated Usage - использование deprecated компонентов.
    """
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    deprecated_patterns = ['deprecated', 'legacy', 'old', 'obsolete', 'v1']

    if config and config.params:
        deprecated_patterns = config.params.get('patterns', deprecated_patterns)

    try:
        violations = []

        deprecated_nodes = set()

        # Находим deprecated узлы
        for node in graph.nodes():
            node_lower = node.lower()
            node_data = graph.nodes[node]

            if any(p in node_lower for p in deprecated_patterns):
                deprecated_nodes.add(node)
            elif node_data.get('deprecated'):
                deprecated_nodes.add(node)

        # Находим использования deprecated узлов
        for deprecated in deprecated_nodes:
            for source, _ in graph.in_edges(deprecated):
                if source not in deprecated_nodes and not _is_excluded(source, exclude):
                    violations.append({
                        'source': source,
                        'deprecated_target': deprecated
                    })

        if violations:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'deprecated_usage',
            'description': 'Отсутствие использования deprecated компонентов',
            'status': status,
            'deprecated_components': list(deprecated_nodes)[:10],
            'deprecated_count': len(deprecated_nodes),
            'violations': violations[:20],
            'violations_count': len(violations)
        }
    except Exception as e:
        return {'name': 'deprecated_usage', 'status': 'ERROR', 'error': str(e)}


def validate_stability_violations(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Stability Violations - стабильные компоненты не должны зависеть от нестабильных.

    Метрика Мартина: I = Ce / (Ca + Ce)
    """
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    try:
        violations = []

        # Вычисляем instability для каждого узла
        instability = {}
        for node in graph.nodes():
            ca = graph.in_degree(node)  # Afferent coupling
            ce = graph.out_degree(node)  # Efferent coupling
            total = ca + ce
            instability[node] = ce / total if total > 0 else 0.5

        # Проверяем SDP: зависимости от менее стабильных к более стабильным
        for source in graph.nodes():
            if _is_excluded(source, exclude):
                continue

            source_instability = instability[source]

            for _, target in graph.out_edges(source):
                target_instability = instability.get(target, 0.5)

                # Нарушение: стабильный зависит от нестабильного
                # source_instability < target_instability означает source стабильнее
                if source_instability < target_instability - 0.2:
                    violations.append({
                        'source': source,
                        'source_instability': round(source_instability, 2),
                        'target': target,
                        'target_instability': round(target_instability, 2),
                        'delta': round(target_instability - source_instability, 2)
                    })

        violations.sort(key=lambda x: x['delta'], reverse=True)

        if violations:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'stability_violations',
            'description': 'Зависимости направлены от нестабильных к стабильным (SDP)',
            'status': status,
            'violations': violations[:10],
            'violations_count': len(violations)
        }
    except Exception as e:
        return {'name': 'stability_violations', 'status': 'ERROR', 'error': str(e)}


def validate_circular_dependency_depth(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Circular Dependency Depth - глубина циклических зависимостей.
    """
    max_cycle_size = 3
    if config and config.threshold is not None:
        max_cycle_size = int(config.threshold)
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    try:
        # Находим все циклы
        try:
            from validator.utils.cycles import simple_cycles_bounded
            cycles = list(simple_cycles_bounded(graph, max_length=10))
        except Exception:
            cycles = []

        # Фильтруем большие циклы
        large_cycles = []
        for cycle in cycles:
            # Проверяем исключения
            if any(_is_excluded(n, exclude) for n in cycle):
                continue

            if len(cycle) > max_cycle_size:
                large_cycles.append({
                    'cycle': cycle,
                    'size': len(cycle)
                })

        large_cycles.sort(key=lambda x: x['size'], reverse=True)

        if large_cycles:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'circular_dependency_depth',
            'description': f'Циклы размером <= {max_cycle_size}',
            'status': status,
            'threshold': max_cycle_size,
            'total_cycles': len(cycles),
            'large_cycles': large_cycles[:10],
            'large_cycles_count': len(large_cycles)
        }
    except Exception as e:
        return {'name': 'circular_dependency_depth', 'status': 'ERROR', 'error': str(e)}


def validate_component_complexity(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Component Complexity - комплексная оценка сложности компонента.

    Combines: fan-in, fan-out, transitivity, centrality.
    """
    max_complexity = 50
    if config and config.threshold is not None:
        max_complexity = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    try:
        violations = []

        try:
            betweenness = nx.betweenness_centrality(graph)
        except Exception:
            betweenness = {n: 0 for n in graph.nodes()}

        for node in graph.nodes():
            if _is_excluded(node, exclude):
                continue

            in_deg = graph.in_degree(node)
            out_deg = graph.out_degree(node)
            bc = betweenness.get(node, 0)

            # Complexity score: weighted sum
            complexity = float(in_deg * 2 + out_deg * 3 + bc * 100)

            if complexity > max_complexity:
                violations.append({
                    'component': node,
                    'complexity': round(complexity, 2),
                    'in_degree': in_deg,
                    'out_degree': out_deg,
                    'betweenness': round(bc, 3)
                })

        violations.sort(key=lambda x: x['complexity'], reverse=True)

        if violations:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'component_complexity',
            'description': f'Комплексная сложность <= {max_complexity}',
            'status': status,
            'threshold': max_complexity,
            'violations': violations[:10],
            'violations_count': len(violations)
        }
    except Exception as e:
        return {'name': 'component_complexity', 'status': 'ERROR', 'error': str(e)}
