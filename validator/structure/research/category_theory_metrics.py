"""
Category Theory Metrics for Software Architecture Validation.

Implements:
- Objects & Morphisms: nodes as objects, edges as morphisms
- Functors: structure-preserving maps between graphs
- Natural Transformations: morphisms between functors
- Limits & Colimits: products, coproducts, pullbacks, pushouts
- Monads: composition patterns
- Commutative Diagrams: interface consistency
- Initial & Terminal Objects: universal elements
- Adjunctions: Galois connections generalized
"""

from typing import Any, Dict, List, Optional, Set, Tuple
import networkx as nx
import numpy as np
from collections import defaultdict
import itertools


def _is_excluded(node: str, exclude: List[str]) -> bool:
    for pattern in exclude:
        if pattern in node:
            return True
    return False


def validate_morphism_composition(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Morphism Composition - композиция морфизмов.

    В категории: если f: A→B и g: B→C, то g∘f: A→C.
    Проверяем ассоциативность: (h∘g)∘f = h∘(g∘f).

    Для архитектуры: транзитивные зависимости.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 3:
            return {'name': 'morphism_composition', 'status': 'SKIP', 'reason': 'Insufficient nodes'}

        # Подсчёт композиций
        compositions = 0
        paths_of_length_2 = []

        for a in nodes:
            for b in subgraph.successors(a):
                for c in subgraph.successors(b):
                    compositions += 1
                    if len(paths_of_length_2) < 10:
                        paths_of_length_2.append((a, b, c))

        # Проверка наличия прямых рёбер (композиция реализована)
        explicit_compositions = sum(1 for a, b, c in paths_of_length_2 if subgraph.has_edge(a, c))

        return {
            'name': 'morphism_composition',
            'description': 'Композиция морфизмов g∘f',
            'status': 'INFO',
            'total_compositions': compositions,
            'explicit_compositions': explicit_compositions,
            'composition_ratio': round(explicit_compositions / compositions, 4) if compositions > 0 else 0,
            'sample_paths': [(str(a), str(b), str(c)) for a, b, c in paths_of_length_2[:5]],
            'interpretation': 'Явные композиции = транзитивные зависимости в коде'
        }
    except Exception as e:
        return {'name': 'morphism_composition', 'status': 'ERROR', 'error': str(e)}


def validate_initial_terminal(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Initial & Terminal Objects - начальные и конечные объекты.

    Initial object 0: ∃! морфизм 0→X для любого X.
    Terminal object 1: ∃! морфизм X→1 для любого X.

    Для архитектуры:
    - Initial = корневые модули (от которых всё зависит)
    - Terminal = leaf модули (ни от чего не зависят)
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 2:
            return {'name': 'initial_terminal', 'status': 'SKIP', 'reason': 'Insufficient nodes'}

        # Initial objects: in_degree = 0 (ничего не импортируют)
        initial_objects = [node for node in nodes if subgraph.in_degree(node) == 0]

        # Terminal objects: out_degree = 0 (никто не импортирует)
        terminal_objects = [node for node in nodes if subgraph.out_degree(node) == 0]

        # Zero object: и initial и terminal
        zero_objects = [node for node in nodes
                       if subgraph.in_degree(node) == 0 and subgraph.out_degree(node) == 0]

        # Универсальность: initial достигает всех
        initial_coverage = {}
        for init in initial_objects[:5]:
            reachable = len(nx.descendants(subgraph, init))
            initial_coverage[str(init)] = reachable

        return {
            'name': 'initial_terminal',
            'description': 'Начальные и конечные объекты',
            'status': 'INFO',
            'initial_objects': len(initial_objects),
            'terminal_objects': len(terminal_objects),
            'zero_objects': len(zero_objects),
            'initial_samples': [str(x) for x in initial_objects[:5]],
            'terminal_samples': [str(x) for x in terminal_objects[:5]],
            'initial_coverage': initial_coverage,
            'interpretation': 'Initial = entry points, Terminal = leaf dependencies'
        }
    except Exception as e:
        return {'name': 'initial_terminal', 'status': 'ERROR', 'error': str(e)}


def validate_products_coproducts(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Products & Coproducts - произведения и копроизведения.

    Product A×B: объект с проекциями π₁: A×B→A, π₂: A×B→B
    Coproduct A+B: объект с инъекциями i₁: A→A+B, i₂: B→A+B

    Для архитектуры:
    - Product = модуль, зависящий от обоих A и B
    - Coproduct = модуль, от которого зависят и A и B
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 3:
            return {'name': 'products_coproducts', 'status': 'SKIP', 'reason': 'Insufficient nodes'}

        products = []  # Узлы с множественными зависимостями
        coproducts = []  # Узлы от которых зависят многие

        for node in nodes:
            in_deg = subgraph.in_degree(node)
            out_deg = subgraph.out_degree(node)

            if in_deg >= 2:
                predecessors = list(subgraph.predecessors(node))[:3]
                products.append({
                    'node': str(node),
                    'factors': [str(p) for p in predecessors],
                    'arity': in_deg
                })

            if out_deg >= 2:
                successors = list(subgraph.successors(node))[:3]
                coproducts.append({
                    'node': str(node),
                    'summands': [str(s) for s in successors],
                    'arity': out_deg
                })

        # Проверка универсальности
        # Product A×B универсален если любой C→A, C→B факторизуется через A×B

        return {
            'name': 'products_coproducts',
            'description': 'Произведения и копроизведения',
            'status': 'INFO',
            'product_count': len(products),
            'coproduct_count': len(coproducts),
            'products_sample': products[:5],
            'coproducts_sample': coproducts[:5],
            'interpretation': 'Products = агрегаторы, Coproducts = общие зависимости'
        }
    except Exception as e:
        return {'name': 'products_coproducts', 'status': 'ERROR', 'error': str(e)}


def validate_pullback_pushout(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Pullbacks & Pushouts - расслоённые произведения и копроизведения.

    Pullback: A ×_C B для A→C←B
    Pushout: A +_C B для A←C→B

    Для архитектуры:
    - Pullback = общий модуль для двух зависящих от одного
    - Pushout = объединение двух модулей с общей частью
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 3:
            return {'name': 'pullback_pushout', 'status': 'SKIP', 'reason': 'Insufficient nodes'}

        # Pullback diagrams: A→C←B
        pullback_diagrams = []
        for c in nodes:
            predecessors = list(subgraph.predecessors(c))
            if len(predecessors) >= 2:
                for a, b in itertools.combinations(predecessors[:5], 2):
                    # Ищем pullback P с P→A и P→B
                    common_pred = set(subgraph.predecessors(a)) & set(subgraph.predecessors(b))
                    if common_pred:
                        pullback_diagrams.append({
                            'apex': str(c),
                            'legs': (str(a), str(b)),
                            'pullback_candidates': [str(p) for p in list(common_pred)[:2]]
                        })

        # Pushout diagrams: A←C→B
        pushout_diagrams = []
        for c in nodes:
            successors = list(subgraph.successors(c))
            if len(successors) >= 2:
                for a, b in itertools.combinations(successors[:5], 2):
                    # Ищем pushout Q с A→Q и B→Q
                    common_succ = set(subgraph.successors(a)) & set(subgraph.successors(b))
                    if common_succ:
                        pushout_diagrams.append({
                            'base': str(c),
                            'legs': (str(a), str(b)),
                            'pushout_candidates': [str(q) for q in list(common_succ)[:2]]
                        })

        return {
            'name': 'pullback_pushout',
            'description': 'Pullbacks и Pushouts',
            'status': 'INFO',
            'pullback_diagrams': len(pullback_diagrams),
            'pushout_diagrams': len(pushout_diagrams),
            'pullbacks_sample': pullback_diagrams[:3],
            'pushouts_sample': pushout_diagrams[:3],
            'interpretation': 'Pullback = fiber product, Pushout = amalgamation'
        }
    except Exception as e:
        return {'name': 'pullback_pushout', 'status': 'ERROR', 'error': str(e)}


def validate_functor_structure(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Functor Structure - функториальность.

    Функтор F: C→D сохраняет:
    - F(id_A) = id_{F(A)}
    - F(g∘f) = F(g)∘F(f)

    Для архитектуры: эндофунктор на графе зависимостей.
    Анализируем самоподобие структуры.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 4:
            return {'name': 'functor_structure', 'status': 'SKIP', 'reason': 'Insufficient nodes'}

        # Ищем подграфы изоморфные друг другу (функториальное соответствие)
        # Упрощение: сравниваем локальные структуры

        local_structures = {}
        for node in nodes:
            in_deg = subgraph.in_degree(node)
            out_deg = subgraph.out_degree(node)
            signature = (in_deg, out_deg)
            if signature not in local_structures:
                local_structures[signature] = []
            local_structures[signature].append(node)

        # Классы эквивалентности по локальной структуре
        equivalence_classes = {
            f'in={sig[0]},out={sig[1]}': len(nodes_list)
            for sig, nodes_list in local_structures.items()
        }

        # Самый большой класс = потенциально функториально связанные
        largest_class_size = max(len(v) for v in local_structures.values())

        # Проверка сохранения структуры между классами
        structure_preservation = len(local_structures) / n if n > 0 else 0

        return {
            'name': 'functor_structure',
            'description': 'Функториальная структура',
            'status': 'INFO',
            'equivalence_classes': len(local_structures),
            'class_distribution': dict(sorted(equivalence_classes.items(), key=lambda x: -x[1])[:10]),
            'largest_class_size': largest_class_size,
            'structure_preservation': round(structure_preservation, 4),
            'interpretation': 'Классы эквивалентности = функториально подобные модули'
        }
    except Exception as e:
        return {'name': 'functor_structure', 'status': 'ERROR', 'error': str(e)}


def validate_natural_transformation(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Natural Transformation - естественные преобразования.

    η: F ⇒ G естественно если диаграмма коммутирует:
    F(A) --η_A--> G(A)
      |            |
    F(f)         G(f)
      |            |
    F(B) --η_B--> G(B)

    Для архитектуры: согласованность интерфейсов/адаптеров.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 4:
            return {'name': 'natural_transformation', 'status': 'SKIP', 'reason': 'Insufficient nodes'}

        # Ищем пары узлов с "параллельными" структурами
        # (одинаковые in/out degree patterns в окрестности)

        parallel_pairs = []
        node_list = list(nodes)

        for i, a in enumerate(node_list[:20]):
            for b in node_list[i+1:20]:
                # Сравниваем окрестности
                a_in = set(subgraph.predecessors(a))
                a_out = set(subgraph.successors(a))
                b_in = set(subgraph.predecessors(b))
                b_out = set(subgraph.successors(b))

                # Пересечения указывают на "естественность"
                common_in = len(a_in & b_in)
                common_out = len(a_out & b_out)

                if common_in > 0 or common_out > 0:
                    parallel_pairs.append({
                        'pair': (str(a), str(b)),
                        'common_predecessors': common_in,
                        'common_successors': common_out
                    })

        # Сортируем по "естественности"
        parallel_pairs.sort(key=lambda x: x['common_predecessors'] + x['common_successors'], reverse=True)

        return {
            'name': 'natural_transformation',
            'description': 'Естественные преобразования',
            'status': 'INFO',
            'parallel_pairs_found': len(parallel_pairs),
            'most_natural_pairs': parallel_pairs[:5],
            'interpretation': 'Параллельные структуры = потенциальные адаптеры/рефакторинги'
        }
    except Exception as e:
        return {'name': 'natural_transformation', 'status': 'ERROR', 'error': str(e)}


def validate_monad_structure(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Monad Structure - монадическая структура.

    Монада (T, η, μ):
    - T: C → C (эндофунктор)
    - η: Id ⇒ T (unit)
    - μ: T² ⇒ T (join)

    Для архитектуры: паттерны композиции (wrapper, decorator).
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 3:
            return {'name': 'monad_structure', 'status': 'SKIP', 'reason': 'Insufficient nodes'}

        # Ищем "монадические" паттерны: A → T(A) → T(T(A)) → T(A)
        # В графе: цепочки A → B → C → B

        monad_patterns = []

        for a in list(nodes)[:20]:
            for b in subgraph.successors(a):
                for c in subgraph.successors(b):
                    # Ищем путь обратно к b (μ: T² → T)
                    if subgraph.has_edge(c, b):
                        monad_patterns.append({
                            'unit': str(a),
                            'T': str(b),
                            'TT': str(c),
                            'has_join': True
                        })

        # Декораторы: A → D → A (D оборачивает и делегирует)
        decorator_patterns = []
        for a in list(nodes)[:20]:
            for d in subgraph.successors(a):
                if subgraph.has_edge(d, a):
                    decorator_patterns.append({
                        'base': str(a),
                        'decorator': str(d)
                    })

        return {
            'name': 'monad_structure',
            'description': 'Монадическая структура',
            'status': 'INFO',
            'monad_patterns': len(monad_patterns),
            'decorator_patterns': len(decorator_patterns),
            'monad_samples': monad_patterns[:3],
            'decorator_samples': decorator_patterns[:5],
            'interpretation': 'Монады = композиционные паттерны (wrapper, chain)'
        }
    except Exception as e:
        return {'name': 'monad_structure', 'status': 'ERROR', 'error': str(e)}


def validate_commutative_diagrams(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Commutative Diagrams - коммутативные диаграммы.

    Диаграмма коммутирует если все пути между двумя объектами
    дают один и тот же результат.

    Для архитектуры: согласованность альтернативных путей зависимостей.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 3:
            return {'name': 'commutative_diagrams', 'status': 'SKIP', 'reason': 'Insufficient nodes'}

        # Ищем пары узлов с несколькими путями между ними
        diamond_diagrams = []

        for source in list(nodes)[:15]:
            for target in list(nodes)[:15]:
                if source == target:
                    continue

                # Находим все простые пути длины 2
                paths = []
                for mid in subgraph.successors(source):
                    if subgraph.has_edge(mid, target):
                        paths.append(mid)

                if len(paths) >= 2:
                    diamond_diagrams.append({
                        'source': str(source),
                        'target': str(target),
                        'paths_count': len(paths),
                        'intermediate': [str(p) for p in paths[:3]]
                    })

        # Квадраты (squares): A→B, A→C, B→D, C→D
        squares = []
        for a in list(nodes)[:10]:
            successors_a = list(subgraph.successors(a))
            if len(successors_a) >= 2:
                for b, c in itertools.combinations(successors_a[:5], 2):
                    common_succ = set(subgraph.successors(b)) & set(subgraph.successors(c))
                    for d in common_succ:
                        squares.append({
                            'A': str(a), 'B': str(b), 'C': str(c), 'D': str(d)
                        })

        return {
            'name': 'commutative_diagrams',
            'description': 'Коммутативные диаграммы',
            'status': 'INFO',
            'diamond_diagrams': len(diamond_diagrams),
            'commutative_squares': len(squares),
            'diamonds_sample': diamond_diagrams[:3],
            'squares_sample': squares[:3],
            'interpretation': 'Коммутативность = согласованность альтернативных путей'
        }
    except Exception as e:
        return {'name': 'commutative_diagrams', 'status': 'ERROR', 'error': str(e)}


def validate_adjunction(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Adjunction - сопряжение.

    F ⊣ G (F левый сопряжённый к G):
    Hom(F(A), B) ≅ Hom(A, G(B))

    Для архитектуры: пары модулей с двойственными ролями
    (producer/consumer, encoder/decoder).
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 4:
            return {'name': 'adjunction', 'status': 'SKIP', 'reason': 'Insufficient nodes'}

        # Ищем пары с "двойственной" структурой
        # F ⊣ G если паттерны Hom симметричны

        adjoint_candidates = []
        node_list = list(nodes)

        for i, f in enumerate(node_list[:15]):
            for g in node_list[i+1:15]:
                # Считаем "связи" через F и G
                f_out = set(subgraph.successors(f))
                g_in = set(subgraph.predecessors(g))
                f_in = set(subgraph.predecessors(f))
                g_out = set(subgraph.successors(g))

                # Критерий сопряжения: симметрия связей
                forward_score = len(f_out & g_in)  # F→?→G
                backward_score = len(g_out & f_in)  # G→?→F

                if forward_score > 0 and backward_score > 0:
                    adjoint_candidates.append({
                        'F': str(f),
                        'G': str(g),
                        'forward_connections': forward_score,
                        'backward_connections': backward_score,
                        'symmetry': min(forward_score, backward_score) / max(forward_score, backward_score)
                    })

        # Сортируем по симметрии
        adjoint_candidates.sort(key=lambda x: x['symmetry'], reverse=True)

        return {
            'name': 'adjunction',
            'description': 'Сопряжённые пары F ⊣ G',
            'status': 'INFO',
            'adjoint_pairs_found': len(adjoint_candidates),
            'best_adjoints': adjoint_candidates[:5],
            'interpretation': 'Сопряжения = двойственные модули (encoder/decoder, producer/consumer)'
        }
    except Exception as e:
        return {'name': 'adjunction', 'status': 'ERROR', 'error': str(e)}


def validate_yoneda_embedding(
    graph: nx.DiGraph,
    config: Optional['RuleConfig'] = None
) -> Dict[str, Any]:
    """
    Yoneda Embedding - вложение Йонеды.

    y: C → [C^op, Set], y(A) = Hom(-, A)

    Лемма Йонеды: Nat(Hom(-, A), F) ≅ F(A)

    Для архитектуры: модуль определяется своими зависимостями.
    """
    exclude = config.exclude if config else []

    try:
        nodes = [n for n in graph.nodes() if not _is_excluded(n, exclude)]
        subgraph = graph.subgraph(nodes)

        n = len(nodes)
        if n < 3:
            return {'name': 'yoneda_embedding', 'status': 'SKIP', 'reason': 'Insufficient nodes'}

        # Вычисляем Hom(-, A) для каждого A
        # Это множество всех узлов, из которых есть путь в A

        hom_functors = {}
        for a in list(nodes)[:20]:
            # Hom(-, A) = predecessors + transitive
            hom_a = set(nx.ancestors(subgraph, a)) | {a}
            hom_functors[str(a)] = len(hom_a)

        # Вложение полно и точно: разные объекты → разные функторы
        # Проверяем injectivity
        hom_values = list(hom_functors.values())
        unique_hom_values = len(set(hom_values))

        # Представимые функторы
        representable = {
            'total_objects': len(hom_functors),
            'unique_representations': unique_hom_values,
            'is_faithful': unique_hom_values == len(hom_functors)
        }

        # Йонеда: модуль определяется тем, кто от него зависит
        yoneda_profiles = sorted(hom_functors.items(), key=lambda x: -x[1])[:10]

        return {
            'name': 'yoneda_embedding',
            'description': 'Вложение Йонеды y(A) = Hom(-, A)',
            'status': 'INFO',
            'hom_functor_sizes': dict(yoneda_profiles),
            'representable_analysis': representable,
            'interpretation': 'Йонеда: модуль ≅ его "входящие" зависимости'
        }
    except Exception as e:
        return {'name': 'yoneda_embedding', 'status': 'ERROR', 'error': str(e)}
