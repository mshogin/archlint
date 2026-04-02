"""
Core structure validators - Maximum value, minimum time

Validators:
- dag_check: Cycle detection (critical!)
- max_fan_out: Too many outgoing dependencies
- coupling: Ca/Ce metrics
- instability: Dependency direction (I = Ce/(Ca+Ce))
- layer_violations: Layer architecture violations
- forbidden_dependencies: Prohibited dependencies
- hub_nodes: God Objects (too many connections)
- orphan_nodes: Isolated components (dead code)
- strongly_connected_components: Mutual dependencies
- graph_depth: Dependency chain depth
"""

from validator.structure.core.metrics import (
    validate_dag,
    validate_max_fan_out,
    validate_coupling,
    validate_instability,
    validate_layer_violations,
    validate_forbidden_dependencies,
    validate_hub_nodes,
    validate_orphan_nodes,
    validate_strongly_connected_components,
    validate_graph_depth,
    validate_component_distance,
    # Also export centrality metrics for use by advanced group
    validate_betweenness_centrality,
    validate_pagerank,
    validate_modularity,
    validate_abstractness,
    validate_distance_from_main_sequence,
)

__all__ = [
    'validate_dag',
    'validate_max_fan_out',
    'validate_coupling',
    'validate_instability',
    'validate_layer_violations',
    'validate_forbidden_dependencies',
    'validate_hub_nodes',
    'validate_orphan_nodes',
    'validate_strongly_connected_components',
    'validate_graph_depth',
    'validate_component_distance',
    'validate_betweenness_centrality',
    'validate_pagerank',
    'validate_modularity',
    'validate_abstractness',
    'validate_distance_from_main_sequence',
]

# Core validators list for quick access
CORE_VALIDATORS = [
    validate_dag,
    validate_max_fan_out,
    validate_coupling,
    validate_instability,
    validate_layer_violations,
    validate_forbidden_dependencies,
    validate_hub_nodes,
    validate_orphan_nodes,
    validate_strongly_connected_components,
    validate_graph_depth,
]
