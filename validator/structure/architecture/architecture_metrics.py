"""
Clean Architecture / Hexagonal Architecture Validations

Validates architectural boundaries and patterns.
"""

import networkx as nx
from typing import Dict, List, Any, Optional, Set, TYPE_CHECKING
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


def validate_domain_isolation(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Domain Isolation - домен не должен зависеть от infrastructure.

    Проверяет что domain слой не импортирует: db, http, grpc, io, net, etc.
    """
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    forbidden_patterns = ['database', 'db', 'sql', 'http', 'grpc', 'net', 'io',
                          'repository', 'adapter', 'infrastructure', 'redis',
                          'kafka', 'rabbitmq', 'aws', 'gcp', 'azure']

    if config and config.params:
        forbidden_patterns = config.params.get('forbidden_patterns', forbidden_patterns)

    try:
        violations = []
        domain_patterns = ['domain', 'entity', 'model', 'core', 'business']

        for node in graph.nodes():
            if _is_excluded(node, exclude):
                continue

            node_lower = node.lower()

            # Проверяем только domain компоненты
            is_domain = any(p in node_lower for p in domain_patterns)
            if not is_domain:
                continue

            for _, target in graph.out_edges(node):
                target_lower = target.lower()

                for forbidden in forbidden_patterns:
                    if forbidden in target_lower:
                        violations.append({
                            'domain_component': node,
                            'forbidden_dependency': target,
                            'pattern': forbidden
                        })
                        break

        if violations:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'domain_isolation',
            'description': 'Domain не зависит от infrastructure',
            'status': status,
            'forbidden_patterns': forbidden_patterns,
            'violations': violations[:20],
            'violations_count': len(violations)
        }
    except Exception as e:
        return {'name': 'domain_isolation', 'status': 'ERROR', 'error': str(e)}


def validate_ports_adapters(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Ports & Adapters - порты (интерфейсы) должны быть в domain, адаптеры в infrastructure.
    """
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    try:
        violations = []

        port_patterns = ['port', 'interface', 'contract', 'gateway']
        adapter_patterns = ['adapter', 'impl', 'repository', 'handler', 'controller']
        domain_patterns = ['domain', 'core', 'business', 'entity']
        infra_patterns = ['infrastructure', 'infra', 'adapter', 'handler', 'controller']

        for node in graph.nodes():
            if _is_excluded(node, exclude):
                continue

            node_lower = node.lower()

            # Порты должны быть в domain
            is_port = any(p in node_lower for p in port_patterns)
            is_in_domain = any(p in node_lower for p in domain_patterns)
            is_in_infra = any(p in node_lower for p in infra_patterns)

            if is_port and is_in_infra and not is_in_domain:
                violations.append({
                    'component': node,
                    'issue': 'Port should be in domain layer',
                    'type': 'port_in_infra'
                })

            # Адаптеры должны быть в infrastructure
            is_adapter = any(p in node_lower for p in adapter_patterns)
            if is_adapter and is_in_domain:
                violations.append({
                    'component': node,
                    'issue': 'Adapter should be in infrastructure layer',
                    'type': 'adapter_in_domain'
                })

        if violations:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'ports_adapters',
            'description': 'Порты в domain, адаптеры в infrastructure',
            'status': status,
            'violations': violations[:20],
            'violations_count': len(violations)
        }
    except Exception as e:
        return {'name': 'ports_adapters', 'status': 'ERROR', 'error': str(e)}


def validate_use_case_purity(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Use Case Purity - use cases не должны содержать infrastructure код.
    """
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    forbidden_patterns = ['http', 'grpc', 'sql', 'database', 'redis', 'kafka',
                          'aws', 'gcp', 'net', 'io']

    if config and config.params:
        forbidden_patterns = config.params.get('forbidden_patterns', forbidden_patterns)

    try:
        violations = []
        usecase_patterns = ['usecase', 'use_case', 'usecases', 'application', 'service']

        for node in graph.nodes():
            if _is_excluded(node, exclude):
                continue

            node_lower = node.lower()

            # Проверяем только use case компоненты
            is_usecase = any(p in node_lower for p in usecase_patterns)
            if not is_usecase:
                continue

            for _, target in graph.out_edges(node):
                target_lower = target.lower()

                for forbidden in forbidden_patterns:
                    if forbidden in target_lower:
                        violations.append({
                            'usecase': node,
                            'forbidden_dependency': target,
                            'pattern': forbidden
                        })
                        break

        if violations:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'use_case_purity',
            'description': 'Use cases не зависят от infrastructure',
            'status': status,
            'violations': violations[:20],
            'violations_count': len(violations)
        }
    except Exception as e:
        return {'name': 'use_case_purity', 'status': 'ERROR', 'error': str(e)}


def validate_dto_boundaries(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    DTO Boundaries - на границах слоёв должны использоваться DTO.
    """
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        violations = []

        boundary_patterns = ['handler', 'controller', 'api', 'endpoint', 'grpc', 'rest']
        domain_entity_patterns = ['entity', 'aggregate', 'domain']
        dto_patterns = ['dto', 'request', 'response', 'view', 'model']

        for node in graph.nodes():
            if _is_excluded(node, exclude):
                continue

            node_lower = node.lower()

            # Проверяем только boundary компоненты
            is_boundary = any(p in node_lower for p in boundary_patterns)
            if not is_boundary:
                continue

            for _, target in graph.out_edges(node):
                target_lower = target.lower()

                # Проверяем что boundary не возвращает domain entities напрямую
                is_domain_entity = any(p in target_lower for p in domain_entity_patterns)
                is_dto = any(p in target_lower for p in dto_patterns)

                if is_domain_entity and not is_dto:
                    violations.append({
                        'boundary': node,
                        'domain_entity': target,
                        'issue': 'Boundary should use DTOs, not domain entities'
                    })

        if violations:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'dto_boundaries',
            'description': 'На границах слоёв используются DTO',
            'status': status,
            'violations': violations[:20],
            'violations_count': len(violations)
        }
    except Exception as e:
        return {'name': 'dto_boundaries', 'status': 'ERROR', 'error': str(e)}


