# Architecture Validation Report [Project Name]

**EN** | [RU](audit.md)

**Date:** [YYYY-MM-DD]
**Commit:** [commit hash] ([commit message])

## Executive Summary

Automated architecture validation of [Project Name] project was conducted using 215 different code quality and architecture metrics. [X] checks performed ([Y]%).

### Overall Statistics

| Status | Count | Percentage |
|--------|-------|------------|
| ✅ PASSED | [X] | [Y]% |
| ❌ ERROR | [X] | [Y]% |
| ⚠️ WARNING | [X] | [Y]% |
| ℹ️ INFO | [X] | [Y]% |
| **Total** | **[X]** | **100%** |

**Overall Assessment:** [Brief architecture state assessment]

---

## 1. Critical Issues (ERROR) - [X]

### 1.1 Connectivity and Coupling Metrics

#### max_fan_out
**Problem:** [Problem description]
**Risk:** [Risk description]

**Problematic Components:**
- `[component-name-1]` ([metric-value], limit: [limit]) - [brief problem description]
  **Improvements:** [specific improvements for this component]
- `[component-name-2]` ([metric-value], limit: [limit]) - [brief problem description]
  **Improvements:** [specific improvements for this component]

#### coupling
**Problem:** [Problem description]
**Risk:** [Risk description]

**Problematic Components:**
- `[component-name-1]` (Ca: [X], Ce: [Y], limit: 10) - [brief problem description]
  **Improvements:** [specific improvements for this component]
- `[component-name-2]` (Ca: [X], Ce: [Y], limit: 10) - [brief problem description]
  **Improvements:** [specific improvements for this component]

#### instability
**Problem:** [Problem description]
**Risk:** [Risk description]

**Problematic Components:**
- `[component-name-1]` (I = [value]) - [brief problem description]
  **Improvements:** [specific improvements for this component]
- `[component-name-2]` (I = [value]) - [brief problem description]
  **Improvements:** [specific improvements for this component]

### 1.2 Architecture Principles

#### layer_violations
**Problem:** [Problem description]
**Status:** ❌ CRITICAL
**Description:** [Detailed description]

**Problematic Components:**
- `[component-name-1]` -> `[target-component]` - [violation description, e.g., "handler accesses repository, bypassing service"]
  **Improvements:** [specific improvements for this component]
- `[component-name-2]` -> `[target-component]` - [violation description]
  **Improvements:** [specific improvements for this component]

**Actions:**
- [Action 1]
- [Action 2]

#### god_class
**Problem:** [Problem description]
**Risk:** [Risk description]

**Problematic Components:**
- `[component-name-1]` (methods: [X], dependencies: [Y]) - [brief problem description]
  **Improvements:** [specific improvements for this component]
- `[component-name-2]` (methods: [X], dependencies: [Y]) - [brief problem description]
  **Improvements:** [specific improvements for this component]

**Recommendations:**
- [Recommendation 1]
- [Recommendation 2]

#### interface_segregation
**Problem:** [Problem description]
**Risk:** [Risk description]

**Problematic Components:**
- `[interface-name-1]` (methods: [X], limit: 5) - [brief problem description]
  **Improvements:** [specific improvements for this component]
- `[interface-name-2]` (methods: [X], limit: 5) - [brief problem description]
  **Improvements:** [specific improvements for this component]

**Recommendations:**
- [Recommendation 1]
- [Recommendation 2]

### 1.3 Domain-Driven Design

#### use_case_purity
**Problem:** [Problem description]
**Risk:** [Risk description]

**Problematic Components:**
- `[use-case-name-1]` - [description of business logic mixing with infrastructure]
  **Improvements:** [specific improvements for this component]
- `[use-case-name-2]` - [description of business logic mixing with infrastructure]
  **Improvements:** [specific improvements for this component]

#### bounded_context_leakage
**Problem:** [Problem description]
**Risk:** [Risk description]

**Problematic Components:**
- `[context-1]` -> `[context-2]` - [description of implementation details leakage]
  **Improvements:** [specific improvements for this component]
- `[context-3]` -> `[context-4]` - [description of implementation details leakage]
  **Improvements:** [specific improvements for this component]

