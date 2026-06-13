"""
archlint validator - Architecture validation engine

DEPRECATED (Tier-3 museum, ADR-0002): research/museum only. Manual run, NOT in
the boevoy gate. The production path is the native Go detectors ('archlint scan':
SCC/cycles, dead-code, ISP, layering — soundness-gated). Structural, provable
metrics here are being ported to Go; magnitude/experimental metrics stay as
museum. Do not wire this into the agent loop or CI gate. See validator/README.md.

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
