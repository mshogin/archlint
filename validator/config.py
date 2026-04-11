"""
Загрузка и управление конфигурацией валидатора
"""

import os
import yaml
from typing import Dict, List, Any, Optional
from dataclasses import dataclass, field


@dataclass
class RuleConfig:
    """Конфигурация одного правила валидации"""
    enabled: bool = True
    threshold: Any = None  # Порог для правила (если применимо)
    error_on_violation: bool = True  # ERROR вместо WARNING при нарушении
    exclude: List[str] = field(default_factory=list)  # Исключения (паттерны)
    params: Dict[str, Any] = field(default_factory=dict)  # Дополнительные параметры


@dataclass
class Config:
    """Полная конфигурация валидатора"""

    # Core checks
    dag_check: RuleConfig = field(default_factory=lambda: RuleConfig())
    max_fan_out: RuleConfig = field(default_factory=lambda: RuleConfig(threshold=5))
    modularity: RuleConfig = field(default_factory=lambda: RuleConfig(threshold=0.3))

    # Centrality metrics
    betweenness_centrality: RuleConfig = field(default_factory=lambda: RuleConfig(threshold=0.3))
    pagerank: RuleConfig = field(default_factory=lambda: RuleConfig(threshold=0.1, error_on_violation=False))

    # Coupling metrics
    coupling: RuleConfig = field(default_factory=lambda: RuleConfig(
        params={'ca_threshold': 10, 'ce_threshold': 10}
    ))
    instability: RuleConfig = field(default_factory=lambda: RuleConfig())

    # Structural checks
    orphan_nodes: RuleConfig = field(default_factory=lambda: RuleConfig())
    strongly_connected_components: RuleConfig = field(default_factory=lambda: RuleConfig(
        params={'max_size': 1}
    ))
    graph_depth: RuleConfig = field(default_factory=lambda: RuleConfig(threshold=10))
    hub_nodes: RuleConfig = field(default_factory=lambda: RuleConfig(threshold=10))

    # Architectural rules
    layer_violations: RuleConfig = field(default_factory=lambda: RuleConfig(
        params={'layers': {
            'cmd': 0,
            'api': 1,
            'handler': 1,
            'controller': 1,
            'service': 2,
            'usecase': 2,
            'internal': 2,
            'domain': 3,
            'entity': 3,
            'model': 3,
            'repository': 4,
            'storage': 4,
            'infrastructure': 5,
            'pkg': 6,
        }}
    ))
    forbidden_dependencies: RuleConfig = field(default_factory=lambda: RuleConfig(
        params={'rules': [
            {'from': 'handler', 'to': 'repository'},
            {'from': 'controller', 'to': 'repository'},
            {'from': 'api', 'to': 'storage'},
            {'from': 'model', 'to': 'service'},
            {'from': 'entity', 'to': 'repository'},
        ]}
    ))
    component_distance: RuleConfig = field(default_factory=lambda: RuleConfig(threshold=5))

    # Design quality
    abstractness: RuleConfig = field(default_factory=lambda: RuleConfig(
        params={'min_threshold': 0.1, 'max_threshold': 0.8,
                'patterns': ['interface', 'abstract', 'base', 'contract', 'port']}
    ))
    distance_from_main_sequence: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.5
    ))

    # Context validations (behavioral)
    context_coverage: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.8,  # 80% критических компонентов должны быть в контекстах
        params={'top_n': 10}
    ))
    untested_components: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.5,  # Максимум 50% непокрытых компонентов
        error_on_violation=False  # INFO по умолчанию
    ))
    ghost_components: RuleConfig = field(default_factory=lambda: RuleConfig())
    single_point_of_failure: RuleConfig = field(default_factory=lambda: RuleConfig(
        error_on_violation=False,  # INFO по умолчанию
        params={'min_contexts': 3}
    ))
    context_complexity: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=15  # Максимум 15 компонентов в контексте
    ))
    context_coupling: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.7,  # Максимум 70% общих компонентов
        error_on_violation=False  # INFO по умолчанию
    ))
    layer_traversal: RuleConfig = field(default_factory=lambda: RuleConfig(
        params={'layers': {
            'cmd': 0, 'api': 1, 'handler': 1, 'controller': 1,
            'service': 2, 'usecase': 2,
            'domain': 3, 'entity': 3, 'model': 3,
            'repository': 4, 'storage': 4,
            'infrastructure': 5, 'pkg': 6,
        }}
    ))
    context_depth: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=10  # Максимальная глубина контекста
    ))

    # =========================
    # Advanced Graph Theory
    # =========================
    clustering_coefficient: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.1,  # Минимальный средний коэффициент кластеризации
        error_on_violation=False
    ))
    edge_density: RuleConfig = field(default_factory=lambda: RuleConfig(
        params={'min_threshold': 0.01, 'max_threshold': 0.3}
    ))
    articulation_points: RuleConfig = field(default_factory=lambda: RuleConfig(
        error_on_violation=False  # INFO по умолчанию
    ))
    bridge_edges: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True
    ))
    graph_diameter: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=10,  # Максимальный диаметр графа
        error_on_violation=True
    ))
    avg_path_length: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=5.0,  # Максимальная средняя длина пути
        error_on_violation=False
    ))
    closeness_centrality: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.5,  # Порог для центральных узлов
        enabled=True
    ))
    eigenvector_centrality: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.3,  # Порог для влиятельных узлов
        error_on_violation=False
    ))
    k_core_decomposition: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=5,  # Максимальное k-ядро
        error_on_violation=False
    ))
    graph_cliques: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=4,  # Максимальный размер клики
        error_on_violation=True
    ))

    # =========================
    # Statistics Validations
    # =========================
    degree_distribution: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True  # Информационная метрика
    ))
    dependency_entropy: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=2.0,  # Минимальная энтропия
        error_on_violation=False
    ))
    gini_coefficient: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.6,  # Максимальный коэффициент Джини
        error_on_violation=True
    ))
    zscore_outliers: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=3.0,  # Z-порог для выбросов
        error_on_violation=True
    ))

    # =========================
    # Spectral Analysis
    # =========================
    algebraic_connectivity: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.1,  # Минимальная алгебраическая связность
        error_on_violation=False
    ))
    spectral_radius: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True  # Информационная метрика
    ))

    # =========================
    # Software Architecture Metrics
    # =========================
    cohesion_lcom4: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=1,  # Максимум 1 компонент связности в типе
        error_on_violation=True
    ))
    interface_segregation: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=5,  # Максимум методов на интерфейс (ISP)
        error_on_violation=True
    ))

    # =========================
    # SOLID Principles
    # =========================
    single_responsibility: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=3,  # Максимум доменов-зависимостей
        error_on_violation=True
    ))
    open_closed: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.2,  # Минимум 20% абстракций
        error_on_violation=False
    ))
    liskov_substitution: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=3,  # Максимум лишних зависимостей у реализации
        error_on_violation=False
    ))
    dependency_inversion: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.3,  # Минимум 30% зависимостей на абстракции
        error_on_violation=True
    ))

    # =========================
    # Design Patterns
    # =========================
    god_class: RuleConfig = field(default_factory=lambda: RuleConfig(
        error_on_violation=True,
        params={'max_methods': 20, 'max_dependencies': 15}
    ))
    feature_envy: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.5,
        error_on_violation=True
    ))
    shotgun_surgery: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=10,  # Максимум зависящих компонентов
        error_on_violation=True
    ))
    divergent_change: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=3,  # Максимум доменов
        error_on_violation=True
    ))
    lazy_class: RuleConfig = field(default_factory=lambda: RuleConfig(
        error_on_violation=False,
        params={'min_methods': 2, 'min_dependencies': 1}
    ))
    middle_man: RuleConfig = field(default_factory=lambda: RuleConfig(
        error_on_violation=False
    ))
    speculative_generality: RuleConfig = field(default_factory=lambda: RuleConfig(
        error_on_violation=False
    ))
    data_clumps: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=3,  # Минимум совместных появлений
        error_on_violation=False
    ))
    zigzag_coupling: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0,  # max allowed zigzag occurrences per function
        error_on_violation=True
    ))

    # =========================
    # Clean Architecture
    # =========================
    domain_isolation: RuleConfig = field(default_factory=lambda: RuleConfig(
        error_on_violation=True,
        params={'forbidden_patterns': ['database', 'db', 'sql', 'http', 'grpc', 'redis']}
    ))
    ports_adapters: RuleConfig = field(default_factory=lambda: RuleConfig(
        error_on_violation=True
    ))
    use_case_purity: RuleConfig = field(default_factory=lambda: RuleConfig(
        error_on_violation=True
    ))
    dto_boundaries: RuleConfig = field(default_factory=lambda: RuleConfig(
        error_on_violation=False
    ))
    inward_dependencies: RuleConfig = field(default_factory=lambda: RuleConfig(
        error_on_violation=True
    ))
    bounded_context_leakage: RuleConfig = field(default_factory=lambda: RuleConfig(
        error_on_violation=True
    ))
    service_autonomy: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=3,  # Максимум синхронных зависимостей
        error_on_violation=True
    ))

    # =========================
    # Change Impact
    # =========================
    change_propagation: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=20,  # Максимум затронутых компонентов
        error_on_violation=True
    ))
    blast_radius: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.3,
        error_on_violation=True
    ))
    hotspot_detection: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=10,
        error_on_violation=True
    ))
    deprecated_usage: RuleConfig = field(default_factory=lambda: RuleConfig(
        error_on_violation=True,
        params={'patterns': ['deprecated', 'legacy', 'old', 'obsolete']}
    ))
    stability_violations: RuleConfig = field(default_factory=lambda: RuleConfig(
        error_on_violation=True
    ))
    circular_dependency_depth: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=3,  # Максимальный размер цикла
        error_on_violation=True
    ))
    component_complexity: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=50,
        error_on_violation=True
    ))

    # =========================
    # Security
    # =========================
    auth_boundary: RuleConfig = field(default_factory=lambda: RuleConfig(
        error_on_violation=True
    ))
    sensitive_data_flow: RuleConfig = field(default_factory=lambda: RuleConfig(
        error_on_violation=True
    ))
    input_validation_layer: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True
    ))

    # =========================
    # Testability
    # =========================
    mockability: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.3,  # 30% зависимостей на интерфейсы
        error_on_violation=False
    ))
    test_isolation: RuleConfig = field(default_factory=lambda: RuleConfig(
        error_on_violation=False
    ))
    test_coverage_structure: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.5,  # 50% покрытие
        error_on_violation=False
    ))

    # =========================
    # Observability
    # =========================
    logging_coverage: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.5,
        error_on_violation=False
    ))
    metrics_exposure: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True
    ))
    health_check_presence: RuleConfig = field(default_factory=lambda: RuleConfig(
        error_on_violation=False
    ))

    # =========================
    # API Quality
    # =========================
    api_consistency: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True
    ))

    # =========================
    # Topology & Algebraic Topology
    # =========================
    betti_numbers: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=10,  # Максимум β₁ (независимых циклов)
        error_on_violation=True
    ))
    euler_characteristic: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=-5,  # Минимальная характеристика Эйлера
        error_on_violation=False
    ))
    simplicial_complexity: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=4,  # Максимальная размерность симплексов
        error_on_violation=True
    ))
    topological_persistence: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.3,  # Максимальная топологическая хрупкость
        error_on_violation=False
    ))
    homological_density: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.5,  # Максимальная гомологическая плотность
        error_on_violation=False
    ))

    # =========================
    # Information Theory (Extended)
    # =========================
    mutual_information: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.5,  # Максимальная взаимная информация
        error_on_violation=False
    ))
    conditional_entropy: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=1.0,  # Минимальная условная энтропия
        error_on_violation=False
    ))
    channel_capacity: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,  # INFO метрика
        error_on_violation=False
    ))
    kolmogorov_complexity: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,  # INFO метрика
        error_on_violation=False
    ))
    cross_entropy: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=2.0,  # Максимальная кросс-энтропия
        error_on_violation=False
    ))

    # =========================
    # Markov Chains
    # =========================
    markov_stationary: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.2,  # Максимальная стационарная вероятность
        error_on_violation=False
    ))
    absorption_probability: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,  # INFO метрика
        error_on_violation=False
    ))
    mixing_time: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=10,  # Максимальное время смешивания
        error_on_violation=False
    ))

    # =========================
    # Advanced Linear Algebra
    # =========================
    matrix_rank: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,  # INFO метрика
        error_on_violation=False
    ))
    condition_number: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=100,  # Максимальное число обусловленности
        error_on_violation=False
    ))
    svd_analysis: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,  # INFO метрика
        error_on_violation=False
    ))
    spectral_gap: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.1,  # Минимальный спектральный зазор
        error_on_violation=False
    ))

    # =========================
    # Dynamical Systems
    # =========================
    lyapunov_stability: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,  # INFO метрика
        error_on_violation=False
    ))
    controllability: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.5,  # Минимальная управляемость
        error_on_violation=False
    ))
    graph_curvature: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,  # INFO метрика (Ollivier-Ricci)
        error_on_violation=False
    ))
    effective_resistance: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,  # INFO метрика
        error_on_violation=False
    ))

    # =========================
    # Advanced Graph Theory (Extended)
    # =========================
    treewidth: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=5,  # Максимальный treewidth
        error_on_violation=True
    ))
    chromatic_number: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=5,  # Максимальное хроматическое число
        error_on_violation=False
    ))
    dominating_set: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.3,  # Максимальное отношение доминирующего множества
        error_on_violation=False
    ))
    independence_number: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.2,  # Минимальное отношение независимости
        error_on_violation=False
    ))
    vertex_cover: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.7,  # Максимальное отношение вершинного покрытия
        error_on_violation=False
    ))
    graph_density_distribution: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.1,  # Максимальная дисперсия плотности
        error_on_violation=False
    ))
    graph_symmetry: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,  # INFO метрика
        error_on_violation=False
    ))

    # =========================
    # Advanced Topology (TDA)
    # =========================
    persistent_homology: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=5,  # Максимум короткоживущих компонент
        error_on_violation=False
    ))
    persistence_diagram: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.3,  # Порог шума
        error_on_violation=False
    ))
    bottleneck_stability: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.3,  # Максимальная нестабильность
        error_on_violation=False
    ))

    # Discrete Morse Theory
    morse_complexity: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.3,  # Максимальное отношение критических клеток
        error_on_violation=False
    ))
    gradient_flow: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,  # INFO метрика
        error_on_violation=False
    ))

    # Sheaf Theory
    sheaf_cohomology: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.5,  # Максимальное obstruction
        error_on_violation=False
    ))
    local_consistency: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.7,  # Минимальная консистентность
        error_on_violation=False
    ))

    # Discrete Curvature
    forman_curvature: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=-2.0,  # Минимальная средняя кривизна
        error_on_violation=False
    ))
    ricci_flow: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,  # INFO метрика
        error_on_violation=False
    ))

    # Homotopy Theory
    fundamental_group: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=10,  # Максимальный rank π₁
        error_on_violation=True
    ))
    covering_space: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,  # INFO метрика
        error_on_violation=False
    ))
    homotopy_equivalence: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,  # INFO метрика
        error_on_violation=False
    ))

    # Filtration & Hodge
    weight_filtration: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,  # INFO метрика
        error_on_violation=False
    ))
    hodge_decomposition: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,  # INFO метрика
        error_on_violation=False
    ))

    # Extended Simplicial Complexes
    clique_complex: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=4,  # Максимальная размерность
        error_on_violation=True
    ))
    nerve_complex: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,  # INFO метрика
        error_on_violation=False
    ))
    vietoris_rips: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,  # INFO метрика
        error_on_violation=False
    ))

    # Additional TDA
    persistence_landscape: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,  # INFO метрика
        error_on_violation=False
    ))
    cech_complex: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=3,  # Максимальная размерность
        error_on_violation=False
    ))
    alpha_complex: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,  # INFO метрика
        error_on_violation=False
    ))
    morse_smale: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,  # INFO метрика
        error_on_violation=False
    ))
    discrete_exterior_calculus: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,  # INFO метрика
        error_on_violation=False
    ))
    spectral_sequence: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,  # INFO метрика
        error_on_violation=False
    ))
    path_homology: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=5,  # Максимум параллельных путей
        error_on_violation=False
    ))
    wasserstein_stability: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.5,  # Максимальное расстояние Вассерштейна
        error_on_violation=False
    ))

    # =========================
    # Mathematical Analysis - Calculus on Graphs
    # =========================
    gradient_flow: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    heat_diffusion: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    laplacian_smoothness: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=1.0,  # Максимальная негладкость
        error_on_violation=False
    ))

    # =========================
    # Mathematical Analysis - Functional Analysis
    # =========================
    operator_norm: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    perturbation_sensitivity: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.5,  # Максимальная чувствительность
        error_on_violation=False
    ))
    sobolev_regularity: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))

    # =========================
    # Mathematical Analysis - Variational Methods
    # =========================
    dirichlet_energy: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=1.0,  # Максимальная энергия Дирихле
        error_on_violation=False
    ))
    geodesic_distance: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    optimal_placement: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))

    # =========================
    # Mathematical Analysis - Dynamical Systems
    # =========================
    lyapunov_stability: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    bifurcation_analysis: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    attractor_basin: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))

    # =========================
    # Mathematical Analysis - Harmonic Analysis
    # =========================
    graph_fourier: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    wavelet_decomposition: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    spectral_filtering: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))

    # =========================
    # Mathematical Analysis - Complex Analysis
    # =========================
    ihara_zeta: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    cycle_polynomial: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))

    # =========================
    # Integral Calculus - Path Integrals
    # =========================
    path_integral: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    circulation: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.1,  # Максимальная циркуляция
        error_on_violation=False
    ))

    # =========================
    # Integral Calculus - Potential Theory
    # =========================
    green_function: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    effective_resistance: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=2.0,  # Максимальное сопротивление
        error_on_violation=False
    ))
    node_capacity: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))

    # =========================
    # Integral Calculus - Heat Kernel
    # =========================
    heat_kernel_trace: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    heat_kernel_diagonal: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))

    # =========================
    # Integral Calculus - Stokes & Isoperimetric
    # =========================
    stokes_theorem: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    cheeger_constant: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.3,  # Минимальная константа Чигера
        error_on_violation=False
    ))

    # =========================
    # Integral Calculus - Hitting Times
    # =========================
    hitting_time: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    commute_time: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=50.0,  # Максимальное время коммутации
        error_on_violation=False
    ))

    # =========================
    # Integral Calculus - Transforms
    # =========================
    graph_laplace_transform: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    resolvent_analysis: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))

    # =========================
    # Set Theory - Relations
    # =========================
    relation_properties: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    equivalence_classes: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))

    # =========================
    # Set Theory - Partial Orders
    # =========================
    partial_order_analysis: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    chain_antichain: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))

    # =========================
    # Set Theory - Lattice Theory
    # =========================
    lattice_structure: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    join_meet_analysis: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))

    # =========================
    # Set Theory - Closures
    # =========================
    transitive_closure: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=3.0,  # Максимальный коэффициент замыкания
        error_on_violation=False
    ))
    closure_operator: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))

    # =========================
    # Set Theory - Fixed Points & Galois
    # =========================
    fixed_point_analysis: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    galois_connection: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))

    # =========================
    # Set Theory - Power Sets & Partitions
    # =========================
    power_set_complexity: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    partition_refinement: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))

    # =========================
    # Set Theory - Boolean & Filters
    # =========================
    boolean_algebra: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    filter_ideal: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))

    # =========================
    # Category Theory
    # =========================
    morphism_composition: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    initial_terminal: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    products_coproducts: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    pullback_pushout: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    functor_structure: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    natural_transformation: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    monad_structure: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    commutative_diagrams: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.8,  # Минимальная коммутативность
        error_on_violation=False
    ))
    adjunction: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    yoneda_embedding: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))

    # =========================
    # Game Theory
    # =========================
    shapley_value: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    nash_equilibrium: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    cooperative_games: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    evolutionary_stability: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    mechanism_design: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    voting_power: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))

    # =========================
    # Combinatorics
    # =========================
    generating_functions: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    inclusion_exclusion: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    stirling_numbers: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    polya_enumeration: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    ramsey_analysis: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=5,  # Максимум Ramsey-клик
        error_on_violation=False
    ))
    extremal_bounds: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    mobius_function: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))

    # =========================
    # Optimization Theory
    # =========================
    max_flow_min_cut: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=3,  # Максимум критических узких мест
        error_on_violation=False
    ))
    resource_allocation: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=5.0,  # Максимальный дисбаланс нагрузки
        error_on_violation=False
    ))
    convex_structure: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.7,  # Минимальная выпуклость
        error_on_violation=False
    ))
    submodular_functions: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.8,  # Минимальная субмодулярность
        error_on_violation=False
    ))
    facility_location: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=3.0,  # Максимальное среднее расстояние
        error_on_violation=False
    ))
    optimal_matching: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.8,  # Минимальный коэффициент паросочетания
        error_on_violation=False
    ))

    # =========================
    # Automata Theory
    # =========================
    state_machine_properties: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    regular_language_properties: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=2,  # Максимум проблем регулярности
        error_on_violation=False
    ))
    pushdown_patterns: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=10,  # Максимальная глубина вызовов
        error_on_violation=False
    ))
    computability_bounds: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    bisimulation: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.5,  # Минимальная редукция бисимуляции
        error_on_violation=False
    ))

    # =========================
    # Number Theory
    # =========================
    prime_factorization: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.3,  # Минимальная доля "простых" компонентов
        error_on_violation=False
    ))
    modular_arithmetic: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=10,  # Максимальная длина цикла
        error_on_violation=False
    ))
    chinese_remainder: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.5,  # Минимальная доля взаимно простых
        error_on_violation=False
    ))
    divisibility_lattice: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.3,  # Минимальная доля делимости
        error_on_violation=False
    ))
    diophantine_constraints: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.8,  # Минимальное выполнение ограничений
        error_on_violation=False
    ))

    # =========================
    # Probability Theory
    # =========================
    random_walk: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=100,  # Максимальное время смешивания
        error_on_violation=False
    ))
    markov_properties: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.5,  # Минимальная доля рекуррентных состояний
        error_on_violation=False
    ))
    entropy_measures: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.3,  # Минимальное отношение энтропии
        error_on_violation=False
    ))
    concentration_bounds: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.1,  # Максимальная доля выбросов
        error_on_violation=False
    ))
    bayesian_dependencies: RuleConfig = field(default_factory=lambda: RuleConfig(
        enabled=True,
        error_on_violation=False  # INFO
    ))
    stochastic_processes: RuleConfig = field(default_factory=lambda: RuleConfig(
        threshold=0.8,  # Минимальная доля стабильных узлов
        error_on_violation=False
    ))


