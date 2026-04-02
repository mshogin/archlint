"""
SOLID Principles Validations

- Single Responsibility Principle (SRP)
- Open/Closed Principle (OCP)
- Liskov Substitution Principle (LSP)
- Interface Segregation Principle (ISP) - already in advanced_metrics
- Dependency Inversion Principle (DIP)
"""

import networkx as nx
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


def validate_single_responsibility(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Single Responsibility Principle - каждый модуль должен иметь одну причину для изменения.

    Метрика: анализ количества разных "доменов" которые использует/от которых зависит модуль.
    Если модуль зависит от слишком разных доменов - нарушение SRP.
    """
    max_responsibilities = 3
    if config and config.threshold is not None:
        max_responsibilities = int(config.threshold)
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    try:
        violations = []

        # Группируем узлы по пакетам/модулям
        packages: Dict[str, List[str]] = defaultdict(list)
        for node in graph.nodes():
            if _is_excluded(node, exclude):
                continue
            # Извлекаем пакет (первые 2 части пути)
            parts = node.split('/')
            if len(parts) >= 2:
                pkg = '/'.join(parts[:2])
            else:
                pkg = parts[0] if parts else node
            packages[pkg].append(node)

        for pkg, nodes in packages.items():
            # Собираем все зависимости пакета
            dependencies = set()
            for node in nodes:
                for _, target in graph.out_edges(node):
                    # Извлекаем домен зависимости
                    target_parts = target.split('/')
                    if len(target_parts) >= 2:
                        dep_domain = target_parts[0]
                    else:
                        dep_domain = target
                    dependencies.add(dep_domain)

            # Исключаем стандартные библиотеки и собственный домен
            own_domain = pkg.split('/')[0] if '/' in pkg else pkg
            external_domains = [d for d in dependencies
                               if d != own_domain
                               and not d.startswith('go/')
                               and not d.startswith('std/')]

            if len(external_domains) > max_responsibilities:
                violations.append({
                    'package': pkg,
                    'domains_count': len(external_domains),
                    'domains': external_domains[:10]
                })

        violations.sort(key=lambda x: x['domains_count'], reverse=True)

        if violations:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'single_responsibility',
            'description': f'Модули зависят от <= {max_responsibilities} разных доменов (SRP)',
            'status': status,
            'threshold': max_responsibilities,
            'violations': violations[:10],
            'violations_count': len(violations)
        }
    except Exception as e:
        return {'name': 'single_responsibility', 'status': 'ERROR', 'error': str(e)}


def validate_open_closed(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Open/Closed Principle - модуль открыт для расширения, закрыт для модификации.

    Метрика: соотношение абстракций (интерфейсов) к конкретным реализациям.
    Высокое соотношение = хорошая расширяемость.
    """
    min_ratio = 0.2
    if config and config.threshold is not None:
        min_ratio = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        abstractions = 0
        implementations = 0

        abstract_patterns = ['interface', 'abstract', 'base', 'contract', 'port', 'spec']

        for node in graph.nodes():
            if _is_excluded(node, exclude):
                continue

            node_data = graph.nodes[node]
            entity = node_data.get('entity', '')
            node_lower = node.lower()

            # Проверяем, является ли узел абстракцией
            is_abstract = (
                entity == 'interface' or
                any(p in node_lower for p in abstract_patterns)
            )

            if is_abstract:
                abstractions += 1
            else:
                implementations += 1

        total = abstractions + implementations
        ratio = abstractions / total if total > 0 else 0

        if ratio < min_ratio and total > 5:
            status = _get_violation_status(error_on_violation)
            issue = f'Низкое соотношение абстракций ({ratio:.2%}) - плохая расширяемость'
        else:
            status = 'PASSED'
            issue = None

        result = {
            'name': 'open_closed',
            'description': f'Соотношение абстракций >= {min_ratio:.0%} (OCP)',
            'status': status,
            'abstractions': abstractions,
            'implementations': implementations,
            'ratio': round(ratio, 3),
            'threshold': min_ratio
        }
        if issue:
            result['issue'] = issue
        return result
    except Exception as e:
        return {'name': 'open_closed', 'status': 'ERROR', 'error': str(e)}


def validate_liskov_substitution(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Liskov Substitution Principle - подтипы должны быть заменяемы базовыми типами.

    Метрика: проверка что реализации интерфейсов не добавляют лишних зависимостей.
    """
    max_extra_deps = 3
    if config and config.threshold is not None:
        max_extra_deps = int(config.threshold)
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else False

    try:
        violations = []

        # Находим интерфейсы и их реализации
        interfaces: Dict[str, set] = {}
        implementations: Dict[str, set] = {}

        for node in graph.nodes():
            if _is_excluded(node, exclude):
                continue

            node_data = graph.nodes[node]

            # Собираем зависимости узла
            deps = set(target for _, target in graph.out_edges(node))

            if node_data.get('entity') == 'interface' or 'interface' in node.lower():
                interfaces[node] = deps
            else:
                # Ищем реализуемые интерфейсы через edge type
                for _, target, data in graph.out_edges(node, data=True):
                    if data.get('type') == 'implements':
                        if target not in implementations:
                            implementations[target] = {}
                        implementations[target][node] = deps

        # Проверяем что реализации не добавляют много лишних зависимостей
        for interface, impls in implementations.items():
            if interface not in interfaces:
                continue

            interface_deps = interfaces[interface]

            for impl, impl_deps in impls.items():
                extra_deps = impl_deps - interface_deps
                if len(extra_deps) > max_extra_deps:
                    violations.append({
                        'implementation': impl,
                        'interface': interface,
                        'extra_deps_count': len(extra_deps),
                        'extra_deps': list(extra_deps)[:5]
                    })

        if violations:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'liskov_substitution',
            'description': f'Реализации не добавляют > {max_extra_deps} лишних зависимостей (LSP)',
            'status': status,
            'threshold': max_extra_deps,
            'violations': violations[:10],
            'violations_count': len(violations),
            'interfaces_analyzed': len(interfaces),
            'implementations_analyzed': sum(len(v) for v in implementations.values())
        }
    except Exception as e:
        return {'name': 'liskov_substitution', 'status': 'ERROR', 'error': str(e)}


def validate_dependency_inversion(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Dependency Inversion Principle - зависимость от абстракций, не от конкретики.

    Метрика: процент зависимостей на абстракции vs конкретные реализации.
    """
    min_abstract_ratio = 0.3
    if config and config.threshold is not None:
        min_abstract_ratio = config.threshold
    exclude = config.exclude if config else []
    error_on_violation = config.error_on_violation if config else True

    try:
        abstract_deps = 0
        concrete_deps = 0
        violations = []

        abstract_patterns = ['interface', 'abstract', 'base', 'contract', 'port', 'spec']
        concrete_patterns = ['impl', 'concrete', 'adapter', 'repository', 'service', 'handler']

        for source in graph.nodes():
            if _is_excluded(source, exclude):
                continue

            source_data = graph.nodes[source]
            # Пропускаем infrastructure/adapter слои - им можно зависеть от конкретики
            if any(p in source.lower() for p in ['adapter', 'infrastructure', 'impl']):
                continue

            source_concrete_deps = []

            for _, target in graph.out_edges(source):
                target_data = graph.nodes.get(target, {})
                target_lower = target.lower()

                is_abstract = (
                    target_data.get('entity') == 'interface' or
                    any(p in target_lower for p in abstract_patterns)
                )

                is_concrete = any(p in target_lower for p in concrete_patterns)

                if is_abstract:
                    abstract_deps += 1
                elif is_concrete:
                    concrete_deps += 1
                    source_concrete_deps.append(target)

            if source_concrete_deps:
                violations.append({
                    'source': source,
                    'concrete_deps': source_concrete_deps[:5],
                    'count': len(source_concrete_deps)
                })

        total = abstract_deps + concrete_deps
        ratio = abstract_deps / total if total > 0 else 1.0

        violations.sort(key=lambda x: x['count'], reverse=True)

        if ratio < min_abstract_ratio and total > 5:
            status = _get_violation_status(error_on_violation)
        else:
            status = 'PASSED'

        return {
            'name': 'dependency_inversion',
            'description': f'Зависимости на абстракции >= {min_abstract_ratio:.0%} (DIP)',
            'status': status,
            'abstract_deps': abstract_deps,
            'concrete_deps': concrete_deps,
            'abstract_ratio': round(ratio, 3),
            'threshold': min_abstract_ratio,
            'violations': violations[:10],
            'violations_count': len(violations)
        }
    except Exception as e:
        return {'name': 'dependency_inversion', 'status': 'ERROR', 'error': str(e)}
