use regex::Regex;
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::io::Write;
use std::path::Path;

/// Result of prompt analysis.
#[derive(Debug, Serialize, Deserialize)]
pub struct PromptAnalysis {
    pub length: usize,
    pub words: usize,
    pub sentences: usize,
    pub has_code_block: bool,
    pub has_code_ref: bool,
    pub questions: usize,
    pub action: String,
    pub domain: HashMap<String, f64>,
    pub complexity: String,
    pub suggested_model: String,
}

/// Analyze a prompt string and return metrics + routing suggestion.
pub fn analyze(prompt: &str) -> PromptAnalysis {
    let words = count_words(prompt);
    let sentences = count_sentences(prompt);
    let questions = count_questions(prompt);
    let has_code_block = prompt.contains("```");
    let has_code_ref = has_code_reference(prompt);
    let action = detect_action(prompt);
    let domain = classify_domain(prompt);
    let complexity = classify_complexity(words, sentences, questions, has_code_block, &action, &domain);
    let suggested_model = suggest_model(&complexity);

    PromptAnalysis {
        length: prompt.len(),
        words,
        sentences,
        has_code_block,
        has_code_ref,
        questions,
        action,
        domain,
        complexity,
        suggested_model,
    }
}

fn count_words(s: &str) -> usize {
    s.split_whitespace().count()
}

fn count_sentences(s: &str) -> usize {
    let count = s.chars().filter(|&c| c == '.' || c == '!' || c == '?').count();
    if count == 0 && !s.is_empty() { 1 } else { count }
}

fn count_questions(s: &str) -> usize {
    s.chars().filter(|&c| c == '?').count()
}

fn has_code_reference(s: &str) -> bool {
    let re = Regex::new(r"\w+\.\w+:\d+|\w+\.(go|rs|py|js|ts|java|cpp|c|rb)").unwrap();
    re.is_match(s)
}

static ACTION_VERBS: &[(&str, &str)] = &[
    ("fix", "fix"), ("repair", "fix"), ("debug", "fix"),
    ("create", "create"), ("add", "create"), ("implement", "create"),
    ("build", "create"), ("write", "create"), ("design", "create"),
    ("architect", "create"), ("plan", "create"), ("propose", "create"),
    ("review", "review"), ("check", "review"), ("analyze", "review"),
    ("refactor", "refactor"), ("rewrite", "refactor"), ("restructure", "refactor"),
    ("migrate", "refactor"), ("optimize", "refactor"), ("improve", "refactor"),
    ("delete", "delete"), ("remove", "delete"),
    ("deploy", "deploy"),
    ("explain", "explain"), ("describe", "explain"),
];

fn detect_action(text: &str) -> String {
    let lower = text.to_lowercase();
    for word in lower.split_whitespace() {
        let clean: String = word.chars().filter(|c| c.is_alphanumeric()).collect();
        for &(verb, action) in ACTION_VERBS {
            if clean == verb {
                return action.to_string();
            }
        }
    }
    "unknown".to_string()
}

static DOMAIN_KEYWORDS: &[(&str, &[&str])] = &[
    ("code", &[
        "function", "method", "variable", "class", "struct", "interface",
        "loop", "array", "string", "int", "bool", "error", "return",
        "import", "package", "module", "test", "unittest", "assert",
    ]),
    ("architecture", &[
        "architecture", "design", "pattern", "solid", "dip", "srp",
        "coupling", "cohesion", "dependency", "layer", "boundary",
        "component", "service", "microservice", "monolith", "graph",
        "cycle", "fan-out", "fan-in", "metric", "cqrs", "event sourcing",
        "saga", "domain driven", "hexagonal", "clean architecture",
        "event-driven", "distributed", "scalab", "resilien",
        "load balanc", "api gateway", "circuit breaker", "retry",
        "dead letter", "idempoten",
    ]),
    ("infrastructure", &[
        "docker", "kubernetes", "k8s", "nginx", "deploy", "ci", "cd",
        "pipeline", "server", "vps", "ssh", "container", "pod",
        "helm", "terraform", "ansible",
    ]),
    ("content", &[
        "article", "post", "blog", "linkedin", "twitter", "write",
        "publish", "draft", "headline", "summary", "translate",
    ]),
];

