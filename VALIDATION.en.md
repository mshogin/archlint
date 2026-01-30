# Architecture Validation in archlint

> [🇷🇺 Русская версия](VALIDATION.md)

This document describes the architecture validation system migrated from the aiarch project.

## Overview

archlint now includes 172 architecture validators organized into two families:
- **Structure** - static dependency graph analysis (167 validators)
- **Behavior** - dynamic context/trace analysis (8 validators)

## Directory Structure

```
archlint/
├── validator/                    # Python validators
│   ├── __init__.py
│   ├── __main__.py              # CLI entry point
│   ├── common.py                # Common utilities
│   ├── config.py                # Configuration
│   ├── graph_loader.py          # Graph loading
│   ├── context_loader.py        # Context loading
│   ├── requirements.txt         # Python dependencies
│   │
│   ├── structure/               # Structure family
│   │   ├── core/                # Maximum results for minimum time
│   │   │   └── metrics.py       # dag, fan_out, coupling, layers, forbidden_deps
│   │   ├── solid/               # SOLID principles
│   │   │   └── solid_metrics.py # SRP, OCP, LSP, DIP
│   │   ├── patterns/            # Code smells
│   │   │   └── patterns_metrics.py
│   │   ├── architecture/        # Clean/Hexagonal
│   │   │   └── architecture_metrics.py
│   │   ├── quality/             # Quality and security
│   │   │   └── quality_metrics.py
│   │   ├── advanced/            # Advanced analytics
│   │   │   ├── advanced_metrics.py
│   │   │   └── change_metrics.py
│   │   └── research/            # Mathematical/experimental
│   │       ├── topology_metrics.py
│   │       ├── information_theory_metrics.py
│   │       ├── linear_algebra_metrics.py
│   │       ├── category_theory_metrics.py
│   │       └── ... (15 modules)
│   │
│   └── behavior/                # Behavior family
│       ├── core/                # Basic context checks
│       │   └── context_metrics.py
│       └── advanced/            # Advanced checks
│           └── context_metrics.py
│
└── internal/cli/
    └── validate.go              # Go command for Python invocation
```

## Validator Groups

### Structure - Core (10 validators)

Maximum results for minimum time:

| Validator | Description |
|-----------|-------------|
| dag_check | Dependency cycle detection |
| max_fan_out | Too many outgoing dependencies |
| coupling | Ca/Ce coupling metrics |
| instability | Direction of dependencies (I = Ce/(Ca+Ce)) |
| layer_violations | Architectural layer violations |
| forbidden_dependencies | Forbidden links |
| hub_nodes | God Objects (many links) |
| orphan_nodes | Isolated components |
| strongly_connected_components | Mutual dependencies |
| graph_depth | Dependency chain depth |

### Structure - SOLID (5 validators)

| Validator | Description |
|-----------|-------------|
| single_responsibility | SRP - one reason to change |
| open_closed | OCP - open for extension, closed for modification |
| liskov_substitution | LSP - subtype substitutability |
| dependency_inversion | DIP - depend on abstractions |
| interface_segregation | ISP - small interfaces |

### Structure - Patterns (8 validators)

| Validator | Description |
|-----------|-------------|
| god_class | God Classes |
| shotgun_surgery | Shotgun Surgery |
| divergent_change | Divergent Change |
| lazy_class | Lazy Classes |
| middle_man | Middle Man classes |
| speculative_generality | Unused abstractions |
| data_clumps | Data Clumps |
| feature_envy | Feature Envy |

### Structure - Architecture (7 validators)

| Validator | Description |
|-----------|-------------|
| domain_isolation | Domain layer isolation |
| ports_adapters | Ports & Adapters |
| use_case_purity | Use case purity |
| dto_boundaries | DTOs at boundaries |
| inward_dependencies | Dependencies toward center |
| bounded_context_leakage | Bounded Context isolation |
| service_autonomy | Service autonomy |

### Structure - Quality (10 validators)

| Validator | Description |
|-----------|-------------|
| auth_boundary | Authentication at boundary |
| sensitive_data_flow | Sensitive data flow |
| input_validation_layer | Input validation |
| mockability | Mockability |
| test_isolation | Test isolation |
| test_coverage_structure | Structural coverage |
| logging_coverage | Logging coverage |
| metrics_exposure | Metrics exposure |
| health_check_presence | Health check presence |
| api_consistency | API consistency |

### Structure - Advanced (~25 validators)

Advanced analytics: centrality, modularity, change propagation, blast radius, etc.

### Structure - Research (~90 validators)

Mathematical approaches:
- Topology (Betti numbers, Euler characteristic)
- Information Theory (entropy, mutual information)
- Linear Algebra (SVD, spectral analysis)
- Category Theory (functors, monads)
- Game Theory (Shapley value, Nash equilibrium)
- Combinatorics and others

### Behavior - Core (4 validators)

| Validator | Description |
|-----------|-------------|
| context_coverage | Test coverage of critical components |
| untested_components | Uncovered components |
| ghost_components | Components in tests but not in architecture |
| context_complexity | Context complexity |

### Behavior - Advanced (4 validators)

| Validator | Description |
|-----------|-------------|
| single_point_of_failure | Components in all contexts |
| context_coupling | Coupling between contexts |
| layer_traversal | Layer violations in execution flow |
| context_depth | Call stack depth |

## Usage

### Go CLI

```bash
# Basic validation (core group)
archlint validate architecture.yaml

# With contexts (behavior validation)
archlint validate architecture.yaml -c contexts.yaml

# Specific group
archlint validate architecture.yaml -g solid

# Parallel execution of all groups
archlint validate architecture.yaml -p

# JSON output
archlint validate architecture.yaml -f json -o report.json
```

### Python CLI (direct)

```bash
cd archlint
python -m validator validate architecture.yaml
python -m validator validate architecture.yaml -g core
python -m validator list
```

## Architecture

```
Go CLI (archlint)
    |
    v
Python Validator (subprocess)
    |
    +-- Structure validators
    |       +-- core (NetworkX)
    |       +-- solid
    |       +-- patterns
    |       +-- architecture
    |       +-- quality
    |       +-- advanced
    |       +-- research (NumPy, SciPy)
    |
    +-- Behavior validators
            +-- core
            +-- advanced
```

Go CLI invokes Python validators via subprocess. With `-p` (parallel) flag, groups are executed in parallel via goroutines.

## Configuration

Validators are configured via `.archlint.yaml`:

```yaml
validators:
  dag_check:
    enabled: true
    error_on_violation: true
    exclude:
      - "external/*"
      - "generated/*"

  max_fan_out:
    threshold: 5
    error_on_violation: false

  layer_violations:
    params:
      layers:
        cmd: 0
        api: 1
        service: 2
        domain: 3
        repository: 4
```

## Dependencies

Python:
- networkx >= 3.0 (graph algorithms)
- pyyaml >= 6.0 (YAML parsing)
- numpy >= 1.24 (for advanced/research)
- scipy >= 1.10 (for advanced/research)

## Migration from aiarch

All 172 validators migrated from aiarch without logic changes. Changes:
1. Reorganization by families (Structure/Behavior) and groups
2. Added Python CLI (__main__.py)
3. Integration with Go CLI via subprocess
4. Support for parallel group execution
