"""
Security, Testability, Observability & API Quality Validations
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


# =============================================================================
# SECURITY VALIDATIONS
# =============================================================================

def validate_auth_boundary(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Auth Boundary - аутентификация должна быть на границе системы.
    """
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    try:
        violations = []

        auth_patterns = ['auth', 'authentication', 'jwt', 'oauth', 'token', 'session']
        boundary_patterns = ['handler', 'controller', 'api', 'endpoint', 'middleware', 'interceptor']
        inner_patterns = ['service', 'usecase', 'domain', 'repository', 'entity']

        for node in graph.nodes():
            if _is_excluded(node, exclude):
                continue

            node_lower = node.lower()

            # Проверяем, содержит ли внутренний компонент auth логику
            is_inner = any(p in node_lower for p in inner_patterns)
            is_boundary = any(p in node_lower for p in boundary_patterns)
            has_auth = any(p in node_lower for p in auth_patterns)

            if is_inner and has_auth and not is_boundary:
                violations.append({
                    'component': node,
                    'issue': 'Auth logic should be at boundary layer'
                })

        if violations:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'auth_boundary',
            'description': 'Аутентификация на границе системы',
            'status': status,
            'violations': violations[:20],
            'violations_count': len(violations)
        }
    except Exception as e:
        return {'name': 'auth_boundary', 'status': 'ERROR', 'error': str(e)}


