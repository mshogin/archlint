"""
Compatibility wrapper for nx.simple_cycles with length_bound support.

networkx 3.1+ added `length_bound` parameter to simple_cycles(), which makes
cycle enumeration dramatically faster on large graphs by skipping cycles longer
than the bound.  This module provides a single helper that uses `length_bound`
when available and falls back to post-filtering on older networkx.
"""

import inspect
import networkx as nx

# Cache once at import time: does the installed networkx support length_bound?
_SIMPLE_CYCLES_HAS_LENGTH_BOUND = 'length_bound' in inspect.signature(nx.simple_cycles).parameters


def simple_cycles_bounded(G, max_length: int = 10):
    """Enumerate simple cycles up to *max_length* edges.

    Uses ``nx.simple_cycles(G, length_bound=max_length)`` when available
    (networkx >= 3.1), otherwise falls back to filtering cycles returned by
    the legacy API.  The fallback is still expensive for dense graphs but at
    least produces correct results on older installations.

    Args:
        G: A networkx directed graph.
        max_length: Maximum cycle length to include.  Cycles longer than this
            are silently skipped.  Default is 10 — cycles of 10+ edges are
            rare in typical architectural graphs and provide little signal.

    Returns:
        A generator of cycles (each cycle is a list of nodes).
    """
    if _SIMPLE_CYCLES_HAS_LENGTH_BOUND:
        return nx.simple_cycles(G, length_bound=max_length)
    else:
        # Fallback: post-filter (expensive but correct)
        return (c for c in nx.simple_cycles(G) if len(c) <= max_length)
