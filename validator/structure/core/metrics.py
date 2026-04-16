"""
Метрические правила валидации графов
"""

import networkx as nx
from typing import Dict, List, Any, Optional, Set, TYPE_CHECKING

if TYPE_CHECKING:
    from validator.config import RuleConfig


def _get_violation_status(error_on_violation: bool) -> str:
    """Возвращает статус при нарушении: ERROR или WARNING"""
    return 'ERROR' if error_on_violation else 'WARNING'


def _is_excluded(node: str, exclude_patterns: List[str]) -> bool:
    """Проверяет, попадает ли узел под исключения"""
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


def validate_dag(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Проверка: граф должен быть ациклическим (DAG)
    """
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    try:
        from validator.utils.cycles import simple_cycles_bounded
        all_cycles = list(simple_cycles_bounded(graph, max_length=10))

        # Фильтруем циклы по исключениям
        cycles = []
        for cycle in all_cycles:
            # Цикл исключён только если ВСЕ узлы в нём исключены
            if not all(_is_excluded(node, exclude) for node in cycle):
                cycles.append(cycle)

        if cycles:
            status = 'FAILED'  # Циклы всегда FAILED
        else:
            status = 'PASSED'

        return {
            'name': 'dag_check',
            'description': 'Граф должен быть ациклическим (без циклических зависимостей)',
            'status': status,
            'cycles': cycles[:10],  # Ограничиваем вывод
            'cycles_count': len(cycles)
        }
    except Exception as e:
        return {
            'name': 'dag_check',
            'description': 'Граф должен быть ациклическим',
            'status': 'ERROR',
            'error': str(e)
        }


def validate_max_fan_out(
    graph: nx.DiGraph,
    threshold: int = 5,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Проверка: компонент не должен зависеть от слишком многих других
    """
    if config and config.threshold is not None:
        threshold = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    violations = []

    for node in graph.nodes():
        if _is_excluded(node, exclude):
            continue

        out_degree = graph.out_degree(node)
        if out_degree > threshold:
            violations.append({
                'node': node,
                'fan_out': out_degree,
                'threshold': threshold
            })

    if violations:
        status = _get_violation_status(error_on_violation)
    else:
        status = 'PASSED'

    return {
        'name': 'max_fan_out',
        'description': f'Компонент не должен зависеть от более чем {threshold} других',
        'status': status,
        'threshold': threshold,
        'violations': violations[:10],
        'violations_count': len(violations)
    }


def validate_modularity(
    graph: nx.DiGraph,
    threshold: float = 0.3,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Проверка: компоненты должны быть хорошо изолированы (modularity)
    """
    if config and config.threshold is not None:
        threshold = config.threshold
    error_on_violation = config.error_on_violation if config else True

    try:
        undirected = graph.to_undirected()
        communities = nx.community.louvain_communities(undirected)
        modularity = nx.community.modularity(undirected, communities)

        if modularity >= threshold:
            status = 'PASSED'
        else:
            status = _get_violation_status(error_on_violation)

        return {
            'name': 'modularity',
            'description': f'Модульность графа должна быть >= {threshold}',
            'status': status,
            'modularity': round(modularity, 3),
            'threshold': threshold,
            'communities_count': len(communities)
        }
    except ZeroDivisionError:
        return {
            'name': 'modularity',
            'description': 'Модульность графа',
            'status': 'SKIP',
            'reason': 'Граф не содержит достаточно связей для вычисления модульности'
        }
    except Exception as e:
        return {
            'name': 'modularity',
            'description': 'Модульность графа',
            'status': 'SKIP',
            'reason': str(e)
        }


# =============================================================================
# CENTRALITY METRICS (Bottleneck Detection)
# =============================================================================

def validate_betweenness_centrality(
    graph: nx.DiGraph,
    threshold: float = 0.3,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Проверка: выявление узких мест (bottlenecks) в архитектуре.
    """
    if config and config.threshold is not None:
        threshold = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    try:
        if len(graph.nodes()) < 2:
            return {
                'name': 'betweenness_centrality',
                'description': 'Выявление узких мест (bottlenecks)',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов для анализа'
            }

        centrality = nx.betweenness_centrality(graph)

        bottlenecks = [
            {'node': node, 'centrality': round(value, 3)}
            for node, value in centrality.items()
            if value > threshold and not _is_excluded(node, exclude)
        ]

        bottlenecks.sort(key=lambda x: x['centrality'], reverse=True)

        if bottlenecks:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'betweenness_centrality',
            'description': f'Компоненты с centrality > {threshold} являются узкими местами',
            'status': status,
            'threshold': threshold,
            'bottlenecks': bottlenecks[:10],
            'bottlenecks_count': len(bottlenecks)
        }
    except Exception as e:
        return {
            'name': 'betweenness_centrality',
            'description': 'Выявление узких мест',
            'status': 'ERROR',
            'error': str(e)
        }


def validate_pagerank(
    graph: nx.DiGraph,
    threshold: float = 0.1,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Проверка: выявление критически важных компонентов через PageRank.
    """
    if config and config.threshold is not None:
        threshold = config.threshold
    exclude = config.exclude if config else []
    # PageRank по умолчанию INFO
    error_on_violation = config.error_on_violation if config else False

    try:
        if len(graph.nodes()) < 2:
            return {
                'name': 'pagerank',
                'description': 'Выявление критически важных компонентов',
                'status': 'SKIP',
                'reason': 'Недостаточно узлов для анализа'
            }

        pagerank = nx.pagerank(graph)

        critical = [
            {'node': node, 'pagerank': round(value, 3)}
            for node, value in pagerank.items()
            if value > threshold and not _is_excluded(node, exclude)
        ]

        critical.sort(key=lambda x: x['pagerank'], reverse=True)

        # PageRank использует INFO если error_on_violation=False
        if critical and error_on_violation:
            status = 'ERROR'
        elif critical:
            status = 'INFO'
        else:
            status = 'PASSED'

        return {
            'name': 'pagerank',
            'description': f'Компоненты с PageRank > {threshold} критически важны',
            'status': status,
            'threshold': threshold,
            'critical_components': critical[:10],
            'critical_count': len(critical)
        }
    except Exception as e:
        return {
            'name': 'pagerank',
            'description': 'Выявление критических компонентов',
            'status': 'ERROR',
            'error': str(e)
        }


# =============================================================================
# COUPLING METRICS
# =============================================================================

def validate_coupling(
    graph: nx.DiGraph,
    ca_threshold: int = 10,
    ce_threshold: int = 10,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Проверка связности компонентов (Ca/Ce).
    """
    if config and config.params:
        ca_threshold = config.params.get('ca_threshold', ca_threshold)
        ce_threshold = config.params.get('ce_threshold', ce_threshold)
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    violations = []

    for node in graph.nodes():
        if _is_excluded(node, exclude):
            continue

        ca = graph.in_degree(node)
        ce = graph.out_degree(node)

        if ca > ca_threshold or ce > ce_threshold:
            violations.append({
                'node': node,
                'afferent_coupling': ca,
                'efferent_coupling': ce,
                'ca_threshold': ca_threshold,
                'ce_threshold': ce_threshold
            })

    if violations:
        status = _get_violation_status(error_on_violation)
    else:
        status = 'PASSED'

    return {
        'name': 'coupling',
        'description': f'Ca <= {ca_threshold}, Ce <= {ce_threshold}',
        'status': status,
        'ca_threshold': ca_threshold,
        'ce_threshold': ce_threshold,
        'violations': violations[:10],
        'violations_count': len(violations)
    }


def validate_instability(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Проверка нестабильности компонентов.
    I = Ce / (Ca + Ce)
    """
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    instability_data = []
    violations = []

    for node in graph.nodes():
        ca = graph.in_degree(node)
        ce = graph.out_degree(node)

        if ca + ce > 0:
            instability = ce / (ca + ce)
        else:
            instability = 0.5

        instability_data.append({
            'node': node,
            'instability': round(instability, 3),
            'ca': ca,
            'ce': ce
        })

    instability_map = {item['node']: item['instability'] for item in instability_data}

    for edge in graph.edges():
        source, target = edge
        # Пропускаем исключённые узлы
        if _is_excluded(source, exclude) or _is_excluded(target, exclude):
            continue

        source_i = instability_map.get(source, 0.5)
        target_i = instability_map.get(target, 0.5)

        if target_i > source_i + 0.1:
            violations.append({
                'from': source,
                'to': target,
                'from_instability': round(source_i, 3),
                'to_instability': round(target_i, 3),
                'issue': 'Зависимость направлена к менее стабильному компоненту'
            })

    if violations:
        status = _get_violation_status(error_on_violation)
    else:
        status = 'PASSED'

    instability_data.sort(key=lambda x: x['instability'], reverse=True)

    return {
        'name': 'instability',
        'description': 'Зависимости должны идти от нестабильных к стабильным',
        'status': status,
        'most_unstable': instability_data[:5],
        'most_stable': instability_data[-5:] if len(instability_data) > 5 else [],
        'violations': violations[:10],
        'violations_count': len(violations)
    }


# =============================================================================
# STRUCTURAL CHECKS
# =============================================================================

def validate_orphan_nodes(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """Проверка: выявление изолированных компонентов."""
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    orphans = [
        node for node in graph.nodes()
        if graph.degree(node) == 0 and not _is_excluded(node, exclude)
    ]

    if orphans:
        status = _get_violation_status(error_on_violation)
    else:
        status = 'PASSED'

    return {
        'name': 'orphan_nodes',
        'description': 'Компоненты без связей (возможно мертвый код)',
        'status': status,
        'orphans': orphans[:20],
        'orphans_count': len(orphans)
    }


def validate_strongly_connected_components(
    graph: nx.DiGraph,
    max_size: int = 1,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """Проверка: выявление групп с взаимными зависимостями."""
    if config and config.params:
        max_size = config.params.get('max_size', max_size)
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    sccs = list(nx.strongly_connected_components(graph))

    # Фильтруем SCC с учётом исключений
    large_sccs = []
    for scc in sccs:
        if len(scc) > max_size:
            # Проверяем что хотя бы один узел не исключён
            if not all(_is_excluded(node, exclude) for node in scc):
                large_sccs.append(list(scc))

    if large_sccs:
        status = _get_violation_status(error_on_violation)
    else:
        status = 'PASSED'

    return {
        'name': 'strongly_connected_components',
        'description': f'Группы с взаимными зависимостями (размер > {max_size})',
        'status': status,
        'total_sccs': len(sccs),
        'problematic_sccs': large_sccs[:10],
        'problematic_count': len(large_sccs)
    }


def validate_graph_depth(
    graph: nx.DiGraph,
    max_depth: int = 10,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """Проверка: глубина графа зависимостей."""
    if config and config.threshold is not None:
        max_depth = int(config.threshold)
    error_on_violation = config.error_on_violation if config else True

    try:
        if not nx.is_directed_acyclic_graph(graph):
            return {
                'name': 'graph_depth',
                'description': 'Глубина графа зависимостей',
                'status': 'SKIP',
                'reason': 'Граф содержит циклы, невозможно вычислить глубину'
            }

        if len(graph.edges()) == 0:
            return {
                'name': 'graph_depth',
                'description': 'Глубина графа зависимостей',
                'status': 'PASSED',
                'depth': 0,
                'max_depth': max_depth
            }

        longest_path = nx.dag_longest_path(graph)
        depth = len(longest_path) - 1 if longest_path else 0

        if depth > max_depth:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'graph_depth',
            'description': f'Глубина графа должна быть <= {max_depth}',
            'status': status,
            'depth': depth,
            'max_depth': max_depth,
            'longest_path': longest_path[:10] if len(longest_path) > 10 else longest_path
        }
    except Exception as e:
        return {
            'name': 'graph_depth',
            'description': 'Глубина графа зависимостей',
            'status': 'ERROR',
            'error': str(e)
        }


# =============================================================================
# ARCHITECTURAL RULES
# =============================================================================

def validate_layer_violations(
    graph: nx.DiGraph,
    layers: Optional[Dict[str, int]] = None,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """Проверка: зависимости должны идти только вниз по слоям."""
    if config and config.params and 'layers' in config.params:
        layers = config.params['layers']
    elif layers is None:
        layers = {
            'cmd': 0, 'api': 1, 'handler': 1, 'controller': 1,
            'service': 2, 'usecase': 2, 'internal': 2,
            'domain': 3, 'entity': 3, 'model': 3,
            'repository': 4, 'storage': 4, 'infrastructure': 5, 'pkg': 6,
        }
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    def get_layer(node: str) -> Optional[int]:
        node_lower = node.lower()
        for pattern, layer in layers.items():
            if pattern in node_lower:
                return layer
        return None

    violations = []
    for source, target in graph.edges():
        if _is_excluded(source, exclude) or _is_excluded(target, exclude):
            continue
        source_layer = get_layer(source)
        target_layer = get_layer(target)
        if source_layer is not None and target_layer is not None:
            if target_layer < source_layer:
                violations.append({
                    'from': source, 'to': target,
                    'from_layer': source_layer, 'to_layer': target_layer,
                    'issue': 'Зависимость направлена вверх по слоям'
                })

    if violations:
        status = _get_violation_status(error_on_violation)
    else:
        status = 'PASSED'

    return {
        'name': 'layer_violations',
        'description': 'Зависимости должны идти только вниз по слоям',
        'status': status,
        'layers': layers,
        'violations': violations[:10],
        'violations_count': len(violations)
    }


def validate_forbidden_dependencies(
    graph: nx.DiGraph,
    rules: Optional[List[Dict[str, str]]] = None,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """Проверка: запрещенные зависимости между компонентами."""
    if config and config.params and 'rules' in config.params:
        rules = config.params['rules']
    elif rules is None:
        rules = [
            {'from': 'handler', 'to': 'repository'},
            {'from': 'controller', 'to': 'repository'},
            {'from': 'api', 'to': 'storage'},
            {'from': 'model', 'to': 'service'},
            {'from': 'entity', 'to': 'repository'},
        ]
    exclude = config.exclude if config else []

    violations = []
    for source, target in graph.edges():
        if _is_excluded(source, exclude) or _is_excluded(target, exclude):
            continue
        source_lower = source.lower()
        target_lower = target.lower()
        for rule in rules:
            from_pattern = rule['from'].lower()
            to_pattern = rule['to'].lower()
            if from_pattern in source_lower and to_pattern in target_lower:
                violations.append({
                    'from': source, 'to': target,
                    'rule': f"{rule['from']} -> {rule['to']}",
                    'issue': 'Запрещенная зависимость'
                })

    # forbidden_dependencies всегда FAILED при нарушении
    status = 'FAILED' if violations else 'PASSED'

    return {
        'name': 'forbidden_dependencies',
        'description': 'Проверка запрещенных зависимостей',
        'status': status,
        'rules': rules,
        'violations': violations[:10],
        'violations_count': len(violations)
    }


def validate_component_distance(
    graph: nx.DiGraph,
    max_distance: int = 5,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """Проверка: расстояние между компонентами."""
    if config and config.threshold is not None:
        max_distance = int(config.threshold)
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    try:
        long_paths = []
        for source in graph.nodes():
            if _is_excluded(source, exclude):
                continue
            lengths = nx.single_source_shortest_path_length(graph, source)
            for target, distance in lengths.items():
                if distance > max_distance and not _is_excluded(target, exclude):
                    long_paths.append({
                        'from': source, 'to': target, 'distance': distance
                    })

        long_paths.sort(key=lambda x: x['distance'], reverse=True)

        if long_paths:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'component_distance',
            'description': f'Расстояние между компонентами должно быть <= {max_distance}',
            'status': status,
            'max_distance': max_distance,
            'long_paths': long_paths[:10],
            'long_paths_count': len(long_paths)
        }
    except Exception as e:
        return {
            'name': 'component_distance',
            'description': 'Расстояние между компонентами',
            'status': 'ERROR',
            'error': str(e)
        }


# =============================================================================
# DESIGN QUALITY METRICS
# =============================================================================

def validate_abstractness(
    graph: nx.DiGraph,
    abstract_patterns: Optional[List[str]] = None,
    min_threshold: float = 0.1,
    max_threshold: float = 0.8,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """Проверка абстрактности архитектуры."""
    if config and config.params:
        abstract_patterns = config.params.get('patterns', abstract_patterns)
        min_threshold = config.params.get('min_threshold', min_threshold)
        max_threshold = config.params.get('max_threshold', max_threshold)
    if abstract_patterns is None:
        abstract_patterns = ['interface', 'abstract', 'base', 'contract', 'port']
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    nodes_list = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
    total_nodes = len(nodes_list)

    if total_nodes == 0:
        return {
            'name': 'abstractness',
            'description': 'Уровень абстрактности архитектуры',
            'status': 'SKIP',
            'reason': 'Граф пуст'
        }

    abstract_nodes = [n for n in nodes_list if any(p in n.lower() for p in abstract_patterns)]
    abstractness = len(abstract_nodes) / total_nodes

    if abstractness < min_threshold:
        status = _get_violation_status(error_on_violation)
        issue = f'Слишком мало абстракций (< {min_threshold})'
    elif abstractness > max_threshold:
        status = _get_violation_status(error_on_violation)
        issue = f'Слишком много абстракций (> {max_threshold})'
    else:
        status = 'PASSED'
        issue = None

    result = {
        'name': 'abstractness',
        'description': f'Абстрактность должна быть между {min_threshold} и {max_threshold}',
        'status': status,
        'abstractness': round(abstractness, 3),
        'abstract_count': len(abstract_nodes),
        'concrete_count': total_nodes - len(abstract_nodes),
        'total_count': total_nodes,
        'min_threshold': min_threshold,
        'max_threshold': max_threshold
    }
    if issue:
        result['issue'] = issue
    return result


def validate_distance_from_main_sequence(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """Проверка Distance from Main Sequence."""
    threshold = 0.5
    if config and config.threshold is not None:
        threshold = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    abstract_patterns = ['interface', 'abstract', 'base', 'contract', 'port']

    packages: Dict[str, Set[str]] = {}
    for node in graph.nodes():
        if _is_excluded(node, exclude):
            continue
        parts = node.replace('.', '/').split('/')
        if parts:
            pkg = parts[0]
            if pkg not in packages:
                packages[pkg] = set()
            packages[pkg].add(node)

    results = []
    violations = []

    for pkg, nodes in packages.items():
        if not nodes:
            continue

        abstract_count = sum(1 for n in nodes if any(p in n.lower() for p in abstract_patterns))
        abstractness = abstract_count / len(nodes)

        total_ca = sum(graph.in_degree(n) for n in nodes)
        total_ce = sum(graph.out_degree(n) for n in nodes)
        instability = total_ce / (total_ca + total_ce) if (total_ca + total_ce) > 0 else 0.5

        distance = abs(abstractness + instability - 1)

        pkg_result = {
            'package': pkg,
            'abstractness': round(abstractness, 3),
            'instability': round(instability, 3),
            'distance': round(distance, 3),
            'nodes_count': len(nodes)
        }
        results.append(pkg_result)

        if distance > threshold:
            if abstractness < 0.2 and instability < 0.2:
                pkg_result['zone'] = 'pain'
                pkg_result['issue'] = 'Зона боли: конкретный и стабильный'
            elif abstractness > 0.8 and instability > 0.8:
                pkg_result['zone'] = 'uselessness'
                pkg_result['issue'] = 'Зона бесполезности: абстрактный и нестабильный'
            violations.append(pkg_result)

    results.sort(key=lambda x: x['distance'], reverse=True)

    if violations:
        status = _get_violation_status(error_on_violation)
    else:
        status = 'PASSED'

    return {
        'name': 'distance_from_main_sequence',
        'description': 'D = |A + I - 1| должен быть близок к 0',
        'status': status,
        'packages': results[:10],
        'violations': violations[:5],
        'violations_count': len(violations)
    }


def validate_hub_nodes(
    graph: nx.DiGraph,
    threshold: int = 10,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """Проверка: выявление hub-узлов (God Objects)."""
    if config and config.threshold is not None:
        threshold = int(config.threshold)
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    hubs = []
    for node in graph.nodes():
        if _is_excluded(node, exclude):
            continue
        total_degree = graph.degree(node)
        if total_degree > threshold:
            hubs.append({
                'node': node,
                'total_degree': total_degree,
                'in_degree': graph.in_degree(node),
                'out_degree': graph.out_degree(node)
            })

    hubs.sort(key=lambda x: x['total_degree'], reverse=True)

    if hubs:
        status = _get_violation_status(error_on_violation)
    else:
        status = 'PASSED'

    return {
        'name': 'hub_nodes',
        'description': f'Компоненты с > {threshold} связей (потенциальные God Objects)',
        'status': status,
        'threshold': threshold,
        'hubs': hubs[:10],
        'hubs_count': len(hubs)
    }
