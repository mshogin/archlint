use serde::{Deserialize, Serialize};
use std::collections::HashMap;

/// Escalation event: recorded when a quality gate fails and the task must
/// be re-routed to a more expensive model.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct EscalationEvent {
    /// Unix epoch seconds when the escalation occurred.
    pub timestamp: u64,
    /// The model tier that was originally used (e.g. "haiku", "sonnet").
    pub from_model: String,
    /// The model tier the task was escalated to (e.g. "sonnet", "opus").
    pub to_model: String,
    /// Number of violations that triggered the quality gate failure.
    pub violation_count: usize,
    /// Violation rule categories that caused the escalation.
    pub violation_categories: Vec<String>,
    /// Approximate extra cost (USD) incurred by escalating to a more expensive model.
    pub escalation_cost_usd: f64,
}

impl EscalationEvent {
    /// Create an escalation event and compute the cost delta automatically.
    ///
    /// `input_tokens` and `output_tokens` are the estimated token counts for
    /// the re-routed request (the "extra work" caused by the quality gate
    /// rejection).
    pub fn new(
        from_model: &str,
        to_model: &str,
        violation_categories: Vec<String>,
        input_tokens: usize,
        output_tokens: usize,
    ) -> Self {
        let base_cost = estimate(from_model, input_tokens, output_tokens);
        let escalated_cost = estimate(to_model, input_tokens, output_tokens);
        let escalation_cost_usd = (escalated_cost - base_cost).max(0.0);

        let now = std::time::SystemTime::now()
            .duration_since(std::time::UNIX_EPOCH)
            .unwrap_or_default()
            .as_secs();

        Self {
            timestamp: now,
            from_model: from_model.to_string(),
            to_model: to_model.to_string(),
            violation_count: violation_categories.len(),
            violation_categories,
            escalation_cost_usd,
        }
    }
}

/// Aggregate report of escalation activity.
#[derive(Debug, Serialize, Deserialize)]
pub struct EscalationReport {
    /// Total number of escalations recorded.
    pub total_escalations: usize,
    /// Total cost incurred by escalations (USD).
    pub total_escalation_cost_usd: f64,
    /// Per-rule statistics: rule name -> how many times it triggered an escalation.
    pub by_rule: HashMap<String, RuleEscalationStats>,
    /// Per route statistics: "haiku->sonnet", "sonnet->opus", etc.
    pub by_route: HashMap<String, RouteStats>,
    /// The most expensive rule category (by total cost impact).
    pub top_rule_by_cost: Option<String>,
}

/// Per-rule escalation statistics.
#[derive(Debug, Serialize, Deserialize, Default)]
pub struct RuleEscalationStats {
    /// How many escalations this rule appeared in.
    pub escalation_count: usize,
    /// Total estimated cost contribution for this rule.
    pub cost_usd: f64,
}

/// Statistics for a specific escalation route (e.g. haiku -> sonnet).
#[derive(Debug, Serialize, Deserialize, Default)]
pub struct RouteStats {
    pub escalation_count: usize,
    pub total_cost_usd: f64,
}

/// Append an escalation event to a JSONL log file.
///
/// Each line is a JSON-serialized `EscalationEvent`. The file is created if it
/// does not exist.
pub fn log_escalation(event: &EscalationEvent, path: &std::path::Path) {
    use std::io::Write;
    if let Ok(mut f) = std::fs::OpenOptions::new()
        .create(true)
        .append(true)
        .open(path)
    {
        if let Ok(line) = serde_json::to_string(event) {
            let _ = writeln!(f, "{}", line);
        }
    }
}

/// Generate an `EscalationReport` from a JSONL escalation log file.
pub fn escalation_report(content: &str) -> EscalationReport {
    let mut report = EscalationReport {
        total_escalations: 0,
        total_escalation_cost_usd: 0.0,
        by_rule: HashMap::new(),
        by_route: HashMap::new(),
        top_rule_by_cost: None,
    };

    for line in content.lines() {
        let line = line.trim();
        if line.is_empty() {
            continue;
        }
        let event: EscalationEvent = match serde_json::from_str(line) {
            Ok(e) => e,
            Err(_) => continue,
        };

        report.total_escalations += 1;
        report.total_escalation_cost_usd += event.escalation_cost_usd;

        // Per-route stats
        let route = format!("{}->{}", event.from_model, event.to_model);
        let rs = report.by_route.entry(route).or_default();
        rs.escalation_count += 1;
        rs.total_cost_usd += event.escalation_cost_usd;

        // Distribute cost evenly across the violation categories that triggered this event
        let rule_count = event.violation_categories.len().max(1);
        let cost_per_rule = event.escalation_cost_usd / rule_count as f64;

        for rule in &event.violation_categories {
            let stats = report.by_rule.entry(rule.clone()).or_default();
            stats.escalation_count += 1;
            stats.cost_usd += cost_per_rule;
        }
    }

    // Find the most expensive rule
    report.top_rule_by_cost = report
        .by_rule
        .iter()
        .max_by(|a, b| a.1.cost_usd.partial_cmp(&b.1.cost_usd).unwrap_or(std::cmp::Ordering::Equal))
        .map(|(rule, _)| rule.clone());

    report
}

