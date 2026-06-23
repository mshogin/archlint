# Connecting archlint to an AI agent (MCP)

archlint ships an **MCP server** (Model Context Protocol over stdio) so an AI
coding agent can query architecture violations *while it works* — and get back
not just *what* is wrong, but *whether it blocks*, *how to fix it*, and *whether
it may auto-fix or must ask a human*.

This is the agent-facing path. For a plain quality gate (CI / pre-commit) see
[agent-gate.md](agent-gate.md).

---

## 1. Install

```bash
go install github.com/mshogin/archlint/cmd/archlint@latest
# or build from source: go build -o archlint ./cmd/archlint
```

## 2. Register the server with your agent

**Claude Code:**

```bash
claude mcp add archlint -- archlint serve
```

**Generic MCP client** — run the server over stdio:

```bash
archlint serve            # MCP server on stdin/stdout (JSON-RPC, Content-Length framed)
archlint serve --log-file /tmp/archlint-mcp.log   # optional logging
```

The server parses the project on initialization, keeps the architecture graph in
memory, and re-parses on each tool call to pick up file changes.

## 3. Tools the agent can call

| Tool | What it returns |
|------|-----------------|
| `check_violations` | architecture violations for a path/package (the gate) |
| `analyze_change` | impact of a file change on the architecture + violations |
| `get_dependencies` | dependency graph for a file or package |
| `get_architecture` | full architecture graph (or a filtered subset) |
| `analyze_file` | full file analysis (types, functions, dependencies) |
| `get_callgraph` | call graph from an entry point |

---

## 4. What the agent gets back — explainable verdict

Every violation from `check_violations` / `analyze_change` carries an
**explainability envelope** so the agent can act correctly:

| Field | Meaning for the agent |
|-------|-----------------------|
| `severity` | `ERROR` = blocks · `WARNING` / `INFO` = signal, not a block |
| `principle` | which architecture principle (SRP / DIP / ISP / layering / DRY / …) |
| `remediation` | concrete direction *how to fix* (guidance, not a proof) |
| `human_in_loop` | `true` → **do NOT auto-fix**, escalate to a human (e.g. dead-code: a wrong delete removes live code) |
| `requires_delta` | the ERROR blocks only as a *new* regression vs baseline, not by absolute count |
| `location` | `file:line` for the fix |

### Real example

A `check_violations` call on a package containing a god-class returns (abridged):

```json
{
  "violations": [
    {
      "kind": "god-class",
      "severity": "INFO",
      "principle": "coupling-cohesion",
      "remediation": "разбить god-class на меньшие типы по ответственностям; вынести группы методов/полей",
      "location": "god.go:4"
    },
    {
      "kind": "srp-lack-of-cohesion",
      "severity": "WARNING",
      "principle": "SRP",
      "remediation": "разделить ответственности (SRP): выделить независимые обязанности в отдельные типы/функции"
    }
  ]
}
```

For a blocking, human-gated case the envelope looks like:

```json
{
  "kind": "dead-code",
  "severity": "ERROR",
  "principle": "reachability",
  "human_in_loop": true,
  "requires_delta": true,
  "remediation": "★подтвердить с человеком (HumanInLoop, не авто): затем удалить неиспользуемый код ЛИБО подключить недостающую точку входа"
}
```

### How the agent should act

```text
for each violation v:
    if v.severity == "ERROR":
        if v.human_in_loop:  escalate to a human (do NOT auto-fix)
        else:                fix using v.remediation, then re-run check_violations
    else:                    treat as a signal (record, optionally improve)
```

ERROR + `human_in_loop` is the safety rail: destructive fixes (deleting
"dead" code that may be live, narrowing a still-used interface) require a human
decision; the agent surfaces `remediation` as the suggested direction but does
not act blindly.

---

## 5. Quick smoke test (no agent needed)

You can drive the server from a script to confirm it works. The server speaks
JSON-RPC with `Content-Length:` framing (LSP-style):

```python
import subprocess, json, re

def frame(obj):
    body = json.dumps(obj).encode()
    return f"Content-Length: {len(body)}\r\n\r\n".encode() + body

msgs = [
    {"jsonrpc": "2.0", "id": 1, "method": "initialize",
     "params": {"protocolVersion": "2024-11-05", "capabilities": {},
                "clientInfo": {"name": "smoke", "version": "1"}}},
    {"jsonrpc": "2.0", "method": "notifications/initialized"},
    {"jsonrpc": "2.0", "id": 2, "method": "tools/call",
     "params": {"name": "check_violations", "arguments": {"path": "god.go"}}},
]

p = subprocess.Popen(["archlint", "serve"], stdin=subprocess.PIPE,
                     stdout=subprocess.PIPE)
out, _ = p.communicate(b"".join(frame(m) for m in msgs), timeout=30)
# each reply is a Content-Length framed JSON-RPC message; the id=2 result
# contains the violations array with severity / remediation / human_in_loop.
print(out.decode(errors="replace"))
```

Run it from a directory with a Go project; you'll get the `initialize` reply
(serverInfo) followed by the `check_violations` result.
