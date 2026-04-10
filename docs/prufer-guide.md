# Prüfer Metrics Guide

A practical guide for developers who want to use Prüfer sequence metrics
to understand the tree-like structure of their architecture.

Credit: Ярослав Черкашин (@Yaroslam, https://github.com/Yaroslam)
        suggested these metrics at Стачка 2026 (10 April 2026).

---

## What is a Prüfer Sequence?

Imagine your dependency graph has a "backbone" - a minimal set of edges that
still connects everything without cycles. That backbone is called a spanning tree.

A Prüfer sequence is a compact way to describe any spanning tree.
For a tree with n nodes, the sequence has exactly n-2 numbers.
Different trees produce different sequences. Same sequence = same tree shape.

Why does this matter for architecture? Because your module dependency graph
usually contains cycles and cross-cutting edges, but the underlying
hierarchical structure - the spanning tree - reveals whether the system is
shaped more like a hub-and-spoke, a pipeline, or a tangled mesh.

Prüfer metrics make that hidden shape measurable.

---

## How to Use

### Step 1: Collect your architecture

```bash
# Scan your project and produce architecture.yaml
archlint-rs collect . > architecture.yaml
```

### Step 2: Run the validator

```bash
# Run all research-group metrics, including prufer_*
python3 -m validator validate architecture.yaml --structure-only --group research
```

### Step 3: Filter for Prüfer results

```bash
python3 -m validator validate architecture.yaml --structure-only --group research \
  | grep -E "prufer|isomorphism|spanning_tree|cayley"
```

Or look for these metric names in the YAML/JSON output:

- `prufer_canonical_form`
- `prufer_entropy`
- `prufer_similarity`
- `tree_isomorphism_class`
- `spanning_tree_coverage`
- `cayley_complexity_bound`

---

## What Each Metric Tells You

### prufer_canonical_form

**What it is:** A numeric fingerprint of your architecture's spanning tree.

The validator computes the minimum spanning tree of your dependency graph,
relabels nodes to integers 1..n, then encodes the tree as a sequence of
n-2 numbers using Prüfer's algorithm.

**Output:**
```yaml
name: prufer_canonical_form
status: PASSED
details:
  nodes: 12
  prufer_code_length: 10
  prufer_code: [3, 3, 5, 3, 7, 5, 3, 5, 7, 3]
```

**Reading the output:**
- A code where one number repeats many times (e.g. `[3,3,3,3,3]`) means
  node 3 is a dominant hub - hub-and-spoke architecture.
- A code with all different numbers means no single hub - flat or mesh structure.
- Two codebases with the same code have structurally identical spanning trees.

**Status:** Always PASSED. This is a descriptive metric.


### prufer_entropy

**What it is:** A complexity measure based on how varied the Prüfer code values are.

Shannon entropy over the Prüfer code values. Low entropy = one or few nodes
dominate the tree (clear hierarchy). High entropy = many nodes appear equally
often (complex, distributed structure).

**Output:**
```yaml
name: prufer_entropy
status: INFO            # or WARNING if entropy is very high
details:
  entropy: 1.8294
  max_entropy: 3.5849   # log2(n)
  entropy_ratio: 0.5104
  threshold: 3.2264
```

**Reading the output:**
- `entropy_ratio` close to 0 = simple, hierarchical tree (good for most projects)
- `entropy_ratio` close to 1 = nearly random tree structure
- Status becomes `WARNING` when `entropy_ratio > 0.9`

**Threshold:** configurable via `prufer_entropy_ratio_threshold` in config.


### prufer_similarity

**What it is:** A way to compare two architecture snapshots.

Measures the edit distance between Prüfer codes of two graphs.
Low distance = similar tree structure. High distance = architectures diverged.

**Current limitation:** requires two graphs as input.
In single-graph mode the validator returns `INFO` and explains
that a baseline is needed.

**Output (single-graph mode):**
```yaml
name: prufer_similarity
status: INFO
details:
  baseline_required: true
```

To use this metric for drift detection, compare the current architecture
against a previously saved snapshot (baseline).


### tree_isomorphism_class

**What it is:** A structural class label for your architecture's spanning tree.

Two spanning trees are "isomorphic" if one can be rearranged into the other
by renaming nodes. The validator computes the sorted Prüfer code as a
fingerprint of this isomorphism class.

**Output:**
```yaml
name: tree_isomorphism_class
status: PASSED
details:
  isomorphism_fingerprint: [1, 1, 2, 2, 3, 4, 5, 5, 5, 5]
  degree_sequence: [6, 3, 3, 2, 2, 1, 1, 1, 1, 1, 1, 1]
```

**Reading the output:**
- `isomorphism_fingerprint` is the same for any two structurally equivalent
  architectures (regardless of module names).
- `degree_sequence` shows hub sizes: [6, 3, 3, ...] means one module with
  6 dependencies, two with 3, etc.
- Use this to compare architectures across different services or microservices -
  same fingerprint means same topological shape.

**Status:** Always PASSED. Descriptive metric.


### spanning_tree_coverage

**What it is:** How "tree-like" your dependency graph is.

Formula: `coverage = (n-1) / total_edges`

A pure tree has exactly n-1 edges, so coverage = 1.0.
Every extra edge (cycle, cross-dependency) reduces coverage below 1.0.

**Output:**
```yaml
name: spanning_tree_coverage
status: INFO            # or WARNING if coverage < 0.5
details:
  nodes: 15
  total_edges: 24
  spanning_tree_edges: 14
  extra_edges: 10
  coverage: 0.5833
  threshold: 0.5
```

**Reading the output:**
- `coverage = 1.0` - pure tree, no cycles, clean hierarchy
- `coverage = 0.5..1.0` - some cross-dependencies, manageable
- `coverage < 0.5` - more than double the minimum edges needed,
  highly interconnected, WARNING is raised

**Threshold:** configurable via `spanning_tree_coverage_threshold`.


### cayley_complexity_bound

**What it is:** The theoretical number of valid spanning tree topologies
for your set of modules.

Cayley's formula: for n nodes, there are exactly `n^(n-2)` distinct labeled trees.

This number represents "architectural freedom": how many different ways you
could connect n modules in a tree structure. Larger = more options = harder
to reason about which structure you ended up with.

**Output (small n):**
```yaml
name: cayley_complexity_bound
status: INFO
details:
  nodes: 10
  cayley_exponent: 8
  cayley_bound_exact: 100000000   # 10^8
  log10_cayley_bound: 8.0
```

**Output (large n > 20):**
```yaml
name: cayley_complexity_bound
status: INFO
details:
  nodes: 50
  cayley_bound_approx: "10^82.5 (too large to display exactly)"
  log10_cayley_bound: 82.5
```

**Reading the output:**
- This metric does not trigger warnings. It is purely informational.
- For 10 modules: 100 million possible tree structures.
- For 20 modules: roughly 10^23 possible structures (more than atoms in a gram).
- Use `log10_cayley_bound` to compare complexity across projects:
  higher log = your team has more implicit "decisions" baked into the
  current architecture that are hard to reconstruct.

---

## Examples

### Example 1: Simple Hierarchical Project

A well-structured monolith or library with clear layers:
`cmd -> service -> repository -> model`

Expected results:
- `prufer_entropy_ratio` ~ 0.2-0.4 (low, dominated by a few hubs)
- `spanning_tree_coverage` ~ 0.8-1.0 (few cross-layer edges)
- `isomorphism_fingerprint` shows a few repeated values (the service/repository layer hubs)

This is the healthy baseline. Adding new modules without new cross-dependencies
keeps coverage high and entropy low.


### Example 2: Microservices with Many Cross-Dependencies

A service mesh or shared-library architecture where many services
import from each other or share a large common library:

Expected results:
- `prufer_entropy_ratio` ~ 0.7-0.9 (many nodes appear equally in the code)
- `spanning_tree_coverage` ~ 0.2-0.4 (WARNING: far more edges than a tree needs)
- `cayley_complexity_bound` is very large (many possible structures, hard to reason about)

This does not mean the architecture is wrong - microservices with a service mesh
are designed this way. But if you expected a hierarchical structure and see
these numbers, the cross-dependencies have grown beyond the original design.


### Example 3: What to Do When Entropy is High

High entropy (ratio > 0.9) means your spanning tree has no dominant hubs -
every node appears roughly equally often in the Prüfer code.

Diagnosis steps:

1. Look at `degree_sequence` from `tree_isomorphism_class`.
   If the sequence is flat (all values near 2-3), there are no clear layers.

2. Check `spanning_tree_coverage`.
   If coverage is also low, you have both many cross-edges AND no clear hierarchy.

3. Find the high-degree nodes (hubs). These are candidates for
   facade/gateway modules that could absorb the cross-dependencies.

4. Refactor: introduce a shared layer or facade that aggregates
   the cross-cutting concerns. After refactoring, entropy should drop
   because the new hub will dominate the Prüfer code.

5. Re-run the metrics to verify the change:
   ```bash
   archlint-rs collect . > architecture-after.yaml
   python3 -m validator validate architecture-after.yaml --group research
   ```

---

## How It Relates to Other Metrics

### HoTT Metrics (hott_metrics.py)

HoTT (Homotopy Type Theory) metrics operate at a higher level of abstraction -
they check whether module paths are contractible (can be deformed into
simpler paths). Prüfer metrics are more concrete: they encode the actual
spanning tree structure as a numeric sequence.

Use HoTT metrics to detect abstract coupling patterns.
Use Prüfer metrics to measure the concrete tree complexity of the same graph.

### Topology Metrics (topology_metrics.py)

Topology metrics compute Betti numbers - how many independent cycles exist
in the dependency graph. Betti number beta_1 counts the number of cycles
beyond what a spanning tree requires.

Relationship: `extra_edges` in `spanning_tree_coverage` equals beta_1.
Both metrics agree on how non-tree-like the graph is, but from different angles:
- Topology: "how many independent cycles?"
- Prüfer: "what shape is the minimal cycle-free backbone?"

### Graph Theory Metrics (advanced_graph_metrics.py)

Graph metrics (centrality, betweenness, clustering) describe individual nodes.
Prüfer metrics describe the global tree structure.

Use centrality to find which specific module is a bottleneck.
Use Prüfer entropy to decide whether the overall structure is
hub-dominated or evenly distributed.

---

## Configuration

All Prüfer thresholds can be overridden in your `archlint.yaml`:

```yaml
rules:
  prufer_entropy:
    prufer_entropy_ratio_threshold: 0.8   # default: 0.9

  spanning_tree_coverage:
    spanning_tree_coverage_threshold: 0.6  # default: 0.5

  prufer_canonical_form:
    exclude:
      - "test"
      - "vendor"
      - "mock"
```

The `exclude` list accepts substring patterns - any node whose name contains
a listed string is removed before analysis.
