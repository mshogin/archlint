"""
Mathematical Analysis Metrics for Software Architecture Validation.

This module implements validations based on mathematical analysis:
- Calculus on Graphs: gradient, heat diffusion, Laplacian smoothness
- Functional Analysis: operator norms, perturbation sensitivity, Sobolev regularity
- Variational Methods: Dirichlet energy, geodesics, optimal placement
- Dynamical Systems: Lyapunov stability, bifurcations, attractors
- Harmonic Analysis: Graph Fourier, wavelets, spectral filtering
- Complex Analysis: Ihara zeta function, cycle polynomials
"""

from typing import Any, Dict, List, Optional, Tuple
import networkx as nx
import numpy as np
from scipy import linalg
from scipy.sparse import csr_matrix
from scipy.sparse.linalg import eigsh, expm_multiply
from collections import defaultdict


def _is_excluded(node: str, exclude: List[str]) -> bool:
    """Check if node matches any exclusion pattern."""
    for pattern in exclude:
        if pattern in node:
            return True
    return False


def _get_laplacian_matrix(graph: nx.Graph) -> np.ndarray:
    """Get normalized Laplacian matrix."""
    L = nx.laplacian_matrix(graph).toarray().astype(float)
    return L


def _get_adjacency_matrix(graph: nx.Graph) -> np.ndarray:
    """Get adjacency matrix."""
    return nx.adjacency_matrix(graph).toarray().astype(float)


# =============================================================================
# CALCULUS ON GRAPHS
# =============================================================================