def get_all_rule_names() -> List[str]:
    """Возвращает список всех имён правил из Config класса"""
    config = Config()
    rule_names = []
    for attr_name in dir(config):
        if not attr_name.startswith('_'):
            attr = getattr(config, attr_name)
            if isinstance(attr, RuleConfig):
                rule_names.append(attr_name)
    return rule_names


def _rule_config_to_dict(rule_config: RuleConfig) -> Dict[str, Any]:
    """Конвертирует RuleConfig в словарь для YAML"""
    result: Dict[str, Any] = {
        'enabled': rule_config.enabled,
        'error_on_violation': rule_config.error_on_violation,
        'exclude': rule_config.exclude if rule_config.exclude else [],
    }
    if rule_config.threshold is not None:
        result['threshold'] = rule_config.threshold
    if rule_config.params:
        result['params'] = rule_config.params
    return result


def ensure_config_complete(config_path: str) -> List[str]:
    """
    Проверяет конфигурационный файл и добавляет недостающие правила.

    Args:
        config_path: Путь к файлу конфигурации

    Returns:
        Список добавленных правил
    """
    # Получаем все имена правил
    all_rules = get_all_rule_names()
    default_config = Config()

    # Загружаем текущий конфиг
    if not os.path.exists(config_path):
        return []

    try:
        with open(config_path, 'r', encoding='utf-8') as f:
            data = yaml.safe_load(f) or {}
    except Exception as e:
        print(f"Warning: не удалось загрузить конфиг {config_path}: {e}")
        return []

    # Проверяем наличие секции rules
    if 'rules' not in data:
        data['rules'] = {}

    existing_rules = set(data['rules'].keys())
    missing_rules = [r for r in all_rules if r not in existing_rules]

    if not missing_rules:
        return []

    # Добавляем недостающие правила с дефолтными значениями
    for rule_name in missing_rules:
        rule_config = getattr(default_config, rule_name)
        data['rules'][rule_name] = _rule_config_to_dict(rule_config)

    # Сохраняем обновлённый конфиг
    try:
        with open(config_path, 'w', encoding='utf-8') as f:
            yaml.dump(data, f, allow_unicode=True, default_flow_style=False, sort_keys=False)
    except Exception as e:
        print(f"Warning: не удалось сохранить конфиг {config_path}: {e}")
        return []

    return missing_rules


