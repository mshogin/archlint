#!/bin/bash
# Usage: anomaly-detect.sh /path/to/project
# Detects components with anomalous metrics (statistical outliers)

ARCHLINT=/home/assistant/projects/archlint-repo/archlint-rs/target/release/archlint
PROJECT=${1:-.}

TMPFILE=$(mktemp /tmp/archlint-scan-XXXXXX.json)
trap 'rm -f "$TMPFILE"' EXIT

$ARCHLINT scan "$PROJECT" --format json 2>/dev/null > "$TMPFILE"

python3 - "$TMPFILE" "$PROJECT" <<'PYEOF'
import json, sys, statistics

with open(sys.argv[1]) as f:
    scan = json.load(f)

project = sys.argv[2]
components = scan.get('components', [])
links = scan.get('links', [])

# Count fan-out per component
fan_out = {}
for link in links:
    src = link.get('from', '')
    fan_out[src] = fan_out.get(src, 0) + 1

# Count fan-in per component
fan_in = {}
for link in links:
    dst = link.get('to', '')
    fan_in[dst] = fan_in.get(dst, 0) + 1

# Detect anomalies (> mean + 2*stdev)
anomalies = []

if fan_out:
    values = list(fan_out.values())
    if len(values) > 2:
        mean = statistics.mean(values)
        stdev = statistics.stdev(values)
        threshold = mean + 2 * stdev
        for comp, val in fan_out.items():
            if val > threshold:
                anomalies.append({
                    'component': comp,
                    'metric': 'fan_out',
                    'value': val,
                    'threshold': round(threshold, 1),
                    'severity': 'high' if val > mean + 3 * stdev else 'medium'
                })

if fan_in:
    values = list(fan_in.values())
    if len(values) > 2:
        mean = statistics.mean(values)
        stdev = statistics.stdev(values)
        threshold = mean + 2 * stdev
        for comp, val in fan_in.items():
            if val > threshold:
                anomalies.append({
                    'component': comp,
                    'metric': 'fan_in',
                    'value': val,
                    'threshold': round(threshold, 1),
                    'severity': 'high' if val > mean + 3 * stdev else 'medium'
                })

report = {
    'project': project,
    'total_components': len(components),
    'anomalies_found': len(anomalies),
    'anomalies': anomalies
}

print(json.dumps(report, indent=2))
PYEOF
