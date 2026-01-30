# Project Handover Process for Architecture Supervision

**EN** | [RU](project-handover-process.md)

## Roles

| Role                        | Responsibility                                                    |
|-----------------------------|-------------------------------------------------------------------|
| **Architect**               | Architecture supervision, audit, recommendations, quality control |
| **Project Manager (PM)**    | Business context, timelines, resources, stakeholders              |
| **Tech Lead**               | Technical implementation, team, code, infrastructure              |

---

## Handover Algorithm

```
[Initiation] -> [Information Gathering] -> [Audit] -> [Decision] -> [Onboarding]
      1                 2-3                    4          5             6
```

---

## Phase 1: Initiation (1 day)

### Entry Trigger
- Request from PM/management for project handover
- Formal assignment of architect as supervisor

### Actions

1. **Get briefing from PM** (30-60 min)
   - Business context and project goals
   - Current state and problems
   - Expectations from architecture supervision
   - Key stakeholders
   - Deadlines and constraints

2. **Request access**
   - Repositories (GitLab)
   - Documentation (Wiki/Confluence/Notion)
   - Monitoring (Grafana, logs)
   - Task tracker (Tracker/Jira/YouTrack)
   - Communications (Slack/Teams channels)

3. **Schedule kickoff meeting** with PM + Tech Lead

### Artifacts
- [ ] Project card filled (name, PM, Tech Lead, contacts)
- [ ] Access granted
- [ ] Kickoff meeting scheduled

---

## Phase 2: Kickoff Meeting (1-2 hours)

### Participants
- Architect
- Project Manager (PM)
- Tech Lead

### Agenda

**Block 1: Business Context (PM, 20 min)**
- What does the system do? For whom?
- Key business metrics (KPI)
- Development plans for next 3-6 months
- Main pain points and problems

**Block 2: Technical Picture (Tech Lead, 30 min)**
- Architecture: what does the system consist of?
- Main technologies and patterns
- Known problems and technical debt
- Team: who does what?

**Block 3: Expectations and Work Format (all, 20 min)**
- What is expected from the architect?
- What level of involvement is needed?
- How will we interact? (regular meetings, ad-hoc, async)
- Who makes final architecture decisions?

**Block 4: Agreements (10 min)**
- Audit timeline
- Report format
- Next meeting date

### Kickoff Questions

**PM:**
- What are the business goals for this quarter/year?
- What problems prevent achieving goals?
- Are there critical deadlines/releases?
- Who are the main system users?

**Tech Lead:**
- Can you show the architecture diagram? (or draw on whiteboard)
- Which services are most critical?
- Where is the most pain now?
- What would you change if you had time?
- How is the development process organized? (code review, CI/CD, tests)

### Artifacts
- [ ] Key meeting points recorded
- [ ] Architecture diagram obtained (or agreed who will create)
- [ ] Audit timeline defined
- [ ] Interaction format agreed

---

## Phase 3: Information Gathering (3-5 days)

### Information Sources

| Source       | What to Review              | Who Provides       |
|--------------|-----------------------------|-------------------|
| Documentation| Architecture, ADR, API specs| Tech Lead         |
| Code         | Structure, quality, tests   | Tech Lead         |
| Monitoring   | Metrics, alerts, incidents  | Tech Lead / DevOps|
| Task tracker | Backlog, tech debt, bugs    | PM / Tech Lead    |
| Stakeholders | Pain points, expectations   | PM                |

### Gathering Checklist

**Documentation:**
- [ ] Architecture documentation (C4, diagrams)
- [ ] ADR (Architecture Decision Records)
- [ ] API documentation
- [ ] Runbooks and instructions
- [ ] Developer onboarding guide

**Code and Infrastructure:**
- [ ] Repository list
- [ ] CI/CD pipelines
- [ ] Test coverage
- [ ] Linter results

**Operational Information:**
- [ ] Monitoring dashboards
- [ ] Incident history (last 3-6 months)
- [ ] SLA/SLO (if available)
- [ ] Load and performance metrics

### Async Questions for Tech Lead

```
Hi! Preparing for audit, need information:

1. Where is the architecture documentation?
2. Are there ADRs? Where?
3. Can you share the main monitoring dashboard link?
4. Which service/component is most problematic now?
5. Were there critical incidents in the last 3 months?

If something doesn't exist - just say so, that's also useful info.
```

---

## Phase 4: Audit (3-7 days)

### Audit Structure

**Day 1-2: Architecture Overview**
- Study diagrams and documentation
- Understand system structure (services, DBs, queues)
- Draw your own diagram if missing
- Identify bounded contexts

**Day 3-4: Deep Dive**
- Review code of key services
- Check test coverage
- Review CI/CD
- Study monitoring and alerts

**Day 5-7: Analysis and Report**
- Formulate problems
- Prioritize recommendations
- Fill system-audit.md template
- Prepare results presentation

### What to Look For

**Structural Issues:**
- Distributed monolith (tightly coupled services)
- Shared database
- Circular dependencies
- God services (doing too much)

