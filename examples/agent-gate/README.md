# examples/agent-gate

Minimal, runnable examples of putting an archlint architecture gate in front of
a commit. See [../../docs/agent-gate.md](../../docs/agent-gate.md) for the full
guide and [../../docs/mcp-setup.md](../../docs/mcp-setup.md) for the agent/MCP path.

Prerequisite:

```bash
go install github.com/mshogin/archlint/cmd/archlint@latest
```

## 1. Pre-commit hook (`pre-commit`)

A two-line gate that blocks a commit when violations exceed the threshold.

```bash
cp examples/agent-gate/pre-commit .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

Or, for a repo-local hook that travels with the repo, use `make setup-hooks`
(see docs/agent-gate.md).

## 2. MCP check before commit (`mcp_check.py`)

Drives `archlint serve` over MCP (stdio, Content-Length framing) and prints the
violations an agent would see — each with `severity` / `remediation` /
`human_in_loop`. This is the shape an AI agent consumes in its edit loop.

```bash
python3 examples/agent-gate/mcp_check.py .          # scan the whole project
python3 examples/agent-gate/mcp_check.py path/to/file.go
```

Exit code is `1` if any `ERROR`-severity violation is present — so the same
script doubles as a gate for an agent loop.
