---
title: "mikeshogin/seclint"
description: "Architecture analysis of mikeshogin/seclint"
---

**Repository:** [mikeshogin/seclint](https://github.com/mikeshogin/seclint)
**Language:** Go
**Health:** 74% (moderate)

## Validation Results

### Core metrics

| Check | Status | Value | Threshold |
|-------|--------|-------|-----------|
| orphan_nodes | ERROR |  |  |
| dag_check | PASSED |  |  |
| max_fan_out | PASSED |  | 5 |
| coupling | PASSED |  |  |
| instability | PASSED |  |  |
| layer_violations | PASSED |  |  |
| forbidden_dependencies | PASSED |  |  |
| hub_nodes | PASSED |  | 10 |
| strongly_connected_components | PASSED |  |  |
| graph_depth | PASSED |  |  |
| component_distance | PASSED |  |  |

### Solid metrics

| Check | Status | Value | Threshold |
|-------|--------|-------|-----------|
| single_responsibility | PASSED |  | 3 |
| open_closed | PASSED |  | 0.2 |
| liskov_substitution | PASSED |  | 3 |
| dependency_inversion | PASSED |  | 0.3 |
| interface_segregation | PASSED |  | 5 |

### Advanced metrics

| Check | Status | Value | Threshold |
|-------|--------|-------|-----------|
| modularity | ERROR |  | 0.3 |
| distance_from_main_sequence | ERROR |  |  |
| gini_coefficient | ERROR |  | 0.6 |
| bridge_edges | INFO |  |  |
| graph_diameter | INFO |  |  |
| closeness_centrality | INFO |  |  |
| eigenvector_centrality | INFO |  | 0.3 |
| degree_distribution | INFO |  |  |
| algebraic_connectivity | INFO |  |  |
| spectral_radius | INFO |  |  |
| betweenness_centrality | PASSED |  | 0.3 |
| pagerank | PASSED |  | 0.1 |
| abstractness | PASSED |  |  |
| edge_density | PASSED |  |  |
| avg_path_length | PASSED |  | 5.0 |
| k_core_decomposition | PASSED |  | 5 |
| graph_cliques | PASSED |  | 4 |
| zscore_outliers | PASSED |  |  |
| cohesion_lcom4 | PASSED |  |  |
| change_propagation | PASSED |  | 20 |
| blast_radius | PASSED |  | 0.3 |
| hotspot_detection | PASSED |  | 20 |
| deprecated_usage | PASSED |  |  |
| stability_violations | PASSED |  |  |
| circular_dependency_depth | PASSED |  | 3 |
| component_complexity | PASSED |  | 50 |
| clustering_coefficient | WARNING |  | 0.1 |
| articulation_points | WARNING |  |  |
| dependency_entropy | WARNING |  |  |

