"""
Structure validators - Static graph analysis

Groups:
- core: dag_check, fan_out, coupling, layers, forbidden_deps
- solid: SRP, OCP, LSP, DIP, ISP
- patterns: God Class, Shotgun Surgery, Feature Envy
- architecture: Domain isolation, Ports & Adapters, DTO boundaries
- quality: Auth boundary, sensitive data flow, mockability
- advanced: Centrality, modularity, change propagation
- research: Topology, information theory, category theory, etc.
"""

from validator.structure import core
from validator.structure import solid
from validator.structure import patterns
from validator.structure import architecture
from validator.structure import quality
from validator.structure import advanced
from validator.structure import research

__all__ = [
    'core',
    'solid',
    'patterns',
    'architecture',
    'quality',
    'advanced',
    'research',
]
