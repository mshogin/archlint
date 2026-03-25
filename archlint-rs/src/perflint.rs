use regex::Regex;
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
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
    let avg_complexity = if result.function_count > 0 {
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
    let mut max_nesting: usize = 0;
    let mut current_nesting: usize = 0;
    for (i, line) in content.lines().enumerate() {
        let open = line.chars().filter(|&c| c == '{').count();
        let close = line.chars().filter(|&c| c == '}').count();
        current_nesting = current_nesting.saturating_add(open).saturating_sub(close);
        if current_nesting > max_nesting {
            max_nesting = current_nesting;
        }
        if current_nesting > 5 {
            result.issues.push(PerfIssue {
                file: rel_path.clone(),
                line: i + 1,
                kind: "deep_nesting".to_string(),
                message: format!("nesting depth {} exceeds limit 5", current_nesting),
                severity: "warning".to_string(),
            });
        }
    }
    result.max_nesting = max_nesting;

    // Allocation in loops
    let alloc_in_loop_re = Regex::new(r"(for|while|loop)\s*.*\{[^}]*\b(append|make|new|vec!|Vec::new|push)\b").unwrap();
    for (i, line) in content.lines().enumerate() {
        if let Some(m) = alloc_in_loop_re.find(line) {
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
    let loop_re = Regex::new(r"^\s*(for|while|loop)\b").unwrap();
    let mut loop_depth = 0;
    for (i, line) in content.lines().enumerate() {
        if loop_re.is_match(line) {
            loop_depth += 1;
            if loop_depth >= 2 {
                result.issues.push(PerfIssue {
                    file: rel_path.clone(),
                    line: i + 1,
                    kind: "nested_loop".to_string(),
                    message: format!("nested loop depth {} - O(n^{}) complexity", loop_depth, loop_depth),
                    severity: "warning".to_string(),
                });
            }
        }
        if line.contains('}') && loop_depth > 0 {
            // Simplified: any closing brace might end a loop
            // For accurate analysis, need proper AST parsing
        }
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
}
