"""
Design Patterns & Anti-Patterns Validations

Detects code smells and anti-patterns in architecture.
"""

import networkx as nx
import numpy as np
from typing import Dict, List, Any, Optional, TYPE_CHECKING
from collections import defaultdict

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


def validate_god_class(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    God Class Detection - классы с слишком большим количеством методов/зависимостей.
    """
    max_methods = 20
    max_dependencies = 15
    if config and config.params:
        max_methods = config.params.get('max_methods', max_methods)
        max_dependencies = config.params.get('max_dependencies', max_dependencies)
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    try:
        violations = []

        # Группируем методы по типам
        types_methods: Dict[str, List[str]] = defaultdict(list)
        types_deps: Dict[str, set] = defaultdict(set)

        for node in graph.nodes():
            if _is_excluded(node, exclude):
                continue

            node_data = graph.nodes[node]
            entity = node_data.get('entity', '')

            if entity == 'method':
                # Извлекаем тип из имени метода (pkg.Type.Method)
                parts = node.rsplit('.', 1)
                if len(parts) >= 2:
                    type_id = parts[0]
                    types_methods[type_id].append(node)

                    # Собираем зависимости метода
                    for _, target in graph.out_edges(node):
                        types_deps[type_id].add(target)

            elif entity == 'type':
                # Собираем зависимости типа
                for _, target in graph.out_edges(node):
                    types_deps[node].add(target)

        for type_id, methods in types_methods.items():
            deps = types_deps.get(type_id, set())
            method_count = len(methods)
            dep_count = len(deps)

            if method_count > max_methods or dep_count > max_dependencies:
                violations.append({
                    'type': type_id,
                    'methods_count': method_count,
                    'dependencies_count': dep_count,
                    'issues': []
                })
                if method_count > max_methods:
                    violations[-1]['issues'].append(f'Too many methods: {method_count}')
                if dep_count > max_dependencies:
                    violations[-1]['issues'].append(f'Too many dependencies: {dep_count}')

        violations.sort(key=lambda x: x['methods_count'] + x['dependencies_count'], reverse=True)

        if violations:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'god_class',
            'description': f'Классы с <= {max_methods} методами и <= {max_dependencies} зависимостями',
            'status': status,
            'max_methods': max_methods,
            'max_dependencies': max_dependencies,
            'violations': violations[:10],
            'violations_count': len(violations)
        }
    except Exception as e:
        return {'name': 'god_class', 'status': 'ERROR', 'error': str(e)}


def validate_feature_envy(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Feature Envy - метод использует данные другого класса больше своего.
    """
    threshold = 0.5
    if config and config.threshold is not None:
        threshold = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    try:
        violations = []

        for node in graph.nodes():
            if _is_excluded(node, exclude):
                continue

            node_data = graph.nodes[node]
            if node_data.get('entity') != 'method':
                continue

            # Извлекаем собственный тип
            parts = node.rsplit('.', 1)
            if len(parts) < 2:
                continue

            own_type = parts[0]

            # Считаем зависимости по типам
            deps_by_type: Dict[str, int] = defaultdict(int)
            total_deps = 0

            for _, target in graph.out_edges(node):
                target_parts = target.rsplit('.', 1)
                if len(target_parts) >= 2:
                    target_type = target_parts[0]
                else:
                    target_type = target

                deps_by_type[target_type] += 1
                total_deps += 1

            if total_deps < 3:
                continue

            # Проверяем, есть ли тип с большим количеством зависимостей чем свой
            own_deps = deps_by_type.get(own_type, 0)

            for other_type, other_deps in deps_by_type.items():
                if other_type == own_type:
                    continue

                if other_deps > own_deps and other_deps / total_deps > threshold:
                    violations.append({
                        'method': node,
                        'own_type': own_type,
                        'envied_type': other_type,
                        'own_deps': own_deps,
                        'envied_deps': other_deps,
                        'envy_ratio': round(other_deps / total_deps, 2)
                    })
                    break

        violations.sort(key=lambda x: x['envy_ratio'], reverse=True)

        if violations:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'feature_envy',
            'description': f'Методы не завидуют другим классам (threshold {threshold:.0%})',
            'status': status,
            'threshold': threshold,
            'violations': violations[:10],
            'violations_count': len(violations)
        }
    except Exception as e:
        return {'name': 'feature_envy', 'status': 'ERROR', 'error': str(e)}