def validate_gradient_flow(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Gradient Flow Analysis - анализ дискретного градиента на графе.

    Дискретный градиент для функции f на вершинах:
    (∇f)(u,v) = f(v) - f(u)

    Анализируем потоки метрик (centrality, degree) вдоль рёбер.
    Высокий градиент = резкие переходы сложности.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        if len(nodes) < 3:
            return {
                'name': 'gradient_flow',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        # Функция на вершинах: degree centrality
        centrality = nx.degree_centrality(subgraph)

        # Вычисляем градиент на каждом ребре
        gradients = []
        gradient_edges = []

        for u, v in subgraph.edges():
            grad = centrality.get(v, 0) - centrality.get(u, 0)
            gradients.append(grad)
            gradient_edges.append((u, v, grad))

        if not gradients:
            return {
                'name': 'gradient_flow',
                'status': 'SKIP',
                'reason': 'No edges'
            }

        gradients = np.array(gradients)

        # Статистики градиента
        grad_mean = np.mean(gradients)
        grad_std = np.std(gradients)
        grad_max = np.max(np.abs(gradients))

        # Энергия градиента (Dirichlet-подобная)
        grad_energy = np.sum(gradients ** 2)

        # Рёбра с максимальным градиентом (резкие переходы)
        sorted_edges = sorted(gradient_edges, key=lambda x: abs(x[2]), reverse=True)
        steep_edges = [(e[0], e[1], round(e[2], 4)) for e in sorted_edges[:5]]

        # Поток: сумма градиентов (должна быть близка к 0 для консервативного поля)
        total_flow = np.sum(gradients)

        return {
            'name': 'gradient_flow',
            'description': 'Дискретный градиент centrality',
            'status': 'INFO',
            'gradient_stats': {
                'mean': round(grad_mean, 4),
                'std': round(grad_std, 4),
                'max_abs': round(grad_max, 4)
            },
            'gradient_energy': round(grad_energy, 4),
            'total_flow': round(total_flow, 4),
            'steepest_edges': steep_edges,
            'interpretation': 'Высокий градиент = резкие переходы сложности между модулями'
        }
    except Exception as e:
        return {'name': 'gradient_flow', 'status': 'ERROR', 'error': str(e)}


def validate_heat_diffusion(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Heat Diffusion Analysis - уравнение теплопроводности на графе.

    ∂f/∂t = -L·f, где L - Лапласиан
    Решение: f(t) = e^(-tL) · f(0)

    Моделирует распространение изменений по архитектуре.
    Быстрая диффузия = хорошая связность.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 3:
            return {
                'name': 'heat_diffusion',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        # Лапласиан
        L = _get_laplacian_matrix(subgraph)

        # Начальное условие: дельта-функция на узле с max degree
        node_list = list(subgraph.nodes())
        degrees = dict(subgraph.degree())
        max_deg_node = max(degrees, key=degrees.get)
        max_deg_idx = node_list.index(max_deg_node)

        f0 = np.zeros(n)
        f0[max_deg_idx] = 1.0

        # Временные шаги для диффузии
        times = [0.1, 0.5, 1.0, 2.0, 5.0]
        diffusion_results = []

        L_sparse = csr_matrix(-L)

        for t in times:
            # f(t) = e^(-tL) · f(0)
            try:
                f_t = expm_multiply(L_sparse * t, f0)
                # Энтропия распределения
                f_t_pos = np.abs(f_t) + 1e-10
                f_t_norm = f_t_pos / f_t_pos.sum()
                entropy = -np.sum(f_t_norm * np.log(f_t_norm))
                diffusion_results.append({
                    't': t,
                    'entropy': round(entropy, 4),
                    'max_value': round(float(np.max(f_t)), 4),
                    'spread': round(float(np.sum(f_t > 0.01)), 0)
                })
            except:
                continue

        # Время релаксации (связано с λ₂)
        try:
            eigenvalues = np.linalg.eigvalsh(L)
            eigenvalues = np.sort(eigenvalues)
            lambda_2 = eigenvalues[1] if len(eigenvalues) > 1 and eigenvalues[1] > 1e-10 else 0.01
            relaxation_time = 1.0 / lambda_2
        except:
            relaxation_time = float('inf')

        return {
            'name': 'heat_diffusion',
            'description': 'Диффузия тепла на графе',
            'status': 'INFO',
            'source_node': max_deg_node,
            'diffusion_evolution': diffusion_results,
            'relaxation_time': round(relaxation_time, 4) if relaxation_time != float('inf') else 'inf',
            'interpretation': 'Быстрая диффузия = изменения быстро распространяются по архитектуре'
        }
    except Exception as e:
        return {'name': 'heat_diffusion', 'status': 'ERROR', 'error': str(e)}


def validate_laplacian_smoothness(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Laplacian Smoothness - гладкость функций на графе.

    Гладкость f: S(f) = f^T · L · f / ||f||²
    Малое S(f) = функция меняется плавно вдоль рёбер.

    Для архитектуры: плавное изменение сложности = хороший дизайн.
    """
    exclude = config.exclude if config else []
    threshold = config.threshold if config and config.threshold else 1.0

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 3:
            return {
                'name': 'laplacian_smoothness',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        node_list = list(subgraph.nodes())
        L = _get_laplacian_matrix(subgraph)

        # Разные функции для анализа гладкости
        smoothness_results = {}

        # 1. Degree centrality
        centrality = nx.degree_centrality(subgraph)
        f_cent = np.array([centrality[node] for node in node_list])
        if np.linalg.norm(f_cent) > 1e-10:
            s_cent = float(f_cent @ L @ f_cent) / (np.linalg.norm(f_cent) ** 2)
            smoothness_results['degree_centrality'] = round(s_cent, 4)

        # 2. Betweenness centrality
        betweenness = nx.betweenness_centrality(subgraph)
        f_betw = np.array([betweenness[node] for node in node_list])
        if np.linalg.norm(f_betw) > 1e-10:
            s_betw = float(f_betw @ L @ f_betw) / (np.linalg.norm(f_betw) ** 2)
            smoothness_results['betweenness'] = round(s_betw, 4)

        # 3. PageRank
        try:
            pagerank = nx.pagerank(subgraph)
            f_pr = np.array([pagerank[node] for node in node_list])
            if np.linalg.norm(f_pr) > 1e-10:
                s_pr = float(f_pr @ L @ f_pr) / (np.linalg.norm(f_pr) ** 2)
                smoothness_results['pagerank'] = round(s_pr, 4)
        except:
            pass

        # 4. Clustering coefficient
        clustering = nx.clustering(subgraph)
        f_clust = np.array([clustering[node] for node in node_list])
        if np.linalg.norm(f_clust) > 1e-10:
            s_clust = float(f_clust @ L @ f_clust) / (np.linalg.norm(f_clust) ** 2)
            smoothness_results['clustering'] = round(s_clust, 4)

        # Средняя гладкость
        avg_smoothness = np.mean(list(smoothness_results.values())) if smoothness_results else 0

        # Оценка
        status = 'PASSED' if avg_smoothness <= threshold else 'WARNING'

        return {
            'name': 'laplacian_smoothness',
            'description': f'Лапласианова гладкость <= {threshold}',
            'status': status,
            'smoothness_by_metric': smoothness_results,
            'average_smoothness': round(avg_smoothness, 4),
            'threshold': threshold,
            'interpretation': 'Низкая гладкость = метрики резко меняются между соседними модулями'
        }
    except Exception as e:
        return {'name': 'laplacian_smoothness', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# FUNCTIONAL ANALYSIS
# =============================================================================

def validate_operator_norm(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Operator Norm Analysis - нормы операторов на графе.

    ||A||₂ = σ_max (спектральная норма)
    ||L||₂ = λ_max (норма Лапласиана)

    Большая норма = сильное "усиление" сигналов в архитектуре.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 3:
            return {
                'name': 'operator_norm',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        A = _get_adjacency_matrix(subgraph)
        L = _get_laplacian_matrix(subgraph)

        # Спектральные нормы
        adj_norm = np.linalg.norm(A, ord=2)  # max singular value
        lap_norm = np.linalg.norm(L, ord=2)

        # Frobenius norms
        adj_frob = np.linalg.norm(A, ord='fro')
        lap_frob = np.linalg.norm(L, ord='fro')

        # Nuclear norm (сумма сингулярных значений)
        try:
            U, s, Vt = np.linalg.svd(A)
            adj_nuclear = np.sum(s)
        except:
            adj_nuclear = 0

        # Число обусловленности (для невырожденного L+I)
        try:
            L_reg = L + 0.01 * np.eye(n)  # регуляризация
            cond_number = np.linalg.cond(L_reg)
        except:
            cond_number = float('inf')

        return {
            'name': 'operator_norm',
            'description': 'Нормы операторов графа',
            'status': 'INFO',
            'adjacency_norms': {
                'spectral': round(adj_norm, 4),
                'frobenius': round(adj_frob, 4),
                'nuclear': round(adj_nuclear, 4)
            },
            'laplacian_norms': {
                'spectral': round(lap_norm, 4),
                'frobenius': round(lap_frob, 4)
            },
            'condition_number': round(cond_number, 4) if cond_number != float('inf') else 'inf',
            'interpretation': 'Высокая норма = сильное усиление возмущений в системе'
        }
    except Exception as e:
        return {'name': 'operator_norm', 'status': 'ERROR', 'error': str(e)}


def validate_perturbation_sensitivity(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Perturbation Sensitivity - чувствительность к возмущениям.

    Анализируем как малые изменения в графе влияют на спектр.
    Использует теорию возмущений Вейля: |λᵢ(A+E) - λᵢ(A)| ≤ ||E||₂

    Высокая чувствительность = нестабильная архитектура.
    """
    exclude = config.exclude if config else []
    threshold = config.threshold if config and config.threshold else 0.5

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 3:
            return {
                'name': 'perturbation_sensitivity',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        L = _get_laplacian_matrix(subgraph)

        # Исходный спектр
        orig_eigenvalues = np.sort(np.linalg.eigvalsh(L))

        # Симулируем возмущения (удаление рёбер)
        edges = list(subgraph.edges())
        sensitivities = []

        for edge in edges[:min(10, len(edges))]:  # Проверяем до 10 рёбер
            # Создаём возмущённый граф
            perturbed = subgraph.copy()
            perturbed.remove_edge(*edge)

            if perturbed.number_of_edges() == 0:
                continue

            # Спектр возмущённого графа
            L_pert = nx.laplacian_matrix(perturbed).toarray().astype(float)
            pert_eigenvalues = np.sort(np.linalg.eigvalsh(L_pert))

            # Сравниваем спектры (Weyl distance)
            min_len = min(len(orig_eigenvalues), len(pert_eigenvalues))
            spectral_shift = np.max(np.abs(orig_eigenvalues[:min_len] - pert_eigenvalues[:min_len]))

            sensitivities.append({
                'edge': edge,
                'spectral_shift': float(spectral_shift)
            })

        if not sensitivities:
            return {
                'name': 'perturbation_sensitivity',
                'status': 'SKIP',
                'reason': 'Cannot compute perturbations'
            }

        # Статистики
        shifts = [s['spectral_shift'] for s in sensitivities]
        max_sensitivity = max(shifts)
        avg_sensitivity = np.mean(shifts)

        # Наиболее критичные рёбра
        sorted_sens = sorted(sensitivities, key=lambda x: x['spectral_shift'], reverse=True)
        critical_edges = [(s['edge'][0], s['edge'][1], round(s['spectral_shift'], 4))
                          for s in sorted_sens[:5]]

        status = 'PASSED' if avg_sensitivity <= threshold else 'WARNING'

        return {
            'name': 'perturbation_sensitivity',
            'description': f'Чувствительность к возмущениям <= {threshold}',
            'status': status,
            'max_sensitivity': round(max_sensitivity, 4),
            'avg_sensitivity': round(avg_sensitivity, 4),
            'critical_edges': critical_edges,
            'threshold': threshold,
            'interpretation': 'Высокая чувствительность = удаление связи сильно меняет структуру'
        }
    except Exception as e:
        return {'name': 'perturbation_sensitivity', 'status': 'ERROR', 'error': str(e)}


def validate_sobolev_regularity(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Sobolev Regularity - регулярность Соболева на графе.

    Пространство Соболева H^s(G) с нормой:
    ||f||²_{H^s} = Σ (1 + λᵢ)^s |<f, φᵢ>|²

    где λᵢ, φᵢ - собственные значения/векторы Лапласиана.
    Высокий порядок s = функция очень гладкая.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 4:
            return {
                'name': 'sobolev_regularity',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        node_list = list(subgraph.nodes())
        L = _get_laplacian_matrix(subgraph)

        # Собственное разложение Лапласиана
        eigenvalues, eigenvectors = np.linalg.eigh(L)

        # Функция для анализа: betweenness centrality
        betweenness = nx.betweenness_centrality(subgraph)
        f = np.array([betweenness[node] for node in node_list])

        # Коэффициенты Фурье на графе
        f_hat = eigenvectors.T @ f

        # Соболевские нормы для разных порядков s
        sobolev_norms = {}
        for s in [0, 0.5, 1, 2]:
            weights = (1 + eigenvalues) ** s
            norm_sq = np.sum(weights * (f_hat ** 2))
            sobolev_norms[f's={s}'] = round(float(np.sqrt(norm_sq)), 4)

        # Оценка регулярности: при каком s норма "взрывается"
        # Ищем максимальное s, при котором норма конечна и разумна
        regularity_order = 2
        for s in [2, 1, 0.5, 0]:
            key = f's={s}'
            if sobolev_norms[key] < 100:  # эвристический порог
                regularity_order = s
                break

        # Энергетический спектр: |f_hat|² vs λ
        energy_spectrum = []
        for i in range(min(5, n)):
            energy_spectrum.append({
                'mode': i,
                'eigenvalue': round(float(eigenvalues[i]), 4),
                'energy': round(float(f_hat[i] ** 2), 6)
            })

        return {
            'name': 'sobolev_regularity',
            'description': 'Соболевская регулярность на графе',
            'status': 'INFO',
            'sobolev_norms': sobolev_norms,
            'regularity_order': regularity_order,
            'energy_spectrum': energy_spectrum,
            'interpretation': 'Высокий порядок регулярности = метрики плавно распределены'
        }
    except Exception as e:
        return {'name': 'sobolev_regularity', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# VARIATIONAL METHODS
# =============================================================================

def validate_dirichlet_energy(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Dirichlet Energy - энергия Дирихле на графе.

    E(f) = ½ Σ_{(u,v)∈E} w(u,v) |f(u) - f(v)|²
         = ½ f^T L f

    Минимизация энергии Дирихле даёт гармонические функции.
    Низкая энергия = плавное распределение.
    """
    exclude = config.exclude if config else []
    threshold = config.threshold if config and config.threshold else 1.0

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 3:
            return {
                'name': 'dirichlet_energy',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        node_list = list(subgraph.nodes())
        L = _get_laplacian_matrix(subgraph)

        # Вычисляем энергию для разных функций
        energies = {}

        # 1. Degree
        degrees = dict(subgraph.degree())
        f_deg = np.array([degrees[node] for node in node_list], dtype=float)
        if np.linalg.norm(f_deg) > 0:
            f_deg_norm = f_deg / np.linalg.norm(f_deg)
            energies['degree'] = round(float(0.5 * f_deg_norm @ L @ f_deg_norm), 4)

        # 2. Betweenness
        betweenness = nx.betweenness_centrality(subgraph)
        f_betw = np.array([betweenness[node] for node in node_list])
        if np.linalg.norm(f_betw) > 0:
            f_betw_norm = f_betw / np.linalg.norm(f_betw)
            energies['betweenness'] = round(float(0.5 * f_betw_norm @ L @ f_betw_norm), 4)

        # 3. Closeness
        try:
            closeness = nx.closeness_centrality(subgraph)
            f_close = np.array([closeness[node] for node in node_list])
            if np.linalg.norm(f_close) > 0:
                f_close_norm = f_close / np.linalg.norm(f_close)
                energies['closeness'] = round(float(0.5 * f_close_norm @ L @ f_close_norm), 4)
        except:
            pass

        # 4. PageRank
        try:
            pagerank = nx.pagerank(subgraph)
            f_pr = np.array([pagerank[node] for node in node_list])
            if np.linalg.norm(f_pr) > 0:
                f_pr_norm = f_pr / np.linalg.norm(f_pr)
                energies['pagerank'] = round(float(0.5 * f_pr_norm @ L @ f_pr_norm), 4)
        except:
            pass

        avg_energy = np.mean(list(energies.values())) if energies else 0

        # Минимальная возможная энергия (λ₂/2 для единичной нормы)
        eigenvalues = np.sort(np.linalg.eigvalsh(L))
        min_possible_energy = eigenvalues[1] / 2 if len(eigenvalues) > 1 else 0

        status = 'PASSED' if avg_energy <= threshold else 'WARNING'

        return {
            'name': 'dirichlet_energy',
            'description': f'Энергия Дирихле <= {threshold}',
            'status': status,
            'energies_by_function': energies,
            'average_energy': round(avg_energy, 4),
            'min_possible_energy': round(min_possible_energy, 4),
            'threshold': threshold,
            'interpretation': 'Низкая энергия = метрики плавно меняются вдоль зависимостей'
        }
    except Exception as e:
        return {'name': 'dirichlet_energy', 'status': 'ERROR', 'error': str(e)}


def validate_geodesic_distance(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Geodesic Distance Analysis - геодезические расстояния на графе.

    Геодезическое расстояние = кратчайший путь.
    Анализируем распределение расстояний и диаметр.

    Экцентриситет(v) = max_u d(v,u)
    Радиус = min_v eccentricity(v)
    Диаметр = max_v eccentricity(v)
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        if len(nodes) < 3:
            return {
                'name': 'geodesic_distance',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        # Проверяем связность
        if not nx.is_connected(subgraph):
            # Берём наибольшую компоненту
            largest_cc = max(nx.connected_components(subgraph), key=len)
            subgraph = subgraph.subgraph(largest_cc)

        n = subgraph.number_of_nodes()
        if n < 3:
            return {
                'name': 'geodesic_distance',
                'status': 'SKIP',
                'reason': 'Largest component too small'
            }

        # Все кратчайшие пути
        distances = dict(nx.all_pairs_shortest_path_length(subgraph))

        # Собираем все расстояния
        all_distances = []
        for u in distances:
            for v, d in distances[u].items():
                if u < v:  # избегаем дублирования
                    all_distances.append(d)

        if not all_distances:
            return {
                'name': 'geodesic_distance',
                'status': 'SKIP',
                'reason': 'No distances computed'
            }

        # Статистики
        diameter = max(all_distances)
        avg_distance = np.mean(all_distances)

        # Экцентриситеты
        eccentricities = nx.eccentricity(subgraph)
        radius = min(eccentricities.values())

        # Центр графа
        center = nx.center(subgraph)

        # Периферия
        periphery = nx.periphery(subgraph)

        # Распределение расстояний
        distance_distribution = {}
        for d in all_distances:
            distance_distribution[d] = distance_distribution.get(d, 0) + 1

        # Wiener index (сумма всех расстояний)
        wiener_index = sum(all_distances)

        return {
            'name': 'geodesic_distance',
            'description': 'Геодезические расстояния на графе',
            'status': 'INFO',
            'diameter': diameter,
            'radius': radius,
            'average_distance': round(avg_distance, 4),
            'wiener_index': wiener_index,
            'center': list(center)[:5],
            'periphery': list(periphery)[:5],
            'distance_distribution': dict(sorted(distance_distribution.items())),
            'interpretation': 'Малый диаметр = компактная архитектура'
        }
    except Exception as e:
        return {'name': 'geodesic_distance', 'status': 'ERROR', 'error': str(e)}


def validate_optimal_placement(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Optimal Placement - оптимальное размещение на основе спектра.

    Использует собственные векторы Лапласиана для оптимального
    размещения узлов (минимизация суммы квадратов длин рёбер).

    Fiedler vector даёт оптимальное 1D размещение.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 4:
            return {
                'name': 'optimal_placement',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        node_list = list(subgraph.nodes())
        L = _get_laplacian_matrix(subgraph)

        # Собственные векторы
        eigenvalues, eigenvectors = np.linalg.eigh(L)

        # Fiedler vector (второй собственный вектор)
        fiedler_vector = eigenvectors[:, 1]

        # 1D размещение по Fiedler
        placement_1d = {node: round(float(fiedler_vector[i]), 4)
                        for i, node in enumerate(node_list)}

        # 2D размещение (Fiedler + третий вектор)
        if n > 3:
            third_vector = eigenvectors[:, 2]
            placement_2d = {node: (round(float(fiedler_vector[i]), 4),
                                   round(float(third_vector[i]), 4))
                           for i, node in enumerate(node_list)}
        else:
            placement_2d = {}

        # Оценка качества размещения
        # Сумма квадратов длин рёбер в 1D
        edge_lengths_sq = []
        for u, v in subgraph.edges():
            i, j = node_list.index(u), node_list.index(v)
            length_sq = (fiedler_vector[i] - fiedler_vector[j]) ** 2
            edge_lengths_sq.append(length_sq)

        total_length = sum(edge_lengths_sq)

        # Кластеризация по Fiedler (знак)
        positive_cluster = [node for node, val in placement_1d.items() if val > 0]
        negative_cluster = [node for node, val in placement_1d.items() if val <= 0]

        # Рёбра между кластерами (cut)
        cut_edges = []
        for u, v in subgraph.edges():
            if (placement_1d[u] > 0) != (placement_1d[v] > 0):
                cut_edges.append((u, v))

        return {
            'name': 'optimal_placement',
            'description': 'Оптимальное спектральное размещение',
            'status': 'INFO',
            'fiedler_value': round(float(eigenvalues[1]), 4),
            'total_edge_length_sq': round(total_length, 4),
            'cluster_sizes': {
                'positive': len(positive_cluster),
                'negative': len(negative_cluster)
            },
            'cut_size': len(cut_edges),
            'placement_1d_sample': dict(list(placement_1d.items())[:5]),
            'interpretation': 'Fiedler vector минимизирует суммарную длину рёбер'
        }
    except Exception as e:
        return {'name': 'optimal_placement', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# DYNAMICAL SYSTEMS
# =============================================================================

def validate_lyapunov_stability(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Lyapunov Stability Analysis - устойчивость по Ляпунову.

    Для линейной системы dx/dt = Ax:
    - Устойчива если все Re(λᵢ) < 0
    - Асимптотически устойчива если все Re(λᵢ) < 0
    - Неустойчива если ∃ Re(λᵢ) > 0

    Для графа: A = -L (Лапласиан), система всегда устойчива.
    Анализируем скорость сходимости.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 3:
            return {
                'name': 'lyapunov_stability',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        L = _get_laplacian_matrix(subgraph)
        A = _get_adjacency_matrix(subgraph)

        # Собственные значения Лапласиана
        lap_eigenvalues = np.sort(np.linalg.eigvalsh(L))

        # Собственные значения матрицы смежности (могут быть комплексными)
        adj_eigenvalues = np.linalg.eigvals(A)

        # Для системы dx/dt = -Lx, устойчивость определяется -λ
        # λ₂ > 0 гарантирует асимптотическую устойчивость к консенсусу

        lambda_2 = lap_eigenvalues[1] if len(lap_eigenvalues) > 1 else 0
        lambda_max = lap_eigenvalues[-1]

        # Показатель Ляпунова (скорость сходимости)
        lyapunov_exponent = -lambda_2 if lambda_2 > 0 else 0

        # Время сходимости
        convergence_time = 1 / lambda_2 if lambda_2 > 1e-10 else float('inf')

        # Спектральный радиус матрицы смежности
        spectral_radius = max(abs(adj_eigenvalues))

        # Определяем тип устойчивости
        if lambda_2 > 0.1:
            stability_type = 'strongly_stable'
        elif lambda_2 > 0.01:
            stability_type = 'stable'
        elif lambda_2 > 1e-10:
            stability_type = 'marginally_stable'
        else:
            stability_type = 'disconnected'

        return {
            'name': 'lyapunov_stability',
            'description': 'Устойчивость по Ляпунову',
            'status': 'INFO',
            'algebraic_connectivity': round(float(lambda_2), 4),
            'spectral_gap': round(float(lambda_max - lambda_2), 4),
            'lyapunov_exponent': round(float(lyapunov_exponent), 4),
            'convergence_time': round(float(convergence_time), 4) if convergence_time != float('inf') else 'inf',
            'spectral_radius': round(float(spectral_radius), 4),
            'stability_type': stability_type,
            'interpretation': 'λ₂ > 0 = консенсусная динамика устойчива'
        }
    except Exception as e:
        return {'name': 'lyapunov_stability', 'status': 'ERROR', 'error': str(e)}


def validate_bifurcation_analysis(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Bifurcation Analysis - анализ бифуркаций.

    Бифуркация = качественное изменение поведения системы
    при малом изменении параметра.

    Для графа: анализируем как удаление рёбер меняет
    качественные свойства (связность, число компонент).
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        e = subgraph.number_of_edges()

        if n < 3 or e < 2:
            return {
                'name': 'bifurcation_analysis',
                'status': 'SKIP',
                'reason': 'Insufficient nodes or edges'
            }

        # Текущее состояние
        current_components = nx.number_connected_components(subgraph)

        # Мосты - рёбра, удаление которых увеличивает число компонент
        bridges = list(nx.bridges(subgraph))

        # Точки сочленения - вершины, удаление которых увеличивает число компонент
        articulation_points = list(nx.articulation_points(subgraph))

        # Анализ λ₂ при удалении рёбер
        L = _get_laplacian_matrix(subgraph)
        eigenvalues = np.sort(np.linalg.eigvalsh(L))
        original_lambda2 = eigenvalues[1] if len(eigenvalues) > 1 else 0

        # Критические рёбра (близкие к бифуркации)
        critical_edges = []
        edges = list(subgraph.edges())

        for edge in edges[:min(15, len(edges))]:
            temp_graph = subgraph.copy()
            temp_graph.remove_edge(*edge)

            if nx.is_connected(temp_graph):
                L_temp = nx.laplacian_matrix(temp_graph).toarray().astype(float)
                temp_eigenvalues = np.sort(np.linalg.eigvalsh(L_temp))
                temp_lambda2 = temp_eigenvalues[1] if len(temp_eigenvalues) > 1 else 0

                # Относительное изменение λ₂
                if original_lambda2 > 1e-10:
                    relative_change = (original_lambda2 - temp_lambda2) / original_lambda2
                    critical_edges.append({
                        'edge': edge,
                        'lambda2_drop': round(relative_change, 4)
                    })

        # Сортируем по критичности
        critical_edges.sort(key=lambda x: x['lambda2_drop'], reverse=True)

        # Порог бифуркации: при каком количестве удалений граф распадается
        bifurcation_threshold = len(bridges)

        return {
            'name': 'bifurcation_analysis',
            'description': 'Анализ бифуркаций графа',
            'status': 'INFO',
            'current_components': current_components,
            'bridges_count': len(bridges),
            'bridges': [(b[0], b[1]) for b in bridges[:5]],
            'articulation_points_count': len(articulation_points),
            'articulation_points': articulation_points[:5],
            'original_lambda2': round(float(original_lambda2), 4),
            'most_critical_edges': [{'edge': (e['edge'][0], e['edge'][1]),
                                     'lambda2_drop': e['lambda2_drop']}
                                    for e in critical_edges[:5]],
            'interpretation': 'Мосты и articulation points = точки бифуркации'
        }
    except Exception as e:
        return {'name': 'bifurcation_analysis', 'status': 'ERROR', 'error': str(e)}


def validate_attractor_basin(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Attractor Basin Analysis - бассейны притяжения.

    Для случайного блуждания на графе:
    - Аттрактор = стационарное распределение π
    - Бассейн = множество узлов, сходящихся к аттрактору

    Анализируем структуру аттракторов в архитектуре.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 3:
            return {
                'name': 'attractor_basin',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        # Для ориентированного графа: сильно связные компоненты
        sccs = list(nx.strongly_connected_components(subgraph))

        # Конденсация (DAG из SCC)
        condensation = nx.condensation(subgraph)

        # Sink SCCs (аттракторы) - SCC без исходящих рёбер в конденсации
        sink_sccs = [node for node in condensation.nodes()
                    if condensation.out_degree(node) == 0]

        # Для каждого sink SCC, найти его бассейн притяжения
        basins = []
        scc_mapping = {node: i for i, scc in enumerate(sccs) for node in scc}

        for sink in sink_sccs:
            # Все узлы, из которых можно достичь этого sink
            ancestors = nx.ancestors(condensation, sink)
            ancestors.add(sink)

            # Собираем все исходные узлы
            basin_nodes = []
            for scc_idx in ancestors:
                basin_nodes.extend(list(sccs)[scc_idx])

            attractor_nodes = list(list(sccs)[sink])

            basins.append({
                'attractor': attractor_nodes[:5],
                'attractor_size': len(attractor_nodes),
                'basin_size': len(basin_nodes)
            })

        # PageRank как приближение стационарного распределения
        try:
            undirected = subgraph.to_undirected()
            pagerank = nx.pagerank(undirected)
            top_pagerank = sorted(pagerank.items(), key=lambda x: x[1], reverse=True)[:5]
            stationary_approx = [(node, round(val, 4)) for node, val in top_pagerank]
        except:
            stationary_approx = []

        return {
            'name': 'attractor_basin',
            'description': 'Бассейны притяжения',
            'status': 'INFO',
            'num_attractors': len(sink_sccs),
            'num_sccs': len(sccs),
            'basins': basins[:5],
            'stationary_distribution_top': stationary_approx,
            'interpretation': 'Sink SCCs = аттракторы, к которым сходится динамика'
        }
    except Exception as e:
        return {'name': 'attractor_basin', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# HARMONIC ANALYSIS
# =============================================================================

def validate_graph_fourier(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Graph Fourier Transform - преобразование Фурье на графе.

    GFT: f̂(λᵢ) = <f, φᵢ> = Σⱼ f(j)·φᵢ(j)

    где φᵢ - собственные векторы Лапласиана.
    Низкие частоты (малые λ) = глобальные паттерны.
    Высокие частоты = локальные вариации.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 4:
            return {
                'name': 'graph_fourier',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        node_list = list(subgraph.nodes())
        L = _get_laplacian_matrix(subgraph)

        # Собственное разложение (базис Фурье)
        eigenvalues, eigenvectors = np.linalg.eigh(L)

        # Сигнал: betweenness centrality
        betweenness = nx.betweenness_centrality(subgraph)
        f = np.array([betweenness[node] for node in node_list])

        # Graph Fourier Transform
        f_hat = eigenvectors.T @ f

        # Спектр мощности
        power_spectrum = f_hat ** 2

        # Анализ частот
        low_freq_energy = np.sum(power_spectrum[:n//3]) if n > 3 else 0
        mid_freq_energy = np.sum(power_spectrum[n//3:2*n//3]) if n > 3 else 0
        high_freq_energy = np.sum(power_spectrum[2*n//3:]) if n > 3 else 0
        total_energy = np.sum(power_spectrum)

        # Нормализованное распределение
        if total_energy > 0:
            freq_distribution = {
                'low': round(low_freq_energy / total_energy, 4),
                'mid': round(mid_freq_energy / total_energy, 4),
                'high': round(high_freq_energy / total_energy, 4)
            }
        else:
            freq_distribution = {'low': 0, 'mid': 0, 'high': 0}

        # Доминирующие моды
        dominant_modes = []
        sorted_indices = np.argsort(power_spectrum)[::-1]
        for idx in sorted_indices[:5]:
            dominant_modes.append({
                'mode': int(idx),
                'frequency': round(float(eigenvalues[idx]), 4),
                'power': round(float(power_spectrum[idx]), 6)
            })

        # Bandwidth (эффективная полоса частот)
        if total_energy > 0:
            normalized_spectrum = power_spectrum / total_energy
            entropy = -np.sum(normalized_spectrum * np.log(normalized_spectrum + 1e-10))
            effective_bandwidth = np.exp(entropy)
        else:
            effective_bandwidth = 0

        return {
            'name': 'graph_fourier',
            'description': 'Преобразование Фурье на графе',
            'status': 'INFO',
            'frequency_distribution': freq_distribution,
            'dominant_modes': dominant_modes,
            'effective_bandwidth': round(effective_bandwidth, 4),
            'total_energy': round(total_energy, 6),
            'interpretation': 'Много низких частот = гладкое распределение метрик'
        }
    except Exception as e:
        return {'name': 'graph_fourier', 'status': 'ERROR', 'error': str(e)}


def validate_wavelet_decomposition(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Wavelet Decomposition on Graph - вейвлет-разложение.

    Спектральные вейвлеты на графе:
    ψ_s(λ) = g(sλ) - масштабирующая функция

    Позволяет анализировать структуру на разных масштабах.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 4:
            return {
                'name': 'wavelet_decomposition',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        node_list = list(subgraph.nodes())
        L = _get_laplacian_matrix(subgraph)

        # Собственное разложение
        eigenvalues, eigenvectors = np.linalg.eigh(L)

        # Сигнал
        betweenness = nx.betweenness_centrality(subgraph)
        f = np.array([betweenness[node] for node in node_list])

        # Коэффициенты Фурье
        f_hat = eigenvectors.T @ f

        # Вейвлет-функция (Mexican hat / Ricker wavelet в спектральной области)
        # g(x) = x * exp(-x)
        def wavelet_kernel(x, scale):
            return x * np.exp(-scale * x)

        # Масштабы для анализа
        scales = [0.5, 1.0, 2.0, 4.0, 8.0]

        wavelet_coeffs = {}
        for scale in scales:
            # Применяем вейвлет в спектральной области
            kernel = wavelet_kernel(eigenvalues, scale)

            # Вейвлет-коэффициенты
            w_hat = kernel * f_hat

            # Обратное преобразование
            w = eigenvectors @ w_hat

            # Энергия на этом масштабе
            energy = np.sum(w ** 2)

            # Локализация (где максимум)
            max_idx = np.argmax(np.abs(w))

            wavelet_coeffs[f'scale_{scale}'] = {
                'energy': round(float(energy), 6),
                'max_node': node_list[max_idx],
                'max_value': round(float(w[max_idx]), 4)
            }

        # Доминирующий масштаб
        energies = {k: v['energy'] for k, v in wavelet_coeffs.items()}
        dominant_scale = max(energies, key=energies.get)

        return {
            'name': 'wavelet_decomposition',
            'description': 'Вейвлет-разложение на графе',
            'status': 'INFO',
            'wavelet_coefficients': wavelet_coeffs,
            'dominant_scale': dominant_scale,
            'interpretation': 'Разные масштабы выявляют структуру на разных уровнях'
        }
    except Exception as e:
        return {'name': 'wavelet_decomposition', 'status': 'ERROR', 'error': str(e)}


def validate_spectral_filtering(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Spectral Filtering - спектральная фильтрация сигналов.

    Применяем фильтры в спектральной области:
    - Low-pass: h(λ) = 1/(1 + αλ) - сглаживание
    - High-pass: h(λ) = αλ/(1 + αλ) - выделение локальных вариаций
    - Band-pass: выделение определённых частот
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 4:
            return {
                'name': 'spectral_filtering',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        node_list = list(subgraph.nodes())
        L = _get_laplacian_matrix(subgraph)

        # Собственное разложение
        eigenvalues, eigenvectors = np.linalg.eigh(L)

        # Сигнал
        betweenness = nx.betweenness_centrality(subgraph)
        f = np.array([betweenness[node] for node in node_list])
        f_hat = eigenvectors.T @ f

        # Параметр фильтра
        alpha = 1.0

        # Low-pass filter
        h_low = 1 / (1 + alpha * eigenvalues)
        f_low = eigenvectors @ (h_low * f_hat)

        # High-pass filter
        h_high = alpha * eigenvalues / (1 + alpha * eigenvalues)
        f_high = eigenvectors @ (h_high * f_hat)

        # Band-pass (средние частоты)
        lambda_center = np.median(eigenvalues[eigenvalues > 0]) if np.any(eigenvalues > 0) else 1
        sigma = lambda_center / 2
        h_band = np.exp(-((eigenvalues - lambda_center) ** 2) / (2 * sigma ** 2))
        f_band = eigenvectors @ (h_band * f_hat)

        # Анализ результатов фильтрации
        def analyze_signal(signal, name):
            return {
                'energy': round(float(np.sum(signal ** 2)), 6),
                'max_node': node_list[np.argmax(signal)],
                'min_node': node_list[np.argmin(signal)],
                'std': round(float(np.std(signal)), 4)
            }

        filtering_results = {
            'low_pass': analyze_signal(f_low, 'low'),
            'high_pass': analyze_signal(f_high, 'high'),
            'band_pass': analyze_signal(f_band, 'band')
        }

        # Соотношение энергий
        total_energy = np.sum(f ** 2)
        if total_energy > 0:
            energy_ratios = {
                'low_pass': round(np.sum(f_low ** 2) / total_energy, 4),
                'high_pass': round(np.sum(f_high ** 2) / total_energy, 4),
                'band_pass': round(np.sum(f_band ** 2) / total_energy, 4)
            }
        else:
            energy_ratios = {'low_pass': 0, 'high_pass': 0, 'band_pass': 0}

        return {
            'name': 'spectral_filtering',
            'description': 'Спектральная фильтрация',
            'status': 'INFO',
            'filtering_results': filtering_results,
            'energy_ratios': energy_ratios,
            'filter_parameter': alpha,
            'interpretation': 'Low-pass = глобальная структура, High-pass = локальные аномалии'
        }
    except Exception as e:
        return {'name': 'spectral_filtering', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# COMPLEX ANALYSIS
# =============================================================================

def validate_ihara_zeta(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Ihara Zeta Function - дзета-функция Ихары.

    ζ_G(u)⁻¹ = (1-u²)^(r-1) det(I - Au + Qu²)

    где A - матрица смежности, Q = D - I (D - степени),
    r = |E| - |V| + 1 (цикломатическое число).

    Кодирует информацию о простых циклах графа.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        e = subgraph.number_of_edges()

        if n < 3 or e < 2:
            return {
                'name': 'ihara_zeta',
                'status': 'SKIP',
                'reason': 'Insufficient nodes or edges'
            }

        A = _get_adjacency_matrix(subgraph)

        # Матрица степеней
        degrees = dict(subgraph.degree())
        node_list = list(subgraph.nodes())
        D = np.diag([degrees[node] for node in node_list])

        # Q = D - I
        Q = D - np.eye(n)

        # Цикломатическое число (первое число Бетти)
        cyclomatic = e - n + nx.number_connected_components(subgraph)

        # Вычисляем det(I - Au + Qu²) для разных u
        u_values = [0.1, 0.2, 0.3, 0.4, 0.5]
        zeta_values = []

        I = np.eye(n)

        for u in u_values:
            M = I - A * u + Q * (u ** 2)
            det_M = np.linalg.det(M)

            # ζ⁻¹ = (1-u²)^(r-1) * det(M)
            factor = (1 - u ** 2) ** (cyclomatic - 1) if cyclomatic > 1 else 1
            zeta_inv = factor * det_M

            zeta_values.append({
                'u': u,
                'det_M': round(float(det_M), 4),
                'zeta_inverse': round(float(zeta_inv), 4)
            })

        # Полюсы ζ связаны с собственными значениями
        # Находим u, где det(M) ≈ 0
        eigenvalues_A = np.linalg.eigvals(A)
        spectral_radius = max(abs(eigenvalues_A))

        # Радиус сходимости
        if spectral_radius > 0:
            convergence_radius = 1 / spectral_radius
        else:
            convergence_radius = float('inf')

        return {
            'name': 'ihara_zeta',
            'description': 'Дзета-функция Ихары',
            'status': 'INFO',
            'cyclomatic_number': cyclomatic,
            'zeta_values': zeta_values,
            'spectral_radius': round(float(spectral_radius), 4),
            'convergence_radius': round(float(convergence_radius), 4) if convergence_radius != float('inf') else 'inf',
            'interpretation': 'Дзета-функция кодирует информацию о циклах'
        }
    except Exception as e:
        return {'name': 'ihara_zeta', 'status': 'ERROR', 'error': str(e)}


def validate_cycle_polynomial(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Cycle Polynomial Analysis - анализ циклической структуры.

    Характеристический полином det(λI - A) кодирует спектр.
    Коэффициенты связаны с количеством путей разной длины.

    Также анализируем распределение длин циклов.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 3:
            return {
                'name': 'cycle_polynomial',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        A = _get_adjacency_matrix(subgraph)

        # Характеристический полином через собственные значения
        eigenvalues = np.linalg.eigvals(A)

        # Коэффициенты характеристического полинома
        # det(λI - A) = λⁿ - c₁λⁿ⁻¹ + c₂λⁿ⁻² - ...
        # c₁ = tr(A) = 0 (для простого графа)
        # c₂ = (tr(A)² - tr(A²))/2 = -|E|

        tr_A = np.trace(A)  # = 0 для простого графа
        tr_A2 = np.trace(A @ A)  # = 2|E|
        tr_A3 = np.trace(A @ A @ A)  # = 6 * (число треугольников)

        num_edges = int(tr_A2 / 2)
        num_triangles = int(tr_A3 / 6)

        # Tr(Aᵏ) = число замкнутых путей длины k
        traces = {
            'k=2': int(tr_A2),  # 2 * edges (пути туда-обратно)
            'k=3': int(tr_A3),  # 6 * triangles
        }

        # Добавляем tr(A⁴) если размер позволяет
        if n <= 100:
            A4 = A @ A @ A @ A
            tr_A4 = np.trace(A4)
            traces['k=4'] = int(tr_A4)

            # Число 4-циклов (квадратов) - более сложная формула
            # tr(A⁴) = 2|E| + 4*(число путей длины 2) + 8*(число 4-циклов)

        # Обхват (длина кратчайшего цикла)
        try:
            girth = nx.girth(subgraph)
        except:
            girth = None

        # Найдём несколько коротких циклов
        try:
            cycle_basis = nx.cycle_basis(subgraph)
            cycle_lengths = sorted([len(c) for c in cycle_basis])[:10]
        except:
            cycle_lengths = []

        return {
            'name': 'cycle_polynomial',
            'description': 'Циклический полином и структура',
            'status': 'INFO',
            'num_edges': num_edges,
            'num_triangles': num_triangles,
            'closed_walks': traces,
            'girth': girth,
            'cycle_basis_lengths': cycle_lengths,
            'spectral_moments': {
                'mean': round(float(np.mean(eigenvalues)), 4),
                'variance': round(float(np.var(eigenvalues)), 4)
            },
            'interpretation': 'Tr(Aᵏ) = число замкнутых путей длины k'
        }
    except Exception as e:
        return {'name': 'cycle_polynomial', 'status': 'ERROR', 'error': str(e)}
