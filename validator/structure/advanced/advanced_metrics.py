"""
Advanced Architecture Validations

Based on:
- Graph Theory
- Statistics
- Spectral Analysis
- Information Theory
- Software Architecture Metrics
"""

import networkx as nx
import numpy as np
from typing import Dict, List, Any, Optional, TYPE_CHECKING
from collections import Counter
import math

if TYPE_CHECKING:
    from validator.config import RuleConfig


def _get_violation_status(error_on_violation: bool) -> str:
    return 'ERROR' if error_on_violation else 'WARNING'


def _to_python(val):
    """Convert numpy types to Python types for YAML serialization"""
    if isinstance(val, (np.integer, np.floating)):
        return float(val)
    if isinstance(val, np.bool_):
        return bool(val)
    if isinstance(val, np.ndarray):
        return val.tolist()
    return val


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


# =============================================================================
# GRAPH THEORY VALIDATIONS (25-34)
# =============================================================================

def validate_clustering_coefficient(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Clustering Coefficient - плотность связей внутри локального окружения.
    C(v) = 2 * edges_between_neighbors / (k * (k-1))
    """
    min_threshold = 0.1
    if config and config.threshold is not None:
        min_threshold = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        undirected = graph.to_undirected()
        clustering = nx.clustering(undirected)

        # Фильтруем исключения
        filtered = {k: v for k, v in clustering.items() if not _is_excluded(k, exclude)}

        if not filtered:
            return {
                'name': 'clustering_coefficient',
                'description': 'Коэффициент кластеризации узлов',
                'status': 'SKIP',
                'reason': 'Нет узлов для анализа'
            }

        avg_clustering = float(np.mean(list(filtered.values())))

        # Узлы с низким clustering (изолированные)
        low_clustering = [
            {'node': k, 'coefficient': round(float(v), 3)}
            for k, v in filtered.items()
            if v < min_threshold and graph.degree(k) > 1
        ]

        if avg_clustering < min_threshold:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'clustering_coefficient',
            'description': f'Средний коэффициент кластеризации >= {min_threshold}',
            'status': status,
            'avg_clustering': round(avg_clustering, 3),
            'threshold': min_threshold,
            'low_clustering_nodes': low_clustering[:10],
            'low_clustering_count': len(low_clustering)
        }
    except Exception as e:
        return {
            'name': 'clustering_coefficient',
            'status': 'ERROR',
            'error': str(e)
        }


def validate_edge_density(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Edge Density - отношение рёбер к максимально возможным.
    density = E / (V * (V-1))
    """
    max_threshold = 0.3
    min_threshold = 0.01
    if config and config.params:
        max_threshold = config.params.get('max_threshold', max_threshold)
        min_threshold = config.params.get('min_threshold', min_threshold)
    error_on_violation = config.error_on_violation if config else False

    try:
        density = nx.density(graph)

        if density > max_threshold:
            status = _get_violation_status(error_on_violation)
            issue = f'Слишком высокая плотность (монолитность) > {max_threshold}'
        elif density < min_threshold:
            status = 'INFO'
            issue = f'Низкая плотность (фрагментация) < {min_threshold}'
        else:
            status = 'PASSED'
            issue = None

        result = {
            'name': 'edge_density',
            'description': f'Плотность графа между {min_threshold} и {max_threshold}',
            'status': status,
            'density': round(density, 4),
            'nodes': len(graph.nodes()),
            'edges': len(graph.edges()),
            'max_possible_edges': len(graph.nodes()) * (len(graph.nodes()) - 1)
        }
        if issue:
            result['issue'] = issue
        return result
    except Exception as e:
        return {'name': 'edge_density', 'status': 'ERROR', 'error': str(e)}


def validate_articulation_points(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Articulation Points - узлы, удаление которых разбивает граф.
    """
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        undirected = graph.to_undirected()
        articulation = list(nx.articulation_points(undirected))

        # Фильтруем исключения
        filtered = [n for n in articulation if not _is_excluded(n, exclude)]

        if not filtered:
            status = 'PASSED'
        else:
            status = _get_violation_status(error_on_violation)

        return {
            'name': 'articulation_points',
            'description': 'Критические узлы (удаление разбивает граф)',
            'status': status,
            'articulation_points': filtered[:20],
            'count': len(filtered),
            'recommendation': 'Уменьшить зависимость от критических узлов' if filtered else None
        }
    except Exception as e:
        return {'name': 'articulation_points', 'status': 'ERROR', 'error': str(e)}


def validate_bridge_edges(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Bridge Edges - рёбра, удаление которых разбивает граф.
    """
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        undirected = graph.to_undirected()
        bridges = list(nx.bridges(undirected))

        # Фильтруем исключения
        filtered = [
            (u, v) for u, v in bridges
            if not _is_excluded(u, exclude) and not _is_excluded(v, exclude)
        ]

        if not filtered:
            status = 'PASSED'
        else:
            status = 'INFO'  # Bridges часто нормальны

        return {
            'name': 'bridge_edges',
            'description': 'Критические рёбра (удаление разбивает граф)',
            'status': status,
            'bridges': [{'from': u, 'to': v} for u, v in filtered[:20]],
            'count': len(filtered)
        }
    except Exception as e:
        return {'name': 'bridge_edges', 'status': 'ERROR', 'error': str(e)}


def validate_graph_diameter(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Graph Diameter - максимальное расстояние между узлами.
    """
    max_diameter = 10
    if config and config.threshold is not None:
        max_diameter = int(config.threshold)
    error_on_violation = config.error_on_violation if config else True

    try:
        # Проверяем связность
        if not nx.is_weakly_connected(graph):
            components = list(nx.weakly_connected_components(graph))
            return {
                'name': 'graph_diameter',
                'description': 'Диаметр графа',
                'status': 'INFO',
                'diameter': 'infinity',
                'reason': f'Граф не связан ({len(components)} компонент)'
            }

        # Для directed графа используем underlying undirected
        undirected = graph.to_undirected()
        if not nx.is_connected(undirected):
            return {
                'name': 'graph_diameter',
                'status': 'SKIP',
                'reason': 'Граф не связан'
            }

        diameter = nx.diameter(undirected)

        if diameter > max_diameter:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'graph_diameter',
            'description': f'Диаметр графа должен быть <= {max_diameter}',
            'status': status,
            'diameter': diameter,
            'threshold': max_diameter
        }
    except Exception as e:
        return {'name': 'graph_diameter', 'status': 'ERROR', 'error': str(e)}


def validate_avg_path_length(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Average Path Length - средняя длина пути.
    """
    max_avg = 5.0
    if config and config.threshold is not None:
        max_avg = config.threshold
    error_on_violation = config.error_on_violation if config else False

    try:
        undirected = graph.to_undirected()
        if not nx.is_connected(undirected):
            # Берём наибольшую компоненту
            largest_cc = max(nx.connected_components(undirected), key=len)
            subgraph = undirected.subgraph(largest_cc)
            avg_path = nx.average_shortest_path_length(subgraph)
            partial = True
        else:
            avg_path = nx.average_shortest_path_length(undirected)
            partial = False

        if avg_path > max_avg:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        result = {
            'name': 'avg_path_length',
            'description': f'Средняя длина пути <= {max_avg}',
            'status': status,
            'avg_path_length': round(avg_path, 3),
            'threshold': max_avg
        }
        if partial:
            result['note'] = 'Вычислено для наибольшей связной компоненты'
        return result
    except Exception as e:
        return {'name': 'avg_path_length', 'status': 'ERROR', 'error': str(e)}


def validate_closeness_centrality(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Closeness Centrality - близость узла ко всем остальным.
    """
    threshold = 0.5
    if config and config.threshold is not None:
        threshold = config.threshold
    exclude = config.exclude if config else []

    try:
        centrality = nx.closeness_centrality(graph)
        filtered = {k: v for k, v in centrality.items() if not _is_excluded(k, exclude)}

        # Сортируем по centrality
        sorted_nodes = sorted(filtered.items(), key=lambda x: x[1], reverse=True)

        central = [{'node': k, 'closeness': round(v, 3)} for k, v in sorted_nodes if v > threshold]
        peripheral = [{'node': k, 'closeness': round(v, 3)} for k, v in sorted_nodes if v < 0.2]

        return {
            'name': 'closeness_centrality',
            'description': 'Близость узлов к остальным',
            'status': 'INFO',
            'central_nodes': central[:10],
            'peripheral_nodes': peripheral[:10],
            'avg_closeness': round(float(np.mean(list(filtered.values()))), 3) if filtered else 0
        }
    except Exception as e:
        return {'name': 'closeness_centrality', 'status': 'ERROR', 'error': str(e)}


def validate_eigenvector_centrality(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Eigenvector Centrality - влиятельность узла.
    """
    threshold = 0.3
    if config and config.threshold is not None:
        threshold = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        try:
            centrality = nx.eigenvector_centrality(graph, max_iter=1000)
        except nx.PowerIterationFailedConvergence:
            centrality = nx.eigenvector_centrality_numpy(graph)

        filtered = {k: v for k, v in centrality.items() if not _is_excluded(k, exclude)}
        sorted_nodes = sorted(filtered.items(), key=lambda x: x[1], reverse=True)

        influential = [{'node': k, 'eigenvector': round(v, 3)} for k, v in sorted_nodes if v > threshold]

        if influential:
            status = 'INFO'
        else:
            status = 'PASSED'

        return {
            'name': 'eigenvector_centrality',
            'description': 'Влиятельность узлов',
            'status': status,
            'threshold': threshold,
            'influential_nodes': influential[:10],
            'influential_count': len(influential),
            'top_5': [{'node': k, 'eigenvector': round(v, 3)} for k, v in sorted_nodes[:5]]
        }
    except Exception as e:
        return {'name': 'eigenvector_centrality', 'status': 'ERROR', 'error': str(e)}


def validate_k_core_decomposition(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    K-Core Decomposition - разбиение на k-ядра.
    """
    max_core = 5
    if config and config.threshold is not None:
        max_core = int(config.threshold)
    error_on_violation = config.error_on_violation if config else False

    try:
        undirected = graph.to_undirected()
        core_numbers = nx.core_number(undirected)

        # Распределение по ядрам
        core_distribution = Counter(core_numbers.values())
        max_k = max(core_numbers.values()) if core_numbers else 0

        # Узлы в высоких ядрах
        high_core_nodes = [n for n, k in core_numbers.items() if k >= max_core]

        if max_k > max_core:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'k_core_decomposition',
            'description': f'Максимальное k-ядро <= {max_core}',
            'status': status,
            'max_k_core': max_k,
            'threshold': max_core,
            'core_distribution': dict(sorted(core_distribution.items())),
            'high_core_nodes': high_core_nodes[:20],
            'high_core_count': len(high_core_nodes)
        }
    except Exception as e:
        return {'name': 'k_core_decomposition', 'status': 'ERROR', 'error': str(e)}


def validate_graph_cliques(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Graph Cliques - обнаружение полных подграфов.
    """
    max_clique_size = 4
    if config and config.threshold is not None:
        max_clique_size = int(config.threshold)
    error_on_violation = config.error_on_violation if config else True

    try:
        undirected = graph.to_undirected()
        cliques = list(nx.find_cliques(undirected))

        # Фильтруем большие клики
        large_cliques = [c for c in cliques if len(c) > max_clique_size]

        if large_cliques:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'graph_cliques',
            'description': f'Клики размером <= {max_clique_size}',
            'status': status,
            'max_clique_size': max(len(c) for c in cliques) if cliques else 0,
            'threshold': max_clique_size,
            'large_cliques': [list(c) for c in large_cliques[:5]],
            'large_cliques_count': len(large_cliques),
            'total_cliques': len(cliques)
        }
    except Exception as e:
        return {'name': 'graph_cliques', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# STATISTICS VALIDATIONS (35-40)
# =============================================================================

def validate_degree_distribution(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Degree Distribution Analysis - анализ распределения степеней.
    """
    try:
        degrees = [d for _, d in graph.degree()]
        if not degrees:
            return {'name': 'degree_distribution', 'status': 'SKIP', 'reason': 'Нет узлов'}

        # Статистики
        mean_deg = float(np.mean(degrees))
        std_deg = float(np.std(degrees))
        max_deg = int(max(degrees))
        min_deg = int(min(degrees))

        # Распределение
        degree_count = Counter(degrees)

        # Проверка на power-law (scale-free)
        # Упрощённая проверка: variance >> mean
        is_scale_free = bool(std_deg > mean_deg)

        return {
            'name': 'degree_distribution',
            'description': 'Распределение степеней узлов',
            'status': 'INFO',
            'mean_degree': round(mean_deg, 2),
            'std_degree': round(std_deg, 2),
            'max_degree': max_deg,
            'min_degree': min_deg,
            'distribution': dict(sorted(degree_count.items())[:20]),
            'is_scale_free': is_scale_free,
            'interpretation': 'Scale-free сеть (hub-dominated)' if is_scale_free else 'Относительно равномерное распределение'
        }
    except Exception as e:
        return {'name': 'degree_distribution', 'status': 'ERROR', 'error': str(e)}


def validate_dependency_entropy(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Dependency Entropy - энтропия распределения зависимостей.
    H = -Σ p(i) * log2(p(i))
    """
    min_entropy = 2.0
    if config and config.threshold is not None:
        min_entropy = config.threshold
    error_on_violation = config.error_on_violation if config else False

    try:
        out_degrees = [d for _, d in graph.out_degree()]
        if not out_degrees or sum(out_degrees) == 0:
            return {'name': 'dependency_entropy', 'status': 'SKIP', 'reason': 'Нет зависимостей'}

        total = sum(out_degrees)
        probabilities = [d / total for d in out_degrees if d > 0]

        entropy = -sum(p * math.log2(p) for p in probabilities if p > 0)
        max_entropy = math.log2(len(out_degrees)) if len(out_degrees) > 1 else 1
        normalized_entropy = entropy / max_entropy if max_entropy > 0 else 0

        if normalized_entropy < min_entropy / max_entropy:
            status = _get_violation_status(error_on_violation)
            issue = 'Низкая энтропия - неравномерное распределение зависимостей'
        else:
            status = 'PASSED'
            issue = None

        result = {
            'name': 'dependency_entropy',
            'description': 'Энтропия распределения зависимостей',
            'status': status,
            'entropy': round(entropy, 3),
            'max_entropy': round(max_entropy, 3),
            'normalized_entropy': round(normalized_entropy, 3)
        }
        if issue:
            result['issue'] = issue
        return result
    except Exception as e:
        return {'name': 'dependency_entropy', 'status': 'ERROR', 'error': str(e)}


def validate_gini_coefficient(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Gini Coefficient - неравенство распределения зависимостей.
    """
    max_gini = 0.6
    if config and config.threshold is not None:
        max_gini = config.threshold
    error_on_violation = config.error_on_violation if config else True

    try:
        degrees = sorted([d for _, d in graph.degree()])
        n = len(degrees)
        if n == 0:
            return {'name': 'gini_coefficient', 'status': 'SKIP', 'reason': 'Нет узлов'}

        # Gini coefficient calculation
        cumsum = np.cumsum(degrees)
        total = float(cumsum[-1])
        if total == 0:
            gini = 0.0
        else:
            gini = float(1 - 2 * sum(cumsum) / (n * total) + 1/n)

        if gini > max_gini:
            status = _get_violation_status(error_on_violation)
            issue = f'Высокое неравенство (God Objects): Gini = {round(gini, 3)}'
        else:
            status = 'PASSED'
            issue = None

        result = {
            'name': 'gini_coefficient',
            'description': f'Коэффициент Джини <= {max_gini}',
            'status': status,
            'gini': round(gini, 3),
            'threshold': max_gini,
            'interpretation': 'Равномерно' if gini < 0.3 else 'Умеренно' if gini < 0.5 else 'Неравномерно'
        }
        if issue:
            result['issue'] = issue
        return result
    except Exception as e:
        return {'name': 'gini_coefficient', 'status': 'ERROR', 'error': str(e)}


def validate_zscore_outliers(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Z-Score Outlier Detection - обнаружение выбросов.
    """
    z_threshold = 3.0
    if config and config.threshold is not None:
        z_threshold = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    try:
        degrees = {n: d for n, d in graph.degree() if not _is_excluded(n, exclude)}
        if len(degrees) < 3:
            return {'name': 'zscore_outliers', 'status': 'SKIP', 'reason': 'Недостаточно данных'}

        values = list(degrees.values())
        mean = float(np.mean(values))
        std = float(np.std(values))

        if std == 0:
            return {
                'name': 'zscore_outliers',
                'status': 'PASSED',
                'description': 'Нет выбросов (все степени одинаковы)'
            }

        outliers = []
        for node, degree in degrees.items():
            z = float((degree - mean) / std)
            if abs(z) > z_threshold:
                outliers.append({
                    'node': node,
                    'degree': int(degree),
                    'z_score': round(z, 2)
                })

        outliers.sort(key=lambda x: abs(x['z_score']), reverse=True)

        if outliers:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'zscore_outliers',
            'description': f'Узлы с |Z| > {z_threshold}',
            'status': status,
            'outliers': outliers[:10],
            'outliers_count': len(outliers),
            'mean_degree': round(mean, 2),
            'std_degree': round(std, 2),
            'z_threshold': z_threshold
        }
    except Exception as e:
        return {'name': 'zscore_outliers', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# SPECTRAL VALIDATIONS (41-44)
# =============================================================================

def validate_algebraic_connectivity(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Algebraic Connectivity (Fiedler Value) - второе собственное значение лапласиана.
    """
    min_connectivity = 0.1
    if config and config.threshold is not None:
        min_connectivity = config.threshold
    error_on_violation = config.error_on_violation if config else False

    try:
        undirected = graph.to_undirected()
        if not nx.is_connected(undirected):
            return {
                'name': 'algebraic_connectivity',
                'status': 'INFO',
                'algebraic_connectivity': 0,
                'reason': 'Граф не связан (λ₂ = 0)'
            }

        connectivity = nx.algebraic_connectivity(undirected)

        if connectivity < min_connectivity:
            status = _get_violation_status(error_on_violation)
            issue = 'Слабая связность - граф легко разбить'
        else:
            status = 'PASSED'
            issue = None

        result = {
            'name': 'algebraic_connectivity',
            'description': f'Алгебраическая связность >= {min_connectivity}',
            'status': status,
            'algebraic_connectivity': round(connectivity, 4),
            'threshold': min_connectivity,
            'interpretation': 'Хорошо связан' if connectivity > 0.5 else 'Слабо связан'
        }
        if issue:
            result['issue'] = issue
        return result
    except Exception as e:
        return {'name': 'algebraic_connectivity', 'status': 'ERROR', 'error': str(e)}


def validate_spectral_radius(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Spectral Radius - максимальное собственное значение матрицы смежности.
    """
    try:
        adj_matrix = nx.adjacency_matrix(graph).todense()
        eigenvalues = np.linalg.eigvals(adj_matrix)
        spectral_radius = float(max(abs(eigenvalues)).real)

        max_degree = int(max(d for _, d in graph.degree())) if graph.nodes() else 0

        return {
            'name': 'spectral_radius',
            'description': 'Спектральный радиус матрицы смежности',
            'status': 'INFO',
            'spectral_radius': round(spectral_radius, 3),
            'max_degree': max_degree,
            'interpretation': 'Коррелирует с максимальной степенью'
        }
    except Exception as e:
        return {'name': 'spectral_radius', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# SOFTWARE ARCHITECTURE VALIDATIONS
# =============================================================================

def validate_cohesion_lcom4(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    LCOM4 - Lack of Cohesion of Methods.
    Считает компоненты связности внутри типов.
    """
    max_lcom = 1
    if config and config.threshold is not None:
        max_lcom = int(config.threshold)
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    try:
        # Группируем методы по типам
        types_methods: Dict[str, List[str]] = {}
        for node in graph.nodes():
            parts = node.split('.')
            if len(parts) >= 3:  # pkg.Type.Method
                type_id = '.'.join(parts[:-1])
                if not _is_excluded(type_id, exclude):
                    if type_id not in types_methods:
                        types_methods[type_id] = []
                    types_methods[type_id].append(node)

        violations = []
        for type_id, methods in types_methods.items():
            if len(methods) < 2:
                continue

            # Строим подграф методов
            subgraph = graph.subgraph(methods).to_undirected()
            components = list(nx.connected_components(subgraph))
            lcom = len(components)

            if lcom > max_lcom:
                violations.append({
                    'type': type_id,
                    'lcom4': lcom,
                    'methods_count': len(methods)
                })

        if violations:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'cohesion_lcom4',
            'description': f'LCOM4 <= {max_lcom} (связность методов)',
            'status': status,
            'violations': violations[:10],
            'violations_count': len(violations),
            'types_analyzed': len(types_methods)
        }
    except Exception as e:
        return {'name': 'cohesion_lcom4', 'status': 'ERROR', 'error': str(e)}


def validate_interface_segregation(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Interface Segregation - проверка размера интерфейсов.
    """
    max_methods = 5
    if config and config.threshold is not None:
        max_methods = int(config.threshold)
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    try:
        # Считаем методы для каждого типа
        types_size: Dict[str, int] = {}
        for node in graph.nodes():
            data = graph.nodes[node]
            if data.get('entity') == 'method':
                type_id = '.'.join(node.split('.')[:-1])
                if not _is_excluded(type_id, exclude):
                    types_size[type_id] = types_size.get(type_id, 0) + 1

        violations = [
            {'type': t, 'methods': c}
            for t, c in types_size.items()
            if c > max_methods
        ]
        violations.sort(key=lambda x: x['methods'], reverse=True)

        if violations:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'interface_segregation',
            'description': f'Типы с <= {max_methods} методами (ISP)',
            'status': status,
            'threshold': max_methods,
            'violations': violations[:10],
            'violations_count': len(violations)
        }
    except Exception as e:
        return {'name': 'interface_segregation', 'status': 'ERROR', 'error': str(e)}
