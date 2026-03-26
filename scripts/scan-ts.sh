#!/bin/bash
# scan-ts.sh - TypeScript/JavaScript project scanner for archlint
# Usage: scan-ts.sh [project_dir]
# Output: JSON in archlint scan format (components, links, metrics)

PROJECT=${1:-.}

python3 - "$PROJECT" <<'PYEOF'
import os
import re
import json
import sys
from collections import defaultdict

project = sys.argv[1] if len(sys.argv) > 1 else '.'
project = os.path.abspath(project)

# Find all TS/JS files (excluding node_modules, .git, dist, build)
EXCLUDE_DIRS = {'node_modules', '.git', 'dist', 'build', 'out', '.next', 'coverage'}
files = []
for root, dirs, filenames in os.walk(project):
    dirs[:] = [d for d in dirs if d not in EXCLUDE_DIRS]
    for f in filenames:
        if f.endswith(('.ts', '.tsx', '.js', '.jsx')) and not f.endswith('.d.ts'):
            files.append(os.path.join(root, f))

# Parse imports for each file
components_data = {}
links = []

for filepath in files:
    name = os.path.relpath(filepath, project)
    try:
        with open(filepath, encoding='utf-8', errors='ignore') as f:
            content = f.read()
    except Exception:
        continue

    line_count = content.count('\n') + 1

    # ES6 imports: import ... from '...' (including multiline)
    es_imports = re.findall(r"import\s+(?:.*?\s+from\s+)?['\"]([^'\"]+)['\"]", content, re.DOTALL)
    # require(): require('...')
    require_imports = re.findall(r"require\s*\(\s*['\"]([^'\"]+)['\"]\s*\)", content)
    # dynamic import(): import('...')
    dynamic_imports = re.findall(r"import\s*\(\s*['\"]([^'\"]+)['\"]\s*\)", content)

    all_imports = list(dict.fromkeys(es_imports + require_imports + dynamic_imports))

    # Separate relative vs external imports
    relative_imports = [i for i in all_imports if i.startswith('.')]
    external_imports = [i for i in all_imports if not i.startswith('.')]

    components_data[name] = {
        'lines': line_count,
        'imports': all_imports,
        'relative_imports': relative_imports,
        'external_imports': external_imports,
    }

# Build component ID (use :: separator like archlint)
def file_to_id(rel_path):
    # Remove extension, replace / and . with ::
    no_ext = re.sub(r'\.(ts|tsx|js|jsx)$', '', rel_path)
    return no_ext.replace('/', '::').replace('\\', '::')

# Resolve relative import to file path
EXTENSIONS = ['.ts', '.tsx', '.js', '.jsx', '/index.ts', '/index.tsx', '/index.js', '/index.jsx']

def resolve_import(source_file, import_path, project):
    source_dir = os.path.dirname(os.path.join(project, source_file))
    base = os.path.normpath(os.path.join(source_dir, import_path))
    rel_base = os.path.relpath(base, project)

    # Try with extensions
    for ext in EXTENSIONS:
        candidate = base + ext
        rel_candidate = os.path.relpath(candidate, project)
        if rel_candidate in components_data:
            return rel_candidate

    # Already has extension
    rel_base_file = rel_base
    if rel_base_file in components_data:
        return rel_base_file

    return None

# Build links list
for source_file, data in components_data.items():
    for imp in data['relative_imports']:
        resolved = resolve_import(source_file, imp, project)
        target_id = file_to_id(resolved) if resolved else imp
        links.append({
            'from': file_to_id(source_file),
            'to': target_id,
            'link_type': 'depends',
        })
    for imp in data['external_imports']:
        # Use package name (first segment)
        pkg = imp.split('/')[0] if '/' in imp else imp
        # Handle scoped packages like @scope/pkg
        if imp.startswith('@') and '/' in imp:
            parts = imp.split('/')
            pkg = parts[0] + '/' + parts[1] if len(parts) >= 2 else imp
        links.append({
            'from': file_to_id(source_file),
            'to': pkg,
            'link_type': 'depends',
        })

