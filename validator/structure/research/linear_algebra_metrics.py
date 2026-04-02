"""
Advanced Linear Algebra & Dynamical Systems Validations

Based on:
- Matrix analysis (SVD, condition number, rank)
- Spectral theory extensions
- Dynamical systems (stability, Lyapunov)
- Differential geometry on graphs
"""

import networkx as nx
import numpy as np
from typing import Dict, List, Any, Optional, TYPE_CHECKING
from scipy import linalg
import warnings

if TYPE_CHECKING:
    from validator.config import RuleConfig

warnings.filterwarnings('ignore', category=RuntimeWarning)


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


def validate_matrix_rank(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Matrix Rank - независимость компонентов.

    Ранг матрицы смежности показывает количество независимых "направлений" зависимостей.
    Низкий ранг = много линейно зависимых компонентов.
    """
    min_rank_ratio = 0.3
    if config and config.threshold is not None:
        min_rank_ratio = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 2:
            return {
                'name': 'matrix_rank',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов'
            }

        # Матрица смежности
        adj_matrix = nx.adjacency_matrix(subgraph).todense()

        # Вычисляем ранг
        rank = int(np.linalg.matrix_rank(adj_matrix))
        rank_ratio = rank / n

        if rank_ratio < min_rank_ratio:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'matrix_rank',
            'description': f'Отношение ранга >= {min_rank_ratio:.0%}',
            'status': status,
            'rank': rank,
            'matrix_size': n,
            'rank_ratio': round(rank_ratio, 3),
            'threshold': min_rank_ratio,
            'interpretation': 'Низкий ранг = много зависимых компонентов'
        }
    except Exception as e:
        return {'name': 'matrix_rank', 'status': 'ERROR', 'error': str(e)}


def validate_condition_number(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Condition Number - чувствительность к изменениям.

    κ(A) = ||A|| · ||A⁻¹||

    Высокое число обусловленности = система чувствительна к малым изменениям.
    """
    max_condition = 100
    if config and config.threshold is not None:
        max_condition = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 2:
            return {
                'name': 'condition_number',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов'
            }

        # Используем Laplacian matrix (более стабильна)
        laplacian = nx.laplacian_matrix(subgraph.to_undirected()).todense().astype(float)

        # Добавляем малое возмущение для регуляризации
        laplacian += np.eye(n) * 1e-10

        try:
            cond = float(np.linalg.cond(laplacian))
        except Exception:
            cond = float('inf')

        if cond == float('inf'):
            status = 'INFO'
            cond_display = 'infinity'
        elif cond > max_condition:
            status = _get_violation_status(error_on_violation)
            cond_display = round(cond, 2)
        else:
            status = 'PASSED'
            cond_display = round(cond, 2)

        return {
            'name': 'condition_number',
            'description': f'Число обусловленности <= {max_condition}',
            'status': status,
            'condition_number': cond_display,
            'threshold': max_condition,
            'interpretation': 'Высокое κ = чувствительность к изменениям'
        }
    except Exception as e:
        return {'name': 'condition_number', 'status': 'ERROR', 'error': str(e)}


def validate_svd_analysis(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    SVD Analysis - Singular Value Decomposition.

    A = UΣVᵀ

    Анализ сингулярных значений показывает "размерность" структуры зависимостей.
    """
    effective_dim_ratio = 0.5
    if config and config.threshold is not None:
        effective_dim_ratio = config.threshold
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 2:
            return {
                'name': 'svd_analysis',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов'
            }

        adj_matrix = nx.adjacency_matrix(subgraph).todense().astype(float)

        # SVD
        U, s, Vt = np.linalg.svd(adj_matrix)

        # Эффективная размерность (количество значимых сингулярных значений)
        total_energy = np.sum(s ** 2)
        if total_energy > 0:
            cumulative_energy = np.cumsum(s ** 2) / total_energy
            effective_dim = int(np.searchsorted(cumulative_energy, 0.9) + 1)
        else:
            effective_dim = 0

        effective_ratio = effective_dim / n if n > 0 else 0

        # Топ сингулярные значения
        top_singular = [round(float(v), 3) for v in s[:10]]

        return {
            'name': 'svd_analysis',
            'description': 'Анализ сингулярного разложения (SVD)',
            'status': 'INFO',
            'effective_dimension': effective_dim,
            'matrix_size': n,
            'effective_ratio': round(effective_ratio, 3),
            'top_singular_values': top_singular,
            'interpretation': 'Низкая эффективная размерность = компактная структура'
        }
    except Exception as e:
        return {'name': 'svd_analysis', 'status': 'ERROR', 'error': str(e)}


def validate_spectral_gap(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Spectral Gap - разница между первыми двумя собственными значениями.

    Большой spectral gap = хорошо разделённые состояния.
    """
    min_gap = 0.1
    if config and config.threshold is not None:
        min_gap = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 3:
            return {
                'name': 'spectral_gap',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов'
            }

        laplacian = nx.laplacian_matrix(subgraph).todense().astype(float)
        eigenvalues = sorted(np.linalg.eigvalsh(laplacian))

        # Spectral gap = λ₂ - λ₁ (λ₁ = 0 для связного графа)
        if len(eigenvalues) >= 2:
            spectral_gap = float(eigenvalues[1] - eigenvalues[0])
        else:
            spectral_gap = 0

        if spectral_gap < min_gap:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'spectral_gap',
            'description': f'Спектральный зазор >= {min_gap}',
            'status': status,
            'spectral_gap': round(spectral_gap, 4),
            'threshold': min_gap,
            'first_eigenvalues': [round(float(e), 4) for e in eigenvalues[:5]],
            'interpretation': 'Большой зазор = хорошо разделённые кластеры'
        }
    except Exception as e:
        return {'name': 'spectral_gap', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# DYNAMICAL SYSTEMS
# =============================================================================

def validate_lyapunov_stability(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Lyapunov Stability - устойчивость системы.

    Система устойчива если все собственные значения имеют отрицательную вещественную часть.
    """
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 2:
            return {
                'name': 'lyapunov_stability',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов'
            }

        # Используем -L (отрицательный Лапласиан) как систему
        laplacian = nx.laplacian_matrix(subgraph.to_undirected()).todense().astype(float)
        system_matrix = -laplacian

        eigenvalues = np.linalg.eigvals(system_matrix)
        real_parts = np.real(eigenvalues)

        # Проверяем устойчивость
        max_real = float(np.max(real_parts))
        unstable_count = int(np.sum(real_parts > 1e-10))

        if unstable_count > 0:
            status = _get_violation_status(error_on_violation)
            stable = False
        else:
            status = 'PASSED'
            stable = True

        return {
            'name': 'lyapunov_stability',
            'description': 'Устойчивость по Ляпунову',
            'status': status,
            'is_stable': stable,
            'max_real_eigenvalue': round(max_real, 4),
            'unstable_modes': unstable_count,
            'interpretation': 'Устойчивая система = изменения затухают'
        }
    except Exception as e:
        return {'name': 'lyapunov_stability', 'status': 'ERROR', 'error': str(e)}


def validate_controllability(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Controllability - возможность управления системой.

    Система управляема если ранг матрицы управляемости = n.
    """
    min_ratio = 0.5
    if config and config.threshold is not None:
        min_ratio = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 2:
            return {
                'name': 'controllability',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов'
            }

        adj_matrix = nx.adjacency_matrix(subgraph).todense().astype(float)

        # Матрица управления B = I (можем воздействовать на все узлы)
        B = np.eye(n)

        # Матрица управляемости [B, AB, A²B, ..., Aⁿ⁻¹B]
        controllability_matrix = B.copy()
        A_power = adj_matrix.copy()

        for _ in range(min(n - 1, 10)):  # Ограничиваем для производительности
            controllability_matrix = np.hstack([controllability_matrix, A_power @ B])
            A_power = A_power @ adj_matrix

        rank = int(np.linalg.matrix_rank(controllability_matrix))
        controllability_ratio = rank / n

        if controllability_ratio < min_ratio:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'controllability',
            'description': f'Управляемость >= {min_ratio:.0%}',
            'status': status,
            'controllability_rank': rank,
            'system_size': n,
            'controllability_ratio': round(controllability_ratio, 3),
            'threshold': min_ratio,
            'is_fully_controllable': rank == n
        }
    except Exception as e:
        return {'name': 'controllability', 'status': 'ERROR', 'error': str(e)}


def validate_graph_curvature(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Graph Curvature (Ollivier-Ricci) - локальная геометрия связей.

    Положительная кривизна = кластеризация.
    Отрицательная кривизна = древовидная структура.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 3:
            return {
                'name': 'graph_curvature',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов'
            }

        # Приближение кривизны через локальный clustering coefficient
        clustering = nx.clustering(subgraph)

        curvatures = []
        for node in nodes:
            neighbors = list(subgraph.neighbors(node))
            if len(neighbors) < 2:
                continue

            # Приближённая кривизна
            local_clustering = clustering.get(node, 0)
            degree = subgraph.degree(node)

            # Кривизна ~ 2 * clustering - 1
            curvature = 2 * local_clustering - 1
            curvatures.append({
                'node': node,
                'curvature': round(curvature, 3),
                'degree': degree
            })

        if not curvatures:
            return {
                'name': 'graph_curvature',
                'status': 'SKIP',
                'reason': 'Недостаточно данных'
            }

        avg_curvature = float(np.mean([c['curvature'] for c in curvatures]))
        curvatures.sort(key=lambda x: x['curvature'])

        # Отрицательная кривизна (древовидные узлы)
        negative = [c for c in curvatures if c['curvature'] < -0.3]

        # Положительная кривизна (кластерные узлы)
        positive = [c for c in curvatures if c['curvature'] > 0.3]

        return {
            'name': 'graph_curvature',
            'description': 'Кривизна графа (Ollivier-Ricci)',
            'status': 'INFO',
            'avg_curvature': round(avg_curvature, 3),
            'negative_curvature_nodes': len(negative),
            'positive_curvature_nodes': len(positive),
            'sample_negative': negative[:5],
            'sample_positive': positive[-5:],
            'interpretation': 'Положительная = кластеры, Отрицательная = деревья'
        }
    except Exception as e:
        return {'name': 'graph_curvature', 'status': 'ERROR', 'error': str(e)}


def validate_effective_resistance(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Effective Resistance - "электрическое" сопротивление между узлами.

    Показывает связность с точки зрения теории электрических цепей.
    """
    max_resistance = 10.0
    if config and config.threshold is not None:
        max_resistance = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        n = len(nodes)
        if n < 2:
            return {
                'name': 'effective_resistance',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов'
            }

        if not nx.is_connected(subgraph):
            return {
                'name': 'effective_resistance',
                'status': 'INFO',
                'total_resistance': 'infinity',
                'reason': 'Граф не связан'
            }

        # Laplacian pseudoinverse для вычисления сопротивления
        laplacian = nx.laplacian_matrix(subgraph).todense().astype(float)

        try:
            L_pinv = np.linalg.pinv(laplacian)

            # Суммарное эффективное сопротивление
            # R_total = n * trace(L⁺)
            total_resistance = float(n * np.trace(L_pinv))

            # Среднее сопротивление
            avg_resistance = total_resistance / (n * (n - 1) / 2)
        except Exception:
            total_resistance = float('inf')
            avg_resistance = float('inf')

        if avg_resistance != float('inf') and avg_resistance > max_resistance:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'effective_resistance',
            'description': f'Среднее сопротивление <= {max_resistance}',
            'status': status,
            'total_resistance': round(total_resistance, 3) if total_resistance != float('inf') else 'infinity',
            'avg_resistance': round(avg_resistance, 3) if avg_resistance != float('inf') else 'infinity',
            'threshold': max_resistance,
            'interpretation': 'Низкое сопротивление = хорошая связность'
        }
    except Exception as e:
        return {'name': 'effective_resistance', 'status': 'ERROR', 'error': str(e)}