fn classify_domain(text: &str) -> HashMap<String, f64> {
    let lower = text.to_lowercase();
    let mut result = HashMap::new();

    for &(domain, keywords) in DOMAIN_KEYWORDS {
        let count: usize = keywords.iter()
            .filter(|&&kw| lower.contains(kw))
            .count();
        if count > 0 {
            let score = (count as f64 / 5.0).min(1.0);
            result.insert(domain.to_string(), score);
        }
    }

    if result.is_empty() {
        result.insert("general".to_string(), 1.0);
    }

    result
}

fn classify_complexity(
    words: usize,
    sentences: usize,
    questions: usize,
    has_code_block: bool,
    action: &str,
    domain: &HashMap<String, f64>,
) -> String {
    let mut score = 0;

    // Length
    if words > 200 { score += 2; }
    else if words > 50 { score += 1; }

    // Sentences
    if sentences > 5 { score += 1; }

    // Questions
    if questions > 2 { score += 1; }

    // Code
    if has_code_block { score += 1; }

    // Multiple domains
    let active_domains = domain.values().filter(|&&v| v > 0.3).count();
    if active_domains > 2 { score += 2; }
    else if active_domains == 2 { score += 1; }

    // Architecture domain boost (even 2 keywords = complex topic)
    if let Some(&arch_score) = domain.get("architecture") {
        if arch_score > 0.3 { score += 2; }
    }

    // Action type weight
    match action {
        "create" => score += 1,
        "refactor" => score += 2,
        _ => {}
    }

    // High domain keyword density
    let max_domain = domain.values().cloned().fold(0.0_f64, f64::max);
    if max_domain >= 0.8 { score += 1; }

    match score {
        0..=1 => "low".to_string(),
        2..=3 => "medium".to_string(),
        _ => "high".to_string(),
    }
}

fn suggest_model(complexity: &str) -> String {
    match complexity {
        "high" => "opus".to_string(),
        "medium" => "sonnet".to_string(),
        _ => "haiku".to_string(),
    }
}

/// Telemetry record appended to a JSONL log file.
#[derive(Debug, Serialize, Deserialize)]
pub struct TelemetryRecord {
    pub ts: u64,
    pub complexity: String,
    pub suggested_model: String,
    pub action: String,
    pub words: usize,
    pub domains: Vec<String>,
    /// Actual model used (filled in by caller; None = not tracked yet).
    pub actual_model: Option<String>,
}

impl TelemetryRecord {
    pub fn from_analysis(analysis: &PromptAnalysis) -> Self {
        let ts = std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .map(|d| d.as_secs())
            .unwrap_or(0);

        let domains: Vec<String> = analysis.domain.keys().cloned().collect();

        TelemetryRecord {
            ts,
            complexity: analysis.complexity.clone(),
            suggested_model: analysis.suggested_model.clone(),
            action: analysis.action.clone(),
            words: analysis.words,
            domains,
            actual_model: None,
        }
    }
}

/// Append a telemetry record to `log_path` (JSONL format, one record per line).
/// Creates the file if it does not exist. Does not panic on I/O errors -
/// failures are printed to stderr and silently ignored so the main pipeline
/// is never blocked by telemetry.
pub fn log_telemetry(record: &TelemetryRecord, log_path: &Path) {
    let line = match serde_json::to_string(record) {
        Ok(s) => s,
        Err(e) => {
            eprintln!("archlint telemetry: serialization error: {}", e);
            return;
        }
    };

    let mut file = match std::fs::OpenOptions::new()
        .create(true)
        .append(true)
        .open(log_path)
    {
        Ok(f) => f,
        Err(e) => {
            eprintln!("archlint telemetry: could not open {}: {}", log_path.display(), e);
            return;
        }
    };

    if let Err(e) = writeln!(file, "{}", line) {
        eprintln!("archlint telemetry: write error: {}", e);
    }
}

