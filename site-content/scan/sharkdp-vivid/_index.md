---
title: "sharkdp/vivid"
description: "Architecture analysis of sharkdp/vivid"
---

**Repository:** [sharkdp/vivid](https://github.com/sharkdp/vivid)
**Language:** Rust
**Health:** 51% (needs work)

## Validation Results

### Core metrics

| Check | Status | Value | Threshold |
|-------|--------|-------|-----------|
| max_fan_out | ERROR |  | 5 |
| coupling | ERROR |  |  |
| hub_nodes | ERROR |  | 10 |
| orphan_nodes | ERROR |  |  |
| dag_check | FAILED |  |  |
| instability | PASSED |  |  |
| layer_violations | PASSED |  |  |
| forbidden_dependencies | PASSED |  |  |
| strongly_connected_components | PASSED |  |  |
| component_distance | PASSED |  |  |
| graph_depth | SKIP |  |  |

### Solid metrics

| Check | Status | Value | Threshold |
|-------|--------|-------|-----------|
| single_responsibility | ERROR |  | 3 |
| liskov_substitution | PASSED |  | 3 |
| dependency_inversion | PASSED |  | 0.3 |
| interface_segregation | PASSED |  | 5 |
| open_closed | WARNING |  | 0.2 |

### Advanced metrics

| Check | Status | Value | Threshold |
|-------|--------|-------|-----------|
| abstractness | ERROR |  |  |
| distance_from_main_sequence | ERROR |  |  |
| k_core_decomposition | ERROR |  |  |
| pagerank | INFO |  | 0.1 |
| bridge_edges | INFO |  |  |
| graph_diameter | INFO |  |  |
| closeness_centrality | INFO |  |  |
| eigenvector_centrality | INFO |  | 0.3 |
| degree_distribution | INFO |  |  |
| algebraic_connectivity | INFO |  |  |
| spectral_radius | INFO |  |  |
| betweenness_centrality | PASSED |  | 0.3 |
| modularity | PASSED |  | 0.3 |
| clustering_coefficient | PASSED |  | 0.1 |
| edge_density | PASSED |  |  |
| avg_path_length | PASSED |  | 5.0 |
| graph_cliques | PASSED |  | 4 |
| dependency_entropy | PASSED |  |  |
| gini_coefficient | PASSED |  | 0.6 |
| zscore_outliers | PASSED |  |  |
| cohesion_lcom4 | PASSED |  |  |
| change_propagation | PASSED |  | 20 |
| blast_radius | PASSED |  | 0.3 |
| hotspot_detection | PASSED |  | 20 |
| deprecated_usage | PASSED |  |  |
| stability_violations | PASSED |  |  |
| circular_dependency_depth | PASSED |  | 3 |
| component_complexity | PASSED |  | 50 |
| articulation_points | WARNING |  |  |

