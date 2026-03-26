#!/bin/bash
# Usage: auto-scan.sh /path/to/project
# Auto-detects language and runs appropriate scanner

PROJECT=${1:-.}
SCRIPTS_DIR="$(dirname "$0")"

# Detect language
detect_language() {
    if [ -f "$PROJECT/go.mod" ]; then echo "go"
    elif [ -f "$PROJECT/Cargo.toml" ]; then echo "rust"
    elif [ -f "$PROJECT/package.json" ]; then echo "typescript"
    elif [ -f "$PROJECT/tsconfig.json" ]; then echo "typescript"
    else echo "unknown"
    fi
}

LANG=$(detect_language)

case "$LANG" in
    go|rust)
        # Use archlint Rust binary (supports both Go and Rust)
        ARCHLINT=/home/assistant/projects/archlint-repo/archlint-rs/target/release/archlint
        $ARCHLINT scan "$PROJECT" --format json 2>/dev/null
        ;;
    typescript)
        # Use TypeScript scanner
        bash "$SCRIPTS_DIR/scan-ts.sh" "$PROJECT"
        ;;
    unknown)
        echo '{"error": "unknown project language", "hint": "expected go.mod, Cargo.toml, or package.json"}'
        exit 1
        ;;
esac
