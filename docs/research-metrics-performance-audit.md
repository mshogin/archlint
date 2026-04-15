# Research Metrics Performance Audit

**Date:** 2026-04-11
**Subject:** Python validator research metrics - performance analysis for large graphs (~1000 nodes)
**Symptom:** Running `--group research` on a graph with ~1000 components ran for over an hour, consumed all memory, and never finished.

---

## Executive Summary

The research validator group contains **17 modules** with roughly **100+ individual metrics**.
At least **25 metric functions** have algorithmic complexity that is catastrophically expensive at
1000+ nodes: NP-hard algorithms called without size guards, full n*n dense matrix allocations,
`nx.simple_cycles` (which enumerates ALL cycles - exponential in the worst case), and
`nx.complement` (which builds an O(n^2) edge graph in memory).

The root cause is that most research metrics were designed and tested on small sample graphs
(10-50 nodes) and lack hard size limits for large graphs.

---

## Summary Table

| File | Metric / Function | Current O() | Issue | Fix | Quality Impact | Priority |
|---|---|---|---|---|---|---|
| `combinatorics_metrics.py` | `validate_ramsey_analysis` | O(n * exp(n)) | Calls `nx.find_cliques` AND `nx.complement` + `nx.find_cliques` on complement graph - twice for each Ramsey number that applies | Add `n > 100` guard -> SKIP; or use clique heuristic only | Low: descriptive only, result is nearly always trivially true for large n | P1 - CRITICAL |
| `advanced_topology_metrics.py` | `validate_gradient_flow` (line ~515) | O(n^2 * paths) | Calls `nx.all_simple_paths` with **no cutoff** for all source/sink pairs - can enumerate exponentially many paths | Add `cutoff=5`, limit pairs to first 10 sources * 10 sinks | Low: using first path is fine for architecture signal | P1 - CRITICAL |
| `linear_algebra_metrics.py` | `validate_controllability` | O(n^3 * 10) = O(n^3) | Builds controllability matrix of shape (n, 10*n) by repeated dense matrix multiplications. At n=1000: a 1000x10000 matrix requiring 80 MB per step. Total: hundreds of MB + O(n^3) per multiply | Add `n > 200` guard -> SKIP; or use random projection rank estimation | Medium: controllability is meaningful but concept is questionable for dependency graphs | P1 - CRITICAL |
| `linear_algebra_metrics.py` | `validate_condition_number` | O(n^3) | Calls `np.linalg.cond(laplacian)` on full dense n*n matrix. At n=1000: 8 MB matrix + O(n^3) = ~10^9 ops | Use `scipy.sparse.linalg.eigs` to get only top/bottom eigenvalues (O(n * k) with k=2); add `n > 500` guard or sparse path | Low: result is always large for disconnected-ish graphs, rarely actionable | P1 - CRITICAL |
| `linear_algebra_metrics.py` | `validate_effective_resistance` | O(n^3) | Calls `np.linalg.pinv(laplacian)` - full pseudo-inverse of n*n matrix. At n=1000: 8 MB matrix + O(n^3) SVD | Use `scipy.sparse.linalg` for Laplacian pseudoinverse via sparse solve; add `n > 300` guard | Medium: effective resistance is a useful signal for connectivity | P1 - CRITICAL |
| `linear_algebra_metrics.py` | `validate_svd_analysis` | O(n^3) | Calls `np.linalg.svd(adj_matrix)` - full SVD of n*n dense adjacency matrix. At n=1000: 8 MB matrix + O(n^3) = ~10^9 flops | Use `scipy.sparse.linalg.svds` (truncated SVD, top k=10 singular values) instead of full SVD; never call `.todense()` | Low: analyzing top-k singular values provides same insight | P1 - CRITICAL |
| `combinatorics_metrics.py` | `validate_extremal_bounds` | O(exp(n)) | Calls `nx.find_cliques` (NP-hard, exponential) on the full graph | Add `n > 100` guard; use `nx.approximation.max_clique` or degree-based upper bound | Low: Turan bound analysis is descriptive; approximation is sufficient | P1 - CRITICAL |
| `topology_metrics.py` | `validate_simplicial_complexity` | O(exp(n)) | Calls `nx.find_cliques` (NP-hard) on full undirected graph, no size guard | Add `n > 100` guard -> SKIP or sample; use `nx.graph_clique_number` which uses heuristics | Low: max simplex dimension rarely changes architectural conclusion | P1 - CRITICAL |
| `advanced_graph_metrics.py` | `validate_chromatic_number` | O(exp(n)) | Calls `nx.find_cliques` for clique lower bound, no size guard | Skip `find_cliques` for n>100; use degeneracy as lower bound (already available) | Negligible: greedy coloring upper bound is the primary result | P1 - CRITICAL |
| `set_theory_metrics.py` | `validate_transitive_closure` (and 3 others using `nx.transitive_closure`) | O(n^3) | `nx.transitive_closure` materializes an n*n graph in memory. At n=1000: up to 10^6 edges. Called in at least 4 functions: `validate_transitive_closure`, `validate_partial_order`, `validate_lattice_structure`, `validate_zeta_function` | Add `n > 200` guard -> SKIP or use DFS reachability on demand | Medium: transitive closure is architecturally meaningful for dependency analysis | P1 - CRITICAL |
| `hott_metrics.py` | `validate_higher_inductive_types` | O(exp(n)) | `nx.simple_cycles` enumerates ALL simple cycles - exponential in the worst case for graphs with many cycles | Add `n > 200` guard; use `nx.cycle_basis` (polynomial O(E)) to count independent cycles instead of enumerating all | Low: the COUNT of cycles matters, not the full enumeration | P2 - HIGH |
| `hott_metrics.py` | `validate_n_truncation` | O(exp(n)) | Same issue: `nx.simple_cycles` called without size limit | Same fix: `nx.cycle_basis` + guard; or limit to checking `nx.is_directed_acyclic_graph` + count length-2 cycles | Medium: n-truncation level is a genuinely useful metric | P2 - HIGH |
| `automata_theory_metrics.py` | `validate_path_coverage` (line ~322) | O(exp(n)) | `nx.all_simple_paths` with `cutoff=15` - cutoff of 15 hops means exponential path explosion even on sparse graphs | Reduce `cutoff` to 5 or 6; limit source/target pairs to 10*10 | Low: coverage signal doesn't need all paths | P2 - HIGH |
| `probability_metrics.py` | `validate_markov_properties` | O(exp(n)) | `nx.simple_cycles` called to compute GCD of cycle lengths, no size guard | Add `n > 200` guard; use `nx.cycle_basis` to get independent cycle lengths | Low: aperiodicity check rarely actionable for architecture | P2 - HIGH |
| `mathematical_analysis_metrics.py` | `validate_heat_diffusion` | O(n^3) | Calls `np.linalg.eigvalsh(L)` on full dense Laplacian. At n=1000: 8 MB matrix + O(n^3) | Use `scipy.sparse.linalg.eigsh` for just top k eigenvalues; keep L sparse | Medium: diffusion analysis is a meaningful stability signal | P2 - HIGH |
| `mathematical_analysis_metrics.py` | `validate_laplacian_smoothness` | O(n^3) | Calls `nx.betweenness_centrality` (O(n*E)) THEN materializes full Laplacian as dense matrix for matrix multiply `f @ L @ f`. Both separately slow at scale | Use sparse `L.dot(f)` instead of `.todense()`; skip betweenness for n>500 (it is O(n*E) alone) | Low: degree centrality gradient is sufficient signal | P2 - HIGH |
| `mathematical_analysis_metrics.py` | `validate_operator_norm` | O(n^3) | Full SVD via `np.linalg.svd(A)` for nuclear norm. Full `np.linalg.cond()` for condition number. Both on dense n*n matrix | Use truncated SVD (`svds(A, k=10)`) for nuclear norm approximation; use sparse eigensolvers for condition number | Low: operator norm is informational only | P2 - HIGH |
| `advanced_topology_metrics.py` | `validate_hodge_decomposition` | O(n^2 * E) | Builds boundary matrix B of shape (E * n) and computes eigenvalues of L0, L1 (both dense). At n=1000, E=5000: B is 5M entries | Add `n > 300` guard; use sparse eigensolvers | Low: Hodge decomposition is purely academic for architecture | P3 - MEDIUM |
| `integral_calculus_metrics.py` | `validate_isoperimetric_profile` (line ~706) | O(n^3) | Nested loop over all 2^n subsets (limited to first 20 subsets of size n//4), computing boundary for each. Inner loop `for v in complement: if has_edge(u, v)` is O(n^2) per subset | Add `n > 100` guard; use spectral approximation via Cheeger inequality | Low: isoperimetric constant is approximated anyway | P3 - MEDIUM |
| `combinatorics_metrics.py` | `validate_polya_enumeration` | O(n!) | `math.factorial(count)` called for every degree class. For n=1000 with degree=500, this overflows and hangs computing 500! | Cap per-degree factorial computation at 20; use Stirling approximation for large counts | Negligible: Polya enumeration bound is purely descriptive | P3 - MEDIUM |
| `information_theory_metrics.py` | `validate_absorption_probability` | O(transient * absorbing * paths) | Double loop: 50 transient * all absorbing nodes, calling `nx.shortest_path_length` per pair. At n=1000 with 500 absorbing: 25,000 path queries | Cap absorbing nodes to min(20, len(absorbing)); use BFS from each absorbing node instead of per-pair queries | Low: average path to absorption is a soft signal | P3 - MEDIUM |
| `topology_metrics.py` | `validate_topological_persistence` | O(E * n) | For each edge (up to E), copies the full graph and calls `nx.number_connected_components`. At E=5000, n=1000: 5000 full graph copies | Use incremental Union-Find instead of full copy per edge; already has the right algorithm nearby | Medium: bridge detection is a strong signal for architecture fragility | P3 - MEDIUM |
| `advanced_topology_metrics.py` | `validate_bottleneck_stability` | O(E * n) | Same pattern: copies graph for every edge to recompute components and cycles. No size guard | Same fix: incremental computation; add `n > 500, E > 2000` guard | Low: duplicates information from `validate_topological_persistence` | P3 - MEDIUM |
| `mathematical_analysis_metrics.py` | `validate_spectral_bifurcation` (line ~935) | O(n^3 * E) | For up to `n_perturbations=10` edges, computes `np.linalg.eigvalsh` of full dense Laplacian per perturbation | Add `n > 200` guard; limit perturbations to 3; use incremental rank-1 eigenvalue update | Low: bifurcation analysis is exploratory only | P3 - MEDIUM |
| `category_theory_metrics.py` | `validate_morphism_composition` | O(n * E * out_degree) | Triple nested loop: for each node, for each successor, for each successor's successor. At n=1000, avg degree=5: 1000 * 5 * 5 = 25,000 iterations. Acceptable but no limit on result size | Add result cap (already caps paths_of_length_2 at 10) - current code is OK for n=1000 | Negligible | P4 - LOW |

---

## Detailed Analysis by File

### 1. `combinatorics_metrics.py` - validate_ramsey_analysis (P1)

**The worst single function in the codebase.** Called inside a loop for up to 7 Ramsey numbers.
For each Ramsey number R(r,s) where n >= R(r,s):

```python
max_clique = max(len(c) for c in nx.find_cliques(undirected))       # O(exp(n)) - NP-hard
complement_graph = nx.complement(undirected)                          # O(n^2) memory
cliques_in_complement = list(nx.find_cliques(complement_graph))       # O(exp(n)) again
```

Then AGAIN at the end of the function (redundant second call):
```python
cliques = list(nx.find_cliques(undirected))
complement = nx.complement(undirected)
independent_sets = list(nx.find_cliques(complement))
```

For n=1000 with a moderately connected graph, `nx.find_cliques` can run for hours. `nx.complement`
materializes up to n*(n-1)/2 = ~500,000 edges in memory. This alone will exhaust RAM.

**Fix:** Add a guard `if n > 100: return SKIP`. The Ramsey implications are trivially true for
large n anyway (n=1000 >> R(4,5)=25), so skipping loses no architectural insight.

---

### 2. `advanced_topology_metrics.py` - validate_gradient_flow (P1, line ~515)

```python
paths = list(nx.all_simple_paths(subgraph, source, sink))   # NO CUTOFF
```

`nx.all_simple_paths` without a `cutoff` parameter can enumerate an exponential number of paths
on a graph with many back-edges. On a 1000-node graph with cycles, this will not terminate.

This function iterates over ALL sources (no incoming edges) cross ALL sinks (no outgoing edges).
On a graph with 100 sources and 100 sinks, that's 10,000 pairs each calling all_simple_paths.

**Fix:** Add `cutoff=5` parameter and limit source/sink pairs to `sources[:10]` and `sinks[:10]`.

---

### 3. `linear_algebra_metrics.py` - Matrix operations (P1)

Multiple functions call `.todense()` on the adjacency/Laplacian matrix, converting a sparse
matrix to a dense n*n array. At n=1000: 8 MB per matrix (float64). This is then passed to
O(n^3) routines:

- `validate_svd_analysis`: `np.linalg.svd(adj_matrix)` - full SVD is O(n^3), returns U (n*n), S (n), Vt (n*n) = 24 MB output
- `validate_condition_number`: `np.linalg.cond(laplacian)` - internally does full SVD, same cost
- `validate_effective_resistance`: `np.linalg.pinv(laplacian)` - full pseudo-inverse via SVD, O(n^3) + 8 MB output
- `validate_controllability`: builds controllability matrix of shape (n, 10*n) = 80 MB for n=1000, then `np.linalg.matrix_rank` on it

These four functions together at n=1000 require approximately 200-400 MB of memory and
~4 * 10^9 floating point operations. Even on fast hardware, this is several minutes each.

**Fix for SVD/condition/eigvals:** Use `scipy.sparse.linalg` routines: `svds(A, k=10)`,
`eigsh(L, k=2)` instead of materializing full matrices.

**Fix for controllability:** Add `n > 200` guard. The concept of a full controllability matrix
(Kalman rank condition) for a 1000-node dependency graph has no meaningful architectural
interpretation - the matrix is always rank-deficient for sparse graphs.

---

### 4. `set_theory_metrics.py` - transitive_closure (P1)

`nx.transitive_closure` is called in at least 4 functions. It materializes a new graph with
potentially O(n^2) edges (every reachable pair). At n=1000 with a DAG of depth 10:
possibly 100k-500k new edges added to a new nx.DiGraph object = hundreds of MB.

Furthermore, `validate_mobius_function` in `combinatorics_metrics.py` does:
```python
tc = nx.transitive_closure(work_graph)
# then loops: for i, x in topo_order: for j, y in topo_order: if tc.has_edge(x, y):
```
This is O(n^2) loop over the already O(n^2) closure graph = O(n^2) minimum, O(n^3) in
the Mobius computation inner loop.

**Fix:** Add `n > 200` guard for all transitive_closure calls. For architectural purposes,
reachability sampling (BFS from a subset of nodes) gives equivalent signal at O(k * (n + E)).

---

### 5. `hott_metrics.py` / `probability_metrics.py` / `number_theory_metrics.py` / `automata_theory_metrics.py` - simple_cycles (P2)

`nx.simple_cycles` is called in at least 8 places across the research modules. The algorithm
enumerates ALL simple cycles in the graph. In the worst case (near-complete directed graph),
the number of simple cycles is exponential. Even in typical sparse dependency graphs with
100 cycles, this can return millions of cycles if there are interleaved short cycles.

For a graph with n=1000 nodes and a modest 5% edge density, `nx.simple_cycles` can run
for many minutes and allocate GB of memory to store all found cycles.

**Fix across all callers:**
- For cycle presence/absence: use `not nx.is_directed_acyclic_graph(graph)` - O(n + E)
- For cycle count / length distribution: use `nx.cycle_basis(G.to_undirected())` - O(n + E), returns independent cycles
- Add `if n > 300: return SKIP` for any function that materializes the full cycle list

---

### 6. `topology_metrics.py` - validate_simplicial_complexity / `advanced_graph_metrics.py` - validate_chromatic_number (P1)

Both call `nx.find_cliques` without a size guard. `find_cliques` is the Bron-Kerbosch
algorithm with O(3^(n/3)) worst-case complexity. At n=1000, even on a sparse graph this
can run indefinitely.

`validate_simplicial_complexity` materializes ALL cliques:
```python
cliques = list(nx.find_cliques(subgraph))   # full materialization, no limit
```

`validate_chromatic_number` also uses it as lower bound. The greedy coloring (already computed)
provides the upper bound, and degeneracy already provides a tight lower bound.

---

### 7. `topology_metrics.py` - validate_topological_persistence (P3)

```python
for edge in edges:                              # O(E) iterations
    test_graph = subgraph.copy()               # O(n + E) copy each time
    test_graph.remove_edge(*edge)
    new_components = nx.number_connected_components(test_graph)  # O(n + E) BFS
```

Total: O(E * (n + E)). At E=5000, n=1000: 50M operations. Runs but is slow.
The same result can be computed in O(n + E) using bridge detection (`nx.bridges`).

---

### 8. `mathematical_analysis_metrics.py` - multiple eigenvalue computations (P2)

Several functions call `np.linalg.eigvalsh` or `np.linalg.eigh` on full dense n*n Laplacians:
`validate_heat_diffusion`, `validate_laplacian_smoothness`, `validate_operator_norm`,
`validate_graph_fourier_transform`, `validate_spectral_filtering`, `validate_wavelet_analysis`.

Each independently converts to dense and computes full spectrum at O(n^3) cost.
At n=1000 this is ~10^9 flops per call, taking 10-30 seconds each.
With 6+ such calls, total cost for this module alone: 60-180 seconds.

**Shared fix:** Extract a single cached `get_sparse_laplacian_top_k_eigenvalues(graph, k=20)`
utility that uses `scipy.sparse.linalg.eigsh` and reuse across all functions.

---

### 9. `combinatorics_metrics.py` - validate_polya_enumeration (P3)

```python
automorphism_bound *= math.factorial(count)
```

For a large graph where 500 nodes share the same degree, `math.factorial(500)` is a number
with 1135 digits. Python arbitrary-precision integers handle this, but subsequent division
`two_colorings / automorphism_bound` = `2^1000 / 500!` involves numbers with thousands of
digits. This is CPU-intensive due to bignum arithmetic. The result is a near-zero float.

**Fix:** Cap per-degree factorial at `min(count, 20)` or use Stirling's approximation in
log-space. The result is purely descriptive and doesn't require exact computation.

---

## Graph Loader Analysis

`graph_loader.py` is clean and O(n + E) for loading. No performance issues found.
The loader correctly uses NetworkX sparse graph structures throughout.

---

## Patterns Validator Analysis

`patterns/patterns_metrics.py` is not problematic for 1000 nodes. All patterns use O(n + E)
graph traversals. The most expensive is `god_class` detection which does per-node edge
iteration but no quadratic loops. No fixes needed here.

---

## Recommended Immediate Actions (before next run)

These changes would make the research group runnable on n=1000 without changes to
algorithmic quality:

**Action 1 - Add size guards to NP-hard metrics.** The following functions should return
`status: SKIP, reason: "Graph too large (n={n}), use --group advanced instead"` when n > 200:

- `validate_ramsey_analysis` (guard at n > 100)
- `validate_simplicial_complexity` (guard at n > 150)
- `validate_chromatic_number` find_cliques path (use degeneracy only for n > 100)
- `validate_extremal_bounds` (guard at n > 100)
- `validate_shapley_value` (already has guard at n > 15, correct)
- `validate_controllability` (guard at n > 200)
- All `simple_cycles` callers (guard at n > 300)
- All `transitive_closure` callers (guard at n > 200)

**Action 2 - Fix the missing cutoff in all_simple_paths.** Search for
`nx.all_simple_paths` without `cutoff=` parameter and add `cutoff=5` everywhere.
Found in `advanced_topology_metrics.py` line ~515.

**Action 3 - Replace `.todense()` with sparse operations.** Every `.todense()` call
creates an n*n matrix in memory. At n=1000 each is 8 MB. Replace:
- `np.linalg.svd(A)` -> `scipy.sparse.linalg.svds(A_sparse, k=10)`
- `np.linalg.eigvalsh(L)` -> `scipy.sparse.linalg.eigsh(L_sparse, k=20)`
- `np.linalg.pinv(L)` -> use Laplacian pseudoinverse via sparse CG solve
- `np.linalg.cond(L)` -> ratio of top/bottom eigenvalue from sparse solver

**Action 4 - Add a global research metric timeout.** Wrap each validator call with a
`signal.alarm()` or `concurrent.futures.ThreadPoolExecutor` with a 30-second timeout.
This prevents any single metric from blocking the entire run.

---

## Metrics That Are Safe at n=1000

The following research metrics have acceptable performance and need no changes:

| Metric | Why it is safe |
|---|---|
| `validate_betti_numbers` | O(n + E) using formula, no enumeration |
| `validate_euler_characteristic` | O(1) formula |
| `validate_homological_density` | O(n + E) |
| `validate_treewidth` | Uses degeneracy O(n * d) via core_number |
| `validate_dominating_set` | Greedy O(n * d) |
| `validate_independence_number` | Greedy O(n * d) |
| `validate_vertex_cover` | 2-approximation O(E) |
| `validate_graph_symmetry` | O(n log n) degree sort |
| `validate_prufer_canonical_form` | O(n log n) |
| `validate_prufer_entropy` | O(n log n) |
| `validate_cayley_complexity_bound` | O(1) formula |
| `validate_spanning_tree_coverage` | O(1) after MST |
| `validate_mutual_information` | O(packages^2 * size) |
| `validate_conditional_entropy` | O(n * d) |
| `validate_channel_capacity` | O(n) |
| `validate_markov_stationary` | O(n * E) PageRank |
| `validate_path_space_structure` | Limited to 20 nodes sample |
| `validate_univalence_property` | Limited to 30 nodes sample |
| `validate_morphism_composition` | O(n * d^2), caps result at 10 |
| `validate_gradient_flow` (non-problematic version) | O(E) edge gradient |
| `validate_heat_diffusion` | O(n^3) -> needs fix but can use sparse |
| `validate_resource_allocation` | O(n + E) |
| `validate_max_flow_min_cut` | O(3 sources * 3 sinks * max_flow) |
| `validate_random_walk` | O(100 * n^2) power iteration + O(n^3) eigvals -> needs fix |

---

## Conclusion

The research metric suite is academically comprehensive but was not designed with scalability
in mind. For production use on graphs with n > 200:

1. **Short-term:** Add size guards (n > threshold -> SKIP) to all NP-hard and O(n^3) metrics.
   Estimated effort: 2-3 hours. Would make the tool usable at n=1000 within minutes.

2. **Medium-term:** Replace dense matrix operations with sparse equivalents from
   `scipy.sparse.linalg`. Estimated effort: 1-2 days. Would reduce memory from GB to MB.

3. **Long-term:** Consider splitting the research group into `research-fast` (all O(n^2) or
   better) and `research-deep` (expensive metrics, only for small graphs). This would make the
   tool's performance characteristics explicit to users.