# Build components list
components = []
for name, data in components_data.items():
    components.append({
        'id': file_to_id(name),
        'title': file_to_id(name),
        'entity': 'typescript' if name.endswith(('.ts', '.tsx')) else 'javascript',
        'lines': data['lines'],
    })

# Detect cycles using DFS
def detect_cycles(components_data, project):
    # Build adjacency list (only resolved relative imports)
    adj = defaultdict(set)
    for source_file, data in components_data.items():
        src_id = file_to_id(source_file)
        for imp in data['relative_imports']:
            resolved = resolve_import(source_file, imp, project)
            if resolved:
                adj[src_id].add(file_to_id(resolved))

    cycles = []
    visited = set()
    rec_stack = set()
    path = []

    def dfs(node):
        visited.add(node)
        rec_stack.add(node)
        path.append(node)

        for neighbor in adj.get(node, set()):
            if neighbor not in visited:
                if dfs(neighbor):
                    return True
            elif neighbor in rec_stack:
                # Found cycle - extract it
                cycle_start = path.index(neighbor)
                cycle = path[cycle_start:]
                cycle_key = tuple(sorted(cycle))
                if cycle_key not in seen_cycles:
                    seen_cycles.add(cycle_key)
                    cycles.append(cycle + [neighbor])
                return False  # Don't stop, find more cycles

        path.pop()
        rec_stack.discard(node)
        return False

    seen_cycles = set()
    for node in list(adj.keys()):
        if node not in visited:
            dfs(node)

    return cycles

cycles = detect_cycles(components_data, project)

# Compute fan-out per component (number of unique dependencies)
fan_out = defaultdict(set)
for link in links:
    fan_out[link['from']].add(link['to'])

FAN_OUT_LIMIT = 10  # Higher limit for TS/JS since external deps are common

# Compute violations
violations = []

# Circular dependencies
for cycle in cycles:
    violations.append({
        'rule': 'circular-dependency',
        'component': cycle[0],
        'message': f"circular import: {' -> '.join(cycle)}",
        'severity': 'error',
    })

# High fan-out (relative imports only, to avoid noise from external packages)
rel_fan_out = defaultdict(set)
for source_file, data in components_data.items():
    src_id = file_to_id(source_file)
    for imp in data['relative_imports']:
        resolved = resolve_import(source_file, imp, project)
        if resolved:
            rel_fan_out[src_id].add(file_to_id(resolved))

for comp_id, deps in rel_fan_out.items():
    if len(deps) > FAN_OUT_LIMIT:
        violations.append({
            'rule': 'fan_out',
            'component': comp_id,
            'message': f"fan-out {len(deps)} exceeds limit {FAN_OUT_LIMIT}",
            'severity': 'warning',
        })

# Large files
LARGE_FILE_LINES = 500
for name, data in components_data.items():
    if data['lines'] > LARGE_FILE_LINES:
        violations.append({
            'rule': 'large-file',
            'component': file_to_id(name),
            'message': f"file has {data['lines']} lines (limit: {LARGE_FILE_LINES})",
            'severity': 'warning',
        })

# Compute metrics
all_fan_out = [len(deps) for deps in fan_out.values()]
all_fan_in = defaultdict(int)
for link in links:
    all_fan_in[link['to']] += 1

result = {
    'components': components,
    'links': links,
    'metrics': {
        'component_count': len(components),
        'link_count': len(links),
        'max_fan_out': max(all_fan_out) if all_fan_out else 0,
        'max_fan_in': max(all_fan_in.values()) if all_fan_in else 0,
        'cycles': cycles,
        'violations': violations,
        'scanner': 'scan-ts',
        'language': 'typescript/javascript',
    },
}

print(json.dumps(result, indent=2))
PYEOF
