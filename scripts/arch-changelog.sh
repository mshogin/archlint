#!/bin/bash
# Usage:
#   arch-changelog.sh save /path/to/project   - save current scan as baseline
#   arch-changelog.sh diff /path/to/project   - compare current vs baseline
#   arch-changelog.sh show /path/to/project   - show baseline

ARCHLINT=/home/assistant/projects/archlint-repo/archlint-rs/target/release/archlint
ACTION=${1:-diff}
PROJECT=${2:-.}
NAME=$(basename "$PROJECT")
BASELINE_DIR="$HOME/.archlint-baselines"
BASELINE="$BASELINE_DIR/$NAME.json"

mkdir -p "$BASELINE_DIR"

case "$ACTION" in
  save)
    $ARCHLINT scan "$PROJECT" --format json > "$BASELINE" 2>/dev/null
    echo "{\"status\": \"saved\", \"project\": \"$NAME\", \"path\": \"$BASELINE\"}"
    ;;

  show)
    if [ -f "$BASELINE" ]; then
      cat "$BASELINE"
    else
      echo '{"error": "no baseline"}'
    fi
    ;;

  diff)
    if [ ! -f "$BASELINE" ]; then
      echo "{\"error\": \"no baseline, run: arch-changelog.sh save $PROJECT first\"}"
      exit 1
    fi

    CURRENT=$($ARCHLINT scan "$PROJECT" --format json 2>/dev/null)

    python3 - <<PYEOF "$CURRENT" "$BASELINE"
import sys, json, datetime

current = json.loads(sys.argv[1])
with open(sys.argv[2]) as f:
    baseline = json.load(f)

def extract(scan):
    components = {c['id'] for c in scan.get('components', [])}
    metrics = scan.get('metrics', {})
    violations = {
        (v['rule'], v['component'])
        for v in metrics.get('violations', [])
    }
    cycles = {tuple(sorted(c)) for c in metrics.get('cycles', [])}
    health = 100 - len(metrics.get('violations', [])) * 5 - len(metrics.get('cycles', [])) * 10
    return components, violations, cycles, health, metrics

cur_comps, cur_viols, cur_cycles, cur_health, cur_metrics = extract(current)
base_comps, base_viols, base_cycles, base_health, base_metrics = extract(baseline)

added_components   = sorted(cur_comps - base_comps)
removed_components = sorted(base_comps - cur_comps)
new_violations     = sorted(
    ({'rule': r, 'component': c} for r, c in (cur_viols - base_viols)),
    key=lambda x: (x['rule'], x['component'])
)
fixed_violations   = sorted(
    ({'rule': r, 'component': c} for r, c in (base_viols - cur_viols)),
    key=lambda x: (x['rule'], x['component'])
)
new_cycles    = sorted(list(c) for c in (cur_cycles - base_cycles))
fixed_cycles  = sorted(list(c) for c in (base_cycles - cur_cycles))
health_change = cur_health - base_health

changelog = {
    'generated_at': datetime.datetime.now(datetime.timezone.utc).isoformat().replace('+00:00', 'Z'),
    'project': '$NAME',
    'health_change': health_change,
    'health_before': base_health,
    'health_after': cur_health,
    'components': {
        'before': len(base_comps),
        'after': len(cur_comps),
        'added': added_components,
        'removed': removed_components,
    },
    'violations': {
        'before': len(base_viols),
        'after': len(cur_viols),
        'new': new_violations,
        'fixed': fixed_violations,
    },
    'cycles': {
        'before': len(base_cycles),
        'after': len(cur_cycles),
        'new': new_cycles,
        'fixed': fixed_cycles,
    },
    'summary': (
        f"health {base_health:+d} -> {cur_health} ({health_change:+d}), "
        f"+{len(added_components)}/-{len(removed_components)} components, "
        f"+{len(new_violations)}/-{len(fixed_violations)} violations, "
        f"+{len(new_cycles)}/-{len(fixed_cycles)} cycles"
    ),
}

print(json.dumps(changelog, indent=2))
PYEOF
    ;;

  *)
    echo "Usage: arch-changelog.sh <save|diff|show> /path/to/project" >&2
    exit 1
    ;;
esac
