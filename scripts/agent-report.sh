#!/bin/bash
# Usage: agent-report.sh /path/to/project
# Runs archlint scan and produces agent-readable report with action items

ARCHLINT=/home/assistant/projects/archlint-repo/archlint-rs/target/release/archlint
PROJECT=${1:-.}
CATALOG=/home/assistant/projects/archlint-repo/docs/smell-catalog.json

SCAN=$($ARCHLINT scan "$PROJECT" --format json 2>/dev/null)

# Use python3 to enrich scan with action items from smell catalog
python3 - <<'PYEOF' "$SCAN" "$CATALOG"
import sys, json

scan = json.loads(sys.argv[1])
with open(sys.argv[2]) as f:
    catalog = json.load(f)

catalog_map = {s['id']: s for s in catalog}
metrics = scan.get('metrics', {})
violations = metrics.get('violations', [])

action_items = []
for v in violations:
    rule = v.get('rule', '')
    component = v.get('component', '')
    severity = v.get('severity', 'warning')
    smell = catalog_map.get(rule, {})
    action_items.append({
        'component': component,
        'smell': rule,
        'severity': severity,
        'fix_strategy': smell.get('fix_strategy', 'review manually'),
        'principle': smell.get('principle', ''),
        'message': v.get('message', '')
    })

report = {
    'components': len(scan.get('components', [])),
    'violations': len(violations),
    'cycles': len(metrics.get('cycles', [])),
    'health_score': 100 - len(violations) * 5 - len(metrics.get('cycles', [])) * 10,
    'action_items': action_items,
    'summary': f"{len(violations)} violations, {len(metrics.get('cycles', []))} cycles"
}

print(json.dumps(report, indent=2))
PYEOF
