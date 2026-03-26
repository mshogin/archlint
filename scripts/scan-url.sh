#!/bin/bash
# Usage: scan-url.sh https://github.com/owner/repo
# Clones repo to temp dir, scans with archlint, outputs agent report, cleans up

URL=$1
if [ -z "$URL" ]; then
    echo '{"error": "usage: scan-url.sh <github-url>"}'
    exit 1
fi

ARCHLINT=/home/assistant/projects/archlint-repo/archlint-rs/target/release/archlint
TMPDIR=$(mktemp -d)
REPO_NAME=$(basename "$URL" .git)

# Clone (shallow for speed)
git clone --depth 1 "$URL" "$TMPDIR/$REPO_NAME" 2>/dev/null

if [ ! -d "$TMPDIR/$REPO_NAME" ]; then
    echo '{"error": "failed to clone repository"}'
    rm -rf "$TMPDIR"
    exit 1
fi

# Run agent report
REPORT=$(bash /home/assistant/projects/archlint-repo/scripts/agent-report.sh "$TMPDIR/$REPO_NAME" 2>/dev/null)

# Clean up
rm -rf "$TMPDIR"

# Add repo info to report
python3 -c "
import json
report = json.loads('''$REPORT''')
report['repo_url'] = '$URL'
report['repo_name'] = '$REPO_NAME'
print(json.dumps(report, indent=2))
"
