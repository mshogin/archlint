use serde::{Deserialize, Serialize};
use std::collections::HashMap;

/// Model pricing (USD per 1M tokens).
#[derive(Debug, Clone)]
pub struct ModelPricing {
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

#[derive(Debug, Serialize, Deserialize, Default)]
pub struct ModelStats {
    pub requests: usize,
    pub input_tokens: usize,
    pub output_tokens: usize,
    pub cost_usd: f64,
}

/// Telemetry record.
#[derive(Debug, Deserialize)]
pub struct TelemetryRecord {
    pub routed_to: Option<String>,
    pub input_tokens: Option<usize>,
    pub output_tokens: Option<usize>,
    pub analysis: Option<AnalysisData>,
}

#[derive(Debug, Deserialize)]
pub struct AnalysisData {
    pub words: Option<usize>,
}

/// Generate cost report from JSONL telemetry file content.
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
}
