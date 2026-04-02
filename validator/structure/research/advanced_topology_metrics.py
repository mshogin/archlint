"""
Advanced Topology & Topological Data Analysis (TDA) Validations

Based on:
- Persistent Homology (TDA)
- Discrete Morse Theory
- Sheaf Theory on Graphs
- Discrete Curvature (Forman, Ollivier-Ricci)
- Homotopy Theory
- Filtration Analysis
- Hodge Decomposition
"""

import networkx as nx
import numpy as np
from typing import Dict, List, Any, Optional, Set, Tuple, TYPE_CHECKING
from collections import defaultdict
from itertools import combinations
import heapq

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


# =============================================================================
# PERSISTENT HOMOLOGY (TDA)
# =============================================================================

def validate_persistent_homology(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Persistent Homology - полный TDA анализ.

    Строим фильтрацию по весам рёбер и отслеживаем рождение/смерть
    топологических признаков (компонент, циклов).

    Birth-Death диаграмма показывает устойчивость структур.
    """
    max_short_lived = 5
    if config and config.threshold is not None:
        max_short_lived = int(config.threshold)
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 3:
            return {
                'name': 'persistent_homology',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов для TDA'
            }

        # Создаём фильтрацию на основе степени узлов
        # Рёбра добавляются в порядке убывания минимальной степени конечных точек
        edges_with_weight = []
        for u, v in subgraph.edges():
            weight = min(subgraph.degree(u), subgraph.degree(v))
            edges_with_weight.append((weight, u, v))

        edges_with_weight.sort(reverse=True)

        # Union-Find для отслеживания компонент (β₀)
        parent = {node: node for node in nodes}
        rank = {node: 0 for node in nodes}

        def find(x):
            if parent[x] != x:
                parent[x] = find(parent[x])
            return parent[x]

        def union(x, y):
            px, py = find(x), find(y)
            if px == py:
                return False
            if rank[px] < rank[py]:
                px, py = py, px
            parent[py] = px
            if rank[px] == rank[py]:
                rank[px] += 1
            return True

        # Барcode для β₀ (компоненты)
        barcode_h0 = []  # (birth, death, component)
        component_birth = {node: 0 for node in nodes}  # Все компоненты рождаются в 0

        # Барcode для β₁ (циклы)
        barcode_h1 = []

        current_edges = 0
        current_vertices = n
        current_components = n

        for idx, (weight, u, v) in enumerate(edges_with_weight):
            filtration_value = idx + 1

            if union(u, v):
                # Компоненты слились - одна "умирает"
                # Та что родилась позже - умирает
                death_time = filtration_value
                barcode_h0.append({
                    'birth': 0,
                    'death': death_time,
                    'persistence': death_time,
                    'type': 'component_merge'
                })
                current_components -= 1
            else:
                # Цикл создан
                barcode_h1.append({
                    'birth': filtration_value,
                    'death': float('inf'),  # Цикл живёт до конца
                    'persistence': float('inf'),
                    'edge': (u, v)
                })

        # Одна компонента живёт бесконечно
        barcode_h0.append({
            'birth': 0,
            'death': float('inf'),
            'persistence': float('inf'),
            'type': 'final_component'
        })

        # Анализ персистентности
        finite_h0 = [b for b in barcode_h0 if b['persistence'] != float('inf')]
        short_lived_h0 = [b for b in finite_h0 if b['persistence'] <= 2]

        # Статистика
        avg_persistence_h0 = np.mean([b['persistence'] for b in finite_h0]) if finite_h0 else 0

        if len(short_lived_h0) > max_short_lived:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'persistent_homology',
            'description': f'Короткоживущих компонент <= {max_short_lived}',
            'status': status,
            'barcode_h0_count': len(barcode_h0),
            'barcode_h1_count': len(barcode_h1),
            'short_lived_components': len(short_lived_h0),
            'threshold': max_short_lived,
            'avg_persistence_h0': round(avg_persistence_h0, 3),
            'cycles_created': len(barcode_h1),
            'interpretation': 'Короткоживущие = нестабильные структурные элементы'
        }
    except Exception as e:
        return {'name': 'persistent_homology', 'status': 'ERROR', 'error': str(e)}


def validate_persistence_diagram(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Persistence Diagram Analysis - анализ диаграммы персистентности.

    Точки далеко от диагонали = устойчивые структуры.
    Точки близко к диагонали = шум.
    """
    noise_threshold = 0.3
    if config and config.threshold is not None:
        noise_threshold = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        e = subgraph.number_of_edges()

        if n < 3:
            return {
                'name': 'persistence_diagram',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов'
            }

        # Строим persistence diagram на основе edge betweenness
        edge_betweenness = nx.edge_betweenness_centrality(subgraph)

        # Фильтрация: добавляем рёбра в порядке убывания betweenness
        sorted_edges = sorted(edge_betweenness.items(), key=lambda x: -x[1])

        # Симулируем добавление рёбер
        diagram_points = []
        temp_graph = nx.Graph()
        temp_graph.add_nodes_from(nodes)

        for idx, ((u, v), betw) in enumerate(sorted_edges):
            birth = idx / len(sorted_edges) if sorted_edges else 0

            # Проверяем, создаёт ли ребро цикл
            if temp_graph.has_node(u) and temp_graph.has_node(v):
                if nx.has_path(temp_graph, u, v):
                    # Создаётся цикл - H1 feature
                    diagram_points.append({
                        'dimension': 1,
                        'birth': birth,
                        'death': 1.0,  # Живёт до конца
                        'persistence': 1.0 - birth,
                        'edge': (u, v)
                    })

            temp_graph.add_edge(u, v)

        # Анализ диаграммы
        if diagram_points:
            persistences = [p['persistence'] for p in diagram_points]
            avg_persistence = np.mean(persistences)
            max_persistence = max(persistences)

            # Шум = точки с низкой персистентностью
            noise_points = [p for p in diagram_points if p['persistence'] < noise_threshold]
            noise_ratio = len(noise_points) / len(diagram_points)
        else:
            avg_persistence = 0
            max_persistence = 0
            noise_ratio = 0

        if noise_ratio > 0.5:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'persistence_diagram',
            'description': f'Анализ диаграммы персистентности',
            'status': status,
            'total_points': len(diagram_points),
            'avg_persistence': round(avg_persistence, 3),
            'max_persistence': round(max_persistence, 3),
            'noise_ratio': round(noise_ratio, 3),
            'noise_threshold': noise_threshold,
            'interpretation': 'Высокий noise_ratio = много нестабильных структур'
        }
    except Exception as e:
        return {'name': 'persistence_diagram', 'status': 'ERROR', 'error': str(e)}


