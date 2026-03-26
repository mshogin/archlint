#!/bin/bash
# Usage: snapshot-timeline.sh /path/to/project [num_commits]
# Tracks architecture metrics over git commits using git worktree (non-destructive)
# Output: JSON timeline with metrics per commit

ARCHLINT=/home/assistant/projects/archlint-repo/archlint-rs/target/release/archlint
PROJECT=${1:-.}
COMMITS=${2:-10}

# Resolve to absolute path
PROJECT=$(cd "$PROJECT" && pwd)

if [ ! -d "$PROJECT/.git" ]; then
    echo '{"error": "not a git repository: '"$PROJECT"'"}' >&2
    exit 1
fi

if [ ! -x "$ARCHLINT" ]; then
    echo '{"error": "archlint binary not found: '"$ARCHLINT"'"}' >&2
    exit 1
fi

# Create temp directory for worktree
WORKTREE_BASE=$(mktemp -d)
WORKTREE="$WORKTREE_BASE/snapshot"

cleanup() {
    # Remove worktree cleanly
    cd "$PROJECT"
    git worktree remove --force "$WORKTREE" 2>/dev/null || true
    rm -rf "$WORKTREE_BASE" 2>/dev/null || true
}
trap cleanup EXIT

# Get last N commit hashes and metadata
HASHES=$(cd "$PROJECT" && git log --format='%H %ai %s' -n "$COMMITS" 2>/dev/null)

if [ -z "$HASHES" ]; then
    echo '{"error": "no commits found in repository"}' >&2
    exit 1
fi

# Add worktree at HEAD first (detached), we will move it per commit
cd "$PROJECT"
FIRST_HASH=$(echo "$HASHES" | head -1 | awk '{print $1}')
git worktree add --detach "$WORKTREE" "$FIRST_HASH" 2>/dev/null

if [ $? -ne 0 ]; then
    echo '{"error": "failed to create git worktree"}' >&2
    exit 1
fi

python3 - <<PYEOF "$PROJECT" "$WORKTREE" "$ARCHLINT" "$HASHES"
import sys, json, subprocess, os, datetime

project    = sys.argv[1]
worktree   = sys.argv[2]
archlint   = sys.argv[3]
hashes_raw = sys.argv[4]

timeline = []

for line in hashes_raw.strip().split('\n'):
    if not line.strip():
        continue
    parts = line.split(' ', 3)
    commit_hash = parts[0]
    # date is parts[1] (date) + parts[2] (time) + parts[3-n] would be timezone
    # git --format='%H %ai %s' gives: hash date+time+tz subject
    # Split more carefully
    tokens = line.split(' ', 4)
    commit_hash = tokens[0]
    commit_date = tokens[1] if len(tokens) > 1 else ''
    commit_time = tokens[2] if len(tokens) > 2 else ''
    commit_tz   = tokens[3] if len(tokens) > 3 else ''
    commit_subject = tokens[4] if len(tokens) > 4 else ''

    commit_datetime = f"{commit_date}T{commit_time}{commit_tz}"

    # Checkout this commit in worktree
    result = subprocess.run(
        ['git', 'checkout', '--detach', commit_hash],
        cwd=worktree,
        capture_output=True, text=True
    )
    if result.returncode != 0:
        timeline.append({
            'commit': commit_hash,
            'date': commit_datetime,
            'subject': commit_subject,
            'error': 'checkout failed: ' + result.stderr.strip()
        })
        continue

    # Run archlint scan
    scan_result = subprocess.run(
        [archlint, 'scan', worktree, '--format', 'json'],
        capture_output=True, text=True
    )

    if scan_result.returncode != 0 or not scan_result.stdout.strip():
        timeline.append({
            'commit': commit_hash,
            'date': commit_datetime,
            'subject': commit_subject,
            'error': 'scan failed: ' + (scan_result.stderr.strip() or 'empty output')
        })
        continue

    try:
        scan = json.loads(scan_result.stdout)
    except json.JSONDecodeError as e:
        timeline.append({
            'commit': commit_hash,
            'date': commit_datetime,
            'subject': commit_subject,
            'error': f'json parse error: {e}'
        })
        continue

    metrics  = scan.get('metrics', {})
    comps    = scan.get('components', [])
    viols    = metrics.get('violations', [])
    cycles   = metrics.get('cycles', [])

    num_violations = len(viols)
    num_cycles     = len(cycles)
    num_components = len(comps)
    health_score   = max(0, 100 - num_violations * 5 - num_cycles * 10)

    timeline.append({
        'commit':      commit_hash,
        'date':        commit_datetime,
        'subject':     commit_subject,
        'components':  num_components,
        'violations':  num_violations,
        'cycles':      num_cycles,
        'health_score': health_score
    })

output = {
    'project':   os.path.basename(project),
    'generated': datetime.datetime.now(datetime.timezone.utc).isoformat().replace('+00:00', 'Z'),
    'commits_analyzed': len(timeline),
    'timeline':  timeline
}

print(json.dumps(output, indent=2))
PYEOF
