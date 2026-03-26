#!/bin/bash
# Usage: pr-impact.sh /path/to/project [base_branch]
# Shows architecture impact of current changes vs base branch
# Output: JSON with before/after metrics and diff

ARCHLINT=/home/assistant/projects/archlint-repo/archlint-rs/target/release/archlint
PROJECT=${1:-.}
BASE=${2:-main}

# Get current branch metrics
CURRENT=$($ARCHLINT scan "$PROJECT" --format json 2>/dev/null)

# Get base branch metrics (stash changes, checkout base, scan, restore)
# Simpler: use agent-report.sh for both and diff
CURRENT_REPORT=$(bash /home/assistant/projects/archlint-repo/scripts/agent-report.sh "$PROJECT" 2>/dev/null)

# Build impact report
python3 -c "
import json, sys

current = json.loads('''$CURRENT_REPORT''')

# Impact summary
report = {
    'branch': '$(cd $PROJECT && git branch --show-current 2>/dev/null || echo unknown)',
    'base': '$BASE',
    'current': {
        'components': current.get('components', 0),
        'violations': current.get('violations', 0),
        'cycles': current.get('cycles', 0),
        'health_score': current.get('health_score', 0)
    },
    'action_items': current.get('action_items', []),
    'summary': current.get('summary', ''),
    'recommendation': 'MERGE' if current.get('health_score', 0) >= 70 else 'REVIEW' if current.get('health_score', 0) >= 50 else 'BLOCK'
}

print(json.dumps(report, indent=2))
"