/// Read and summarize telemetry from a JSONL log file.
/// Returns (total, by_model, by_complexity) counts.
pub fn summarize_telemetry(log_path: &Path) -> Option<TelemetrySummary> {
    let content = std::fs::read_to_string(log_path).ok()?;

    let mut total = 0usize;
    let mut by_model: HashMap<String, usize> = HashMap::new();
    let mut by_complexity: HashMap<String, usize> = HashMap::new();
    let mut routing_accuracy: Option<RoutingAccuracy> = None;
    let mut matched = 0usize;
    let mut compared = 0usize;

    for line in content.lines() {
        let line = line.trim();
        if line.is_empty() {
            continue;
        }
        if let Ok(rec) = serde_json::from_str::<TelemetryRecord>(line) {
            total += 1;
            *by_model.entry(rec.suggested_model.clone()).or_insert(0) += 1;
            *by_complexity.entry(rec.complexity.clone()).or_insert(0) += 1;

            if let Some(actual) = &rec.actual_model {
                compared += 1;
                if actual == &rec.suggested_model {
                    matched += 1;
                }
            }
        }
    }

    if compared > 0 {
        routing_accuracy = Some(RoutingAccuracy {
            compared,
            matched,
            accuracy: matched as f64 / compared as f64,
        });
    }

    Some(TelemetrySummary {
        total,
        by_model,
        by_complexity,
        routing_accuracy,
    })
}

#[derive(Debug, Serialize, Deserialize)]
pub struct RoutingAccuracy {
    pub compared: usize,
    pub matched: usize,
    pub accuracy: f64,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct TelemetrySummary {
    pub total: usize,
    pub by_model: HashMap<String, usize>,
    pub by_complexity: HashMap<String, usize>,
    pub routing_accuracy: Option<RoutingAccuracy>,
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_simple_fix() {
        let result = analyze("Fix typo in README");
        assert_eq!(result.complexity, "low");
        assert_eq!(result.suggested_model, "haiku");
        assert_eq!(result.action, "fix");
    }

    #[test]
    fn test_architecture_design() {
        let result = analyze("Design a microservices architecture with CQRS and event sourcing");
        assert_eq!(result.complexity, "high");
        assert_eq!(result.suggested_model, "opus");
        assert_eq!(result.action, "create");
    }

    #[test]
    fn test_refactor() {
        let result = analyze("Refactor auth module to hexagonal architecture with clean separation");
        assert_eq!(result.complexity, "high");
        assert_eq!(result.suggested_model, "opus");
        assert_eq!(result.action, "refactor");
    }

    #[test]
    fn test_telemetry_record_from_analysis() {
        let analysis = analyze("Fix typo in README");
        let record = TelemetryRecord::from_analysis(&analysis);
        assert_eq!(record.complexity, "low");
        assert_eq!(record.suggested_model, "haiku");
        assert_eq!(record.action, "fix");
        assert!(record.actual_model.is_none());
        assert!(record.ts > 0);
    }

    #[test]
    fn test_log_and_summarize_telemetry() {
        let dir = tempfile::tempdir().unwrap();
        let log_path = dir.path().join("telemetry.jsonl");

        // Log a haiku record (low complexity)
        let r1 = TelemetryRecord::from_analysis(&analyze("Fix typo"));
        log_telemetry(&r1, &log_path);

        // Log an opus record (high complexity architecture)
        let r2 = TelemetryRecord::from_analysis(
            &analyze("Design a microservices architecture with CQRS and event sourcing"),
        );
        log_telemetry(&r2, &log_path);

        // Log a record with actual_model matching suggestion
        let mut r3 = TelemetryRecord::from_analysis(&analyze("Fix typo"));
        r3.actual_model = Some("haiku".to_string());
        log_telemetry(&r3, &log_path);

        let summary = summarize_telemetry(&log_path).expect("summary should exist");
        assert_eq!(summary.total, 3);
        assert!(summary.by_model.contains_key("haiku") || summary.by_model.contains_key("opus"));

        // Routing accuracy should be 100% (1 matched out of 1 compared)
        let acc = summary.routing_accuracy.expect("accuracy should be computed");
        assert_eq!(acc.compared, 1);
        assert_eq!(acc.matched, 1);
        assert!((acc.accuracy - 1.0).abs() < 1e-6);
    }

    #[test]
    fn test_summarize_telemetry_missing_file() {
        let result = summarize_telemetry(Path::new("/nonexistent/path/telemetry.jsonl"));
        assert!(result.is_none());
    }
}