**Operational Issues:**
- Missing monitoring
- No alerts or alert fatigue
- Missing runbooks
- No DR plan

**Quality Issues:**
- Low test coverage
- Missing code review
- No CI/CD
- Outdated dependencies

---

## Phase 5: Handover Decision (1 day)

### Audit Results Meeting

**Participants:** Architect, PM, Tech Lead

**Agenda (1 hour):**
1. Audit results presentation (20 min)
2. Critical issues discussion (20 min)
3. Improvement plan agreement (15 min)
4. Handover decision (5 min)

### Decision Options

| Decision                  | When                            | What's Next                              |
|---------------------------|---------------------------------|------------------------------------------|
| **Accepted**              | No critical blockers            | Move to onboarding                       |
| **Accepted with Conditions** | Issues exist but manageable  | Fix conditions and resolution deadlines  |
| **Not Accepted**          | Critical blockers               | Return for rework, repeat audit          |

### Handover Criteria (minimum)

- [ ] Architecture understanding exists (diagram)
- [ ] Code and monitoring access available
- [ ] Tech Lead or technical responsible person available
- [ ] No critical security issues
- [ ] System in production and working

### Artifacts
- [ ] Audit report (system-audit.md)
- [ ] Results presentation
- [ ] Agreed improvement plan
- [ ] Handover decision (recorded)

---

## Phase 6: Supervision Onboarding (ongoing)

### First Week After Handover

1. **Join Communications**
   - Slack/Teams project channels
   - Incident mailing lists
   - Invitations to key meetings

2. **Setup Regular Touchpoints**
   - With Tech Lead: weekly 30 min (technical)
   - With PM: bi-weekly 30 min (business + priorities)

3. **Define Control Points**
   - Code review of architecturally significant changes
   - Participation in design review of new features
   - ADR review

### Regular Interaction Format

**Weekly Sync with Tech Lead (30 min):**
- What's happening this week?
- What technical decisions are being made?
- Need help/review?
- Progress on improvement plan

**Bi-weekly with PM (30 min):**
- Business priorities
- New requirements and their impact on architecture
- Resources and timelines
- Escalations (if any)

### Escalation Points

| Situation                                      | Action                                  |
|------------------------------------------------|-----------------------------------------|
| Tech Lead disagrees with recommendation        | Escalate to PM, joint discussion        |
| Critical tech debt being ignored               | Escalate to PM + management             |
| Architecture decision contradicts strategy     | Record in ADR, escalate if needed       |

---

## Communication Templates

### Information Request (async)

```
Hi [Tech Lead]!

Took project [name] under architecture supervision.
Preparing for audit, need your help.

Can you share:
1. [Specific question 1]
2. [Specific question 2]
3. [Specific question 3]

If something doesn't exist - say so, that's also important to know.

Deadline: by [date], if possible.
Thanks!
```

### Kickoff Invitation

```
Subject: Kickoff: project handover [name]

Hi!

Assigned as architecture supervisor for project [name].
Proposing to meet for introduction and discussion.

Agenda:
1. Business context (PM)
2. Technical picture (Tech Lead)
3. Expectations and work format

Participants: [PM], [Tech Lead], [Architect]
Time: [date, time]
Duration: 1.5 hours

Please confirm attendance.
```

### Audit Results Delivery

```
Subject: Audit results for project [name]

Hi!

Completed audit of project [name].

Key findings:
- Overall score: [X]/5
- Critical issues: [X]
- Recommendations: [X]

Top-3 issues:
1. [Issue 1]
2. [Issue 2]
3. [Issue 3]

Full report: [link]

Proposing to meet [date] for discussion.
```

---

## Supervision Success Metrics

| Metric                     | How to Measure                  | Target     |
|----------------------------|---------------------------------|------------|
| Handover Time              | Days from request to decision   | < 2 weeks  |
| Critical Issues Closure    | % closed per quarter            | > 80%      |
| Tech Lead Satisfaction     | Survey once per quarter         | > 4/5      |
| Design Review Participation| % architecture decisions reviewed| > 90%      |
| Architecture Incidents     | Count per quarter               | Decreasing |

---

## Handover Checklist (summary)

### Phase 1: Initiation
- [ ] PM briefing received
- [ ] Access requested
- [ ] Kickoff meeting scheduled

### Phase 2: Kickoff
- [ ] Meeting with PM + Tech Lead conducted
- [ ] Business context understood
- [ ] Technical picture understood
- [ ] Work format agreed

### Phase 3: Information Gathering
- [ ] Documentation collected
- [ ] Code access obtained
- [ ] Monitoring access obtained
- [ ] Incident history reviewed

### Phase 4: Audit
- [ ] Architecture audit conducted
- [ ] system-audit.md filled
- [ ] Issues and recommendations formulated

### Phase 5: Decision
- [ ] Results presented
- [ ] Improvement plan agreed
- [ ] Handover decision made

### Phase 6: Onboarding
- [ ] Regular meetings configured
- [ ] Added to communications
- [ ] Control points defined
