# Spec 0015: MCP server for architecture analysis

**Metadata:**
- Priority: 0015 (Medium)
- Status: Done
- Created: 2026-03-19
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
- Re-parses on each tool call to pick up file changes
- Exposes 6 tools for architecture analysis via the MCP protocol
- Integrates directly with Claude Code via `claude mcp add archlint -- archlint serve`

### Success Metrics

- `go build ./...` passes
- `go test ./...` passes
- MCP server starts via `archlint serve`
- All 6 tools return correct results

---

## Architecture

### New components

```
internal/mcp/
  server.go          - JSON-RPC 2.0 MCP server over stdio
  state.go           - Thread-safe in-memory graph storage
  tools.go           - Tool definitions and implementations
  server_test.go     - Server tests
  state_test.go      - State tests

internal/cli/
  serve.go           - CLI command "archlint serve"
```

### Transport

JSON-RPC 2.0 over stdio with Content-Length headers (standard MCP transport). No external dependencies.

### Thread safety

`sync.RWMutex` for graph access, as MCP handlers may be called concurrently.

---

## Requirements

### R1: CLI command `archlint serve`

- New cobra subcommand: `archlint serve`
- Flags: `--log-file`
- Starts MCP server over stdio

### R2: Project initialization

- On `initialize` — parses project root via `GoAnalyzer.Analyze()`
- Stores graph and analyzer in memory
- Returns MCP capabilities with tools support

### R3: MCP Tools

- `analyze_file` — full file analysis (types, functions, methods, dependencies, violations)
- `analyze_change` — analysis of file change impact (affected nodes, edges, impact level)
- `get_dependencies` — dependency graph for a file or package (what depends on what)
- `get_architecture` — full graph with optional package filtering
- `check_violations` — circular dependency and high coupling detection
- `get_callgraph` — call graph from an entry point with configurable depth

---

## Acceptance Criteria

- [x] AC1: `go build ./...` passes
- [x] AC2: `go test ./...` passes
- [x] AC3: `archlint serve` starts MCP server
- [x] AC4: initialize parses project and returns MCP capabilities
- [x] AC5: tools/list returns all 6 tools
- [x] AC6: analyze_file returns types, functions, dependencies
- [x] AC7: analyze_change returns affected nodes and impact
- [x] AC8: get_architecture returns graph (with filtering)
- [x] AC9: check_violations detects circular deps and high coupling
- [x] AC10: get_callgraph traverses call chain
- [x] AC11: All new files are covered by tests

---

## Testing Strategy

### Unit tests

- `TestInitialize` — initialization with temp directory
- `TestToolsList` — all 6 tools returned
- `TestToolsCallAnalyzeFile` — file analysis with types, functions, methods
- `TestToolsCallAnalyzeChange` — change impact analysis
- `TestToolsCallGetArchitecture` — full graph retrieval
- `TestToolsCallCheckViolations` — violation detection
- `TestToolsCallGetDependencies` — dependency query
- `TestToolsCallGetCallgraph` — call graph traversal
- `TestMethodNotFound` — handling of unsupported methods
- `TestPing` — MCP ping/pong
- `TestReadWriteMessage` — JSON-RPC read/write
- `TestStateInitialize` — state initialization
- `TestStateReparse` — incremental update
- `TestStateRootDir` — rootDir storage
- `TestStateGetGraphReturnsACopy` — graph copy safety

---

## Notes

### Design Decisions

**Why MCP instead of LSP:**
MCP (Model Context Protocol) is purpose-built for AI assistant integration. Unlike LSP which requires complex editor-specific lifecycle management (didOpen, didChange, didSave, didClose, etc.), MCP simply exposes tools that AI assistants can call directly. This eliminates all the editor-specific protocol overhead and makes integration with Claude Code trivial: `claude mcp add archlint -- archlint serve`.

**Graph update strategy:**
Full rebuild on each tool call (instead of file watching with incremental updates). For typical Go projects (< 1000 files) this takes < 100ms, which is fast enough for interactive use. This is significantly simpler than file watching and guarantees correctness.

### Claude Code Integration

```bash
# Add archlint as an MCP server
claude mcp add archlint -- archlint serve

# Now Claude Code can use archlint tools directly:
# - analyze_file: "Analyze the architecture of internal/service/order.go"
# - analyze_change: "What would be affected if I change this file?"
# - get_dependencies: "What does this package depend on?"
# - get_architecture: "Show me the architecture graph for internal/service"
# - check_violations: "Are there any circular dependencies?"
# - get_callgraph: "Trace the call chain from ProcessOrder"
```
