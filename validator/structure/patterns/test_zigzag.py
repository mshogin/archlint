"""
Tests for validate_zigzag_coupling validator.

The zigzag pattern occurs when a caller function calls the same *component*
(package/class prefix) non-adjacently in its call sequence:

    Caller -> b.DoFirst   # component b
    Caller -> c.Process   # component c
    Caller -> b.DoSecond  # component b (zigzag!)
    Caller -> d.Finalize  # component d

In the graph, targets are distinct nodes (different methods), but they share
the same component prefix. The validator maps each target to its component and
checks for non-adjacent repeats.

Covers:
- clean sequences with no zigzag
- simple zigzag [b.Do1, c.Do1, b.Do2] -> component b zigzags
- adjacent repeats that are NOT violations [b.Do1, b.Do2, c.Do1]
- multiple zigzags in one caller
- threshold configuration
- exclude list
"""

import networkx as nx
import pytest

from validator.structure.patterns.patterns_metrics import validate_zigzag_coupling
from validator.config import RuleConfig


def _build_callgraph(caller: str, targets: list) -> nx.DiGraph:
    """
    Build a directed graph simulating a callgraph.

    Each target is a distinct method node (e.g. 'b.DoFirst', 'c.Process').
    The validator maps 'b.DoFirst' -> component 'b', 'c.Process' -> component 'c'.
    Targets without a dot are used as-is.

    To mark nodes as method-level (so the validator strips the last segment),
    we set entity='method' on each target node.
    """
    g = nx.DiGraph()
    g.add_node(caller, entity='function')
    for t in targets:
        g.add_node(t, entity='method')
        g.add_edge(caller, t)
    return g


def _make_config(threshold: int = 0, error_on_violation: bool = True, exclude: list = None) -> RuleConfig:
    return RuleConfig(
        enabled=True,
        threshold=threshold,
        error_on_violation=error_on_violation,
        exclude=exclude or [],
    )


class TestNoZigzag:
    def test_clean_sequence_abcd(self):
        """[a.X, b.X, c.X, d.X] - each component appears once, no zigzag."""
        g = _build_callgraph('pkg.Caller', ['a.Method', 'b.Method', 'c.Method', 'd.Method'])
        result = validate_zigzag_coupling(g, _make_config())
        assert result['status'] == 'PASSED'
        assert result['violations_count'] == 0

    def test_single_edge_no_violation(self):
        """Single outgoing edge cannot produce zigzag."""
        g = _build_callgraph('pkg.Caller', ['a.Method'])
        result = validate_zigzag_coupling(g, _make_config())
        assert result['status'] == 'PASSED'
        assert result['violations_count'] == 0

    def test_two_edges_no_violation(self):
        """Two outgoing edges cannot produce zigzag (need at least 3 for [X,Y,X])."""
        g = _build_callgraph('pkg.Caller', ['a.Method1', 'b.Method1'])
        result = validate_zigzag_coupling(g, _make_config())
        assert result['status'] == 'PASSED'
        assert result['violations_count'] == 0

    def test_empty_graph_no_violation(self):
        """Empty graph produces no violations."""
        g = nx.DiGraph()
        result = validate_zigzag_coupling(g, _make_config())
        assert result['status'] == 'PASSED'
        assert result['violations_count'] == 0


class TestSimpleZigzag:
    def test_aba_is_zigzag(self):
        """[a.Do1, b.Do1, a.Do2] - component 'a' at 0 and 2 with 'b' between = zigzag."""
        g = _build_callgraph('pkg.Caller', ['a.Do1', 'b.Do1', 'a.Do2'])
        result = validate_zigzag_coupling(g, _make_config())
        assert result['status'] == 'ERROR'
        assert result['violations_count'] == 1
        violation = result['violations'][0]
        assert violation['caller'] == 'pkg.Caller'
        assert violation['zigzag_count'] == 1

    def test_abac_has_one_zigzag(self):
        """[a.Do1, b.Do1, a.Do2, c.Do1] - a zigzags once (positions 0 and 2)."""
        g = _build_callgraph('pkg.Caller', ['a.Do1', 'b.Do1', 'a.Do2', 'c.Do1'])
        result = validate_zigzag_coupling(g, _make_config())
        assert result['status'] == 'ERROR'
        assert result['violations_count'] == 1

    def test_abca_is_zigzag(self):
        """[a.Do1, b.Do1, c.Do1, a.Do2] - 'a' at 0 and 3 with b, c between."""
        g = _build_callgraph('pkg.Caller', ['a.Do1', 'b.Do1', 'c.Do1', 'a.Do2'])
        result = validate_zigzag_coupling(g, _make_config())
        assert result['status'] == 'ERROR'
        assert result['violations_count'] == 1


