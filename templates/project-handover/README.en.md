# Project Handover (Project Onboarding)

Templates for systematic project onboarding into architectural oversight.

---

## Quick Start

1. Read [Handover Process](project-handover-process.md) - 6 phases, 7-14 days
2. Use [Handover Checklist](project-handover.md) - 11 sections, 80+ items
3. Conduct audit using templates from [../system-audit/](../system-audit/)

---

## Files

### [project-handover-process.md](project-handover-process.md)
**Process for project onboarding into architectural oversight**

**What's inside:**
- 6 phases: Initiation -> Kickoff -> Information Gathering -> Audit -> Decision -> Onboarding
- Timeline: 7-14 days
- Roles: Architect, PM, Tech Lead
- Communication templates (email, meetings)
- Success metrics for oversight

**Process structure:**

```
[Initiation] -> [Kickoff] -> [Info Gathering] -> [Audit] -> [Decision] -> [Onboarding]
   1 day       1-2 hours        3-5 days          3-7 days    1 day      ongoing
```

**When to use:**
- Accepting project under architectural supervision
- Need formal project handover process
- Want to standardize onboarding

**Key phases:**

**Phase 1: Initiation (1 day)**
- Get briefing from PM
- Request access (repos, monitoring, docs)
- Schedule kickoff meeting

**Phase 2: Kickoff meeting (1-2 hours)**
- Business context from PM
- Technical picture from Tech Lead
- Align expectations and work format

**Phase 3: Information Gathering (3-5 days)**
- Documentation (architecture, ADR, API specs)
- Code and infrastructure
- Operational info (monitoring, incidents, SLA)

**Phase 4: Audit (3-7 days)**
- Architecture overview (structure, bounded contexts)
- Deep dive (code, tests, CI/CD, monitoring)
- Analysis and report

**Phase 5: Decision (1 day)**
- Present audit results
- Discuss critical issues
- Agree on improvement plan
- Decision: Accepted / With Conditions / Rejected

**Phase 6: Onboarding (ongoing)**
- Join communications
- Set up regular touchpoints
- Define control points (code review, design review, ADR)

---

### [project-handover.md](project-handover.md)
**Checklist for project onboarding**

**What's inside:**
- 11 sections, 80+ check items
- Acceptance criteria
- Readiness ratings (1-5)
- Three decision options

**Sections:**

1. **Business Context**
2. **System Analysis**
3. **Architecture**
4. **Code and Repositories**
5. **Infrastructure**
6. **Operational Readiness**
7. **Security**
8. **Team and Knowledge**
9. **Open Issues and Risks**
10. **Improvement Plan**
11. **Decision**

**When to use:**
- During "Audit" phase of handover process
- For structured information gathering about project
- To capture project state

**Minimum acceptance criteria:**
- [ ] Architecture understanding (diagram)
- [ ] Access to code and monitoring
- [ ] Tech Lead or responsible person
- [ ] No critical security issues
- [ ] System in production and working

**Result:**
- Documented project state
- Readiness ratings across all areas
- List of blockers and risks
- Formal decision on acceptance

---

## Handover Workflow

### Typical Scenario (7-14 days)

**Day 1: Initiation**
```bash
# Get request
# Create project card
# Request access
# Schedule kickoff
```

**Day 2: Kickoff**
```bash
# Meeting PM + Tech Lead (1-2 hours)
# Understand business context
# Understand technical picture
# Align expectations
```

**Day 3-7: Information Gathering + Audit**
```bash
# Collect documentation
# Study code
# Check monitoring
# Fill project-handover.md
# Conduct system-audit (see ../system-audit/)
```

**Day 8: Decision**
```bash
# Present results
# Discuss issues
# Agree on plan
# Make decision: ACCEPTED / WITH CONDITIONS / REJECTED
```

**Day 9+: Onboarding**
```bash
# Set up regular meetings
# Define control points
# Start work
```

---

## Usage Examples

### Project A: Distributed Monolith
**Audit showed:**
- 5 tightly coupled services
- Shared database
- No monitoring alerts

**Decision:** Accepted with conditions
- 3-month refactoring plan
- Set up alerts in 2 weeks

### Project B: All Good
**Audit showed:**
- Clear architecture
- Tests, monitoring, documentation in place
- Competent Tech Lead

**Decision:** Accepted
- Weekly sync with Tech Lead
- Participation in design reviews

### Project C: Critical Issues
**Audit showed:**
- Critical security issues
- Tech Lead leaving in a week
- No documentation

**Decision:** Rejected
- Return for improvements
- Requirements: fix security issues, knowledge transfer
- Re-audit in a month

---

## Best Practices

1. **Don't skip kickoff** - most important meeting for understanding context
2. **Document everything** - don't rely on memory, fill checklist
3. **Be objective** - rely on metrics, not impressions
4. **Provide plan** - not just problems, but specific actions
5. **Watch timeline** - 2 weeks maximum, otherwise get stuck

---

## FAQ

**Q: Must go through all phases?**

A: Yes, for formal handover. Can expedite but not skip.

**Q: What if Tech Lead unavailable?**

A: Escalate to PM. Full handover impossible without Tech Lead.

**Q: How many projects can onboard in parallel?**

A: Recommended 1-2 maximum. Each requires 50-70% time for 2 weeks.

**Q: What if project not accepted?**

A: Document blockers, return for improvements, schedule re-audit.

**Q: Need to fill all checklist items?**

A: Strive for completeness. Minimum: business context, architecture, operational readiness, security.

---

## Related Resources

**Audit:**
- [../system-audit/](../system-audit/) - templates for detailed audit

**Industry Practices:**
- [Futurice Project Handover Checklist](https://github.com/futurice/project-handover-checklist)
- [TOGAF Architecture Review](https://www.opengroup.org/architecture/togaf7-doc/arch/p4/comp/clists/syseng.htm)
- [Harvard EA Checklist](https://enterprisearchitecture.harvard.edu/application-architecture-checklist)

---

*Process developed based on industry practices and real experience onboarding projects into architectural oversight.*
