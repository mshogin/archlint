# Plugin Architecture Analysis

Analysis of shared patterns across the GenieArchi linter ecosystem.

## Shared Patterns

All 4 linters follow the same architecture:

### CLI Pattern
- cmd/toolname/main.go - command dispatch
- Stdin input, JSON output
- Commands: analyze/rate + serve (HTTP)

### Package Structure
- pkg/classifier or pkg/analyzer - core logic
- pkg/config - YAML config loading
- pkg/server - HTTP API (net/http)

### Integration Points
- All tools accept stdin text, output JSON
- All have HTTP endpoints
- Pipeline: echo text | tool command -> JSON

## Plugin Interface (Proposed)

```go
type Linter interface {
    Name() string
    Analyze(text string) (json.RawMessage, error)
    Score() int  // 0-100
}
```

Each tool implements Linter interface.
geniearchi CLI loads all linters and runs them in sequence.

## Cross-tool Dependencies

| Tool | Depends On | Depended By |
|------|-----------|-------------|
| seclint | - | pipeline (first filter) |
| promptlint | - | routing, costlint |
| costlint | promptlint (model routing) | budget tracking |
| archlint | - | CI, compliance |

## Recommendation

Phase 1: Keep tools separate, integrate via shell pipeline (current)
Phase 2: Go plugin interface for embedding
Phase 3: Single binary with subcommands
