# AI Agent Ecosystem

How all projects connect and work together.

---

## Projects

### archlint (github.com/mshogin/archlint)
Code architecture analysis. Structural graphs, SOLID violations, cycle detection.
Role: QUALITY GATE - ensures code produced by agents meets architecture standards.

### promptlint (github.com/mshogin/promptlint)
Prompt complexity scoring for model routing. No LLM required.
Role: ROUTER - analyzes prompts and decides which model (haiku/sonnet/opus) to use.

### costlint (github.com/mshogin/costlint)
Token cost analysis and optimization.
Role: COST OPTIMIZER - tracks spending, runs A/B tests, finds savings.

### myhome (github.com/kgatilin/myhome)
AI agent workspace orchestration. Daemon-based agent lifecycle management.
Role: ORCHESTRATOR - schedules tasks, manages agent fleet, workflow stages.

---

## Data Flow

```
User request
     |
     v
[1] promptlint: analyze complexity -> pick model tier
     |
     v
[2] myhome: launch agent in selected container profile
     |
     v
[3] Agent executes task (haiku/sonnet/opus)
     |
     v
[4] archlint: scan output for architecture violations
     |
     +-- PASS --> [5] costlint: log tokens + cost -> done
     |
     +-- FAIL --> [2] re-route to more powerful model
                       costlint: log escalation cost
```

---

## Integration Matrix

| From -> To | archlint | promptlint | costlint | myhome |
|------------|----------|------------|----------|--------|
| archlint | - | quality feedback for routing rules | escalation cost data | quality gate stage |
| promptlint | - | - | routing decisions for cost tracking | pre-route hook |
| costlint | cost of re-routes | routing accuracy metrics | - | budget limits |
| myhome | post-validate hook | pre-route hook | telemetry consumer | - |

---

## Specific Integrations

### promptlint -> costlint
- promptlint decides model, costlint tracks cost of that decision
- Feedback: costlint identifies cases where cheaper model would suffice
- Data: promptlint telemetry.jsonl -> costlint report

### archlint -> costlint
- archlint rejection causes escalation to expensive model
- costlint tracks escalation cost patterns
- Goal: reduce escalations by improving first-pass prompts

### promptlint -> archlint
- Both analyze artifacts (prompts vs code) using structural metrics
- Shared philosophy: extract structure, measure, decide
- Shared patterns: scoring functions, threshold-based decisions

### myhome -> all
- Orchestrates the pipeline: prompt analysis -> execution -> validation -> cost tracking
- Each linter is a stage in myhome workflow
- myhome provides: scheduling, agent lifecycle, workflow state

---

## Shared Infrastructure

- Language: Go
- Config: YAML
- Telemetry: JSONL
- Communication: CLI pipes, HTTP APIs
- Deployment: Docker containers

---

## GitHub Issues (cross-project)

### myhome
- #48 - plugin/hook API (for promptlint + archlint)
- #49 - routing by complexity scoring (promptlint integration)
- #50 - quality gate stage (archlint integration)

### archlint
- #13 - prompt telemetry and architecture analysis
- #14 - quality gate plugin for myhome

### promptlint
- ISSUES-MYHOME.md #1 - CLI exit codes for pipeline
- ISSUES-MYHOME.md #2 - batch analysis endpoint
- ISSUES-MYHOME.md #3 - model tier config
- ISSUES-MYHOME.md #4 - telemetry export

### costlint
- TODO: token counting accuracy (tiktoken vs approximation)
- TODO: A/B test framework
- TODO: real-time cost dashboard
- TODO: budget alerts

---

## Blocker

myhome orchestrates Claude Code from Go application - potentially violates Claude Code policy.
Options: Agent SDK migration, scheduling-only mode, or stay with current architecture.
See: ROADMAP-MYHOME-MIGRATION.md
