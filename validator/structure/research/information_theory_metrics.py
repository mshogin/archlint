"""
Information Theory & Markov Chains Validations

Based on:
- Shannon Entropy extensions
- Mutual Information
- Conditional Entropy
- Markov Chain analysis
- Kolmogorov Complexity approximations
"""

import networkx as nx
import numpy as np
from typing import Dict, List, Any, Optional, TYPE_CHECKING
from collections import defaultdict, Counter
import math

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
            if len(parts) == 2 and parts[1] and node_lower.endswith(parts[1]):
                return True
        elif pattern_lower in node_lower:
            return True
    return False


def validate_mutual_information(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Mutual Information между модулями.

    I(X;Y) = H(X) + H(Y) - H(X,Y)

    Высокая MI = тесная связь между модулями.
    """
    max_mi = 0.7
    if config and config.threshold is not None:
        max_mi = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        # Группируем узлы по пакетам
        packages: Dict[str, set] = defaultdict(set)
        for node in graph.nodes():
            if _is_excluded(node, exclude):
                continue
            parts = node.split('/')
            pkg = parts[0] if parts else node
            packages[pkg].add(node)

        if len(packages) < 2:
            return {
                'name': 'mutual_information',
                'status': 'SKIP',
                'reason': 'Недостаточно пакетов для анализа'
            }

        # Вычисляем MI между парами пакетов
        high_mi_pairs = []
        pkg_list = list(packages.keys())

        for i, pkg1 in enumerate(pkg_list):
            for pkg2 in pkg_list[i + 1:]:
                nodes1 = packages[pkg1]
                nodes2 = packages[pkg2]

                # Считаем связи между пакетами
                cross_edges = 0
                for n1 in nodes1:
                    for _, target in graph.out_edges(n1):
                        if target in nodes2:
                            cross_edges += 1
                    for source, _ in graph.in_edges(n1):
                        if source in nodes2:
                            cross_edges += 1

                # Нормализуем
                max_edges = len(nodes1) * len(nodes2) * 2
                if max_edges > 0:
                    mi = cross_edges / max_edges
                    if mi > max_mi:
                        high_mi_pairs.append({
                            'package1': pkg1,
                            'package2': pkg2,
                            'mutual_information': round(mi, 3),
                            'cross_edges': cross_edges
                        })

        high_mi_pairs.sort(key=lambda x: x['mutual_information'], reverse=True)

        if high_mi_pairs:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'mutual_information',
            'description': f'Взаимная информация между модулями <= {max_mi}',
            'status': status,
            'threshold': max_mi,
            'high_mi_pairs': high_mi_pairs[:10],
            'high_mi_count': len(high_mi_pairs),
            'packages_analyzed': len(packages)
        }
    except Exception as e:
        return {'name': 'mutual_information', 'status': 'ERROR', 'error': str(e)}


def validate_conditional_entropy(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Conditional Entropy - H(X|Y).

    Неопределённость компонента при известных зависимостях.
    Низкая условная энтропия = предсказуемая архитектура.
    """
    max_entropy = 3.0
    if config and config.threshold is not None:
        max_entropy = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]

        if len(nodes) < 2:
            return {
                'name': 'conditional_entropy',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов'
            }

        # Для каждого узла вычисляем условную энтропию его зависимостей
        conditional_entropies = []

        for node in nodes:
            successors = list(graph.successors(node))
            if not successors:
                continue

            # Распределение зависимостей по типам/пакетам
            dep_types = Counter()
            for succ in successors:
                parts = succ.split('/')
                dep_type = parts[0] if parts else succ
                dep_types[dep_type] += 1

            # Вычисляем энтропию распределения зависимостей
            total = sum(dep_types.values())
            entropy = 0
            for count in dep_types.values():
                p = count / total
                if p > 0:
                    entropy -= p * math.log2(p)

            conditional_entropies.append({
                'node': node,
                'entropy': round(entropy, 3),
                'dependencies': len(successors)
            })

        if not conditional_entropies:
            return {
                'name': 'conditional_entropy',
                'status': 'SKIP',
                'reason': 'Нет зависимостей для анализа'
            }

        avg_entropy = float(np.mean([e['entropy'] for e in conditional_entropies]))
        max_node_entropy = max(e['entropy'] for e in conditional_entropies)

        violations = [e for e in conditional_entropies if e['entropy'] > max_entropy]
        violations.sort(key=lambda x: x['entropy'], reverse=True)

        if avg_entropy > max_entropy:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'conditional_entropy',
            'description': f'Условная энтропия зависимостей <= {max_entropy}',
            'status': status,
            'avg_entropy': round(avg_entropy, 3),
            'max_entropy': round(max_node_entropy, 3),
            'threshold': max_entropy,
            'high_entropy_nodes': violations[:10],
            'high_entropy_count': len(violations)
        }
    except Exception as e:
        return {'name': 'conditional_entropy', 'status': 'ERROR', 'error': str(e)}