def validate_shotgun_surgery(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Shotgun Surgery - изменение одного компонента требует изменений во многих местах.

    Метрика: компоненты с очень высоким fan-in (много зависящих от них).
    """
    max_dependents = 10
    if config and config.threshold is not None:
        max_dependents = int(config.threshold)
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    try:
        violations = []

        for node in graph.nodes():
            if _is_excluded(node, exclude):
                continue

            # Считаем входящие зависимости (fan-in)
            dependents = list(graph.predecessors(node))
            fan_in = len(dependents)

            if fan_in > max_dependents:
                violations.append({
                    'component': node,
                    'dependents_count': fan_in,
                    'dependents': dependents[:10]
                })

        violations.sort(key=lambda x: x['dependents_count'], reverse=True)

        if violations:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'shotgun_surgery',
            'description': f'Компоненты с <= {max_dependents} зависящими от них',
            'status': status,
            'threshold': max_dependents,
            'violations': violations[:10],
            'violations_count': len(violations)
        }
    except Exception as e:
        return {'name': 'shotgun_surgery', 'status': 'ERROR', 'error': str(e)}


def validate_divergent_change(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Divergent Change - класс меняется по разным причинам.

    Метрика: класс зависит от компонентов разных доменов.
    """
    max_domains = 3
    if config and config.threshold is not None:
        max_domains = int(config.threshold)
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    try:
        violations = []

        for node in graph.nodes():
            if _is_excluded(node, exclude):
                continue

            node_data = graph.nodes[node]
            if node_data.get('entity') not in ['type', 'package']:
                continue

            # Собираем домены зависимостей
            domains = set()
            for _, target in graph.out_edges(node):
                parts = target.split('/')
                if len(parts) >= 1:
                    domain = parts[0]
                    # Исключаем стандартные библиотеки
                    if not domain.startswith('go') and not domain.startswith('std'):
                        domains.add(domain)

            # Исключаем собственный домен
            own_parts = node.split('/')
            own_domain = own_parts[0] if own_parts else ''
            domains.discard(own_domain)

            if len(domains) > max_domains:
                violations.append({
                    'component': node,
                    'domains_count': len(domains),
                    'domains': list(domains)[:10]
                })

        violations.sort(key=lambda x: x['domains_count'], reverse=True)

        if violations:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'divergent_change',
            'description': f'Компоненты зависят от <= {max_domains} доменов',
            'status': status,
            'threshold': max_domains,
            'violations': violations[:10],
            'violations_count': len(violations)
        }
    except Exception as e:
        return {'name': 'divergent_change', 'status': 'ERROR', 'error': str(e)}