def validate_inward_dependencies(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Inward Dependencies - зависимости направлены только внутрь (к domain).

    Clean Architecture: outer layers depend on inner layers, not vice versa.
    """
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    # Слои от внешнего к внутреннему (0 = самый внешний)
    layer_order = {
        'handler': 0, 'controller': 0, 'api': 0, 'endpoint': 0,
        'adapter': 1, 'gateway': 1, 'repository': 1,
        'usecase': 2, 'service': 2, 'application': 2,
        'domain': 3, 'entity': 3, 'core': 3, 'model': 3
    }

    if config and config.params:
        layer_order = config.params.get('layer_order', layer_order)

    try:
        violations = []

        def get_layer(node: str) -> int:
            node_lower = node.lower()
            for pattern, layer in layer_order.items():
                if pattern in node_lower:
                    return layer
            return -1  # Unknown layer

        for source in graph.nodes():
            if _is_excluded(source, exclude):
                continue

            source_layer = get_layer(source)
            if source_layer == -1:
                continue

            for _, target in graph.out_edges(source):
                target_layer = get_layer(target)
                if target_layer == -1:
                    continue

                # Зависимость должна идти от внешнего к внутреннему (от меньшего к большему)
                if source_layer > target_layer:
                    violations.append({
                        'source': source,
                        'source_layer': source_layer,
                        'target': target,
                        'target_layer': target_layer,
                        'issue': f'Inner layer ({source_layer}) depends on outer layer ({target_layer})'
                    })

        if violations:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'inward_dependencies',
            'description': 'Зависимости направлены к центру (domain)',
            'status': status,
            'layer_order': layer_order,
            'violations': violations[:20],
            'violations_count': len(violations)
        }
    except Exception as e:
        return {'name': 'inward_dependencies', 'status': 'ERROR', 'error': str(e)}


def validate_bounded_context_leakage(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Bounded Context Leakage - типы одного контекста не используются в другом напрямую.
    """
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    try:
        violations = []

        # Определяем контексты по структуре пакетов
        contexts: Dict[str, Set[str]] = defaultdict(set)

        for node in graph.nodes():
            if _is_excluded(node, exclude):
                continue

            parts = node.split('/')
            if len(parts) >= 3:
                # Предполагаем структуру: org/project/context/...
                context = parts[2] if len(parts) > 2 else parts[1]
                contexts[context].add(node)

        # Проверяем зависимости между контекстами
        allowed_cross_context = ['dto', 'event', 'message', 'api', 'contract']

        for context, nodes in contexts.items():
            for node in nodes:
                node_lower = node.lower()

                # Пропускаем shared/common
                if any(p in node_lower for p in ['shared', 'common', 'pkg', 'lib']):
                    continue

                for _, target in graph.out_edges(node):
                    target_lower = target.lower()

                    # Определяем контекст target
                    target_parts = target.split('/')
                    target_context = target_parts[2] if len(target_parts) > 2 else ''

                    if target_context and target_context != context:
                        # Проверяем, что это не разрешённый тип
                        is_allowed = any(p in target_lower for p in allowed_cross_context)

                        if not is_allowed:
                            violations.append({
                                'source_context': context,
                                'source': node,
                                'target_context': target_context,
                                'target': target
                            })

        if violations:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'bounded_context_leakage',
            'description': 'Контексты общаются только через DTO/events',
            'status': status,
            'contexts_found': list(contexts.keys()),
            'violations': violations[:20],
            'violations_count': len(violations)
        }
    except Exception as e:
        return {'name': 'bounded_context_leakage', 'status': 'ERROR', 'error': str(e)}


def validate_service_autonomy(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Service Autonomy - сервис не должен критически зависеть от синхронных вызовов других сервисов.
    """
    max_sync_deps = 3
    if config and config.threshold is not None:
        max_sync_deps = int(config.threshold)
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    try:
        violations = []

        sync_patterns = ['client', 'http', 'grpc', 'rpc', 'rest', 'api']
        async_patterns = ['kafka', 'rabbitmq', 'nats', 'event', 'message', 'queue']

        for node in graph.nodes():
            if _is_excluded(node, exclude):
                continue

            sync_deps = []
            async_deps = []

            for _, target in graph.out_edges(node):
                target_lower = target.lower()

                is_sync = any(p in target_lower for p in sync_patterns)
                is_async = any(p in target_lower for p in async_patterns)

                if is_sync and not is_async:
                    sync_deps.append(target)
                elif is_async:
                    async_deps.append(target)

            if len(sync_deps) > max_sync_deps:
                violations.append({
                    'service': node,
                    'sync_deps_count': len(sync_deps),
                    'async_deps_count': len(async_deps),
                    'sync_deps': sync_deps[:5]
                })

        violations.sort(key=lambda x: x['sync_deps_count'], reverse=True)

        if violations:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'service_autonomy',
            'description': f'Сервисы имеют <= {max_sync_deps} синхронных зависимостей',
            'status': status,
            'threshold': max_sync_deps,
            'violations': violations[:10],
            'violations_count': len(violations)
        }
    except Exception as e:
        return {'name': 'service_autonomy', 'status': 'ERROR', 'error': str(e)}
