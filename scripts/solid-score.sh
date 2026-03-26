#!/bin/bash
# Usage: solid-score.sh /path/to/project
# Grades each component on SOLID principles (A-F scale)
#
# Scoring per principle (20 points max each, total 100):
#   S - Single Responsibility: penalize srp-too-many-methods / srp-too-many-fields
#   O - Open/Closed:           neutral (hard to detect statically, benefit of doubt)
#   L - Liskov:                neutral
#   I - Interface Segregation: penalize isp-too-many-methods violations
#   D - Dependency Inversion:  penalize dip-concrete-dependency violations
#
# Grade: A (90+), B (80+), C (70+), D (60+), F (<60)

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ARCHLINT="${SCRIPT_DIR}/../bin/archlint"

# Fallback: look for archlint in PATH
if [ ! -x "$ARCHLINT" ]; then
    ARCHLINT="$(command -v archlint 2>/dev/null)"
fi

if [ -z "$ARCHLINT" ] || [ ! -x "$ARCHLINT" ]; then
    echo '{"error": "archlint binary not found"}' >&2
    exit 1
fi

PROJECT="${1:-.}"
NAME="$(basename "$(realpath "$PROJECT")")"

TMPFILE=$(mktemp /tmp/archlint-check-XXXXXX.json)
trap 'rm -f "$TMPFILE"' EXIT

"$ARCHLINT" check "$PROJECT" --format json 2>/dev/null > "$TMPFILE"

python3 - "$TMPFILE" "$NAME" <<'PYEOF'
import json, sys
from collections import defaultdict

with open(sys.argv[1]) as f:
    check_data = json.load(f)

project_name = sys.argv[2]
violations = check_data.get('violations', [])

# Group violations by component (target field).
# For method-level targets like "pkg/foo.Type.Method" -> component is "pkg/foo.Type"
# For type-level targets like "pkg/foo.Type" -> component is "pkg/foo.Type"
# For package-level targets like "internal/cli" -> component is "internal/cli"

def extract_component(target):
    """Normalize target to component level (strip method suffix)."""
    parts = target.split('.')
    # pkg/foo.Type.Method -> 3 parts after splitting on '.'
    # But path separators in first segment complicate things.
    # Strategy: split on '.' and if last segment looks like a method (starts with lowercase),
    # drop it to get the type-level component.
    if len(parts) >= 3:
        # e.g. "pkg/router.Router.routeFromAnalysis"
        # last part is method name - drop it
        return '.'.join(parts[:-1])
    return target

# Collect per-component violation counts by SOLID category
# Mapping violation kinds to SOLID principles:
#   S: srp-too-many-methods, srp-too-many-fields, god-class, feature-envy
#   O: (none mapped - benefit of doubt)
#   L: (none mapped - benefit of doubt)
#   I: isp-too-many-methods (if present)
#   D: dip-concrete-dependency

SOLID_MAP = {
    'srp-too-many-methods': 'S',
    'srp-too-many-fields':  'S',
    'god-class':            'S',
    'feature-envy':         'S',
    'isp-too-many-methods': 'I',
    'dip-concrete-dependency': 'D',
}

# Per-component violation counts per principle
comp_violations = defaultdict(lambda: defaultdict(int))

for v in violations:
    kind = v.get('kind', '')
    target = v.get('target', '')
    principle = SOLID_MAP.get(kind)
    if principle and target:
        comp = extract_component(target)
        comp_violations[comp][principle] += 1

# Penalty per violation count (diminishing returns)
def penalty(count):
    """Return score deduction from 0..20 for a given violation count."""
    if count == 0:
        return 0
    elif count == 1:
        return 5
    elif count == 2:
        return 10
    elif count == 3:
        return 15
    else:
        return 20  # max penalty, cap at 20

def grade(score):
    if score >= 90:
        return 'A'
    elif score >= 80:
        return 'B'
    elif score >= 70:
        return 'C'
    elif score >= 60:
        return 'D'
    else:
        return 'F'

# If no components with violations found, report at least the project-level
if not comp_violations:
    result = {
        'project': project_name,
        'components': [],
        'average_score': 100,
        'average_grade': 'A',
        'note': 'No SOLID violations detected'
    }
    print(json.dumps(result, indent=2))
    sys.exit(0)

components_out = []
total_score = 0

for comp in sorted(comp_violations.keys()):
    viol = comp_violations[comp]

    s_score = 20 - penalty(viol.get('S', 0))
    o_score = 20  # neutral
    l_score = 20  # neutral
    i_score = 20 - penalty(viol.get('I', 0))
    d_score = 20 - penalty(viol.get('D', 0))

    total = s_score + o_score + l_score + i_score + d_score
    g = grade(total)
    total_score += total

    components_out.append({
        'name': comp,
        'score': total,
        'grade': g,
        'breakdown': {
            'S': s_score,
            'O': o_score,
            'L': l_score,
            'I': i_score,
            'D': d_score
        },
        'violations': {
            'S': viol.get('S', 0),
            'I': viol.get('I', 0),
            'D': viol.get('D', 0)
        }
    })

avg_score = round(total_score / len(components_out)) if components_out else 100

result = {
    'project': project_name,
    'components': components_out,
    'average_score': avg_score,
    'average_grade': grade(avg_score)
}

print(json.dumps(result, indent=2))
PYEOF
