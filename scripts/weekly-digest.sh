#!/bin/bash
# Usage: weekly-digest.sh /path/to/project [project_name]
# Generates weekly architecture digest as text report

ARCHLINT=/home/assistant/projects/archlint-repo/archlint-rs/target/release/archlint
PROJECT=${1:-.}
NAME=${2:-$(basename "$PROJECT")}

REPORT=$(bash /home/assistant/projects/archlint-repo/scripts/agent-report.sh "$PROJECT" 2>/dev/null)

python3 -c "
import json, datetime

report = json.loads('''$REPORT''')
date = datetime.date.today().isoformat()
week = datetime.date.today().isocalendar()[1]

digest = {
    'project': '$NAME',
    'date': date,
    'week': week,
    'health_score': report.get('health_score', 0),
    'components': report.get('components', 0),
    'violations': report.get('violations', 0),
    'cycles': report.get('cycles', 0),
    'action_items_count': len(report.get('action_items', [])),
    'top_issues': report.get('action_items', [])[:3],
    'text_summary': f'''Architecture Digest - {\"$NAME\"} - Week {week}
Health: {report.get('health_score', 0)}/100
Components: {report.get('components', 0)} | Violations: {report.get('violations', 0)} | Cycles: {report.get('cycles', 0)}
Top issues: {len(report.get('action_items', []))} action items'''
}

print(json.dumps(digest, indent=2))
"
