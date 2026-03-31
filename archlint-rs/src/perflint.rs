use regex::Regex;
use serde::{Deserialize, Serialize};
use std::fs;
use std::path::{Path, PathBuf};
use rayon::prelude::*;
use walkdir::WalkDir;

/// Performance analysis result.
#[derive(Debug, Serialize, Deserialize)]
pub struct PerfReport {
    pub files_scanned: usize,
    pub total_functions: usize,
    pub max_cyclomatic_complexity: usize,
    pub max_nesting_depth: usize,
    pub issues: Vec<PerfIssue>,
}

/// Single performance issue found.
#[derive(Debug, Serialize, Deserialize)]
pub struct PerfIssue {
    pub file: String,
    pub line: usize,
    pub kind: String,
    pub message: String,
    pub severity: String,
}

/// Analyze a project for performance issues.
pub fn analyze(dir: &Path) -> PerfReport {
    let files: Vec<PathBuf> = WalkDir::new(dir)
        .into_iter()
        .filter_map(|e| e.ok())
        .filter(|e| {
            let ext = e.path().extension().and_then(|e| e.to_str()).unwrap_or("");
            (ext == "go" || ext == "rs") && !e.path().to_string_lossy().contains("vendor")
                && !e.path().to_string_lossy().contains("target")
        })
        .map(|e| e.path().to_path_buf())
        .collect();

    let results: Vec<FileResult> = files
        .par_iter()
        .filter_map(|path| analyze_file(path, dir).ok())
        .collect();

    let mut report = PerfReport {
        files_scanned: results.len(),
        total_functions: 0,
        max_cyclomatic_complexity: 0,
        max_nesting_depth: 0,
        issues: Vec::new(),
    };

    for fr in results {
        report.total_functions += fr.function_count;
        if fr.max_complexity > report.max_cyclomatic_complexity {
            report.max_cyclomatic_complexity = fr.max_complexity;
        }
        if fr.max_nesting > report.max_nesting_depth {
            report.max_nesting_depth = fr.max_nesting;
        }
        report.issues.extend(fr.issues);
    }

    report
}

struct FileResult {
    function_count: usize,
    max_complexity: usize,
    max_nesting: usize,
    issues: Vec<PerfIssue>,
}