#### service_autonomy
**Problem:** [Problem description]
**Risk:** [Risk description]

**Problematic Components:**
- `[service-name-1]` - [description of direct dependencies on other services]
  **Improvements:** [specific improvements for this component]
- `[service-name-2]` - [description of direct dependencies on other services]
  **Improvements:** [specific improvements for this component]

### 1.4 Change Metrics

#### change_propagation
**Problem:** [Problem description]
**Risk:** [Risk description]

**Problematic Components:**
- `[component-name-1]` (affects [X] components) - [description of change propagation]
  **Improvements:** [specific improvements for this component]
- `[component-name-2]` (affects [X] components) - [description of change propagation]
  **Improvements:** [specific improvements for this component]

#### hotspot_detection
**Problem:** [Problem description]
**Risk:** [Risk description]

**Problematic Components:**
- `[component-name-1]` (changes: [X]) - [description of change frequency]
  **Improvements:** [specific improvements for this component]
- `[component-name-2]` (changes: [X]) - [description of change frequency]
  **Improvements:** [specific improvements for this component]

#### stability_violations
**Problem:** [Problem description]
**Risk:** [Risk description]

**Problematic Components:**
- `[component-name-1]` (unstable) -> `[component-name-2]` (unstable) - [violation description]
  **Improvements:** [specific improvements for this component]
- `[component-name-3]` (unstable) -> `[component-name-4]` (unstable) - [violation description]
  **Improvements:** [specific improvements for this component]

### 1.5 Other Critical Metrics

#### hub_nodes
**Description:** [Problem description]

**Problematic Components:**
- `[component-name-1]` (connections: [X]) - [problem description]
  **Improvements:** [specific improvements for this component]

#### component_distance
**Description:** [Problem description]

**Problematic Components:**
- `[component-name-1]` -> `[component-name-2]` (distance: [X]) - [problem description]
  **Improvements:** [specific improvements for this component]

#### distance_from_main_sequence
**Description:** [Problem description]

**Problematic Components:**
- `[component-name-1]` (D = [value]) - [description of deviation from main sequence]
  **Improvements:** [specific improvements for this component]

#### zscore_outliers
**Description:** [Problem description]

**Problematic Components:**
- `[component-name-1]` (z-score: [X]) - [anomaly description]
  **Improvements:** [specific improvements for this component]

#### cohesion_lcom4
**Description:** [Problem description]

**Problematic Components:**
- `[component-name-1]` (LCOM4: [X]) - [low cohesion description]
  **Improvements:** [specific improvements for this component]

#### feature_envy
**Description:** [Problem description]

**Problematic Components:**
- `[method-name-1]` in `[class-1]` -> `[class-2]` - [description of accessing foreign data]
  **Improvements:** [specific improvements for this component]

#### inward_dependencies
**Description:** [Problem description]

**Problematic Components:**
- `[component-name-1]` (internal) -> `[component-name-2]` (external) - [violation description]
  **Improvements:** [specific improvements for this component]

#### component_complexity
**Description:** [Problem description]

**Problematic Components:**
- `[component-name-1]` (cyclomatic: [X], cognitive: [Y]) - [excessive complexity description]
  **Improvements:** [specific improvements for this component]

#### fundamental_group
**Description:** [Problem description]

**Problematic Components:**
- Cycle: `[comp-1]` -> `[comp-2]` -> `[comp-3]` -> `[comp-1]` - [circular dependency description]
  **Improvements:** [specific improvements for this component]

---

## 2. Warnings (WARNING) - [X]

### 2.1 Structural Warnings

#### articulation_points
**Description:** [Description]
**Risk:** [Risk]

**Problematic Components:**
- `[component-name-1]` - [description that component is articulation point]
  **Improvements:** [specific improvements for this component]
- `[component-name-2]` - [description that component is articulation point]
  **Improvements:** [specific improvements for this component]

#### avg_path_length
**Description:** [Description]
**Risk:** [Risk]

**Problematic Components:**
- `[component-name-1]` -> `[component-name-2]` (path: [X] steps) - [long chain description]
  **Improvements:** [specific improvements for this component]
- `[component-name-3]` -> `[component-name-4]` (path: [X] steps) - [long chain description]
  **Improvements:** [specific improvements for this component]

