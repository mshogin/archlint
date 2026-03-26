#!/bin/bash
# Usage: dependency-age.sh /path/to/project
# Analyzes dependency age via git history to classify stability of architecture links.
# Output: JSON with each link + age + stability classification.
#
# Stability:
#   stable  (>90 days): well-established dependency
#   recent  (30-90 days): newer but settling
#   fresh   (<30 days): very new, might change
#   unknown: can't determine (not a git repo, no history)

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

# Collect architecture into a temp file
TMPDIR_LOCAL=$(mktemp -d)
trap 'rm -rf "$TMPDIR_LOCAL"' EXIT

ARCH_FILE="$TMPDIR_LOCAL/architecture.yaml"

cd "$PROJECT" || exit 1
"$ARCHLINT" collect "$PROJECT" -o "$ARCH_FILE" 2>/dev/null

if [ ! -f "$ARCH_FILE" ]; then
    echo '{"error": "archlint collect failed"}' >&2
    exit 1
fi

python3 - <<PYEOF "$ARCH_FILE" "$PROJECT"
import sys
import json
import yaml
import subprocess
import os
import datetime

arch_file = sys.argv[1]
project_dir = sys.argv[2]

with open(arch_file) as f:
    data = yaml.safe_load(f)

links = data.get("links", [])
project_name = os.path.basename(project_dir.rstrip("/"))

# Filter to import and dependency links only (skip contains/calls/uses that are structural)
dep_links = [l for l in links if l.get("type") in ("import", "uses", "calls")]

TODAY = datetime.date.today()


def classify_age(age_days):
    if age_days is None:
        return "unknown"
    if age_days > 90:
        return "stable"
    if age_days >= 30:
        return "recent"
    return "fresh"


def strip_module_prefix(component_id):
    """Convert 'promptlint/pkg/abtest' to 'pkg/abtest'."""
    parts = component_id.split("/")
    if len(parts) > 1:
        return "/".join(parts[1:])
    return component_id


def to_import_path(to_id):
    """
    Convert architecture link target to an importable path.
    'promptlint/pkg/abtest' -> 'github.com/mikeshogin/promptlint/pkg/abtest'
    Already full path (starts with github.com/...) -> return as-is.
    """
    return to_id  # archlint already stores full import path in 'to' for import links


def find_files_in_package(project_dir, pkg_rel_path):
    """Find .go files in the package directory."""
    pkg_dir = os.path.join(project_dir, pkg_rel_path)
    if not os.path.isdir(pkg_dir):
        return []
    return [
        os.path.join(pkg_dir, f)
        for f in os.listdir(pkg_dir)
        if f.endswith(".go") and not f.endswith("_test.go")
    ]


def get_import_first_commit(project_dir, import_str):
    """
    Find the oldest commit that introduced the import string.
    Uses git log -S to find all commits touching the string, returns the oldest.
    """
    try:
        result = subprocess.run(
            ["git", "log", "--all", "-S", import_str,
             "--pretty=format:%H %ad", "--date=short"],
            capture_output=True, text=True, cwd=project_dir, timeout=15
        )
        lines = [l.strip() for l in result.stdout.strip().splitlines() if l.strip()]
        if not lines:
            return None, None
        # Last line = oldest commit (git log is newest-first)
        oldest = lines[-1]
        parts = oldest.split(" ", 1)
        if len(parts) == 2:
            return parts[0], parts[1]
        return None, None
    except Exception:
        return None, None


def days_since(date_str):
    """Calculate days since a date string like '2026-03-26'."""
    try:
        d = datetime.date.fromisoformat(date_str)
        return (TODAY - d).days
    except Exception:
        return None


results = []
summary = {"stable": 0, "recent": 0, "fresh": 0, "unknown": 0}

for link in dep_links:
    from_id = link.get("from", "")
    to_id = link.get("to", "")
    link_type = link.get("type", "")

    # Derive the source package path (relative to project root)
    from_rel = strip_module_prefix(from_id)

    # The import string to search for in git history
    # For import links: to_id is the full import path already
    import_str = to_id

    first_commit, first_date = get_import_first_commit(project_dir, import_str)

    age_days = days_since(first_date) if first_date else None
    stability = classify_age(age_days)
    summary[stability] += 1

    results.append({
        "from": from_id,
        "to": to_id,
        "type": link_type,
        "age_days": age_days,
        "stability": stability,
        "first_commit": first_commit,
        "first_date": first_date,
    })

# Sort: stable first, then recent, then fresh, then unknown; within group by age desc
stability_order = {"stable": 0, "recent": 1, "fresh": 2, "unknown": 3}
results.sort(key=lambda x: (stability_order[x["stability"]], -(x["age_days"] or -1)))

output = {
    "project": project_name,
    "generated_at": datetime.datetime.now(datetime.timezone.utc).isoformat().replace("+00:00", "Z"),
    "links": results,
    "summary": summary,
}

print(json.dumps(output, indent=2))
PYEOF