fn analyze_file(path: &Path, base_dir: &Path) -> Result<FileResult, String> {
    let content = fs::read_to_string(path).map_err(|e| e.to_string())?;
    let rel_path = path.strip_prefix(base_dir).unwrap_or(path)
        .to_string_lossy().to_string();

    let ext = path.extension().and_then(|e| e.to_str()).unwrap_or("");

    let mut result = FileResult {
        function_count: 0,
        max_complexity: 0,
        max_nesting: 0,
        issues: Vec::new(),
    };

    // Count functions and measure complexity
    let func_re = match ext {
        "go" => Regex::new(r"func\s+").unwrap(),
        "rs" => Regex::new(r"fn\s+\w+").unwrap(),
        _ => return Ok(result),
    };
    result.function_count = func_re.find_iter(&content).count();

    // Cyclomatic complexity estimation per function
    // Count decision points: if, else if, for, while, match/switch, case, &&, ||
    let decision_re = Regex::new(r"\b(if|else if|elif|for|while|switch|match|case)\b|&&|\|\|").unwrap();
    let total_decisions = decision_re.find_iter(&content).count();
    let _avg_complexity = if result.function_count > 0 {
        total_decisions / result.function_count
    } else {
        0
    };

    // Find high complexity functions (rough: count decisions per function block)
    let mut in_func = false;
    let mut func_decisions = 0;
    let mut func_name = String::new();
    let mut func_line = 0;
    let mut brace_depth = 0;

    for (i, line) in content.lines().enumerate() {
        let trimmed = line.trim();

        if !in_func {
            if let Some(m) = func_re.find(trimmed) {
                in_func = true;
                func_decisions = 0;
                func_name = m.as_str().trim().to_string();
                func_line = i + 1;
                brace_depth = 0;
            }
        }

        if in_func {
            brace_depth += trimmed.chars().filter(|&c| c == '{').count() as i32;
            brace_depth -= trimmed.chars().filter(|&c| c == '}').count() as i32;
            func_decisions += decision_re.find_iter(trimmed).count();

            if brace_depth <= 0 && i > func_line {
                // Function ended
                let complexity = func_decisions + 1;
                if complexity > result.max_complexity {
                    result.max_complexity = complexity;
                }
                if complexity > 10 {
                    result.issues.push(PerfIssue {
                        file: rel_path.clone(),
                        line: func_line,
                        kind: "high_complexity".to_string(),
                        message: format!("{} has cyclomatic complexity {} (limit: 10)", func_name, complexity),
                        severity: if complexity > 20 { "error" } else { "warning" }.to_string(),
                    });
                }
                in_func = false;
            }
        }
    }

    // Nesting depth analysis
    // Only count actual loop constructs (for/while/loop) as nesting depth,
    // not control flow (if/match). This avoids false positives on deeply
    // nested if/match chains which don't have O(n^k) complexity implications.
    let loop_nesting_re = Regex::new(r"^\s*(for|while|loop)\b").unwrap();
    let mut max_nesting: usize = 0;
    // Track nesting using brace depth relative to each loop start
    let mut loop_brace_stack: Vec<i32> = Vec::new(); // brace_depth when loop opened
    let mut brace_depth_nesting: i32 = 0;
    let deep_nesting_threshold = 8; // Raised from 5 to reduce noise
    let mut last_deep_nesting_file: Option<(String, usize)> = None; // dedup per file
    for (i, line) in content.lines().enumerate() {
        let open = line.chars().filter(|&c| c == '{').count() as i32;
        let close = line.chars().filter(|&c| c == '}').count() as i32;

        // Check if this line starts a loop (before counting braces for this line)
        if loop_nesting_re.is_match(line) {
            loop_brace_stack.push(brace_depth_nesting);
        }

        brace_depth_nesting += open - close;

        // Pop loops whose opening brace depth is now above current depth
        loop_brace_stack.retain(|&open_depth| brace_depth_nesting > open_depth);
        let current_nesting = loop_brace_stack.len();

        if current_nesting > max_nesting {
            max_nesting = current_nesting;
        }

        if brace_depth_nesting as usize > deep_nesting_threshold {
            // Dedup: only one warning per file (avoid 714 warnings per file)
            let should_warn = match &last_deep_nesting_file {
                None => true,
                Some((f, _)) => f != &rel_path,
            };
            if should_warn {
                last_deep_nesting_file = Some((rel_path.clone(), i + 1));
                result.issues.push(PerfIssue {
                    file: rel_path.clone(),
                    line: i + 1,
                    kind: "deep_nesting".to_string(),
                    message: format!("nesting depth {} exceeds limit {}", brace_depth_nesting, deep_nesting_threshold),
                    severity: "warning".to_string(),
                });
            }
        }
    }
    result.max_nesting = max_nesting;

    // Allocation in loops
    let alloc_in_loop_re = Regex::new(r"(for|while|loop)\s*.*\{[^}]*\b(append|make|new|vec!|Vec::new|push)\b").unwrap();
    for (i, line) in content.lines().enumerate() {
        if let Some(_m) = alloc_in_loop_re.find(line) {
            result.issues.push(PerfIssue {
                file: rel_path.clone(),
                line: i + 1,
                kind: "allocation_in_loop".to_string(),
                message: "potential allocation inside loop".to_string(),
                severity: "info".to_string(),
            });
        }
    }

    // Nested loops (O(n^2))
    // Only for/while/loop constructs contribute to nested loop depth.
    // if/match are branching, not iteration - they don't create O(n^k) complexity.
    let loop_kw_re = Regex::new(r"^\s*(for|while|loop)\b").unwrap();
    // Stack of brace depths at which each loop was opened
    let mut nested_loop_brace_stack: Vec<i32> = Vec::new();
    let mut nested_brace_depth: i32 = 0;
    for (i, line) in content.lines().enumerate() {
        let open = line.chars().filter(|&c| c == '{').count() as i32;
        let close = line.chars().filter(|&c| c == '}').count() as i32;

        // If this line opens a loop, push current depth before counting this line's braces
        if loop_kw_re.is_match(line) {
            nested_loop_brace_stack.push(nested_brace_depth);
            let depth = nested_loop_brace_stack.len();
            if depth >= 2 {
                result.issues.push(PerfIssue {
                    file: rel_path.clone(),
                    line: i + 1,
                    kind: "nested_loop".to_string(),
                    message: format!("nested loop depth {} - O(n^{}) complexity", depth, depth),
                    severity: "warning".to_string(),
                });
            }
        }

        nested_brace_depth += open - close;

        // Pop any loops whose scope has ended (brace depth has returned to opening level)
        nested_loop_brace_stack.retain(|&open_depth| nested_brace_depth > open_depth);
    }

    Ok(result)
}