def load_config(config_path: Optional[str] = None) -> Config:
    """
    Загружает конфигурацию из файла.

    Args:
        config_path: Путь к файлу конфигурации.
                    Если не указан, ищет .archlint.yaml в текущей директории.

    Returns:
        Config объект с настройками
    """
    config = Config()

    # Определяем путь к конфигу
    if config_path is None:
        config_path = '.archlint.yaml'

    # Если конфига нет - возвращаем дефолтный
    if not os.path.exists(config_path):
        return config

    # Проверяем и дополняем конфиг недостающими правилами
    added_rules = ensure_config_complete(config_path)
    if added_rules:
        print(f"Добавлены недостающие правила в {config_path}: {', '.join(added_rules)}")

    # Загружаем YAML
    try:
        with open(config_path, 'r', encoding='utf-8') as f:
            data = yaml.safe_load(f) or {}
    except Exception as e:
        print(f"Warning: не удалось загрузить конфиг {config_path}: {e}")
        return config

    # Парсим правила
    rules = data.get('rules', {})

    for rule_name, rule_data in rules.items():
        if hasattr(config, rule_name):
            rule_config = getattr(config, rule_name)
            _update_rule_config(rule_config, rule_data)

    return config


def _update_rule_config(rule_config: RuleConfig, data: Dict[str, Any]) -> None:
    """Обновляет конфигурацию правила из словаря"""
    if data is None:
        return

    if 'enabled' in data:
        rule_config.enabled = bool(data['enabled'])

    if 'threshold' in data:
        rule_config.threshold = data['threshold']

    if 'error_on_violation' in data:
        rule_config.error_on_violation = bool(data['error_on_violation'])

    if 'exclude' in data:
        rule_config.exclude = list(data['exclude']) if data['exclude'] else []

    # Обновляем params
    if 'params' in data and isinstance(data['params'], dict):
        rule_config.params.update(data['params'])

    # Также поддерживаем прямые параметры (без вложенности в params)
    direct_params = {k: v for k, v in data.items()
                     if k not in ('enabled', 'threshold', 'error_on_violation', 'exclude', 'params')}
    if direct_params:
        rule_config.params.update(direct_params)