def validate_lazy_class(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Lazy Class - классы с минимальной функциональностью.
    """
    min_methods = 2
    min_dependencies = 1
    if config and config.params:
        min_methods = config.params.get('min_methods', min_methods)
        min_dependencies = config.params.get('min_dependencies', min_dependencies)
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        violations = []

        # Группируем методы по типам
        types_methods: Dict[str, List[str]] = defaultdict(list)

        for node in graph.nodes():
            if _is_excluded(node, exclude):
                continue

            node_data = graph.nodes[node]
            if node_data.get('entity') == 'method':
                parts = node.rsplit('.', 1)
                if len(parts) >= 2:
                    type_id = parts[0]
                    types_methods[type_id].append(node)

        for type_id, methods in types_methods.items():
            out_degree = graph.out_degree(type_id) if type_id in graph else 0

            if len(methods) < min_methods and out_degree < min_dependencies:
                violations.append({
                    'type': type_id,
                    'methods_count': len(methods),
                    'dependencies_count': out_degree
                })

        violations.sort(key=lambda x: x['methods_count'])

        if violations:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'lazy_class',
            'description': f'Классы с >= {min_methods} методами или >= {min_dependencies} зависимостями',
            'status': status,
            'min_methods': min_methods,
            'min_dependencies': min_dependencies,
            'violations': violations[:10],
            'violations_count': len(violations)
        }
    except Exception as e:
        return {'name': 'lazy_class', 'status': 'ERROR', 'error': str(e)}


def validate_middle_man(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Middle Man - классы-посредники без собственной логики.

    Метрика: класс только делегирует вызовы без добавления логики.
    """
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        violations = []

        for node in graph.nodes():
            if _is_excluded(node, exclude):
                continue

            # Входящие и исходящие зависимости
            in_edges = list(graph.in_edges(node))
            out_edges = list(graph.out_edges(node))

            # Middle man: есть входящие, есть исходящие, но соотношение 1:1
            if len(in_edges) >= 2 and len(out_edges) >= 1:
                # Если количество входящих примерно равно исходящим - возможно посредник
                if 0.8 <= len(in_edges) / max(len(out_edges), 1) <= 1.2:
                    # Проверяем паттерны имени
                    node_lower = node.lower()
                    if any(p in node_lower for p in ['proxy', 'wrapper', 'delegate', 'facade']):
                        violations.append({
                            'component': node,
                            'in_degree': len(in_edges),
                            'out_degree': len(out_edges),
                            'ratio': round(len(in_edges) / len(out_edges), 2)
                        })

        violations.sort(key=lambda x: x['in_degree'], reverse=True)

        if violations:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'middle_man',
            'description': 'Обнаружение классов-посредников без логики',
            'status': status,
            'violations': violations[:10],
            'violations_count': len(violations)
        }
    except Exception as e:
        return {'name': 'middle_man', 'status': 'ERROR', 'error': str(e)}


def validate_speculative_generality(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Speculative Generality - неиспользуемые абстракции "на будущее".

    Метрика: интерфейсы/абстрактные классы без реализаций или с одной реализацией.
    """
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        violations = []

        abstract_patterns = ['interface', 'abstract', 'base', 'contract']

        for node in graph.nodes():
            if _is_excluded(node, exclude):
                continue

            node_data = graph.nodes[node]
            node_lower = node.lower()

            is_abstract = (
                node_data.get('entity') == 'interface' or
                any(p in node_lower for p in abstract_patterns)
            )

            if not is_abstract:
                continue

            # Считаем реализации (входящие implements связи)
            implementations = []
            for source, _, data in graph.in_edges(node, data=True):
                if data.get('type') == 'implements':
                    implementations.append(source)

            # Также считаем просто входящие зависимости
            in_degree = graph.in_degree(node)

            if len(implementations) <= 1 and in_degree <= 1:
                violations.append({
                    'abstraction': node,
                    'implementations_count': len(implementations),
                    'usages_count': in_degree,
                    'implementations': implementations
                })

        violations.sort(key=lambda x: x['usages_count'])

        if violations:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'speculative_generality',
            'description': 'Абстракции должны иметь > 1 использования',
            'status': status,
            'violations': violations[:10],
            'violations_count': len(violations)
        }
    except Exception as e:
        return {'name': 'speculative_generality', 'status': 'ERROR', 'error': str(e)}


def validate_data_clumps(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Data Clumps - группы данных, которые всегда появляются вместе.

    Метрика: компоненты, которые всегда используются вместе.
    """
    min_co_occurrence = 3
    if config and config.threshold is not None:
        min_co_occurrence = int(config.threshold)
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        # Собираем наборы зависимостей для каждого узла
        dependency_sets: Dict[str, set] = {}

        for node in graph.nodes():
            if _is_excluded(node, exclude):
                continue

            deps = frozenset(target for _, target in graph.out_edges(node))
            if len(deps) >= 2:
                dependency_sets[node] = deps

        # Ищем часто повторяющиеся пары
        pair_count: Dict[tuple, int] = defaultdict(int)

        for node, deps in dependency_sets.items():
            deps_list = sorted(deps)
            for i, dep1 in enumerate(deps_list):
                for dep2 in deps_list[i + 1:]:
                    pair_count[(dep1, dep2)] += 1

        # Фильтруем часто встречающиеся пары
        clumps = [
            {'pair': list(pair), 'occurrences': count}
            for pair, count in pair_count.items()
            if count >= min_co_occurrence
        ]

        clumps.sort(key=lambda x: x['occurrences'], reverse=True)

        if clumps:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'data_clumps',
            'description': f'Группы зависимостей, появляющиеся >= {min_co_occurrence} раз',
            'status': status,
            'threshold': min_co_occurrence,
            'clumps': clumps[:10],
            'clumps_count': len(clumps)
        }
    except Exception as e:
        return {'name': 'data_clumps', 'status': 'ERROR', 'error': str(e)}


