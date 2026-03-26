#!/bin/bash
# Usage: compliance-report.sh /path/to/project [output.html]
# Generates HTML architecture compliance report

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

PROJECT=${1:-.}
OUTPUT=${2:-architecture-report.html}
NAME=$(basename "$(realpath "$PROJECT")")

# Gather data from existing scripts into temp files
TMP_AGENT=$(mktemp /tmp/archlint-agent-XXXXXX.json)
TMP_API=$(mktemp /tmp/archlint-api-XXXXXX.json)
TMP_ANOMALY=$(mktemp /tmp/archlint-anomaly-XXXXXX.json)
trap 'rm -f "$TMP_AGENT" "$TMP_API" "$TMP_ANOMALY"' EXIT

echo "Collecting agent report..." >&2
bash "$SCRIPT_DIR/agent-report.sh" "$PROJECT" 2>/dev/null > "$TMP_AGENT"

echo "Collecting API surface..." >&2
bash "$SCRIPT_DIR/api-surface.sh" "$PROJECT" 2>/dev/null > "$TMP_API"

echo "Detecting anomalies..." >&2
bash "$SCRIPT_DIR/anomaly-detect.sh" "$PROJECT" 2>/dev/null > "$TMP_ANOMALY"

echo "Generating HTML report..." >&2

python3 - "$TMP_AGENT" "$TMP_API" "$TMP_ANOMALY" "$NAME" "$OUTPUT" <<'PYEOF'
import json, sys
from datetime import datetime

agent_file, api_file, anomaly_file, name, output = sys.argv[1:]

with open(agent_file) as f:
    agent = json.load(f)
with open(api_file) as f:
    api = json.load(f)
with open(anomaly_file) as f:
    anomaly = json.load(f)

health = max(0, min(100, agent.get('health_score', 100)))

if health >= 80:
    score_class = 'good'
    score_label = 'Healthy'
elif health >= 60:
    score_class = 'warn'
    score_label = 'Needs Attention'
else:
    score_class = 'bad'
    score_label = 'Critical'

date_str = datetime.now().strftime('%Y-%m-%d %H:%M')

# Build violations table rows
violations_rows = ''
for item in agent.get('action_items', []):
    sev = item.get('severity', 'warning')
    sev_class = 'bad' if sev == 'error' else ('warn' if sev == 'warning' else '')
    violations_rows += f"""
    <tr>
      <td>{item.get('component', '')}</td>
      <td class="{sev_class}">{sev}</td>
      <td>{item.get('smell', '')}</td>
      <td>{item.get('message', '')}</td>
      <td>{item.get('fix_strategy', '')}</td>
    </tr>"""

if not violations_rows:
    violations_rows = '<tr><td colspan="5" style="text-align:center;color:#16a34a">No violations found</td></tr>'

# Build API surface table rows
api_rows = ''
for pkg in sorted(api.get('packages', []), key=lambda x: x.get('exports', 0), reverse=True):
    warn = ' class="warn"' if pkg.get('exports', 0) > 20 else ''
    api_rows += f"""
    <tr{warn}>
      <td>{pkg.get('name', '')}</td>
      <td>{pkg.get('exports', 0)}</td>
      <td>{pkg.get('functions', 0)}</td>
      <td>{pkg.get('types', 0)}</td>
      <td>{pkg.get('interfaces', 0)}</td>
      <td>{pkg.get('constants', 0)}</td>
    </tr>"""

if not api_rows:
    api_rows = '<tr><td colspan="6" style="text-align:center">No packages found</td></tr>'

# API warnings
api_warn_html = ''
for w in api.get('warnings', []):
    api_warn_html += f'<li class="warn">{w}</li>'
if api_warn_html:
    api_warn_html = f'<ul>{api_warn_html}</ul>'

# Build anomalies table rows
anomaly_rows = ''
for a in anomaly.get('anomalies', []):
    sev = a.get('severity', 'medium')
    sev_class = 'bad' if sev == 'high' else 'warn'
    anomaly_rows += f"""
    <tr>
      <td>{a.get('component', '')}</td>
      <td>{a.get('metric', '')}</td>
      <td class="{sev_class}">{a.get('value', 0)}</td>
      <td>{a.get('threshold', 0)}</td>
      <td class="{sev_class}">{sev}</td>
    </tr>"""

if not anomaly_rows:
    anomaly_rows = '<tr><td colspan="5" style="text-align:center;color:#16a34a">No anomalies detected</td></tr>'

