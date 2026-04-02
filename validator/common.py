"""
Common utilities for validators
"""

from typing import List


def get_violation_status(error_on_violation: bool) -> str:
    """Returns status on violation: ERROR or WARNING"""
    return 'ERROR' if error_on_violation else 'WARNING'


def is_excluded(node: str, exclude_patterns: List[str]) -> bool:
    """Checks if node matches any exclusion pattern"""
    if not exclude_patterns:
        return False

    node_lower = node.lower()
    for pattern in exclude_patterns:
        pattern_lower = pattern.lower()
        if pattern_lower.endswith('*'):
            if node_lower.startswith(pattern_lower[:-1]):
                return True
        elif pattern_lower.startswith('*'):
            if node_lower.endswith(pattern_lower[1:]):
                return True
        elif '*' in pattern_lower:
            parts = pattern_lower.split('*', 1)
            if node_lower.startswith(parts[0]) and node_lower.endswith(parts[1]):
                return True
        else:
            if pattern_lower in node_lower:
                return True
    return False