/// Build quality-gate escalation metadata from a scan result.
///
/// Returns a JSON-serializable struct that can be embedded in the quality gate
/// output so downstream tools (costlint, routing systems) can act on it.
#[derive(Debug, Serialize, Deserialize)]
pub struct QualityGateEscalationMeta {
    /// Whether the quality gate passed.
    pub gate_passed: bool,
    /// Total violation count.
    pub violation_count: usize,
    /// Unique violation categories/rules found.
    pub violation_categories: Vec<String>,
    /// Recommended escalation target model (empty if gate passed).
    pub escalate_to: String,
    /// Estimated additional cost of processing this at the escalated model tier.
    pub estimated_escalation_cost_usd: f64,
}

impl QualityGateEscalationMeta {
    /// Build from a list of violation rule names and the current model tier.
    ///
    /// Escalation logic:
    /// - haiku -> sonnet when violations found
    /// - sonnet -> opus when violations found
    /// - opus stays at opus (already at top)
    pub fn from_violations(
        violations: &[String],
        current_model: &str,
        input_tokens: usize,
        output_tokens: usize,
    ) -> Self {
        let gate_passed = violations.is_empty();

        let escalate_to = if gate_passed {
            String::new()
        } else {
            match current_model {
                "haiku" => "sonnet".to_string(),
                "sonnet" => "opus".to_string(),
                _ => current_model.to_string(),
            }
        };

        let estimated_escalation_cost_usd = if gate_passed || escalate_to == current_model {
            0.0
        } else {
            let base = estimate(current_model, input_tokens, output_tokens);
            let escalated = estimate(&escalate_to, input_tokens, output_tokens);
            (escalated - base).max(0.0)
        };

        // Deduplicate categories preserving order
        let mut seen = std::collections::HashSet::new();
        let violation_categories: Vec<String> = violations
            .iter()
            .filter(|v| seen.insert((*v).clone()))
            .cloned()
            .collect();

        Self {
            gate_passed,
            violation_count: violations.len(),
            violation_categories,
            escalate_to,
            estimated_escalation_cost_usd,
        }
    }
}

/// Model pricing (USD per 1M tokens).
#[derive(Debug, Clone)]
pub struct ModelPricing {
    #[allow(dead_code)]
    pub name: String,
    pub input_per_m: f64,
    pub output_per_m: f64,
}

/// Default pricing table.
pub fn default_pricing() -> HashMap<String, ModelPricing> {
    let mut m = HashMap::new();
    m.insert("haiku".to_string(), ModelPricing {
        name: "claude-haiku-4-5".to_string(),
        input_per_m: 0.80,
        output_per_m: 4.00,
    });
    m.insert("sonnet".to_string(), ModelPricing {
        name: "claude-sonnet-4-6".to_string(),
        input_per_m: 3.00,
        output_per_m: 15.00,
    });
    m.insert("opus".to_string(), ModelPricing {
        name: "claude-opus-4-6".to_string(),
        input_per_m: 15.00,
        output_per_m: 75.00,
    });
    m
}

/// Estimate cost in USD.
pub fn estimate(model: &str, input_tokens: usize, output_tokens: usize) -> f64 {
    let pricing = default_pricing();
    if let Some(p) = pricing.get(model) {
        let input_cost = input_tokens as f64 / 1_000_000.0 * p.input_per_m;
        let output_cost = output_tokens as f64 / 1_000_000.0 * p.output_per_m;
        input_cost + output_cost
    } else {
        0.0
    }
}

/// Compare costs across all models.
pub fn compare_models(input_tokens: usize, output_tokens: usize) -> HashMap<String, f64> {
    let pricing = default_pricing();
    pricing.keys()
        .map(|k| (k.clone(), estimate(k, input_tokens, output_tokens)))
        .collect()
}

/// Token count estimate (word-based approximation).
pub fn count_tokens(text: &str) -> usize {
    let words = text.split_whitespace().count();
    ((words as f64) * 1.33) as usize
}