html = f"""<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="UTF-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <title>Architecture Report - {name}</title>
  <style>
    body {{
      font-family: -apple-system, BlinkMacSystemFont, 'Segoe UI', Roboto, sans-serif;
      max-width: 960px;
      margin: 0 auto;
      padding: 2rem;
      color: #1a1a1a;
      background: #f9fafb;
    }}
    h1 {{ border-bottom: 2px solid #333; padding-bottom: 0.5rem; }}
    h2 {{ margin-top: 2rem; color: #374151; border-left: 4px solid #6b7280; padding-left: 0.75rem; }}
    .meta {{ color: #6b7280; margin-bottom: 2rem; }}
    .score-block {{
      display: inline-block;
      background: white;
      border-radius: 12px;
      padding: 1.5rem 2.5rem;
      margin-bottom: 2rem;
      box-shadow: 0 1px 4px rgba(0,0,0,0.1);
      text-align: center;
    }}
    .score {{ font-size: 3rem; font-weight: bold; line-height: 1; }}
    .score-label {{ font-size: 1rem; margin-top: 0.25rem; }}
    .good {{ color: #16a34a; }}
    .warn {{ color: #d97706; }}
    .bad {{ color: #dc2626; }}
    .summary-grid {{
      display: grid;
      grid-template-columns: repeat(4, 1fr);
      gap: 1rem;
      margin-bottom: 2rem;
    }}
    .stat-card {{
      background: white;
      border-radius: 8px;
      padding: 1rem;
      box-shadow: 0 1px 3px rgba(0,0,0,0.08);
      text-align: center;
    }}
    .stat-value {{ font-size: 2rem; font-weight: bold; color: #1f2937; }}
    .stat-label {{ font-size: 0.85rem; color: #6b7280; margin-top: 0.25rem; }}
    table {{
      border-collapse: collapse;
      width: 100%;
      background: white;
      border-radius: 8px;
      overflow: hidden;
      box-shadow: 0 1px 3px rgba(0,0,0,0.08);
      margin-bottom: 1rem;
    }}
    th {{
      background: #f3f4f6;
      padding: 10px 12px;
      text-align: left;
      font-size: 0.85rem;
      font-weight: 600;
      color: #374151;
      border-bottom: 1px solid #e5e7eb;
    }}
    td {{
      padding: 9px 12px;
      border-bottom: 1px solid #f3f4f6;
      font-size: 0.9rem;
    }}
    tr:last-child td {{ border-bottom: none; }}
    tr:hover td {{ background: #fafafa; }}
    tr.warn td {{ background: #fffbeb; }}
    footer {{ margin-top: 3rem; padding-top: 1rem; border-top: 1px solid #e5e7eb; color: #9ca3af; font-size: 0.8rem; }}
  </style>
</head>
<body>
  <h1>Architecture Compliance Report</h1>
  <p class="meta">Project: <strong>{name}</strong> &nbsp;|&nbsp; Generated: {date_str}</p>

  <div class="score-block">
    <div class="score {score_class}">{health}/100</div>
    <div class="score-label {score_class}">{score_label}</div>
  </div>

  <div class="summary-grid">
    <div class="stat-card">
      <div class="stat-value">{agent.get('components', 0)}</div>
      <div class="stat-label">Components</div>
    </div>
    <div class="stat-card">
      <div class="stat-value {'bad' if agent.get('violations', 0) > 0 else 'good'}" style="font-size:2rem;font-weight:bold;color:{'#dc2626' if agent.get('violations',0) > 0 else '#16a34a'}">{agent.get('violations', 0)}</div>
      <div class="stat-label">Violations</div>
    </div>
    <div class="stat-card">
      <div class="stat-value" style="color:{'#dc2626' if agent.get('cycles', 0) > 0 else '#16a34a'}">{agent.get('cycles', 0)}</div>
      <div class="stat-label">Cycles</div>
    </div>
    <div class="stat-card">
      <div class="stat-value" style="color:{'#dc2626' if anomaly.get('anomalies_found', 0) > 0 else '#16a34a'}">{anomaly.get('anomalies_found', 0)}</div>
      <div class="stat-label">Anomalies</div>
    </div>
  </div>

  <h2>Violations</h2>
  <table>
    <thead>
      <tr>
        <th>Component</th>
        <th>Severity</th>
        <th>Rule</th>
        <th>Message</th>
        <th>Fix Strategy</th>
      </tr>
    </thead>
    <tbody>{violations_rows}
    </tbody>
  </table>

  <h2>Anomalies</h2>
  <table>
    <thead>
      <tr>
        <th>Component</th>
        <th>Metric</th>
        <th>Value</th>
        <th>Threshold</th>
        <th>Severity</th>
      </tr>
    </thead>
    <tbody>{anomaly_rows}
    </tbody>
  </table>

  <h2>API Surface</h2>
  <p class="meta">Total exports: <strong>{api.get('total_exports', 0)}</strong></p>
  {api_warn_html}
  <table>
    <thead>
      <tr>
        <th>Package</th>
        <th>Total Exports</th>
        <th>Functions</th>
        <th>Types</th>
        <th>Interfaces</th>
        <th>Constants</th>
      </tr>
    </thead>
    <tbody>{api_rows}
    </tbody>
  </table>

  <footer>
    Generated by <a href="https://github.com/mshogin/archlint">archlint</a> compliance-report.sh
  </footer>
</body>
</html>"""

with open(output, 'w') as f:
    f.write(html)

print(f"Report written to: {output}")
PYEOF
