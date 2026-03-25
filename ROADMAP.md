# Roadmap

## Done
- [x] fix: double split in metrics.go:362 (issue #9)
- [x] fix: resource leak - log file handle not closed in server.go (issue #8)
- [x] test: collaborator access (issue #2)

## In Progress
- [ ] feat: rewrite archlint in Rust - safety, parallelism, performance (issue #17)
- [ ] feat: archlint-rs phase 1 - core graph engine in Rust (issue #18)
- [ ] plan: migration to Rust monolith - all linters + orchestrator (issue #19)
- [ ] feat: GitHub Actions CI pipeline (issue #11)
- [ ] feat: StateReader/MetricsProvider interfaces - DIP refactoring (issue #10)
- [ ] feat: replace DFS cycle detection with Tarjan's SCC algorithm (issue #5)

## Planned
- [ ] feat: Rust project support - parse Cargo.toml and Rust AST (issue #16)
- [ ] test: compare Go and Rust scanner results - must match on same project (issue #28)
- [ ] feat: auto-detect project language - go.mod/Cargo.toml/package.json (issue #29)
- [ ] feat: SOLID score per component - A-F grade (issue #30)
- [ ] feat: automated PR architecture review + branch-based workflow (issue #31)
- [ ] feat: architecture health badge for README - SVG (issue #24)
- [ ] feat: pre-compiled binaries in GitHub Releases + make init (issue #21)
- [ ] feat: archlint self-scan results dashboard (issue #12)
- [ ] feat: myhome integration - quality gate plugin (issue #14)
- [ ] feat: costlint integration - escalation cost tracking (issue #15)
- [ ] feat: prompt telemetry and architecture analysis for routing (issue #13)
- [ ] feat: architecture weekly digest - automated report with image to Telegram (issue #43)
- [ ] feat: publish archlint as GitHub Action in Marketplace (issue #44)
- [ ] feat: architecture diff between commits - show structural impact of changes (issue #23)
- [ ] feat: architecture snapshot timeline - track evolution over commits (issue #27)
- [ ] feat: architecture compliance report - PDF/HTML with SVG graphs (issue #36)

## Future Ideas
- [ ] feat: scan repo by URL in Telegram bot - send link, get architecture report (issue #42)
- [ ] feat: architecture anomaly detection - statistical outliers per project (issue #41)
- [ ] feat: API surface analysis - track exported functions, types, endpoints (issue #40)
- [ ] feat: architecture risk score per PR author - smart review routing (issue #39)
- [ ] feat: component ownership map via git blame (issue #38)
- [ ] feat: architecture test assertions library - ArchUnit for Go/Rust (issue #37)
- [ ] analysis: plugin architecture for shared features across all linters (issue #35)
- [ ] feat: architecture changelog - unified format for all tools in ecosystem (issue #34)
- [ ] feat: TypeScript/JavaScript project support - parse imports, components, modules (issue #33)
- [ ] feat: architecture complexity budget - visual remaining capacity (issue #32)
- [ ] feat: dependency age map - visualize stability via git blame (issue #26)
- [ ] feat: multi-language architecture graph - cross-language edges via OpenAPI/protobuf (issue #25)
- [ ] feat: performance metrics - hot path detection, complexity estimation (issue #22)
- [ ] feat: auto-fix worker - find violations and fix them automatically (issue #20)
- [ ] Bot-to-bot communication channel (issue #3)
