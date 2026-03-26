#!/bin/bash
# Usage: ownership-map.sh /path/to/project
# Maps component ownership by analyzing git history per component.
# Output: JSON with primary owner, contributors, and orphaned components.

ARCHLINT=/home/assistant/projects/archlint-repo/bin/archlint
PROJECT=${1:-.}

if [ ! -d "$PROJECT" ]; then
    echo '{"error": "Project directory not found"}' >&2
    exit 1
fi

if [ ! -d "$PROJECT/.git" ]; then
    echo '{"error": "Not a git repository"}' >&2
    exit 1
fi

# Run archlint metrics to get list of components (packages)
METRICS=$("$ARCHLINT" metrics "$PROJECT" 2>/dev/null)

if [ -z "$METRICS" ]; then
    echo '{"error": "archlint metrics failed or returned no output"}' >&2
    exit 1
fi

# Pass metrics via environment variable to avoid shell word-splitting issues
export ARCHLINT_METRICS="$METRICS"
export ARCHLINT_PROJECT="$PROJECT"

python3 - <<'PYEOF'
import sys
import json
import subprocess
import os
import re
from datetime import datetime, timezone

metrics_text = os.environ["ARCHLINT_METRICS"]
project = os.environ["ARCHLINT_PROJECT"]

# Detect Go module name from go.mod
def get_module_name(project):
    gomod = os.path.join(project, "go.mod")
    if not os.path.isfile(gomod):
        return None
    try:
        with open(gomod) as f:
            for line in f:
                line = line.strip()
                if line.startswith("module "):
                    return line.split()[1]
    except Exception:
        pass
    return None

module_name = get_module_name(project)

# Parse package names from archlint metrics output.
# Package lines have no leading whitespace and contain a slash.
# When archlint is run with a full path it may prepend the module name,
# e.g. "promptlint/pkg/analyzer" instead of "pkg/analyzer".
package_pattern = re.compile(r'^([a-zA-Z0-9_\-\.]+(?:/[a-zA-Z0-9_\-\.]+)+)$')
packages = []
for line in metrics_text.splitlines():
    line = line.strip()
    m = package_pattern.match(line)
    if m and '=' not in line and ':' not in line:
        pkg = m.group(1)
        # Strip module name prefix if present.
        # archlint uses the base name of the module as prefix,
        # e.g. for "github.com/mikeshogin/promptlint" it uses "promptlint/".
        if module_name:
            # Try full module name first, then just the basename
            if pkg.startswith(module_name + "/"):
                pkg = pkg[len(module_name) + 1:]
            else:
                base = module_name.split("/")[-1]
                if pkg.startswith(base + "/"):
                    pkg = pkg[len(base) + 1:]
        # Verify the directory exists in project
        full_path = os.path.join(project, pkg)
        if os.path.isdir(full_path):
            packages.append(pkg)

def get_commit_authors(pkg_path, project, since=None):
    """Get author emails with commit counts for a package directory."""
    full_path = os.path.join(project, pkg_path)
    if not os.path.isdir(full_path):
        return {}
    cmd = ["git", "log", "--format=%ae", "--", full_path]
    if since:
        cmd = ["git", "log", "--format=%ae", f"--since={since}", "--", full_path]
    try:
        result = subprocess.run(
            cmd,
            capture_output=True, text=True, cwd=project, timeout=15
        )
        counts = {}
        for email in result.stdout.splitlines():
            email = email.strip()
            if email:
                counts[email] = counts.get(email, 0) + 1
        return counts
    except Exception:
        return {}

def get_last_commit_date(pkg_path, project):
    """Get date of the last commit touching this package."""
    full_path = os.path.join(project, pkg_path)
    if not os.path.isdir(full_path):
        return None
    try:
        result = subprocess.run(
            ["git", "log", "-1", "--format=%ci", "--", full_path],
            capture_output=True, text=True, cwd=project, timeout=10
        )
        date_str = result.stdout.strip()
        if date_str:
            return date_str[:10]  # YYYY-MM-DD
        return None
    except Exception:
        return None

ORPHAN_DAYS = 90
since_90_days = f"{ORPHAN_DAYS} days ago"

components = []
orphaned = []

for pkg in packages:
    # Recent commits (last 90 days) for primary owner
    recent_authors = get_commit_authors(pkg, project, since=since_90_days)
    # All-time commits for contributors list
    all_authors = get_commit_authors(pkg, project)
    total_commits = sum(all_authors.values())
    last_commit = get_last_commit_date(pkg, project)

    # Primary owner = most commits in last 90 days
    if recent_authors:
        primary_owner = max(recent_authors, key=lambda e: recent_authors[e])
    elif all_authors:
        # Fall back to all-time if no recent commits
        primary_owner = max(all_authors, key=lambda e: all_authors[e])
    else:
        primary_owner = None

    # Contributors = all unique authors sorted by commit count descending
    contributors = sorted(all_authors.keys(), key=lambda e: all_authors[e], reverse=True)

    entry = {
        "name": pkg,
        "primary_owner": primary_owner,
        "contributors": contributors,
        "total_commits": total_commits,
        "last_commit": last_commit
    }
    components.append(entry)

    # Mark as orphaned if no commits in last 90 days
    if not recent_authors:
        orphaned.append(pkg)

project_name = os.path.basename(os.path.abspath(project))

report = {
    "project": project_name,
    "generated_at": datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ"),
    "ownership_window_days": ORPHAN_DAYS,
    "components": components,
    "orphaned": orphaned
}

print(json.dumps(report, indent=2))
PYEOF
