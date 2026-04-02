"""
Advanced behavior validators

- single_point_of_failure: Components in ALL contexts
- context_coupling: Contexts sharing too many components
- layer_traversal: Layer violations in execution flow
- context_depth: Call stack depth in context
"""

from validator.behavior.advanced.context_metrics import (
    validate_single_point_of_failure,
    validate_context_coupling,
    validate_layer_traversal,
    validate_context_depth,
)

__all__ = [
    'validate_single_point_of_failure',
    'validate_context_coupling',
    'validate_layer_traversal',
    'validate_context_depth',
]

ADVANCED_BEHAVIOR_VALIDATORS = [
    validate_single_point_of_failure,
    validate_context_coupling,
    validate_layer_traversal,
    validate_context_depth,
]
