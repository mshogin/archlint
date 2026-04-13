---
title: "svenstaro/miniserve"
description: "Architecture analysis of svenstaro/miniserve"
---

**Repository:** [svenstaro/miniserve](https://github.com/svenstaro/miniserve)
**Language:** Rust
**Health:** 39% (poor)

## Validation Results

### Core metrics

| Check | Status | Value | Threshold |
|-------|--------|-------|-----------|
| max_fan_out | ERROR |  | 5 |
| coupling | ERROR |  |  |
| hub_nodes | ERROR |  | 10 |
| orphan_nodes | ERROR |  |  |
| strongly_connected_components | ERROR |  |  |
| dag_check | FAILED |  |  |
| instability | PASSED |  |  |
| layer_violations | PASSED |  |  |
| forbidden_dependencies | PASSED |  |  |
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
| zscore_outliers | ERROR |  |  |
| component_complexity | ERROR |  | 50 |
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
| edge_density | PASSED |  |  |
| avg_path_length | PASSED |  | 5.0 |
| k_core_decomposition | PASSED |  | 5 |
| graph_cliques | PASSED |  | 4 |
| dependency_entropy | PASSED |  |  |
| gini_coefficient | PASSED |  | 0.6 |
| cohesion_lcom4 | PASSED |  |  |
| change_propagation | PASSED |  | 20 |
| blast_radius | PASSED |  | 0.3 |
| hotspot_detection | PASSED |  | 20 |
| deprecated_usage | PASSED |  |  |
| stability_violations | PASSED |  |  |
| circular_dependency_depth | PASSED |  | 3 |
| clustering_coefficient | WARNING |  | 0.1 |
| articulation_points | WARNING |  |  |

