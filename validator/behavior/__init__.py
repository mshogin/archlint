"""
Behavior validators - Dynamic analysis (contexts/traces)

Groups:
- core: coverage, untested, ghost, complexity
- advanced: SPOF, coupling, layer traversal, depth
"""

from validator.behavior import core
from validator.behavior import advanced

__all__ = ['core', 'advanced']