/// Cost report from telemetry records.
#[allow(dead_code)]
#[derive(Debug, Serialize, Deserialize)]
pub struct CostReport {
    pub total_requests: usize,
    pub total_input_tokens: usize,
    pub total_output_tokens: usize,
    pub estimated_cost_usd: f64,
    pub optimal_cost_usd: f64,
    pub savings_pct: f64,
    pub by_model: HashMap<String, ModelStats>,
}

#[allow(dead_code)]
#[derive(Debug, Serialize, Deserialize, Default)]
pub struct ModelStats {
    pub requests: usize,
    pub input_tokens: usize,
    pub output_tokens: usize,
    pub cost_usd: f64,
}

/// Telemetry record.
#[allow(dead_code)]
#[derive(Debug, Deserialize)]
pub struct TelemetryRecord {
    pub routed_to: Option<String>,
    pub input_tokens: Option<usize>,
    pub output_tokens: Option<usize>,
    pub analysis: Option<AnalysisData>,
}

#[allow(dead_code)]
#[derive(Debug, Deserialize)]
pub struct AnalysisData {
    pub words: Option<usize>,
}

/// Generate cost report from JSONL telemetry file content.
#[allow(dead_code)]
pub fn generate_report(content: &str) -> CostReport {
    let mut report = CostReport {
        total_requests: 0,
        total_input_tokens: 0,
        total_output_tokens: 0,
        estimated_cost_usd: 0.0,
        optimal_cost_usd: 0.0,
        savings_pct: 0.0,
        by_model: HashMap::new(),
    };

    for line in content.lines() {
        let line = line.trim();
        if line.is_empty() { continue; }

        let record: TelemetryRecord = match serde_json::from_str(line) {
            Ok(r) => r,
            Err(_) => continue,
        };

        let model = record.routed_to.unwrap_or_else(|| "unknown".to_string());

        // Estimate tokens from words if no explicit count
        let input = record.input_tokens.unwrap_or_else(|| {
            record.analysis
                .as_ref()
                .and_then(|a| a.words)
                .map(|w| ((w as f64) * 1.33) as usize)
                .unwrap_or(0)
        });
        let output = record.output_tokens.unwrap_or(input);

        report.total_requests += 1;
        report.total_input_tokens += input;
        report.total_output_tokens += output;

        let stats = report.by_model.entry(model.clone()).or_default();
        stats.requests += 1;
        stats.input_tokens += input;
        stats.output_tokens += output;
        stats.cost_usd += estimate(&model, input, output);
    }

    // Total cost
    for stats in report.by_model.values() {
        report.estimated_cost_usd += stats.cost_usd;
    }

    // Optimal: route opus requests to sonnet where possible (50%)
    for (model, stats) in &report.by_model {
        if model == "opus" {
            let saveable = stats.requests / 2;
            let kept = stats.requests - saveable;
            report.optimal_cost_usd += estimate("sonnet",
                stats.input_tokens * saveable / stats.requests.max(1),
                stats.output_tokens * saveable / stats.requests.max(1));
            report.optimal_cost_usd += estimate("opus",
                stats.input_tokens * kept / stats.requests.max(1),
                stats.output_tokens * kept / stats.requests.max(1));
        } else {
            report.optimal_cost_usd += stats.cost_usd;
        }
    }

    if report.estimated_cost_usd > 0.0 {
        report.savings_pct = ((report.estimated_cost_usd - report.optimal_cost_usd) / report.estimated_cost_usd) * 100.0;
    }

    report
}

