# Add an architecture gate to your workflow in ~5 minutes

archlint's quality gate blocks **new** ERROR-class architecture regressions
(cycles, layer violations, ISP, dead-code, …) without blocking existing
baseline debt. There are three entry points — pick the one that matches how the
code is written.

```bash
go install github.com/mshogin/archlint/cmd/archlint@latest
```

| Entry point | When to use | Blocks where |
|-------------|-------------|--------------|
| **pre-commit hook** | a human (or agent) commits locally | developer machine, before commit |
| **CI check** | pull requests / pushes | CI server, before merge |
| **MCP loop** | an AI agent edits code in a loop | inside the agent, before it even commits |

All three call the same engine, so the verdict is identical.

---

## 1. Pre-commit hook (local)

A repo-local gate that travels with the repo (not a global hook):

```bash
make setup-hooks    # sets core.hooksPath -> .githooks (committed pre-commit)
```

Every commit then runs `make gate` (~1–2s): a delta-ERROR check vs
`.archlint-baseline.json`. Existing debt does not block; a *new* cycle / layer
violation / dead-code does. Bypass in exceptional cases: `git commit --no-verify`.

To create/refresh the baseline (existing debt becomes the accepted floor):

```bash
archlint baseline . -o .archlint-baseline.json
```

Don't have `make`? The raw command is:

```bash
archlint scan . --threshold 0 --format text   # exit 1 if violations exceed threshold
```

## 2. CI check (pull request)

**GitHub Actions** — posts a summary to the PR and fails on threshold breach:

```yaml
- name: Architecture Review
  uses: mshogin/archlint@v1
  with:
    directory: '.'
    threshold: '0'
    comment: 'true'
```

**Any CI** — just run the binary; non-zero exit fails the job:

```bash
archlint scan . --threshold 0 --format json
```

## 3. MCP loop (AI agent)

The agent calls `check_violations` over MCP **before it commits**, reads the
`severity` / `remediation` / `human_in_loop` envelope, fixes ERROR-class issues
it may fix, and escalates the human-in-loop ones. Setup and the response format
are in [mcp-setup.md](mcp-setup.md).

Minimal agent loop:

```text
1. agent edits files
2. agent calls check_violations (MCP)
3. for each ERROR violation:
     - human_in_loop=true -> ask the human
     - else                -> apply remediation, go to 2
4. no ERROR left -> commit
```

---

## Which one?

- **Solo / small repo:** pre-commit hook is enough.
- **Team:** pre-commit (fast feedback) **+** CI check (the enforced wall).
- **AI-assisted development:** add the MCP loop so the agent self-corrects
  *before* producing a commit — the gate becomes part of generation, not a
  post-hoc rejection.

A minimal runnable example lives in [`examples/agent-gate/`](../examples/agent-gate/).
