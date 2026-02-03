# System Audit

Templates for detailed architecture system audits: manual and automated.

---

## Quick Start

**Combined approach (recommended):**
1. Run automated validation: `archlint validate`
2. Conduct manual audit per [system-audit.md](system-audit.md)
3. Combine results
4. Create improvement plan

**Manual audit only:**
1. Use template [system-audit.md](system-audit.md)
2. Fill 12 sections
3. Give ratings (1-5)
4. Create improvement plan

**Automated only:**
1. Configure `.archlint.yaml`
2. Run: `archlint validate`
3. Use [audit.md](audit.md) as report template

---

## Files

### [system-audit.md](system-audit.md)
**Template for detailed manual system audit**

**Volume:** 12 sections, 450+ lines

**What's inside:**

1. **System Overview**
   - Purpose and boundaries
   - Context diagram (C4 Level 1)
   - External systems and users

2. **System Structure**
   - Service catalog
   - Container diagram (C4 Level 2)
   - Bounded Contexts (if DDD)
   - Structure rating

3. **Service Interactions**
   - Synchronous (REST, gRPC)
   - Asynchronous (Kafka, RabbitMQ)
   - Dependency matrix
   - Critical paths

4. **Patterns and Practices**
   - Used patterns (API Gateway, Circuit Breaker, etc.)
   - ADR (Architecture Decision Records)
   - Anti-patterns

5. **Data and Storage**
   - Databases
   - Caching
   - Data consistency

6. **Test Coverage**
   - Unit / Integration / E2E
   - Contract Testing
   - Load Testing

7. **Observability**
   - Metrics (RED, USE)
   - Logging (correlation ID)
   - Tracing
   - Alerting

8. **Resilience**
   - Single Points of Failure
   - Graceful Degradation
   - Scalability
   - Disaster Recovery

9. **Security**
   - Authentication/Authorization
   - Data protection
   - Known vulnerabilities

10. **Issues and Recommendations Summary**
    - Critical (P0)
    - Important (P1)
    - Desirable (P2)

11. **Improvement Plan**
    - Immediate (1-2 weeks)
    - Mid-term (1-3 months)
    - Long-term (3+ months)

12. **Conclusion**
    - Overall rating (X/5)
    - Key findings
    - Next audit recommendation

**When to use:**
- During "Audit" phase of project handover
- For deep analysis of existing system
- Before migration or refactoring
- For quarterly architecture review

**Ratings:**
- Architectural maturity (1-5)
- Operational readiness (1-5)
- Resilience (1-5)
- Scalability (1-5)
- Security (1-5)

**Result:**
- Objective system state picture
- Critical issues list with priorities
- Prioritized improvement plan
- Production/scale readiness assessment

---

### [audit.md](audit.md)
**Report template for automated architecture validation**

**What's inside:**
- Results of checking 215 quality metrics
- Categories: ERROR, WARNING, PASSED, INFO
- Specific issues with recommendations

**Metrics:**

**1. Coupling**
- max_fan_out - component depends on too many others
- coupling - afferent (Ca) and efferent (Ce) coupling
- instability - component instability metric

**2. Architectural Principles**
- layer_violations - layer violations (handler -> repository)
- god_class - too many methods and dependencies
- interface_segregation - interfaces > 5 methods

**3. Domain-Driven Design**
- use_case_purity - mixing business logic with infrastructure
- bounded_context_leakage - detail leakage between contexts
- service_autonomy - direct dependencies between services

**4. Change Metrics**
- change_propagation - how changes propagate
- hotspot_detection - component change frequency
- stability_violations - unstable depending on unstable

**5. Topological Metrics**
- betti_numbers - topological complexity
- euler_characteristic - graph Euler characteristic
- forman_curvature - edge curvature (structural stress)
- spectral_gap - impact on change propagation

**6. Other Metrics**
- hub_nodes - nodes with too many connections
- articulation_points - articulation points (SPOF)
- circular_dependencies - circular dependencies
- feature_envy - method uses foreign data

**Tool:** archlint validate

```bash
# Basic validation
archlint validate

# With config
archlint validate --config .archlint.yaml

# With output to file
archlint validate > docs/audits/2026-01-audit.md
```

**When to use:**
- Supplement to manual audit system-audit.md
- For objective code quality assessment
- In CI/CD for continuous architecture validation
- For tracking architectural degradation

**Result:**
- ERROR/WARNING/PASSED counts
- Problem components list with metrics
- Specific recommendations: "What will improve"
- Successful checks list

---

## Audit Approaches

### Approach 1: Combined (recommended)

**Step 1: Automated Validation**
```bash
archlint validate --config .archlint.yaml > audit-auto.md
```

**Step 2: Manual Audit**
- Fill system-audit.md
- Focus on what automation doesn't see:
  - Business context
  - ADR and architectural decisions
  - Operational readiness (runbooks, DR)
  - Team and knowledge transfer

**Step 3: Combine**
- Critical issues from automation -> system-audit.md section 10
- Metrics from automation -> system-audit.md section 3
- Overall conclusions

**Advantages:**
- Objectivity (automation)
- Context (manual)
- Complete picture

