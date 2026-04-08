#!/usr/bin/env python3
"""Collect architecture metrics across git history for a repo."""

import subprocess
import json
import os
import sys
import tempfile
import shutil
import argparse
import signal
from datetime import datetime, timezone
from pathlib import Path


ARCHLINT = "/home/assistant/projects/archlint-repo/archlint-rs/target/release/archlint"
VALIDATOR_DIR = "/home/assistant/projects/archlint-repo"
TIMEOUT_PER_COMMIT = 60  # seconds


def run_cmd(cmd, cwd=None, timeout=TIMEOUT_PER_COMMIT, capture=True):
    """Run command, return (stdout, returncode). Returns (None, -1) on timeout."""
    try:
        result = subprocess.run(
            cmd,
            cwd=cwd,
            capture_output=capture,
            text=True,
            timeout=timeout,
        )
        return result.stdout, result.returncode
    except subprocess.TimeoutExpired:
        return None, -1
    except Exception as e:
        return None, -2


def get_commit_samples(repo_dir, num_samples=100):
    """Get evenly-spaced commits from full git history."""
    out, rc = run_cmd(
        ["git", "log", "--format=%H %aI %s", "--no-merges"],
        cwd=repo_dir,
        timeout=30,
    )
    if rc != 0 or not out:
        # Try with merges if no-merges gives nothing
        out, rc = run_cmd(
            ["git", "log", "--format=%H %aI %s"],
            cwd=repo_dir,
            timeout=30,
        )
    if not out:
        return []

    commits = []
    for line in out.strip().splitlines():
        parts = line.split(" ", 2)
        if len(parts) >= 2:
            commits.append({
                "hash": parts[0],
                "date": parts[1][:10],  # YYYY-MM-DD
                "message": parts[2] if len(parts) > 2 else "",
            })

    if not commits:
        return []

    # Evenly-space samples across total history
    total = len(commits)
    if total <= num_samples:
        return commits

    # Pick evenly spaced indices from newest to oldest
    step = total / num_samples
    sampled = []
    for i in range(num_samples):
        idx = int(i * step)
        sampled.append(commits[idx])
    return sampled


def run_validator_group(repo_dir, group):
    """Run validator for a specific group, return {'passed': N, 'failed': N}."""
    arch_yaml = os.path.join(repo_dir, "architecture.yaml")
    if not os.path.exists(arch_yaml):
        return {"passed": 0, "failed": 0, "error": "no architecture.yaml"}

    out, rc = run_cmd(
        [
            sys.executable, "-m", "validator", "validate",
            arch_yaml,
            "--structure-only",
            "--group", group,
            "-f", "json",
        ],
        cwd=VALIDATOR_DIR,
        timeout=30,
    )
    if out is None:
        return {"passed": 0, "failed": 0, "error": "timeout"}

    try:
        data = json.loads(out)
        summary = data.get("summary", {})
        return {
            "passed": summary.get("passed", 0),
            "failed": summary.get("failed", 0),
        }
    except (json.JSONDecodeError, KeyError):
        return {"passed": 0, "failed": 0, "error": "parse_error"}


def scan_commit(repo_dir, commit):
    """Checkout commit and collect metrics. Returns data_point dict or None."""
    commit_hash = commit["hash"]

    # Checkout the commit
    out, rc = run_cmd(
        ["git", "checkout", commit_hash, "--force"],
        cwd=repo_dir,
        timeout=30,
    )
    if rc != 0:
        return None

    # Run archlint scan to get health + violations
    scan_out, rc = run_cmd(
        [ARCHLINT, "scan", ".", "--format", "json"],
        cwd=repo_dir,
        timeout=TIMEOUT_PER_COMMIT,
    )
    if scan_out is None or rc not in (0, 1):
        return None

    try:
        scan_data = json.loads(scan_out)
    except json.JSONDecodeError:
        return None

    # Extract primary language data (first entry)
    per_lang = scan_data.get("per_language", [])
    if not per_lang:
        return None

    lang_data = per_lang[0]
    health = lang_data.get("health", 0)
    components = lang_data.get("components", 0)
    links = lang_data.get("links", 0)
    violations_detail = lang_data.get("violations_detail", [])

    # Calculate fan_out_max, cycles, god_classes from violations_detail
    god_classes = sum(1 for v in violations_detail if v.get("rule") == "god_class")
    cycles_count = scan_data.get("total_cycles", 0)

    # fan_out_max from scan metrics if available, otherwise approximate
    fan_out_max = 0
    for v in violations_detail:
        if v.get("rule") == "fan_out":
            # Try to extract number from message
            import re
            msg = v.get("message", "")
            m = re.search(r"(\d+)", msg)
            if m:
                fan_out_max = max(fan_out_max, int(m.group(1)))

    # Run archlint collect to produce architecture.yaml for validator
    arch_yaml = os.path.join(repo_dir, "architecture.yaml")
    if os.path.exists(arch_yaml):
        os.remove(arch_yaml)

    collect_out, rc = run_cmd(
        [ARCHLINT, "collect", "."],
        cwd=repo_dir,
        timeout=TIMEOUT_PER_COMMIT,
    )

    violations_by_group = {}
    if os.path.exists(arch_yaml):
        for group in ("core", "solid", "advanced"):
            violations_by_group[group] = run_validator_group(repo_dir, group)
    else:
        for group in ("core", "solid", "advanced"):
            violations_by_group[group] = {"passed": 0, "failed": 0, "error": "collect_failed"}

    return {
        "commit": commit_hash[:8],
        "commit_full": commit_hash,
        "date": commit["date"],
        "message": commit["message"][:120],
        "health": health,
        "components": components,
        "links": links,
        "violations": violations_by_group,
        "metrics": {
            "fan_out_max": fan_out_max,
            "cycles": cycles_count,
            "god_classes": god_classes,
        },
    }


