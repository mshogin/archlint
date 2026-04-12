#!/usr/bin/env python3
"""Generate Hugo-compatible markdown pages from nightly scan results.

Reads scan-results/health-summary.json and per-repo JSON files,
produces site-content/scan/ directory with Hugo markdown pages.

These pages can be copied to archlint.ru/content/scan/ for publishing.
"""

import json
import os
from datetime import datetime


def health_bar(health: int) -> str:
    """Text-based health indicator."""
    if health >= 80:
        return f"{health}% (good)"
    elif health >= 60:
        return f"{health}% (moderate)"
    elif health >= 40:
        return f"{health}% (needs work)"
    else:
        return f"{health}% (poor)"


def generate_index(summary: dict, output_dir: str):
    """Generate the main scan results index page."""
    scan_date = summary.get("scan_date", "unknown")
    repos = summary.get("repos", [])

    # Sort by health descending
    repos_sorted = sorted(repos, key=lambda r: r["health"], reverse=True)

    lines = [
        "---",
        'title: "Scan Results"',
        'description: "Nightly architecture analysis of open-source projects"',
        "---",
        "",
        f"Last scan: {scan_date[:10]}",
        f"Repositories monitored: {len(repos)}",
        "",
        "## Health Dashboard",
        "",
        "| Repository | Language | Health | Passed | Failed | Warnings |",
        "|-----------|----------|--------|--------|--------|----------|",
    ]

    for r in repos_sorted:
        repo_link = f"[{r['owner']}/{r['name']}]({r['owner']}-{r['name']}/)"
        lines.append(
            f"| {repo_link} | {r['language']} | {health_bar(r['health'])} "
            f"| {r['passed']} | {r['failed']} | {r['warnings']} |"
        )

    lines.extend([
        "",
        "## How it works",
        "",
        "Every night at 3:00 UTC, archlint clones each monitored repository, "
        "builds the architecture dependency graph, and runs 229+ validators "
        "across core metrics, SOLID principles, and advanced graph analysis.",
        "",
        "Health score: `100 - (failed * 5) - (warnings * 2)`, minimum 0.",
        "",
        "Want your repo scanned? "
        "[Open an issue](https://github.com/mshogin/archlint/issues/new?title=Add+repo:+owner/name).",
    ])

    with open(os.path.join(output_dir, "_index.md"), "w") as f:
        f.write("\n".join(lines) + "\n")


def generate_repo_page(repo: dict, scan_results_dir: str, output_dir: str):
    """Generate a per-repo detail page."""
    owner = repo["owner"]
    name = repo["name"]
    lang = repo["language"]
    health = repo["health"]

    repo_dir = os.path.join(output_dir, f"{owner}-{name}")
    os.makedirs(repo_dir, exist_ok=True)

    lines = [
        "---",
        f'title: "{owner}/{name}"',
        f'description: "Architecture analysis of {owner}/{name}"',
        "---",
        "",
        f"**Repository:** [{owner}/{name}](https://github.com/{owner}/{name})",
        f"**Language:** {lang}",
        f"**Health:** {health_bar(health)}",
        "",
        "## Validation Results",
        "",
    ]

    for group in ["core", "solid", "advanced"]:
        json_path = os.path.join(scan_results_dir, owner, name, f"{group}.json")
        if not os.path.exists(json_path):
            continue

        try:
            data = json.load(open(json_path))
            checks = data.get("checks", [])
        except (json.JSONDecodeError, KeyError):
            continue

        if not checks:
            continue

        lines.append(f"### {group.title()} metrics")
        lines.append("")
        lines.append("| Check | Status | Value | Threshold |")
        lines.append("|-------|--------|-------|-----------|")

        for check in sorted(checks, key=lambda c: c.get("status", "")):
            status = check.get("status", "?")
            check_name = check.get("name", "?")
            value = check.get("value", "")
            threshold = check.get("threshold", "")
            if isinstance(value, float):
                value = f"{value:.3f}"
            lines.append(f"| {check_name} | {status} | {value} | {threshold} |")

        lines.append("")

    with open(os.path.join(repo_dir, "_index.md"), "w") as f:
        f.write("\n".join(lines) + "\n")


def main():
    summary_path = "scan-results/health-summary.json"
    scan_results_dir = "scan-results"
    output_dir = "site-content/scan"

    if not os.path.exists(summary_path):
        print("No health-summary.json found, skipping page generation.")
        return

    summary = json.load(open(summary_path))
    os.makedirs(output_dir, exist_ok=True)

    generate_index(summary, output_dir)

    for repo in summary.get("repos", []):
        generate_repo_page(repo, scan_results_dir, output_dir)

    print(f"Generated Hugo pages for {len(summary.get('repos', []))} repos in {output_dir}/")


if __name__ == "__main__":
    main()
