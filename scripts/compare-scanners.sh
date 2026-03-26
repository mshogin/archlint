#!/usr/bin/env bash
# compare-scanners.sh - Compare Go and Rust scanner results on the same project
#
# Usage:
#   ./scripts/compare-scanners.sh [PROJECT_DIR]
#
# Default project: tests/testdata/sample
#
# Requires: python3
# Go scanner:   go run ./cmd/archlint/... (uses Go runtime)
# Rust scanner: archlint-rs/target/release/archlint

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

PROJECT_DIR="${1:-$REPO_ROOT/tests/testdata/sample}"
PROJECT_NAME="$(basename "$PROJECT_DIR")"

GO_BIN="${GO_BIN:-/home/assistant/bin/go/bin/go}"
RUST_BIN="${RUST_BIN:-$REPO_ROOT/archlint-rs/target/release/archlint}"

TMP_DIR="$(mktemp -d)"
trap 'rm -rf "$TMP_DIR"' EXIT

GO_COLLECT_OUT="$TMP_DIR/go_collect.yaml"
GO_CHECK_OUT="$TMP_DIR/go_check.json"
RUST_SCAN_OUT="$TMP_DIR/rust_scan.json"

# ---------------------------------------------------------------------------
# Run Go scanner
# ---------------------------------------------------------------------------
GO_AVAILABLE=false
GO_ERROR=""

if [ -x "$GO_BIN" ]; then
    if (cd "$REPO_ROOT" && "$GO_BIN" run ./cmd/archlint/... collect "$PROJECT_DIR" -l go -o "$GO_COLLECT_OUT" >/dev/null 2>&1); then
        if (cd "$REPO_ROOT" && "$GO_BIN" run ./cmd/archlint/... check "$PROJECT_DIR" --format json > "$GO_CHECK_OUT" 2>/dev/null); then
            GO_AVAILABLE=true
        else
            GO_ERROR="Go 'check' command failed"
        fi
    else
        GO_ERROR="Go 'collect' command failed"
    fi
else
    GO_ERROR="Go binary not found at $GO_BIN"
fi

# ---------------------------------------------------------------------------
# Run Rust scanner
# ---------------------------------------------------------------------------
RUST_AVAILABLE=false
RUST_ERROR=""

if [ -x "$RUST_BIN" ]; then
    if "$RUST_BIN" scan "$PROJECT_DIR" --format json > "$RUST_SCAN_OUT" 2>/dev/null; then
        RUST_AVAILABLE=true
    else
        RUST_ERROR="Rust scanner exited with error"
    fi
else
    RUST_ERROR="Rust binary not found at $RUST_BIN"
fi

# ---------------------------------------------------------------------------
# Compare and produce JSON report via Python
# ---------------------------------------------------------------------------
python3 - "$PROJECT_NAME" \
    "$GO_AVAILABLE" "$GO_ERROR" "$GO_COLLECT_OUT" "$GO_CHECK_OUT" \
    "$RUST_AVAILABLE" "$RUST_ERROR" "$RUST_SCAN_OUT" \
<<'PYEOF'
import sys, json, re

project_name   = sys.argv[1]
go_available   = sys.argv[2] == "true"
go_error       = sys.argv[3]
go_collect     = sys.argv[4]
go_check       = sys.argv[5]
rust_available = sys.argv[6] == "true"
rust_error     = sys.argv[7]
rust_scan      = sys.argv[8]

def parse_go(collect_path, check_path):
    """Parse Go YAML collect + JSON check into comparable counts."""
    # Count components from YAML (simple line-based, no dependency on pyyaml)
    components = 0
    links = 0
    in_links = False
    try:
        with open(collect_path) as f:
            for line in f:
                stripped = line.strip()
                if stripped == "links:":
                    in_links = True
                elif stripped == "components:":
                    in_links = False
                elif stripped.startswith("- id:"):
                    if not in_links:
                        components += 1
                elif stripped.startswith("- from:"):
                    links += 1
    except FileNotFoundError:
        pass

    violations = 0
    try:
        with open(check_path) as f:
            data = json.load(f)
            violations = data.get("total", 0) or 0
            if data.get("violations"):
                violations = len(data["violations"])
    except (FileNotFoundError, json.JSONDecodeError):
        pass

    return components, links, violations

def parse_rust(scan_path):
    """Parse Rust JSON scan output."""
    try:
        with open(scan_path) as f:
            data = json.load(f)
    except (FileNotFoundError, json.JSONDecodeError):
        return 0, 0, 0

    metrics = data.get("metrics", {})
    components = metrics.get("component_count", len(data.get("components", [])))
    links      = metrics.get("link_count",      len(data.get("links", [])))
    violations = len(metrics.get("violations", []))
    return components, links, violations

# Build scanner result objects
go_result = None
rust_result = None
differences = []

if go_available:
    gc, gl, gv = parse_go(go_collect, go_check)
    go_result = {"components": gc, "links": gl, "violations": gv}
else:
    go_result = {"error": go_error}

if rust_available:
    rc, rl, rv = parse_rust(rust_scan)
    rust_result = {"components": rc, "links": rl, "violations": rv}
else:
    rust_result = {"error": rust_error}

# Determine match
match = False
if go_available and rust_available:
    gc, gl, gv = go_result["components"], go_result["links"], go_result["violations"]
    rc, rl, rv = rust_result["components"], rust_result["links"], rust_result["violations"]
    if gc != rc:
        differences.append(f"component count differs: go={gc} rust={rc}")
    if gl != rl:
        differences.append(f"link count differs: go={gl} rust={rl}")
    if gv != rv:
        differences.append(f"violation count differs: go={gv} rust={rv}")
    match = len(differences) == 0
else:
    if not go_available:
        differences.append(f"Go scanner unavailable: {go_error}")
    if not rust_available:
        differences.append(f"Rust scanner unavailable: {rust_error}")

report = {
    "project": project_name,
    "go_scanner": go_result,
    "rust_scanner": rust_result,
    "match": match,
    "differences": differences,
}

print(json.dumps(report, indent=2))
PYEOF
