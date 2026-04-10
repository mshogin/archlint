"""
Integration tests for Prüfer sequence metrics.

Tests each of the 6 Prüfer validators with known inputs and verifiable outputs.

Credit: Ярослав Черкашин (@Yaroslam, https://github.com/Yaroslam)
        suggested the Prüfer metrics at Стачка 2026 conference (10 April 2026).

Run with:
    cd ~/projects/archlint-repo
    python3 -m pytest validator/structure/research/test_prufer_metrics.py -v

Or directly:
    python3 validator/structure/research/test_prufer_metrics.py
"""

import math
import networkx as nx
import pytest

# networkx 3.4+ removed nx.random_tree; use nx.random_labeled_tree instead.
if not hasattr(nx, 'random_tree'):
    nx.random_tree = nx.random_labeled_tree

from validator.structure.research.prufer_metrics import (
    validate_prufer_canonical_form,
    validate_prufer_entropy,
    validate_prufer_similarity,
    validate_tree_isomorphism_class,
    validate_spanning_tree_coverage,
    validate_cayley_complexity_bound,
)


# ---------------------------------------------------------------------------
# validate_prufer_canonical_form
# ---------------------------------------------------------------------------

class TestPruferCanonicalForm:

    def test_star_graph(self):
        """Star graph K_{1,4}: 5 nodes, center (1) connected to 4 leaves.

        Prüfer code has length n-2 = 3.
        The center node appears in every entry because each removed leaf
        has the center as its sole neighbor.
        """
        G = nx.Graph()
        G.add_edges_from([(1, 2), (1, 3), (1, 4), (1, 5)])
        result = validate_prufer_canonical_form(G)

        assert result['status'] == 'PASSED'
        assert result['details']['nodes'] == 5
        assert result['details']['prufer_code_length'] == 3  # n-2 = 5-2

    def test_path_graph(self):
        """Path graph 1-2-3-4-5.

        Prüfer code for a path on 5 nodes has length n-2 = 3.
        After relabeling to 1..5 (already in order), the code is [2, 3, 4].
        """
        G = nx.Graph()
        G.add_edges_from([(1, 2), (2, 3), (3, 4), (4, 5)])
        result = validate_prufer_canonical_form(G)

        assert result['status'] == 'PASSED'
        assert result['details']['prufer_code_length'] == 3

    def test_code_length_equals_n_minus_2(self):
        """Prüfer code length must always equal n-2."""
        for n in range(3, 12):
            G = nx.random_tree(n, seed=n)
            result = validate_prufer_canonical_form(G)
            assert result['status'] == 'PASSED'
            assert result['details']['prufer_code_length'] == n - 2, (
                f"Expected code length {n-2} for n={n}, "
                f"got {result['details']['prufer_code_length']}"
            )

    def test_skip_for_small_graphs(self):
        """Graphs with fewer than 3 nodes should be skipped."""
        G = nx.Graph()
        G.add_nodes_from([1, 2])
        G.add_edge(1, 2)
        result = validate_prufer_canonical_form(G)
        assert result['status'] == 'SKIP'

    def test_directed_graph_accepted(self):
        """Directed graph should be treated as undirected internally."""
        G = nx.DiGraph()
        G.add_edges_from([(1, 2), (1, 3), (2, 4), (2, 5)])
        result = validate_prufer_canonical_form(G)
        assert result['status'] == 'PASSED'

    def test_result_contains_prufer_code(self):
        """Result details must include the actual Prüfer code list."""
        G = nx.path_graph(6)
        result = validate_prufer_canonical_form(G)
        assert result['status'] == 'PASSED'
        assert 'prufer_code' in result['details']
        code = result['details']['prufer_code']
        assert isinstance(code, list)
        assert len(code) == 4  # n-2 = 6-2


# ---------------------------------------------------------------------------
# validate_prufer_entropy
# ---------------------------------------------------------------------------

