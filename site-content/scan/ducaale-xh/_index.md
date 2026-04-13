---
title: "ducaale/xh"
description: "Architecture analysis of ducaale/xh"
---

**Repository:** [ducaale/xh](https://github.com/ducaale/xh)
**Language:** Rust
**Health:** 26% (poor)

## Validation Results

### Core metrics

| Check | Status | Value | Threshold |
|-------|--------|-------|-----------|
| max_fan_out | ERROR |  | 5 |
| coupling | ERROR |  |  |
| instability | ERROR |  |  |
| hub_nodes | ERROR |  | 10 |
| orphan_nodes | ERROR |  |  |
| strongly_connected_components | ERROR |  |  |
| dag_check | FAILED |  |  |
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
| graph_cliques | ERROR |  | 4 |
| zscore_outliers | ERROR |  |  |
| hotspot_detection | ERROR |  | 20 |
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
| clustering_coefficient | PASSED |  | 0.1 |
| edge_density | PASSED |  |  |
| avg_path_length | PASSED |  | 5.0 |
| k_core_decomposition | PASSED |  | 5 |
| dependency_entropy | PASSED |  |  |
| gini_coefficient | PASSED |  | 0.6 |
| cohesion_lcom4 | PASSED |  |  |
| change_propagation | PASSED |  | 20 |
| blast_radius | PASSED |  | 0.3 |
| deprecated_usage | PASSED |  |  |
| stability_violations | PASSED |  |  |
| circular_dependency_depth | PASSED |  | 3 |
| articulation_points | WARNING |  |  |