def validate_sensitive_data_flow(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Sensitive Data Flow - отслеживание потока чувствительных данных.
    """
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    sensitive_patterns = ['password', 'secret', 'credential', 'token', 'key', 'private', 'ssn', 'credit']
    unsafe_patterns = ['log', 'print', 'debug', 'trace', 'cache', 'redis', 'memcache']

    if config and config.params:
        sensitive_patterns = config.params.get('sensitive_patterns', sensitive_patterns)
        unsafe_patterns = config.params.get('unsafe_patterns', unsafe_patterns)

    try:
        violations = []

        sensitive_nodes = set()

        # Находим компоненты с чувствительными данными
        for node in graph.nodes():
            node_lower = node.lower()
            if any(p in node_lower for p in sensitive_patterns):
                sensitive_nodes.add(node)

        # Проверяем, куда текут чувствительные данные
        for sensitive in sensitive_nodes:
            # Получаем все достижимые узлы
            reachable = nx.descendants(graph, sensitive)

            for target in reachable:
                target_lower = target.lower()
                if any(p in target_lower for p in unsafe_patterns):
                    if not _is_excluded(target, exclude):
                        violations.append({
                            'sensitive_source': sensitive,
                            'unsafe_target': target
                        })

        if violations:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'sensitive_data_flow',
            'description': 'Чувствительные данные не попадают в небезопасные места',
            'status': status,
            'sensitive_components': list(sensitive_nodes)[:10],
            'violations': violations[:20],
            'violations_count': len(violations)
        }
    except Exception as e:
        return {'name': 'sensitive_data_flow', 'status': 'ERROR', 'error': str(e)}


def validate_input_validation_layer(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Input Validation Layer - валидация входных данных на границе.
    """
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        info = []

        validation_patterns = ['valid', 'sanitiz', 'check', 'verify', 'parse']
        boundary_patterns = ['handler', 'controller', 'api', 'endpoint']

        boundary_with_validation = 0
        boundary_without_validation = 0

        for node in graph.nodes():
            if _is_excluded(node, exclude):
                continue

            node_lower = node.lower()

            is_boundary = any(p in node_lower for p in boundary_patterns)
            if not is_boundary:
                continue

            # Проверяем, есть ли валидация в зависимостях
            has_validation = False
            for _, target in graph.out_edges(node):
                if any(p in target.lower() for p in validation_patterns):
                    has_validation = True
                    break

            if has_validation:
                boundary_with_validation += 1
            else:
                boundary_without_validation += 1
                info.append({
                    'boundary': node,
                    'issue': 'No validation dependency found'
                })

        total = boundary_with_validation + boundary_without_validation
        coverage = boundary_with_validation / total if total > 0 else 1.0

        return {
            'name': 'input_validation_layer',
            'description': 'Границы системы имеют валидацию входных данных',
            'status': 'INFO',
            'boundaries_total': total,
            'with_validation': boundary_with_validation,
            'without_validation': boundary_without_validation,
            'coverage': round(coverage, 2),
            'without_validation_list': info[:10]
        }
    except Exception as e:
        return {'name': 'input_validation_layer', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# TESTABILITY VALIDATIONS
# =============================================================================

def validate_mockability(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Mockability - зависимости можно замокать (через интерфейсы).
    """
    min_interface_ratio = 0.3
    if config and config.threshold is not None:
        min_interface_ratio = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        interface_deps = 0
        concrete_deps = 0

        interface_patterns = ['interface', 'contract', 'port', 'spec']

        for node in graph.nodes():
            if _is_excluded(node, exclude):
                continue

            for _, target in graph.out_edges(node):
                target_data = graph.nodes.get(target, {})
                target_lower = target.lower()

                is_interface = (
                    target_data.get('entity') == 'interface' or
                    any(p in target_lower for p in interface_patterns)
                )

                if is_interface:
                    interface_deps += 1
                else:
                    concrete_deps += 1

        total = interface_deps + concrete_deps
        ratio = interface_deps / total if total > 0 else 0

        if ratio < min_interface_ratio:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'mockability',
            'description': f'Зависимости на интерфейсы >= {min_interface_ratio:.0%}',
            'status': status,
            'interface_deps': interface_deps,
            'concrete_deps': concrete_deps,
            'ratio': round(ratio, 3),
            'threshold': min_interface_ratio
        }
    except Exception as e:
        return {'name': 'mockability', 'status': 'ERROR', 'error': str(e)}


def validate_test_isolation(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Test Isolation - тестовые компоненты изолированы друг от друга.
    """
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        violations = []

        test_patterns = ['test', 'spec', 'mock', 'fake', 'stub']

        test_nodes = set()
        for node in graph.nodes():
            if any(p in node.lower() for p in test_patterns):
                test_nodes.add(node)

        # Проверяем зависимости между тестами
        for test in test_nodes:
            if _is_excluded(test, exclude):
                continue

            for _, target in graph.out_edges(test):
                if target in test_nodes and target != test:
                    violations.append({
                        'test': test,
                        'depends_on_test': target
                    })

        if violations:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'test_isolation',
            'description': 'Тесты изолированы друг от друга',
            'status': status,
            'test_components': len(test_nodes),
            'violations': violations[:20],
            'violations_count': len(violations)
        }
    except Exception as e:
        return {'name': 'test_isolation', 'status': 'ERROR', 'error': str(e)}


def validate_test_coverage_structure(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Test Coverage Structure - структурное покрытие тестами.
    """
    min_coverage = 0.5
    if config and config.threshold is not None:
        min_coverage = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        test_patterns = ['test', 'spec']
        production_nodes = set()
        tested_nodes = set()

        for node in graph.nodes():
            if _is_excluded(node, exclude):
                continue

            node_lower = node.lower()
            is_test = any(p in node_lower for p in test_patterns)

            if is_test:
                # Найти что тестирует этот тест
                for _, target in graph.out_edges(node):
                    if not any(p in target.lower() for p in test_patterns):
                        tested_nodes.add(target)
            else:
                production_nodes.add(node)

        coverage = len(tested_nodes) / len(production_nodes) if production_nodes else 1.0
        untested = production_nodes - tested_nodes

        if coverage < min_coverage:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'test_coverage_structure',
            'description': f'Структурное покрытие тестами >= {min_coverage:.0%}',
            'status': status,
            'production_components': len(production_nodes),
            'tested_components': len(tested_nodes),
            'coverage': round(coverage, 3),
            'threshold': min_coverage,
            'untested_sample': list(untested)[:10]
        }
    except Exception as e:
        return {'name': 'test_coverage_structure', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# OBSERVABILITY VALIDATIONS
# =============================================================================

def validate_logging_coverage(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Logging Coverage - покрытие логированием критических компонентов.
    """
    min_coverage = 0.5
    if config and config.threshold is not None:
        min_coverage = config.threshold
    exclude = config.exclude if config else []

    try:
        logging_patterns = ['log', 'logger', 'logging', 'zap', 'logrus', 'slog']
        critical_patterns = ['handler', 'service', 'usecase', 'repository']

        critical_nodes = set()
        nodes_with_logging = set()

        for node in graph.nodes():
            if _is_excluded(node, exclude):
                continue

            node_lower = node.lower()

            is_critical = any(p in node_lower for p in critical_patterns)
            if not is_critical:
                continue

            critical_nodes.add(node)

            # Проверяем, использует ли логирование
            for _, target in graph.out_edges(node):
                if any(p in target.lower() for p in logging_patterns):
                    nodes_with_logging.add(node)
                    break

        coverage = len(nodes_with_logging) / len(critical_nodes) if critical_nodes else 1.0

        return {
            'name': 'logging_coverage',
            'description': f'Критические компоненты используют логирование >= {min_coverage:.0%}',
            'status': 'PASSED' if coverage >= min_coverage else 'INFO',
            'critical_components': len(critical_nodes),
            'with_logging': len(nodes_with_logging),
            'coverage': round(coverage, 3),
            'threshold': min_coverage
        }
    except Exception as e:
        return {'name': 'logging_coverage', 'status': 'ERROR', 'error': str(e)}


def validate_metrics_exposure(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Metrics Exposure - экспозиция метрик для мониторинга.
    """
    exclude = config.exclude if config else []

    try:
        metrics_patterns = ['metric', 'prometheus', 'statsd', 'datadog', 'counter', 'gauge', 'histogram']

        nodes_with_metrics = set()

        for node in graph.nodes():
            if _is_excluded(node, exclude):
                continue

            for _, target in graph.out_edges(node):
                if any(p in target.lower() for p in metrics_patterns):
                    nodes_with_metrics.add(node)
                    break

        return {
            'name': 'metrics_exposure',
            'description': 'Компоненты экспортируют метрики',
            'status': 'INFO',
            'components_with_metrics': len(nodes_with_metrics),
            'total_components': len(graph.nodes()),
            'sample': list(nodes_with_metrics)[:10]
        }
    except Exception as e:
        return {'name': 'metrics_exposure', 'status': 'ERROR', 'error': str(e)}


def validate_health_check_presence(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Health Check Presence - наличие health check компонентов.
    """
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        health_patterns = ['health', 'ready', 'live', 'readiness', 'liveness', 'probe']

        health_nodes = []

        for node in graph.nodes():
            if _is_excluded(node, exclude):
                continue

            if any(p in node.lower() for p in health_patterns):
                health_nodes.append(node)

        has_health = len(health_nodes) > 0

        if has_health:
            status = 'PASSED'
        else:
            status = _get_violation_status(error_on_violation)

        return {
            'name': 'health_check_presence',
            'description': 'Наличие health check компонентов',
            'status': status,
            'health_components': health_nodes,
            'count': len(health_nodes)
        }
    except Exception as e:
        return {'name': 'health_check_presence', 'status': 'ERROR', 'error': str(e)}


# =============================================================================
# API QUALITY VALIDATIONS
# =============================================================================

def validate_api_consistency(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    API Consistency - консистентность API компонентов.
    """
    exclude = config.exclude if config else []

    try:
        api_patterns = ['handler', 'controller', 'endpoint', 'api', 'resource']

        api_nodes = []
        naming_issues = []

        for node in graph.nodes():
            if _is_excluded(node, exclude):
                continue

            node_lower = node.lower()

            if any(p in node_lower for p in api_patterns):
                api_nodes.append(node)

                # Проверяем консистентность именования
                name = node.split('/')[-1].split('.')[-1]

                # Проверки
                if name[0].islower() and 'Handler' in name:
                    naming_issues.append({
                        'component': node,
                        'issue': 'Inconsistent casing'
                    })

        return {
            'name': 'api_consistency',
            'description': 'Консистентность API компонентов',
            'status': 'INFO',
            'api_components': len(api_nodes),
            'naming_issues': naming_issues[:10],
            'naming_issues_count': len(naming_issues)
        }
    except Exception as e:
        return {'name': 'api_consistency', 'status': 'ERROR', 'error': str(e)}
