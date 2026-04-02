"""
Метрические правила валидации контекстов (поведенческих компонентов)
"""

import networkx as nx
from typing import Dict, List, Any, Optional, Set, TYPE_CHECKING
from collections import Counter

if TYPE_CHECKING:
    from validator.config import RuleConfig
    from validator.context_loader import Context


def _get_violation_status(error_on_violation: bool) -> str:
    """Возвращает статус при нарушении: ERROR или WARNING"""
    return 'ERROR' if error_on_violation else 'WARNING'


def _is_excluded(item: str, exclude_patterns: List[str]) -> bool:
    """Проверяет, попадает ли элемент под исключения"""
    if not exclude_patterns:
        return False

    item_lower = item.lower()
    for pattern in exclude_patterns:
        pattern_lower = pattern.lower()
        if pattern_lower.endswith('*'):
            if item_lower.startswith(pattern_lower[:-1]):
                return True
        elif pattern_lower.startswith('*'):
            if item_lower.endswith(pattern_lower[1:]):
                return True
        elif '*' in pattern_lower:
            parts = pattern_lower.split('*', 1)
            if item_lower.startswith(parts[0]) and item_lower.endswith(parts[1]):
                return True
        else:
            if pattern_lower in item_lower:
                return True
    return False


def _normalize_component_name(name: str) -> str:
    """Нормализует имя компонента для сравнения"""
    # Убираем путь пакета, оставляем только Type.Method или функцию
    parts = name.split('/')
    return parts[-1] if parts else name


def _find_matching_node(component: str, graph_nodes: Set[str]) -> Optional[str]:
    """Находит соответствующий узел в графе для компонента контекста"""
    norm_component = _normalize_component_name(component)

    for node in graph_nodes:
        norm_node = _normalize_component_name(node)
        # Точное совпадение
        if norm_component == norm_node:
            return node
        # Компонент содержится в узле
        if norm_component in node or component in node:
            return node
        # Узел содержится в компоненте
        if norm_node in component or node in component:
            return node

    return None


# =============================================================================
# COVERAGE VALIDATIONS
# =============================================================================

