"""
Combinatorics Metrics for Software Architecture Validation.

Implements:
- Generating Functions: counting structures
- Inclusion-Exclusion: intersection analysis
- Stirling Numbers: partition counting
- Polya Enumeration: symmetry counting
- Ramsey Theory: inevitable structures
- Extremal Combinatorics: bounds
"""

from typing import Any, Dict, List, Optional, Set
import networkx as nx
import numpy as np
from collections import defaultdict
import itertools
import math


def _is_excluded(node: str, exclude: List[str]) -> bool:
    for pattern in exclude:
        if pattern in node:
            return True
    return False


def validate_generating_functions(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Generating Functions - производящие функции.

    G(x) = Σ aₙxⁿ где aₙ = число структур размера n.

    Для графа: считаем пути, циклы, подграфы разных размеров.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 3:
            return {'name': 'generating_functions', 'status': 'SKIP', 'reason': 'Insufficient nodes'}

        # Для больших графов (>50 узлов) используем sampling
        use_sampling = n > 50
        sample_size = min(20, n) if use_sampling else min(10, n)

        # Считаем структуры по размерам

        # Пути длины k - ограничиваем число источников и целей
        path_counts = defaultdict(int)
        sources_sample = list(nodes)[:sample_size]
        targets_sample = list(nodes)[:sample_size] if use_sampling else nodes

        for source in sources_sample:
            for target in targets_sample:
                if source != target:
                    try:
                        # Ограничиваем число путей для подсчета
                        path_iter = nx.all_simple_paths(subgraph, source, target, cutoff=4)
                        for i, path in enumerate(path_iter):
                            if i >= 100:  # Максимум 100 путей на пару
                                break
                            path_counts[len(path) - 1] += 1
                    except:
                        pass

        # Независимые множества размера k - только для малых графов или sampling
        independent_counts = defaultdict(int)
        undirected = subgraph.to_undirected()

        if n <= 30:
            # Для малых графов - полный подсчет
            max_k = min(n + 1, 5)
        elif n <= 50:
            # Для средних - только k=1,2
            max_k = 3
        else:
            # Для больших - только k=1 (тривиально) и оценка для k=2
            max_k = 3
            nodes = list(nodes)[:50]  # sampling

        for k in range(1, max_k):
            if k == 1:
                # Тривиально - все узлы
                independent_counts[k] = len(nodes)
            else:
                count = 0
                max_combinations = 10000  # Ограничение на число проверяемых комбинаций
                for i, subset in enumerate(itertools.combinations(nodes, k)):
                    if i >= max_combinations:
                        break
                    is_independent = True
                    for u, v in itertools.combinations(subset, 2):
                        if undirected.has_edge(u, v):
                            is_independent = False
                            break
                    if is_independent:
                        count += 1
                independent_counts[k] = count

        # Клики размера k - ограничиваем время поиска
        clique_counts = defaultdict(int)
        clique_limit = 1000 if n > 50 else None
        clique_count = 0

        for clique in nx.find_cliques(undirected):
            clique_counts[len(clique)] += 1
            clique_count += 1
            if clique_limit and clique_count >= clique_limit:
                break

        # "Производящая функция" как список коэффициентов
        path_gf = [path_counts.get(k, 0) for k in range(6)]
        independent_gf = [independent_counts.get(k, 0) for k in range(1, max_k)]

        result = {
            'name': 'generating_functions',
            'description': 'Производящие функции структур',
            'status': 'INFO',
            'path_counts': dict(path_counts),
            'independent_set_counts': dict(independent_counts),
            'clique_counts': dict(clique_counts),
            'path_generating_function': path_gf,
            'interpretation': 'G(x) кодирует число структур каждого размера'
        }

        if use_sampling:
            result['note'] = f'Используется sampling для графа из {n} узлов'

        return result
    except Exception as e:
        return {'name': 'generating_functions', 'status': 'ERROR', 'error': str(e)}


def validate_inclusion_exclusion(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Inclusion-Exclusion Principle - принцип включения-исключения.

    |A₁ ∪ A₂ ∪ ... ∪ Aₙ| = Σ|Aᵢ| - Σ|Aᵢ∩Aⱼ| + Σ|Aᵢ∩Aⱼ∩Aₖ| - ...

    Для архитектуры: пересечения зависимостей модулей.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 3:
            return {'name': 'inclusion_exclusion', 'status': 'SKIP', 'reason': 'Insufficient nodes'}

        # Aᵢ = зависимости узла i (successors)
        dependency_sets = {node: set(subgraph.successors(node)) for node in nodes}

        # Применяем inclusion-exclusion для подсчёта объединения
        sample_nodes = list(nodes)[:5]
        sets = [dependency_sets[node] for node in sample_nodes]

        # Вычисляем |A₁ ∪ ... ∪ Aₖ| через inclusion-exclusion
        union_size = 0
        intersection_terms = []

        for k in range(1, len(sets) + 1):
            term_sum = 0
            for combo in itertools.combinations(range(len(sets)), k):
                intersection = sets[combo[0]]
                for i in combo[1:]:
                    intersection = intersection & sets[i]
                term_sum += len(intersection)

            sign = (-1) ** (k + 1)
            union_size += sign * term_sum
            intersection_terms.append({
                'k': k,
                'term': sign * term_sum,
                'combinations': len(list(itertools.combinations(range(len(sets)), k)))
            })

        # Прямой подсчёт для проверки
        direct_union = set()
        for s in sets:
            direct_union |= s

        return {
            'name': 'inclusion_exclusion',
            'description': 'Принцип включения-исключения',
            'status': 'INFO',
            'sample_sets_count': len(sample_nodes),
            'union_via_ie': union_size,
            'union_direct': len(direct_union),
            'verification': union_size == len(direct_union),
            'inclusion_exclusion_terms': intersection_terms,
            'interpretation': 'Подсчёт пересечений зависимостей'
        }
    except Exception as e:
        return {'name': 'inclusion_exclusion', 'status': 'ERROR', 'error': str(e)}


def validate_stirling_numbers(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Stirling Numbers - числа Стирлинга.

    S(n,k) первого рода: число перестановок n элементов с k циклами.
    S(n,k) второго рода: число разбиений n элементов на k непустых блоков.

    Для архитектуры: разбиения модулей на компоненты.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 3:
            return {'name': 'stirling_numbers', 'status': 'SKIP', 'reason': 'Insufficient nodes'}

        # Числа Стирлинга второго рода (рекуррентно)
        def stirling2(n, k):
            if n == k == 0:
                return 1
            if n == 0 or k == 0:
                return 0
            if k > n:
                return 0
            # S(n,k) = k*S(n-1,k) + S(n-1,k-1)
            dp = [[0] * (n + 1) for _ in range(n + 1)]
            dp[0][0] = 1
            for i in range(1, n + 1):
                for j in range(1, i + 1):
                    dp[i][j] = j * dp[i-1][j] + dp[i-1][j-1]
            return dp[n][k]

        # Вычисляем S(n, k) для разных k
        stirling_second = {}
        for k in range(1, min(n + 1, 8)):
            stirling_second[k] = stirling2(n, k)

        # Число Белла B(n) = Σ S(n,k)
        bell_number = sum(stirling_second.values())

        # Фактическое число компонент в графе
        undirected = subgraph.to_undirected()
        actual_components = nx.number_connected_components(undirected)

        # SCC
        num_sccs = len(list(nx.strongly_connected_components(subgraph)))

        return {
            'name': 'stirling_numbers',
            'description': 'Числа Стирлинга S(n,k)',
            'status': 'INFO',
            'n': n,
            'stirling_second_kind': stirling_second,
            'bell_number': bell_number,
            'actual_weak_components': actual_components,
            'actual_strong_components': num_sccs,
            'interpretation': 'S(n,k) = способов разбить n модулей на k групп'
        }
    except Exception as e:
        return {'name': 'stirling_numbers', 'status': 'ERROR', 'error': str(e)}


def validate_polya_enumeration(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Pólya Enumeration - перечисление Пойа.

    Подсчёт неизоморфных структур с учётом симметрий.

    Для архитектуры: сколько существенно различных конфигураций?
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 3:
            return {'name': 'polya_enumeration', 'status': 'SKIP', 'reason': 'Insufficient nodes'}

        # Группа автоморфизмов графа
        # NetworkX не имеет встроенного, используем приближение

        # Симметрии по степеням
        degree_sequence = sorted([subgraph.degree(node) for node in nodes], reverse=True)

        # Число узлов с одинаковой степенью
        degree_counts = defaultdict(int)
        for node in nodes:
            degree_counts[subgraph.degree(node)] += 1

        # Оценка размера группы автоморфизмов
        # Aut(G) ≤ Π(degree_counts[d]!)
        automorphism_bound = 1
        for count in degree_counts.values():
            automorphism_bound *= math.factorial(count)

        # Cycle index (упрощённый)
        # Z(G) для тривиальной группы = x₁ⁿ
        # Для симметричной группы Sₙ: Z(Sₙ) = Σ (1/n!) Π xᵢ^{aᵢ}

        # Число неизоморфных раскрасок вершин в 2 цвета
        # По Бёрнсайду: |X/G| = (1/|G|) Σ |Fix(g)|
        # Для приближения: n узлов, ~automorphism_bound симметрий
        two_colorings = 2 ** n
        nonisomorphic_colorings_estimate = two_colorings / automorphism_bound

        return {
            'name': 'polya_enumeration',
            'description': 'Перечисление Пойа (симметрии)',
            'status': 'INFO',
            'degree_sequence': degree_sequence[:10],
            'degree_distribution': dict(degree_counts),
            'automorphism_group_bound': automorphism_bound,
            'total_2_colorings': two_colorings if n <= 20 else f'2^{n}',
            'nonisomorphic_estimate': round(nonisomorphic_colorings_estimate, 2),
            'interpretation': 'Учёт симметрий уменьшает число различных конфигураций'
        }
    except Exception as e:
        return {'name': 'polya_enumeration', 'status': 'ERROR', 'error': str(e)}


def validate_ramsey_analysis(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Ramsey Theory Analysis - теория Рамсея.

    R(r,s): минимальное n такое что любая 2-раскраска Kₙ
    содержит красную Kᵣ или синюю Kₛ.

    Для архитектуры: неизбежные структуры в больших системах.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 4:
            return {'name': 'ramsey_analysis', 'status': 'SKIP', 'reason': 'Insufficient nodes'}

        undirected = subgraph.to_undirected()

        # Известные числа Рамсея
        ramsey_numbers = {
            (3, 3): 6,
            (3, 4): 9,
            (3, 5): 14,
            (4, 4): 18,
            (3, 6): 18,
            (3, 7): 23,
            (4, 5): 25
        }

        # Проверяем какие структуры "неизбежны"
        ramsey_implications = []

        for (r, s), R_rs in ramsey_numbers.items():
            if n >= R_rs:
                # По теореме Рамсея, есть либо клика размера r, либо независимое множество размера s
                max_clique = max(len(c) for c in nx.find_cliques(undirected))
                complement_graph = nx.complement(undirected)
                cliques_in_complement = list(nx.find_cliques(complement_graph)) if len(complement_graph.nodes) > 0 else []
                max_independent = max((len(c) for c in cliques_in_complement), default=0)

                ramsey_implications.append({
                    'R(r,s)': f'R({r},{s})={R_rs}',
                    'n >= R': True,
                    'has_clique_r': max_clique >= r,
                    'has_independent_s': max_independent >= s,
                    'ramsey_satisfied': max_clique >= r or max_independent >= s
                })

        # Текущие клики и независимые множества
        cliques = list(nx.find_cliques(undirected))
        max_clique_size = max(len(c) for c in cliques) if cliques else 0

        complement = nx.complement(undirected)
        independent_sets = list(nx.find_cliques(complement))
        max_independent_size = max(len(s) for s in independent_sets) if independent_sets else 0

        return {
            'name': 'ramsey_analysis',
            'description': 'Анализ теории Рамсея',
            'status': 'INFO',
            'num_nodes': n,
            'max_clique': max_clique_size,
            'max_independent_set': max_independent_size,
            'ramsey_implications': ramsey_implications[:3],
            'interpretation': 'Рамсей: большие графы неизбежно содержат структуры'
        }
    except Exception as e:
        return {'name': 'ramsey_analysis', 'status': 'ERROR', 'error': str(e)}


def validate_extremal_bounds(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Extremal Combinatorics - экстремальные границы.

    Теорема Турана: ex(n, Kᵣ) = максимальное число рёбер без Kᵣ.
    Теорема Эрдёша-Ко-Радо: размер пересекающегося семейства.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        e = subgraph.number_of_edges()

        if n < 3:
            return {'name': 'extremal_bounds', 'status': 'SKIP', 'reason': 'Insufficient nodes'}

        undirected = subgraph.to_undirected()
        e_undirected = undirected.number_of_edges()

        # Туран: ex(n, K_r) = (1 - 1/(r-1)) * n²/2
        def turan_bound(n, r):
            if r <= 1:
                return 0
            return (1 - 1/(r-1)) * n * n / 2

        # Проверяем для разных r
        turan_analysis = []
        cliques = list(nx.find_cliques(undirected))
        max_clique = max(len(c) for c in cliques) if cliques else 0

        for r in range(3, 7):
            bound = turan_bound(n, r)
            has_clique = max_clique >= r
            turan_analysis.append({
                'r': r,
                'turan_bound': round(bound, 2),
                'actual_edges': e_undirected,
                'exceeds_bound': e_undirected > bound,
                'has_K_r': has_clique
            })

        # Плотность vs Туран
        max_edges = n * (n - 1) / 2
        density = e_undirected / max_edges if max_edges > 0 else 0

        # Эрдёш-Ко-Радо для k-подмножеств
        # Максимальное пересекающееся семейство k-подмножеств из n: C(n-1, k-1)
        k = 2
        if n > k:
            ekr_bound = math.comb(n - 1, k - 1)
        else:
            ekr_bound = 0

        return {
            'name': 'extremal_bounds',
            'description': 'Экстремальные комбинаторные границы',
            'status': 'INFO',
            'num_nodes': n,
            'num_edges': e_undirected,
            'density': round(density, 4),
            'max_clique_size': max_clique,
            'turan_analysis': turan_analysis,
            'erdos_ko_rado_bound': ekr_bound,
            'interpretation': 'Туран: плотный граф содержит клики'
        }
    except Exception as e:
        return {'name': 'extremal_bounds', 'status': 'ERROR', 'error': str(e)}


def validate_mobius_function(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Möbius Function - функция Мёбиуса на poset.

    μ(x,y) определена рекурсивно:
    μ(x,x) = 1
    Σ_{x≤z≤y} μ(x,z) = 0 для x < y

    Используется для инверсии: g(x) = Σ_{y≤x} f(y) ⟹ f(x) = Σ_{y≤x} μ(y,x)g(y)
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 3:
            return {'name': 'mobius_function', 'status': 'SKIP', 'reason': 'Insufficient nodes'}

        # Работаем с DAG или конденсацией
        if not nx.is_directed_acyclic_graph(subgraph):
            work_graph = nx.condensation(subgraph)
            nodes_work = list(work_graph.nodes())
        else:
            work_graph = subgraph
            nodes_work = list(nodes)

        # Транзитивное замыкание для порядка
        tc = nx.transitive_closure(work_graph)

        # Вычисляем μ(x,y) для некоторых пар
        node_idx = {node: i for i, node in enumerate(nodes_work)}
        m = len(nodes_work)

        # μ matrix
        mu = np.zeros((m, m))
        for i in range(m):
            mu[i, i] = 1

        # Заполняем по уровням (топологический порядок)
        try:
            topo_order = list(nx.topological_sort(work_graph))

            for i, x in enumerate(topo_order):
                for j, y in enumerate(topo_order):
                    if i < j and tc.has_edge(x, y):
                        # μ(x,y) = -Σ_{x≤z<y} μ(x,z)
                        xi, yi = node_idx[x], node_idx[y]
                        total = 0
                        for z in topo_order[i:j]:
                            if tc.has_edge(x, z) or x == z:
                                if tc.has_edge(z, y) and z != y:
                                    zi = node_idx[z]
                                    total += mu[xi, zi]
                        mu[xi, yi] = -total

            # Статистики
            nonzero_mu = np.sum(mu != 0)
            positive_mu = np.sum(mu > 0)
            negative_mu = np.sum(mu < 0)

            # Zeta function ζ(s) на poset
            # ζ_P(s) = Σ_{x≤y} s^{rank(y)-rank(x)}

        except Exception:
            nonzero_mu = 0
            positive_mu = 0
            negative_mu = 0

        return {
            'name': 'mobius_function',
            'description': 'Функция Мёбиуса на poset',
            'status': 'INFO',
            'poset_size': m,
            'nonzero_mobius_values': int(nonzero_mu),
            'positive_values': int(positive_mu),
            'negative_values': int(negative_mu),
            'interpretation': 'μ используется для инверсии сумм по poset'
        }
    except Exception as e:
        return {'name': 'mobius_function', 'status': 'ERROR', 'error': str(e)}
