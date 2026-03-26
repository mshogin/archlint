#!/bin/bash
# Usage: health-badge.sh /path/to/project [output.svg]
# Generates shields.io-style SVG badge with architecture health score

PROJECT=${1:-.}
OUTPUT=${2:-architecture-badge.svg}

# Get health score from agent-report
REPORT=$(bash "$(dirname "$0")/agent-report.sh" "$PROJECT" 2>/dev/null)
SCORE=$(echo "$REPORT" | python3 -c "import sys,json; print(json.load(sys.stdin).get('health_score',0))")

# Color based on score
if [ "$SCORE" -ge 80 ]; then COLOR="#4c1"; LABEL="healthy"
elif [ "$SCORE" -ge 60 ]; then COLOR="#dfb317"; LABEL="moderate"
else COLOR="#e05d44"; LABEL="at risk"
fi

# Generate shields.io-style SVG
cat > "$OUTPUT" << SVGEOF
<svg xmlns="http://www.w3.org/2000/svg" width="180" height="20">
  <linearGradient id="b" x2="0" y2="100%">
    <stop offset="0" stop-color="#bbb" stop-opacity=".1"/>
    <stop offset="1" stop-opacity=".1"/>
  </linearGradient>
  <clipPath id="a"><rect width="180" height="20" rx="3"/></clipPath>
  <g clip-path="url(#a)">
    <rect width="100" height="20" fill="#555"/>
    <rect x="100" width="80" height="20" fill="$COLOR"/>
    <rect width="180" height="20" fill="url(#b)"/>
  </g>
  <g fill="#fff" text-anchor="middle" font-family="DejaVu Sans,Verdana,Geneva,sans-serif" font-size="11">
    <text x="50" y="15" fill="#010101" fill-opacity=".3">architecture</text>
    <text x="50" y="14">architecture</text>
    <text x="140" y="15" fill="#010101" fill-opacity=".3">$SCORE/100</text>
    <text x="140" y="14">$SCORE/100</text>
  </g>
</svg>
SVGEOF

# Also output JSON for programmatic use
echo "{\"score\": $SCORE, \"label\": \"$LABEL\", \"color\": \"$COLOR\", \"badge\": \"$OUTPUT\"}"