/// Format report as human-readable string.
#[allow(dead_code)]
pub fn format_report(r: &CostReport) -> String {
    let mut s = format!("Cost Report:\n  Total requests: {}\n  Total tokens: {} (in: {} / out: {})\n\n  By model:\n",
        r.total_requests,
        r.total_input_tokens + r.total_output_tokens,
        r.total_input_tokens,
        r.total_output_tokens);

    for (model, stats) in &r.by_model {
        s.push_str(&format!("    {}: {} requests, {}K tokens, ~${:.2}\n",
            model, stats.requests,
            (stats.input_tokens + stats.output_tokens) / 1000,
            stats.cost_usd));
    }

    s.push_str(&format!("\n  Estimated total: ~${:.2}\n", r.estimated_cost_usd));
    s.push_str(&format!("  With optimal routing: ~${:.2} (savings: {:.0}%)\n", r.optimal_cost_usd, r.savings_pct));

    s
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_estimate_haiku() {
        let cost = estimate("haiku", 1_000_000, 1_000_000);
        assert!((cost - 4.80).abs() < 0.01); // 0.80 + 4.00
    }

    #[test]
    fn test_estimate_opus() {
        let cost = estimate("opus", 1_000_000, 1_000_000);
        assert!((cost - 90.0).abs() < 0.01); // 15.00 + 75.00
    }

    #[test]
    fn test_count_tokens() {
        let tokens = count_tokens("Fix the bug in server.go line 42");
        assert!(tokens > 0);
        assert!(tokens < 20);
    }

    // --- Escalation tracking tests ---

    #[test]
    fn test_escalation_event_cost_delta() {
        // haiku -> sonnet escalation should have a positive cost delta
        let event = EscalationEvent::new(
            "haiku",
            "sonnet",
            vec!["fan-out".to_string(), "cycles".to_string()],
            10_000,
            10_000,
        );
        assert_eq!(event.from_model, "haiku");
        assert_eq!(event.to_model, "sonnet");
        assert_eq!(event.violation_count, 2);
        assert!(event.escalation_cost_usd > 0.0, "escalation should cost more than base");
    }

    #[test]
    fn test_escalation_event_same_model_zero_cost() {
        // opus -> opus: cost delta should be 0
        let event = EscalationEvent::new(
            "opus",
            "opus",
            vec!["some-rule".to_string()],
            10_000,
            10_000,
        );
        assert!((event.escalation_cost_usd).abs() < 1e-9);
    }

    #[test]
    fn test_escalation_report_empty() {
        let report = escalation_report("");
        assert_eq!(report.total_escalations, 0);
        assert!((report.total_escalation_cost_usd).abs() < 1e-9);
        assert!(report.top_rule_by_cost.is_none());
    }

    #[test]
    fn test_escalation_report_single_event() {
        let event = EscalationEvent::new(
            "haiku",
            "sonnet",
            vec!["fan-out".to_string()],
            100_000,
            100_000,
        );
        let line = serde_json::to_string(&event).unwrap();
        let report = escalation_report(&line);
        assert_eq!(report.total_escalations, 1);
        assert!(report.total_escalation_cost_usd > 0.0);
        assert_eq!(report.top_rule_by_cost.as_deref(), Some("fan-out"));
        let route_stats = report.by_route.get("haiku->sonnet").unwrap();
        assert_eq!(route_stats.escalation_count, 1);
    }

    #[test]
    fn test_escalation_report_multiple_events() {
        let e1 = EscalationEvent::new(
            "haiku", "sonnet",
            vec!["fan-out".to_string()],
            100_000, 100_000,
        );
        let e2 = EscalationEvent::new(
            "sonnet", "opus",
            vec!["cycles".to_string(), "fan-out".to_string()],
            50_000, 50_000,
        );
        let content = format!(
            "{}\n{}",
            serde_json::to_string(&e1).unwrap(),
            serde_json::to_string(&e2).unwrap(),
        );
        let report = escalation_report(&content);
        assert_eq!(report.total_escalations, 2);
        // fan-out appeared in both events
        let fan_out = report.by_rule.get("fan-out").unwrap();
        assert_eq!(fan_out.escalation_count, 2);
    }

    #[test]
    fn test_quality_gate_meta_gate_passed() {
        let meta = QualityGateEscalationMeta::from_violations(&[], "haiku", 5_000, 5_000);
        assert!(meta.gate_passed);
        assert_eq!(meta.violation_count, 0);
        assert!(meta.escalate_to.is_empty());
        assert!((meta.estimated_escalation_cost_usd).abs() < 1e-9);
    }

    #[test]
    fn test_quality_gate_meta_haiku_escalates_to_sonnet() {
        let violations = vec!["fan-out".to_string(), "cycles".to_string(), "fan-out".to_string()];
        let meta = QualityGateEscalationMeta::from_violations(&violations, "haiku", 10_000, 10_000);
        assert!(!meta.gate_passed);
        assert_eq!(meta.violation_count, 3);
        // deduplication
        assert_eq!(meta.violation_categories.len(), 2);
        assert_eq!(meta.escalate_to, "sonnet");
        assert!(meta.estimated_escalation_cost_usd > 0.0);
    }

    #[test]
    fn test_quality_gate_meta_sonnet_escalates_to_opus() {
        let violations = vec!["cycles".to_string()];
        let meta = QualityGateEscalationMeta::from_violations(&violations, "sonnet", 10_000, 10_000);
        assert_eq!(meta.escalate_to, "opus");
        assert!(meta.estimated_escalation_cost_usd > 0.0);
    }

    #[test]
    fn test_quality_gate_meta_opus_no_escalation() {
        let violations = vec!["cycles".to_string()];
        let meta = QualityGateEscalationMeta::from_violations(&violations, "opus", 10_000, 10_000);
        assert_eq!(meta.escalate_to, "opus");
        // Same model -> 0 extra cost
        assert!((meta.estimated_escalation_cost_usd).abs() < 1e-9);
    }
}
