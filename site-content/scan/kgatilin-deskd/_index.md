---
title: "kgatilin/deskd"
description: "Architecture analysis of kgatilin/deskd"
---

**Repository:** [kgatilin/deskd](https://github.com/kgatilin/deskd)
**Language:** Rust
**Health:** 29% (poor)

## Validation Results

### Core metrics

| Check | Status | Value | Threshold |
|-------|--------|-------|-----------|
| max_fan_out | ERROR |  | 5 |
| coupling | ERROR |  |  |
| instability | ERROR |  |  |
| hub_nodes | ERROR |  | 10 |
| orphan_nodes | ERROR |  |  |
| dag_check | PASSED |  |  |
| layer_violations | PASSED |  |  |
| forbidden_dependencies | PASSED |  |  |
| strongly_connected_components | PASSED |  |  |
| graph_depth | PASSED |  |  |
| component_distance | PASSED |  |  |

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
| gini_coefficient | ERROR |  | 0.6 |
| zscore_outliers | ERROR |  |  |
| change_propagation | ERROR |  | 20 |
| hotspot_detection | ERROR |  | 20 |
| component_complexity | ERROR |  | 50 |
| bridge_edges | INFO |  |  |
| graph_diameter | INFO |  |  |
| closeness_centrality | INFO |  |  |
| degree_distribution | INFO |  |  |
| algebraic_connectivity | INFO |  |  |
| spectral_radius | INFO |  |  |
| betweenness_centrality | PASSED |  | 0.3 |
| pagerank | PASSED |  | 0.1 |
| modularity | PASSED |  | 0.3 |
| edge_density | PASSED |  |  |
| avg_path_length | PASSED |  | 5.0 |
| eigenvector_centrality | PASSED |  | 0.3 |
| k_core_decomposition | PASSED |  | 5 |
| graph_cliques | PASSED |  | 4 |
| dependency_entropy | PASSED |  |  |
| cohesion_lcom4 | PASSED |  |  |
| blast_radius | PASSED |  | 0.3 |
| deprecated_usage | PASSED |  |  |
| stability_violations | PASSED |  |  |
| circular_dependency_depth | PASSED |  | 3 |
| clustering_coefficient | WARNING |  | 0.1 |
| articulation_points | WARNING |  |  |

