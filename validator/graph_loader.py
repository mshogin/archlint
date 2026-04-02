"""
Загрузчик графов из YAML форматов (DocHub и archlint)
"""

import yaml
import networkx as nx
from typing import Dict, Any, List, Union


class GraphLoader:
    """Загружает граф из YAML формата (DocHub или archlint)"""

    def load_yaml(self, filename: str) -> nx.DiGraph:
        """
        Загружает граф из YAML файла

        Args:
            filename: Путь к YAML файлу

        Returns:
            Направленный граф NetworkX
        """
        with open(filename, 'r', encoding='utf-8') as f:
            data = yaml.safe_load(f)

        return self.parse_yaml(data)

    def parse_yaml(self, data: Dict[str, Any]) -> nx.DiGraph:
        """
        Парсит YAML в граф NetworkX (автоопределение формата)

        Args:
            data: Словарь с данными из YAML

        Returns:
            Направленный граф
        """
        components = data.get('components', [])
        links = data.get('links', [])

        # Определяем формат по типу components
        if isinstance(components, list):
            return self._parse_archlint_format(data)
        else:
            return self._parse_dochub_format(data)

    def _parse_archlint_format(self, data: Dict[str, Any]) -> nx.DiGraph:
        """
        Парсит archlint формат:
        components: [{id, title, entity}, ...]
        links: [{from, to, type}, ...]
        """
        graph = nx.DiGraph()

        # Добавляем узлы
        components = data.get('components', [])
        for comp in components:
            if isinstance(comp, dict):
                comp_id = comp.get('id', '')
                if comp_id:
                    graph.add_node(
                        comp_id,
                        title=comp.get('title', ''),
                        entity=comp.get('entity', ''),
                        properties=comp.get('properties', {})
                    )

        # Добавляем рёбра
        links = data.get('links', [])
        for link in links:
            if isinstance(link, dict):
                from_id = link.get('from', '')
                to_id = link.get('to', '')
                if from_id and to_id:
                    graph.add_edge(
                        from_id,
                        to_id,
                        type=link.get('type', ''),
                        method=link.get('method', ''),
                        properties=link.get('properties', {})
                    )

        return graph

    def _parse_dochub_format(self, data: Dict[str, Any]) -> nx.DiGraph:
        """
        Парсит DocHub формат:
        components: {id: {title, entity}, ...}
        links: {from_id: [{to, type}, ...], ...}
        """
        graph = nx.DiGraph()

        # Добавляем узлы (components)
        components = data.get('components', {})
        for comp_id, comp_data in components.items():
            if isinstance(comp_data, dict):
                graph.add_node(
                    comp_id,
                    title=comp_data.get('title', ''),
                    entity=comp_data.get('entity', ''),
                    properties=comp_data.get('properties', {})
                )

        # Добавляем рёбра (links)
        links = data.get('links', {})
        for from_id, link_list in links.items():
            if isinstance(link_list, list):
                for link in link_list:
                    to_id = link.get('to')
                    if to_id:
                        graph.add_edge(
                            from_id,
                            to_id,
                            method=link.get('method', ''),
                            type=link.get('type', ''),
                            properties=link.get('properties', {})
                        )

        return graph
