---
title: "charmbracelet/log"
description: "Architecture analysis of charmbracelet/log"
---

**Repository:** [charmbracelet/log](https://github.com/charmbracelet/log)
**Language:** Go
**Health:** 86% (good)

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
| liskov_substitution | PASSED |  | 3 |
| dependency_inversion | PASSED |  | 0.3 |
| interface_segregation | PASSED |  | 5 |
| open_closed | WARNING |  | 0.2 |

### Advanced metrics

| Check | Status | Value | Threshold |
|-------|--------|-------|-----------|
| abstractness | ERROR |  |  |
| edge_density | INFO |  |  |
| graph_diameter | INFO |  |  |
| closeness_centrality | INFO |  |  |
| degree_distribution | INFO |  |  |
| algebraic_connectivity | INFO |  |  |
| spectral_radius | INFO |  |  |
| betweenness_centrality | PASSED |  | 0.3 |
| pagerank | PASSED |  | 0.1 |
| distance_from_main_sequence | PASSED |  |  |
| articulation_points | PASSED |  |  |
| bridge_edges | PASSED |  |  |
| avg_path_length | PASSED |  | 5.0 |
| eigenvector_centrality | PASSED |  | 0.3 |
| k_core_decomposition | PASSED |  | 5 |
| graph_cliques | PASSED |  | 4 |
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
| modularity | SKIP |  |  |
| dependency_entropy | SKIP |  |  |
| clustering_coefficient | WARNING |  | 0.1 |