def validate_bottleneck_stability(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Bottleneck Distance Stability - устойчивость к малым изменениям.

    Проверяем, как меняется топология при удалении рёбер.
    Маленькое bottleneck distance = устойчивая архитектура.
    """
    max_instability = 0.3
    if config and config.threshold is not None:
        max_instability = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        edges = list(subgraph.edges())
        n_edges = len(edges)

        if n_edges < 3:
            return {
                'name': 'bottleneck_stability',
                'status': 'SKIP',
                'reason': 'Недостаточно рёбер'
            }

        # Базовые топологические характеристики
        base_components = nx.number_connected_components(subgraph)
        base_cycles = n_edges - len(nodes) + base_components

        # Проверяем стабильность при удалении каждого ребра
        instabilities = []
        critical_edges = []

        for edge in edges:
            test_graph = subgraph.copy()
            test_graph.remove_edge(*edge)

            new_components = nx.number_connected_components(test_graph)
            new_cycles = test_graph.number_of_edges() - test_graph.number_of_nodes() + new_components

            # Изменение в топологии
            delta_h0 = abs(new_components - base_components)
            delta_h1 = abs(new_cycles - base_cycles)

            instability = delta_h0 + delta_h1
            instabilities.append(instability)

            if instability > 0:
                critical_edges.append({
                    'edge': edge,
                    'delta_h0': delta_h0,
                    'delta_h1': delta_h1
                })

        avg_instability = np.mean(instabilities) if instabilities else 0
        max_edge_instability = max(instabilities) if instabilities else 0

        # Нормализуем
        normalized_instability = avg_instability / max(1, base_cycles + base_components)

        if normalized_instability > max_instability:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'bottleneck_stability',
            'description': f'Топологическая устойчивость <= {max_instability}',
            'status': status,
            'avg_instability': round(avg_instability, 3),
            'normalized_instability': round(normalized_instability, 3),
            'max_edge_instability': max_edge_instability,
            'threshold': max_instability,
            'critical_edges_count': len(critical_edges),
            'critical_edges_sample': critical_edges[:5],
            'interpretation': 'Низкая instability = устойчивая архитектура'
        }
    except Exception as e:
        return {'name': 'bottleneck_stability', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# DISCRETE MORSE THEORY
# =============================================================================

def validate_morse_complexity(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Discrete Morse Theory - анализ критических клеток.

    Критические вершины и рёбра определяют топологическую сложность.
    Меньше критических клеток = проще структура.
    """
    max_critical_ratio = 0.3
    if config and config.threshold is not None:
        max_critical_ratio = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        e = subgraph.number_of_edges()

        if n < 2:
            return {
                'name': 'morse_complexity',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов'
            }

        # Дискретная функция Морса на основе степени
        morse_function = {node: subgraph.degree(node) for node in nodes}

        # Критические вершины (локальные минимумы/максимумы)
        critical_vertices = []

        for node in nodes:
            node_value = morse_function[node]
            neighbors = list(subgraph.neighbors(node))

            if not neighbors:
                # Изолированная вершина - критическая (минимум)
                critical_vertices.append({
                    'node': node,
                    'type': 'minimum',
                    'value': node_value
                })
                continue

            neighbor_values = [morse_function[nb] for nb in neighbors]

            if all(node_value <= v for v in neighbor_values):
                critical_vertices.append({
                    'node': node,
                    'type': 'minimum',
                    'value': node_value
                })
            elif all(node_value >= v for v in neighbor_values):
                critical_vertices.append({
                    'node': node,
                    'type': 'maximum',
                    'value': node_value
                })

        # Критические рёбра (седловые точки)
        critical_edges = []

        for u, v in subgraph.edges():
            u_val, v_val = morse_function[u], morse_function[v]

            # Ребро критическое, если это "перевал"
            u_neighbors = [morse_function[nb] for nb in subgraph.neighbors(u) if nb != v]
            v_neighbors = [morse_function[nb] for nb in subgraph.neighbors(v) if nb != u]

            if u_neighbors and v_neighbors:
                if (min(u_val, v_val) > max(min(u_neighbors), min(v_neighbors))):
                    critical_edges.append({
                        'edge': (u, v),
                        'type': 'saddle',
                        'values': (u_val, v_val)
                    })

        total_critical = len(critical_vertices) + len(critical_edges)
        total_cells = n + e
        critical_ratio = total_critical / total_cells if total_cells > 0 else 0

        # Morse неравенства: #critical >= Betti numbers
        # Проверяем, насколько близко к оптимуму
        beta_0 = nx.number_connected_components(subgraph)
        beta_1 = e - n + beta_0

        morse_excess = total_critical - (beta_0 + beta_1)

        if critical_ratio > max_critical_ratio:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'morse_complexity',
            'description': f'Морсова сложность <= {max_critical_ratio:.0%}',
            'status': status,
            'critical_vertices': len(critical_vertices),
            'critical_edges': len(critical_edges),
            'total_critical': total_critical,
            'critical_ratio': round(critical_ratio, 3),
            'threshold': max_critical_ratio,
            'morse_excess': morse_excess,
            'betti_sum': beta_0 + beta_1,
            'interpretation': 'Низкий excess = оптимальная структура'
        }
    except Exception as e:
        return {'name': 'morse_complexity', 'status': 'ERROR', 'error': str(e)}


