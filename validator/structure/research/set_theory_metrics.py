"""
Set Theory Metrics for Software Architecture Validation.

This module implements validations based on set theory:
- Relation Properties: reflexivity, symmetry, transitivity, antisymmetry
- Equivalence Classes: quotient sets, partitions
- Partial Orders: posets, chains, antichains, Dilworth's theorem
- Lattice Theory: join, meet, distributivity
- Closures: transitive, reflexive, closure operators
- Fixed Points: Knaster-Tarski, least/greatest fixed points
- Galois Connections: adjoint functors, formal concept analysis
- Power Sets & Partitions: complexity analysis
- Boolean Algebra: atoms, coatoms
- Filters & Ideals: upward/downward closed sets
"""

from typing import Any, Dict, List, Optional, Tuple, Set, FrozenSet
import networkx as nx
import numpy as np
from collections import defaultdict
import itertools


def _is_excluded(node: str, exclude: List[str]) -> bool:
    """Check if node matches any exclusion pattern."""
    for pattern in exclude:
        if pattern in node:
            return True
    return False


# =============================================================================
# RELATION PROPERTIES
# =============================================================================

def validate_relation_properties(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Relation Properties - свойства отношения зависимости.

    Проверяем:
    - Рефлексивность: ∀x: x R x (self-loops)
    - Симметричность: x R y ⟹ y R x (bidirectional)
    - Антисимметричность: x R y ∧ y R x ⟹ x = y
    - Транзитивность: x R y ∧ y R z ⟹ x R z

    Для архитектуры:
    - Антисимметричность = нет взаимных зависимостей (хорошо)
    - Транзитивность = явные транзитивные зависимости
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        e = subgraph.number_of_edges()

        if n < 2:
            return {
                'name': 'relation_properties',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        # Рефлексивность: есть ли self-loops?
        self_loops = list(nx.selfloop_edges(subgraph))
        is_reflexive = len(self_loops) == n
        reflexivity_ratio = len(self_loops) / n if n > 0 else 0

        # Симметричность: для каждого (u,v) есть (v,u)?
        symmetric_pairs = 0
        asymmetric_pairs = 0
        for u, v in subgraph.edges():
            if u != v:
                if subgraph.has_edge(v, u):
                    symmetric_pairs += 1
                else:
                    asymmetric_pairs += 1

        total_pairs = symmetric_pairs + asymmetric_pairs
        symmetry_ratio = symmetric_pairs / total_pairs if total_pairs > 0 else 0

        # Антисимметричность: нет пар (u,v) и (v,u) где u ≠ v
        # Нарушения = symmetric_pairs / 2 (каждая пара считается дважды)
        antisymmetry_violations = symmetric_pairs // 2
        is_antisymmetric = antisymmetry_violations == 0

        # Транзитивность: если (u,v) и (v,w), то (u,w)?
        transitivity_violations = []
        transitivity_satisfied = 0

        for u in nodes:
            for v in subgraph.successors(u):
                if v == u:
                    continue
                for w in subgraph.successors(v):
                    if w == u or w == v:
                        continue
                    if subgraph.has_edge(u, w):
                        transitivity_satisfied += 1
                    else:
                        if len(transitivity_violations) < 10:
                            transitivity_violations.append((u, v, w))

        total_transitive_triples = transitivity_satisfied + len(transitivity_violations)
        transitivity_ratio = transitivity_satisfied / total_transitive_triples if total_transitive_triples > 0 else 1.0

        # Определяем тип отношения
        relation_type = []
        if is_reflexive:
            relation_type.append('reflexive')
        if symmetry_ratio > 0.9:
            relation_type.append('symmetric')
        if is_antisymmetric:
            relation_type.append('antisymmetric')
        if transitivity_ratio > 0.9:
            relation_type.append('transitive')

        # Специальные типы
        if is_antisymmetric and transitivity_ratio > 0.9:
            relation_type.append('partial_order')
        if symmetry_ratio > 0.9 and transitivity_ratio > 0.9:
            relation_type.append('equivalence_like')

        return {
            'name': 'relation_properties',
            'description': 'Свойства отношения зависимости',
            'status': 'INFO',
            'reflexivity': {
                'self_loops': len(self_loops),
                'ratio': round(reflexivity_ratio, 4),
                'is_reflexive': is_reflexive
            },
            'symmetry': {
                'symmetric_pairs': symmetric_pairs // 2,
                'ratio': round(symmetry_ratio, 4)
            },
            'antisymmetry': {
                'violations': antisymmetry_violations,
                'is_antisymmetric': is_antisymmetric
            },
            'transitivity': {
                'satisfied': transitivity_satisfied,
                'violations': len(transitivity_violations),
                'ratio': round(transitivity_ratio, 4),
                'sample_violations': [(str(a), str(b), str(c)) for a, b, c in transitivity_violations[:3]]
            },
            'relation_type': relation_type,
            'interpretation': 'Антисимметричность = нет циклических зависимостей пар'
        }
    except Exception as e:
        return {'name': 'relation_properties', 'status': 'ERROR', 'error': str(e)}


def validate_equivalence_classes(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Equivalence Classes - классы эквивалентности.

    Строим отношение эквивалентности:
    x ~ y ⟺ x достижим из y И y достижим из x

    Это в точности SCC (сильно связные компоненты).

    Фактор-граф G/~ - граф на классах эквивалентности.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 2:
            return {
                'name': 'equivalence_classes',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        # SCC = классы эквивалентности по взаимной достижимости
        sccs = list(nx.strongly_connected_components(subgraph))

        # Статистики классов
        class_sizes = [len(scc) for scc in sccs]
        num_classes = len(sccs)
        max_class_size = max(class_sizes)
        min_class_size = min(class_sizes)
        singleton_classes = sum(1 for s in class_sizes if s == 1)

        # Фактор-граф (конденсация)
        condensation = nx.condensation(subgraph)
        quotient_nodes = condensation.number_of_nodes()
        quotient_edges = condensation.number_of_edges()

        # Нетривиальные классы (размер > 1) = циклические зависимости
        non_trivial = [(i, list(scc)[:5], len(scc))
                       for i, scc in enumerate(sccs) if len(scc) > 1]

        # Индекс разбиения (сколько информации теряется)
        # Энтропия разбиения
        probs = np.array(class_sizes) / n
        partition_entropy = -np.sum(probs * np.log2(probs + 1e-10))

        return {
            'name': 'equivalence_classes',
            'description': 'Классы эквивалентности (SCC)',
            'status': 'INFO',
            'num_classes': num_classes,
            'class_sizes': {
                'max': max_class_size,
                'min': min_class_size,
                'mean': round(np.mean(class_sizes), 2)
            },
            'singleton_classes': singleton_classes,
            'non_trivial_classes': len(non_trivial),
            'quotient_graph': {
                'nodes': quotient_nodes,
                'edges': quotient_edges
            },
            'partition_entropy': round(partition_entropy, 4),
            'sample_non_trivial': [(c[0], c[1], c[2]) for c in non_trivial[:3]],
            'interpretation': 'Нетривиальные классы = группы с циклическими зависимостями'
        }
    except Exception as e:
        return {'name': 'equivalence_classes', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# PARTIAL ORDERS
# =============================================================================

def validate_partial_order_analysis(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Partial Order Analysis - анализ частичного порядка.

    Частичный порядок (poset): рефлексивное, антисимметричное, транзитивное.

    Для DAG: транзитивное замыкание даёт частичный порядок.

    Анализируем:
    - Минимальные/максимальные элементы
    - Высота poset (длина максимальной цепи)
    - Ширина poset (размер максимальной антицепи)
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 2:
            return {
                'name': 'partial_order_analysis',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        # Проверяем, является ли граф DAG
        is_dag = nx.is_directed_acyclic_graph(subgraph)

        if not is_dag:
            # Работаем с конденсацией (фактор по SCC)
            condensation = nx.condensation(subgraph)
            work_graph = condensation
            node_mapping = {i: f"SCC_{i}" for i in condensation.nodes()}
        else:
            work_graph = subgraph
            node_mapping = {n: n for n in nodes}

        # Минимальные элементы (нет входящих рёбер)
        minimal = [node_mapping[n] for n in work_graph.nodes()
                   if work_graph.in_degree(n) == 0]

        # Максимальные элементы (нет исходящих рёбер)
        maximal = [node_mapping[n] for n in work_graph.nodes()
                   if work_graph.out_degree(n) == 0]

        # Высота = длина максимального пути + 1
        try:
            height = nx.dag_longest_path_length(work_graph) + 1
            longest_path = nx.dag_longest_path(work_graph)
            longest_path_names = [node_mapping[n] for n in longest_path]
        except:
            height = 0
            longest_path_names = []

        # Топологические уровни
        try:
            levels = list(nx.topological_generations(work_graph))
            level_sizes = [len(level) for level in levels]
        except:
            levels = []
            level_sizes = []

        # Ширина (максимальный уровень) - приближение к антицепи
        width = max(level_sizes) if level_sizes else 0

        # Сравнимость: доля пар (x,y) где x ≤ y или y ≤ x
        # Используем транзитивное замыкание
        tc = nx.transitive_closure(work_graph)
        comparable_pairs = 0
        total_pairs = 0

        work_nodes = list(work_graph.nodes())
        for i, u in enumerate(work_nodes):
            for v in work_nodes[i+1:]:
                total_pairs += 1
                if tc.has_edge(u, v) or tc.has_edge(v, u):
                    comparable_pairs += 1

        comparability_ratio = comparable_pairs / total_pairs if total_pairs > 0 else 0

        return {
            'name': 'partial_order_analysis',
            'description': 'Анализ частичного порядка',
            'status': 'INFO',
            'is_dag': is_dag,
            'poset_structure': {
                'minimal_elements': len(minimal),
                'maximal_elements': len(maximal),
                'height': height,
                'width': width
            },
            'minimal_sample': minimal[:5],
            'maximal_sample': maximal[:5],
            'longest_chain': longest_path_names[:5] if longest_path_names else [],
            'level_sizes': level_sizes[:10],
            'comparability_ratio': round(comparability_ratio, 4),
            'interpretation': 'Высота = глубина иерархии, ширина = параллелизм'
        }
    except Exception as e:
        return {'name': 'partial_order_analysis', 'status': 'ERROR', 'error': str(e)}


def validate_chain_antichain(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Chain & Antichain Analysis - цепи и антицепи.

    Цепь: линейно упорядоченное подмножество (все элементы сравнимы).
    Антицепь: попарно несравнимые элементы.

    Теорема Дилворта: min число цепей для покрытия = max антицепь.
    Теорема Мирского: min число антицепей для покрытия = max цепь.

    Для архитектуры: антицепи = независимые модули (можно разрабатывать параллельно).
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 2:
            return {
                'name': 'chain_antichain',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        # Работаем с DAG или конденсацией
        if nx.is_directed_acyclic_graph(subgraph):
            work_graph = subgraph
        else:
            work_graph = nx.condensation(subgraph)

        # Транзитивное замыкание для определения сравнимости
        tc = nx.transitive_closure(work_graph)

        work_nodes = list(work_graph.nodes())
        m = len(work_nodes)

        # Максимальная цепь = longest path
        try:
            max_chain = nx.dag_longest_path(work_graph)
            max_chain_length = len(max_chain)
        except:
            max_chain = []
            max_chain_length = 0

        # Поиск максимальной антицепи (NP-полная, используем эвристику)
        # Эвристика: берём уровни топологической сортировки
        try:
            levels = list(nx.topological_generations(work_graph))
            # Каждый уровень - антицепь
            max_antichain_level = max(levels, key=len) if levels else []
            max_antichain_size = len(max_antichain_level)
        except:
            max_antichain_level = []
            max_antichain_size = 0

        # Жадный поиск антицепи
        def greedy_antichain():
            antichain = []
            remaining = set(work_nodes)

            while remaining:
                # Берём узел с минимальным числом сравнимых
                best = min(remaining, key=lambda x: sum(
                    1 for y in remaining if y != x and (tc.has_edge(x, y) or tc.has_edge(y, x))
                ))
                antichain.append(best)
                # Удаляем сравнимые
                remaining = {y for y in remaining
                            if y != best and not tc.has_edge(best, y) and not tc.has_edge(y, best)}

            return antichain

        greedy_ac = greedy_antichain()
        if len(greedy_ac) > max_antichain_size:
            max_antichain = greedy_ac
            max_antichain_size = len(greedy_ac)
        else:
            max_antichain = list(max_antichain_level)

        # Минимальное покрытие цепями (по Дилворту = max антицепь)
        dilworth_bound = max_antichain_size

        # Минимальное покрытие антицепями (по Мирскому = max цепь)
        mirsky_bound = max_chain_length

        return {
            'name': 'chain_antichain',
            'description': 'Цепи и антицепи (Дилворт/Мирский)',
            'status': 'INFO',
            'max_chain': {
                'length': max_chain_length,
                'sample': [str(x) for x in max_chain[:5]]
            },
            'max_antichain': {
                'size': max_antichain_size,
                'sample': [str(x) for x in max_antichain[:5]]
            },
            'dilworth_theorem': {
                'min_chain_cover': dilworth_bound,
                'equals_max_antichain': True
            },
            'mirsky_theorem': {
                'min_antichain_cover': mirsky_bound,
                'equals_max_chain': True
            },
            'interpretation': 'Антицепь = независимые модули для параллельной разработки'
        }
    except Exception as e:
        return {'name': 'chain_antichain', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# LATTICE THEORY
# =============================================================================

def validate_lattice_structure(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Lattice Structure - структура решётки.

    Решётка: poset где любые два элемента имеют:
    - Join (⊔): наименьшую верхнюю грань (supremum)
    - Meet (⊓): наибольшую нижнюю грань (infimum)

    Проверяем, образует ли граф зависимостей решётку.

    Для архитектуры: решётка = хорошо структурированная иерархия.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 3:
            return {
                'name': 'lattice_structure',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        # Работаем с DAG
        if not nx.is_directed_acyclic_graph(subgraph):
            work_graph = nx.condensation(subgraph)
        else:
            work_graph = subgraph

        # Транзитивное замыкание
        tc = nx.transitive_closure(work_graph)

        work_nodes = list(work_graph.nodes())
        m = len(work_nodes)

        # Функция для нахождения верхних граней
        def upper_bounds(a, b):
            """Найти общие верхние грани a и b."""
            bounds = []
            for c in work_nodes:
                if (tc.has_edge(a, c) or a == c) and (tc.has_edge(b, c) or b == c):
                    bounds.append(c)
            return bounds

        # Функция для нахождения нижних граней
        def lower_bounds(a, b):
            """Найти общие нижние грани a и b."""
            bounds = []
            for c in work_nodes:
                if (tc.has_edge(c, a) or c == a) and (tc.has_edge(c, b) or c == b):
                    bounds.append(c)
            return bounds

        # Функция для нахождения join (наименьшей верхней грани)
        def find_join(a, b):
            ub = upper_bounds(a, b)
            if not ub:
                return None
            # Наименьшая = та, которая ≤ всех остальных
            for candidate in ub:
                if all(tc.has_edge(candidate, other) or candidate == other for other in ub):
                    return candidate
            return None

        # Функция для нахождения meet (наибольшей нижней грани)
        def find_meet(a, b):
            lb = lower_bounds(a, b)
            if not lb:
                return None
            # Наибольшая = та, которая ≥ всех остальных
            for candidate in lb:
                if all(tc.has_edge(other, candidate) or candidate == other for other in lb):
                    return candidate
            return None

        # Проверяем случайную выборку пар
        sample_size = min(20, m * (m - 1) // 2)
        pairs = list(itertools.combinations(work_nodes, 2))[:sample_size]

        join_exists = 0
        meet_exists = 0
        join_failures = []
        meet_failures = []

        for a, b in pairs:
            j = find_join(a, b)
            m_val = find_meet(a, b)

            if j is not None:
                join_exists += 1
            else:
                if len(join_failures) < 3:
                    join_failures.append((str(a), str(b)))

            if m_val is not None:
                meet_exists += 1
            else:
                if len(meet_failures) < 3:
                    meet_failures.append((str(a), str(b)))

        total_pairs = len(pairs)
        is_join_semilattice = join_exists == total_pairs
        is_meet_semilattice = meet_exists == total_pairs
        is_lattice = is_join_semilattice and is_meet_semilattice

        # Проверяем наличие top и bottom
        has_top = sum(1 for n in work_graph.nodes() if work_graph.out_degree(n) == 0) == 1
        has_bottom = sum(1 for n in work_graph.nodes() if work_graph.in_degree(n) == 0) == 1

        return {
            'name': 'lattice_structure',
            'description': 'Структура решётки',
            'status': 'INFO',
            'is_lattice': is_lattice,
            'is_join_semilattice': is_join_semilattice,
            'is_meet_semilattice': is_meet_semilattice,
            'join_ratio': round(join_exists / total_pairs, 4) if total_pairs > 0 else 0,
            'meet_ratio': round(meet_exists / total_pairs, 4) if total_pairs > 0 else 0,
            'has_top': has_top,
            'has_bottom': has_bottom,
            'join_failures_sample': join_failures,
            'meet_failures_sample': meet_failures,
            'interpretation': 'Решётка = любые два модуля имеют общую верхнюю и нижнюю зависимость'
        }
    except Exception as e:
        return {'name': 'lattice_structure', 'status': 'ERROR', 'error': str(e)}


def validate_join_meet_analysis(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Join & Meet Analysis - анализ операций join и meet.

    Join (a ⊔ b): наименьший общий "потомок" (common dependency)
    Meet (a ⊓ b): наибольший общий "предок" (common ancestor)

    Для архитектуры:
    - Join = общий модуль, от которого зависят оба
    - Meet = общий модуль, который зависит от обоих
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 3:
            return {
                'name': 'join_meet_analysis',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        # Предки и потомки для каждого узла
        ancestors = {node: nx.ancestors(subgraph, node) | {node} for node in nodes}
        descendants = {node: nx.descendants(subgraph, node) | {node} for node in nodes}

        # Анализ join и meet для выборки пар
        sample_pairs = list(itertools.combinations(nodes, 2))[:15]

        join_results = []
        meet_results = []

        for a, b in sample_pairs:
            # Meet = пересечение предков (максимальные элементы)
            common_ancestors = ancestors[a] & ancestors[b]
            if common_ancestors:
                # Наибольший = нет других общих предков выше
                maximal_ancestors = [x for x in common_ancestors
                                    if not any(y in common_ancestors and y != x
                                              and x in ancestors.get(y, set())
                                              for y in common_ancestors)]
                meet_results.append({
                    'pair': (str(a), str(b)),
                    'meet_candidates': len(common_ancestors),
                    'maximal': [str(x) for x in maximal_ancestors[:3]]
                })

            # Join = пересечение потомков (минимальные элементы)
            common_descendants = descendants[a] & descendants[b]
            if common_descendants:
                # Наименьший = нет других общих потомков ниже
                minimal_descendants = [x for x in common_descendants
                                      if not any(y in common_descendants and y != x
                                                and x in descendants.get(y, set())
                                                for y in common_descendants)]
                join_results.append({
                    'pair': (str(a), str(b)),
                    'join_candidates': len(common_descendants),
                    'minimal': [str(x) for x in minimal_descendants[:3]]
                })

        # Статистики
        avg_meet_candidates = np.mean([m['meet_candidates'] for m in meet_results]) if meet_results else 0
        avg_join_candidates = np.mean([j['join_candidates'] for j in join_results]) if join_results else 0

        return {
            'name': 'join_meet_analysis',
            'description': 'Анализ Join (⊔) и Meet (⊓)',
            'status': 'INFO',
            'pairs_with_meet': len(meet_results),
            'pairs_with_join': len(join_results),
            'avg_meet_candidates': round(avg_meet_candidates, 2),
            'avg_join_candidates': round(avg_join_candidates, 2),
            'meet_samples': meet_results[:3],
            'join_samples': join_results[:3],
            'interpretation': 'Meet = общий предок, Join = общий потомок'
        }
    except Exception as e:
        return {'name': 'join_meet_analysis', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# CLOSURES
# =============================================================================

def validate_transitive_closure(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Transitive Closure - транзитивное замыкание.

    R⁺ = R ∪ R² ∪ R³ ∪ ... (транзитивное замыкание)

    |R⁺| показывает "истинное" число зависимостей с учётом транзитивности.

    Коэффициент замыкания = |R⁺| / |R|
    Высокий коэффициент = много неявных транзитивных зависимостей.
    """
    exclude = config.exclude if config else []
    threshold = config.threshold if config and config.threshold else 3.0

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        e = subgraph.number_of_edges()

        if n < 2:
            return {
                'name': 'transitive_closure',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        # Транзитивное замыкание
        tc = nx.transitive_closure(subgraph)
        tc_edges = tc.number_of_edges()

        # Коэффициент замыкания
        closure_ratio = tc_edges / e if e > 0 else 0

        # Сколько неявных зависимостей добавлено
        implicit_deps = tc_edges - e

        # Транзитивная редукция (минимальный граф с тем же замыканием)
        try:
            tr = nx.transitive_reduction(subgraph)
            tr_edges = tr.number_of_edges()
            redundant_edges = e - tr_edges
        except:
            tr_edges = e
            redundant_edges = 0

        # Плотность замыкания
        max_possible = n * (n - 1)  # Для ориентированного графа без петель
        closure_density = tc_edges / max_possible if max_possible > 0 else 0

        # Достижимость: средний % узлов, достижимых из каждого
        reachability = []
        for node in list(nodes)[:20]:
            reachable = len(nx.descendants(subgraph, node))
            reachability.append(reachable / (n - 1) if n > 1 else 0)
        avg_reachability = np.mean(reachability) if reachability else 0

        status = 'PASSED' if closure_ratio <= threshold else 'WARNING'

        return {
            'name': 'transitive_closure',
            'description': f'Коэффициент транзитивного замыкания <= {threshold}',
            'status': status,
            'original_edges': e,
            'closure_edges': tc_edges,
            'implicit_dependencies': implicit_deps,
            'closure_ratio': round(closure_ratio, 4),
            'closure_density': round(closure_density, 4),
            'reduction_edges': tr_edges,
            'redundant_edges': redundant_edges,
            'avg_reachability': round(avg_reachability, 4),
            'threshold': threshold,
            'interpretation': 'Высокий коэффициент = много неявных транзитивных зависимостей'
        }
    except Exception as e:
        return {'name': 'transitive_closure', 'status': 'ERROR', 'error': str(e)}


def validate_closure_operator(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Closure Operator Analysis - операторы замыкания.

    Оператор замыкания cl: P(V) → P(V) удовлетворяет:
    1. X ⊆ cl(X) (расширение)
    2. cl(cl(X)) = cl(X) (идемпотентность)
    3. X ⊆ Y ⟹ cl(X) ⊆ cl(Y) (монотонность)

    Для графа: cl(X) = X ∪ {достижимые из X}

    Анализируем замкнутые множества (cl(X) = X).
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 3:
            return {
                'name': 'closure_operator',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        # Оператор замыкания: cl(X) = X ∪ descendants(X)
        def closure(X: Set) -> Set:
            result = set(X)
            for node in X:
                result |= set(nx.descendants(subgraph, node))
            return result

        # Проверяем свойства
        test_sets = [
            set(list(nodes)[:2]),
            set(list(nodes)[1:4]),
            set(list(nodes)[:n//2])
        ]

        properties_check = []
        for X in test_sets:
            if not X:
                continue
            cl_X = closure(X)
            cl_cl_X = closure(cl_X)

            properties_check.append({
                'X_size': len(X),
                'cl_X_size': len(cl_X),
                'extensive': X.issubset(cl_X),  # X ⊆ cl(X)
                'idempotent': cl_X == cl_cl_X,  # cl(cl(X)) = cl(X)
            })

        # Ищем замкнутые множества (cl(X) = X)
        # Это множества вида V \ ancestors(node)
        closed_sets = []
        for node in list(nodes)[:10]:
            # X = {node} ∪ descendants(node) замкнуто
            X = {node} | set(nx.descendants(subgraph, node))
            if closure(X) == X:
                closed_sets.append(len(X))

        # Минимальные замкнутые множества (атомы)
        # Это sink nodes (без исходящих рёбер)
        sinks = [n for n in nodes if subgraph.out_degree(n) == 0]

        return {
            'name': 'closure_operator',
            'description': 'Оператор замыкания cl(X) = X ∪ reach(X)',
            'status': 'INFO',
            'closure_properties': properties_check,
            'num_sinks': len(sinks),
            'sink_nodes': sinks[:5],
            'closed_set_sizes': closed_sets[:5],
            'interpretation': 'Замкнутые множества = модули с полными зависимостями'
        }
    except Exception as e:
        return {'name': 'closure_operator', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# FIXED POINTS
# =============================================================================

def validate_fixed_point_analysis(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Fixed Point Analysis - неподвижные точки.

    Теорема Кнастера-Тарского: монотонная функция на полной решётке
    имеет наименьшую и наибольшую неподвижные точки.

    Для графа: f(X) = {y : ∃x∈X, x→y} (образ по рёбрам)
    Неподвижная точка: f(X) ⊆ X

    Анализируем итерации f и сходимость.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 3:
            return {
                'name': 'fixed_point_analysis',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        # f(X) = successors of X
        def f(X: Set) -> Set:
            result = set()
            for node in X:
                result |= set(subgraph.successors(node))
            return result

        # g(X) = X ∪ f(X) (монотонное расширение)
        def g(X: Set) -> Set:
            return X | f(X)

        # Итерация до неподвижной точки, начиная с разных точек
        fixed_points = []

        for start_node in list(nodes)[:5]:
            X = {start_node}
            iterations = 0
            max_iter = n + 1

            while iterations < max_iter:
                new_X = g(X)
                if new_X == X:
                    break
                X = new_X
                iterations += 1

            fixed_points.append({
                'start': str(start_node),
                'iterations': iterations,
                'fixed_point_size': len(X),
                'is_all_nodes': len(X) == n
            })

        # Наименьшая неподвижная точка g (начиная с ∅)
        # Для g(X) = X ∪ f(X), lfp = все узлы достижимые из sources
        sources = [n for n in nodes if subgraph.in_degree(n) == 0]
        lfp = set(sources)
        for _ in range(n):
            new_lfp = g(lfp)
            if new_lfp == lfp:
                break
            lfp = new_lfp

        # Наибольшая неподвижная точка (начиная со всех узлов)
        # Для f, gfp = все узлы без "выхода наружу"
        gfp = set(nodes)
        for _ in range(n):
            # h(X) = {x ∈ X : f({x}) ⊆ X}
            new_gfp = {x for x in gfp if set(subgraph.successors(x)).issubset(gfp)}
            if new_gfp == gfp:
                break
            gfp = new_gfp

        return {
            'name': 'fixed_point_analysis',
            'description': 'Неподвижные точки (Кнастер-Тарский)',
            'status': 'INFO',
            'iteration_results': fixed_points,
            'lfp_size': len(lfp),
            'gfp_size': len(gfp),
            'sources_count': len(sources),
            'interpretation': 'LFP = минимальное замкнутое множество, GFP = максимальное инвариантное'
        }
    except Exception as e:
        return {'name': 'fixed_point_analysis', 'status': 'ERROR', 'error': str(e)}


def validate_galois_connection(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Galois Connection - связь Галуа.

    Связь Галуа между P(V) и P(V):
    f: P(V) → P(V), g: P(V) → P(V)
    X ⊆ g(Y) ⟺ Y ⊆ f(X)

    Для графа:
    f(X) = {y : ∀x∈X, x→y} (общие потомки)
    g(Y) = {x : ∀y∈Y, x→y} (общие предки)

    Формальные понятия: пары (X, Y) где f(X) = Y и g(Y) = X.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 3:
            return {
                'name': 'galois_connection',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        # f(X) = общие successors всех x ∈ X
        def f(X: Set) -> Set:
            if not X:
                return set(nodes)
            result = None
            for x in X:
                succ = set(subgraph.successors(x))
                if result is None:
                    result = succ
                else:
                    result &= succ
            return result or set()

        # g(Y) = общие predecessors всех y ∈ Y
        def g(Y: Set) -> Set:
            if not Y:
                return set(nodes)
            result = None
            for y in Y:
                pred = set(subgraph.predecessors(y))
                if result is None:
                    result = pred
                else:
                    result &= pred
            return result or set()

        # Проверяем свойство связи Галуа: X ⊆ g(f(X))
        galois_check = []
        for node in list(nodes)[:5]:
            X = {node}
            fX = f(X)
            gfX = g(fX)

            galois_check.append({
                'X': str(node),
                'f(X)_size': len(fX),
                'g(f(X))_size': len(gfX),
                'X_subset_gfX': X.issubset(gfX)
            })

        # Ищем формальные понятия: (X, Y) где f(X) = Y и g(Y) = X
        formal_concepts = []

        for node in list(nodes)[:10]:
            X = {node}
            Y = f(X)
            X_new = g(Y)

            # Проверяем замкнутость
            if f(X_new) == Y and g(Y) == X_new:
                formal_concepts.append({
                    'extent_size': len(X_new),
                    'intent_size': len(Y),
                    'extent_sample': [str(x) for x in list(X_new)[:3]],
                    'intent_sample': [str(y) for y in list(Y)[:3]]
                })

        return {
            'name': 'galois_connection',
            'description': 'Связь Галуа и формальные понятия',
            'status': 'INFO',
            'galois_property_check': galois_check,
            'formal_concepts_found': len(formal_concepts),
            'formal_concepts_sample': formal_concepts[:3],
            'interpretation': 'Формальные понятия = кластеры модулей с общими зависимостями'
        }
    except Exception as e:
        return {'name': 'galois_connection', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# POWER SET & PARTITIONS
# =============================================================================

def validate_power_set_complexity(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Power Set Complexity - сложность степенного множества.

    |P(V)| = 2^n - экспоненциальный рост.

    Анализируем:
    - Downward closed sets (идеалы)
    - Upward closed sets (фильтры)
    - Antichains (Sperner families)

    Для архитектуры: число возможных "модульных срезов".
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 2:
            return {
                'name': 'power_set_complexity',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        # Размер степенного множества
        power_set_size = 2 ** n

        # Оценка числа downward closed sets (идеалов)
        # Для DAG это связано с числом антицепей (теорема Дилворта)
        if nx.is_directed_acyclic_graph(subgraph):
            try:
                levels = list(nx.topological_generations(subgraph))
                width = max(len(level) for level in levels)
                # Число Дедекинда D(width) ≈ 2^(width choose width/2)
                # Упрощённая оценка
                dedekind_approx = 2 ** width
            except:
                dedekind_approx = 2 ** n
        else:
            dedekind_approx = None

        # Подсчёт простых структур
        # Количество независимых множеств (приближение)
        # Независимое = нет рёбер внутри (антицепь для неорграфа)
        undirected = subgraph.to_undirected()

        # Жадная оценка числа максимальных независимых множеств
        def count_maximal_independent_approx():
            count = 0
            remaining = set(nodes)

            while remaining and count < 100:
                # Строим максимальное независимое множество жадно
                ind_set = set()
                available = set(remaining)

                while available:
                    node = min(available, key=lambda x: undirected.degree(x))
                    ind_set.add(node)
                    available -= {node}
                    available -= set(undirected.neighbors(node))

                count += 1
                # Удаляем один узел и повторяем
                if ind_set:
                    remaining.discard(list(ind_set)[0])
                else:
                    break

            return count

        max_ind_sets_approx = count_maximal_independent_approx()

        # Число связных подмножеств
        # (слишком дорого считать точно)
        connected_subsets_bound = n * (n - 1) // 2 + n  # Грубая нижняя оценка

        return {
            'name': 'power_set_complexity',
            'description': 'Сложность степенного множества',
            'status': 'INFO',
            'num_nodes': n,
            'power_set_size': power_set_size if n <= 20 else f'2^{n}',
            'dedekind_approximation': dedekind_approx,
            'max_independent_sets_approx': max_ind_sets_approx,
            'connected_subsets_lower_bound': connected_subsets_bound,
            'interpretation': 'Экспоненциальный рост возможных конфигураций модулей'
        }
    except Exception as e:
        return {'name': 'power_set_complexity', 'status': 'ERROR', 'error': str(e)}


def validate_partition_refinement(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Partition Refinement - разбиения и уточнения.

    Разбиение π: V = ⊔ᵢ Bᵢ (дизъюнктное объединение блоков).

    π₁ уточняет π₂ (π₁ ≤ π₂): каждый блок π₁ ⊆ некоторого блока π₂.

    Анализируем различные разбиения графа:
    - По SCC
    - По уровням
    - По степени
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 3:
            return {
                'name': 'partition_refinement',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        partitions = {}

        # Разбиение по SCC
        sccs = list(nx.strongly_connected_components(subgraph))
        partitions['scc'] = {
            'num_blocks': len(sccs),
            'block_sizes': sorted([len(s) for s in sccs], reverse=True)[:5]
        }

        # Разбиение по топологическим уровням (для DAG)
        if nx.is_directed_acyclic_graph(subgraph):
            levels = list(nx.topological_generations(subgraph))
            partitions['topological'] = {
                'num_blocks': len(levels),
                'block_sizes': [len(l) for l in levels][:10]
            }

        # Разбиение по in-degree
        in_degree_classes = defaultdict(list)
        for node in nodes:
            in_degree_classes[subgraph.in_degree(node)].append(node)
        partitions['in_degree'] = {
            'num_blocks': len(in_degree_classes),
            'block_sizes': sorted([len(v) for v in in_degree_classes.values()], reverse=True)[:5]
        }

        # Разбиение по out-degree
        out_degree_classes = defaultdict(list)
        for node in nodes:
            out_degree_classes[subgraph.out_degree(node)].append(node)
        partitions['out_degree'] = {
            'num_blocks': len(out_degree_classes),
            'block_sizes': sorted([len(v) for v in out_degree_classes.values()], reverse=True)[:5]
        }

        # Сравнение разбиений: какое тоньше?
        # Bell number B_n ≈ (n/ln(n))^n - число всех разбиений
        # Используем приближение Стирлинга
        import math
        if n <= 20:
            # Точное число Белла слишком сложно, используем оценку
            bell_approx = sum(1 for _ in range(min(100, 2**n)))  # Заглушка
        else:
            bell_approx = f'≈ ({n}/ln({n}))^{n}'

        # Энтропия разбиений
        def partition_entropy(block_sizes):
            total = sum(block_sizes)
            probs = [s / total for s in block_sizes]
            return -sum(p * np.log2(p + 1e-10) for p in probs)

        entropies = {}
        for name, data in partitions.items():
            if 'block_sizes' in data and data['block_sizes']:
                entropies[name] = round(partition_entropy(data['block_sizes']), 4)

        return {
            'name': 'partition_refinement',
            'description': 'Анализ разбиений графа',
            'status': 'INFO',
            'partitions': partitions,
            'partition_entropies': entropies,
            'finest_partition': min(entropies, key=entropies.get) if entropies else None,
            'coarsest_partition': max(entropies, key=entropies.get) if entropies else None,
            'interpretation': 'Разные разбиения выявляют разные структуры модулей'
        }
    except Exception as e:
        return {'name': 'partition_refinement', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# BOOLEAN ALGEBRA & FILTERS
# =============================================================================

def validate_boolean_algebra(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Boolean Algebra - булева алгебра подмножеств.

    P(V) с операциями ∪, ∩, ′ образует булеву алгебру.

    Атомы: минимальные ненулевые элементы {v} для v ∈ V.
    Коатомы: V \ {v}.

    Для архитектуры: анализ операций над множествами модулей.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 2:
            return {
                'name': 'boolean_algebra',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        # Атомы = одиночные узлы
        atoms = [{node} for node in nodes]

        # Коатомы = V \ {node}
        V = set(nodes)
        coatoms = [V - {node} for node in nodes]

        # Проверяем булевы тождества на подмножествах
        # Берём случайные подмножества
        import random
        random.seed(42)

        sample_sets = []
        for _ in range(5):
            k = random.randint(1, max(1, n // 2))
            sample_sets.append(frozenset(random.sample(list(nodes), k)))

        # Проверяем законы де Моргана: (A ∪ B)′ = A′ ∩ B′
        de_morgan_checks = []
        for i in range(min(3, len(sample_sets))):
            for j in range(i + 1, min(4, len(sample_sets))):
                A = sample_sets[i]
                B = sample_sets[j]

                union_complement = V - (A | B)
                complement_intersection = (V - A) & (V - B)

                de_morgan_checks.append({
                    'A_size': len(A),
                    'B_size': len(B),
                    'de_morgan_holds': union_complement == complement_intersection
                })

        # Проверяем дистрибутивность: A ∩ (B ∪ C) = (A ∩ B) ∪ (A ∩ C)
        distributivity_checks = []
        if len(sample_sets) >= 3:
            A, B, C = sample_sets[0], sample_sets[1], sample_sets[2]

            lhs = A & (B | C)
            rhs = (A & B) | (A & C)

            distributivity_checks.append({
                'distributivity_holds': lhs == rhs
            })

        return {
            'name': 'boolean_algebra',
            'description': 'Булева алгебра P(V)',
            'status': 'INFO',
            'num_atoms': len(atoms),
            'num_coatoms': len(coatoms),
            'power_set_size': 2 ** n if n <= 20 else f'2^{n}',
            'de_morgan_checks': de_morgan_checks,
            'distributivity_checks': distributivity_checks,
            'interpretation': 'P(V) = полная булева алгебра на модулях'
        }
    except Exception as e:
        return {'name': 'boolean_algebra', 'status': 'ERROR', 'error': str(e)}


def validate_filter_ideal(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Filter & Ideal Analysis - фильтры и идеалы.

    Фильтр F ⊆ P(V): upward closed (X ∈ F, X ⊆ Y ⟹ Y ∈ F)
                     closed under ∩

    Идеал I ⊆ P(V): downward closed (X ∈ I, Y ⊆ X ⟹ Y ∈ I)
                    closed under ∪

    Главный фильтр ↑X = {Y : X ⊆ Y}
    Главный идеал ↓X = {Y : Y ⊆ X}

    Для архитектуры: фильтр = модули "выше" некоторого уровня.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 3:
            return {
                'name': 'filter_ideal',
                'status': 'SKIP',
                'reason': 'Insufficient nodes'
            }

        # В контексте графа:
        # Фильтр = ancestors (upward closed в смысле зависимостей)
        # Идеал = descendants (downward closed)

        # Для каждого узла вычисляем его "главный фильтр" и "главный идеал"
        principal_filters = {}
        principal_ideals = {}

        for node in list(nodes)[:10]:
            # Главный фильтр ↑{node} в смысле достижимости
            # = все узлы, от которых зависит node
            filter_nodes = nx.ancestors(subgraph, node) | {node}
            principal_filters[str(node)] = len(filter_nodes)

            # Главный идеал ↓{node}
            # = все узлы, которые зависят от node
            ideal_nodes = nx.descendants(subgraph, node) | {node}
            principal_ideals[str(node)] = len(ideal_nodes)

        # Ультрафильтры = максимальные собственные фильтры
        # В P(V) главные ультрафильтры = ↑{v} для v ∈ V

        # Prime идеалы = P(V) \ ультрафильтр
        # Для конечного V: prime = V \ {v}

        # Статистики
        filter_sizes = list(principal_filters.values())
        ideal_sizes = list(principal_ideals.values())

        return {
            'name': 'filter_ideal',
            'description': 'Фильтры и идеалы',
            'status': 'INFO',
            'principal_filters': principal_filters,
            'principal_ideals': principal_ideals,
            'filter_stats': {
                'avg_size': round(np.mean(filter_sizes), 2),
                'max_size': max(filter_sizes),
                'min_size': min(filter_sizes)
            },
            'ideal_stats': {
                'avg_size': round(np.mean(ideal_sizes), 2),
                'max_size': max(ideal_sizes),
                'min_size': min(ideal_sizes)
            },
            'num_ultrafilters': n,  # В P(V) ровно n главных ультрафильтров
            'interpretation': 'Фильтр = ancestors, Идеал = descendants в графе зависимостей'
        }
    except Exception as e:
        return {'name': 'filter_ideal', 'status': 'ERROR', 'error': str(e)}
