"""
Research/experimental validators - Mathematical approaches

Modules:
- topology_metrics: Betti numbers, Euler characteristic, simplicial complexity
- information_theory_metrics: Entropy, mutual information, channel capacity
- linear_algebra_metrics: SVD, spectral analysis, Lyapunov stability
- advanced_graph_metrics: Treewidth, chromatic number, dominating set
- advanced_topology_metrics: Extended topological analysis
- mathematical_analysis_metrics: Gradient flow, heat diffusion, Fourier
- integral_calculus_metrics: Path integrals, Green function, Stokes theorem
- set_theory_metrics: Relations, partial orders, lattice structure
- category_theory_metrics: Morphisms, functors, monads, adjunctions
- game_theory_metrics: Shapley value, Nash equilibrium, ESS
- combinatorics_metrics: Generating functions, Ramsey analysis
- optimization_metrics: Optimization-based analysis
- automata_theory_metrics: Automata-based analysis
- number_theory_metrics: Number-theoretic analysis
- probability_metrics: Probabilistic analysis
- hott_metrics: Homotopy Type Theory (HoTT) - univalence, path spaces, identity types
"""

from validator.structure.research import topology_metrics
from validator.structure.research import information_theory_metrics
from validator.structure.research import linear_algebra_metrics
from validator.structure.research import advanced_graph_metrics
from validator.structure.research import advanced_topology_metrics
from validator.structure.research import mathematical_analysis_metrics
from validator.structure.research import integral_calculus_metrics
from validator.structure.research import set_theory_metrics
from validator.structure.research import category_theory_metrics
from validator.structure.research import game_theory_metrics
from validator.structure.research import combinatorics_metrics
from validator.structure.research import optimization_metrics
from validator.structure.research import automata_theory_metrics
from validator.structure.research import number_theory_metrics
from validator.structure.research import probability_metrics
from validator.structure.research import hott_metrics

__all__ = [
    'topology_metrics',
    'information_theory_metrics',
    'linear_algebra_metrics',
    'advanced_graph_metrics',
    'advanced_topology_metrics',
    'mathematical_analysis_metrics',
    'integral_calculus_metrics',
    'set_theory_metrics',
    'category_theory_metrics',
    'game_theory_metrics',
    'combinatorics_metrics',
    'optimization_metrics',
    'automata_theory_metrics',
    'number_theory_metrics',
    'probability_metrics',
    'hott_metrics',
]
