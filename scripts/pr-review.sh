#!/bin/bash
# Usage: pr-review.sh /path/to/project [base_branch]
# Generates a PR review comment in markdown format suitable for GitHub PR comments.
# Uses agent-report.sh, complexity-budget.sh, and anomaly-detect.sh under the hood.
#
# Output: Markdown text for PR comment

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ARCHLINT=${ARCHLINT:-"$SCRIPT_DIR/../archlint-rs/target/release/archlint"}

PROJECT=${1:-.}
BASE=${2:-main}

if [[ ! -x "$ARCHLINT" ]]; then
    echo "Error: archlint binary not found at $ARCHLINT" >&2
    exit 1
fi

# Run all three analysis scripts
AGENT_REPORT=$(bash "$SCRIPT_DIR/agent-report.sh" "$PROJECT" 2>/dev/null)
BUDGET_REPORT=$(bash "$SCRIPT_DIR/complexity-budget.sh" "$PROJECT" 2>/dev/null)
ANOMALY_REPORT=$(bash "$SCRIPT_DIR/anomaly-detect.sh" "$PROJECT" 2>/dev/null)

# Generate markdown PR comment
python3 - <<'PYEOF' "$AGENT_REPORT" "$BUDGET_REPORT" "$ANOMALY_REPORT" "$BASE"
import sys
import json

agent_raw   = sys.argv[1]
budget_raw  = sys.argv[2]
anomaly_raw = sys.argv[3]
base_branch = sys.argv[4]

def safe_parse(raw, label):
    try:
        return json.loads(raw)
    except Exception as e:
        sys.stderr.write(f"Warning: failed to parse {label}: {e}\n")
        return {}

agent   = safe_parse(agent_raw,  "agent-report")
budget  = safe_parse(budget_raw, "complexity-budget")
anomaly = safe_parse(anomaly_raw,"anomaly-detect")

# Core metrics from agent-report
components   = agent.get("components", 0)
violations   = agent.get("violations", 0)
cycles       = agent.get("cycles", 0)
health_score = agent.get("health_score", 100)
if health_score < 0:
    health_score = 0

# Budget entries
b = budget.get("budget", {})

def budget_row(label, metric_key, default_budget):
    entry  = b.get(metric_key, {})
    used   = entry.get("used",   0)
    bgt    = entry.get("budget", default_budget)
    status = entry.get("status", "ok")
    return label, used, bgt, status

rows = [
    budget_row("Components", "components", 50),
    budget_row("Violations", "violations", 10),
    budget_row("Max fan-out", "fan_out",   8),
    budget_row("Cycles",     "cycles",     0),
]

# Recommendation
if health_score >= 70:
    recommendation = "MERGE"
elif health_score >= 50:
    recommendation = "REVIEW"
else:
    recommendation = "BLOCK"

# Action items from agent-report
action_items = agent.get("action_items", [])

# Anomalies supplement action items with structural outliers
anomalies = anomaly.get("anomalies", [])

lines = []
lines.append("## Architecture Review by archlint")
lines.append("")
lines.append(f"### Health Score: {health_score}/100 - {recommendation}")
lines.append("")

# Metrics table
lines.append("| Metric | Value | Budget | Status |")
lines.append("|--------|-------|--------|--------|")
for label, used, bgt, status in rows:
    lines.append(f"| {label} | {used} | {bgt} | {status} |")
lines.append("")

# Action items section
total_items = len(action_items) + len(anomalies)
lines.append(f"### Action Items ({total_items})")

if action_items or anomalies:
    lines.append("")
    for item in action_items:
        comp     = item.get("component", "unknown")
        smell    = item.get("smell", "")
        fix      = item.get("fix_strategy", "review manually")
        message  = item.get("message", "")
        sev      = item.get("severity", "warning")
        detail   = message if message else smell
        lines.append(f"- `{comp}` - {detail} ({sev}) -> {fix}")
    for a in anomalies:
        comp      = a.get("component", "unknown")
        metric    = a.get("metric", "")
        value     = a.get("value", 0)
        threshold = a.get("threshold", 0)
        sev       = a.get("severity", "medium")
        lines.append(f"- `{comp}` - {metric}: {value} exceeds threshold {threshold} ({sev}) -> extract or split component")
else:
    lines.append("")
    lines.append("No action items - architecture looks clean.")

lines.append("")
lines.append("### Recommendation")
lines.append("")
if recommendation == "MERGE":
    lines.append(f"MERGE - health score {health_score}/100 is above threshold (70)")
elif recommendation == "REVIEW":
    lines.append(f"REVIEW - health score {health_score}/100 is below threshold (70), address action items before merging")
else:
    lines.append(f"BLOCK - health score {health_score}/100 is critically low (below 50), significant refactoring required")
lines.append("")
lines.append(f"_Compared against base branch: `{base_branch}` | Powered by [archlint](https://github.com/mshogin/archlint)_")

print("\n".join(lines))
PYEOF