def collect_repo_history(repo_url, language, output_dir, num_samples=100):
    """Full pipeline: clone repo, sample commits, collect metrics, save results."""
    repo_name = repo_url.rstrip("/").split("/")[-1].replace(".git", "")
    owner = repo_url.rstrip("/").split("/")[-2] if "/" in repo_url else "unknown"
    repo_slug = f"{owner}/{repo_name}"

    print(f"[collect-history] Repo: {repo_slug}", flush=True)
    print(f"[collect-history] Samples: {num_samples}", flush=True)

    output_dir = Path(output_dir)
    output_dir.mkdir(parents=True, exist_ok=True)
    output_file = output_dir / "history.json"

    # Load partial results if they exist (for resume)
    partial = {}
    if output_file.exists():
        try:
            existing = json.loads(output_file.read_text())
            for dp in existing.get("data_points", []):
                partial[dp.get("commit_full", dp.get("commit", ""))] = dp
            print(f"[collect-history] Resuming: {len(partial)} existing data points", flush=True)
        except Exception:
            pass

    tmpdir = tempfile.mkdtemp(prefix="archlint-history-")
    clone_dir = os.path.join(tmpdir, repo_name)

    result = {
        "repo": repo_slug,
        "repo_url": repo_url,
        "language": language,
        "collected_at": datetime.now(timezone.utc).isoformat().replace("+00:00", "Z"),
        "data_points": [],
    }

    def save_partial():
        all_points = list(partial.values()) + result["data_points"]
        # Deduplicate by commit_full
        seen = {}
        for dp in all_points:
            key = dp.get("commit_full", dp.get("commit", ""))
            seen[key] = dp
        result["data_points"] = sorted(seen.values(), key=lambda x: x.get("date", ""))
        output_file.write_text(json.dumps(result, indent=2, ensure_ascii=False))

    # Handle interruption gracefully
    interrupted = [False]
    original_sigint = signal.getsignal(signal.SIGINT)

    def handle_interrupt(sig, frame):
        interrupted[0] = True
        print("\n[collect-history] Interrupted - saving partial results...", flush=True)
        save_partial()
        signal.signal(signal.SIGINT, original_sigint)

    signal.signal(signal.SIGINT, handle_interrupt)

    try:
        print(f"[collect-history] Cloning {repo_url} ...", flush=True)
        out, rc = run_cmd(
            ["git", "clone", "--no-single-branch", repo_url, clone_dir],
            timeout=120,
        )
        if rc != 0:
            print(f"[collect-history] ERROR: Clone failed (rc={rc})", flush=True)
            return None

        samples = get_commit_samples(clone_dir, num_samples)
        if not samples:
            print("[collect-history] ERROR: No commits found", flush=True)
            return None

        total = len(samples)
        print(f"[collect-history] Total commits sampled: {total}", flush=True)

        skipped = 0
        for i, commit in enumerate(samples):
            if interrupted[0]:
                break

            commit_hash = commit["hash"]
            progress = f"[{i+1}/{total}]"

            # Skip if already collected
            if commit_hash in partial:
                print(f"{progress} SKIP {commit_hash[:8]} (already collected)", flush=True)
                continue

            print(f"{progress} Scanning {commit_hash[:8]} ({commit['date']}) {commit['message'][:50]}", flush=True)

            try:
                data_point = scan_commit(clone_dir, commit)
            except Exception as e:
                print(f"{progress} ERROR: {e}", flush=True)
                data_point = None

            if data_point is None:
                skipped += 1
                print(f"{progress} SKIP {commit_hash[:8]} (scan failed)", flush=True)
                continue

            result["data_points"].append(data_point)
            # Save after each successful scan (partial results protection)
            save_partial()

        # Restore git to HEAD
        run_cmd(["git", "checkout", "HEAD", "--force"], cwd=clone_dir, timeout=30)

        print(f"\n[collect-history] Done: {len(result['data_points'])} points collected, {skipped} skipped", flush=True)
        save_partial()
        print(f"[collect-history] Saved to: {output_file}", flush=True)

    finally:
        signal.signal(signal.SIGINT, original_sigint)
        shutil.rmtree(tmpdir, ignore_errors=True)

    return output_file


def main():
    parser = argparse.ArgumentParser(
        description="Collect architecture metrics across git history for a repo."
    )
    parser.add_argument("--repo", required=True, help="GitHub repo URL (e.g. https://github.com/fatih/color)")
    parser.add_argument("--language", default="Go", help="Primary language (Go, Rust, TypeScript)")
    parser.add_argument("--samples", type=int, default=100, help="Number of evenly-spaced commits to sample")
    parser.add_argument("--output", required=True, help="Output directory (will contain history.json)")
    args = parser.parse_args()

    output_file = collect_repo_history(
        repo_url=args.repo,
        language=args.language,
        output_dir=args.output,
        num_samples=args.samples,
    )

    if output_file:
        sys.exit(0)
    else:
        sys.exit(1)


if __name__ == "__main__":
    main()
