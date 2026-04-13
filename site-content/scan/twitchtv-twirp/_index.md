---
title: "twitchtv/twirp"
description: "Architecture analysis of twitchtv/twirp"
---

**Repository:** [twitchtv/twirp](https://github.com/twitchtv/twirp)
**Language:** Go
**Health:** 14% (poor)

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
| dependency_inversion | ERROR |  | 0.3 |
| liskov_substitution | PASSED |  | 3 |
| interface_segregation | PASSED |  | 5 |
| open_closed | WARNING |  | 0.2 |

### Advanced metrics

| Check | Status | Value | Threshold |
|-------|--------|-------|-----------|
| abstractness | ERROR |  |  |
| distance_from_main_sequence | ERROR |  |  |
| k_core_decomposition | ERROR |  |  |
| gini_coefficient | ERROR |  | 0.6 |
| zscore_outliers | ERROR |  |  |
| cohesion_lcom4 | ERROR |  |  |
| change_propagation | ERROR |  | 20 |
| hotspot_detection | ERROR |  | 20 |
| component_complexity | ERROR |  | 50 |
| edge_density | INFO |  |  |
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
| avg_path_length | PASSED |  | 5.0 |
| graph_cliques | PASSED |  | 4 |
| dependency_entropy | PASSED |  |  |
| blast_radius | PASSED |  | 0.3 |
| deprecated_usage | PASSED |  |  |
| stability_violations | PASSED |  |  |
| circular_dependency_depth | PASSED |  | 3 |
| clustering_coefficient | WARNING |  | 0.1 |
| articulation_points | WARNING |  |  |

