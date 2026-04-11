"""
Загрузчик графов из YAML форматов (DocHub, archlint и callgraph)
"""

import yaml
import networkx as nx
from typing import Dict, Any, List, Union, Tuple


# Graph source type constants
SOURCE_STRUCTURE = 'structure'
SOURCE_BEHAVIOR = 'behavior'


class GraphLoader:
    """Загружает граф из YAML формата (DocHub, archlint или callgraph)"""

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

    def load_yaml_with_source(self, filename: str) -> Tuple[nx.DiGraph, str]:
        """
        Загружает граф из YAML файла и возвращает тип источника

        Args:
            filename: Путь к YAML файлу

        Returns:
            Tuple (граф NetworkX, тип источника: 'structure' или 'behavior')
        """
        with open(filename, 'r', encoding='utf-8') as f:
            data = yaml.safe_load(f)

        source = self.detect_source(data)
        graph = self.parse_yaml(data)
        return graph, source

    def detect_source(self, data: Dict[str, Any]) -> str:
        """
        Определяет тип источника: structural (architecture.yaml) или behavioral (callgraph.yaml)

        Callgraph format has 'nodes' and 'edges' with optional 'entry_point'.
        Structural format has 'components' and 'links'.

        Args:
            data: Словарь с данными из YAML

        Returns:
            'behavior' для callgraph, 'structure' для architecture
        """
        if 'nodes' in data and 'edges' in data:
            # Additional check: callgraph nodes have 'package'/'function' fields
            nodes = data.get('nodes', [])
            if isinstance(nodes, list) and nodes:
                first_node = nodes[0] if isinstance(nodes[0], dict) else {}
                if 'package' in first_node or 'function' in first_node or 'entry_point' in data:
                    return SOURCE_BEHAVIOR
        return SOURCE_STRUCTURE

    def parse_yaml(self, data: Dict[str, Any]) -> nx.DiGraph:
        """
        Парсит YAML в граф NetworkX (автоопределение формата)

        Args:
            data: Словарь с данными из YAML

        Returns:
            Направленный граф
        """
        # Check for callgraph format first (nodes/edges with entry_point or function fields)
        if self.detect_source(data) == SOURCE_BEHAVIOR:
            return self._parse_callgraph_format(data)

        components = data.get('components', [])

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

    def _parse_callgraph_format(self, data: Dict[str, Any]) -> nx.DiGraph:
        """
        Парсит callgraph формат (из `archlint callgraph`):
        entry_point: "pkg.Type.Method"
        nodes:
          - id: ..., package: ..., function: ..., type: method|function|external
        edges:
          - from: ..., to: ..., call_type: direct|deferred|goroutine
        stats: {...}

        Nodes are mapped to graph nodes preserving function/package metadata.
        Entry point is stored as graph attribute.
        """
        graph = nx.DiGraph()

        entry_point = data.get('entry_point', '')
        if entry_point:
            graph.graph['entry_point'] = entry_point

        # Add nodes
        nodes = data.get('nodes', [])
        for node in nodes:
            if isinstance(node, dict):
                node_id = node.get('id', '')
                if node_id:
                    graph.add_node(
                        node_id,
                        package=node.get('package', ''),
                        function=node.get('function', ''),
                        receiver=node.get('receiver', ''),
                        type=node.get('type', ''),
                        file=node.get('file', ''),
                        line=node.get('line', 0),
                        depth=node.get('depth', 0),
                        # Map to structural field names for validator compatibility
                        title=node.get('function', node_id),
                        entity=node.get('type', ''),
                    )

        # Add edges preserving order
        edges = data.get('edges', [])
        for edge in edges:
            if isinstance(edge, dict):
                from_id = edge.get('from', '')
                to_id = edge.get('to', '')
                if from_id and to_id:
                    # Ensure nodes exist even if not declared in nodes list
                    if from_id not in graph.nodes:
                        graph.add_node(from_id, title=from_id, entity='unknown')
                    if to_id not in graph.nodes:
                        graph.add_node(to_id, title=to_id, entity='unknown')
                    graph.add_edge(
                        from_id,
                        to_id,
                        call_type=edge.get('call_type', 'direct'),
                        line=edge.get('line', 0),
                        # Map to structural field for validator compatibility
                        type=edge.get('call_type', 'direct'),
                    )

        # Store stats as graph metadata
        stats = data.get('stats', {})
        if stats:
            graph.graph['stats'] = stats

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