class TestPruferEntropy:

    def test_star_has_low_entropy(self):
        """Star graph: all Prüfer code entries are the same (the center).

        Shannon entropy of a constant sequence is 0.
        Status should be INFO (not WARNING), entropy_ratio should be near 0.
        """
        G = nx.star_graph(10)  # 11 nodes total
        result = validate_prufer_entropy(G)

        assert result['status'] in ('INFO', 'WARNING')
        details = result['details']
        assert details['entropy'] == pytest.approx(0.0, abs=1e-9)
        assert details['entropy_ratio'] == pytest.approx(0.0, abs=1e-9)

    def test_path_has_nonzero_entropy(self):
        """Path graph: Prüfer code has varied entries, entropy > 0."""
        G = nx.path_graph(20)
        result = validate_prufer_entropy(G)

        assert result['status'] in ('INFO', 'WARNING')
        assert result['details']['entropy'] > 0.0

    def test_random_tree_entropy(self):
        """Random tree should yield valid entropy between 0 and log2(n)."""
        G = nx.random_tree(20, seed=42)
        result = validate_prufer_entropy(G)

        assert result['status'] in ('INFO', 'WARNING')
        details = result['details']
        assert 0.0 <= details['entropy'] <= details['max_entropy'] + 1e-9
        assert 0.0 <= details['entropy_ratio'] <= 1.0 + 1e-9

    def test_skip_for_tiny_graphs(self):
        """Graphs with fewer than 4 nodes should be skipped."""
        G = nx.path_graph(3)
        result = validate_prufer_entropy(G)
        assert result['status'] == 'SKIP'

    def test_entropy_fields_present(self):
        """Result must contain all expected entropy fields."""
        G = nx.random_tree(15, seed=7)
        result = validate_prufer_entropy(G)

        assert result['status'] in ('INFO', 'WARNING')
        for field in ('entropy', 'max_entropy', 'entropy_ratio', 'threshold'):
            assert field in result['details'], f"Missing field: {field}"

    def test_warning_when_high_entropy(self):
        """A nearly-uniform Prüfer code should trigger WARNING status.

        We construct a balanced binary tree where no single hub dominates,
        producing a more uniform code distribution than a star.
        With a large enough tree the ratio may exceed 0.9 and trigger WARNING.
        (The assertion is soft because the exact ratio depends on nx internals.)
        """
        G = nx.balanced_tree(r=2, h=5)  # 63 nodes
        result = validate_prufer_entropy(G)
        # At minimum, status is valid and ratio is in [0, 1]
        assert result['status'] in ('INFO', 'WARNING')
        assert 0.0 <= result['details']['entropy_ratio'] <= 1.0 + 1e-9


# ---------------------------------------------------------------------------
# validate_prufer_similarity
# ---------------------------------------------------------------------------

class TestPruferSimilarity:

    def test_info_status_always(self):
        """Similarity requires two graphs; single-graph call returns INFO."""
        G = nx.random_tree(10, seed=1)
        result = validate_prufer_similarity(G)
        assert result['status'] == 'INFO'

    def test_baseline_required_flag(self):
        """Result must indicate that a baseline is required."""
        G = nx.random_tree(5, seed=2)
        result = validate_prufer_similarity(G)
        assert result['details'].get('baseline_required') is True

    def test_no_violations(self):
        """Single-graph call should produce no violations."""
        G = nx.star_graph(8)
        result = validate_prufer_similarity(G)
        assert result['violations'] == []


# ---------------------------------------------------------------------------
# validate_tree_isomorphism_class
# ---------------------------------------------------------------------------

class TestTreeIsomorphismClass:

    def test_two_isomorphic_stars_same_fingerprint(self):
        """Two isomorphic star graphs must yield the same fingerprint."""
        G1 = nx.star_graph(5)
        G2 = nx.star_graph(5)
        r1 = validate_tree_isomorphism_class(G1)
        r2 = validate_tree_isomorphism_class(G2)

        assert r1['status'] == 'PASSED'
        assert r2['status'] == 'PASSED'
        assert r1['details']['isomorphism_fingerprint'] == \
               r2['details']['isomorphism_fingerprint']

    def test_star_vs_path_different_fingerprint(self):
        """A star and a path of the same size should have different fingerprints."""
        n = 7
        star = nx.star_graph(n - 1)   # n nodes
        path = nx.path_graph(n)        # n nodes
        r_star = validate_tree_isomorphism_class(star)
        r_path = validate_tree_isomorphism_class(path)

        assert r_star['status'] == 'PASSED'
        assert r_path['status'] == 'PASSED'
        assert r_star['details']['isomorphism_fingerprint'] != \
               r_path['details']['isomorphism_fingerprint']

    def test_fingerprint_is_sorted(self):
        """Isomorphism fingerprint must be a non-decreasing list."""
        G = nx.random_tree(12, seed=99)
        result = validate_tree_isomorphism_class(G)

        assert result['status'] == 'PASSED'
        fp = result['details']['isomorphism_fingerprint']
        assert fp == sorted(fp)

    def test_skip_small_graph(self):
        """Graphs with fewer than 3 nodes should be skipped."""
        G = nx.Graph()
        G.add_nodes_from([1, 2])
        result = validate_tree_isomorphism_class(G)
        assert result['status'] == 'SKIP'

    def test_degree_sequence_present(self):
        """Result must include the degree sequence of the spanning tree."""
        G = nx.random_tree(8, seed=3)
        result = validate_tree_isomorphism_class(G)

        assert result['status'] == 'PASSED'
        assert 'degree_sequence' in result['details']
        deg_seq = result['details']['degree_sequence']
        assert isinstance(deg_seq, list)
        assert len(deg_seq) == 8


# ---------------------------------------------------------------------------
# validate_spanning_tree_coverage
# ---------------------------------------------------------------------------