### 2.2 Code Smells

#### open_closed
**Description:** [Description]
**Risk:** [Risk]

**Problematic Components:**
- `[component-name-1]` - [Open-Closed Principle violation description]
  **Improvements:** [specific improvements for this component]
- `[component-name-2]` - [Open-Closed Principle violation description]
  **Improvements:** [specific improvements for this component]

#### speculative_generality
**Description:** [Description]
**Risk:** [Risk]

**Problematic Components:**
- `[component-name-1]` - [unused abstraction description]
  **Improvements:** [specific improvements for this component]
- `[component-name-2]` - [unused abstraction description]
  **Improvements:** [specific improvements for this component]

#### data_clumps
**Description:** [Description]
**Risk:** [Risk]

**Problematic Components:**
- `[component-name-1]` - parameters: `[param1]`, `[param2]`, `[param3]` - [data group description]
  **Improvements:** [specific improvements for this component]
- `[component-name-2]` - parameters: `[param1]`, `[param2]`, `[param3]` - [data group description]
  **Improvements:** [specific improvements for this component]

### 2.3 Testing

#### mockability
**Description:** [Description]
**Risk:** [Risk]

**Problematic Components:**
- `[component-name-1]` - [mocking difficulty description]
  **Improvements:** [specific improvements for this component]
- `[component-name-2]` - [mocking difficulty description]
  **Improvements:** [specific improvements for this component]

#### test_isolation
**Description:** [Description]
**Risk:** [Risk]

**Problematic Components:**
- `[test-name-1]` - [external state dependency description]
  **Improvements:** [specific improvements for this component]
- `[test-name-2]` - [external state dependency description]
  **Improvements:** [specific improvements for this component]

#### test_coverage_structure
**Description:** [Description]
**Risk:** [Risk]

**Problematic Components:**
- `[component-name-1]` - [missing or mismatched tests description]
  **Improvements:** [specific improvements for this component]
- `[component-name-2]` - [missing or mismatched tests description]
  **Improvements:** [specific improvements for this component]

### 2.4 Topological Metrics

#### betti_numbers
**Description:** [Problem description]

**Problematic Components:**
- `[component-name-1]` (b0: [X], b1: [Y]) - [topological complexity description]
  **Improvements:** [specific improvements for this component]

#### euler_characteristic
**Description:** [Problem description]

**Problematic Components:**
- Graph has Euler characteristic: [X] - [deviation description]
  **Improvements:** [specific improvements for this component]

#### topological_persistence
**Description:** [Problem description]

**Problematic Components:**
- `[component-name-1]` (persistence: [X]) - [long-lived structure description]
  **Improvements:** [specific improvements for this component]

#### channel_capacity
**Description:** [Problem description]

**Problematic Components:**
- `[component-name-1]` -> `[component-name-2]` (capacity: [X]) - [limitation description]
  **Improvements:** [specific improvements for this component]

#### cross_entropy
**Description:** [Problem description]

**Problematic Components:**
- Cross-entropy: [X] - [divergence description]
  **Improvements:** [specific improvements for this component]

#### matrix_rank
**Description:** [Problem description]

**Problematic Components:**
- Matrix rank: [X] (full rank: [Y]) - [redundancy description]
  **Improvements:** [specific improvements for this component]

#### condition_number
**Description:** [Problem description]

**Problematic Components:**
- Condition number: [X] - [sensitivity description]
  **Improvements:** [specific improvements for this component]

#### spectral_gap
**Description:** [Problem description]

**Problematic Components:**
- Spectral gap: [X] - [impact on change propagation description]
  **Improvements:** [specific improvements for this component]

#### graph_density_distribution
**Description:** [Problem description]

**Problematic Components:**
- Area `[area-name]` (density: [X]) - [over-densification/sparsity description]
  **Improvements:** [specific improvements for this component]

#### persistence_diagram
**Description:** [Problem description]

**Problematic Components:**
- Structural feature at scale [X] - [description]
  **Improvements:** [specific improvements for this component]

#### morse_complexity
**Description:** [Problem description]

**Problematic Components:**
- Critical points: [X] - [topological complexity description]
  **Improvements:** [specific improvements for this component]

