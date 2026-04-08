#!/usr/bin/env python3
"""Collect architecture history for all repos in monitored-repos.yaml."""

import subprocess
import sys
import os
import argparse
from pathlib import Path

try:
    import yaml
except ImportError:
    print("ERROR: PyYAML required. Install with: pip install pyyaml", file=sys.stderr)
    sys.exit(1)

ARCHLINT_REPO = Path(__file__).parent.parent
DEFAULT_MONITORED_REPOS = ARCHLINT_REPO / "monitored-repos.yaml"
DEFAULT_OUTPUT_BASE = ARCHLINT_REPO / "scan-results"
COLLECT_HISTORY_SCRIPT = Path(__file__).parent / "collect-history.py"


def load_repos(yaml_path):
    """Load repos list from monitored-repos.yaml."""
    with open(yaml_path) as f:
        data = yaml.safe_load(f)
    return data.get("repos", [])


def repo_output_dir(base_dir, repo_url):
    """Derive output dir from repo URL: base/owner/name/."""
    # https://github.com/fatih/color -> fatih/color
    parts = repo_url.rstrip("/").split("/")
    if len(parts) >= 2:
        owner = parts[-2]
        name = parts[-1].replace(".git", "")
        return Path(base_dir) / owner / name
    return Path(base_dir) / "unknown" / parts[-1]


def collect_one(repo_url, language, output_dir, samples, verbose=False):
    """Run collect-history.py for one repo as a subprocess."""
    cmd = [
        sys.executable,
        str(COLLECT_HISTORY_SCRIPT),
        "--repo", repo_url,
        "--language", language,
        "--samples", str(samples),
        "--output", str(output_dir),
    ]

    print(f"\n{'='*60}", flush=True)
    print(f"Collecting: {repo_url}", flush=True)
    print(f"Output:     {output_dir}", flush=True)
    print(f"{'='*60}", flush=True)

    result = subprocess.run(cmd, text=True)
    return result.returncode == 0


def main():
    parser = argparse.ArgumentParser(
        description="Collect architecture history for all repos in monitored-repos.yaml."
    )
    parser.add_argument(
        "--config",
        default=str(DEFAULT_MONITORED_REPOS),
        help="Path to monitored-repos.yaml",
    )
    parser.add_argument(
        "--output-base",
        default=str(DEFAULT_OUTPUT_BASE),
        help="Base directory for output (default: scan-results/)",
    )
    parser.add_argument(
        "--samples",
        type=int,
        default=100,
        help="Number of commits to sample per repo (default: 100)",
    )
    parser.add_argument(
        "--filter-lang",
        help="Only process repos with this language (e.g. Go, Rust)",
    )
    parser.add_argument(
        "--filter-repo",
        help="Only process repos matching this substring (e.g. fatih/color)",
    )
    parser.add_argument(
        "--skip-existing",
        action="store_true",
        help="Skip repos that already have history.json in output dir",
    )
    parser.add_argument(
        "--dry-run",
        action="store_true",
        help="Show what would be collected without actually running",
    )
    args = parser.parse_args()

    repos = load_repos(args.config)
    if not repos:
        print("ERROR: No repos found in config", file=sys.stderr)
        sys.exit(1)

    # Apply filters
    filtered = []
    for repo in repos:
        url = repo.get("url", "")
        lang = repo.get("language", "Go")
        status = repo.get("status", "active")

        if status != "active":
            continue
        if args.filter_lang and lang.lower() != args.filter_lang.lower():
            continue
        if args.filter_repo and args.filter_repo.lower() not in url.lower():
            continue

        output_dir = repo_output_dir(args.output_base, url)

        if args.skip_existing and (output_dir / "history.json").exists():
            print(f"SKIP (existing): {url}", flush=True)
            continue

        filtered.append((url, lang, output_dir))

    if not filtered:
        print("No repos to process (after filters).", flush=True)
        sys.exit(0)

    print(f"Repos to collect: {len(filtered)}", flush=True)

    if args.dry_run:
        for url, lang, output_dir in filtered:
            print(f"  {url} ({lang}) -> {output_dir}", flush=True)
        sys.exit(0)

    success = 0
    failed = 0
    failed_repos = []

    for i, (url, lang, output_dir) in enumerate(filtered):
        print(f"\nProgress: {i+1}/{len(filtered)} repos", flush=True)
        ok = collect_one(url, lang, output_dir, args.samples)
        if ok:
            success += 1
        else:
            failed += 1
            failed_repos.append(url)

    print(f"\n{'='*60}", flush=True)
    print(f"Summary: {success} success, {failed} failed", flush=True)
    if failed_repos:
        print("Failed repos:", flush=True)
        for r in failed_repos:
            print(f"  - {r}", flush=True)

    sys.exit(0 if failed == 0 else 1)


if __name__ == "__main__":
    main()