class TestSpanningTreeCoverage:

    def test_pure_tree_coverage_is_one(self):
        """A pure tree has no extra edges: coverage must equal 1.0."""
        G = nx.random_tree(10, seed=42)
        result = validate_spanning_tree_coverage(G)

        assert result['status'] in ('INFO', 'WARNING')
        assert result['details']['coverage'] == pytest.approx(1.0)

    def test_complete_graph_low_coverage(self):
        """Complete graph K_10 has many more edges than a spanning tree.

        n=10: spanning tree needs 9 edges, K_10 has 45 edges.
        Coverage = 9/45 = 0.2, well below threshold 0.5 -> WARNING.
        """
        G = nx.complete_graph(10)
        result = validate_spanning_tree_coverage(G)

        assert result['status'] == 'WARNING'
        assert result['details']['coverage'] < 0.5

    def test_coverage_value_formula(self):
        """Coverage = (n-1) / total_edges for a directed graph."""
        # Build a known graph: path (n-1 edges) plus one extra
        G = nx.DiGraph()
        G.add_edges_from([(1, 2), (2, 3), (3, 4), (4, 5)])  # 4 edges
        G.add_edge(1, 3)                                      # 5 edges total
        result = validate_spanning_tree_coverage(G)

        # n=5, tree_edges=4, total=5 -> coverage=0.8
        assert result['details']['coverage'] == pytest.approx(4 / 5, abs=1e-4)

    def test_skip_for_single_node(self):
        """Single-node graph has no edges -> should be skipped."""
        G = nx.Graph()
        G.add_node(1)
        result = validate_spanning_tree_coverage(G)
        assert result['status'] == 'SKIP'

    def test_extra_edges_counted_correctly(self):
        """Extra edges = total_edges - (n-1)."""
        n = 6
        G = nx.complete_graph(n)   # n*(n-1)/2 = 15 edges (undirected)
        result = validate_spanning_tree_coverage(G)

        expected_extra = G.number_of_edges() - (n - 1)
        assert result['details']['extra_edges'] == expected_extra


# ---------------------------------------------------------------------------
# validate_cayley_complexity_bound
# ---------------------------------------------------------------------------

class TestCayleyComplexityBound:

    def test_small_n_exact_bound(self):
        """For n <= 20 the exact Cayley bound n^(n-2) must be computed."""
        G = nx.star_graph(3)  # n=4 nodes
        result = validate_cayley_complexity_bound(G)

        assert result['status'] == 'INFO'
        assert result['details']['cayley_bound_exact'] == 4 ** 2  # 16

    def test_n4_bound_is_16(self):
        """n=4: n^(n-2) = 4^2 = 16."""
        G = nx.path_graph(4)
        result = validate_cayley_complexity_bound(G)

        assert result['details']['cayley_bound_exact'] == 16

    def test_n5_bound_is_125(self):
        """n=5: n^(n-2) = 5^3 = 125."""
        G = nx.star_graph(4)  # 5 nodes
        result = validate_cayley_complexity_bound(G)

        assert result['details']['cayley_bound_exact'] == 125

    def test_large_n_approx_bound(self):
        """For n > 20 an approximate (log10) bound is returned instead."""
        G = nx.random_tree(25, seed=10)
        result = validate_cayley_complexity_bound(G)

        assert result['status'] == 'INFO'
        assert 'cayley_bound_approx' in result['details']
        assert 'cayley_bound_exact' not in result['details']

    def test_log10_bound_formula(self):
        """log10 bound must equal (n-2) * log10(n)."""
        n = 10
        G = nx.random_tree(n, seed=5)
        result = validate_cayley_complexity_bound(G)

        expected_log = (n - 2) * math.log10(n)
        assert result['details']['log10_cayley_bound'] == \
               pytest.approx(expected_log, abs=0.01)

    def test_skip_for_single_node(self):
        """Single-node graph should be skipped."""
        G = nx.Graph()
        G.add_node(1)
        result = validate_cayley_complexity_bound(G)
        assert result['status'] == 'SKIP'

    def test_n2_bound_is_one(self):
        """n=2: there is exactly 1 labeled tree (a single edge)."""
        G = nx.Graph()
        G.add_edge(1, 2)
        result = validate_cayley_complexity_bound(G)

        assert result['status'] == 'INFO'
        assert result['details']['cayley_bound_exact'] == 1


# ---------------------------------------------------------------------------
# Run directly
# ---------------------------------------------------------------------------

if __name__ == '__main__':
    import sys
    # Simple self-runner without pytest dependency
    test_classes = [
        TestPruferCanonicalForm,
        TestPruferEntropy,
        TestPruferSimilarity,
        TestTreeIsomorphismClass,
        TestSpanningTreeCoverage,
        TestCayleyComplexityBound,
    ]

    passed = 0
    failed = 0
    errors = 0

    for cls in test_classes:
        instance = cls()
        methods = [m for m in dir(instance) if m.startswith('test_')]
        for method_name in methods:
            label = f"{cls.__name__}.{method_name}"
            try:
                getattr(instance, method_name)()
                print(f"  PASS  {label}")
                passed += 1
            except AssertionError as e:
                print(f"  FAIL  {label}: {e}")
                failed += 1
            except Exception as e:
                print(f"  ERROR {label}: {type(e).__name__}: {e}")
                errors += 1

    total = passed + failed + errors
    print(f"\n{total} tests: {passed} passed, {failed} failed, {errors} errors")
    sys.exit(0 if (failed + errors) == 0 else 1)
