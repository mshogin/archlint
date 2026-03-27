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
}
