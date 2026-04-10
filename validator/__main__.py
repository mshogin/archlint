#!/usr/bin/env python3
"""
archlint validator CLI

Usage:
    python -m validator validate <architecture.yaml> [options]
    python -m validator validate <architecture.yaml> -c <contexts.yaml> [options]

Options:
    -c, --contexts    Contexts file for behavior validation
    -g, --group       Validator group: core, solid, patterns, architecture, quality, advanced, research
    -f, --format      Output format: yaml, json (default: yaml)
    -o, --output      Output file (default: stdout)
    --structure-only  Run only structure validators
    --behavior-only   Run only behavior validators
"""

import argparse
import json
import sys
from pathlib import Path
from typing import Dict, List, Any, Optional

import yaml

from validator.graph_loader import GraphLoader
from validator.context_loader import load_contexts
from validator.config import load_config


def load_graph(filename: str):
    """Load graph from YAML file"""
    loader = GraphLoader()
    return loader.load_yaml(filename)


def get_structure_validators(group: Optional[str] = None) -> List:
    """Get structure validators by group"""
    validators = []

    if group is None or group == 'core':
        from validator.structure.core import CORE_VALIDATORS
        validators.extend(CORE_VALIDATORS)

    if group is None or group == 'solid':
        from validator.structure.solid import SOLID_VALIDATORS
        validators.extend(SOLID_VALIDATORS)

    if group is None or group == 'patterns':
        from validator.structure.patterns import PATTERN_VALIDATORS
        validators.extend(PATTERN_VALIDATORS)

    if group is None or group == 'architecture':
        from validator.structure.architecture import ARCHITECTURE_VALIDATORS
        validators.extend(ARCHITECTURE_VALIDATORS)

    if group is None or group == 'quality':
        from validator.structure.quality import QUALITY_VALIDATORS
        validators.extend(QUALITY_VALIDATORS)

    # Advanced and research are opt-in only (or included when group=None for full run)
    if group is None or group == 'advanced':
        from validator.structure.advanced import (
            validate_betweenness_centrality,
            validate_pagerank,
            validate_modularity,
            validate_abstractness,
            validate_distance_from_main_sequence,
            validate_clustering_coefficient,
            validate_edge_density,
            validate_articulation_points,
            validate_bridge_edges,
            validate_graph_diameter,
            validate_avg_path_length,
            validate_closeness_centrality,
            validate_eigenvector_centrality,
            validate_k_core_decomposition,
            validate_graph_cliques,
            validate_degree_distribution,
            validate_dependency_entropy,
            validate_gini_coefficient,
            validate_zscore_outliers,
            validate_algebraic_connectivity,
            validate_spectral_radius,
            validate_cohesion_lcom4,
            validate_change_propagation,
            validate_blast_radius,
            validate_hotspot_detection,
            validate_deprecated_usage,
            validate_stability_violations,
            validate_circular_dependency_depth,
            validate_component_complexity,
        )
        validators.extend([
            validate_betweenness_centrality,
            validate_pagerank,
            validate_modularity,
            validate_abstractness,
            validate_distance_from_main_sequence,
            validate_clustering_coefficient,
            validate_edge_density,
            validate_articulation_points,
            validate_bridge_edges,
            validate_graph_diameter,
            validate_avg_path_length,
            validate_closeness_centrality,
            validate_eigenvector_centrality,
            validate_k_core_decomposition,
            validate_graph_cliques,
            validate_degree_distribution,
            validate_dependency_entropy,
            validate_gini_coefficient,
            validate_zscore_outliers,
            validate_algebraic_connectivity,
            validate_spectral_radius,
            validate_cohesion_lcom4,
            validate_change_propagation,
            validate_blast_radius,
            validate_hotspot_detection,
            validate_deprecated_usage,
            validate_stability_violations,
            validate_circular_dependency_depth,
            validate_component_complexity,
        ])

    if group is None or group == 'research':
        try:
            import inspect
            from validator.structure.research import (
                topology_metrics, information_theory_metrics, linear_algebra_metrics,
                advanced_graph_metrics, advanced_topology_metrics,
                category_theory_metrics, game_theory_metrics, combinatorics_metrics,
                optimization_metrics, automata_theory_metrics, number_theory_metrics,
                probability_metrics, mathematical_analysis_metrics,
                integral_calculus_metrics, set_theory_metrics,
                hott_metrics,
            )
            research_modules = [
                topology_metrics, information_theory_metrics, linear_algebra_metrics,
                advanced_graph_metrics, advanced_topology_metrics,
                category_theory_metrics, game_theory_metrics, combinatorics_metrics,
                optimization_metrics, automata_theory_metrics, number_theory_metrics,
                probability_metrics, mathematical_analysis_metrics,
                integral_calculus_metrics, set_theory_metrics,
                hott_metrics,
            ]
            seen_ids = set(id(v) for v in validators)
            seen_names = set(v.__name__ for v in validators)
            for mod in research_modules:
                for name, func in inspect.getmembers(mod, inspect.isfunction):
                    if name.startswith('validate_') and id(func) not in seen_ids and name not in seen_names:
                        validators.append(func)
                        seen_ids.add(id(func))
                        seen_names.add(name)
        except ImportError as e:
            import sys
            print(f"Warning: research validators not available: {e}", file=sys.stderr)

    return validators


