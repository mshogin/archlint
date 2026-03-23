# Spec 0015: MCP server for architecture analysis

**Metadata:**
- Priority: 0015 (Medium)
- Status: Done
- Created: 2026-03-19
- Updated: 2026-03-19
- Effort: M
- Parent Spec: -

---

## Overview

### Problem Statement

archlint works as a CLI tool: each run re-parses the entire project, builds a graph and outputs the result. This means:

1. **High latency** — each run requires full AST parsing of the entire project.
2. **No interactivity** — the editor/IDE does not get architecture warnings in real time.
3. **No AI assistant integration** — Claude Code cannot request architecture analysis without spawning a separate process.

### Solution Summary

Implement an MCP (Model Context Protocol) server (`archlint serve`) that:
- Parses the project on initialization and keeps the architecture graph in memory
- Watches the project directory for .go file changes (fsnotify) with 100ms debounce
- On file change: re-parses atomically, computes metrics, detects degradation
- Exposes 8 tools for architecture analysis via the MCP protocol
- Provides rich per-file metrics (coupling, SOLID, code smells, health score)
- Tracks degradation between metric snapshots
- Integrates directly with Claude Code via `claude mcp add archlint -- archlint serve`

### Success Metrics

- `go build ./...` passes
- `go test ./...` passes
- MCP server starts via `archlint serve`
- All 8 tools return correct results
- File watcher detects changes and updates state automatically
- Health scores and degradation reports are computed correctly

---

## Architecture

### New components

```
internal/mcp/
  server.go              - JSON-RPC 2.0 MCP server over stdio
  state.go               - Thread-safe in-memory graph storage with file node tracking
  tools.go               - Tool definitions and implementations (8 tools)
  metrics.go             - Rich per-file architecture metrics computation
  degradation.go         - Degradation detection (before/after health comparison)
  watcher.go             - File watching with fsnotify and debouncing
  server_test.go         - Server tests
  state_test.go          - State tests
  metrics_test.go        - Metrics computation tests
  degradation_test.go    - Degradation detection tests

internal/cli/
  serve.go               - CLI command "archlint serve"
```

### Transport

JSON-RPC 2.0 over stdio with Content-Length headers (standard MCP transport).

### Dependencies

- `github.com/fsnotify/fsnotify` — file system notifications for the watcher

### Thread safety

`sync.RWMutex` for graph access, as MCP handlers may be called concurrently. The watcher runs in a background goroutine and acquires write locks for state updates.

---

## Requirements

### R1: CLI command `archlint serve`

- New cobra subcommand: `archlint serve`
- Flags: `--log-file`
- Starts MCP server over stdio

### R2: Project initialization

- On `initialize` — parses project root via `GoAnalyzer.Analyze()`
- Stores graph and analyzer in memory
- Computes initial metrics baseline for all files
- Starts file watcher goroutine
- Returns MCP capabilities with tools support

### R3: File watching (fsnotify)

- Watches the project directory recursively for .go file changes
- Debounces changes (100ms) to avoid thrashing during saves
- On file change: re-parses atomically, computes metrics, checks degradation
- Sends `notifications/fileChanged` when degradation is detected
- Replaces per-tool-call `Reparse()` — watcher keeps state fresh

### R4: Rich per-file metrics

Per-file metrics computed on demand or after file changes:
- **Coupling**: Afferent (Ca), Efferent (Ce), Instability (I), Abstractness (A), Distance from Main Sequence (D)
- **SOLID**: SRP violations (>7 methods or >10 fields), DIP violations (concrete dependencies), ISP violations (>5 method interfaces)
- **Code Smells**: God classes (>15 methods / >20 fields / fan-out >10), Hub nodes (fan-in + fan-out > 15), Orphan nodes, Feature envy, Shotgun surgery
- **Structural**: Cyclic dependencies, Max call depth, Fan-in, Fan-out
- **Health Score**: 0-100 computed from violations with configurable penalties

### R5: Degradation detection

- Stores previous metrics snapshots per file
- On file change, compares new metrics to baseline
- Reports delta, new/fixed violations, and status (improved/stable/degraded/critical)
- Status thresholds: improved (delta > 5), stable (-5 to 5), degraded (-15 to -5), critical (< -15)

### R6: MCP Tools

- `analyze_file` — full file analysis (types, functions, methods, dependencies, violations)
- `analyze_change` — analysis of file change impact (affected nodes, edges, impact level, degradation report)
- `get_dependencies` — dependency graph for a file or package (what depends on what)
- `get_architecture` — full graph with optional package filtering
- `check_violations` — full violation detection including SOLID, god classes, code smells, coupling, cycles
- `get_callgraph` — call graph from an entry point with configurable depth
- `get_file_metrics` — rich per-file metrics with health score (0-100)
- `get_degradation_report` — before/after health comparison with new/fixed violations

