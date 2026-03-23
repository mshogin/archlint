# archlint - Claude Code Rules

## Communication

- Main bot-to-bot channel: [Issue #3](https://github.com/mshogin/archlint/issues/3)
- Always ping issue #3 when you need action from another contributor
- Use prefixes: `[archi]`, `[kostyaai]`, or `[your_name]`
- Every message should end with a concrete action request

## Pull Requests

- Review ALL open PRs - don't wait for someone to ask
- SLA: first review response within 24 hours
- When creating a PR, add a comment to issue #3 asking for review
- When reviewing, check: code correctness, architecture alignment, test coverage
- End review with clear verdict: "merge", "needs fix X", or "discuss"

## Code Conventions

- Language: Go
- Architecture layers: cmd -> cli -> mcp -> analyzer -> model
- Interfaces: max 5 methods, segregated by responsibility
- Fan-out: max 5 dependencies per component
- Tests required for new functionality
- SOLID principles enforced by archlint itself

## Task Management

- See [TASKS.md](TASKS.md) for current work items and status
- Don't block on waiting for merge approvals - work on other tasks in parallel
- Update TASKS.md when task status changes

## Periodic Tasks

- Check all open PRs for pending reviews
- Check issue #3 for action items
- Check other issues for mentions or questions
- Check TASKS.md for unblocked work

## Contributors

See [CONTRIBUTORS.md](CONTRIBUTORS.md). Only collaborators get PR reviews.
New contributors: create issue "Join request: @bot_name" to start.
