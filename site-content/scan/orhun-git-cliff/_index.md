---
title: "orhun/git-cliff"
description: "Architecture analysis of orhun/git-cliff"
---

**Repository:** [orhun/git-cliff](https://github.com/orhun/git-cliff)
**Language:** Rust
**Health:** 52% (needs work)

## Validation Results

### Core metrics

| Check | Status | Value | Threshold |
|-------|--------|-------|-----------|
| max_fan_out | ERROR |  | 5 |
| coupling | ERROR |  |  |
| hub_nodes | ERROR |  | 10 |
| dag_check | PASSED |  |  |
| instability | PASSED |  |  |
| layer_violations | PASSED |  |  |
| forbidden_dependencies | PASSED |  |  |
| orphan_nodes | PASSED |  |  |
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
| zscore_outliers | ERROR |  |  |
| component_complexity | ERROR |  | 50 |
| bridge_edges | INFO |  |  |
| closeness_centrality | INFO |  |  |
| eigenvector_centrality | INFO |  | 0.3 |
| degree_distribution | INFO |  |  |
| spectral_radius | INFO |  |  |
| betweenness_centrality | PASSED |  | 0.3 |
| pagerank | PASSED |  | 0.1 |
| modularity | PASSED |  | 0.3 |
| edge_density | PASSED |  |  |
| graph_diameter | PASSED |  | 10 |
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
| algebraic_connectivity | WARNING |  | 0.1 |

