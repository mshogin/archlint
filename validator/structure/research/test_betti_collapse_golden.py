"""Golden-тест схлопывания π₁→β₁ (DR-0003) + severity β₁ → INFO (DR-0005).

Пинит числовую эквивалентность rank(π₁) ≡ β₁ = E − V + C (то, что считал удалённый
validate_fundamental_group, теперь даёт validate_betti_numbers), и «ромб»-контрпример,
доказавший что β₁ НЕ про ориентированные циклы (структурный дескриптор).
"""
import networkx as nx
from validator.structure.research.topology_metrics import validate_betti_numbers


def beta1_formula(g: nx.DiGraph) -> int:
    """rank(π₁) = β₁ = E − V + C на неориентированной проекции (π₁ свободна для графа)."""
    u = g.to_undirected()
    return u.number_of_edges() - u.number_of_nodes() + nx.number_connected_components(u)


def test_betti_diamond_counterexample():
    # «Ромб» A->B, A->C, B->D, C->D: β₁ = E−V+C = 4−4+1 = 1, при 0 ориент. циклах.
    # Доказывает: β₁ — дескриптор формы, НЕ нарушение «нет циклов» (DR-0005).
    g = nx.DiGraph([("A", "B"), ("A", "C"), ("B", "D"), ("C", "D")])
    r = validate_betti_numbers(g)
    assert r["beta_1"] == 1
    assert r["beta_1"] == beta1_formula(g)        # схлопывание: β₁ == формула
    assert r["status"] == "INFO"                  # дескриптор, НЕ ERROR (DR-0005)
    assert len(list(nx.simple_cycles(g))) == 0    # 0 ориентированных циклов, а β₁=1


def test_betti_tree_is_zero():
    tree = nx.DiGraph([("r", "a"), ("r", "b"), ("a", "c")])
    r = validate_betti_numbers(tree)
    assert r["beta_1"] == 0 == beta1_formula(tree)
    assert r["status"] == "INFO"


def test_betti_equals_formula_on_cyclic():
    # Три независимых 2-цикла: β₁ должен равняться формуле E−V+C.
    g = nx.DiGraph([("1", "2"), ("2", "1"), ("3", "4"), ("4", "3"), ("5", "6"), ("6", "5")])
    r = validate_betti_numbers(g)
    assert r["beta_1"] == beta1_formula(g)
    assert r["status"] == "INFO"                  # даже при высоком β₁ — INFO, не гейт


def test_fundamental_group_removed():
    # DR-0003: validate_fundamental_group УДАЛЁН (тождественный алиас β₁).
    import validator.structure.research.advanced_topology_metrics as atm
    assert not hasattr(atm, "validate_fundamental_group")