class TestAdjacentRepeat:
    def test_aab_adjacent_not_zigzag(self):
        """[a.Do1, a.Do2, b.Do1] - adjacent 'a' calls at positions 0,1 - NOT a zigzag."""
        g = _build_callgraph('pkg.Caller', ['a.Do1', 'a.Do2', 'b.Do1'])
        result = validate_zigzag_coupling(g, _make_config())
        assert result['status'] == 'PASSED'
        assert result['violations_count'] == 0

    def test_aabb_adjacent_not_zigzag(self):
        """[a.Do1, a.Do2, b.Do1, b.Do2] - two pairs of adjacent calls - NOT zigzags."""
        g = _build_callgraph('pkg.Caller', ['a.Do1', 'a.Do2', 'b.Do1', 'b.Do2'])
        result = validate_zigzag_coupling(g, _make_config())
        assert result['status'] == 'PASSED'
        assert result['violations_count'] == 0

    def test_aaab_triple_adjacent_not_zigzag(self):
        """[a.Do1, a.Do2, a.Do3, b.Do1] - three adjacent 'a' calls - NOT a zigzag."""
        g = _build_callgraph('pkg.Caller', ['a.Do1', 'a.Do2', 'a.Do3', 'b.Do1'])
        result = validate_zigzag_coupling(g, _make_config())
        assert result['status'] == 'PASSED'
        assert result['violations_count'] == 0


class TestMultipleZigzags:
    def test_ababa_has_multiple_zigzags(self):
        """[a.Do1, b.Do1, a.Do2, b.Do2, a.Do3] - a and b both zigzag."""
        g = _build_callgraph('pkg.Caller', ['a.Do1', 'b.Do1', 'a.Do2', 'b.Do2', 'a.Do3'])
        result = validate_zigzag_coupling(g, _make_config())
        assert result['status'] == 'ERROR'
        assert result['violations_count'] == 1  # one caller
        violation = result['violations'][0]
        # a zigzags at (0,2) and (2,4), b zigzags at (1,3) -> at least 2
        assert violation['zigzag_count'] >= 2

    def test_multiple_callers_independent(self):
        """Each caller is checked independently."""
        g = nx.DiGraph()
        # Caller1: [a.Do1, b.Do1, a.Do2] -> 1 zigzag
        g.add_node('pkg.Caller1', entity='function')
        g.add_node('a.Do1', entity='method')
        g.add_node('b.Do1', entity='method')
        g.add_node('a.Do2', entity='method')
        g.add_edge('pkg.Caller1', 'a.Do1')
        g.add_edge('pkg.Caller1', 'b.Do1')
        g.add_edge('pkg.Caller1', 'a.Do2')
        # Caller2: [c.Do1, d.Do1] -> 0 zigzags
        g.add_node('pkg.Caller2', entity='function')
        g.add_node('c.Do1', entity='method')
        g.add_node('d.Do1', entity='method')
        g.add_edge('pkg.Caller2', 'c.Do1')
        g.add_edge('pkg.Caller2', 'd.Do1')
        result = validate_zigzag_coupling(g, _make_config())
        assert result['status'] == 'ERROR'
        assert result['violations_count'] == 1
        assert result['violations'][0]['caller'] == 'pkg.Caller1'


