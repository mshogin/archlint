"""
Game Theory Metrics for Software Architecture Validation.

Implements:
- Nash Equilibrium: stable module configurations
- Shapley Value: fair contribution distribution
- Cooperative Games: coalitions of modules
- Bargaining: resource allocation
- Evolutionary Stability: robust architectures
"""

from typing import Any, Dict, List, Optional, Set, Tuple
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


def validate_shapley_value(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Shapley Value - значение Шепли.

    φᵢ(v) = Σ |S|!(n-|S|-1)!/n! · [v(S∪{i}) - v(S)]

    Справедливое распределение "важности" между модулями.
    v(S) = число рёбер внутри + выходящих из коалиции S.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 2 or n > 15:  # Shapley экспоненциальный
            return {
                'name': 'shapley_value',
                'status': 'SKIP' if n > 15 else 'SKIP',
                'reason': 'Too many nodes for exact Shapley' if n > 15 else 'Insufficient nodes'
            }

        node_list = list(nodes)

        # Характеристическая функция v(S) = edges внутри S + edges из S
        def coalition_value(S: Set) -> float:
            if not S:
                return 0
            internal = sum(1 for u, v in subgraph.edges() if u in S and v in S)
            outgoing = sum(1 for u, v in subgraph.edges() if u in S and v not in S)
            return internal + 0.5 * outgoing

        # Вычисляем Shapley value для каждого игрока
        shapley_values = {}

        for i, player in enumerate(node_list):
            phi = 0.0
            other_players = [p for p in node_list if p != player]

            # Перебираем все подмножества без player
            for r in range(len(other_players) + 1):
                for S_tuple in itertools.combinations(other_players, r):
                    S = set(S_tuple)
                    S_with_i = S | {player}

                    marginal = coalition_value(S_with_i) - coalition_value(S)
                    weight = math.factorial(len(S)) * math.factorial(n - len(S) - 1) / math.factorial(n)
                    phi += weight * marginal

            shapley_values[str(player)] = round(phi, 4)

        # Сортируем по важности
        sorted_shapley = sorted(shapley_values.items(), key=lambda x: -x[1])

        # Проверка: сумма Shapley = v(N)
        total_shapley = sum(shapley_values.values())
        grand_coalition = coalition_value(set(node_list))

        return {
            'name': 'shapley_value',
            'description': 'Значение Шепли (справедливая важность)',
            'status': 'INFO',
            'shapley_values': dict(sorted_shapley),
            'total_shapley': round(total_shapley, 4),
            'grand_coalition_value': round(grand_coalition, 4),
            'efficiency_check': round(abs(total_shapley - grand_coalition), 6),
            'interpretation': 'Shapley = справедливый вклад модуля в архитектуру'
        }
    except Exception as e:
        return {'name': 'shapley_value', 'status': 'ERROR', 'error': str(e)}


def validate_nash_equilibrium(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Nash Equilibrium Analysis - равновесие Нэша.

    Конфигурация равновесна если ни один модуль не хочет
    изменить свои зависимости односторонне.

    Анализируем стабильность текущей конфигурации.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 3:
            return {'name': 'nash_equilibrium', 'status': 'SKIP', 'reason': 'Insufficient nodes'}

        # Для каждого узла оцениваем "полезность" текущих зависимостей
        # Utility = centrality - cost_of_dependencies

        betweenness = nx.betweenness_centrality(subgraph)
        pagerank = nx.pagerank(subgraph)

        utilities = {}
        deviation_incentives = {}

        for node in nodes:
            out_deg = subgraph.out_degree(node)
            in_deg = subgraph.in_degree(node)

            # Utility = влияние - стоимость зависимостей
            utility = pagerank.get(node, 0) * 10 - out_deg * 0.1
            utilities[str(node)] = round(utility, 4)

            # Incentive to deviate: можно ли улучшить utility?
            # Проверяем удаление одной зависимости
            max_improvement = 0
            for succ in subgraph.successors(node):
                # Если убрать эту зависимость
                improvement = 0.1  # Экономия на зависимости
                # Но теряем доступ к функциональности
                loss = pagerank.get(succ, 0) * 0.5
                net = improvement - loss
                if net > max_improvement:
                    max_improvement = net

            deviation_incentives[str(node)] = round(max_improvement, 4)

        # Конфигурация в равновесии если никто не хочет отклоняться
        max_incentive = max(deviation_incentives.values())
        is_equilibrium = max_incentive <= 0.01

        return {
            'name': 'nash_equilibrium',
            'description': 'Анализ равновесия Нэша',
            'status': 'INFO',
            'utilities': dict(sorted(utilities.items(), key=lambda x: -x[1])[:10]),
            'deviation_incentives': dict(sorted(deviation_incentives.items(), key=lambda x: -x[1])[:5]),
            'max_deviation_incentive': round(max_incentive, 4),
            'is_nash_equilibrium': is_equilibrium,
            'interpretation': 'Равновесие = никто не хочет менять зависимости односторонне'
        }
    except Exception as e:
        return {'name': 'nash_equilibrium', 'status': 'ERROR', 'error': str(e)}


def validate_cooperative_games(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Cooperative Games - кооперативные игры.

    Анализируем коалиции модулей:
    - Core: стабильные распределения
    - Nucleolus: минимизация недовольства
    - Superadditivity: v(S∪T) ≥ v(S) + v(T)
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 3:
            return {'name': 'cooperative_games', 'status': 'SKIP', 'reason': 'Insufficient nodes'}

        node_list = list(nodes)

        # v(S) = связность коалиции
        def coalition_value(S: Set) -> float:
            if len(S) <= 1:
                return 0
            sub = subgraph.subgraph(S)
            return sub.number_of_edges()

        # Проверка супераддитивности на выборке
        superadditive_checks = []
        pairs = list(itertools.combinations(range(min(n, 8)), 2))[:10]

        for i, j in pairs:
            S = {node_list[i]}
            T = {node_list[j]}
            v_S = coalition_value(S)
            v_T = coalition_value(T)
            v_ST = coalition_value(S | T)

            is_superadditive = v_ST >= v_S + v_T
            superadditive_checks.append({
                'S': str(node_list[i]),
                'T': str(node_list[j]),
                'v(S)': v_S,
                'v(T)': v_T,
                'v(S∪T)': v_ST,
                'superadditive': is_superadditive
            })

        # Естественные коалиции: сильно связные компоненты
        sccs = list(nx.strongly_connected_components(subgraph))
        scc_values = []
        for scc in sccs:
            val = coalition_value(scc)
            scc_values.append({
                'size': len(scc),
                'value': val,
                'value_per_member': round(val / len(scc), 4) if len(scc) > 0 else 0
            })

        scc_values.sort(key=lambda x: -x['value_per_member'])

        return {
            'name': 'cooperative_games',
            'description': 'Кооперативные игры и коалиции',
            'status': 'INFO',
            'num_natural_coalitions': len(sccs),
            'coalition_values': scc_values[:5],
            'superadditivity_checks': superadditive_checks[:5],
            'superadditivity_ratio': sum(1 for c in superadditive_checks if c['superadditive']) / len(superadditive_checks) if superadditive_checks else 0,
            'interpretation': 'Коалиции = группы модулей с синергией'
        }
    except Exception as e:
        return {'name': 'cooperative_games', 'status': 'ERROR', 'error': str(e)}


def validate_evolutionary_stability(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Evolutionary Stability - эволюционная устойчивость.

    ESS (Evolutionarily Stable Strategy): стратегия, устойчивая
    к вторжению мутантов.

    Для архитектуры: устойчивость к изменениям/добавлениям модулей.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 3:
            return {'name': 'evolutionary_stability', 'status': 'SKIP', 'reason': 'Insufficient nodes'}

        # "Стратегия" модуля = его паттерн зависимостей
        strategies = {}
        for node in nodes:
            in_deg = subgraph.in_degree(node)
            out_deg = subgraph.out_degree(node)
            strategies[str(node)] = (in_deg, out_deg)

        # Группируем по стратегиям
        strategy_groups = defaultdict(list)
        for node, strat in strategies.items():
            strategy_groups[strat].append(node)

        # Доминирующая стратегия
        dominant = max(strategy_groups.items(), key=lambda x: len(x[1]))

        # Проверка устойчивости: может ли "мутант" вторгнуться?
        # Мутант успешен если его fitness > fitness резидента

        fitness_scores = {}
        for node in nodes:
            # Fitness = PageRank (влияние)
            pr = nx.pagerank(subgraph)
            fitness_scores[str(node)] = round(pr.get(node, 0), 4)

        # Средний fitness по стратегиям
        strategy_fitness = {}
        for strat, members in strategy_groups.items():
            avg_fit = np.mean([fitness_scores[m] for m in members])
            strategy_fitness[f'in={strat[0]},out={strat[1]}'] = {
                'count': len(members),
                'avg_fitness': round(avg_fit, 4)
            }

        return {
            'name': 'evolutionary_stability',
            'description': 'Эволюционная устойчивость (ESS)',
            'status': 'INFO',
            'num_strategies': len(strategy_groups),
            'dominant_strategy': f'in={dominant[0][0]},out={dominant[0][1]}',
            'dominant_count': len(dominant[1]),
            'strategy_fitness': dict(sorted(strategy_fitness.items(), key=lambda x: -x[1]['avg_fitness'])[:5]),
            'interpretation': 'ESS = устойчивый паттерн зависимостей'
        }
    except Exception as e:
        return {'name': 'evolutionary_stability', 'status': 'ERROR', 'error': str(e)}


def validate_mechanism_design(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Mechanism Design - дизайн механизмов.

    Как спроектировать "правила игры" чтобы достичь
    желаемого результата (оптимальной архитектуры)?

    Анализируем incentive compatibility и efficiency.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 3:
            return {'name': 'mechanism_design', 'status': 'SKIP', 'reason': 'Insufficient nodes'}

        # Социальный оптимум: максимизация общей связности
        total_edges = subgraph.number_of_edges()

        # Анализ эффективности: насколько близко к оптимуму?
        # Оптимум для DAG с n узлами = n*(n-1)/2 (полный DAG)
        max_edges = n * (n - 1) // 2
        efficiency = total_edges / max_edges if max_edges > 0 else 0

        # Incentive compatibility: правда ли выгодно?
        # Модуль "говорит правду" о зависимостях если это выгодно

        # Price of Anarchy: отношение социального оптимума к равновесию
        # Здесь: сравниваем с случайным графом той же плотности
        density = nx.density(subgraph)
        poa_estimate = 1 / (1 + density) if density < 1 else 0.5

        # Механизм VCG (Vickrey-Clarke-Groves)
        # Каждый модуль "платит" за externality

        externalities = {}
        for node in list(nodes)[:10]:
            # Externality = влияние удаления узла на других
            temp = subgraph.copy()
            temp.remove_node(node)
            edges_lost = subgraph.number_of_edges() - temp.number_of_edges()
            externalities[str(node)] = edges_lost

        return {
            'name': 'mechanism_design',
            'description': 'Дизайн механизмов (оптимальные правила)',
            'status': 'INFO',
            'total_edges': total_edges,
            'efficiency': round(efficiency, 4),
            'price_of_anarchy_estimate': round(poa_estimate, 4),
            'externalities': dict(sorted(externalities.items(), key=lambda x: -x[1])[:5]),
            'interpretation': 'Mechanism design = как incentivize хорошую архитектуру'
        }
    except Exception as e:
        return {'name': 'mechanism_design', 'status': 'ERROR', 'error': str(e)}


def validate_voting_power(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Voting Power - власть голосования.

    Banzhaf index: вероятность быть swing voter.
    Применяем к "голосованию" за изменения в архитектуре.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 2 or n > 12:
            return {
                'name': 'voting_power',
                'status': 'SKIP',
                'reason': 'Too many nodes for Banzhaf' if n > 12 else 'Insufficient nodes'
            }

        node_list = list(nodes)

        # Коалиция "выигрывает" если содержит все зависимости
        # Swing voter: без него коалиция не выигрывает

        def is_winning(S: Set) -> bool:
            # Выигрывает если содержит все достижимые из источников
            sources = [node for node in S if subgraph.in_degree(node) == 0]
            if not sources:
                return len(S) > n // 2  # Простое большинство
            return len(S) > n // 2

        # Banzhaf index
        banzhaf = {}
        total_swings = 0

        for player in node_list:
            swing_count = 0
            other = [p for p in node_list if p != player]

            for r in range(len(other) + 1):
                for S_tuple in itertools.combinations(other, r):
                    S = set(S_tuple)
                    S_with = S | {player}

                    # Player is swing if S∪{player} wins but S doesn't
                    if is_winning(S_with) and not is_winning(S):
                        swing_count += 1

            banzhaf[str(player)] = swing_count
            total_swings += swing_count

        # Нормализуем
        if total_swings > 0:
            banzhaf_normalized = {k: round(v / total_swings, 4) for k, v in banzhaf.items()}
        else:
            banzhaf_normalized = {k: 0 for k in banzhaf.keys()}

        return {
            'name': 'voting_power',
            'description': 'Власть голосования (Banzhaf index)',
            'status': 'INFO',
            'banzhaf_index': dict(sorted(banzhaf_normalized.items(), key=lambda x: -x[1])),
            'total_swing_positions': total_swings,
            'interpretation': 'Banzhaf = вероятность быть решающим при изменениях'
        }
    except Exception as e:
        return {'name': 'voting_power', 'status': 'ERROR', 'error': str(e)}
