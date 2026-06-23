#!/usr/bin/env python3
"""Drive `archlint serve` (MCP, stdio) and print the violations an agent sees.

Each violation carries the explainability envelope: severity / principle /
remediation / human_in_loop. Exit code 1 if any ERROR-severity violation is
present, so this doubles as a gate inside an agent's edit loop.

Usage:
    python3 mcp_check.py .                 # whole project
    python3 mcp_check.py path/to/file.go   # one file/package
"""
import json
import re
import subprocess
import sys


def frame(obj):
    body = json.dumps(obj).encode()
    return f"Content-Length: {len(body)}\r\n\r\n".encode() + body


def main():
    path = sys.argv[1] if len(sys.argv) > 1 else "."
    msgs = [
        {"jsonrpc": "2.0", "id": 1, "method": "initialize",
         "params": {"protocolVersion": "2024-11-05", "capabilities": {},
                    "clientInfo": {"name": "agent-gate", "version": "1"}}},
        {"jsonrpc": "2.0", "method": "notifications/initialized"},
        {"jsonrpc": "2.0", "id": 2, "method": "tools/call",
         "params": {"name": "check_violations", "arguments": {"path": path}}},
    ]
    proc = subprocess.Popen(["archlint", "serve"], stdin=subprocess.PIPE,
                            stdout=subprocess.PIPE, stderr=subprocess.DEVNULL)
    out, _ = proc.communicate(b"".join(frame(m) for m in msgs), timeout=60)

    violations = []
    for part in re.split(r"Content-Length: \d+\r\n\r\n", out.decode(errors="replace")):
        if '"id":2' in part:
            payload = json.loads(json.loads(part.strip())["result"]["content"][0]["text"])
            violations = payload.get("violations") or []

    if not violations:
        print(f"OK: no violations in {path}")
        return 0

    errors = 0
    for v in violations:
        sev = v.get("severity", "?")
        flag = " [human-in-loop]" if v.get("human_in_loop") else ""
        print(f"[{sev}] {v['kind']} ({v.get('principle', '-')}){flag}")
        if v.get("remediation"):
            print(f"    fix: {v['remediation']}")
        if sev == "ERROR":
            errors += 1

    if errors:
        print(f"\nFAIL: {errors} ERROR-severity violation(s) — gate blocks.")
        return 1
    print(f"\nOK: {len(violations)} signal(s), no ERROR — gate passes.")
    return 0


if __name__ == "__main__":
    sys.exit(main())