class TestThreshold:
    def test_threshold_zero_catches_one_zigzag(self):
        """threshold=0 means any zigzag is a violation."""
        g = _build_callgraph('pkg.Caller', ['a.Do1', 'b.Do1', 'a.Do2'])
        result = validate_zigzag_coupling(g, _make_config(threshold=0))
        assert result['violations_count'] == 1

    def test_threshold_one_allows_one_zigzag(self):
        """threshold=1 allows up to 1 zigzag per function."""
        g = _build_callgraph('pkg.Caller', ['a.Do1', 'b.Do1', 'a.Do2'])
        result = validate_zigzag_coupling(g, _make_config(threshold=1))
        assert result['status'] == 'PASSED'
        assert result['violations_count'] == 0

    def test_threshold_one_catches_two_zigzags(self):
        """threshold=1: caller with 2+ zigzags still violates."""
        g = _build_callgraph('pkg.Caller', ['a.Do1', 'b.Do1', 'a.Do2', 'b.Do2', 'a.Do3'])
        result = validate_zigzag_coupling(g, _make_config(threshold=1))
        assert result['violations_count'] == 1

    def test_threshold_large_no_violation(self):
        """Very large threshold: no violations regardless of pattern."""
        g = _build_callgraph('pkg.Caller', ['a.Do1', 'b.Do1', 'a.Do2', 'b.Do2', 'a.Do3'])
        result = validate_zigzag_coupling(g, _make_config(threshold=100))
        assert result['status'] == 'PASSED'
        assert result['violations_count'] == 0


class TestExcludeList:
    def test_exclude_caller_skipped(self):
        """Excluded callers are not checked."""
        g = _build_callgraph('pkg.Caller', ['a.Do1', 'b.Do1', 'a.Do2'])
        result = validate_zigzag_coupling(g, _make_config(exclude=['pkg.Caller']))
        assert result['status'] == 'PASSED'
        assert result['violations_count'] == 0

    def test_exclude_wildcard(self):
        """Wildcard exclude pattern matching multiple callers."""
        g = _build_callgraph('pkg.Caller', ['a.Do1', 'b.Do1', 'a.Do2'])
        result = validate_zigzag_coupling(g, _make_config(exclude=['pkg.*']))
        assert result['status'] == 'PASSED'
        assert result['violations_count'] == 0

    def test_exclude_suffix_wildcard(self):
        """Suffix wildcard: *Caller excludes pkg.Caller."""
        g = _build_callgraph('pkg.Caller', ['a.Do1', 'b.Do1', 'a.Do2'])
        result = validate_zigzag_coupling(g, _make_config(exclude=['*Caller']))
        assert result['status'] == 'PASSED'
        assert result['violations_count'] == 0


class TestReturnFormat:
    def test_result_has_required_keys(self):
        """Result dict must have name, description, status, threshold, violations, violations_count."""
        g = _build_callgraph('pkg.Caller', ['a.Method', 'b.Method', 'c.Method'])
        result = validate_zigzag_coupling(g, _make_config())
        assert 'name' in result
        assert 'description' in result
        assert 'status' in result
        assert 'threshold' in result
        assert 'violations' in result
        assert 'violations_count' in result
        assert result['name'] == 'zigzag_coupling'

    def test_violation_has_required_keys(self):
        """Each violation must have caller, zigzag_count, sequence, zigzags."""
        g = _build_callgraph('pkg.Caller', ['a.Do1', 'b.Do1', 'a.Do2'])
        result = validate_zigzag_coupling(g, _make_config())
        assert result['violations_count'] == 1
        v = result['violations'][0]
        assert 'caller' in v
        assert 'zigzag_count' in v
        assert 'sequence' in v
        assert 'zigzags' in v

    def test_warning_mode(self):
        """error_on_violation=False produces WARNING status, not ERROR."""
        g = _build_callgraph('pkg.Caller', ['a.Do1', 'b.Do1', 'a.Do2'])
        result = validate_zigzag_coupling(g, _make_config(error_on_violation=False))
        assert result['status'] == 'WARNING'

    def test_no_config_uses_defaults(self):
        """Calling without config uses safe defaults (threshold=0, error_on_violation=True)."""
        g = _build_callgraph('pkg.Caller', ['a.Do1', 'b.Do1', 'a.Do2'])
        result = validate_zigzag_coupling(g)
        assert result['name'] == 'zigzag_coupling'
        assert result['status'] in ('PASSED', 'ERROR', 'WARNING')

    def test_zigzag_detail_has_positions_and_between(self):
        """Each zigzag detail entry must have component, positions, sequence_between."""
        g = _build_callgraph('pkg.Caller', ['a.Do1', 'b.Do1', 'a.Do2'])
        result = validate_zigzag_coupling(g, _make_config())
        assert result['violations_count'] == 1
        zigzag = result['violations'][0]['zigzags'][0]
        assert 'component' in zigzag
        assert 'positions' in zigzag
        assert 'sequence_between' in zigzag
        assert zigzag['component'] == 'a'
        assert zigzag['positions'] == [0, 2]
        assert zigzag['sequence_between'] == ['b']
