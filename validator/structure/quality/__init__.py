"""
Quality and security validators

- Auth boundary
- Sensitive data flow
- Input validation layer
- Mockability
- Test isolation
- Test coverage structure
- Logging coverage
- Metrics exposure
- Health check presence
- API consistency
"""

from validator.structure.quality.quality_metrics import (
    validate_auth_boundary,
    validate_sensitive_data_flow,
    validate_input_validation_layer,
    validate_mockability,
    validate_test_isolation,
    validate_test_coverage_structure,
    validate_logging_coverage,
    validate_metrics_exposure,
    validate_health_check_presence,
    validate_api_consistency,
)

__all__ = [
    'validate_auth_boundary',
    'validate_sensitive_data_flow',
    'validate_input_validation_layer',
    'validate_mockability',
    'validate_test_isolation',
    'validate_test_coverage_structure',
    'validate_logging_coverage',
    'validate_metrics_exposure',
    'validate_health_check_presence',
    'validate_api_consistency',
]

QUALITY_VALIDATORS = [
    validate_auth_boundary,
    validate_sensitive_data_flow,
    validate_input_validation_layer,
    validate_mockability,
    validate_test_isolation,
    validate_test_coverage_structure,
    validate_logging_coverage,
    validate_metrics_exposure,
    validate_health_check_presence,
    validate_api_consistency,
]