def validate_zigzag_coupling(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Zigzag/Ping-Pong Coupling Detection - detects when a caller oscillates between
    components non-adjacently in its call sequence.

    Pattern (zigzag):
        A.Orchestrate():
          a.b.DoFirst()    # A -> B
          a.c.Process()    # A -> C  (something between)
          a.b.DoSecond()   # A -> B  (non-adjacent repeat = ZIGZAG)
          a.d.Finalize()   # A -> D

    Call sequence: [B, C, B, D] - B appears at positions 0 and 2 with C between = zigzag.

    Adjacent repeats (e.g., [A, A, B]) are NOT violations - only non-adjacent ones.

    Config (.archlint.yaml):
        rules:
          zigzag_coupling:
            enabled: true
            error_on_violation: true
            threshold: 0  # max allowed zigzag occurrences per function
            exclude: []

    Author: @mshogin
    """
    threshold = 0
    if config and config.threshold is not None:
        threshold = int(config.threshold)
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    try:
        violations = []

        for caller in graph.nodes():
            if _is_excluded(caller, exclude):
                continue

            # Get outgoing edges in insertion order (preserves YAML source order)
            out_edges = list(graph.out_edges(caller))
            if len(out_edges) < 3:
                # Need at least 3 edges to have a non-adjacent repeat: [X, Y, X]
                continue

            # Map each target to its component identifier.
            # For callgraph (method) nodes like "a.b.DoFirst", strip the last
            # dot-segment to get the package component "a.b".
            # For architecture (component) nodes, use the node as-is.
            def _to_component(node: str) -> str:
                """
                Map a target node to its component identifier.

                - If the node has entity='method' in the graph, strip last dot-segment.
                - If the node is not in the graph (external callgraph target) and looks
                  like a method call (has a dot/receiver pattern), strip last dot-segment.
                - Otherwise use the node as-is (already a component identifier).
                """
                node_data = graph.nodes.get(node, {})
                entity = node_data.get('entity', '')

                if entity == 'method':
                    # Callgraph method: strip method name to get the class/pkg
                    dot_idx = node.rfind('.')
                    if dot_idx > 0:
                        return node[:dot_idx]

                # For structural component nodes (entity != 'method'), use as-is.
                return node

            # Build sequence of component identifiers preserving YAML/insertion order
            sequence = [_to_component(tgt) for _, tgt in out_edges]

            # Scan for non-adjacent repeats: component X at positions i and j (j > i+1)
            # where at least one different component Y appears between i and j.
            component_last_seen: Dict[str, int] = {}
            zigzag_count = 0
            zigzag_details: List[Dict[str, Any]] = []

            for pos, component in enumerate(sequence):
                if _is_excluded(component, exclude):
                    component_last_seen[component] = pos
                    continue

                if component in component_last_seen:
                    prev_pos = component_last_seen[component]
                    # Non-adjacent: there is at least one position between prev and current
                    if pos > prev_pos + 1:
                        # Verify there is a different component between prev_pos and pos
                        between = sequence[prev_pos + 1:pos]
                        if any(c != component for c in between):
                            zigzag_count += 1
                            zigzag_details.append({
                                'component': component,
                                'positions': [prev_pos, pos],
                                'sequence_between': between,
                            })

                component_last_seen[component] = pos

            if zigzag_count > threshold:
                violations.append({
                    'caller': caller,
                    'zigzag_count': zigzag_count,
                    'sequence': sequence,
                    'zigzags': zigzag_details,
                })

        violations.sort(key=lambda x: x['zigzag_count'], reverse=True)

        if violations:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'zigzag_coupling',
            'description': (
                f'Caller functions should not alternate between the same components '
                f'non-adjacently (threshold: {threshold})'
            ),
            'status': status,
            'threshold': threshold,
            'violations': violations[:10],
            'violations_count': len(violations),
        }
    except Exception as e:
        return {'name': 'zigzag_coupling', 'status': 'ERROR', 'error': str(e)}
