#!/bin/bash
# Usage: complexity-budget.sh /path/to/project [--max-components 50] [--max-violations 10] [--max-fanout 8] [--max-cycles 0]
# Tracks architecture complexity budget - how much "room" is left before architecture degrades.
#
# Environment variables (override defaults):
#   MAX_COMPONENTS, MAX_VIOLATIONS, MAX_FANOUT, MAX_CYCLES

PROJECT=${1:-.}
shift || true

# Parse optional flags
while [[ $# -gt 0 ]]; do
    case "$1" in
        --max-components) MAX_COMPONENTS="$2"; shift 2 ;;
        --max-violations) MAX_VIOLATIONS="$2"; shift 2 ;;
        --max-fanout)     MAX_FANOUT="$2";     shift 2 ;;
        --max-cycles)     MAX_CYCLES="$2";     shift 2 ;;
        *) shift ;;
    esac
done

MAX_COMPONENTS=${MAX_COMPONENTS:-50}
MAX_VIOLATIONS=${MAX_VIOLATIONS:-10}
MAX_FANOUT=${MAX_FANOUT:-8}
MAX_CYCLES=${MAX_CYCLES:-0}

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ARCHLINT=${ARCHLINT:-"$SCRIPT_DIR/../archlint-rs/target/release/archlint"}

if [[ ! -x "$ARCHLINT" ]]; then
    echo '{"error": "archlint binary not found at '"$ARCHLINT"'"}' >&2
    exit 1
fi

SCAN=$("$ARCHLINT" scan "$PROJECT" --format json 2>/dev/null)

if [[ -z "$SCAN" ]]; then
    echo '{"error": "archlint scan returned no output"}' >&2
    exit 1
fi

python3 - <<PYEOF "$SCAN" "$MAX_COMPONENTS" "$MAX_VIOLATIONS" "$MAX_FANOUT" "$MAX_CYCLES"
import sys
import json

scan_raw  = sys.argv[1]
max_components = int(sys.argv[2])
max_violations = int(sys.argv[3])
max_fanout     = int(sys.argv[4])
max_cycles     = int(sys.argv[5])

try:
    scan = json.loads(scan_raw)
except json.JSONDecodeError as e:
    print(json.dumps({"error": f"failed to parse archlint output: {e}"}))
    sys.exit(1)

metrics = scan.get('metrics', {})

used_components = metrics.get('component_count', len(scan.get('components', [])))
used_violations = len(metrics.get('violations', []))
used_fanout     = metrics.get('max_fan_out', 0)
used_cycles     = len(metrics.get('cycles', []))


def budget_entry(metric, used, budget):
    remaining = budget - used
    if budget > 0:
        pct_used = round(used / budget * 100, 1)
    else:
        # budget=0 means zero allowed; any usage exceeds
        pct_used = 100.0 if used > 0 else 0.0

    if pct_used >= 100:
        status = "exceeded"
    elif pct_used >= 80:
        status = "warning"
    else:
        status = "ok"

    return {
        "metric":     metric,
        "used":       used,
        "budget":     budget,
        "remaining":  remaining,
        "pct_used":   pct_used,
        "status":     status
    }


entries = [
    budget_entry("components", used_components, max_components),
    budget_entry("violations", used_violations, max_violations),
    budget_entry("fan_out",    used_fanout,     max_fanout),
    budget_entry("cycles",     used_cycles,     max_cycles),
]

# Overall status: worst of all metrics
overall = "ok"
for e in entries:
    if e["status"] == "exceeded":
        overall = "exceeded"
        break
    if e["status"] == "warning" and overall == "ok":
        overall = "warning"

report = {
    "project": "$PROJECT",
    "overall_status": overall,
    "budget": {e["metric"]: e for e in entries}
}

print(json.dumps(report, indent=2))
PYEOF
