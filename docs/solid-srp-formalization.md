# SRP as Graph Invariant: Formalizing Single Responsibility Principle

**Author:** Mikhail Shogin
**Status:** Research draft
**Target:** Academic article for Russian IT architecture community
**Date:** 2026-04-16

---

## 0. Abstract

We formalize the Single Responsibility Principle (SRP) as a graph-theoretic invariant computable without human code review. The formalization treats methods of a type as nodes in a compound interaction graph enriched with the type's fields and the external targets each method reaches. The number of responsibilities of a type is defined as the number of connected components under a specific equivalence relation over method reach-profiles. We cross-validate this definition through five independent mathematical theories (set theory, graph theory / community detection, information theory, spectral analysis, algebraic topology) and show empirically that converging verdicts across theories provide a robust signal that survives expert scrutiny.

---

## 1. Motivation and problem statement

### 1.1 The subjective nature of classical SRP

Robert Martin's formulation of SRP — *"a class should have one reason to change"* — is intuitive but operationally useless. "Reason to change" is a counterfactual over future history; it cannot be measured from the current code. In practice, SRP is applied via eyeball review: a senior developer reads the class and decides whether its methods "feel cohesive." This is both expensive (requires expert time) and unreliable (disagreement between experts is the norm).

### 1.2 Existing metric-based proxies

Classical proxies attempt to approximate SRP via structural counts:
- **Method count** (crude size-based proxy). Fails on legitimate large APIs (Facades, Repositories).
- **Field count**. Fails on data classes.
- **LCOM1-5** (Chidamber & Kemerer 1994, Hitz & Montazeri 1995). Measures lack of cohesion via method-field interaction; LCOM4 builds a graph of methods connected by shared fields and reports the number of connected components. Fails on data classes where each method accesses a distinct field (17 unrelated getters score LCOM4 = 17).

The fundamental weakness of these proxies: they measure **internal structure** only, ignoring the **external surface** of each method. A type whose methods all reach *outside themselves* to coordinate different external domains is semantically different from a type whose methods only manage internal state.

### 1.3 The informal thesis

We claim that SRP can be operationalized through a **two-dimensional profile** of each method:
- **Internal reach**: the fields of the enclosing type that the method reads or writes.
- **External reach**: the external packages, types, and functions that the method calls.

The combined profile determines whether a method is "accountable to" a responsibility. Methods with overlapping profiles share a responsibility; methods with disjoint profiles discharge distinct responsibilities.

The key technical insight that separates our formulation from LCOM4: methods with **empty external reach** should be treated as sharing a common "pure" responsibility, because they are all in the business of managing the type itself. This reflects the intuition that 17 getters on a `User` struct do not each represent a separate responsibility — collectively, they implement the single responsibility "exposing User state."

---

## 2. Formal definition

### 2.1 The enriched graph

Let the architecture graph be the quadruple $G = (N, E, \tau, \kappa)$ where:
- $N$ is the set of nodes (methods, functions, types, packages, fields),
- $E \subseteq N \times N$ is the set of directed edges,
- $\tau : N \to \text{Kind}$ assigns each node a kind (method, function, type, package, field),
- $\kappa : E \to \text{Rel}$ assigns each edge a relation (call, contains, implements, field_read, field_write).

This is the graph produced by `archlint collect` after issue #153 extends the model to include field nodes and field-access edges.

### 2.2 The reach of a method

For each method node $m$ in a type $T$, define its **reach** as:

$$
R(m) = R_{\text{int}}(m) \sqcup R_{\text{ext}}(m)
$$

where:
- $R_{\text{int}}(m) = \{ f \in N : \tau(f) = \text{field}, f \in T, (m, f) \in E \}$ is the set of fields of $T$ touched by $m$ (via field_read or field_write edges),
- $R_{\text{ext}}(m) = \{ n \in N : \tau(n) \in \{\text{method}, \text{function}, \text{type}\}, n \notin T, (m, n) \in E \}$ is the set of external nodes called by $m$.

We treat $R(m)$ as a typed disjoint union so that $R_{\text{int}}$ and $R_{\text{ext}}$ cannot accidentally intersect.

### 2.3 The responsibility equivalence relation

Define a relation $\sim$ on the methods $M(T)$ of type $T$:

$$
m_i \sim m_j \iff
\begin{cases}
R(m_i) \cap R(m_j) \ne \emptyset, \text{ or} \\
(m_i, m_j) \in E \text{ or } (m_j, m_i) \in E, \text{ or} \\
R_{\text{ext}}(m_i) = R_{\text{ext}}(m_j) = \emptyset.
\end{cases}
$$

Let $\approx$ be the transitive closure of $\sim$. Then $\approx$ is an equivalence relation on $M(T)$.

