"""
Clean/Hexagonal architecture validators

- Domain isolation
- Ports & Adapters
- Use case purity
- DTO boundaries
- Inward dependencies
- Bounded context isolation
- Service autonomy
"""

from validator.structure.architecture.architecture_metrics import (
    validate_domain_isolation,
    validate_ports_adapters,
    validate_use_case_purity,
    validate_dto_boundaries,
    validate_inward_dependencies,
    validate_bounded_context_leakage,
    validate_service_autonomy,
)

__all__ = [
    'validate_domain_isolation',
    'validate_ports_adapters',
    'validate_use_case_purity',
    'validate_dto_boundaries',
    'validate_inward_dependencies',
    'validate_bounded_context_leakage',
    'validate_service_autonomy',
]

ARCHITECTURE_VALIDATORS = [
    validate_domain_isolation,
    validate_ports_adapters,
    validate_use_case_purity,
    validate_dto_boundaries,
    validate_inward_dependencies,
    validate_bounded_context_leakage,
    validate_service_autonomy,
]
