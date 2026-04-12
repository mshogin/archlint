# Running archlint on TypeScript/React projects

archlint supports TypeScript monorepos with full detection of packages, components, React components, hooks, and import graphs.

## Prerequisites

- Go 1.22+ installed (for building archlint)
- archlint binary built: `go build -o archlint ./cmd/archlint/`
- Python 3.10+ with validator installed (optional, for deep metric validation)

## Quick start

```bash
# Collect architecture graph
archlint collect . -l typescript -o architecture.yaml

# Scan for violations (uses .archlint.yaml if present)
archlint scan .

# Network topology metrics
archlint network .
```

Running `collect` without `-l typescript` also works if archlint auto-detects `package.json` in the project root.

## What gets detected

### Component types

| Entity type      | What it maps to                                      |
|------------------|------------------------------------------------------|
| `ts-package`     | An npm workspace package (e.g. `packages/graph`)     |
| `react-package`  | A package that uses React/Ink (JSX files detected)   |
| `component`      | Exported class, function, type, interface            |
| `external_module`| Third-party npm dependency (prefixed with `ext:`)    |

### Detected elements

- **Packages** - workspace entries from `package.json` `workspaces` field
- **Components** - exported symbols from TypeScript/TSX source files
- **React components** - TSX files and function components returning JSX
- **Hooks** - functions starting with `use` (e.g. `useNode`, `useView`)
- **Imports** - `import from` statements become directed edges in the graph
- **Re-exports** - `export { X } from` chains are traced transitively

### Example: viewgraph monorepo

```
archlint collect . -l typescript
> Analyzing code: . (language: typescript)
> Found components: 64
>   - component: 45
>   - react-package: 3
>   - external_module: 13
>   - ts-package: 3
> Found edges: 106
> Graph saved to architecture.yaml
```

64 nodes, 106 edges collected from 4 packages in under 1 second.

## Configuring .archlint.yaml for TypeScript

### Generate a starter config

```bash
archlint init .
# or preview without writing:
archlint init . --dry-run
```

### Full example for a TypeScript monorepo

Layer structure: `cli -> tui -> graph <- adapters`

```yaml
# .archlint.yaml
rules:
  fan_out:
    enabled: true
    threshold: 5
    level: telemetry
  cycles:
    enabled: true
    level: taboo
  dip:
    enabled: true
    level: telemetry
  isp:
    enabled: true
    threshold: 5
    level: telemetry
  feature_envy:
    enabled: true
    level: telemetry
  god_class:
    enabled: true
    level: telemetry
  srp:
    enabled: true
    level: telemetry
  layer_violations:
    enabled: true
    error_on_violation: true
    params:
      layers:
        graph: 0
        adapters: 1
        tui: 2
        cli: 3

layers:
  - name: graph
    paths:
      - packages/graph/src
  - name: adapters
    paths:
      - packages/adapters/src
  - name: tui
    paths:
      - packages/tui/src
  - name: cli
    paths:
      - packages/cli/src

allowed_dependencies:
  cli:
    - tui
    - graph
    - adapters
  tui:
    - graph
  adapters:
    - graph
  graph: []
```

### Key rules for TypeScript projects

**`cycles`** - detect circular imports between packages. Set `level: taboo` to treat cycles as hard failures.

**`fan_out`** - limit how many packages/modules a component depends on. Threshold 5 works well for focused components; raise to 10 for entry points like `cli`.

**`layer_violations`** - enforce directional dependencies between monorepo packages. Assign numeric levels (lower = more stable/foundational).

**`isp`** - interface segregation: interfaces should not exceed `threshold` methods.

## Running the Python validators

The Python validator runs 200+ structural and topological metrics on top of the collected graph.

### Installation

```bash
cd /path/to/archlint-repo
pip install -e .
```

### Run validation

```bash
# Basic structural checks
python3 -m validator validate architecture.yaml --structure-only

# Full research metrics (topology, homology, spectral analysis)
python3 -m validator validate architecture.yaml --structure-only --group research

# JSON output (for CI integration)
python3 -m validator validate architecture.yaml --structure-only -f json
```

### Example output summary

```
graph:
  edges: 75
  nodes: 64
status: FAILED
summary:
  errors: 14
  failed: 1
  info: 103
  passed: 64
  skipped: 3
  total_checks: 222
  warnings: 37
```

### What the Python validator checks on TypeScript graphs

The validator operates on the YAML graph, so it is language-agnostic. The same 222 checks run on Go, Rust, and TypeScript projects:

- **Structural**: DAG check, fan-out, coupling (Ca/Ce), layer violations, orphan nodes, SCC
- **Object-oriented patterns**: SRP, OCP, LSP, DIP, ISP, god class, feature envy, shotgun surgery
- **Network topology**: betweenness centrality, PageRank, modularity, clustering coefficient
- **Algebraic topology**: Betti numbers, Euler characteristic, persistent homology, simplicial complexity
- **Stability**: distance from main sequence, instability, bounded context leakage

A key difference for TypeScript projects: the validator counts `external_module` nodes (npm dependencies) in coupling metrics. High efferent coupling from a package to many external modules will show as an error even if internal architecture is clean.

## Example: scan output

```bash
archlint scan .
> config: /path/to/.archlint.yaml
> PASSED: No violations found (threshold: 0)
```

With a properly configured `.archlint.yaml`, all layer rules are enforced. The viewgraph project passes the scan with zero violations, confirming that `cli -> tui -> graph <- adapters` layering is respected in the actual imports.

## Example: network metrics output

```bash
archlint network .
> Analyzing architecture: .
> Nodes: 64  Edges: 106
>
> === Global Metrics ===
>   Diameter:                     2
>   Average Shortest Path Length: 1.1039
>   Global Clustering Coefficient:0.0290
>   Small-World Coefficient (σ):  3.9968
>   Graph Entropy (H):            0.8075 nats
>   Modularity (Q):               0.5791
```

**Modularity Q = 0.58** means the package graph has strong community structure - a healthy sign for a monorepo.

**Small-World Coefficient σ = 4.0** confirms the graph has efficient information flow with high local clustering, typical of well-designed component hierarchies.

## Tips for TypeScript monorepos

1. **Run `archlint init .`** first to get a starter config, then customize layers.

2. **External modules inflate coupling metrics.** The `packages/graph/src` package shows Ce = 22 because it imports `yaml`, `eventemitter3`, and `node:*` modules. This is expected - use the `exclude` field in coupling rules to suppress known external dependencies if needed.

3. **React packages are detected automatically.** If a package contains `.tsx` files or imports `react`/`ink`, it is tagged as `react-package` in the graph. Hooks (`use*` functions) are collected as components.

4. **`archlint collect` is idempotent.** Run it after any code change to refresh `architecture.yaml`, then re-run validators or network analysis on the updated graph.

5. **For CI gates**, use `archlint scan .` - it exits with code 1 if violations exceed the configured threshold. The Python validator exits non-zero on ERROR status.