def is_excluded(node: str, exclude_patterns: List[str]) -> bool:
    """
    Проверяет, попадает ли узел под исключения.

    Args:
        node: Имя узла
        exclude_patterns: Список паттернов для исключения

    Returns:
        True если узел исключён
    """
    if not exclude_patterns:
        return False

    node_lower = node.lower()
    for pattern in exclude_patterns:
        pattern_lower = pattern.lower()
        # Поддержка wildcard
        if pattern_lower.endswith('*'):
            if node_lower.startswith(pattern_lower[:-1]):
                return True
        elif pattern_lower.startswith('*'):
            if node_lower.endswith(pattern_lower[1:]):
                return True
        elif '*' in pattern_lower:
            # Поддержка паттерна типа "test/*"
            prefix, suffix = pattern_lower.split('*', 1)
            if node_lower.startswith(prefix) and node_lower.endswith(suffix):
                return True
        else:
            # Точное совпадение или содержание
            if pattern_lower in node_lower:
                return True

    return False


def generate_example_config() -> str:
    """Генерирует пример конфигурационного файла"""
    return '''# Конфигурация archlint валидатора
# Файл: .archlint.yaml

rules:
  # =========================
  # Core Checks
  # =========================

  dag_check:
    enabled: true
    error_on_violation: true  # Циклы - всегда критическая ошибка
    exclude: []

  max_fan_out:
    enabled: true
    threshold: 5              # Максимум исходящих зависимостей
    error_on_violation: true
    exclude:
      - "cmd/*"               # Исключить main пакеты
      - "test/*"              # Исключить тесты

  modularity:
    enabled: true
    threshold: 0.3            # Минимальная модульность
    error_on_violation: true
    exclude: []

  # =========================
  # Centrality Metrics
  # =========================

  betweenness_centrality:
    enabled: true
    threshold: 0.3            # Порог для bottleneck
    error_on_violation: true
    exclude: []

  pagerank:
    enabled: true
    threshold: 0.1
    error_on_violation: false # INFO статус
    exclude: []

  # =========================
  # Coupling Metrics
  # =========================

  coupling:
    enabled: true
    error_on_violation: true
    exclude: []
    params:
      ca_threshold: 10        # Макс входящих зависимостей
      ce_threshold: 10        # Макс исходящих зависимостей

  instability:
    enabled: true
    error_on_violation: true
    exclude:
      - "cmd/*"               # Entry points всегда нестабильны

  # =========================
  # Structural Checks
  # =========================

  orphan_nodes:
    enabled: true
    error_on_violation: true
    exclude:
      - "test/*"

  strongly_connected_components:
    enabled: true
    error_on_violation: true
    exclude: []
    params:
      max_size: 1             # Макс размер SCC

  graph_depth:
    enabled: true
    threshold: 10             # Макс глубина графа
    error_on_violation: true
    exclude: []

  hub_nodes:
    enabled: true
    threshold: 10             # Порог для God Object
    error_on_violation: true
    exclude:
      - "pkg/*"               # Utility пакеты могут быть hub-ами

  # =========================
  # Architectural Rules
  # =========================

  layer_violations:
    enabled: true
    error_on_violation: true
    exclude: []
    params:
      layers:
        cmd: 0
        api: 1
        handler: 1
        controller: 1
        service: 2
        usecase: 2
        internal: 2
        domain: 3
        entity: 3
        model: 3
        repository: 4
        storage: 4
        infrastructure: 5
        pkg: 6

  forbidden_dependencies:
    enabled: true
    error_on_violation: true
    exclude: []
    params:
      rules:
        - from: handler
          to: repository
        - from: controller
          to: repository
        - from: api
          to: storage
        - from: model
          to: service
        - from: entity
          to: repository

  component_distance:
    enabled: true
    threshold: 5              # Макс расстояние между компонентами
    error_on_violation: true
    exclude: []

  # =========================
  # Design Quality
  # =========================

  abstractness:
    enabled: true
    error_on_violation: true
    exclude: []
    params:
      min_threshold: 0.1      # Минимум абстрактности
      max_threshold: 0.8      # Максимум абстрактности
      patterns:               # Паттерны абстрактных типов
        - interface
        - abstract
        - base
        - contract
        - port

  distance_from_main_sequence:
    enabled: true
    threshold: 0.5            # Макс distance от main sequence
    error_on_violation: true
    exclude: []

  # =========================
  # Context Validations (Behavioral)
  # =========================

  context_coverage:
    enabled: true
    threshold: 0.8            # 80% критических компонентов должны быть в контекстах
    error_on_violation: true
    exclude: []
    params:
      top_n: 10               # Топ N компонентов по PageRank

  untested_components:
    enabled: true
    threshold: 0.5            # Максимум 50% непокрытых компонентов
    error_on_violation: false # INFO статус
    exclude:
      - "test/*"
      - "pkg/*"

  ghost_components:
    enabled: true
    error_on_violation: true
    exclude: []

  single_point_of_failure:
    enabled: true
    error_on_violation: false # INFO статус
    exclude: []
    params:
      min_contexts: 3         # Минимум контекстов для анализа

  context_complexity:
    enabled: true
    threshold: 15             # Максимум компонентов в контексте
    error_on_violation: true
    exclude: []

  context_coupling:
    enabled: true
    threshold: 0.7            # Максимум 70% общих компонентов
    error_on_violation: false # INFO статус
    exclude: []

  layer_traversal:
    enabled: true
    error_on_violation: true
    exclude: []
    params:
      layers:
        cmd: 0
        api: 1
        handler: 1
        controller: 1
        service: 2
        usecase: 2
        domain: 3
        entity: 3
        model: 3
        repository: 4
        storage: 4
        infrastructure: 5
        pkg: 6

  context_depth:
    enabled: true
    threshold: 10             # Максимальная глубина контекста
    error_on_violation: true
    exclude: []

  # =========================
  # Advanced Graph Theory
  # =========================

  clustering_coefficient:
    enabled: true
    threshold: 0.1            # Минимальный средний коэффициент кластеризации
    error_on_violation: false
    exclude: []

  edge_density:
    enabled: true
    error_on_violation: true
    exclude: []
    params:
      min_threshold: 0.01     # Минимальная плотность
      max_threshold: 0.3      # Максимальная плотность (монолитность)

  articulation_points:
    enabled: true
    error_on_violation: false # INFO - критические узлы
    exclude: []

  bridge_edges:
    enabled: true
    error_on_violation: false # INFO - критические рёбра
    exclude: []

  graph_diameter:
    enabled: true
    threshold: 10             # Максимальный диаметр графа
    error_on_violation: true
    exclude: []

  avg_path_length:
    enabled: true
    threshold: 5.0            # Максимальная средняя длина пути
    error_on_violation: false
    exclude: []

  closeness_centrality:
    enabled: true
    threshold: 0.5            # Порог для центральных узлов
    error_on_violation: false # INFO
    exclude: []

  eigenvector_centrality:
    enabled: true
    threshold: 0.3            # Порог для влиятельных узлов
    error_on_violation: false # INFO
    exclude: []

  k_core_decomposition:
    enabled: true
    threshold: 5              # Максимальное k-ядро
    error_on_violation: false # INFO
    exclude: []

  graph_cliques:
    enabled: true
    threshold: 4              # Максимальный размер клики
    error_on_violation: true
    exclude: []

  # =========================
  # Statistics Validations
  # =========================

  degree_distribution:
    enabled: true
    error_on_violation: false # INFO - аналитическая метрика
    exclude: []

  dependency_entropy:
    enabled: true
    threshold: 2.0            # Минимальная энтропия
    error_on_violation: false
    exclude: []

  gini_coefficient:
    enabled: true
    threshold: 0.6            # Максимальный коэффициент Джини
    error_on_violation: true  # God Objects
    exclude: []

  zscore_outliers:
    enabled: true
    threshold: 3.0            # Z-порог для выбросов
    error_on_violation: true
    exclude: []

  # =========================
  # Spectral Analysis
  # =========================

  algebraic_connectivity:
    enabled: true
    threshold: 0.1            # Минимальная алгебраическая связность
    error_on_violation: false
    exclude: []

  spectral_radius:
    enabled: true
    error_on_violation: false # INFO - аналитическая метрика
    exclude: []

  # =========================
  # Software Architecture Metrics
  # =========================

  cohesion_lcom4:
    enabled: true
    threshold: 1              # Максимум 1 компонент связности в типе
    error_on_violation: true
    exclude: []

  interface_segregation:
    enabled: true
    threshold: 5              # Максимум методов на интерфейс (ISP)
    error_on_violation: true
    exclude: []
'''
