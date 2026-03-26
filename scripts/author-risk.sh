#!/bin/bash
# Usage: author-risk.sh /path/to/project
# Analyzes git blame to score authors by architectural impact.
# Output: JSON with per-author risk score for smart PR review routing.

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

# Run archlint check to get violations
SCAN=$("$ARCHLINT" check "$PROJECT" --format json 2>/dev/null)

if [ -z "$SCAN" ]; then
    echo '{"error": "archlint check failed or returned no output"}' >&2
    exit 1
fi

# Compute author risk scores using python3
python3 - <<PYEOF "$SCAN" "$PROJECT"
import sys
import json
import subprocess
import os

scan_json = sys.argv[1]
project = sys.argv[2]

try:
    scan = json.loads(scan_json)
except json.JSONDecodeError as e:
    print(json.dumps({"error": f"Failed to parse archlint output: {e}"}))
    sys.exit(1)

violations = scan.get("violations", [])

# Extract package path from target like "promptlint/pkg/abtest.ABTest"
# -> "pkg/abtest"
def target_to_pkg_path(target):
    # target format: "module/pkg/subpkg.TypeName" or "module/pkg/subpkg.TypeName.MethodName"
    # Strip the type/method name after the last dot following a slash
    parts = target.split("/")
    if not parts:
        return None
    # last part may be "subpkg.TypeName" or just "TypeName"
    last = parts[-1]
    # Remove the type name: everything after first dot
    pkg_name = last.split(".")[0]
    parts[-1] = pkg_name
    # Return relative path within project (drop module prefix = first part)
    if len(parts) > 1:
        return "/".join(parts[1:])
    return parts[0]

# Map violation targets to unique packages
violation_pkgs = {}
for v in violations:
    target = v.get("target", "")
    if not target:
        continue
    pkg_path = target_to_pkg_path(target)
    if pkg_path:
        violation_pkgs[pkg_path] = violation_pkgs.get(pkg_path, 0) + 1

# For each violating package, find contributors via git log
author_violations = {}   # author -> count of violations they contributed to
author_components = {}   # author -> set of component paths touched

def get_pkg_authors(pkg_path, project):
    """Get unique author emails for all commits touching a package directory."""
    full_path = os.path.join(project, pkg_path)
    if not os.path.exists(full_path):
        # Try as a file (some packages may be single files)
        parent = os.path.dirname(full_path)
        if not os.path.exists(parent):
            return []
        full_path = parent
    try:
        result = subprocess.run(
            ["git", "log", "--format=%ae", "--", full_path],
            capture_output=True, text=True, cwd=project, timeout=10
        )
        emails = [e.strip() for e in result.stdout.splitlines() if e.strip()]
        return list(set(emails))
    except Exception:
        return []

def get_all_pkg_authors(pkg_path, project):
    """Get unique author emails for all commits touching any package in the project."""
    try:
        result = subprocess.run(
            ["git", "log", "--format=%ae"],
            capture_output=True, text=True, cwd=project, timeout=10
        )
        emails = [e.strip() for e in result.stdout.splitlines() if e.strip()]
        return list(set(emails))
    except Exception:
        return []

# Collect all unique authors across the project
all_authors = get_all_pkg_authors(".", project)
for author in all_authors:
    if author not in author_components:
        author_components[author] = set()
    if author not in author_violations:
        author_violations[author] = 0

# Map violations to authors
for pkg_path, viol_count in violation_pkgs.items():
    authors = get_pkg_authors(pkg_path, project)
    for author in authors:
        author_violations[author] = author_violations.get(author, 0) + viol_count
        if author not in author_components:
            author_components[author] = set()
        author_components[author].add(pkg_path)

# Collect total components touched per author across all packages
# Walk all packages and assign to each author
try:
    result = subprocess.run(
        ["git", "log", "--format=%ae %H"],
        capture_output=True, text=True, cwd=project, timeout=15
    )
    commit_authors = {}
    for line in result.stdout.splitlines():
        parts = line.strip().split(" ", 1)
        if len(parts) == 2:
            commit_authors[parts[1]] = parts[0]
except Exception:
    commit_authors = {}

# Get changed files per commit to map to packages
all_component_authors = {}  # pkg -> set of authors
try:
    result = subprocess.run(
        ["git", "log", "--name-only", "--format=%ae"],
        capture_output=True, text=True, cwd=project, timeout=30
    )
    current_author = None
    for line in result.stdout.splitlines():
        line = line.strip()
        if not line:
            continue
        if "@" in line and "/" not in line:
            current_author = line
        elif current_author and line.endswith(".go"):
            # Map file to package directory
            pkg = os.path.dirname(line)
            if pkg not in all_component_authors:
                all_component_authors[pkg] = set()
            all_component_authors[pkg].add(current_author)
            author_components.setdefault(current_author, set()).add(pkg)
except Exception:
    pass

# Build result
result_authors = []
for author in sorted(author_violations.keys()):
    viols = author_violations[author]
    comps = len(author_components.get(author, set()))
    if comps == 0:
        comps = 1  # avoid division by zero
    risk_score = round(viols / comps, 3)
    result_authors.append({
        "author": author,
        "violations_authored": viols,
        "components_touched": comps,
        "risk_score": risk_score
    })

# Sort by risk_score descending
result_authors.sort(key=lambda x: x["risk_score"], reverse=True)

project_name = os.path.basename(os.path.abspath(project))

report = {
    "project": project_name,
    "total_violations": len(violations),
    "authors": result_authors,
    "recommendation": "Review PRs from high-risk authors more carefully"
}

print(json.dumps(report, indent=2))
PYEOF
