"""
SOLID principles validators

- SRP: Single Responsibility Principle
- OCP: Open/Closed Principle
- LSP: Liskov Substitution Principle
- DIP: Dependency Inversion Principle
- ISP: Interface Segregation Principle (from advanced)
"""

from validator.structure.solid.solid_metrics import (
    validate_single_responsibility,
    validate_open_closed,
    validate_liskov_substitution,
    validate_dependency_inversion,
)

# ISP is in advanced_metrics
from validator.structure.advanced.advanced_metrics import (
    validate_interface_segregation,
)

__all__ = [
    'validate_single_responsibility',
    'validate_open_closed',
    'validate_liskov_substitution',
    'validate_dependency_inversion',
    'validate_interface_segregation',
]

SOLID_VALIDATORS = [
    validate_single_responsibility,
    validate_open_closed,
    validate_liskov_substitution,
    validate_dependency_inversion,
    validate_interface_segregation,
]