---

### Approach 2: Manual Only

**When to use:**
- No code access (only docs and monitoring)
- Process audit, not code
- High-level overview for management

**Steps:**
1. Fill all 12 sections of system-audit.md
2. Give ratings (1-5)
3. Create improvement plan

---

### Approach 3: Automated Only

**When to use:**
- Continuous validation in CI/CD
- Quick check of changes
- Tracking metrics over time

**Steps:**
1. Configure `.archlint.yaml`
2. Run `archlint validate`
3. Analyze ERROR and WARNING

**Limitations:**
- Sees only code, not context
- Doesn't know business requirements
- Doesn't assess operational readiness

---

## Usage Examples

### Example 1: Project Onboarding

**Context:** Accepting project under supervision

**Actions:**
1. Combined audit
2. Automation: `archlint validate`
3. Manual: fill system-audit.md
4. Document critical issues
5. Decision: Accepted/With Conditions/Rejected

**Result:**
- Report system-audit.md with rating X/5
- Blockers list for acceptance
- 3-month improvement plan

---

### Example 2: Quarterly Review

**Context:** Quarterly architecture check

**Actions:**
1. Quick automated validation
2. Compare with previous quarter
3. Focus on changes and degradation
4. Update improvement plan

**Result:**
- Metrics trends (improvement/degradation)
- New issues
- Improvement plan progress

---

### Example 3: Before Migration

**Context:** Planning microservices migration

**Actions:**
1. Detailed manual audit
2. Focus on bounded contexts and coupling
3. Automation for metric confirmation
4. Migration plan with priorities

**Result:**
- Dependency map
- Service separation priorities
- Migration complexity assessment

---

## Metrics and Thresholds

### Critical Metrics (ERROR if exceeded)

| Metric | Threshold | Meaning |
|---------|-------|-------------|
| max_fan_out | 5 | Component depends on > 5 others |
| coupling (Ca/Ce) | 10 | Too many incoming/outgoing dependencies |
| layer_violations | 0 | Architectural layer violations |
| god_class methods | 20 | Too many methods in one class |
| god_class deps | 15 | Too many dependencies |
| interface methods | 5 | Interface too large |
| circular_dependencies | 0 | Circular dependencies |

### Quality Metrics (rating)

| Metric | Normal | Shows |
|---------|-------|----------------|
| test_coverage | >= 70% | Test coverage |
| instability | 0.0-1.0 | Component stability (0=stable) |
| abstractness | 0.0-1.0 | Component abstractness |
| distance_from_main_sequence | 0.0-0.5 | Deviation from ideal |

---

## Best Practices

### Manual Audit
1. **Start with overview** - understand system as whole before details
2. **Use diagrams** - C4 helps structure thinking
3. **Focus on risks** - not all problems critical
4. **Provide plan** - not just problems, but solutions
5. **Be objective** - rely on metrics, not impressions

### Automated Validation
1. **Configure thresholds** - adapt to your project
2. **Integrate in CI/CD** - check on every PR
3. **Track trends** - dynamics matter, not absolute
4. **Fix ERROR** - WARNING can be deferred
5. **Document exceptions** - why ignoring metric

### Combined Approach
1. **Automation first** - quick overview
2. **Manual for context** - understand problem causes
3. **Combine results** - unified report
4. **Prioritize** - not everything critical
5. **Repeat regularly** - track progress

---

## FAQ

**Q: How often to audit?**

A: Depends on maturity:
- New projects: every 3 months
- Stable: every 6-12 months
- After critical changes: ad-hoc
- In CI/CD: automatically on every PR

**Q: How long does manual audit take?**

A: Usually 3-7 days:
- Day 1-2: Architecture overview and info gathering
- Day 3-4: Deep dive
- Day 5-7: Analysis and report

**Q: Need to fill all system-audit.md sections?**

A: Strive for completeness. Minimum for acceptance:
- System structure
- Service interactions
- Observability
- Security

**Q: What if many ERRORs in automated validation?**

A: Prioritize:
1. First layer_violations (architecturally critical)
2. Then coupling and fan_out (affect maintainability)
3. Then rest

**Q: Can ignore some metrics?**

A: Yes, but document in `.archlint.yaml`:
```yaml
ignore:
  - metric: god_class
    component: LegacyService
    reason: "Gradual refactoring, Q2 plan"
```

---

## Related Resources

**Project Handover:**
- [../project-handover/](../project-handover/) - process and checklist

**Industry Practices:**
- [TOGAF Architecture Review](https://www.opengroup.org/architecture/togaf7-doc/arch/p4/comp/clists/syseng.htm)
- [C4 Model](https://c4model.com/)
- Software Architecture in Practice (Bass, Clements, Kazman)

**Metrics:**
- [Robert Martin's Metrics](https://en.wikipedia.org/wiki/Software_package_metrics)
- [Cyclomatic Complexity](https://en.wikipedia.org/wiki/Cyclomatic_complexity)
- [Cognitive Complexity](https://www.sonarsource.com/resources/cognitive-complexity/)

---

*Templates created based on industry practices and real experience auditing systems of various scales.*
