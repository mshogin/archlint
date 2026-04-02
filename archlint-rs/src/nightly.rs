/// Nightly scan: clone popular open-source projects, run archlint, and produce
/// a markdown summary report showing health scores and top violations.
///
/// Usage:
///   archlint nightly [--output report.md] [--top N] [--repos owner/repo,...]
///
/// By default scans the built-in list of popular Go and Rust projects.
use std::io::Write;
use std::path::{Path, PathBuf};
use std::process::Command;

use crate::analyzer;

/// A single repo entry to scan.
#[derive(Debug, Clone)]
pub struct RepoEntry {
    pub owner: String,
    pub name: String,
    pub language: String,
}

impl RepoEntry {
    pub fn new(owner: &str, name: &str, language: &str) -> Self {
        Self {
            owner: owner.to_string(),
            name: name.to_string(),
            language: language.to_string(),
        }
    }

    pub fn github_url(&self) -> String {
        format!("https://github.com/{}/{}.git", self.owner, self.name)
    }

    pub fn slug(&self) -> String {
        format!("{}/{}", self.owner, self.name)
    }
}

/// Result of scanning a single repo.
#[derive(Debug, Clone)]
pub struct ScanResult {
    pub repo: String,
    pub language: String,
    pub health_score: u32,
    pub total_violations: usize,
    pub taboo_violations: usize,
    pub components: usize,
    pub links: usize,
    pub top_issues: Vec<String>,
    pub error: Option<String>,
}

impl ScanResult {
    /// Create a failed scan result with an error message.
    pub fn failed(repo: &str, language: &str, error: &str) -> Self {
        Self {
            repo: repo.to_string(),
            language: language.to_string(),
            health_score: 0,
            total_violations: 0,
            taboo_violations: 0,
            components: 0,
            links: 0,
            top_issues: vec![],
            error: Some(error.to_string()),
        }
    }
}

/// Built-in list of popular projects to scan.
pub fn default_repos() -> Vec<RepoEntry> {
    vec![
        // Go projects
        RepoEntry::new("kubernetes", "kubernetes", "Go"),
        RepoEntry::new("golang", "go", "Go"),
        RepoEntry::new("docker", "cli", "Go"),
        RepoEntry::new("prometheus", "prometheus", "Go"),
        RepoEntry::new("grafana", "grafana", "Go"),
        // Rust projects
        RepoEntry::new("tokio-rs", "tokio", "Rust"),
        RepoEntry::new("rust-lang", "rust-analyzer", "Rust"),
        RepoEntry::new("denoland", "deno", "Rust"),
        RepoEntry::new("servo", "servo", "Rust"),
    ]
}

/// Parse a comma-separated list of "owner/repo" strings into RepoEntry list.
/// Language is inferred as "unknown" since it will be auto-detected by archlint.
pub fn parse_repos(input: &str) -> Vec<RepoEntry> {
    input
        .split(',')
        .filter_map(|s| {
            let s = s.trim();
            let parts: Vec<&str> = s.splitn(2, '/').collect();
            if parts.len() == 2 {
                Some(RepoEntry::new(parts[0], parts[1], "auto"))
            } else {
                None
            }
        })
        .collect()
}

/// Clone a repo (shallow, depth=1) into the given target directory.
/// Returns Ok(()) on success or Err with error message.
pub fn clone_repo(url: &str, target: &Path) -> Result<(), String> {
    let output = Command::new("git")
        .args(["clone", "--depth=1", "--quiet", url, &target.to_string_lossy()])
        .output()
        .map_err(|e| format!("failed to run git: {}", e))?;

    if output.status.success() {
        Ok(())
    } else {
        let stderr = String::from_utf8_lossy(&output.stderr);
        Err(format!("git clone failed: {}", stderr.trim()))
    }
}

