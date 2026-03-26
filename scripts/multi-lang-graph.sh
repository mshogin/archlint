#!/bin/bash
# multi-lang-graph.sh - Multi-language architecture graph builder
# Usage: multi-lang-graph.sh [project_dir]
# Output: Unified archlint JSON with cross-language edges
#
# Detects all languages in a project, runs appropriate scanners,
# merges component graphs, and detects cross-language edges
# via API contracts (OpenAPI specs, protobuf files, fetch/axios calls).

PROJECT=$(realpath "${1:-.}")
SCRIPTS_DIR="$(dirname "$(realpath "$0")")"
ARCHLINT_BIN=/home/assistant/projects/archlint-repo/archlint-rs/target/release/archlint

# Collect detected languages
LANGUAGES=()

[ -f "$PROJECT/go.mod" ] && LANGUAGES+=("go")
[ -f "$PROJECT/Cargo.toml" ] && LANGUAGES+=("rust")
[ -f "$PROJECT/package.json" ] && LANGUAGES+=("typescript")
[ -f "$PROJECT/tsconfig.json" ] && [[ ! " ${LANGUAGES[*]} " =~ " typescript " ]] && LANGUAGES+=("typescript")

# Scan subdirectories for additional language roots
for subdir in "$PROJECT"/*/; do
    [ -d "$subdir" ] || continue
    name=$(basename "$subdir")
    # Skip common non-source dirs
    case "$name" in
        node_modules|.git|dist|build|out|target|vendor) continue ;;
    esac
    [ -f "$subdir/go.mod" ] && [[ ! " ${LANGUAGES[*]} " =~ " go " ]] && LANGUAGES+=("go")
    [ -f "$subdir/Cargo.toml" ] && [[ ! " ${LANGUAGES[*]} " =~ " rust " ]] && LANGUAGES+=("rust")
    [ -f "$subdir/package.json" ] && [[ ! " ${LANGUAGES[*]} " =~ " typescript " ]] && LANGUAGES+=("typescript")
    [ -f "$subdir/build.gradle" ] && [[ ! " ${LANGUAGES[*]} " =~ " kotlin " ]] && LANGUAGES+=("kotlin")
done

# Top-level Kotlin/Java (Android projects)
[ -f "$PROJECT/build.gradle" ] && [[ ! " ${LANGUAGES[*]} " =~ " kotlin " ]] && LANGUAGES+=("kotlin")

if [ ${#LANGUAGES[@]} -eq 0 ]; then
    echo '{"error": "no supported languages detected", "hint": "expected go.mod, Cargo.toml, package.json, or build.gradle"}'
    exit 1
fi

# Temporary directory for per-language scan results
TMPDIR_SCANS=$(mktemp -d)
trap "rm -rf $TMPDIR_SCANS" EXIT

# --- Scan each language ---

scan_go() {
    local scan_root="$1"
    local outfile="$2"
    if [ -x "$ARCHLINT_BIN" ]; then
        "$ARCHLINT_BIN" scan "$scan_root" --format json 2>/dev/null > "$outfile"
    else
        # Fallback: minimal Go scanner using archlint CLI if in PATH
        archlint scan "$scan_root" --format json 2>/dev/null > "$outfile" || \
            echo '{"components":[],"links":[],"metrics":{"scanner":"go","language":"go"}}' > "$outfile"
    fi
}

scan_rust() {
    local scan_root="$1"
    local outfile="$2"
    if [ -x "$ARCHLINT_BIN" ]; then
        "$ARCHLINT_BIN" scan "$scan_root" --format json 2>/dev/null > "$outfile"
    else
        echo '{"components":[],"links":[],"metrics":{"scanner":"rust","language":"rust"}}' > "$outfile"
    fi
}

scan_typescript() {
    local scan_root="$1"
    local outfile="$2"
    bash "$SCRIPTS_DIR/scan-ts.sh" "$scan_root" > "$outfile"
}

scan_kotlin() {
    local scan_root="$1"
    local outfile="$2"
    # Minimal Kotlin/Java scanner: detect .kt and .java files, extract imports
    python3 - "$scan_root" <<'PYEOF' > "$outfile"
import os, re, json, sys
from collections import defaultdict

project = sys.argv[1]
project = os.path.abspath(project)

EXCLUDE_DIRS = {'.git', 'build', 'out', '.gradle', 'node_modules'}
files = []
for root, dirs, filenames in os.walk(project):
    dirs[:] = [d for d in dirs if d not in EXCLUDE_DIRS]
    for f in filenames:
        if f.endswith(('.kt', '.java')):
            files.append(os.path.join(root, f))

components = []
links = []

for filepath in files:
    name = os.path.relpath(filepath, project)
    try:
        with open(filepath, encoding='utf-8', errors='ignore') as f:
            content = f.read()
    except Exception:
        continue

    line_count = content.count('\n') + 1
    pkg_match = re.search(r'^package\s+([\w.]+)', content, re.MULTILINE)
    pkg = pkg_match.group(1) if pkg_match else ''
    imports = re.findall(r'^import\s+([\w.]+)', content, re.MULTILINE)

    comp_id = name.replace('/', '::').replace('\\', '::').replace('.kt', '').replace('.java', '')
    entity = 'kotlin' if name.endswith('.kt') else 'java'
    components.append({'id': comp_id, 'title': comp_id, 'entity': entity, 'lines': line_count})

    for imp in imports:
        links.append({'from': comp_id, 'to': imp, 'link_type': 'depends'})

result = {
    'components': components,
    'links': links,
    'metrics': {
        'component_count': len(components),
        'link_count': len(links),
        'scanner': 'kotlin',
        'language': 'kotlin',
    },
}
print(json.dumps(result, indent=2))
PYEOF
}

# Run scanners for each detected language
for lang in "${LANGUAGES[@]}"; do
    outfile="$TMPDIR_SCANS/${lang}.json"
    # Find the scan root for this language (prefer subdir with manifest)
    scan_root="$PROJECT"
    for subdir in "$PROJECT"/*/; do
        case "$lang" in
            go)       [ -f "$subdir/go.mod" ] && scan_root="$subdir" && break ;;
            rust)     [ -f "$subdir/Cargo.toml" ] && scan_root="$subdir" && break ;;
            typescript) [ -f "$subdir/package.json" ] && scan_root="$subdir" && break ;;
            kotlin)   [ -f "$subdir/build.gradle" ] && scan_root="$subdir" && break ;;
        esac
    done
    # If not found in subdir, use project root
    case "$lang" in
        go)        [ -f "$scan_root/go.mod" ] || scan_root="$PROJECT" ;;
        rust)      [ -f "$scan_root/Cargo.toml" ] || scan_root="$PROJECT" ;;
        typescript) [ -f "$scan_root/package.json" ] || scan_root="$PROJECT" ;;
        kotlin)    [ -f "$scan_root/build.gradle" ] || scan_root="$PROJECT" ;;
    esac

    case "$lang" in
        go)         scan_go "$scan_root" "$outfile" ;;
        rust)       scan_rust "$scan_root" "$outfile" ;;
        typescript) scan_typescript "$scan_root" "$outfile" ;;
        kotlin)     scan_kotlin "$scan_root" "$outfile" ;;
    esac