def validate_channel_capacity(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Channel Capacity - пропускная способность интерфейсов.

    Метрика: максимальное количество информации через узел.
    """
    max_capacity = 20
    if config and config.threshold is not None:
        max_capacity = int(config.threshold)
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]

        capacities = []
        violations = []

        for node in nodes:
            in_deg = graph.in_degree(node)
            out_deg = graph.out_degree(node)

            # Capacity = throughput potential = in * out
            capacity = in_deg * out_deg

            capacities.append({'node': node, 'capacity': capacity, 'in': in_deg, 'out': out_deg})

            if capacity > max_capacity:
                violations.append({
                    'node': node,
                    'capacity': capacity,
                    'in_degree': in_deg,
                    'out_degree': out_deg
                })

        violations.sort(key=lambda x: x['capacity'], reverse=True)
        capacities.sort(key=lambda x: x['capacity'], reverse=True)

        avg_capacity = float(np.mean([c['capacity'] for c in capacities])) if capacities else 0

        if violations:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'channel_capacity',
            'description': f'Пропускная способность узлов <= {max_capacity}',
            'status': status,
            'avg_capacity': round(avg_capacity, 2),
            'max_capacity': capacities[0]['capacity'] if capacities else 0,
            'threshold': max_capacity,
            'top_capacity_nodes': capacities[:5],
            'violations': violations[:10],
            'violations_count': len(violations)
        }
    except Exception as e:
        return {'name': 'channel_capacity', 'status': 'ERROR', 'error': str(e)}


def validate_kolmogorov_complexity(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Kolmogorov Complexity Approximation.

    Приближение через сжимаемость описания графа.
    Низкая сложность = регулярная, предсказуемая структура.
    """
    max_complexity = 0.8
    if config and config.threshold is not None:
        max_complexity = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        e = subgraph.number_of_edges()

        if n == 0:
            return {
                'name': 'kolmogorov_complexity',
                'status': 'SKIP',
                'reason': 'Нет узлов'
            }

        # Приближение через энтропию структуры
        # Максимальная энтропия для n узлов с e рёбрами
        max_edges = n * (n - 1)  # directed graph

        if max_edges == 0:
            complexity = 0
        else:
            # Нормализованная сложность
            edge_density = e / max_edges

            # Entropy of edge distribution
            degree_sequence = [graph.degree(n) for n in nodes]
            total_degree = sum(degree_sequence)

            if total_degree > 0:
                probs = [d / total_degree for d in degree_sequence if d > 0]
                degree_entropy = -sum(p * math.log2(p) for p in probs if p > 0)
                max_entropy = math.log2(n) if n > 1 else 1
                normalized_entropy = degree_entropy / max_entropy if max_entropy > 0 else 0
            else:
                normalized_entropy = 0

            # Complexity combines density and entropy
            complexity = float((edge_density + normalized_entropy) / 2)

        if complexity > max_complexity:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'kolmogorov_complexity',
            'description': f'Приближённая сложность Колмогорова <= {max_complexity}',
            'status': status,
            'complexity': round(complexity, 3),
            'threshold': max_complexity,
            'nodes': n,
            'edges': e,
            'interpretation': 'Низкая сложность = регулярная структура'
        }
    except Exception as e:
        return {'name': 'kolmogorov_complexity', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# MARKOV CHAIN VALIDATIONS
# =============================================================================

def validate_markov_stationary(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Markov Stationary Distribution.

    Распределение "времени" в компонентах при случайном блуждании.
    Аналогично PageRank, но с интерпретацией Маркова.
    """
    threshold = 0.2
    if config and config.threshold is not None:
        threshold = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        if len(nodes) == 0:
            return {
                'name': 'markov_stationary',
                'status': 'SKIP',
                'reason': 'Нет узлов'
            }

        # Используем PageRank как стационарное распределение
        try:
            stationary = nx.pagerank(subgraph, alpha=0.85)
        except Exception:
            stationary = {n: 1.0 / len(nodes) for n in nodes}

        # Находим компоненты с высокой стационарной вероятностью
        high_prob = [
            {'node': n, 'probability': round(float(p), 4)}
            for n, p in stationary.items()
            if p > threshold
        ]
        high_prob.sort(key=lambda x: x['probability'], reverse=True)

        # Вычисляем энтропию распределения
        probs = list(stationary.values())
        entropy = -sum(p * math.log2(p) for p in probs if p > 0)
        max_entropy = math.log2(len(nodes)) if len(nodes) > 1 else 1
        normalized_entropy = entropy / max_entropy if max_entropy > 0 else 0

        return {
            'name': 'markov_stationary',
            'description': 'Стационарное распределение Маркова',
            'status': 'INFO',
            'high_probability_nodes': high_prob[:10],
            'high_probability_count': len(high_prob),
            'distribution_entropy': round(entropy, 3),
            'normalized_entropy': round(normalized_entropy, 3),
            'interpretation': 'Высокая вероятность = часто посещаемые компоненты'
        }
    except Exception as e:
        return {'name': 'markov_stationary', 'status': 'ERROR', 'error': str(e)}


def validate_absorption_probability(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Absorption Probability - вероятность достижения "поглощающих" состояний.

    Поглощающие состояния = узлы без исходящих рёбер (листья).
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        # Находим поглощающие состояния (листья)
        absorbing = [n for n in nodes if subgraph.out_degree(n) == 0]

        # Находим транзиентные состояния
        transient = [n for n in nodes if subgraph.out_degree(n) > 0]

        # Вычисляем средний путь до поглощающих состояний
        paths_to_absorbing = []

        for trans in transient[:50]:  # Ограничиваем для производительности
            for absorb in absorbing:
                try:
                    path_len = nx.shortest_path_length(subgraph, trans, absorb)
                    paths_to_absorbing.append(path_len)
                except nx.NetworkXNoPath:
                    continue

        avg_path = float(np.mean(paths_to_absorbing)) if paths_to_absorbing else 0

        return {
            'name': 'absorption_probability',
            'description': 'Анализ поглощающих состояний (листовых узлов)',
            'status': 'INFO',
            'absorbing_states': len(absorbing),
            'transient_states': len(transient),
            'absorbing_nodes': absorbing[:10],
            'avg_path_to_absorption': round(avg_path, 2),
            'interpretation': 'Поглощающие = конечные зависимости (библиотеки, stdlib)'
        }
    except Exception as e:
        return {'name': 'absorption_probability', 'status': 'ERROR', 'error': str(e)}


def validate_mixing_time(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Mixing Time - время до достижения стационарного распределения.

    Связано со вторым собственным значением матрицы переходов.
    """
    max_mixing = 10
    if config and config.threshold is not None:
        max_mixing = int(config.threshold)
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes).to_undirected()

        if len(nodes) < 2:
            return {
                'name': 'mixing_time',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов'
            }

        if not nx.is_connected(subgraph):
            return {
                'name': 'mixing_time',
                'status': 'INFO',
                'mixing_time': 'infinity',
                'reason': 'Граф не связан'
            }

        # Mixing time ~ 1 / spectral_gap
        # spectral_gap = 1 - λ₂ (второе собственное значение)
        try:
            algebraic_connectivity = nx.algebraic_connectivity(subgraph)
            if algebraic_connectivity > 0:
                mixing_time = int(1.0 / algebraic_connectivity)
            else:
                mixing_time = float('inf')
        except Exception:
            mixing_time = len(nodes)  # Приближение

        if mixing_time != float('inf') and mixing_time > max_mixing:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'mixing_time',
            'description': f'Время смешивания <= {max_mixing}',
            'status': status,
            'mixing_time': mixing_time if mixing_time != float('inf') else 'infinity',
            'threshold': max_mixing,
            'interpretation': 'Низкое время = быстрое распространение изменений'
        }
    except Exception as e:
        return {'name': 'mixing_time', 'status': 'ERROR', 'error': str(e)}


