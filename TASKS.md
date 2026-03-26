# Tasks

Current work items and their status. Updated by contributor bots.

## Open PRs

| PR | Title | Author | Status | Blocker |
|----|-------|--------|--------|---------|
| #6 | Add AI agent collaboration + CONTRIBUTORS.md | archi | Approved by kostyaai, ready to merge | Needs merge by repo owner |
| #4 | feat: check/metrics CLI + MCP server | kgatilin/kostyaai | Approved by archi, ready to merge | Needs merge by repo owner |

## Open Issues

| Issue | Title | Assignee | Status |
|-------|-------|----------|--------|
| #5 | Tarjan SCC cycle detection | kostyaai | Ready to start (after PR #4 merge) |
| #3 | Bot-to-bot communication | all | Active channel |

## Backlog

| Task | Priority | Notes |
|------|----------|-------|
| ~~Resource leak server.go:57~~ | P2 | Fixed: added defer server.Close() in serve.go |
| ~~Double split metrics.go:362~~ | P3 | Fixed: replaced strings.Split with strings.LastIndex |
| StateReader/MetricsProvider interfaces | P2 | DIP refactoring, separate PR after #4 merge |
| ~~strings.Join cosmetic~~ | P4 | Fixed: replaced strings.Join+intToStr with fmt.Sprintf in metrics.go |
| Tarjan SCC (issue #5) | P1 | Next after PR #4 merge |

## Rules

- Don't block on waiting for merge. If a PR is waiting for owner approval, work on other tasks.
- Update this file when task status changes.
- Each task has a clear owner (assignee).
