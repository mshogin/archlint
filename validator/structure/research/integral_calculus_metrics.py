"""
Integral Calculus Metrics for Software Architecture Validation.

This module implements validations based on integral calculus on graphs:
- Path Integrals: line integrals, circulation
- Potential Theory: Green's function, effective resistance, capacity
- Heat Kernel: trace, diagonal, spectral zeta
- Stokes Theorem & Isoperimetric: boundary integrals, Cheeger constant
- Hitting Times: random walk expected times, commute times
- Integral Transforms: graph Laplace transform, resolvent
"""

from typing import Any, Dict, List, Optional, Tuple, Set
import networkx as nx
import numpy as np
from scipy import linalg
from scipy.sparse import csr_matrix
from scipy.sparse.linalg import spsolve
from collections import defaultdict
import itertools


def _is_excluded(node: str, exclude: List[str]) -> bool:
    """Check if node matches any exclusion pattern."""
    for pattern in exclude:
        if pattern in node:
            return True
    return False


def _get_laplacian_matrix(graph: nx.Graph) -> np.ndarray:
    """Get Laplacian matrix."""
    return nx.laplacian_matrix(graph).toarray().astype(float)


def _get_adjacency_matrix(graph: nx.Graph) -> np.ndarray:
    """Get adjacency matrix."""
    return nx.adjacency_matrix(graph).toarray().astype(float)


def _pseudoinverse(L: np.ndarray) -> np.ndarray:
    """Compute Moore-Penrose pseudoinverse of Laplacian."""
    return np.linalg.pinv(L)


# =============================================================================
# PATH INTEGRALS
# =============================================================================