/// Format performance report as human-readable string.
pub fn format_report(r: &PerfReport) -> String {
    let mut s = format!(
        "Performance Report:\n  Files scanned: {}\n  Functions: {}\n  Max cyclomatic complexity: {}\n  Max nesting depth: {}\n  Issues: {}\n",
        r.files_scanned, r.total_functions, r.max_cyclomatic_complexity, r.max_nesting_depth, r.issues.len()
    );

    if !r.issues.is_empty() {
        s.push_str("\n  Issues:\n");
        for issue in &r.issues {
            s.push_str(&format!("    [{}] {}:{} - {}\n", issue.severity, issue.file, issue.line, issue.message));
        }
    }

    s
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_simple_code() {
        let dir = std::env::temp_dir().join("perftest");
        let _ = fs::create_dir_all(&dir);
        fs::write(dir.join("simple.go"), "package main\n\nfunc main() {\n\tfmt.Println(\"hello\")\n}\n").unwrap();
        let report = analyze(&dir);
        assert!(report.files_scanned > 0);
        assert_eq!(report.max_cyclomatic_complexity, 1);
        let _ = fs::remove_dir_all(&dir);
    }

    #[test]
    fn test_nested_if_not_counted_as_loop() {
        // Deeply nested if/match should NOT produce nested_loop warnings
        let dir = std::env::temp_dir().join("perftest_if");
        let _ = fs::create_dir_all(&dir);
        let code = r#"
fn agent_handler(x: i32) -> i32 {
    if x > 0 {
        if x > 10 {
            if x > 100 {
                match x {
                    1 => {
                        if x > 5 {
                            if x > 50 {
                                return 1;
                            }
                        }
                    }
                    _ => {}
                }
            }
        }
    }
    0
}
"#;
        fs::write(dir.join("agent.rs"), code).unwrap();
        let report = analyze(&dir);
        let nested_loop_issues: Vec<_> = report.issues.iter()
            .filter(|i| i.kind == "nested_loop")
            .collect();
        assert!(
            nested_loop_issues.is_empty(),
            "nested if/match should not produce nested_loop warnings, got: {:?}",
            nested_loop_issues
        );
        let _ = fs::remove_dir_all(&dir);
    }

    #[test]
    fn test_real_nested_loops_detected() {
        // Actual nested for loops SHOULD produce nested_loop warnings
        let dir = std::env::temp_dir().join("perftest_loops");
        let _ = fs::create_dir_all(&dir);
        let code = r#"
fn matrix_mul(a: &[Vec<i32>], b: &[Vec<i32>]) -> Vec<Vec<i32>> {
    let n = a.len();
    let mut result = vec![vec![0; n]; n];
    for i in 0..n {
        for j in 0..n {
            for k in 0..n {
                result[i][j] += a[i][k] * b[k][j];
            }
        }
    }
    result
}
"#;
        fs::write(dir.join("matrix.rs"), code).unwrap();
        let report = analyze(&dir);
        let nested_loop_issues: Vec<_> = report.issues.iter()
            .filter(|i| i.kind == "nested_loop")
            .collect();
        assert!(
            !nested_loop_issues.is_empty(),
            "nested for loops should produce nested_loop warnings"
        );
        // Should detect O(n^2) and O(n^3)
        let max_depth = nested_loop_issues.iter()
            .map(|i| {
                i.message.split("depth ").nth(1)
                    .and_then(|s| s.split(' ').next())
                    .and_then(|s| s.parse::<usize>().ok())
                    .unwrap_or(0)
            })
            .max()
            .unwrap_or(0);
        assert!(max_depth >= 3, "should detect O(n^3) nesting, got depth {}", max_depth);
        let _ = fs::remove_dir_all(&dir);
    }

    #[test]
    fn test_deep_nesting_dedup() {
        // deep_nesting should produce at most 1 warning per file (dedup)
        let dir = std::env::temp_dir().join("perftest_dedup");
        let _ = fs::create_dir_all(&dir);
        // Create deeply nested code that would previously produce hundreds of warnings
        let mut code = String::from("fn deep() {\n");
        for _ in 0..15 {
            code.push_str("    if true {\n");
        }
        for _ in 0..15 {
            code.push_str("    }\n");
        }
        code.push_str("}\n");
        fs::write(dir.join("deep.rs"), &code).unwrap();
        let report = analyze(&dir);
        let deep_nesting_issues: Vec<_> = report.issues.iter()
            .filter(|i| i.kind == "deep_nesting")
            .collect();
        assert!(
            deep_nesting_issues.len() <= 1,
            "deep_nesting should be deduped to at most 1 warning per file, got {}",
            deep_nesting_issues.len()
        );
        let _ = fs::remove_dir_all(&dir);
    }
}
