# Templates for Software Architecture & Development

A collection of templates for systematic work with architecture: from project handover to specifications and ADRs.

---

## Sections

### 📋 [Project Handover](project-handover/) - Project Onboarding
Process and checklists for project onboarding into architectural oversight.

**What's inside:**
- Handover process (6 phases, 7-14 days)
- Handover checklist (11 sections, 80+ items)

**When to use:**
- Accepting a project under architectural supervision
- Need a formal handover process
- Want to standardize onboarding

---

### 🔍 [System Audit](system-audit/) - System Audits
Templates for detailed architecture audits: manual and automated.

**What's inside:**
- Manual audit (12 sections, 450+ lines)
- Automated validation (215 metrics)

**When to use:**
- During project onboarding
- For quarterly architecture review
- Before migration/refactoring
- In CI/CD for continuous validation

---

### 📝 [Specifications](specifications/) - Specifications
Templates for spec-driven development with Claude Code.

**What's inside:**
- Universal template (XS/S/M/L/XL)
- Filled examples

**When to use:**
- For detailed specs before implementation
- When working with AI (Claude Code)
- For documenting functionality

---

### 🏛️ [ADR](adr/) - Architecture Decision Records
Template for documenting architectural decisions.

**What's inside:**
- ADR template (RU/EN)
- Usage examples

**When to use:**
- When choosing technology
- When changing architectural style
- For decisions with trade-offs

---

## Quick Start

### Project Onboarding

1. Read [Handover Process](project-handover/project-handover-process.md)
2. Use [Handover Checklist](project-handover/project-handover.md)
3. Conduct [System Audit](system-audit/system-audit.md)
4. (Optional) Run automated validation

### Creating Specification

1. Choose size (XS/S/M/L/XL)
2. Copy [spec-template.md](specifications/spec-template.md)
3. Follow comments in the template
4. Check [example](specifications/example-spec.md)

### Documenting Decision

1. Use [ADR template](adr/adr.md)
2. Describe context, options, decision
3. Save in `docs/adr/`

---

## Typical Workflows

### Workflow 1: Onboarding 4 Projects

**Situation:** Manager assigned 4 diverse projects for oversight

**Actions:**

```bash
# For each project:

# 1. Initiation (1 day)
- Get briefing from PM
- Request access
- Schedule kickoff

# 2. Kickoff (1-2 hours)
- Meeting PM + Tech Lead
- Understand context and expectations

# 3. Information Gathering (3-5 days)
cp templates/project-handover/project-handover.md docs/handover/project-a.md
# Fill checklist

# 4. Audit (3-7 days)
cp templates/system-audit/system-audit.md docs/audits/project-a-audit.md
archlint validate > docs/audits/project-a-validation.md
# Conduct combined audit

# 5. Decision (1 day)
# Present results
# Make decision: ACCEPTED / WITH CONDITIONS / REJECTED

# 6. Onboarding (ongoing)
# Set up regular meetings
# Define control points
```

**Result:**
- 4 projects accepted with assessments and improvement plans
- Systematic oversight established
- Everything documented

---

### Workflow 2: Spec-Driven Development

**Situation:** Need to implement new feature with Claude Code

**Actions:**

```bash
# 1. Define size
# M-size: new feature with integration

# 2. Create specification
cp templates/specifications/spec-template.md specs/todo/0042-new-feature.md

# 3. Fill in detail
# - Component + Data Model + Sequence diagrams
# - Requirements with API
# - 10-15 Acceptance Criteria
# - 5-10 Implementation Steps

# 4. Move to work
mv specs/todo/0042-new-feature.md specs/inprogress/

# 5. Implement with Claude Code
# Claude reads specification and implements

# 6. Complete
mv specs/inprogress/0042-new-feature.md specs/done/
```

**Result:**
- Detailed specification for AI
- Quality implementation
- Documentation for future

---

### Workflow 3: Architectural Decision

**Situation:** Need to choose between PostgreSQL and MongoDB

**Actions:**

```bash
# 1. Create ADR
cp templates/adr/adr.md docs/adr/ADR-0005-database-choice.md

# 2. Fill sections
# - Context (why DB needed, requirements)
# - Options (PostgreSQL, MongoDB, MySQL)
# - Decision (PostgreSQL)
# - Consequences (pros and cons)

# 3. Review
# Tech Lead + Architect

# 4. Accept
# Status: Accepted
```

**Result:**
- Documented decision with context
- Justification for future generations
- History of architectural decisions

---

## Project Directory Structure

Recommended structure for using templates:

```
your-project/
├── docs/
│   ├── adr/                           # Architecture Decision Records
│   │   ├── README.md                  # Index of all ADRs
│   │   ├── ADR-0001-database.md
│   │   └── ADR-0002-api-gateway.md
│   ├── audits/                        # Audit results
│   │   ├── 2026-01-project-audit.md
│   │   └── 2026-01-validation.md
│   └── handover/                      # Project handover
│       └── project-a-handover.md
├── specs/                             # Specifications
│   ├── todo/
│   ├── inprogress/
│   └── done/
└── .archlint.yaml                     # Config for automated validation
```

---

## Best Practices

### Project Handover
1. **Follow the process** - don't skip phases
2. **Kickoff is critical** - make sure you understand expectations
3. **Document everything** - don't rely on memory
4. **Combine approaches** - manual audit + automation
5. **Provide plan** - not just problems, but solutions

### System Audits
1. **Automation first** - quick overview
2. **Manual for context** - understand causes
3. **Prioritize** - not everything is critical
4. **Be objective** - rely on metrics
5. **Repeat regularly** - track progress

### Specifications
1. **Define size correctly** - don't overcomplicate
2. **Follow comments** - template contains hints
3. **Detail requirements** - larger spec = more detailed API
4. **UML with methods** - not just fields!
5. **Code examples** - mandatory for M/L/XL

### ADR
1. **Document context** - why the problem arose
2. **Consider alternatives** - minimum 3 options
3. **Justify decision** - not just "chose X"
4. **Honest consequences** - positive AND negative
5. **Update statuses** - don't delete outdated ones

---

## Related Resources

**Industry Practices:**
- [Futurice Project Handover Checklist](https://github.com/futurice/project-handover-checklist)
- [TOGAF Architecture Review](https://www.opengroup.org/architecture/togaf7-doc/arch/p4/comp/clists/syseng.htm)
- [C4 Model](https://c4model.com/)
- [ADR GitHub Organization](https://adr.github.io/)

**Books:**
- Software Architecture in Practice (Bass, Clements, Kazman)
- Building Evolutionary Architectures (Ford, Parsons, Kua)
- Fundamentals of Software Architecture (Richards, Ford)

**Tools:**
- [PlantUML](https://plantuml.com/) - diagrams
- [archlint](https://github.com/mshogin/archlint) - automated validation

---

## Feedback

Templates are developed and improved continuously. If you have suggestions:
- Open Issues: https://github.com/mshogin/archlint/issues
- Create Pull Requests
- Share your experience

---

*These templates are based on industry practices and real experience working with architecture in enterprise.*