def get_behavior_validators(group: Optional[str] = None) -> List:
    """Get behavior validators by group"""
    validators = []

    if group is None or group == 'core':
        from validator.behavior.core import CORE_BEHAVIOR_VALIDATORS
        validators.extend(CORE_BEHAVIOR_VALIDATORS)

    if group == 'advanced':
        from validator.behavior.advanced import ADVANCED_BEHAVIOR_VALIDATORS
        validators.extend(ADVANCED_BEHAVIOR_VALIDATORS)

    return validators


def run_validation(
    arch_file: str,
    contexts_file: Optional[str] = None,
    group: Optional[str] = None,
    structure_only: bool = False,
    behavior_only: bool = False,
    config_file: Optional[str] = None,
) -> Dict[str, Any]:
    """Run validation and return results"""

    # Load graph
    graph = load_graph(arch_file)

    # Load contexts if provided
    contexts = {}
    if contexts_file:
        contexts = load_contexts(contexts_file)

    # Load config: use explicit path if given, otherwise try .archlint.yaml in cwd.
    config = load_config(config_file if config_file else None)

    results = {
        'status': 'PASSED',
        'summary': {
            'total_checks': 0,
            'passed': 0,
            'failed': 0,
            'warnings': 0,
            'info': 0,
            'skipped': 0,
            'errors': 0,
        },
        'graph': {
            'nodes': len(graph.nodes()),
            'edges': len(graph.edges()),
        },
        'checks': [],
    }

    # Run structure validators
    if not behavior_only:
        validators = get_structure_validators(group)
        for validator in validators:
            try:
                # Extract per-rule config from the Config object.
                # Validator names are "validate_<rule_name>" and Config has matching attributes.
                rule_config = None
                if config is not None:
                    func_name = validator.__name__
                    rule_name = func_name.removeprefix('validate_')
                    rule_config = getattr(config, rule_name, None)
                result = validator(graph, config=rule_config)
                results['checks'].append(result)
                _update_summary(results, result)
            except Exception as e:
                results['checks'].append({
                    'name': validator.__name__,
                    'status': 'ERROR',
                    'error': str(e),
                })
                results['summary']['errors'] += 1

    # Run behavior validators if contexts provided
    if contexts and not structure_only:
        validators = get_behavior_validators(group)
        for validator in validators:
            try:
                result = validator(graph, contexts, config=config)
                results['checks'].append(result)
                _update_summary(results, result)
            except Exception as e:
                results['checks'].append({
                    'name': validator.__name__,
                    'status': 'ERROR',
                    'error': str(e),
                })
                results['summary']['errors'] += 1

    results['summary']['total_checks'] = len(results['checks'])

    # Determine overall status
    if results['summary']['failed'] > 0:
        results['status'] = 'FAILED'
    elif results['summary']['errors'] > 0:
        results['status'] = 'ERROR'
    elif results['summary']['warnings'] > 0:
        results['status'] = 'WARNING'

    return results