done

# --- Merge all scan results and detect cross-language edges ---

python3 - "$TMPDIR_SCANS" "$PROJECT" "${LANGUAGES[@]}" <<'PYEOF'
import os, re, json, sys, glob

scans_dir = sys.argv[1]
project_root = sys.argv[2]
languages = sys.argv[3:]

all_components = []
all_links = []
all_violations = []
all_cycles = []
per_lang_metrics = {}

# Load and prefix per-language scan results
for lang in languages:
    scan_file = os.path.join(scans_dir, f"{lang}.json")
    if not os.path.exists(scan_file):
        continue
    try:
        with open(scan_file) as f:
            data = json.load(f)
    except Exception:
        continue

    components = data.get('components', [])
    links = data.get('links', [])
    metrics = data.get('metrics', {})

    # Prefix all component IDs with language
    id_map = {}
    for comp in components:
        orig_id = comp['id']
        new_id = f"{lang}::{orig_id}"
        id_map[orig_id] = new_id
        comp['id'] = new_id
        comp['title'] = comp.get('title', orig_id)
        comp['lang'] = lang
        all_components.append(comp)

    # Remap link IDs
    for link in links:
        src = link.get('from', '')
        tgt = link.get('to', '')
        link['from'] = id_map.get(src, f"{lang}::{src}")
        # Only prefix target if it was a known internal component
        if tgt in id_map:
            link['to'] = id_map[tgt]
        else:
            link['to'] = tgt  # external dependency, keep as-is
        link['lang'] = lang
        all_links.append(link)

    # Collect violations and cycles
    for v in metrics.get('violations', []):
        if 'component' in v:
            v['component'] = id_map.get(v['component'], f"{lang}::{v['component']}")
        v['lang'] = lang
        all_violations.append(v)

    for cycle in metrics.get('cycles', []):
        prefixed = [id_map.get(n, f"{lang}::{n}") for n in cycle]
        all_cycles.append(prefixed)

    per_lang_metrics[lang] = {
        'component_count': metrics.get('component_count', len(components)),
        'link_count': metrics.get('link_count', len(links)),
    }

# --- Detect cross-language edges ---
# Strategy 1: OpenAPI / protobuf files -> service names in Go/Rust match fetch targets in TS
# Strategy 2: HTTP fetch/axios calls in TS matching Go/Rust service names
# Strategy 3: URL patterns in TS that look like service endpoints