#### sheaf_cohomology
**Description:** [Problem description]

**Problematic Components:**
- Cohomology: H^[n] = [X] - [global constraints description]
  **Improvements:** [specific improvements for this component]

#### local_consistency
**Description:** [Problem description]

**Problematic Components:**
- Area `[area-name]` - [local consistency violation description]
  **Improvements:** [specific improvements for this component]

#### forman_curvature
**Description:** [Problem description]

**Problematic Components:**
- Edge `[comp-1]` -> `[comp-2]` (curvature: [X]) - [structural stress description]
  **Improvements:** [specific improvements for this component]

---

## 3. Successful Checks (PASSED) - [X]

### 3.1 Core Architecture Principles

✅ **dag_check:** [Description]
✅ **modularity:** [Description]
✅ **single_responsibility:** [Description]
✅ **liskov_substitution:** [Description]
✅ **dependency_inversion:** [Description]

### 3.2 Graph Metrics

✅ **betweenness_centrality:** [Description]
✅ **pagerank:** [Description]
✅ **orphan_nodes:** [Description]
✅ **strongly_connected_components:** [Description]
✅ **graph_depth:** [Description]
✅ **clustering_coefficient:** [Description]

### 3.3 Code Quality

✅ **forbidden_dependencies:** [Description]
✅ **abstractness:** [Description]
✅ **shotgun_surgery:** [Description]
✅ **divergent_change:** [Description]
✅ **lazy_class:** [Description]
✅ **middle_man:** [Description]

### 3.4 Domain & Architecture

✅ **domain_isolation:** [Description]
✅ **ports_adapters:** [Description]
✅ **dto_boundaries:** [Description]
✅ **blast_radius:** [Description]
✅ **deprecated_usage:** [Description]
✅ **circular_dependency_depth:** [Description]

### 3.5 Security

✅ **auth_boundary:** [Description]
✅ **sensitive_data_flow:** [Description]
✅ **health_check_presence:** [Description]

### 3.6 Mathematical Metrics

[List of successful mathematical metrics]

---

## 4. Informational Metrics (INFO) - [X]

Metrics providing additional information without assessment:

[List of informational metrics]

---

## 5. Improvement Recommendations

### 5.1 Critical (Require Immediate Attention)

1. **[Problem Name]**
   - [Action 1]
   - [Action 2]
   - [Action 3]

2. **[Problem Name]**
   - [Action 1]
   - [Action 2]
   - [Action 3]

### 5.2 Important (Medium-term Perspective)

3. **[Problem Name]**
   - [Action 1]
   - [Action 2]

4. **[Problem Name]**
   - [Action 1]
   - [Action 2]

### 5.3 Desirable (Long-term Perspective)

5. **[Problem Name]**
   - [Action 1]
   - [Action 2]

---

## 6. Positive Aspects

✅ **[Aspect 1]** - [Description]
✅ **[Aspect 2]** - [Description]
✅ **[Aspect 3]** - [Description]
✅ **[Aspect 4]** - [Description]
✅ **[Aspect 5]** - [Description]
✅ **[Aspect 6]** - [Description]

---

## 7. Component Metrics

**Total Components:** [X]
- Packages: [X]
- Structs: [X]
- Interfaces: [X]
- Types: [X]
- Functions: [X]
- Methods: [X]
- External: [X]

**Total Connections:** [X]

---

## 8. Test Contexts

Analyzed [X] execution contexts:
1. [context-1] ([X] components)
2. [context-2] ([X] components)
[...]

---

## 9. Conclusion

The architecture of [Project Name] project [overall assessment]. **[X] critical issues** and **[X] warnings** identified that require attention.

**Priorities:**
1. [Priority 1]
2. [Priority 2]
3. [Priority 3]
4. [Priority 4]
5. [Priority 5]

**Next Steps:**
1. [Step 1]
2. [Step 2]
3. [Step 3]
4. [Step 4]
5. [Step 5]

**Overall Score:** [X]/10
- Architecture: [Score]
- Code Quality: [Score]
- Testability: [Score]
- Maintainability: [Score]

---

*Report generated automatically using aiarch validate*
*Generation Date: [YYYY-MM-DD]*