def validate_path_integral(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Path Integral Analysis - линейные интегралы по путям.

    Для 1-формы ω на рёбрах, интеграл по пути γ = (v₀, v₁, ..., vₖ):
    ∫_γ ω = Σᵢ ω(vᵢ, vᵢ₊₁)

    Если ω = ∇f (градиент), то ∫_γ ω = f(vₖ) - f(v₀) (независимо от пути).

    Для архитектуры: интеграл метрики вдоль цепочки зависимостей.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 3:
            return {
                'name': 'path_integral',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        node_list = list(nodes)

        # 1-форма на рёбрах: edge betweenness
        edge_betweenness = nx.edge_betweenness_centrality(subgraph)

        # Функция на вершинах для градиента: degree centrality
        centrality = nx.degree_centrality(subgraph)

        # Находим несколько путей и вычисляем интегралы
        path_integrals = []

        # Берём пары узлов и находим пути
        sampled_pairs = []
        for u in list(nodes)[:5]:
            for v in list(nodes)[:5]:
                if u != v:
                    sampled_pairs.append((u, v))

        for source, target in sampled_pairs[:10]:
            try:
                # Находим простой путь
                paths = list(nx.all_simple_paths(subgraph, source, target, cutoff=5))
                if not paths:
                    continue

                path = paths[0]  # Берём первый путь

                # Интеграл 1-формы (edge betweenness) по пути
                integral = 0.0
                for i in range(len(path) - 1):
                    edge = (path[i], path[i+1])
                    # Получаем значение 1-формы на ребре
                    omega = edge_betweenness.get(edge, edge_betweenness.get((edge[1], edge[0]), 0))
                    integral += omega

                # Разность потенциала (для сравнения с градиентом)
                potential_diff = centrality.get(target, 0) - centrality.get(source, 0)

                path_integrals.append({
                    'path': f"{source} -> {target}",
                    'length': len(path) - 1,
                    'integral': round(integral, 4),
                    'potential_diff': round(potential_diff, 4)
                })

            except nx.NetworkXNoPath:
                continue

        if not path_integrals:
            return {
                'name': 'path_integral',
                'status': 'SKIP',
                'reason': 'No paths found'
            }

        # Статистики
        integrals = [p['integral'] for p in path_integrals]
        avg_integral = np.mean(integrals)
        max_integral = max(integrals)

        return {
            'name': 'path_integral',
            'description': 'Линейные интегралы по путям',
            'status': 'INFO',
            'paths_analyzed': len(path_integrals),
            'average_integral': round(avg_integral, 4),
            'max_integral': round(max_integral, 4),
            'sample_paths': path_integrals[:5],
            'interpretation': 'Интеграл = накопленная "стоимость" вдоль цепочки зависимостей'
        }
    except Exception as e:
        return {'name': 'path_integral', 'status': 'ERROR', 'error': str(e)}


def validate_circulation(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Circulation Analysis - циркуляция по циклам.

    Циркуляция 1-формы ω по замкнутому пути C:
    ∮_C ω = Σ_{e∈C} ω(e)

    Для градиентного поля (ω = ∇f): ∮_C ∇f = 0 (всегда).
    Ненулевая циркуляция = поле не потенциальное.

    Для архитектуры: циркуляция ≠ 0 указывает на "вихри" в зависимостях.
    """
    exclude = config.exclude if config else []
    threshold = config.threshold if config and config.threshold else 0.1

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 3:
            return {
                'name': 'circulation',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        # 1-форма: edge betweenness (с ориентацией)
        edge_betweenness = nx.edge_betweenness_centrality(subgraph)

        # Находим циклы через cycle_basis
        try:
            cycles = nx.cycle_basis(subgraph)
        except:
            cycles = []

        if not cycles:
            return {
                'name': 'circulation',
                'status': 'INFO',
                'cycles_found': 0,
                'interpretation': 'Нет циклов - граф ациклический (дерево)'
            }

        # Вычисляем циркуляцию по каждому циклу
        circulations = []

        for cycle in cycles[:20]:  # Ограничиваем количество
            if len(cycle) < 3:
                continue

            # Замыкаем цикл
            cycle_closed = cycle + [cycle[0]]

            circ = 0.0
            for i in range(len(cycle_closed) - 1):
                u, v = cycle_closed[i], cycle_closed[i+1]
                # Значение 1-формы (с учётом ориентации)
                if (u, v) in edge_betweenness:
                    circ += edge_betweenness[(u, v)]
                elif (v, u) in edge_betweenness:
                    circ -= edge_betweenness[(v, u)]  # Обратная ориентация

            circulations.append({
                'cycle_length': len(cycle),
                'circulation': round(circ, 4),
                'cycle_sample': cycle[:4] if len(cycle) > 4 else cycle
            })

        if not circulations:
            return {
                'name': 'circulation',
                'status': 'INFO',
                'cycles_found': len(cycles),
                'circulations_computed': 0
            }

        # Статистики
        circ_values = [c['circulation'] for c in circulations]
        max_circ = max(abs(c) for c in circ_values)
        avg_circ = np.mean([abs(c) for c in circ_values])

        # Для неориентированного графа с симметричной 1-формой циркуляция = 0
        # Ненулевая циркуляция указывает на асимметрию

        status = 'PASSED' if max_circ <= threshold else 'WARNING'

        return {
            'name': 'circulation',
            'description': f'Циркуляция по циклам <= {threshold}',
            'status': status,
            'cycles_found': len(cycles),
            'max_circulation': round(max_circ, 4),
            'avg_circulation': round(avg_circ, 4),
            'threshold': threshold,
            'sample_circulations': circulations[:5],
            'interpretation': 'Высокая циркуляция = асимметрия потоков в циклах'
        }
    except Exception as e:
        return {'name': 'circulation', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# POTENTIAL THEORY
# =============================================================================

def validate_green_function(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Green's Function Analysis - функция Грина на графе.

    Функция Грина G = L⁺ (псевдообратный Лапласиан).
    G(x,y) = потенциал в x от единичного источника в y.

    Решает уравнение Пуассона: Lf = g, f = Gg.

    Для архитектуры: G(x,y) показывает влияние модуля y на модуль x.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 3:
            return {
                'name': 'green_function',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        node_list = list(subgraph.nodes())
        L = _get_laplacian_matrix(subgraph)

        # Псевдообратный Лапласиан (функция Грина)
        G = _pseudoinverse(L)

        # Диагональ G(x,x) - "самовлияние"
        G_diag = np.diag(G)

        # Статистики функции Грина
        G_max = np.max(G)
        G_min = np.min(G)
        G_trace = np.trace(G)

        # Узлы с максимальным самовлиянием
        sorted_indices = np.argsort(G_diag)[::-1]
        top_self_influence = [(node_list[i], round(float(G_diag[i]), 4))
                              for i in sorted_indices[:5]]

        # Пары с максимальным влиянием G(x,y)
        max_influences = []
        for i in range(n):
            for j in range(n):
                if i != j:
                    max_influences.append((i, j, G[i, j]))

        max_influences.sort(key=lambda x: abs(x[2]), reverse=True)
        top_pairs = [(node_list[i], node_list[j], round(float(v), 4))
                     for i, j, v in max_influences[:5]]

        return {
            'name': 'green_function',
            'description': 'Функция Грина (псевдообратный Лапласиан)',
            'status': 'INFO',
            'trace_G': round(G_trace, 4),
            'max_G': round(G_max, 4),
            'min_G': round(G_min, 4),
            'top_self_influence': top_self_influence,
            'top_influence_pairs': top_pairs,
            'interpretation': 'G(x,y) = влияние источника в y на потенциал в x'
        }
    except Exception as e:
        return {'name': 'green_function', 'status': 'ERROR', 'error': str(e)}


def validate_effective_resistance(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Effective Resistance - эффективное сопротивление.

    R(a,b) = (eₐ - e_b)^T G (eₐ - e_b) = G(a,a) + G(b,b) - 2G(a,b)

    Интерпретация: если рассматривать граф как электрическую цепь,
    R(a,b) = сопротивление между узлами a и b.

    Связь с random walk: commute_time(a,b) = 2|E| · R(a,b)

    Для архитектуры: высокое сопротивление = слабая связь между модулями.
    """
    exclude = config.exclude if config else []
    threshold = config.threshold if config and config.threshold else 2.0

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        e = subgraph.number_of_edges()

        if n < 3:
            return {
                'name': 'effective_resistance',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        node_list = list(subgraph.nodes())
        L = _get_laplacian_matrix(subgraph)
        G = _pseudoinverse(L)

        # Вычисляем эффективное сопротивление для всех пар
        resistances = []

        for i in range(n):
            for j in range(i + 1, n):
                R_ij = G[i, i] + G[j, j] - 2 * G[i, j]
                resistances.append({
                    'pair': (node_list[i], node_list[j]),
                    'resistance': float(R_ij)
                })

        if not resistances:
            return {
                'name': 'effective_resistance',
                'status': 'SKIP',
                'reason': 'No pairs to analyze'
            }

        # Статистики
        R_values = [r['resistance'] for r in resistances]
        avg_resistance = np.mean(R_values)
        max_resistance = max(R_values)
        min_resistance = min(R_values)

        # Полное сопротивление графа (Kirchhoff index)
        kirchhoff_index = sum(R_values)

        # Пары с максимальным сопротивлением (слабо связанные)
        sorted_resistances = sorted(resistances, key=lambda x: x['resistance'], reverse=True)
        weakly_connected = [(r['pair'][0], r['pair'][1], round(r['resistance'], 4))
                            for r in sorted_resistances[:5]]

        # Пары с минимальным сопротивлением (сильно связанные)
        strongly_connected = [(r['pair'][0], r['pair'][1], round(r['resistance'], 4))
                              for r in sorted_resistances[-5:]]

        status = 'PASSED' if max_resistance <= threshold else 'WARNING'

        return {
            'name': 'effective_resistance',
            'description': f'Эффективное сопротивление <= {threshold}',
            'status': status,
            'kirchhoff_index': round(kirchhoff_index, 4),
            'avg_resistance': round(avg_resistance, 4),
            'max_resistance': round(max_resistance, 4),
            'min_resistance': round(min_resistance, 4),
            'threshold': threshold,
            'weakly_connected_pairs': weakly_connected,
            'strongly_connected_pairs': strongly_connected,
            'interpretation': 'Высокое R = слабая электрическая связь между модулями'
        }
    except Exception as e:
        return {'name': 'effective_resistance', 'status': 'ERROR', 'error': str(e)}


def validate_node_capacity(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Node Capacity - ёмкость узлов.

    Ёмкость cap(v) связана со способностью узла "поглощать" поток.

    Для графа: cap(v) ~ 1/G(v,v) (обратное самовлияние).

    Высокая ёмкость = узел может обрабатывать много зависимостей.
    Низкая ёмкость = узел-бутылочное горлышко.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 3:
            return {
                'name': 'node_capacity',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        node_list = list(subgraph.nodes())
        L = _get_laplacian_matrix(subgraph)
        G = _pseudoinverse(L)

        # Ёмкость как обратное самовлияние
        G_diag = np.diag(G)
        capacities = {}

        for i, node in enumerate(node_list):
            if G_diag[i] > 1e-10:
                cap = 1.0 / G_diag[i]
            else:
                cap = float('inf')
            capacities[node] = cap

        # Сортируем по ёмкости
        sorted_caps = sorted(capacities.items(), key=lambda x: x[1] if x[1] != float('inf') else 1e10)

        # Узлы с низкой ёмкостью (бутылочные горлышки)
        bottlenecks = [(node, round(cap, 4) if cap != float('inf') else 'inf')
                       for node, cap in sorted_caps[:5]]

        # Узлы с высокой ёмкостью
        high_capacity = [(node, round(cap, 4) if cap != float('inf') else 'inf')
                         for node, cap in sorted_caps[-5:]]

        # Статистики
        finite_caps = [c for c in capacities.values() if c != float('inf')]
        if finite_caps:
            avg_capacity = np.mean(finite_caps)
            std_capacity = np.std(finite_caps)
        else:
            avg_capacity = 0
            std_capacity = 0

        return {
            'name': 'node_capacity',
            'description': 'Ёмкость узлов (способность обрабатывать поток)',
            'status': 'INFO',
            'avg_capacity': round(avg_capacity, 4),
            'std_capacity': round(std_capacity, 4),
            'bottlenecks': bottlenecks,
            'high_capacity_nodes': high_capacity,
            'interpretation': 'Низкая ёмкость = бутылочное горлышко в архитектуре'
        }
    except Exception as e:
        return {'name': 'node_capacity', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# HEAT KERNEL
# =============================================================================

def validate_heat_kernel_trace(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Heat Kernel Trace - след ядра теплопроводности.

    K_t = e^{-tL} - ядро теплопроводности.
    Tr(K_t) = Σᵢ e^{-tλᵢ} - спектральная дзета-функция.

    При t→0: Tr(K_t) → n (число узлов)
    При t→∞: Tr(K_t) → (число компонент связности)

    Скорость убывания связана со спектральной щелью.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 3:
            return {
                'name': 'heat_kernel_trace',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        L = _get_laplacian_matrix(subgraph)

        # Собственные значения Лапласиана
        eigenvalues = np.sort(np.linalg.eigvalsh(L))

        # След ядра для разных времён
        times = [0.01, 0.1, 0.5, 1.0, 2.0, 5.0, 10.0]
        traces = []

        for t in times:
            trace = np.sum(np.exp(-t * eigenvalues))
            traces.append({
                't': t,
                'trace': round(float(trace), 4)
            })

        # Спектральная дзета-функция ζ(s) = Σ λᵢ^{-s} (для λᵢ > 0)
        positive_eigenvalues = eigenvalues[eigenvalues > 1e-10]
        if len(positive_eigenvalues) > 0:
            zeta_1 = np.sum(1.0 / positive_eigenvalues)
            zeta_2 = np.sum(1.0 / (positive_eigenvalues ** 2))
        else:
            zeta_1 = float('inf')
            zeta_2 = float('inf')

        # Число компонент связности (λ=0 с кратностью)
        num_components = np.sum(eigenvalues < 1e-10)

        # Спектральная щель
        spectral_gap = eigenvalues[int(num_components)] if num_components < n else 0

        return {
            'name': 'heat_kernel_trace',
            'description': 'След ядра теплопроводности Tr(e^{-tL})',
            'status': 'INFO',
            'num_nodes': n,
            'num_components': int(num_components),
            'spectral_gap': round(float(spectral_gap), 4),
            'trace_evolution': traces,
            'spectral_zeta': {
                'zeta_1': round(zeta_1, 4) if zeta_1 != float('inf') else 'inf',
                'zeta_2': round(zeta_2, 4) if zeta_2 != float('inf') else 'inf'
            },
            'interpretation': 'Tr(K_t) показывает "размер" графа на масштабе t'
        }
    except Exception as e:
        return {'name': 'heat_kernel_trace', 'status': 'ERROR', 'error': str(e)}


def validate_heat_kernel_diagonal(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Heat Kernel Diagonal - диагональ ядра теплопроводности.

    K_t(x,x) = Σᵢ e^{-tλᵢ} |φᵢ(x)|² - локальная "теплоёмкость".

    Высокое K_t(x,x) = узел x медленно отдаёт тепло = изолирован.
    Низкое K_t(x,x) = узел x быстро диффундирует = хорошо связан.

    Связь с локальной геометрией и кривизной.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 3:
            return {
                'name': 'heat_kernel_diagonal',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        node_list = list(subgraph.nodes())
        L = _get_laplacian_matrix(subgraph)

        # Собственное разложение
        eigenvalues, eigenvectors = np.linalg.eigh(L)

        # Вычисляем K_t(x,x) для фиксированного t
        t = 1.0  # Характерное время

        K_diag = np.zeros(n)
        for i in range(n):
            for k in range(n):
                K_diag[i] += np.exp(-t * eigenvalues[k]) * (eigenvectors[i, k] ** 2)

        # Узлы с высокой диагональю (изолированные)
        sorted_indices = np.argsort(K_diag)[::-1]
        isolated_nodes = [(node_list[i], round(float(K_diag[i]), 4))
                          for i in sorted_indices[:5]]

        # Узлы с низкой диагональю (хорошо связанные)
        well_connected = [(node_list[i], round(float(K_diag[i]), 4))
                          for i in sorted_indices[-5:]]

        # Статистики
        avg_diag = np.mean(K_diag)
        std_diag = np.std(K_diag)

        # Неравномерность диагонали
        uniformity = std_diag / avg_diag if avg_diag > 0 else 0

        return {
            'name': 'heat_kernel_diagonal',
            'description': 'Диагональ ядра K_t(x,x) при t=1',
            'status': 'INFO',
            'time_scale': t,
            'avg_diagonal': round(avg_diag, 4),
            'std_diagonal': round(std_diag, 4),
            'uniformity': round(uniformity, 4),
            'isolated_nodes': isolated_nodes,
            'well_connected_nodes': well_connected,
            'interpretation': 'Высокая K(x,x) = узел изолирован, низкая = хорошо связан'
        }
    except Exception as e:
        return {'name': 'heat_kernel_diagonal', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# STOKES THEOREM & ISOPERIMETRIC
# =============================================================================

def validate_stokes_theorem(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Stokes Theorem on Graphs - теорема Стокса.

    ∫_∂Ω ω = ∫_Ω dω

    Для графа: сумма по границе = сумма производных внутри.

    Граница подграфа ∂S = рёбра между S и V\S.
    Проверяем согласованность потоков.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 4:
            return {
                'name': 'stokes_theorem',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        node_list = list(subgraph.nodes())

        # 0-форма на вершинах (потенциал)
        centrality = nx.degree_centrality(subgraph)
        f = {node: centrality[node] for node in node_list}

        # Выбираем несколько подмножеств и проверяем теорему Стокса
        stokes_checks = []

        # Случайные подмножества
        for size in [2, 3, n // 2]:
            if size >= n or size < 2:
                continue

            S = set(node_list[:size])
            complement = set(node_list) - S

            # Граница: рёбра между S и complement
            boundary_edges = []
            for u in S:
                for v in complement:
                    if subgraph.has_edge(u, v):
                        boundary_edges.append((u, v))

            if not boundary_edges:
                continue

            # ∫_∂S df = Σ_{(u,v)∈∂S} (f(v) - f(u)), где u∈S, v∉S
            boundary_integral = sum(f[v] - f[u] for u, v in boundary_edges)

            # Альтернативно: Σ_{v∈S} (Lf)(v) (дискретный аналог ∫_S Δf)
            L = _get_laplacian_matrix(subgraph)
            f_vec = np.array([f[node] for node in node_list])
            Lf = L @ f_vec

            interior_integral = sum(Lf[node_list.index(v)] for v in S)

            stokes_checks.append({
                'subset_size': len(S),
                'boundary_size': len(boundary_edges),
                'boundary_integral': round(boundary_integral, 4),
                'interior_laplacian': round(interior_integral, 4),
                'difference': round(abs(boundary_integral - interior_integral), 6)
            })

        return {
            'name': 'stokes_theorem',
            'description': 'Проверка теоремы Стокса на графе',
            'status': 'INFO',
            'checks': stokes_checks,
            'interpretation': '∫_∂Ω ω ≈ ∫_Ω dω - связь границы и внутренности'
        }
    except Exception as e:
        return {'name': 'stokes_theorem', 'status': 'ERROR', 'error': str(e)}


def validate_cheeger_constant(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Cheeger Constant - константа Чигера (изопериметрическая константа).

    h(G) = min_S |∂S| / min(|S|, |V\S|)

    где ∂S = рёбра между S и V\S.

    Cheeger inequality: λ₂/2 ≤ h ≤ √(2λ₂)

    Высокий h = граф хорошо связан, трудно разрезать.
    Низкий h = есть "бутылочное горлышко".
    """
    exclude = config.exclude if config else []
    threshold = config.threshold if config and config.threshold else 0.3

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 4:
            return {
                'name': 'cheeger_constant',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        if n > 100:
            return {
                'name': 'cheeger_constant',
                'status': 'SKIPPED',
                'value': None,
                'message': f'Skipped: graph too large ({n} nodes, limit 100) - algorithm is O(n^3)/NP-hard',
                'threshold': 100,
            }

        node_list = list(subgraph.nodes())

        # Приближённый поиск минимального разреза через Fiedler vector
        L = _get_laplacian_matrix(subgraph)
        eigenvalues, eigenvectors = np.linalg.eigh(L)

        lambda_2 = eigenvalues[1] if len(eigenvalues) > 1 else 0
        fiedler = eigenvectors[:, 1]

        # Разбиваем по знаку Fiedler vector
        S_positive = [node_list[i] for i in range(n) if fiedler[i] > 0]
        S_negative = [node_list[i] for i in range(n) if fiedler[i] <= 0]

        # Вычисляем разрез
        def compute_cut(S):
            S_set = set(S)
            cut_edges = 0
            for u, v in subgraph.edges():
                if (u in S_set) != (v in S_set):
                    cut_edges += 1
            return cut_edges

        cut_positive = compute_cut(S_positive)
        cut_negative = compute_cut(S_negative)

        # Cheeger ratio для каждого разбиения
        if len(S_positive) > 0 and len(S_negative) > 0:
            h_positive = cut_positive / min(len(S_positive), len(S_negative))
            h_negative = cut_negative / min(len(S_positive), len(S_negative))
            cheeger_approx = min(h_positive, h_negative)
        else:
            cheeger_approx = float('inf')

        # Границы Чигера через λ₂
        cheeger_lower = lambda_2 / 2
        cheeger_upper = np.sqrt(2 * lambda_2) if lambda_2 > 0 else 0

        status = 'PASSED' if cheeger_approx >= threshold else 'WARNING'

        return {
            'name': 'cheeger_constant',
            'description': f'Константа Чигера >= {threshold}',
            'status': status,
            'cheeger_approximation': round(cheeger_approx, 4) if cheeger_approx != float('inf') else 'inf',
            'lambda_2': round(float(lambda_2), 4),
            'cheeger_bounds': {
                'lower': round(cheeger_lower, 4),
                'upper': round(cheeger_upper, 4)
            },
            'partition_sizes': {
                'positive': len(S_positive),
                'negative': len(S_negative)
            },
            'cut_size': cut_positive,
            'threshold': threshold,
            'interpretation': 'Низкий Cheeger = есть бутылочное горлышко в архитектуре'
        }
    except Exception as e:
        return {'name': 'cheeger_constant', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# HITTING TIMES (RANDOM WALK)
# =============================================================================

def validate_hitting_time(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Hitting Time Analysis - времена попадания random walk.

    H(a→b) = E[время достижения b, стартуя из a]

    Решается через: H(a→b) = 1 + Σ_{c≠b} P(a,c) H(c→b)

    Или через функцию Грина: H(a→b) = (G(b,b) - G(a,b)) / π(b)
    где π - стационарное распределение.

    Высокое H = сложно достичь модуль b из a.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 3:
            return {
                'name': 'hitting_time',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        # Проверяем связность
        if not nx.is_connected(subgraph):
            return {
                'name': 'hitting_time',
                'status': 'SKIP',
                'reason': 'Graph not connected'
            }

        node_list = list(subgraph.nodes())

        # Матрица переходов random walk
        A = _get_adjacency_matrix(subgraph)
        degrees = np.sum(A, axis=1)
        D_inv = np.diag(1.0 / np.maximum(degrees, 1))
        P = D_inv @ A  # Transition matrix

        # Стационарное распределение π = d / (2|E|)
        total_degree = np.sum(degrees)
        pi = degrees / total_degree

        # Функция Грина
        L = _get_laplacian_matrix(subgraph)
        G = _pseudoinverse(L)

        # Вычисляем времена попадания для выборки пар
        hitting_times = []

        for i in range(min(n, 5)):
            for j in range(min(n, 5)):
                if i != j and pi[j] > 1e-10:
                    # H(i→j) = (G(j,j) - G(i,j)) * 2|E| / d(j)
                    # Упрощённо через объём
                    H_ij = (G[j, j] - G[i, j]) / pi[j]
                    hitting_times.append({
                        'from': node_list[i],
                        'to': node_list[j],
                        'hitting_time': round(float(H_ij), 2)
                    })

        # Статистики
        if hitting_times:
            times = [h['hitting_time'] for h in hitting_times]
            avg_time = np.mean(times)
            max_time = max(times)

            # Самые труднодостижимые пары
            sorted_times = sorted(hitting_times, key=lambda x: x['hitting_time'], reverse=True)
            hardest_pairs = sorted_times[:5]
        else:
            avg_time = 0
            max_time = 0
            hardest_pairs = []

        return {
            'name': 'hitting_time',
            'description': 'Времена попадания random walk',
            'status': 'INFO',
            'avg_hitting_time': round(avg_time, 2),
            'max_hitting_time': round(max_time, 2),
            'hardest_to_reach': hardest_pairs,
            'interpretation': 'Высокое H(a→b) = сложно достичь b из a случайным блужданием'
        }
    except Exception as e:
        return {'name': 'hitting_time', 'status': 'ERROR', 'error': str(e)}


def validate_commute_time(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Commute Time Analysis - времена коммутации.

    κ(a,b) = H(a→b) + H(b→a) = 2|E| · R(a,b)

    где R(a,b) - эффективное сопротивление.

    Commute time симметричен и определяет метрику на графе.

    Низкое κ = узлы "близки" в смысле random walk.
    """
    exclude = config.exclude if config else []
    threshold = config.threshold if config and config.threshold else 50.0

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        e = subgraph.number_of_edges()

        if n < 3 or not nx.is_connected(subgraph):
            return {
                'name': 'commute_time',
                'status': 'SKIP',
                'reason': 'Insufficient nodes or not connected'
            }

        node_list = list(subgraph.nodes())
        L = _get_laplacian_matrix(subgraph)
        G = _pseudoinverse(L)

        # κ(a,b) = 2|E| · R(a,b) = 2|E| · (G(a,a) + G(b,b) - 2G(a,b))
        commute_times = []

        for i in range(n):
            for j in range(i + 1, n):
                R_ij = G[i, i] + G[j, j] - 2 * G[i, j]
                kappa = 2 * e * R_ij
                commute_times.append({
                    'pair': (node_list[i], node_list[j]),
                    'commute_time': float(kappa),
                    'resistance': float(R_ij)
                })

        # Статистики
        kappas = [c['commute_time'] for c in commute_times]
        avg_commute = np.mean(kappas)
        max_commute = max(kappas)

        # Пары с максимальным временем коммутации
        sorted_commute = sorted(commute_times, key=lambda x: x['commute_time'], reverse=True)
        distant_pairs = [(c['pair'][0], c['pair'][1], round(c['commute_time'], 2))
                         for c in sorted_commute[:5]]

        # Пары с минимальным временем
        close_pairs = [(c['pair'][0], c['pair'][1], round(c['commute_time'], 2))
                       for c in sorted_commute[-5:]]

        status = 'PASSED' if max_commute <= threshold else 'WARNING'

        return {
            'name': 'commute_time',
            'description': f'Время коммутации <= {threshold}',
            'status': status,
            'num_edges': e,
            'avg_commute_time': round(avg_commute, 2),
            'max_commute_time': round(max_commute, 2),
            'threshold': threshold,
            'most_distant_pairs': distant_pairs,
            'closest_pairs': close_pairs,
            'interpretation': 'κ(a,b) = время туда-обратно random walk, метрика на графе'
        }
    except Exception as e:
        return {'name': 'commute_time', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# INTEGRAL TRANSFORMS
# =============================================================================

def validate_graph_laplace_transform(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Graph Laplace Transform - преобразование Лапласа на графе.

    F(s) = (sI + L)⁻¹ - резольвента со сдвигом.

    Для сигнала f: F(s)f = ∫₀^∞ e^{-st} e^{-Lt} f dt

    Полюсы F(s) находятся при s = -λᵢ (собственные значения L).

    Анализ передаточной функции архитектуры.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 3:
            return {
                'name': 'graph_laplace_transform',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        node_list = list(subgraph.nodes())
        L = _get_laplacian_matrix(subgraph)
        I = np.eye(n)

        # Собственные значения (полюсы при s = -λ)
        eigenvalues = np.sort(np.linalg.eigvalsh(L))

        # Вычисляем F(s) для разных s
        s_values = [0.1, 0.5, 1.0, 2.0, 5.0]
        transform_results = []

        for s in s_values:
            try:
                F_s = np.linalg.inv(s * I + L)

                # Норма резольвенты
                norm_F = np.linalg.norm(F_s, ord=2)

                # След
                trace_F = np.trace(F_s)

                transform_results.append({
                    's': s,
                    'norm': round(float(norm_F), 4),
                    'trace': round(float(trace_F), 4)
                })
            except np.linalg.LinAlgError:
                continue

        # Полюсы (отрицательные собственные значения)
        poles = [-float(lam) for lam in eigenvalues[:5]]

        # Оценка устойчивости: все полюсы <= 0
        max_pole = max(poles) if poles else 0
        is_stable = max_pole <= 0

        return {
            'name': 'graph_laplace_transform',
            'description': 'Преобразование Лапласа F(s) = (sI + L)⁻¹',
            'status': 'INFO',
            'poles': [round(p, 4) for p in poles],
            'max_pole': round(max_pole, 4),
            'is_stable': is_stable,
            'transform_values': transform_results,
            'interpretation': 'Полюсы определяют динамику системы, все ≤ 0 = устойчива'
        }
    except Exception as e:
        return {'name': 'graph_laplace_transform', 'status': 'ERROR', 'error': str(e)}


def validate_resolvent_analysis(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Resolvent Analysis - анализ резольвенты.

    R_λ = (λI - L)⁻¹ - резольвента Лапласиана.

    Особенности R_λ находятся в спектре L.
    ||R_λ|| = 1/dist(λ, σ(L)) - расстояние до спектра.

    Псевдоспектр: Λ_ε = {λ : ||R_λ|| > 1/ε}

    Анализ чувствительности архитектуры к возмущениям.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 3:
            return {
                'name': 'resolvent_analysis',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        L = _get_laplacian_matrix(subgraph)
        I = np.eye(n)

        # Спектр Лапласиана
        eigenvalues = np.sort(np.linalg.eigvalsh(L))

        # Анализируем резольвенту вдоль вещественной оси
        # Выбираем точки между собственными значениями
        test_points = []
        for i in range(len(eigenvalues) - 1):
            mid = (eigenvalues[i] + eigenvalues[i + 1]) / 2
            if mid > 0.01:  # Избегаем 0
                test_points.append(mid)

        # Добавляем точки за пределами спектра
        test_points.extend([eigenvalues[-1] + 1, eigenvalues[-1] + 5])

        resolvent_norms = []
        for lam in test_points[:10]:
            try:
                R_lam = np.linalg.inv(lam * I - L)
                norm_R = np.linalg.norm(R_lam, ord=2)

                # Расстояние до ближайшего собственного значения
                dist_to_spectrum = min(abs(lam - ev) for ev in eigenvalues)

                resolvent_norms.append({
                    'lambda': round(lam, 4),
                    'resolvent_norm': round(norm_R, 4),
                    'dist_to_spectrum': round(dist_to_spectrum, 4),
                    'theory_bound': round(1.0 / dist_to_spectrum, 4) if dist_to_spectrum > 1e-10 else 'inf'
                })
            except np.linalg.LinAlgError:
                continue

        # Псевдоспектральный радиус
        # ε-псевдоспектр содержит λ где ||R_λ|| > 1/ε
        epsilon = 0.1
        pseudospectral_points = [r for r in resolvent_norms
                                  if r['resolvent_norm'] > 1.0 / epsilon]

        return {
            'name': 'resolvent_analysis',
            'description': 'Анализ резольвенты R_λ = (λI - L)⁻¹',
            'status': 'INFO',
            'spectrum': [round(float(ev), 4) for ev in eigenvalues[:5]],
            'spectral_radius': round(float(eigenvalues[-1]), 4),
            'resolvent_samples': resolvent_norms[:5],
            'pseudospectral_extent': len(pseudospectral_points),
            'interpretation': '||R_λ|| показывает чувствительность при возмущении спектра к λ'
        }
    except Exception as e:
        return {'name': 'resolvent_analysis', 'status': 'ERROR', 'error': str(e)}