def validate_gradient_flow(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Gradient Flow Analysis - анализ градиентного потока.

    В дискретной теории Морса градиентный поток показывает
    "направление зависимостей" от источников к стокам.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 2:
            return {
                'name': 'gradient_flow',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов'
            }

        # Источники (нет входящих) и стоки (нет исходящих)
        sources = [node for node in nodes if subgraph.in_degree(node) == 0]
        sinks = [node for node in nodes if subgraph.out_degree(node) == 0]

        # Анализ потоков от источников к стокам
        flow_paths = []
        for source in sources:
            for sink in sinks:
                try:
                    paths = list(nx.all_simple_paths(subgraph, source, sink))
                    for path in paths:
                        flow_paths.append({
                            'source': source,
                            'sink': sink,
                            'length': len(path) - 1,
                            'path': path if len(path) <= 5 else path[:3] + ['...'] + path[-2:]
                        })
                except nx.NetworkXNoPath:
                    pass

        # Статистика потоков
        if flow_paths:
            avg_flow_length = np.mean([fp['length'] for fp in flow_paths])
            max_flow_length = max(fp['length'] for fp in flow_paths)
        else:
            avg_flow_length = 0
            max_flow_length = 0

        # Анализ "бассейнов притяжения" - компоненты, ведущие к каждому стоку
        basins = {}
        for sink in sinks:
            basin = set()
            for node in nodes:
                try:
                    if nx.has_path(subgraph, node, sink):
                        basin.add(node)
                except:
                    pass
            basins[sink] = len(basin)

        # Баланс бассейнов
        if basins:
            basin_sizes = list(basins.values())
            basin_variance = np.var(basin_sizes)
            largest_basin = max(basins.items(), key=lambda x: x[1])
        else:
            basin_variance = 0
            largest_basin = (None, 0)

        return {
            'name': 'gradient_flow',
            'description': 'Анализ градиентного потока',
            'status': 'INFO',
            'sources': len(sources),
            'sinks': len(sinks),
            'flow_paths_count': len(flow_paths),
            'avg_flow_length': round(avg_flow_length, 2),
            'max_flow_length': max_flow_length,
            'basins_count': len(basins),
            'basin_variance': round(basin_variance, 2),
            'largest_basin': {'sink': largest_basin[0], 'size': largest_basin[1]},
            'interpretation': 'Высокая дисперсия бассейнов = неравномерная архитектура'
        }
    except Exception as e:
        return {'name': 'gradient_flow', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# SHEAF THEORY ON GRAPHS
# =============================================================================

def validate_sheaf_cohomology(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Sheaf Cohomology - когомологии пучков на графе.

    Cellular sheaf присваивает векторные пространства узлам и рёбрам.
    Sheaf Laplacian обобщает граф Лапласиан.

    H⁰ = глобальные секции (консистентные присваивания)
    H¹ = препятствия к расширению локальных секций
    """
    max_obstruction = 0.5
    if config and config.threshold is not None:
        max_obstruction = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        e = subgraph.number_of_edges()

        if n < 2:
            return {
                'name': 'sheaf_cohomology',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов'
            }

        # Простейший cellular sheaf: каждому узлу присваиваем его степень
        # Restriction maps: проверяем "совместимость" степеней

        node_list = list(nodes)
        edge_list = list(subgraph.edges())

        node_idx = {node: i for i, node in enumerate(node_list)}

        # Coboundary оператор δ⁰: C⁰ → C¹
        # Для каждого ребра (u,v) проверяем разницу значений
        if not edge_list:
            return {
                'name': 'sheaf_cohomology',
                'status': 'SKIP',
                'reason': 'Нет рёбер'
            }

        coboundary = np.zeros((e, n))
        for i, (u, v) in enumerate(edge_list):
            coboundary[i, node_idx[u]] = 1
            coboundary[i, node_idx[v]] = -1

        # Sheaf Laplacian L = δᵀδ
        sheaf_laplacian = coboundary.T @ coboundary

        # Собственные значения
        eigenvalues = np.linalg.eigvalsh(sheaf_laplacian)
        eigenvalues = np.sort(np.abs(eigenvalues))

        # dim H⁰ = количество нулевых собственных значений
        zero_threshold = 1e-10
        dim_h0 = np.sum(eigenvalues < zero_threshold)

        # Это равно числу компонент связности
        expected_h0 = nx.number_connected_components(subgraph)

        # Анализ спектра Sheaf Laplacian
        if len(eigenvalues) > 1:
            spectral_gap = eigenvalues[int(dim_h0)] if dim_h0 < len(eigenvalues) else 0
        else:
            spectral_gap = 0

        # Obstruction measure: как далеко от "гладкого" пучка
        # Используем норму Frobenius кобраницы
        degree_vector = np.array([subgraph.degree(node) for node in node_list])
        coboundary_image = coboundary @ degree_vector
        obstruction = np.linalg.norm(coboundary_image) / (np.linalg.norm(degree_vector) + 1e-10)

        if obstruction > max_obstruction:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'sheaf_cohomology',
            'description': f'Sheaf obstruction <= {max_obstruction}',
            'status': status,
            'dim_h0': int(dim_h0),
            'expected_h0': expected_h0,
            'spectral_gap': round(float(spectral_gap), 4),
            'obstruction': round(float(obstruction), 4),
            'threshold': max_obstruction,
            'interpretation': 'Низкое obstruction = консистентная структура'
        }
    except Exception as e:
        return {'name': 'sheaf_cohomology', 'status': 'ERROR', 'error': str(e)}


def validate_local_consistency(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Local-to-Global Consistency - проверка локальной консистентности.

    В теории пучков: можно ли склеить локальные секции в глобальную?
    Применительно к архитектуре: согласованы ли локальные паттерны?
    """
    min_consistency = 0.7
    if config and config.threshold is not None:
        min_consistency = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 3:
            return {
                'name': 'local_consistency',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов'
            }

        # Локальная характеристика: паттерн соседей
        local_patterns = {}
        for node in nodes:
            neighbors = list(subgraph.neighbors(node))
            # Паттерн = сортированные степени соседей
            pattern = tuple(sorted([subgraph.degree(nb) for nb in neighbors]))
            local_patterns[node] = pattern

        # Проверяем консистентность: похожие узлы должны иметь похожие паттерны
        consistency_scores = []

        for node in nodes:
            neighbors = list(subgraph.neighbors(node))
            if not neighbors:
                continue

            node_degree = subgraph.degree(node)

            # Соседи с похожей степенью
            similar_nodes = [nb for nb in neighbors
                           if abs(subgraph.degree(nb) - node_degree) <= 1]

            if similar_nodes:
                # Проверяем, похожи ли их паттерны
                node_pattern = set(local_patterns[node])
                similar_patterns = [set(local_patterns[sn]) for sn in similar_nodes]

                # Jaccard similarity
                similarities = []
                for sp in similar_patterns:
                    intersection = len(node_pattern & sp)
                    union = len(node_pattern | sp)
                    if union > 0:
                        similarities.append(intersection / union)

                if similarities:
                    consistency_scores.append(np.mean(similarities))

        if consistency_scores:
            avg_consistency = np.mean(consistency_scores)
            min_local_consistency = min(consistency_scores)
        else:
            avg_consistency = 1.0
            min_local_consistency = 1.0

        # Глобальная консистентность: энтропия распределения паттернов
        pattern_counts = defaultdict(int)
        for pattern in local_patterns.values():
            pattern_counts[pattern] += 1

        pattern_probs = np.array(list(pattern_counts.values())) / n
        pattern_entropy = -np.sum(pattern_probs * np.log2(pattern_probs + 1e-10))
        max_entropy = np.log2(n)
        normalized_entropy = pattern_entropy / max_entropy if max_entropy > 0 else 0

        if avg_consistency < min_consistency:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'local_consistency',
            'description': f'Локальная консистентность >= {min_consistency}',
            'status': status,
            'avg_consistency': round(avg_consistency, 3),
            'min_consistency': round(min_local_consistency, 3),
            'threshold': min_consistency,
            'unique_patterns': len(pattern_counts),
            'pattern_entropy': round(normalized_entropy, 3),
            'interpretation': 'Высокая консистентность = предсказуемая структура'
        }
    except Exception as e:
        return {'name': 'local_consistency', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# DISCRETE CURVATURE
# =============================================================================

def validate_forman_curvature(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Forman-Ricci Curvature - дискретная кривизна Формана.

    F(e) = 4 - deg(u) - deg(v) + |triangles containing e|

    Положительная кривизна = локально "сферическая" структура
    Отрицательная = "гиперболическая" (разветвлённая)
    """
    min_avg_curvature = -2.0
    if config and config.threshold is not None:
        min_avg_curvature = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        edges = list(subgraph.edges())
        if not edges:
            return {
                'name': 'forman_curvature',
                'status': 'SKIP',
                'reason': 'Нет рёбер'
            }

        # Считаем треугольники для каждого ребра
        triangles_count = {}
        for edge in edges:
            u, v = edge
            common_neighbors = set(subgraph.neighbors(u)) & set(subgraph.neighbors(v))
            triangles_count[edge] = len(common_neighbors)

        # Forman curvature для каждого ребра
        curvatures = []
        edge_curvatures = []

        for edge in edges:
            u, v = edge
            deg_u = subgraph.degree(u)
            deg_v = subgraph.degree(v)
            triangles = triangles_count[edge]

            # Forman-Ricci formula
            forman = 4 - deg_u - deg_v + 3 * triangles

            curvatures.append(forman)
            edge_curvatures.append({
                'edge': edge,
                'curvature': forman,
                'triangles': triangles
            })

        avg_curvature = np.mean(curvatures)
        min_curvature = min(curvatures)
        max_curvature = max(curvatures)

        # Распределение по типам
        positive = sum(1 for c in curvatures if c > 0)
        negative = sum(1 for c in curvatures if c < 0)
        zero = sum(1 for c in curvatures if c == 0)

        # Сортируем по кривизне
        edge_curvatures.sort(key=lambda x: x['curvature'])

        if avg_curvature < min_avg_curvature:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'forman_curvature',
            'description': f'Средняя кривизна Формана >= {min_avg_curvature}',
            'status': status,
            'avg_curvature': round(avg_curvature, 3),
            'min_curvature': min_curvature,
            'max_curvature': max_curvature,
            'threshold': min_avg_curvature,
            'distribution': {
                'positive': positive,
                'zero': zero,
                'negative': negative
            },
            'most_negative': edge_curvatures[:3],
            'most_positive': edge_curvatures[-3:][::-1],
            'interpretation': 'Отрицательная = разветвлённая структура'
        }
    except Exception as e:
        return {'name': 'forman_curvature', 'status': 'ERROR', 'error': str(e)}


def validate_ricci_flow(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Discrete Ricci Flow - симуляция потока Риччи.

    Ricci flow "сглаживает" граф, приближая кривизну к константе.
    Анализируем, насколько граф далёк от "сглаженного" состояния.
    """
    max_iterations = 10
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        edges = list(subgraph.edges())
        if len(edges) < 2:
            return {
                'name': 'ricci_flow',
                'status': 'SKIP',
                'reason': 'Недостаточно рёбер'
            }

        # Начальные веса рёбер = 1
        edge_weights = {edge: 1.0 for edge in edges}

        # Симулируем Ricci flow
        curvature_history = []

        for iteration in range(max_iterations):
            # Вычисляем кривизну для каждого ребра
            curvatures = {}
            for edge in edges:
                u, v = edge
                deg_u = subgraph.degree(u)
                deg_v = subgraph.degree(v)
                common = len(set(subgraph.neighbors(u)) & set(subgraph.neighbors(v)))

                # Взвешенная Forman кривизна
                w = edge_weights[edge]
                curvatures[edge] = w * (4 - deg_u - deg_v + 3 * common)

            avg_curv = np.mean(list(curvatures.values()))
            curvature_history.append(avg_curv)

            # Обновляем веса (дискретный Ricci flow)
            # w' = w * (1 - ε * Ric)
            epsilon = 0.1
            for edge in edges:
                edge_weights[edge] *= (1 - epsilon * curvatures[edge])
                edge_weights[edge] = max(0.01, min(10, edge_weights[edge]))  # Ограничиваем

        # Анализ сходимости
        if len(curvature_history) >= 2:
            initial_variance = np.var([curvatures[e] for e in edges])
            convergence_rate = abs(curvature_history[-1] - curvature_history[0]) / max(1, abs(curvature_history[0]))
        else:
            initial_variance = 0
            convergence_rate = 0

        # Финальное распределение весов
        final_weights = list(edge_weights.values())
        weight_variance = np.var(final_weights)

        return {
            'name': 'ricci_flow',
            'description': 'Анализ дискретного потока Риччи',
            'status': 'INFO',
            'iterations': max_iterations,
            'initial_avg_curvature': round(curvature_history[0], 3) if curvature_history else 0,
            'final_avg_curvature': round(curvature_history[-1], 3) if curvature_history else 0,
            'convergence_rate': round(convergence_rate, 3),
            'weight_variance': round(weight_variance, 3),
            'interpretation': 'Низкая дисперсия весов = близко к равновесию'
        }
    except Exception as e:
        return {'name': 'ricci_flow', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# HOMOTOPY THEORY
# =============================================================================

def validate_fundamental_group(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Fundamental Group π₁ - фундаментальная группа графа.

    π₁(G) = свободная группа с rank = β₁ = E - V + 1 (для связного)

    Rank π₁ = количество независимых циклов = cyclomatic complexity.
    """
    max_rank = 10
    if config and config.threshold is not None:
        max_rank = int(config.threshold)
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        e = subgraph.number_of_edges()
        c = nx.number_connected_components(subgraph)

        if n < 2:
            return {
                'name': 'fundamental_group',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов'
            }

        # rank π₁ = β₁ = E - V + C
        rank_pi1 = e - n + c

        # Находим базис циклов (генераторы π₁)
        try:
            cycle_basis = nx.cycle_basis(subgraph)
            generators = [{'cycle': cyc[:5] + ['...'] if len(cyc) > 5 else cyc,
                          'length': len(cyc)}
                         for cyc in cycle_basis[:10]]
        except:
            generators = []

        # Анализ длин циклов
        if cycle_basis:
            cycle_lengths = [len(c) for c in cycle_basis]
            avg_cycle_length = np.mean(cycle_lengths)
            min_cycle_length = min(cycle_lengths)
            max_cycle_length = max(cycle_lengths)
        else:
            avg_cycle_length = 0
            min_cycle_length = 0
            max_cycle_length = 0

        # Контрактируемость: π₁ = 0 ⟺ граф - дерево
        is_contractible = (rank_pi1 == 0)

        if rank_pi1 > max_rank:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'fundamental_group',
            'description': f'rank(π₁) <= {max_rank}',
            'status': status,
            'rank_pi1': rank_pi1,
            'threshold': max_rank,
            'is_contractible': is_contractible,
            'generators_count': len(cycle_basis) if cycle_basis else 0,
            'generators_sample': generators[:5],
            'avg_cycle_length': round(avg_cycle_length, 2),
            'cycle_length_range': [min_cycle_length, max_cycle_length],
            'interpretation': 'rank π₁ = cyclomatic complexity'
        }
    except Exception as e:
        return {'name': 'fundamental_group', 'status': 'ERROR', 'error': str(e)}


def validate_covering_space(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Covering Space Analysis - анализ накрывающих пространств.

    Для графа: универсальное накрытие = развёртка всех циклов.
    Число листов накрытия связано с π₁.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        e = subgraph.number_of_edges()

        if n < 2:
            return {
                'name': 'covering_space',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов'
            }

        # Универсальное накрытие графа - это дерево
        # Размер универсального накрытия = |π₁| * n (для конечных групп)
        # Для свободной группы rank r: бесконечно, но можно оценить

        c = nx.number_connected_components(subgraph)
        rank_pi1 = e - n + c

        # Строим spanning tree
        spanning_trees = []
        for component in nx.connected_components(subgraph):
            comp_subgraph = subgraph.subgraph(component)
            tree = nx.minimum_spanning_tree(comp_subgraph)
            spanning_trees.append(tree)

        # Рёбра вне spanning tree = "генераторы накрытия"
        tree_edges = set()
        for tree in spanning_trees:
            tree_edges.update(tree.edges())

        non_tree_edges = []
        for edge in subgraph.edges():
            if edge not in tree_edges and (edge[1], edge[0]) not in tree_edges:
                non_tree_edges.append(edge)

        # Локальная структура: степень накрытия в каждой точке
        # Для графа это связано с локальными циклами
        local_covering_degree = {}
        for node in nodes:
            # Количество циклов через эту вершину
            cycles_through_node = 0
            try:
                for cycle in nx.cycle_basis(subgraph):
                    if node in cycle:
                        cycles_through_node += 1
            except:
                pass
            local_covering_degree[node] = cycles_through_node

        avg_covering_degree = np.mean(list(local_covering_degree.values())) if local_covering_degree else 0
        max_covering_degree = max(local_covering_degree.values()) if local_covering_degree else 0

        # Узлы с высоким covering degree
        high_degree_nodes = [node for node, deg in local_covering_degree.items()
                           if deg > avg_covering_degree + 1]

        return {
            'name': 'covering_space',
            'description': 'Анализ накрывающих пространств',
            'status': 'INFO',
            'rank_pi1': rank_pi1,
            'non_tree_edges': len(non_tree_edges),
            'avg_local_covering': round(avg_covering_degree, 2),
            'max_local_covering': max_covering_degree,
            'high_covering_nodes': high_degree_nodes[:5],
            'interpretation': 'Высокое накрытие = узел в центре многих циклов'
        }
    except Exception as e:
        return {'name': 'covering_space', 'status': 'ERROR', 'error': str(e)}


def validate_homotopy_equivalence(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Homotopy Equivalence Check - проверка гомотопической эквивалентности.

    Граф гомотопически эквивалентен букету окружностей (wedge of circles).
    Проверяем, насколько структура близка к "минимальной".
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        e = subgraph.number_of_edges()
        c = nx.number_connected_components(subgraph)

        if n < 2:
            return {
                'name': 'homotopy_equivalence',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов'
            }

        # β₁ = rank свободной группы
        beta_1 = e - n + c

        # Минимальный граф с тем же π₁: букет из β₁ окружностей
        # Это 1 вершина и β₁ петель = 1 вершина, β₁ рёбер
        minimal_vertices = c  # По одной вершине на компоненту
        minimal_edges = beta_1 + (c - 1)  # Циклы + соединения

        # "Избыточность" текущего графа
        vertex_excess = n - minimal_vertices
        edge_excess = e - minimal_edges

        # Коэффициент минимальности
        if n > 0:
            minimality = minimal_vertices / n
        else:
            minimality = 1.0

        # Ретракт: можно ли стянуть граф?
        # Листья (степень 1) можно стянуть
        leaves = [node for node in nodes if subgraph.degree(node) == 1]
        retractable_vertices = len(leaves)

        # Итеративная ретракция
        temp_graph = subgraph.copy()
        retraction_steps = 0
        while True:
            leaves = [node for node in temp_graph.nodes() if temp_graph.degree(node) == 1]
            if not leaves:
                break
            for leaf in leaves:
                temp_graph.remove_node(leaf)
            retraction_steps += 1

        core_size = temp_graph.number_of_nodes()

        return {
            'name': 'homotopy_equivalence',
            'description': 'Анализ гомотопической эквивалентности',
            'status': 'INFO',
            'beta_1': beta_1,
            'current_vertices': n,
            'minimal_vertices': minimal_vertices,
            'vertex_excess': vertex_excess,
            'minimality_ratio': round(minimality, 3),
            'retraction_steps': retraction_steps,
            'core_size': core_size,
            'interpretation': 'Низкий excess = близко к минимальной структуре'
        }
    except Exception as e:
        return {'name': 'homotopy_equivalence', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# FILTRATION & SPECTRAL SEQUENCES
# =============================================================================

def validate_weight_filtration(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Weight Filtration - фильтрация по весам.

    Строим последовательность подграфов по порогу "важности".
    Анализируем, как топология меняется на каждом уровне.
    """
    num_levels = 5
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 3:
            return {
                'name': 'weight_filtration',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов'
            }

        # Вес узла = степень
        node_weights = {node: subgraph.degree(node) for node in nodes}
        max_weight = max(node_weights.values()) if node_weights else 1

        # Строим фильтрацию
        filtration_levels = []

        for level in range(num_levels):
            threshold = (level + 1) * max_weight / num_levels

            # Узлы с весом >= threshold
            filtered_nodes = [node for node, w in node_weights.items() if w >= threshold]

            if len(filtered_nodes) < 2:
                filtration_levels.append({
                    'level': level,
                    'threshold': round(threshold, 2),
                    'nodes': len(filtered_nodes),
                    'edges': 0,
                    'components': len(filtered_nodes),
                    'beta_1': 0
                })
                continue

            filtered_subgraph = subgraph.subgraph(filtered_nodes)
            e = filtered_subgraph.number_of_edges()
            c = nx.number_connected_components(filtered_subgraph)
            beta_1 = e - len(filtered_nodes) + c

            filtration_levels.append({
                'level': level,
                'threshold': round(threshold, 2),
                'nodes': len(filtered_nodes),
                'edges': e,
                'components': c,
                'beta_1': beta_1
            })

        # Анализ стабильности топологии
        beta_1_sequence = [fl['beta_1'] for fl in filtration_levels]
        component_sequence = [fl['components'] for fl in filtration_levels]

        # Скачки в β₁
        beta_1_jumps = sum(1 for i in range(1, len(beta_1_sequence))
                          if beta_1_sequence[i] != beta_1_sequence[i-1])

        # Монотонность компонент
        component_monotonic = all(component_sequence[i] <= component_sequence[i-1]
                                  for i in range(1, len(component_sequence)))

        return {
            'name': 'weight_filtration',
            'description': 'Весовая фильтрация графа',
            'status': 'INFO',
            'levels': num_levels,
            'filtration': filtration_levels,
            'beta_1_jumps': beta_1_jumps,
            'component_monotonic': component_monotonic,
            'interpretation': 'Много скачков = нестабильная топология по весам'
        }
    except Exception as e:
        return {'name': 'weight_filtration', 'status': 'ERROR', 'error': str(e)}


def validate_hodge_decomposition(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Hodge Decomposition - разложение Ходжа.

    Для графа: любой поток = градиентный + гармонический + циркуляция.

    Гармонические формы соответствуют гомологиям.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        e = subgraph.number_of_edges()

        if n < 2 or e < 1:
            return {
                'name': 'hodge_decomposition',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов или рёбер'
            }

        node_list = list(nodes)
        edge_list = list(subgraph.edges())

        node_idx = {node: i for i, node in enumerate(node_list)}

        # Incidence matrix B (nodes x edges)
        B = np.zeros((n, e))
        for j, (u, v) in enumerate(edge_list):
            B[node_idx[u], j] = 1
            B[node_idx[v], j] = -1

        # Hodge Laplacians
        # L₀ = B @ B.T (node Laplacian)
        # L₁ = B.T @ B (edge Laplacian)

        L0 = B @ B.T
        L1 = B.T @ B

        # Собственные значения
        eig_L0 = np.linalg.eigvalsh(L0)
        eig_L1 = np.linalg.eigvalsh(L1)

        # Гармонические формы: ker(L)
        zero_threshold = 1e-10

        # dim H⁰ = nullity(L₀) = # компонент
        harmonic_0 = np.sum(np.abs(eig_L0) < zero_threshold)

        # dim H¹ = nullity(L₁) = β₁
        harmonic_1 = np.sum(np.abs(eig_L1) < zero_threshold)

        # Спектральный зазор
        nonzero_L0 = eig_L0[np.abs(eig_L0) >= zero_threshold]
        spectral_gap_0 = float(np.min(nonzero_L0)) if len(nonzero_L0) > 0 else 0

        nonzero_L1 = eig_L1[np.abs(eig_L1) >= zero_threshold]
        spectral_gap_1 = float(np.min(nonzero_L1)) if len(nonzero_L1) > 0 else 0

        # Hodge числа
        c = nx.number_connected_components(subgraph)
        expected_h0 = c
        expected_h1 = e - n + c

        return {
            'name': 'hodge_decomposition',
            'description': 'Разложение Ходжа графа',
            'status': 'INFO',
            'harmonic_0': int(harmonic_0),
            'harmonic_1': int(harmonic_1),
            'expected_h0': expected_h0,
            'expected_h1': expected_h1,
            'spectral_gap_L0': round(spectral_gap_0, 4),
            'spectral_gap_L1': round(spectral_gap_1, 4),
            'interpretation': 'Гармонические формы = топологические инварианты'
        }
    except Exception as e:
        return {'name': 'hodge_decomposition', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# SIMPLICIAL COMPLEXES (EXTENDED)
# =============================================================================

def validate_clique_complex(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Clique Complex Analysis - анализ кликового комплекса.

    Кликовый комплекс: каждая клика размера k+1 = k-симплекс.
    Полный топологический инвариант графа.
    """
    max_dimension = 4
    if config and config.threshold is not None:
        max_dimension = int(config.threshold)
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 2:
            return {
                'name': 'clique_complex',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов'
            }

        # Находим все клики
        cliques = list(nx.find_cliques(subgraph))

        # Распределение по размерам (= размерностям + 1)
        size_distribution = defaultdict(int)
        max_found_dim = 0

        for clique in cliques:
            dim = len(clique) - 1
            size_distribution[dim] += 1
            max_found_dim = max(max_found_dim, dim)

        # f-вектор (face vector): f_k = количество k-симплексов
        f_vector = []
        for k in range(max_found_dim + 1):
            f_vector.append(size_distribution.get(k, 0))

        # Euler characteristic через f-вектор
        # χ = Σ (-1)^k * f_k
        euler_char = sum((-1)**k * f_k for k, f_k in enumerate(f_vector))

        # Большие клики (нарушения)
        violations = []
        if max_found_dim > max_dimension:
            for clique in cliques:
                if len(clique) - 1 > max_dimension:
                    violations.append({
                        'dimension': len(clique) - 1,
                        'vertices': list(clique)[:8]
                    })

        if violations:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'clique_complex',
            'description': f'Размерность кликового комплекса <= {max_dimension}',
            'status': status,
            'max_dimension': max_found_dim,
            'threshold': max_dimension,
            'f_vector': f_vector,
            'euler_characteristic': euler_char,
            'total_cliques': len(cliques),
            'violations': violations[:5],
            'interpretation': 'Высокая размерность = тесно связанные группы'
        }
    except Exception as e:
        return {'name': 'clique_complex', 'status': 'ERROR', 'error': str(e)}


def validate_nerve_complex(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Nerve Complex - нерв покрытия.

    Для графа: покрытие = окрестности вершин.
    Nerve theorem: нерв гомотопически эквивалентен объединению.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 3:
            return {
                'name': 'nerve_complex',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов'
            }

        # Покрытие: замкнутые окрестности вершин
        neighborhoods = {}
        for node in nodes:
            # N[v] = {v} ∪ neighbors(v)
            neighborhoods[node] = {node} | set(subgraph.neighbors(node))

        # Строим нерв: симплекс для каждого непустого пересечения
        # 0-симплексы: вершины (всегда)
        # 1-симплексы: пары с непустым пересечением
        # k-симплексы: (k+1)-наборы с общим пересечением

        nerve_simplices = {0: list(nodes)}

        # 1-симплексы
        one_simplices = []
        for u, v in combinations(nodes, 2):
            if neighborhoods[u] & neighborhoods[v]:
                one_simplices.append((u, v))
        nerve_simplices[1] = one_simplices

        # 2-симплексы
        two_simplices = []
        for u, v, w in combinations(nodes, 3):
            if neighborhoods[u] & neighborhoods[v] & neighborhoods[w]:
                two_simplices.append((u, v, w))
        nerve_simplices[2] = two_simplices

        # Euler characteristic нерва
        euler_nerve = len(nerve_simplices[0]) - len(nerve_simplices[1]) + len(nerve_simplices[2])

        # Сравниваем с Euler графа
        c = nx.number_connected_components(subgraph)
        e = subgraph.number_of_edges()
        euler_graph = n - e + c

        # Homotopy equivalence check
        nerve_matches_graph = (euler_nerve == euler_graph)

        # Средний размер пересечения
        intersection_sizes = []
        for u, v in one_simplices:
            intersection_sizes.append(len(neighborhoods[u] & neighborhoods[v]))

        avg_intersection = np.mean(intersection_sizes) if intersection_sizes else 0

        return {
            'name': 'nerve_complex',
            'description': 'Нерв покрытия окрестностями',
            'status': 'INFO',
            'nerve_0_simplices': len(nerve_simplices[0]),
            'nerve_1_simplices': len(nerve_simplices[1]),
            'nerve_2_simplices': len(nerve_simplices[2]),
            'euler_nerve': euler_nerve,
            'euler_graph': euler_graph,
            'homotopy_equivalent': nerve_matches_graph,
            'avg_intersection_size': round(avg_intersection, 2),
            'interpretation': 'Нерв сохраняет топологию покрытия'
        }
    except Exception as e:
        return {'name': 'nerve_complex', 'status': 'ERROR', 'error': str(e)}


def validate_vietoris_rips(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Vietoris-Rips Complex - комплекс Виеториса-Рипса.

    VR_ε: симплекс на {v₀,...,vₖ} если все попарные расстояния <= ε.
    Строим фильтрацию по ε.
    """
    num_scales = 5
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 3:
            return {
                'name': 'vietoris_rips',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов'
            }

        node_list = list(nodes)

        # Вычисляем матрицу расстояний (кратчайшие пути)
        try:
            distances = dict(nx.all_pairs_shortest_path_length(subgraph))
        except:
            return {
                'name': 'vietoris_rips',
                'status': 'SKIP',
                'reason': 'Граф не связен'
            }

        # Максимальное расстояние
        max_dist = 0
        for u in distances:
            for v, d in distances[u].items():
                max_dist = max(max_dist, d)

        if max_dist == 0:
            return {
                'name': 'vietoris_rips',
                'status': 'SKIP',
                'reason': 'Все расстояния нулевые'
            }

        # Строим VR комплексы для разных ε
        vr_filtration = []

        for scale_idx in range(num_scales):
            epsilon = (scale_idx + 1) * max_dist / num_scales

            # Рёбра в VR_ε
            vr_edges = []
            for i, u in enumerate(node_list):
                for j, v in enumerate(node_list[i+1:], i+1):
                    if u in distances and v in distances[u]:
                        if distances[u][v] <= epsilon:
                            vr_edges.append((u, v))

            # 2-симплексы (треугольники)
            vr_triangles = 0
            for u, v, w in combinations(node_list, 3):
                try:
                    if (distances[u][v] <= epsilon and
                        distances[v][w] <= epsilon and
                        distances[u][w] <= epsilon):
                        vr_triangles += 1
                except KeyError:
                    pass

            vr_filtration.append({
                'epsilon': round(epsilon, 2),
                'edges': len(vr_edges),
                'triangles': vr_triangles
            })

        # Анализ роста
        edge_growth = [vr['edges'] for vr in vr_filtration]
        triangle_growth = [vr['triangles'] for vr in vr_filtration]

        return {
            'name': 'vietoris_rips',
            'description': 'Фильтрация Виеториса-Рипса',
            'status': 'INFO',
            'max_distance': max_dist,
            'scales': num_scales,
            'filtration': vr_filtration,
            'edge_growth_rate': round((edge_growth[-1] - edge_growth[0]) / num_scales, 2) if edge_growth else 0,
            'interpretation': 'Быстрый рост = плотная структура на малых масштабах'
        }
    except Exception as e:
        return {'name': 'vietoris_rips', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# PERSISTENCE LANDSCAPES
# =============================================================================

def validate_persistence_landscape(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Persistence Landscape - функциональное представление персистентности.

    Landscape λₖ(t) - k-й "слой" ландшафта.
    Преобразует диаграмму персистентности в функцию для статистического анализа.
    """
    num_landscapes = 3
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        e = subgraph.number_of_edges()

        if n < 3:
            return {
                'name': 'persistence_landscape',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов'
            }

        # Строим persistence diagram на основе degree filtration
        birth_death_pairs = []

        # Union-Find для H0
        parent = {node: node for node in nodes}

        def find(x):
            if parent[x] != x:
                parent[x] = find(parent[x])
            return parent[x]

        def union(x, y):
            px, py = find(x), find(y)
            if px != py:
                parent[py] = px
                return True
            return False

        # Сортируем рёбра по весу (минимальная степень)
        edges_weighted = []
        for u, v in subgraph.edges():
            weight = min(subgraph.degree(u), subgraph.degree(v))
            edges_weighted.append((weight, u, v))
        edges_weighted.sort()

        # Добавляем рёбра и отслеживаем слияния (H0)
        for idx, (weight, u, v) in enumerate(edges_weighted):
            birth = 0
            death = idx / len(edges_weighted) if edges_weighted else 1
            if union(u, v):
                birth_death_pairs.append((birth, death))

        # Добавляем H1 (циклы)
        c = nx.number_connected_components(subgraph)
        beta_1 = e - n + c
        for i in range(beta_1):
            # Циклы живут "вечно" - нормализуем
            birth_death_pairs.append((0.5 + i * 0.1, 1.0))

        if not birth_death_pairs:
            return {
                'name': 'persistence_landscape',
                'status': 'SKIP',
                'reason': 'Нет persistence pairs'
            }

        # Строим ландшафт
        # λₖ(t) = k-й максимум из min(t-b, d-t) для всех пар (b,d)
        resolution = 20
        t_values = np.linspace(0, 1, resolution)

        landscapes = []
        for k in range(num_landscapes):
            landscape_k = []
            for t in t_values:
                # Вычисляем "tent functions" для всех пар
                tent_values = []
                for b, d in birth_death_pairs:
                    if b <= t <= d:
                        tent = min(t - b, d - t)
                        tent_values.append(tent)

                # k-й максимум
                tent_values.sort(reverse=True)
                if len(tent_values) > k:
                    landscape_k.append(tent_values[k])
                else:
                    landscape_k.append(0)

            landscapes.append(landscape_k)

        # Статистики ландшафтов
        landscape_stats = []
        for k, landscape in enumerate(landscapes):
            landscape_stats.append({
                'k': k + 1,
                'max': round(max(landscape), 4),
                'mean': round(np.mean(landscape), 4),
                'integral': round(np.trapezoid(landscape, t_values), 4)
            })

        # Норма ландшафта (L² норма первого)
        l2_norm = np.sqrt(np.trapezoid(np.array(landscapes[0])**2, t_values))

        return {
            'name': 'persistence_landscape',
            'description': 'Персистентный ландшафт',
            'status': 'INFO',
            'num_pairs': len(birth_death_pairs),
            'num_landscapes': num_landscapes,
            'landscape_stats': landscape_stats,
            'l2_norm': round(float(l2_norm), 4),
            'interpretation': 'Большая норма = устойчивые топологические признаки'
        }
    except Exception as e:
        return {'name': 'persistence_landscape', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# ČECH COMPLEX
# =============================================================================

def validate_cech_complex(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Čech Complex - комплекс Чеха.

    Čech_ε: симплекс на {v₀,...,vₖ} если ∩ B_ε(vᵢ) ≠ ∅
    (шары радиуса ε имеют общую точку).

    Для графа: используем расстояния кратчайших путей.
    """
    max_dimension = 3
    if config and config.threshold is not None:
        max_dimension = int(config.threshold)
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 3:
            return {
                'name': 'cech_complex',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов'
            }

        node_list = list(nodes)

        # Матрица расстояний
        try:
            distances = dict(nx.all_pairs_shortest_path_length(subgraph))
        except:
            return {
                'name': 'cech_complex',
                'status': 'SKIP',
                'reason': 'Граф не связен'
            }

        # Находим диаметр для нормализации
        max_dist = max(distances[u][v] for u in distances for v in distances[u])

        # Строим Čech комплекс для ε = diameter/2
        epsilon = max_dist / 2

        # Для Čech: k-симплекс существует если есть точка в пересечении всех шаров
        # Упрощение: проверяем что все попарные расстояния <= 2ε
        # (достаточное условие для существования общей точки)

        cech_simplices = {0: list(node_list)}

        # Helper to safely get distance
        def get_dist(u, v):
            try:
                return distances[u].get(v, float('inf'))
            except:
                return float('inf')

        # 1-симплексы
        one_simplices = []
        for i, u in enumerate(node_list):
            for j, v in enumerate(node_list[i+1:], i+1):
                if get_dist(u, v) <= 2 * epsilon:
                    one_simplices.append((u, v))
        cech_simplices[1] = one_simplices

        # 2-симплексы
        two_simplices = []
        for u, v, w in combinations(node_list, 3):
            # Čech условие: существует точка в пересечении шаров
            # Проверяем: max расстояние <= 2ε
            dists = [get_dist(u, v), get_dist(v, w), get_dist(u, w)]
            if max(dists) <= 2 * epsilon:
                two_simplices.append((u, v, w))
        cech_simplices[2] = two_simplices

        # 3-симплексы
        three_simplices = []
        if max_dimension >= 3:
            for combo in combinations(node_list, 4):
                all_dists = [get_dist(combo[i], combo[j])
                            for i in range(4) for j in range(i+1, 4)]
                if max(all_dists) <= 2 * epsilon:
                    three_simplices.append(combo)
        cech_simplices[3] = three_simplices

        # Максимальная размерность
        max_found_dim = 0
        for dim in range(4):
            if cech_simplices.get(dim, []):
                max_found_dim = dim

        # Euler characteristic
        euler = sum((-1)**k * len(cech_simplices.get(k, [])) for k in range(4))

        violations = []
        if max_found_dim > max_dimension:
            violations.append({
                'dimension': max_found_dim,
                'count': len(cech_simplices.get(max_found_dim, []))
            })

        if violations:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'cech_complex',
            'description': f'Čech комплекс размерности <= {max_dimension}',
            'status': status,
            'epsilon': round(epsilon, 2),
            'max_dimension': max_found_dim,
            'threshold': max_dimension,
            'simplices': {
                0: len(cech_simplices[0]),
                1: len(cech_simplices[1]),
                2: len(cech_simplices[2]),
                3: len(cech_simplices[3])
            },
            'euler_characteristic': euler,
            'interpretation': 'Čech комплекс точнее аппроксимирует топологию'
        }
    except Exception as e:
        return {'name': 'cech_complex', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# ALPHA COMPLEX
# =============================================================================

def validate_alpha_complex(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Alpha Complex - альфа-комплекс (подкомплекс Делоне).

    Alpha complex ⊆ Delaunay triangulation.
    Более эффективный чем Čech/VR для евклидовых данных.

    Для графа: аппроксимируем через degree-based embedding.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 4:
            return {
                'name': 'alpha_complex',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов'
            }

        node_list = list(nodes)

        # Embedding: используем spectral coordinates
        try:
            laplacian = nx.laplacian_matrix(subgraph).toarray()
            eigenvalues, eigenvectors = np.linalg.eigh(laplacian)

            # Берём 2-3 наименьших ненулевых собственных вектора
            nonzero_idx = np.where(eigenvalues > 1e-10)[0]
            if len(nonzero_idx) < 2:
                return {
                    'name': 'alpha_complex',
                    'status': 'SKIP',
                    'reason': 'Недостаточно ненулевых собственных значений'
                }

            coords = eigenvectors[:, nonzero_idx[:2]]
        except:
            # Fallback: degree-based embedding
            degrees = np.array([subgraph.degree(node) for node in node_list])
            clustering = np.array([nx.clustering(subgraph, node) for node in node_list])
            coords = np.column_stack([degrees, clustering])

        # Нормализуем координаты
        coords = (coords - coords.mean(axis=0)) / (coords.std(axis=0) + 1e-10)

        # Вычисляем попарные расстояния в embedding
        from scipy.spatial.distance import pdist, squareform
        dist_matrix = squareform(pdist(coords))

        # Строим alpha complex через фильтрацию по расстояниям
        # Alpha value = circumradius of simplex

        alpha_simplices = {0: list(range(n))}

        # 1-симплексы: рёбра с их длинами
        edges_with_alpha = []
        for i in range(n):
            for j in range(i+1, n):
                alpha = dist_matrix[i, j] / 2  # Радиус описанной окружности для ребра
                edges_with_alpha.append((alpha, i, j))

        edges_with_alpha.sort()

        # Берём рёбра до медианного alpha
        median_alpha = np.median([e[0] for e in edges_with_alpha])
        alpha_simplices[1] = [(node_list[i], node_list[j])
                              for a, i, j in edges_with_alpha if a <= median_alpha]

        # 2-симплексы: треугольники
        triangles_with_alpha = []
        for i, j, k in combinations(range(n), 3):
            # Circumradius для треугольника
            a = dist_matrix[i, j]
            b = dist_matrix[j, k]
            c = dist_matrix[i, k]
            s = (a + b + c) / 2
            area_sq = s * (s-a) * (s-b) * (s-c)
            if area_sq > 0:
                area = np.sqrt(area_sq)
                circumradius = (a * b * c) / (4 * area) if area > 0 else float('inf')
            else:
                circumradius = max(a, b, c) / 2

            triangles_with_alpha.append((circumradius, i, j, k))

        triangles_with_alpha.sort()

        # Берём треугольники до 1.5 * median_alpha
        alpha_simplices[2] = [(node_list[i], node_list[j], node_list[k])
                              for a, i, j, k in triangles_with_alpha
                              if a <= 1.5 * median_alpha][:50]  # Limit

        # Статистика
        alpha_values = [e[0] for e in edges_with_alpha]

        return {
            'name': 'alpha_complex',
            'description': 'Alpha комплекс (spectral embedding)',
            'status': 'INFO',
            'embedding_dim': coords.shape[1],
            'median_alpha': round(median_alpha, 4),
            'alpha_range': [round(min(alpha_values), 4), round(max(alpha_values), 4)],
            'simplices': {
                0: len(alpha_simplices[0]),
                1: len(alpha_simplices[1]),
                2: len(alpha_simplices[2])
            },
            'interpretation': 'Alpha complex эффективен для embedded данных'
        }
    except Exception as e:
        return {'name': 'alpha_complex', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# MORSE-SMALE COMPLEX
# =============================================================================

def validate_morse_smale(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Morse-Smale Complex - разбиение на ячейки по градиентному потоку.

    Каждая ячейка = компоненты связности stable/unstable manifolds.
    Показывает "бассейны" и "водоразделы" архитектуры.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 4:
            return {
                'name': 'morse_smale',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов'
            }

        # Функция Морса: степень узла
        morse_func = {node: subgraph.degree(node) for node in nodes}

        # Критические точки
        minima = []
        maxima = []
        saddles = []

        for node in nodes:
            neighbors = list(subgraph.neighbors(node))
            if not neighbors:
                minima.append(node)
                continue

            node_val = morse_func[node]
            neighbor_vals = [morse_func[nb] for nb in neighbors]

            lower_neighbors = sum(1 for v in neighbor_vals if v < node_val)
            higher_neighbors = sum(1 for v in neighbor_vals if v > node_val)

            if lower_neighbors == 0:
                minima.append(node)
            elif higher_neighbors == 0:
                maxima.append(node)
            elif lower_neighbors > 0 and higher_neighbors > 0:
                saddles.append(node)

        # Stable manifolds (descending from maxima)
        stable_cells = {}
        for maximum in maxima:
            cell = set()
            queue = [maximum]
            visited = {maximum}

            while queue:
                current = queue.pop(0)
                cell.add(current)
                for neighbor in subgraph.neighbors(current):
                    if neighbor not in visited and morse_func[neighbor] < morse_func[current]:
                        visited.add(neighbor)
                        queue.append(neighbor)

            stable_cells[maximum] = cell

        # Unstable manifolds (ascending from minima)
        unstable_cells = {}
        for minimum in minima:
            cell = set()
            queue = [minimum]
            visited = {minimum}

            while queue:
                current = queue.pop(0)
                cell.add(current)
                for neighbor in subgraph.neighbors(current):
                    if neighbor not in visited and morse_func[neighbor] > morse_func[current]:
                        visited.add(neighbor)
                        queue.append(neighbor)

            unstable_cells[minimum] = cell

        # Morse-Smale cells = intersections
        ms_cells = []
        for max_node, stable in stable_cells.items():
            for min_node, unstable in unstable_cells.items():
                intersection = stable & unstable
                if intersection:
                    ms_cells.append({
                        'maximum': max_node,
                        'minimum': min_node,
                        'size': len(intersection)
                    })

        # Статистика
        cell_sizes = [c['size'] for c in ms_cells]

        return {
            'name': 'morse_smale',
            'description': 'Morse-Smale комплекс',
            'status': 'INFO',
            'critical_points': {
                'minima': len(minima),
                'maxima': len(maxima),
                'saddles': len(saddles)
            },
            'morse_smale_cells': len(ms_cells),
            'avg_cell_size': round(np.mean(cell_sizes), 2) if cell_sizes else 0,
            'max_cell_size': max(cell_sizes) if cell_sizes else 0,
            'interpretation': 'Ячейки = естественное разбиение архитектуры'
        }
    except Exception as e:
        return {'name': 'morse_smale', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# DISCRETE EXTERIOR CALCULUS (DEC)
# =============================================================================

def validate_discrete_exterior_calculus(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Discrete Exterior Calculus - дискретное внешнее исчисление.

    DEC обобщает векторный анализ на графы:
    - 0-формы: функции на вершинах
    - 1-формы: функции на рёбрах
    - d: внешняя производная (кобраница)
    - δ: ко-производная (граница)
    - Δ = dδ + δd: Лапласиан
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        e = subgraph.number_of_edges()

        if n < 3 or e < 2:
            return {
                'name': 'discrete_exterior_calculus',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов или рёбер'
            }

        node_list = list(nodes)
        edge_list = list(subgraph.edges())
        node_idx = {node: i for i, node in enumerate(node_list)}

        # Boundary operator ∂₁: C₁ → C₀ (edges → vertices)
        boundary_1 = np.zeros((n, e))
        for j, (u, v) in enumerate(edge_list):
            boundary_1[node_idx[u], j] = 1
            boundary_1[node_idx[v], j] = -1

        # Coboundary d₀ = ∂₁ᵀ: C⁰ → C¹
        d0 = boundary_1.T

        # Hodge star operators (diagonal for graphs)
        # *₀: weights on vertices (degree)
        star_0 = np.diag([subgraph.degree(node) for node in node_list])
        star_0_inv = np.diag([1/subgraph.degree(node) if subgraph.degree(node) > 0 else 0
                             for node in node_list])

        # *₁: weights on edges (1 for unweighted)
        star_1 = np.eye(e)

        # Codifferential δ₁ = *₀⁻¹ ∂₁ *₁
        delta_1 = star_0_inv @ boundary_1 @ star_1

        # Laplacians
        # Δ₀ = δ₁ d₀ (Laplacian on 0-forms)
        laplacian_0 = delta_1 @ d0

        # Δ₁ = d₀ δ₁ (Laplacian on 1-forms)
        laplacian_1 = d0 @ delta_1

        # Спектральный анализ
        eig_0 = np.linalg.eigvalsh(laplacian_0)
        eig_1 = np.linalg.eigvalsh(laplacian_1)

        # Harmonic forms
        zero_threshold = 1e-10
        harmonic_0 = np.sum(np.abs(eig_0) < zero_threshold)
        harmonic_1 = np.sum(np.abs(eig_1) < zero_threshold)

        # DEC Dirac operator D = d + δ
        # На графе: D² = Δ

        # Анализ форм разной степени

        # Тестовая 0-форма: centrality на вершинах
        centrality = nx.degree_centrality(subgraph)
        omega_0 = np.array([centrality.get(node, 0) for node in node_list])

        # Тестовая 1-форма: betweenness на рёбрах
        edge_betweenness = nx.edge_betweenness_centrality(subgraph)
        omega_1 = np.array([edge_betweenness.get(e, edge_betweenness.get((e[1], e[0]), 0))
                           for e in edge_list])

        # Применяем d₀ к 0-форме: d₀(ω₀) → 1-форма (градиент)
        d_omega_0 = d0 @ omega_0  # shape (e,)
        grad_norm = np.linalg.norm(d_omega_0)

        # Применяем δ₁ к 1-форме: δ₁(ω₁) → 0-форма (дивергенция)
        # delta_1 имеет shape (n, e), omega_1 shape (e,)
        delta_omega = delta_1 @ omega_1  # shape (n,) - дивергенция

        # Норма дивергенции
        div_norm = np.linalg.norm(delta_omega)

        # Для графа (1-skeleton): d₁ω₁ = 0 (нет 2-клеток)
        # Лапласиан на 1-формах: Δ₁ω₁ = d₀δ₁ω₁ (т.к. d₁=0)
        laplacian_omega_1 = d0 @ delta_omega  # shape (e,)

        return {
            'name': 'discrete_exterior_calculus',
            'description': 'Дискретное внешнее исчисление',
            'status': 'INFO',
            'dimensions': {
                'vertices': n,
                'edges': e
            },
            'harmonic_forms': {
                'h0': int(harmonic_0),
                'h1': int(harmonic_1)
            },
            'spectral_gap_0': round(float(eig_0[eig_0 > zero_threshold].min()), 4) if np.any(eig_0 > zero_threshold) else 0,
            'spectral_gap_1': round(float(eig_1[eig_1 > zero_threshold].min()), 4) if np.any(eig_1 > zero_threshold) else 0,
            'gradient_norm': round(grad_norm, 4),
            'divergence_norm': round(div_norm, 4),
            'laplacian_1_norm': round(float(np.linalg.norm(laplacian_omega_1)), 4),
            'interpretation': 'DEC обобщает div/grad/curl на графы'
        }
    except Exception as e:
        return {'name': 'discrete_exterior_calculus', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# SPECTRAL SEQUENCES
# =============================================================================

def validate_spectral_sequence(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Spectral Sequence Analysis - спектральная последовательность.

    Спектральная последовательность вычисляет гомологии через фильтрацию.
    E₀ → E₁ → E₂ → ... → E∞ = H*

    Для графа: фильтрация по degree.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        e = subgraph.number_of_edges()

        if n < 3:
            return {
                'name': 'spectral_sequence',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов'
            }

        # Фильтрация по степени
        degrees = {node: subgraph.degree(node) for node in nodes}
        max_degree = max(degrees.values())

        # E₀ page: chain groups по фильтрации
        # F_p C_q = chains supported on vertices of degree <= p

        pages = []

        for p in range(max_degree + 1):
            # Вершины степени <= p
            filtered_nodes = [node for node in nodes if degrees[node] <= p]

            if len(filtered_nodes) < 2:
                pages.append({
                    'p': p,
                    'filtered_nodes': len(filtered_nodes),
                    'filtered_edges': 0,
                    'h0': len(filtered_nodes),
                    'h1': 0
                })
                continue

            filtered_subgraph = subgraph.subgraph(filtered_nodes)
            fe = filtered_subgraph.number_of_edges()
            fn = len(filtered_nodes)

            # Гомологии
            c = nx.number_connected_components(filtered_subgraph)
            h0 = c
            h1 = fe - fn + c

            pages.append({
                'p': p,
                'filtered_nodes': fn,
                'filtered_edges': fe,
                'h0': h0,
                'h1': h1
            })

        # Анализ сходимости: E∞ = H* полного графа
        c_full = nx.number_connected_components(subgraph)
        h0_full = c_full
        h1_full = e - n + c_full

        # Дифференциалы: изменения между страницами
        differentials = []
        for i in range(1, len(pages)):
            d_h0 = pages[i]['h0'] - pages[i-1]['h0']
            d_h1 = pages[i]['h1'] - pages[i-1]['h1']
            differentials.append({
                'from_p': pages[i-1]['p'],
                'to_p': pages[i]['p'],
                'd_h0': d_h0,
                'd_h1': d_h1
            })

        # Страница стабилизации
        stabilization_page = max_degree
        for i in range(len(pages) - 1, 0, -1):
            if pages[i]['h0'] != h0_full or pages[i]['h1'] != h1_full:
                stabilization_page = pages[i]['p'] + 1
                break

        return {
            'name': 'spectral_sequence',
            'description': 'Спектральная последовательность',
            'status': 'INFO',
            'filtration_depth': max_degree,
            'pages_sample': pages[:5],
            'e_infinity': {
                'h0': h0_full,
                'h1': h1_full
            },
            'stabilization_page': stabilization_page,
            'differentials_sample': differentials[:5],
            'interpretation': 'Ранняя стабилизация = простая фильтрация'
        }
    except Exception as e:
        return {'name': 'spectral_sequence', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# PERSISTENT PATH HOMOLOGY
# =============================================================================

def validate_path_homology(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Persistent Path Homology - гомология путей для ориентированных графов.

    В отличие от обычной гомологии, path homology учитывает направление.
    Особенно полезно для DAG архитектур.
    """
    max_path_cycles = 5
    if config and config.threshold is not None:
        max_path_cycles = int(config.threshold)
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)  # Сохраняем направление!

        n = len(nodes)
        e = subgraph.number_of_edges()

        if n < 3:
            return {
                'name': 'path_homology',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов'
            }

        # Path homology для DiGraph
        # Ω_n = пространство путей длины n
        # ∂: Ω_n → Ω_{n-1} граничный оператор

        # Пути длины 1 (рёбра)
        paths_1 = list(subgraph.edges())

        # Пути длины 2
        paths_2 = []
        for u, v in subgraph.edges():
            for w in subgraph.successors(v):
                if w != u:  # Избегаем тривиальных
                    paths_2.append((u, v, w))

        # "Циклы" в path homology: пути где начало = конец в проекции
        # Для DAG: ищем параллельные пути
        parallel_paths = []
        for source in nodes:
            for target in nodes:
                if source != target:
                    try:
                        all_paths = list(nx.all_simple_paths(subgraph, source, target, cutoff=4))
                        if len(all_paths) > 1:
                            parallel_paths.append({
                                'source': source,
                                'target': target,
                                'num_paths': len(all_paths),
                                'paths': [p[:4] for p in all_paths[:3]]
                            })
                    except nx.NetworkXNoPath:
                        pass

        # "Path cycles" = количество пар с параллельными путями
        num_path_cycles = len(parallel_paths)

        # Boundary operator analysis
        # ∂(u→v→w) = (v→w) - (u→w) + (u→v)
        # Ищем элементы ядра (циклы)

        # Strongly connected components как path cycles
        sccs = list(nx.strongly_connected_components(subgraph))
        non_trivial_sccs = [scc for scc in sccs if len(scc) > 1]

        # Path Betti numbers (приближение)
        # β₀^path = слабо связные компоненты
        # β₁^path ≈ параллельные пути + SCC

        weak_components = nx.number_weakly_connected_components(subgraph)
        beta_0_path = weak_components
        beta_1_path = len(non_trivial_sccs) + len([p for p in parallel_paths if p['num_paths'] > 2])

        if num_path_cycles > max_path_cycles:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'path_homology',
            'description': f'Path cycles <= {max_path_cycles}',
            'status': status,
            'paths_1': len(paths_1),
            'paths_2': len(paths_2),
            'parallel_path_pairs': num_path_cycles,
            'threshold': max_path_cycles,
            'path_betti': {
                'beta_0': beta_0_path,
                'beta_1': beta_1_path
            },
            'non_trivial_sccs': len(non_trivial_sccs),
            'parallel_paths_sample': parallel_paths[:5],
            'interpretation': 'Параллельные пути = альтернативные зависимости'
        }
    except Exception as e:
        return {'name': 'path_homology', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# WASSERSTEIN DISTANCE
# =============================================================================

def validate_wasserstein_stability(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Wasserstein Distance Stability - устойчивость через расстояние Вассерштейна.

    W_p distance между persistence diagrams.
    Измеряет "стоимость" трансформации одной диаграммы в другую.
    """
    max_wasserstein = 0.5
    if config and config.threshold is not None:
        max_wasserstein = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        edges = list(subgraph.edges())

        if n < 4 or len(edges) < 3:
            return {
                'name': 'wasserstein_stability',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов или рёбер'
            }

        # Строим базовую persistence diagram
        def build_persistence_diagram(g):
            diagram = []
            nodes_g = list(g.nodes())
            if len(nodes_g) < 2:
                return diagram

            # Union-Find
            parent = {node: node for node in nodes_g}

            def find(x):
                if parent[x] != x:
                    parent[x] = find(parent[x])
                return parent[x]

            def union(x, y):
                px, py = find(x), find(y)
                if px != py:
                    parent[py] = px
                    return True
                return False

            # Сортируем рёбра
            edge_list = list(g.edges())
            for idx, (u, v) in enumerate(edge_list):
                t = idx / len(edge_list) if edge_list else 0
                if union(u, v):
                    diagram.append((0, t))  # Component dies

            return diagram

        base_diagram = build_persistence_diagram(subgraph)

        # Вычисляем Wasserstein distance при удалении рёбер
        wasserstein_distances = []

        for edge in edges[:min(20, len(edges))]:  # Limit for performance
            perturbed = subgraph.copy()
            perturbed.remove_edge(*edge)

            perturbed_diagram = build_persistence_diagram(perturbed)

            # Wasserstein-1 distance (Earth Mover's Distance)
            # Упрощённая версия: сумма |d1 - d2| для matched points

            if not base_diagram or not perturbed_diagram:
                continue

            # Сортируем по death time
            base_sorted = sorted(base_diagram, key=lambda x: x[1])
            pert_sorted = sorted(perturbed_diagram, key=lambda x: x[1])

            # Pad to same length
            max_len = max(len(base_sorted), len(pert_sorted))
            while len(base_sorted) < max_len:
                base_sorted.append((0, 0))
            while len(pert_sorted) < max_len:
                pert_sorted.append((0, 0))

            # L1 Wasserstein
            w1 = sum(abs(b[1] - p[1]) for b, p in zip(base_sorted, pert_sorted))
            wasserstein_distances.append({
                'edge': edge,
                'w1_distance': w1
            })

        if not wasserstein_distances:
            return {
                'name': 'wasserstein_stability',
                'status': 'SKIP',
                'reason': 'Не удалось вычислить расстояния'
            }

        w1_values = [wd['w1_distance'] for wd in wasserstein_distances]
        avg_wasserstein = np.mean(w1_values)
        max_wasserstein_found = max(w1_values)

        # Самые нестабильные рёбра
        wasserstein_distances.sort(key=lambda x: -x['w1_distance'])

        if avg_wasserstein > max_wasserstein:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'wasserstein_stability',
            'description': f'Wasserstein устойчивость <= {max_wasserstein}',
            'status': status,
            'avg_wasserstein': round(avg_wasserstein, 4),
            'max_wasserstein': round(max_wasserstein_found, 4),
            'threshold': max_wasserstein,
            'edges_tested': len(wasserstein_distances),
            'most_unstable_edges': wasserstein_distances[:5],
            'interpretation': 'Высокий W₁ = топология чувствительна к изменениям'
        }
    except Exception as e:
        return {'name': 'wasserstein_stability', 'status': 'ERROR', 'error': str(e)}
