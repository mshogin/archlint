# Architecture Decision Records (ADR)

Template for documenting architectural decisions.

---

## Quick Start

1. Copy [adr.md](adr.md):
   ```bash
   cp templates/adr/adr.md docs/adr/ADR-0001-database-choice.md
   ```
2. Fill sections
3. Save in `docs/adr/`

---

## What is ADR?

**Architecture Decision Record (ADR)** is a document that captures:
- **Context** - why the problem arose
- **Options** - what solutions were considered
- **Decision** - what was chosen and why
- **Consequences** - what it provides (positive and negative)

**Goal:** Preserve decision-making context for future generations (and yourself in a year).

---

## Files

### [adr.md](adr.md)
**Template for documenting architectural decision**

**Structure:**
- Metadata (number, status, date)
- Context and problem
- Considered options (3+)
- Accepted decision with justification
- C4: Context, Container, Component
- Sequence diagram
- Consequences (positive/negative)
- Alternatives (why rejected)

**Statuses:**
- `Proposed` - proposed, under discussion
- `Accepted` - accepted, being implemented
- `Deprecated` - outdated but still used
- `Superseded by ADR-XXXX` - replaced by another decision

---

### [adr.en.md](adr.en.md)
**English version of template**

For teams with international composition or English documentation.

---

## When to Use ADR

### Must Document

- **Technology/framework choice**
  - Example: PostgreSQL vs MongoDB
  - Example: gRPC vs REST

- **Architectural style change**
  - Example: Monolith -> Microservices
  - Example: Event-Driven -> Request-Response

- **New architectural patterns**
  - Example: Implementing CQRS
  - Example: Moving to Event Sourcing

- **Decisions with trade-offs**
  - Example: Consistency vs Availability (CAP)
  - Example: Latency vs Throughput

- **Security and compliance**
  - Example: Authentication method choice
  - Example: Data encryption strategy

### Can Skip

- Trivial choices (obvious decisions)
- Temporary workarounds
- Local changes without global impact
- Library choices without architectural impact

---

## Directory Structure

```
docs/adr/
├── README.md                          # Index of all ADRs
├── ADR-0001-database-choice.md
├── ADR-0002-api-gateway.md
├── ADR-0003-caching-strategy.md
├── ADR-0004-message-broker.md        # Superseded by ADR-0007
└── ADR-0007-event-streaming.md       # Replaces ADR-0004
```

---

## Naming

```
ADR-NNNN-short-title.md
```

- `NNNN` = 4-digit number (0001, 0002, ..., 9999)
- `short-title` = brief description (kebab-case)

**Examples:**
```
ADR-0001-database-choice.md
ADR-0002-api-gateway-pattern.md
ADR-0003-distributed-tracing.md
```

---

## Workflow

### Step 1: Create ADR

```bash
# Determine number (next in sequence)
NEXT_ADR=0005

# Copy template
cp templates/adr/adr.md docs/adr/ADR-${NEXT_ADR}-your-decision.md
```

### Step 2: Fill

**Required sections:**
1. Metadata (number, status, date)
2. Context and problem
3. Considered options (minimum 3)
4. Accepted decision
5. Consequences

**Optional sections:**
- C4 diagrams
- Sequence diagrams
- Alternatives

### Step 3: Review

ADR should undergo review like code:
- Tech Lead
- Architect
- Stakeholders

### Step 4: Accept

After review, status changes:
```markdown
Status: Accepted
Date: 2026-01-30
```

---

## Updating ADR

### When ADR Becomes Outdated

**Option 1: Deprecated**
```markdown
Status: Deprecated
Deprecated: 2026-06-30
Reason: New performance requirements
```

**Option 2: Superseded**
```markdown
Status: Superseded by ADR-0007
Superseded: 2026-06-30
Reason: Moving to Event Streaming instead of Message Queue
```

### Creating New ADR

When decision changes - create **new ADR**, mark old one:

```bash
# New ADR
cp templates/adr/adr.md docs/adr/ADR-0007-event-streaming.md

# In new ADR specify
Replaces: ADR-0004
Reason: Message Queue cannot handle load

# In old ADR update
Status: Superseded by ADR-0007
```

---

## Best Practices

### 1. Document Context

**Bad:**
```markdown
## Decision
Chose PostgreSQL.
```

**Good:**
```markdown
## Context
Need DB for orders. 10K TPS, ACID transactions critical.
Team knows PostgreSQL. Budget limited.

## Decision
Chose PostgreSQL because...
```

### 2. Consider Minimum 3 Options

Shows problem was analyzed, not just grabbed first thing.

### 3. Justify Decision

**Bad:**
```markdown
Chose Kafka because it's popular.
```

**Good:**
```markdown
Chose Kafka because:
1. RabbitMQ cannot handle (10K vs 50K msg/sec needed)
2. Need event log for Event Sourcing
3. Successful industry cases (LinkedIn, Uber)
```

### 4. Describe Consequences Honestly

Not just positive, but also **negative**:
- Additional complexity
- Overhead
- Risks
- Technical debt

### 5. Update Statuses

When decision becomes outdated - update ADR, don't delete!

---

## FAQ

**Q: Need ADR for all decisions?**

A: No, only for **significant architectural decisions** with global impact.

**Q: Who writes ADR?**

A: Usually architect or Tech Lead. But any team member can.

**Q: Who makes decision?**

A: Depends on organization. Usually Tech Lead + Architect. Sometimes needs CTO approval.

**Q: What to do with outdated ADRs?**

A: Don't delete! Update status to `Deprecated` or `Superseded by ADR-XXXX`.

**Q: Are diagrams needed in ADR?**

A: Desirable but not mandatory. If decision is complex - diagrams help.

**Q: Can accepted ADR be changed?**

A: No. Create new ADR that replaces old one (`Superseded`).

---

## Related Resources

**Methodology:**
- [ADR GitHub Organization](https://adr.github.io/)
- [Documenting Architecture Decisions](https://cognitect.com/blog/2011/11/15/documenting-architecture-decisions)
- [Michael Nygard's ADR article](https://thinkrelevance.com/blog/2011/11/15/documenting-architecture-decisions)

**Tools:**
- [adr-tools](https://github.com/npryce/adr-tools) - CLI for managing ADRs
- [log4brains](https://github.com/thomvaill/log4brains) - ADR visualization

**Books:**
- Fundamentals of Software Architecture (Richards, Ford)
- Building Evolutionary Architectures (Ford, Parsons, Kua)

---

*Template created based on Michael Nygard's practice and industry standards for documenting architectural decisions.*
