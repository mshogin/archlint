"""
Core behavior validators - Context/trace analysis

- context_coverage: Critical components covered by tests
- untested_components: Components not in any context
- ghost_components: Components in contexts but not in architecture
- context_complexity: Too many components in one context
"""

from validator.behavior.core.context_metrics import (
    validate_context_coverage,
    validate_untested_components,
    validate_ghost_components,
    validate_context_complexity,
)

__all__ = [
    'validate_context_coverage',
    'validate_untested_components',
    'validate_ghost_components',
    'validate_context_complexity',
]

CORE_BEHAVIOR_VALIDATORS = [
    validate_context_coverage,
    validate_untested_components,
    validate_ghost_components,
    validate_context_complexity,
]