---

## Acceptance Criteria

- [x] AC1: `go build ./...` passes
- [x] AC2: `go test ./...` passes
- [x] AC3: `archlint serve` starts MCP server
- [x] AC4: initialize parses project, computes baseline metrics, starts watcher
- [x] AC5: tools/list returns all 8 tools
- [x] AC6: analyze_file returns types, functions, dependencies
- [x] AC7: analyze_change returns affected nodes, impact, and degradation report
- [x] AC8: get_architecture returns graph (with filtering)
- [x] AC9: check_violations detects SOLID, god classes, coupling, cycles, smells
- [x] AC10: get_callgraph traverses call chain
- [x] AC11: get_file_metrics returns coupling, SOLID, smells, health score
- [x] AC12: get_degradation_report returns before/after comparison
- [x] AC13: File watcher detects .go changes and triggers reparse
- [x] AC14: Degradation notification sent on health drop
- [x] AC15: All new files are covered by tests

---

## Testing Strategy

### Unit tests

- `TestInitialize` — initialization with temp directory
- `TestToolsList` — all 8 tools returned
- `TestToolsCallAnalyzeFile` — file analysis with types, functions, methods
- `TestToolsCallAnalyzeChange` — change impact analysis
- `TestToolsCallGetArchitecture` — full graph retrieval
- `TestToolsCallCheckViolations` — violation detection (returns ViolationReport)
- `TestToolsCallGetDependencies` — dependency query
- `TestToolsCallGetCallgraph` — call graph traversal
- `TestToolsCallGetFileMetrics` — per-file metrics with health score
- `TestToolsCallGetDegradationReport` — degradation report
- `TestMethodNotFound` — handling of unsupported methods
- `TestPing` — MCP ping/pong
- `TestReadWriteMessage` — JSON-RPC read/write
- `TestStateInitialize` — state initialization
- `TestStateReparse` — incremental update
- `TestStateRootDir` — rootDir storage
- `TestStateGetGraphReturnsACopy` — graph copy safety
- `TestComputeFileMetricsBasic` — basic metric computation
- `TestComputeFileMetricsGodClass` — god class detection
- `TestComputeFileMetricsOrphanNodes` — orphan node detection
- `TestComputeAllFileMetrics` — multi-file metrics
- `TestHealthScoreComputation` — health score formula verification
- `TestHealthScoreFloor` — score floors at 0
- `TestDegradationDetectorBasic` — stable baseline check
- `TestDegradationDetectorDegraded` — degradation detection
- `TestDegradationDetectorImproved` — improvement detection
- `TestDegradationDetectorNoBaseline` — no-baseline handling
- `TestClassifyDelta` — delta classification thresholds
- `TestSetBaselines` — baseline storage

---

## Notes

### Design Decisions

**Why MCP instead of LSP:**
MCP (Model Context Protocol) is purpose-built for AI assistant integration. Unlike LSP which requires complex editor-specific lifecycle management (didOpen, didChange, didSave, didClose, etc.), MCP simply exposes tools that AI assistants can call directly. This eliminates all the editor-specific protocol overhead and makes integration with Claude Code trivial: `claude mcp add archlint -- archlint serve`.

**Graph update strategy:**
File watcher with fsnotify detects .go file changes and triggers a full atomic re-analyze. For typical Go projects (< 1000 files) this takes < 100ms. The watcher debounces at 100ms to avoid thrashing during rapid saves. When no watcher is running (e.g. in tests), a per-call Reparse() fallback maintains compatibility.

**Health score computation:**
A simple weighted penalty model starting at 100, with deductions for each violation type. The score floors at 0. This provides a single "how healthy is this file?" number that Claude Code hooks can use for gating.

**Degradation detection:**
Compares current metrics against a stored baseline. The baseline is set on initialization and updated after each file change. Status thresholds (improved/stable/degraded/critical) use the health score delta to classify the change severity.

### Claude Code Integration

```bash
# Add archlint as an MCP server
claude mcp add archlint -- archlint serve

# Now Claude Code can use archlint tools directly:
# - analyze_file: "Analyze the architecture of internal/service/order.go"
# - analyze_change: "What would be affected if I change this file?"
# - get_dependencies: "What does this package depend on?"
# - get_architecture: "Show me the architecture graph for internal/service"
# - check_violations: "Are there any circular dependencies or SOLID violations?"
# - get_callgraph: "Trace the call chain from ProcessOrder"
# - get_file_metrics: "How healthy is this file?"
# - get_degradation_report: "Did my changes make this file worse?"
```
