---
title: "sharkdp/hyperfine"
description: "Architecture analysis of sharkdp/hyperfine"
---

**Repository:** [sharkdp/hyperfine](https://github.com/sharkdp/hyperfine)
**Language:** Rust
**Health:** 24% (poor)

## Validation Results

### Core metrics

| Check | Status | Value | Threshold |
|-------|--------|-------|-----------|
| max_fan_out | ERROR |  | 5 |
| coupling | ERROR |  |  |
| instability | ERROR |  |  |
| hub_nodes | ERROR |  | 10 |
| orphan_nodes | ERROR |  |  |
| dag_check | FAILED |  |  |
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
| distance_from_main_sequence | ERROR |  |  |
| k_core_decomposition | ERROR |  |  |
| zscore_outliers | ERROR |  |  |
| change_propagation | ERROR |  | 20 |
| hotspot_detection | ERROR |  | 20 |
| stability_violations | ERROR |  |  |
| component_complexity | ERROR |  | 50 |
| bridge_edges | INFO |  |  |
| graph_diameter | INFO |  |  |
| closeness_centrality | INFO |  |  |
| eigenvector_centrality | INFO |  | 0.3 |
| degree_distribution | INFO |  |  |
| algebraic_connectivity | INFO |  |  |
| spectral_radius | INFO |  |  |
| betweenness_centrality | PASSED |  | 0.3 |
| pagerank | PASSED |  | 0.1 |
| modularity | PASSED |  | 0.3 |
| abstractness | PASSED |  |  |
| edge_density | PASSED |  |  |
| avg_path_length | PASSED |  | 5.0 |
| graph_cliques | PASSED |  | 4 |
| dependency_entropy | PASSED |  |  |
| gini_coefficient | PASSED |  | 0.6 |
| cohesion_lcom4 | PASSED |  |  |
| blast_radius | PASSED |  | 0.3 |
| deprecated_usage | PASSED |  |  |
| circular_dependency_depth | PASSED |  | 3 |
| clustering_coefficient | WARNING |  | 0.1 |
| articulation_points | WARNING |  |  |

