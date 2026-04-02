"""
archlint validator - Architecture validation engine

Two families of validators:
- structure: Static graph analysis (dependencies, coupling, layers)
- behavior: Dynamic analysis (test coverage, execution flows)

Each family has groups:
- core: Maximum value, minimum time
- solid/patterns/architecture/quality/advanced/research: Specialized validators
"""

from validator import structure
from validator import behavior

__all__ = ['structure', 'behavior']