def validate_cross_entropy(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Cross-Entropy - отличие от идеальной архитектуры.

    H(p,q) = -Σ p(x) log q(x)

    Сравниваем с равномерным распределением зависимостей.
    """
    max_cross_entropy = 2.0
    if config and config.threshold is not None:
        max_cross_entropy = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]

        if len(nodes) == 0:
            return {
                'name': 'cross_entropy',
                'status': 'SKIP',
                'reason': 'Нет узлов'
            }

        # Фактическое распределение степеней
        out_degrees = [graph.out_degree(n) for n in nodes]
        total = sum(out_degrees)

        if total == 0:
            return {
                'name': 'cross_entropy',
                'status': 'SKIP',
                'reason': 'Нет зависимостей'
            }

        # Фактическое распределение
        p = np.array([d / total for d in out_degrees if d > 0])

        # Идеальное (равномерное) распределение
        n_nonzero = len(p)
        q = np.ones(n_nonzero) / n_nonzero

        # Cross-entropy
        cross_entropy = float(-np.sum(p * np.log2(q + 1e-10)))

        # KL-divergence для сравнения
        kl_div = float(np.sum(p * np.log2((p + 1e-10) / (q + 1e-10))))

        if cross_entropy > max_cross_entropy:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'cross_entropy',
            'description': f'Кросс-энтропия <= {max_cross_entropy}',
            'status': status,
            'cross_entropy': round(cross_entropy, 3),
            'kl_divergence': round(kl_div, 3),
            'threshold': max_cross_entropy,
            'interpretation': 'Низкая кросс-энтропия = близко к равномерному распределению'
        }
    except Exception as e:
        return {'name': 'cross_entropy', 'status': 'ERROR', 'error': str(e)}
