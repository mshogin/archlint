"""
Code smell and anti-pattern validators

- God Class
- Shotgun Surgery
- Divergent Change
- Lazy Class
- Middle Man
- Speculative Generality
- Data Clumps
- Feature Envy
- Zigzag Coupling
"""

from validator.structure.patterns.patterns_metrics import (
    validate_god_class,
    validate_feature_envy,
    validate_shotgun_surgery,
    validate_divergent_change,
    validate_lazy_class,
    validate_middle_man,
    validate_speculative_generality,
    validate_data_clumps,
    validate_zigzag_coupling,
)

__all__ = [
    'validate_god_class',
    'validate_feature_envy',
    'validate_shotgun_surgery',
    'validate_divergent_change',
    'validate_lazy_class',
    'validate_middle_man',
    'validate_speculative_generality',
    'validate_data_clumps',
    'validate_zigzag_coupling',
]

PATTERN_VALIDATORS = [
    validate_god_class,
    validate_feature_envy,
    validate_shotgun_surgery,
    validate_divergent_change,
    validate_lazy_class,
    validate_middle_man,
    validate_speculative_generality,
    validate_data_clumps,
    validate_zigzag_coupling,
]