The three disjunctive clauses express:
1. **Shared resource**: two methods touch some common field or call some common external target.
2. **Internal delegation**: one method calls the other — they cooperate on the same responsibility.
3. **Pure union**: both methods have empty external reach; they are unified as "pure self-management."

Clause 3 is the critical departure from LCOM4 and resolves the 17-getters false positive.

### 2.4 The responsibility count

The **responsibility count** of type $T$ is:

$$
\rho(T) = | M(T) / \approx |
$$

— the cardinality of the quotient set of $M(T)$ by $\approx$.

The principle of single responsibility is satisfied iff $\rho(T) = 1$. Values $\rho(T) > 1$ indicate the type should be decomposed into $\rho(T)$ distinct types, one per equivalence class.

### 2.5 Worked examples

Let $T = \text{User}$ with fields $\{\text{name}, \text{email}, \text{age}\}$ and three configurations of methods.

**Configuration A: pure data class (3 getters).**
- $m_1 = \text{GetName}$: $R(m_1) = (\{\text{name}\}, \emptyset)$.
- $m_2 = \text{GetEmail}$: $R(m_2) = (\{\text{email}\}, \emptyset)$.
- $m_3 = \text{GetAge}$: $R(m_3) = (\{\text{age}\}, \emptyset)$.

Pairwise intersections of $R$ are empty. Pairwise calls are absent. But all three methods satisfy $R_{\text{ext}} = \emptyset$, so clause 3 unifies them: $m_1 \sim m_2, m_2 \sim m_3$. Thus $\rho(T) = 1$.

**Configuration B: god object (3 methods reaching different external domains).**
- $m_1 = \text{Save}$: $R(m_1) = (\{\text{name}, \text{email}\}, \{\text{DatabaseRepo}\})$.
- $m_2 = \text{Notify}$: $R(m_2) = (\{\text{email}\}, \{\text{EmailService}\})$.
- $m_3 = \text{Charge}$: $R(m_3) = (\{\text{...}\}, \{\text{PaymentGateway}\})$.

$m_1$ and $m_2$ share the field `email`, so $m_1 \sim m_2$. But $m_3$ shares neither a field (only `email` is common) nor an external target (`PaymentGateway` is distinct from both `DatabaseRepo` and `EmailService`) nor is it connected to $m_1$ or $m_2$ through empty external (its external is non-empty). So $m_3 \not\sim m_1$ and $m_3 \not\sim m_2$.

Result: $\{m_1, m_2\}$ and $\{m_3\}$ are distinct classes. $\rho(T) = 2$.

Note: this example reveals that the relation requires further refinement in practice. In a classic god object all three methods would typically share the `id` field; the example is contrived. In real code $\rho$ tends toward the intuitive answer.

**Configuration C: focused service.**
- $m_1 = \text{CreateOrder}$: $R(m_1) = (\{\text{userId}\}, \{\text{OrderRepo}\})$.
- $m_2 = \text{GetOrders}$: $R(m_2) = (\{\text{userId}\}, \{\text{OrderRepo}\})$.
- $m_3 = \text{PublishOrder}$: $R(m_3) = (\{\text{userId}\}, \{\text{OrderRepo}, \text{EventBus}\})$.

All three share $\{\text{userId}\}$ and $\{\text{OrderRepo}\}$. All pairs are in relation. $\rho(T) = 1$.

---

## 3. Set-theoretic treatment

*[draft in progress]*

The equivalence relation above is fundamentally a set-theoretic construct: we partition methods by the transitive closure of a symmetric binary relation defined via set intersection. This section develops the treatment rigorously, relates it to the partition lattice on $M(T)$, and proves several structural properties.

### 3.1 The partition lattice

*[to continue]*

### 3.2 Monotonicity under refinement of reach

*[to continue]*

### 3.3 Stability under edge addition

*[to continue]*

---

## 4. Graph-theoretic treatment (community detection)

*[draft in progress]*

---

## 5. Information-theoretic treatment

*[draft in progress]*

---

## 6. Spectral treatment

*[draft in progress]*

---

## 7. Topological treatment

*[draft in progress]*

---

## 8. Cross-validation matrix

*[to compute]*

---

## 9. Relationship to existing metrics

*[draft in progress]*

### 9.1 LCOM1-5 family

### 9.2 CK metrics

### 9.3 MOOD metrics

### 9.4 Why $\rho$ is not LCOM4 rebadged

---

## 10. Golden dataset

*[to construct — 20-30 hand-labeled types]*

---

## 11. Anticipated critiques and rebuttals

*[draft in progress]*

---

## 12. Bibliography

*[to compile]*