def _update_summary(results: Dict, check_result: Dict) -> None:
    """Update summary counters based on check result"""
    status = check_result.get('status', 'UNKNOWN')

    if status == 'PASSED':
        results['summary']['passed'] += 1
    elif status == 'FAILED':
        results['summary']['failed'] += 1
    elif status == 'WARNING':
        results['summary']['warnings'] += 1
    elif status == 'INFO':
        results['summary']['info'] += 1
    elif status == 'SKIP':
        results['summary']['skipped'] += 1
    elif status == 'ERROR':
        results['summary']['errors'] += 1


def main():
    parser = argparse.ArgumentParser(
        description='archlint validator - Architecture validation engine',
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog=__doc__,
    )

    subparsers = parser.add_subparsers(dest='command', help='Commands')

    # validate command
    validate_parser = subparsers.add_parser('validate', help='Validate architecture')
    validate_parser.add_argument('architecture', help='Architecture YAML file')
    validate_parser.add_argument('-c', '--contexts', help='Contexts YAML file')
    validate_parser.add_argument('-g', '--group',
                                 choices=['core', 'solid', 'patterns', 'architecture',
                                         'quality', 'advanced', 'research'],
                                 help='Validator group')
    validate_parser.add_argument('-f', '--format', choices=['yaml', 'json'],
                                 default='yaml', help='Output format')
    validate_parser.add_argument('-o', '--output', help='Output file')
    validate_parser.add_argument('--config', help='Config file')
    validate_parser.add_argument('--structure-only', action='store_true',
                                 help='Run only structure validators')
    validate_parser.add_argument('--behavior-only', action='store_true',
                                 help='Run only behavior validators')

    # list command
    list_parser = subparsers.add_parser('list', help='List available validators')
    list_parser.add_argument('-g', '--group', help='Filter by group')

    args = parser.parse_args()

    if args.command == 'validate':
        results = run_validation(
            arch_file=args.architecture,
            contexts_file=args.contexts,
            group=args.group,
            structure_only=args.structure_only,
            behavior_only=args.behavior_only,
            config_file=args.config,
        )

        # Convert numpy types to native Python for serialization
        def convert_numpy(obj):
            try:
                import numpy as np
                if isinstance(obj, dict):
                    return {k: convert_numpy(v) for k, v in obj.items()}
                elif isinstance(obj, list):
                    return [convert_numpy(i) for i in obj]
                elif isinstance(obj, (np.integer,)):
                    return int(obj)
                elif isinstance(obj, (np.floating,)):
                    return float(obj)
                elif isinstance(obj, np.ndarray):
                    return obj.tolist()
                elif isinstance(obj, (np.bool_,)):
                    return bool(obj)
            except ImportError:
                pass
            return obj

        results = convert_numpy(results)

        if args.format == 'json':
            output = json.dumps(results, indent=2, ensure_ascii=False, default=str)
        else:
            output = yaml.dump(results, allow_unicode=True, default_flow_style=False)

        if args.output:
            Path(args.output).write_text(output)
            print(f"Results saved to {args.output}")
        else:
            print(output)

        # Exit code based on status
        if results['status'] == 'FAILED':
            sys.exit(1)
        elif results['status'] == 'ERROR':
            sys.exit(2)

    elif args.command == 'list':
        print("Structure validators:")
        print("  core: dag_check, max_fan_out, coupling, instability, layers, forbidden_deps, hub_nodes, orphan_nodes, scc, graph_depth")
        print("  solid: SRP, OCP, LSP, DIP, ISP")
        print("  patterns: god_class, shotgun_surgery, divergent_change, lazy_class, middle_man, speculative_generality, data_clumps, feature_envy")
        print("  architecture: domain_isolation, ports_adapters, use_case_purity, dto_boundaries, inward_deps, bounded_context, service_autonomy")
        print("  quality: auth_boundary, sensitive_data_flow, input_validation, mockability, test_isolation, logging, metrics, health_check, api_consistency")
        print("  advanced: centrality, pagerank, modularity, clustering, change_propagation, blast_radius")
        print("  research: topology, information_theory, linear_algebra, category_theory, game_theory, combinatorics, ...")
        print()
        print("Behavior validators:")
        print("  core: context_coverage, untested_components, ghost_components, context_complexity")
        print("  advanced: single_point_of_failure, context_coupling, layer_traversal, context_depth")

    else:
        parser.print_help()


if __name__ == '__main__':
    main()
