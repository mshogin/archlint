# Features Index

All archlint features with CLI usage and status.

## Architecture Analysis

| Feature | CLI | Config | Issue | Status |
|---------|-----|--------|-------|--------|
| Go project scan | `archlint collect .` | architecture.yaml | - | done (Go) |
| Rust project scan | `archlint-rs scan .` | - | #16 | done (Rust) |
| Architecture rules | `.archlint.yaml` | dag_check, fan_out, modularity, centrality | - | done (Go) |
| Component graph | `archlint collect .` | architecture.yaml | - | done (Go) |

## archlint-rs (Rust monolith)

| Feature | CLI | Issue | Status |
|---------|-----|-------|--------|
| Architecture scan | `archlint scan <dir>` | #18 | done |
| Prompt scoring | `archlint prompt [--model-only]` | #19 | done |
| Cost estimation | `archlint cost [--compare]` | #19 | done |
| Content rating | `archlint rate [--max-rating N]` | #19 | done |
| Performance metrics | `archlint perf <dir>` | #22 | done |
| Architecture diff | `archlint diff FROM..TO` | #23 | done |
| Docker workers | `archlint worker create/list/stop` | #19 | done |
| HTTP API server | `archlint serve [--port 8080]` | #19 | done |

## Configuration

| File | Purpose |
|------|---------|
| `.archlint.yaml` | Architecture rules: dag_check, max_fan_out, modularity, betweenness_centrality |
| `architecture.yaml` | Component definitions (pre-defined graph) |
| `budget.yaml` | Complexity budget limits (planned, #32) |

## Planned Features

| Feature | Issue | Priority |
|---------|-------|----------|
| Auto-detect language | #29 | P1 |
| SOLID score per component | #30 | P1 |
| Automated PR review | #31 | P1 |
| Complexity budget | #32 | P2 |
| Architecture badge SVG | #24 | P2 |
| Multi-language graph | #25 | P2 |
| Dependency age map | #26 | P2 |
| Snapshot timeline | #27 | P2 |
| Scanner comparison (Go vs Rust) | #28 | P1 |
| Auto-fix worker | #20 | P2 |
| GitHub Releases | #21 | done |
| Rust project support | #16 | done |
| Rust rewrite | #17 | phases 1-6 done |
| Performance metrics (archlint) | #22 | done |
| Quality gate for myhome | #14 | P2 |
| Costlint escalation tracking | #15 | P3 |

## HTTP API Endpoints (archlint serve)

| Method | Path | Description |
|--------|------|-------------|
| POST | /scan | Architecture scan (JSON body: {"dir": "/path"}) |
| POST | /analyze | Prompt scoring (text body) |
| POST | /rate | Content rating (text body) |
| POST | /cost | Cost estimation (text body, ?model=) |
| POST | /perf | Performance analysis (JSON body: {"dir": "/path"}) |
| GET | /health | Health check |