def validate_context_coverage(
    graph: nx.DiGraph,
    contexts: Dict[str, 'Context'],
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Проверка: критические компоненты (по PageRank) должны быть покрыты тестами.
    """
    threshold = 0.8  # Минимум 80% критических компонентов должны быть в контекстах
    top_n = 10  # Топ N компонентов по PageRank

    if config and config.threshold is not None:
        threshold = config.threshold
    if config and config.params:
        top_n = config.params.get('top_n', top_n)
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    try:
        if len(graph.nodes()) < 2 or not contexts:
            return {
                'name': 'context_coverage',
                'description': 'Покрытие критических компонентов контекстами',
                'status': 'SKIP',
                'reason': 'Недостаточно данных для анализа'
            }

        # Вычисляем PageRank
        pagerank = nx.pagerank(graph)

        # Сортируем по PageRank и берем топ N
        sorted_nodes = sorted(pagerank.items(), key=lambda x: x[1], reverse=True)
        critical_nodes = [
            node for node, _ in sorted_nodes[:top_n]
            if not _is_excluded(node, exclude)
        ]

        # Собираем все компоненты из контекстов
        context_components = set()
        for ctx in contexts.values():
            context_components.update(ctx.components)

        graph_nodes = set(graph.nodes())

        # Проверяем покрытие критических узлов
        covered = []
        uncovered = []

        for node in critical_nodes:
            # Ищем соответствие в компонентах контекстов
            found = False
            for comp in context_components:
                if _find_matching_node(comp, {node}):
                    found = True
                    break

            if found:
                covered.append(node)
            else:
                uncovered.append({
                    'node': node,
                    'pagerank': round(pagerank[node], 4)
                })

        coverage = len(covered) / len(critical_nodes) if critical_nodes else 1.0

        if coverage >= threshold:
            status = 'PASSED'
        else:
            status = _get_violation_status(error_on_violation)

        return {
            'name': 'context_coverage',
            'description': f'Покрытие критических компонентов контекстами >= {threshold*100}%',
            'status': status,
            'coverage': round(coverage, 3),
            'threshold': threshold,
            'critical_count': len(critical_nodes),
            'covered_count': len(covered),
            'uncovered': uncovered[:10]
        }

    except Exception as e:
        return {
            'name': 'context_coverage',
            'description': 'Покрытие критических компонентов',
            'status': 'ERROR',
            'error': str(e)
        }


def validate_untested_components(
    graph: nx.DiGraph,
    contexts: Dict[str, 'Context'],
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Проверка: компоненты из архитектуры, не появляющиеся ни в одном контексте.
    """
    max_untested_ratio = 0.5  # Максимум 50% непокрытых компонентов

    if config and config.threshold is not None:
        max_untested_ratio = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False  # INFO по умолчанию

    # Собираем все компоненты из контекстов
    context_components = set()
    for ctx in contexts.values():
        context_components.update(ctx.components)

    graph_nodes = set(graph.nodes())

    # Находим непокрытые узлы
    untested = []
    tested = []

    for node in graph.nodes():
        if _is_excluded(node, exclude):
            continue

        found = False
        for comp in context_components:
            if _find_matching_node(comp, {node}):
                found = True
                break

        if found:
            tested.append(node)
        else:
            untested.append(node)

    total = len(tested) + len(untested)
    untested_ratio = len(untested) / total if total > 0 else 0

    if not contexts:
        status = 'SKIP'
    elif untested_ratio <= max_untested_ratio:
        status = 'PASSED'
    elif error_on_violation:
        status = 'ERROR'
    else:
        status = 'INFO'

    return {
        'name': 'untested_components',
        'description': f'Компоненты без покрытия контекстами (<= {max_untested_ratio*100}%)',
        'status': status,
        'untested_ratio': round(untested_ratio, 3),
        'threshold': max_untested_ratio,
        'total_components': total,
        'tested_count': len(tested),
        'untested_count': len(untested),
        'untested': untested[:20]
    }


def validate_ghost_components(
    graph: nx.DiGraph,
    contexts: Dict[str, 'Context'],
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Проверка: компоненты в контекстах, отсутствующие в архитектуре (устаревшие тесты).
    """
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    # Собираем все компоненты из контекстов
    context_components = set()
    for ctx in contexts.values():
        context_components.update(ctx.components)

    graph_nodes = set(graph.nodes())

    # Находим "призрачные" компоненты
    ghosts = []

    for comp in context_components:
        if _is_excluded(comp, exclude):
            continue

        # Ищем соответствие в графе
        if not _find_matching_node(comp, graph_nodes):
            ghosts.append(comp)

    if not contexts:
        status = 'SKIP'
    elif not ghosts:
        status = 'PASSED'
    else:
        status = _get_violation_status(error_on_violation)

    return {
        'name': 'ghost_components',
        'description': 'Компоненты в контекстах, отсутствующие в архитектуре',
        'status': status,
        'ghosts': ghosts[:20],
        'ghosts_count': len(ghosts),
        'context_components_total': len(context_components)
    }


# =============================================================================
# DEPENDENCY ANALYSIS
# =============================================================================

def validate_single_point_of_failure(
    graph: nx.DiGraph,
    contexts: Dict[str, 'Context'],
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Проверка: компоненты, присутствующие во ВСЕХ контекстах (критическая зависимость).
    """
    min_contexts = 3  # Минимум контекстов для анализа

    if config and config.params:
        min_contexts = config.params.get('min_contexts', min_contexts)
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False  # INFO по умолчанию

    if len(contexts) < min_contexts:
        return {
            'name': 'single_point_of_failure',
            'description': 'Компоненты, присутствующие во всех контекстах',
            'status': 'SKIP',
            'reason': f'Недостаточно контекстов (< {min_contexts})'
        }

    # Считаем в скольких контекстах появляется каждый компонент
    component_count: Dict[str, int] = Counter()
    for ctx in contexts.values():
        for comp in ctx.components:
            if not _is_excluded(comp, exclude):
                component_count[comp] += 1

    total_contexts = len(contexts)

    # Находим компоненты, присутствующие во всех контекстах
    spof = [
        {'component': comp, 'contexts': count, 'ratio': round(count / total_contexts, 2)}
        for comp, count in component_count.items()
        if count == total_contexts
    ]

    # Также находим "почти везде" (>= 80%)
    high_usage = [
        {'component': comp, 'contexts': count, 'ratio': round(count / total_contexts, 2)}
        for comp, count in component_count.items()
        if count >= total_contexts * 0.8 and count < total_contexts
    ]

    if not spof:
        status = 'PASSED'
    elif error_on_violation:
        status = 'ERROR'
    else:
        status = 'INFO'

    return {
        'name': 'single_point_of_failure',
        'description': 'Компоненты, присутствующие во всех контекстах',
        'status': status,
        'total_contexts': total_contexts,
        'spof_components': spof,
        'spof_count': len(spof),
        'high_usage_components': high_usage[:10],
        'high_usage_count': len(high_usage)
    }


# =============================================================================
# COMPLEXITY METRICS
# =============================================================================

def validate_context_complexity(
    graph: nx.DiGraph,
    contexts: Dict[str, 'Context'],
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Проверка: слишком много компонентов в одном контексте.
    """
    max_components = 15  # Максимум компонентов в контексте

    if config and config.threshold is not None:
        max_components = int(config.threshold)
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    violations = []
    context_stats = []

    for ctx_id, ctx in contexts.items():
        if _is_excluded(ctx_id, exclude):
            continue

        component_count = len(ctx.components)
        context_stats.append({
            'context': ctx_id,
            'title': ctx.title,
            'components': component_count
        })

        if component_count > max_components:
            violations.append({
                'context': ctx_id,
                'title': ctx.title,
                'components': component_count,
                'threshold': max_components
            })

    context_stats.sort(key=lambda x: x['components'], reverse=True)

    if not contexts:
        status = 'SKIP'
    elif not violations:
        status = 'PASSED'
    else:
        status = _get_violation_status(error_on_violation)

    return {
        'name': 'context_complexity',
        'description': f'Контексты с <= {max_components} компонентами',
        'status': status,
        'threshold': max_components,
        'violations': violations[:10],
        'violations_count': len(violations),
        'most_complex': context_stats[:5],
        'total_contexts': len(contexts)
    }


def validate_context_coupling(
    graph: nx.DiGraph,
    contexts: Dict[str, 'Context'],
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Проверка: контексты, разделяющие слишком много общих компонентов.
    """
    max_shared_ratio = 0.7  # Максимум 70% общих компонентов

    if config and config.threshold is not None:
        max_shared_ratio = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False  # INFO по умолчанию

    if len(contexts) < 2:
        return {
            'name': 'context_coupling',
            'description': 'Связность между контекстами',
            'status': 'SKIP',
            'reason': 'Недостаточно контекстов для анализа'
        }

    violations = []
    context_list = list(contexts.items())

    for i, (ctx1_id, ctx1) in enumerate(context_list):
        if _is_excluded(ctx1_id, exclude):
            continue

        for ctx2_id, ctx2 in context_list[i+1:]:
            if _is_excluded(ctx2_id, exclude):
                continue

            set1 = set(ctx1.components)
            set2 = set(ctx2.components)

            shared = set1 & set2
            min_size = min(len(set1), len(set2))

            if min_size > 0:
                shared_ratio = len(shared) / min_size
                if shared_ratio > max_shared_ratio:
                    violations.append({
                        'context1': ctx1_id,
                        'context2': ctx2_id,
                        'shared_count': len(shared),
                        'shared_ratio': round(shared_ratio, 2),
                        'shared_components': list(shared)[:5]
                    })

    violations.sort(key=lambda x: x['shared_ratio'], reverse=True)

    if not violations:
        status = 'PASSED'
    elif error_on_violation:
        status = 'ERROR'
    else:
        status = 'INFO'

    return {
        'name': 'context_coupling',
        'description': f'Контексты с общими компонентами <= {max_shared_ratio*100}%',
        'status': status,
        'threshold': max_shared_ratio,
        'violations': violations[:10],
        'violations_count': len(violations),
        'total_context_pairs': len(context_list) * (len(context_list) - 1) // 2
    }


# =============================================================================
# ARCHITECTURAL COMPLIANCE
# =============================================================================

def validate_layer_traversal(
    graph: nx.DiGraph,
    contexts: Dict[str, 'Context'],
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Проверка: execution flow нарушает слои архитектуры.
    """
    layers = {
        'cmd': 0, 'api': 1, 'handler': 1, 'controller': 1,
        'service': 2, 'usecase': 2,
        'domain': 3, 'entity': 3, 'model': 3,
        'repository': 4, 'storage': 4,
        'infrastructure': 5, 'pkg': 6,
    }

    if config and config.params and 'layers' in config.params:
        layers = config.params['layers']
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    def get_layer(component: str) -> Optional[int]:
        comp_lower = component.lower()
        for pattern, layer in layers.items():
            if pattern in comp_lower:
                return layer
        return None

    violations = []

    for ctx_id, ctx in contexts.items():
        if _is_excluded(ctx_id, exclude):
            continue

        # Анализируем последовательность компонентов
        components = ctx.components
        for i in range(len(components) - 1):
            curr_layer = get_layer(components[i])
            next_layer = get_layer(components[i + 1])

            # Если оба компонента имеют слой и происходит "подъем" вверх
            if curr_layer is not None and next_layer is not None:
                if next_layer < curr_layer:
                    violations.append({
                        'context': ctx_id,
                        'from': components[i],
                        'to': components[i + 1],
                        'from_layer': curr_layer,
                        'to_layer': next_layer,
                        'issue': 'Вызов направлен вверх по слоям'
                    })

    if not contexts:
        status = 'SKIP'
    elif not violations:
        status = 'PASSED'
    else:
        status = _get_violation_status(error_on_violation)

    return {
        'name': 'layer_traversal',
        'description': 'Контексты должны следовать слоям архитектуры',
        'status': status,
        'layers': layers,
        'violations': violations[:10],
        'violations_count': len(violations)
    }


def validate_context_depth(
    graph: nx.DiGraph,
    contexts: Dict[str, 'Context'],
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Проверка: глубина стека вызовов в контексте.
    Примечание: Это приближение на основе количества компонентов,
    для точного анализа нужны данные о вложенности из трейсов.
    """
    max_depth = 10  # Максимальная глубина

    if config and config.threshold is not None:
        max_depth = int(config.threshold)
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    violations = []
    depth_stats = []

    for ctx_id, ctx in contexts.items():
        if _is_excluded(ctx_id, exclude):
            continue

        # Приближенная оценка глубины = количество компонентов
        # (для точной оценки нужен анализ sequence диаграммы)
        estimated_depth = len(ctx.components)

        depth_stats.append({
            'context': ctx_id,
            'title': ctx.title,
            'depth': estimated_depth
        })

        if estimated_depth > max_depth:
            violations.append({
                'context': ctx_id,
                'title': ctx.title,
                'depth': estimated_depth,
                'max_depth': max_depth
            })

    depth_stats.sort(key=lambda x: x['depth'], reverse=True)

    if not contexts:
        status = 'SKIP'
    elif not violations:
        status = 'PASSED'
    else:
        status = _get_violation_status(error_on_violation)

    return {
        'name': 'context_depth',
        'description': f'Глубина контекстов должна быть <= {max_depth}',
        'status': status,
        'threshold': max_depth,
        'violations': violations[:10],
        'violations_count': len(violations),
        'deepest_contexts': depth_stats[:5],
        'total_contexts': len(contexts)
    }
