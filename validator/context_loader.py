"""
Загрузчик контекстов из YAML файлов (формат DocHub)
"""

import yaml
import fnmatch
from typing import Dict, List, Any, Optional, Union
from dataclasses import dataclass, field


@dataclass
class UMLConfig:
    """Конфигурация UML диаграммы"""
    before: str = ""      # PlantUML код перед диаграммой ($before)
    after: str = ""       # PlantUML код после диаграммы ($after)
    file: str = ""        # Путь к внешнему .puml файлу


@dataclass
class Context:
    """Контекст выполнения в формате DocHub"""
    id: str
    title: str
    location: str = ""
    presentation: str = "plantuml"  # plantuml, smartants
    extra_links: bool = False       # Показывать связанные компоненты
    components: List[str] = field(default_factory=list)
    uml: Optional[UMLConfig] = None


def _parse_uml_config(uml_data: Any) -> Optional[UMLConfig]:
    """Парсит конфигурацию UML из YAML"""
    if uml_data is None:
        return None

    if isinstance(uml_data, str):
        # Простой путь к файлу
        return UMLConfig(file=uml_data)

    if isinstance(uml_data, dict):
        return UMLConfig(
            before=uml_data.get('$before', ''),
            after=uml_data.get('$after', ''),
            file=uml_data.get('file', '')
        )

    return None


def load_contexts(filepath: str) -> Dict[str, Context]:
    """
    Загружает контексты из YAML файла в формате DocHub.

    Args:
        filepath: Путь к файлу contexts.yaml

    Returns:
        Словарь контекстов {id: Context}
    """
    with open(filepath, 'r', encoding='utf-8') as f:
        data = yaml.safe_load(f)

    contexts = {}
    contexts_data = data.get('contexts', {})

    for ctx_id, ctx_data in contexts_data.items():
        contexts[ctx_id] = Context(
            id=ctx_id,
            title=ctx_data.get('title', ''),
            location=ctx_data.get('location', ''),
            presentation=ctx_data.get('presentation', 'plantuml'),
            extra_links=ctx_data.get('extra-links', False),
            components=ctx_data.get('components', []),
            uml=_parse_uml_config(ctx_data.get('uml'))
        )

    return contexts


def match_component_pattern(component_id: str, pattern: str) -> bool:
    """
    Проверяет соответствие компонента паттерну с поддержкой wildcards.

    Паттерны:
        - "component.id" - точное совпадение
        - "component.*" - все прямые потомки
        - "component.**" - все потомки рекурсивно

    Args:
        component_id: ID компонента (например, "order_service.process_order")
        pattern: Паттерн для сопоставления

    Returns:
        True если компонент соответствует паттерну
    """
    # Точное совпадение
    if component_id == pattern:
        return True

    # Паттерн с ** (рекурсивный wildcard)
    if pattern.endswith('.**'):
        prefix = pattern[:-3]  # Убираем ".**"
        return component_id.startswith(prefix + '.')

    # Паттерн с * (одноуровневый wildcard)
    if pattern.endswith('.*'):
        prefix = pattern[:-2]  # Убираем ".*"
        if not component_id.startswith(prefix + '.'):
            return False
        # Проверяем что после префикса нет больше точек
        suffix = component_id[len(prefix) + 1:]
        return '.' not in suffix

    return False


def expand_component_patterns(patterns: List[str], all_components: List[str]) -> List[str]:
    """
    Раскрывает паттерны компонентов в список конкретных компонентов.

    Args:
        patterns: Список паттернов (могут содержать * и **)
        all_components: Список всех доступных компонентов

    Returns:
        Список компонентов, соответствующих паттернам
    """
    result = set()

    for pattern in patterns:
        for comp in all_components:
            if match_component_pattern(comp, pattern):
                result.add(comp)

    return list(result)


def get_all_components_from_contexts(contexts: Dict[str, Context]) -> set:
    """Возвращает все уникальные компоненты из всех контекстов (без раскрытия паттернов)"""
    all_components = set()
    for ctx in contexts.values():
        all_components.update(ctx.components)
    return all_components


def get_expanded_components_from_context(
    ctx: Context,
    all_architecture_components: List[str]
) -> List[str]:
    """
    Возвращает раскрытый список компонентов контекста с учетом wildcards.

    Args:
        ctx: Контекст
        all_architecture_components: Все компоненты из архитектуры

    Returns:
        Список конкретных компонентов (без wildcards)
    """
    return expand_component_patterns(ctx.components, all_architecture_components)


def get_component_context_map(contexts: Dict[str, Context]) -> Dict[str, List[str]]:
    """
    Создает маппинг: компонент -> список контекстов, в которых он участвует.
    Примечание: Не раскрывает wildcards, работает с сырыми паттернами.
    """
    component_map: Dict[str, List[str]] = {}

    for ctx_id, ctx in contexts.items():
        for component in ctx.components:
            if component not in component_map:
                component_map[component] = []
            component_map[component].append(ctx_id)

    return component_map


def get_component_context_map_expanded(
    contexts: Dict[str, Context],
    all_architecture_components: List[str]
) -> Dict[str, List[str]]:
    """
    Создает маппинг: компонент -> список контекстов с раскрытыми wildcards.

    Args:
        contexts: Словарь контекстов
        all_architecture_components: Все компоненты из архитектуры

    Returns:
        Маппинг компонент -> список контекстов
    """
    component_map: Dict[str, List[str]] = {}

    for ctx_id, ctx in contexts.items():
        expanded = expand_component_patterns(ctx.components, all_architecture_components)
        for component in expanded:
            if component not in component_map:
                component_map[component] = []
            component_map[component].append(ctx_id)

    return component_map