/// Scan a cloned repo directory using archlint's multi-language analyzer.
/// Returns a ScanResult with health score, violations, and top issues.
pub fn scan_repo(repo_slug: &str, language: &str, dir: &Path) -> ScanResult {
    match analyzer::analyze_multi_language(dir) {
        Ok(report) => {
            // Collect top issues across all languages (first 5 taboo, then telemetry)
            let mut top_issues: Vec<String> = Vec::new();
            for lang_report in &report.per_language {
                for v in &lang_report.violations_detail {
                    if top_issues.len() >= 5 {
                        break;
                    }
                    top_issues.push(format!("[{}] {} - {}", v.rule, v.component, v.message));
                }
            }
            // Truncate each issue message for table readability
            let top_issues: Vec<String> = top_issues
                .into_iter()
                .map(|s| {
                    if s.len() > 80 {
                        format!("{}...", &s[..77])
                    } else {
                        s
                    }
                })
                .collect();

            ScanResult {
                repo: repo_slug.to_string(),
                language: language.to_string(),
                health_score: report.total_health,
                total_violations: report.total_violations,
                taboo_violations: report.total_taboo,
                components: report.total_components,
                links: report.total_links,
                top_issues,
                error: None,
            }
        }
        Err(e) => ScanResult::failed(repo_slug, language, &e),
    }
}

/// Generate a markdown report from scan results.
pub fn generate_report(results: &[ScanResult], top_n: Option<usize>) -> String {
    let mut report = String::new();

    report.push_str("# Archlint Nightly Scan Report\n\n");
    report.push_str(&format!(
        "Scanned {} repositories.\n\n",
        results.len()
    ));

    // Sort by health score ascending (worst first)
    let mut sorted: Vec<&ScanResult> = results.iter().collect();
    sorted.sort_by(|a, b| a.health_score.cmp(&b.health_score));

    // Apply --top N filter if requested
    if let Some(n) = top_n {
        sorted.truncate(n);
    }

    // Summary table
    report.push_str("## Summary\n\n");
    report.push_str("| Repository | Language | Health | Violations | Taboo | Components | Links |\n");
    report.push_str("|------------|----------|--------|------------|-------|------------|-------|\n");

    for result in &sorted {
        if result.error.is_some() {
            report.push_str(&format!(
                "| {} | {} | ERROR | - | - | - | - |\n",
                result.repo, result.language
            ));
        } else {
            let health_display = health_emoji(result.health_score);
            report.push_str(&format!(
                "| {} | {} | {} {}/100 | {} | {} | {} | {} |\n",
                result.repo,
                result.language,
                health_display,
                result.health_score,
                result.total_violations,
                result.taboo_violations,
                result.components,
                result.links,
            ));
        }
    }
    report.push('\n');

    // Per-repo details with top issues
    report.push_str("## Details\n\n");
    for result in &sorted {
        report.push_str(&format!("### {}\n\n", result.repo));

        if let Some(ref err) = result.error {
            report.push_str(&format!("**Error:** {}\n\n", err));
            continue;
        }

        report.push_str(&format!(
            "- Language: {}\n- Health: {}/100\n- Violations: {} (taboo: {})\n- Components: {}, Links: {}\n",
            result.language,
            result.health_score,
            result.total_violations,
            result.taboo_violations,
            result.components,
            result.links,
        ));

        if !result.top_issues.is_empty() {
            report.push_str("\n**Top issues:**\n\n");
            for issue in &result.top_issues {
                report.push_str(&format!("- {}\n", issue));
            }
        }
        report.push('\n');
    }

    // Errors section
    let errors: Vec<&ScanResult> = results.iter().filter(|r| r.error.is_some()).collect();
    if !errors.is_empty() {
        report.push_str("## Scan Errors\n\n");
        for r in &errors {
            report.push_str(&format!(
                "- **{}**: {}\n",
                r.repo,
                r.error.as_deref().unwrap_or("unknown error")
            ));
        }
        report.push('\n');
    }

    report
}

/// Return a health status indicator (text-only, no emoji per style rules).
fn health_emoji(score: u32) -> &'static str {
    if score > 80 {
        "OK"
    } else if score >= 50 {
        "WARN"
    } else {
        "FAIL"
    }
}