def detect_cross_language_edges(project_root, all_components):
    cross_edges = []

    # Build sets of component IDs per language for lookup
    lang_comps = {}
    for comp in all_components:
        lang = comp.get('lang', '')
        lang_comps.setdefault(lang, set()).add(comp['id'])

    # Scan TS files for HTTP calls (fetch, axios) with URL patterns
    ts_http_calls = []
    exclude_dirs = {'node_modules', '.git', 'dist', 'build', 'out', '.next'}
    for root, dirs, files in os.walk(project_root):
        dirs[:] = [d for d in dirs if d not in exclude_dirs]
        for fname in files:
            if not fname.endswith(('.ts', '.tsx', '.js', '.jsx')):
                continue
            fpath = os.path.join(root, fname)
            rel = os.path.relpath(fpath, project_root)
            try:
                content = open(fpath, encoding='utf-8', errors='ignore').read()
            except Exception:
                continue
            # Match fetch/axios/http calls with URL paths like /api/something or http://service/
            patterns = [
                r'''(?:fetch|axios\.get|axios\.post|axios\.put|axios\.delete|http\.get|http\.post)\s*\(\s*['"` ]([^'"`\s,)]+)''',
                r'''url\s*[:=]\s*['"` ]([^'"`\s,]+)''',
            ]
            for pat in patterns:
                for url in re.findall(pat, content):
                    if url.startswith(('/api', '/v1', '/v2', 'http://', 'https://', 'ws://', 'wss://')):
                        ts_http_calls.append({
                            'caller_file': rel,
                            'url': url,
                            'lang': 'typescript',
                        })

    # Scan Go/Rust files for route/handler definitions
    go_routes = {}
    for root, dirs, files in os.walk(project_root):
        dirs[:] = [d for d in dirs if d not in {'vendor', '.git', 'testdata'}]
        for fname in files:
            if not fname.endswith(('.go', '.rs')):
                continue
            fpath = os.path.join(root, fname)
            rel = os.path.relpath(fpath, project_root)
            try:
                content = open(fpath, encoding='utf-8', errors='ignore').read()
            except Exception:
                continue
            lang = 'go' if fname.endswith('.go') else 'rust'
            # Match route registrations: HandleFunc("/path", ...) or .Route("/path", ...)
            route_pats = [
                r'''HandleFunc\s*\(\s*["'`]([^"'`]+)["'`]''',
                r'''\.(?:GET|POST|PUT|DELETE|PATCH|Handle)\s*\(\s*["'`]([^"'`]+)["'`]''',
                r'''router\s*\.\s*(?:get|post|put|delete|patch)\s*\(\s*["'`]([^"'`]+)["'`]''',
                r'''#\[(?:get|post|put|delete|patch)\s*\(\s*["']([^"']+)["']''',  # Rust actix/axum
            ]
            for pat in route_pats:
                for route in re.findall(pat, content):
                    go_routes[route] = {'file': rel, 'lang': lang}

    # Also scan OpenAPI / protobuf specs for service names
    api_specs = []
    for root, dirs, files in os.walk(project_root):
        dirs[:] = [d for d in dirs if d not in {'.git', 'node_modules'}]
        for fname in files:
            if fname.endswith(('.yaml', '.yml', '.json')) and ('openapi' in fname.lower() or 'swagger' in fname.lower() or 'api' in fname.lower()):
                api_specs.append(os.path.join(root, fname))
            if fname.endswith('.proto'):
                api_specs.append(os.path.join(root, fname))

    # Match TS callers with Go/Rust routes
    for call in ts_http_calls:
        url_path = call['url']
        # Normalize: strip domain/port if present
        url_path = re.sub(r'^https?://[^/]+', '', url_path)
        url_path = url_path.split('?')[0].rstrip('/')

        for route_path, route_info in go_routes.items():
            route_norm = route_path.rstrip('/')
            # Direct match or prefix match (e.g. /api matches /api/*)
            if url_path == route_norm or url_path.startswith(route_norm + '/') or route_norm.startswith(url_path + '/'):
                # Find the TS component ID
                ts_file = call['caller_file']
                ts_comp_id = f"typescript::{re.sub(r'\\.(ts|tsx|js|jsx)$', '', ts_file).replace('/', '::')}"
                # Find the Go/Rust component ID
                target_lang = route_info['lang']
                route_file = route_info['file']
                target_comp_id = f"{target_lang}::{re.sub(r'\\.(go|rs)$', '', route_file).replace('/', '::')}"

                cross_edges.append({
                    'from': ts_comp_id,
                    'to': target_comp_id,
                    'link_type': 'http_call',
                    'lang': 'cross',
                    'route': route_path,
                    'url': call['url'],
                })

    return cross_edges

cross_edges = detect_cross_language_edges(project_root, all_components)
all_links.extend(cross_edges)

# Build unified metrics
from collections import defaultdict
fan_out = defaultdict(set)
for link in all_links:
    fan_out[link['from']].add(link['to'])
fan_out_vals = [len(v) for v in fan_out.values()]

fan_in = defaultdict(int)
for link in all_links:
    fan_in[link['to']] += 1

result = {
    'components': all_components,
    'links': all_links,
    'metrics': {
        'component_count': len(all_components),
        'link_count': len(all_links),
        'cross_language_edges': len(cross_edges),
        'max_fan_out': max(fan_out_vals) if fan_out_vals else 0,
        'max_fan_in': max(fan_in.values()) if fan_in else 0,
        'cycles': all_cycles,
        'violations': all_violations,
        'scanner': 'multi-lang-graph',
        'languages': languages,
        'per_language': per_lang_metrics,
    },
}

print(json.dumps(result, indent=2))
PYEOF
