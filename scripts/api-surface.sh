#!/bin/bash
# Usage: api-surface.sh <project-dir>
# Analyzes the public API surface of a Go project.
# Scans exported symbols (functions, types, interfaces, constants/vars) grouped by package.

PROJECT_DIR=$1
if [ -z "$PROJECT_DIR" ]; then
    echo '{"error": "usage: api-surface.sh <project-dir>"}'
    exit 1
fi

if [ ! -d "$PROJECT_DIR" ]; then
    echo "{\"error\": \"directory not found: $PROJECT_DIR\"}"
    exit 1
fi

THRESHOLD=20

# Detect project name from go.mod
PROJECT_NAME=$(grep -m1 "^module " "$PROJECT_DIR/go.mod" 2>/dev/null | awk '{print $2}' | xargs basename 2>/dev/null)
if [ -z "$PROJECT_NAME" ]; then
    PROJECT_NAME=$(basename "$PROJECT_DIR")
fi

total_exports=0
packages_json=""
warnings_json=""
first_pkg=1
first_warn=1

# Find all packages (unique directories containing .go files)
while IFS= read -r pkg_dir; do
    pkg_name=$(basename "$pkg_dir")

    # Count exported functions: lines starting with "func [A-Z]"
    funcs=$(grep -rh "^func [A-Z]" "$pkg_dir"/*.go 2>/dev/null | wc -l | tr -d ' ')

    # Count exported types (excluding interfaces counted separately)
    types=$(grep -rh "^type [A-Z]" "$pkg_dir"/*.go 2>/dev/null | grep -v "interface" | wc -l | tr -d ' ')

    # Count exported interfaces
    ifaces=$(grep -rh "^type [A-Z].*interface" "$pkg_dir"/*.go 2>/dev/null | wc -l | tr -d ' ')

    # Count exported vars and consts
    consts=$(grep -rh "^\(var\|const\) [A-Z]" "$pkg_dir"/*.go 2>/dev/null | wc -l | tr -d ' ')

    pkg_total=$((funcs + types + ifaces + consts))

    # Skip packages with zero exports
    if [ "$pkg_total" -eq 0 ]; then
        continue
    fi

    total_exports=$((total_exports + pkg_total))

    # Build package JSON entry
    if [ $first_pkg -eq 0 ]; then
        packages_json="${packages_json},"
    fi
    packages_json="${packages_json}
    {\"name\": \"${pkg_name}\", \"exports\": ${pkg_total}, \"functions\": ${funcs}, \"types\": ${types}, \"interfaces\": ${ifaces}, \"constants\": ${consts}}"
    first_pkg=0

    # Warn if over threshold
    if [ "$pkg_total" -gt "$THRESHOLD" ]; then
        if [ $first_warn -eq 0 ]; then
            warnings_json="${warnings_json},"
        fi
        warnings_json="${warnings_json}
    \"package ${pkg_name} has ${pkg_total} exports (threshold: ${THRESHOLD})\""
        first_warn=0
    fi

done < <(find "$PROJECT_DIR" -type f -name "*.go" ! -name "*_test.go" -exec dirname {} \; | sort -u)

# Compose final JSON
cat <<EOF
{
  "project": "${PROJECT_NAME}",
  "total_exports": ${total_exports},
  "packages": [${packages_json}
  ],
  "warnings": [${warnings_json}
  ]
}
EOF