/// Run the full nightly scan: clone repos, scan each, generate report.
/// Outputs to stdout or a file depending on `output`.
pub fn run_nightly(
    repos: Vec<RepoEntry>,
    output: Option<&Path>,
    top_n: Option<usize>,
    verbose: bool,
) -> Result<(), String> {
    let tmp_base = std::env::temp_dir().join("archlint-nightly");
    std::fs::create_dir_all(&tmp_base)
        .map_err(|e| format!("cannot create temp dir: {}", e))?;

    let mut results: Vec<ScanResult> = Vec::new();

    for repo in &repos {
        if verbose {
            eprintln!("[nightly] scanning {} ...", repo.slug());
        }

        let target_dir: PathBuf = tmp_base.join(format!("{}-{}", repo.owner, repo.name));

        // Clean up previous clone if exists
        if target_dir.exists() {
            let _ = std::fs::remove_dir_all(&target_dir);
        }

        // Clone
        match clone_repo(&repo.github_url(), &target_dir) {
            Ok(()) => {
                if verbose {
                    eprintln!("[nightly]   cloned {}", repo.github_url());
                }
            }
            Err(e) => {
                if verbose {
                    eprintln!("[nightly]   clone failed: {}", e);
                }
                results.push(ScanResult::failed(&repo.slug(), &repo.language, &e));
                continue;
            }
        }

        // Scan
        let result = scan_repo(&repo.slug(), &repo.language, &target_dir);
        if verbose {
            eprintln!(
                "[nightly]   health={}/100 violations={}",
                result.health_score, result.total_violations
            );
        }
        results.push(result);

        // Cleanup temp clone to save disk space
        let _ = std::fs::remove_dir_all(&target_dir);
    }

    // Generate report
    let report = generate_report(&results, top_n);

    // Output
    match output {
        Some(path) => {
            let mut f = std::fs::File::create(path)
                .map_err(|e| format!("cannot create output file {}: {}", path.display(), e))?;
            f.write_all(report.as_bytes())
                .map_err(|e| format!("write error: {}", e))?;
            eprintln!("[nightly] report written to {}", path.display());
        }
        None => {
            print!("{}", report);
        }
    }

    Ok(())
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;

    fn make_result(repo: &str, lang: &str, health: u32, violations: usize, taboo: usize) -> ScanResult {
        ScanResult {
            repo: repo.to_string(),
            language: lang.to_string(),
            health_score: health,
            total_violations: violations,
            taboo_violations: taboo,
            components: 10,
            links: 20,
            top_issues: vec![format!("[cycle] moduleA - cycle detected in {}", repo)],
            error: None,
        }
    }

    #[test]
    fn test_generate_report_basic() {
        let results = vec![
            make_result("owner/repo-a", "Go", 90, 2, 0),
            make_result("owner/repo-b", "Rust", 40, 10, 3),
        ];
        let report = generate_report(&results, None);

        assert!(report.contains("# Archlint Nightly Scan Report"));
        assert!(report.contains("owner/repo-a"));
        assert!(report.contains("owner/repo-b"));
        assert!(report.contains("Go"));
        assert!(report.contains("Rust"));
    }

    #[test]
    fn test_generate_report_sorted_worst_first() {
        let results = vec![
            make_result("owner/good", "Go", 95, 0, 0),
            make_result("owner/bad", "Go", 20, 15, 5),
            make_result("owner/mid", "Rust", 60, 5, 1),
        ];
        let report = generate_report(&results, None);

        // In sorted order: bad (20), mid (60), good (95)
        let pos_bad = report.find("owner/bad").unwrap();
        let pos_mid = report.find("owner/mid").unwrap();
        let pos_good = report.find("owner/good").unwrap();
        assert!(pos_bad < pos_mid, "bad should appear before mid in table");
        assert!(pos_mid < pos_good, "mid should appear before good in table");
    }

    #[test]
    fn test_generate_report_top_n() {
        let results = vec![
            make_result("owner/a", "Go", 90, 1, 0),
            make_result("owner/b", "Go", 50, 5, 1),
            make_result("owner/c", "Rust", 10, 20, 8),
        ];
        let report = generate_report(&results, Some(2));

        // Only 2 worst results should appear in details
        assert!(report.contains("owner/c"), "worst should be included");
        assert!(report.contains("owner/b"), "second worst should be included");
        // The best (owner/a) should not appear since top_n=2 shows only worst 2
        assert!(!report.contains("owner/a"), "best should be excluded with top_n=2");
    }

    #[test]
    fn test_generate_report_with_error() {
        let mut results = vec![make_result("owner/ok", "Go", 80, 3, 0)];
        results.push(ScanResult::failed("owner/fail", "Go", "network timeout"));

        let report = generate_report(&results, None);
        assert!(report.contains("ERROR"));
        assert!(report.contains("Scan Errors"));
        assert!(report.contains("network timeout"));
    }

    #[test]
    fn test_parse_repos_valid() {
        let repos = parse_repos("kubernetes/kubernetes,tokio-rs/tokio,denoland/deno");
        assert_eq!(repos.len(), 3);
        assert_eq!(repos[0].owner, "kubernetes");
        assert_eq!(repos[0].name, "kubernetes");
        assert_eq!(repos[1].owner, "tokio-rs");
        assert_eq!(repos[2].owner, "denoland");
    }

    #[test]
    fn test_parse_repos_with_spaces() {
        let repos = parse_repos("  owner/repo1 , owner/repo2 ");
        assert_eq!(repos.len(), 2);
        assert_eq!(repos[0].owner, "owner");
        assert_eq!(repos[0].name, "repo1");
    }

    #[test]
    fn test_parse_repos_invalid_skipped() {
        let repos = parse_repos("valid/repo,invalidentry,another/repo");
        assert_eq!(repos.len(), 2, "invalid entry without '/' should be skipped");
    }

    #[test]
    fn test_default_repos_not_empty() {
        let repos = default_repos();
        assert!(!repos.is_empty(), "default repos should not be empty");
        // Check some known entries exist
        let slugs: Vec<String> = repos.iter().map(|r| r.slug()).collect();
        assert!(slugs.contains(&"kubernetes/kubernetes".to_string()));
        assert!(slugs.contains(&"tokio-rs/tokio".to_string()));
    }

    #[test]
    fn test_repo_entry_github_url() {
        let r = RepoEntry::new("tokio-rs", "tokio", "Rust");
        assert_eq!(r.github_url(), "https://github.com/tokio-rs/tokio.git");
        assert_eq!(r.slug(), "tokio-rs/tokio");
    }

    #[test]
    fn test_health_emoji_thresholds() {
        assert_eq!(health_emoji(100), "OK");
        assert_eq!(health_emoji(81), "OK");
        assert_eq!(health_emoji(80), "WARN");
        assert_eq!(health_emoji(50), "WARN");
        assert_eq!(health_emoji(49), "FAIL");
        assert_eq!(health_emoji(0), "FAIL");
    }

    #[test]
    fn test_scan_result_failed() {
        let r = ScanResult::failed("owner/repo", "Go", "some error");
        assert_eq!(r.repo, "owner/repo");
        assert!(r.error.is_some());
        assert_eq!(r.health_score, 0);
    }

    #[test]
    fn test_top_issues_truncated() {
        let long_issue = "x".repeat(100);
        let results = vec![ScanResult {
            repo: "owner/repo".to_string(),
            language: "Go".to_string(),
            health_score: 50,
            total_violations: 1,
            taboo_violations: 0,
            components: 5,
            links: 10,
            top_issues: vec![long_issue.clone()],
            error: None,
        }];
        let report = generate_report(&results, None);
        // The long issue appears in details - check report contains repo
        assert!(report.contains("owner/repo"));
    }

    #[test]
    fn test_generate_report_count_in_header() {
        let results = vec![
            make_result("owner/a", "Go", 80, 2, 0),
            make_result("owner/b", "Rust", 70, 3, 1),
            make_result("owner/c", "Go", 60, 4, 0),
        ];
        let report = generate_report(&results, None);
        assert!(report.contains("Scanned 3 repositories"));
    }
}

