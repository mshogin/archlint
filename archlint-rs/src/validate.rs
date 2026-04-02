//! validate: analyze external architecture.yaml files without scanning source code.
//!
//! Reads a YAML file in ArchGraph format (components + links) and runs
//! metrics/analysis (fan-out, fan-in, cycles, layer violations), then
//! outputs results in the same format as the scan command.

use crate::config::Config;
use crate::model::{ArchGraph, Component, IndexedGraph, Link, Metrics, Violation};
use serde::{Deserialize, Serialize};
use std::path::Path;

// ---------------------------------------------------------------------------
// YAML import format (matches Go's GraphExport / validate.go)
// ---------------------------------------------------------------------------

/// Top-level YAML graph file produced by archlint collect or hand-crafted.
/// Field names match the ArchGraph model: components/links.
#[derive(Debug, Deserialize)]
pub struct YamlGraph {
    #[serde(default)]
    pub components: Vec<YamlComponent>,
    #[serde(default)]
    pub links: Vec<YamlLink>,
    /// Optional metadata block (may be absent in hand-crafted files).
    #[serde(default)]
    pub metadata: Option<YamlMetadata>,
    /// Optional pre-computed metrics (will be recomputed during validation).
    #[serde(default)]
    pub metrics: Option<serde_json::Value>,
}

/// A component node in the YAML graph.
#[derive(Debug, Deserialize)]
pub struct YamlComponent {
    pub id: String,
    #[serde(default)]
    pub title: String,
    #[serde(default)]
    pub entity: String,
}

/// A dependency link in the YAML graph.
#[derive(Debug, Deserialize)]
pub struct YamlLink {
    pub from: String,
    pub to: String,
    #[serde(default)]
    pub link_type: Option<String>,
    #[serde(default)]
    pub method: Option<String>,
}

/// Optional metadata section in the YAML graph.
#[derive(Debug, Deserialize, Default)]
pub struct YamlMetadata {
    #[serde(default)]
    pub language: String,
    #[serde(default)]
    pub root_dir: String,
    #[serde(default)]
    pub analyzed_at: String,
}

// ---------------------------------------------------------------------------
// Validation result
// ---------------------------------------------------------------------------

/// Result of validating a YAML graph file.
#[derive(Debug, Serialize)]
pub struct ValidateReport {
    pub source: String,
    pub language: String,
    pub root_dir: String,
    pub analyzed_at: String,
    pub components: usize,
    pub links: usize,
    pub max_fan_out: usize,
    pub max_fan_in: usize,
    pub cycles: Vec<Vec<String>>,
    pub violations: Vec<ValidateViolation>,
    pub violation_count: usize,
    pub health: u32,
}

/// A violation entry in the validate report.
#[derive(Debug, Serialize)]
pub struct ValidateViolation {
    pub rule: String,
    pub component: String,
    pub message: String,
    pub severity: String,
    pub level: String,
}

// ---------------------------------------------------------------------------
// Core logic
// ---------------------------------------------------------------------------

/// Read a YAML graph file and run metrics analysis on it.
/// `source` is the display name (file path or "-" for stdin).
pub fn validate_from_str(yaml: &str, source: &str, config: &Config) -> Result<ValidateReport, String> {
    let graph_yaml: YamlGraph = serde_yaml::from_str(yaml)
        .map_err(|e| format!("failed to parse YAML: {}", e))?;

    validate_yaml_graph(&graph_yaml, source, config)
}

/// Validate a parsed YamlGraph.
fn validate_yaml_graph(
    yaml: &YamlGraph,
    source: &str,
    config: &Config,
) -> Result<ValidateReport, String> {
    // Convert to internal ArchGraph representation
    let components: Vec<Component> = yaml
        .components
        .iter()
        .map(|c| Component {
            id: c.id.clone(),
            title: if c.title.is_empty() { c.id.clone() } else { c.title.clone() },
            entity: if c.entity.is_empty() { "unknown".to_string() } else { c.entity.clone() },
        })
        .collect();

    let links: Vec<Link> = yaml
        .links
        .iter()
        .map(|l| Link {
            from: l.from.clone(),
            to: l.to.clone(),
            method: l.method.clone(),
            link_type: l.link_type.clone(),
        })
        .collect();

    // Build IndexedGraph for metrics computation
    let mut indexed = IndexedGraph::new();
    for c in &components {
        indexed.add_node(&c.id);
    }
    for l in &links {
        indexed.add_edge(&l.from, &l.to, l.link_type.as_deref().unwrap_or("depends"));
    }

    // Compute metrics using the same logic as the analyzer
    let metrics = compute_metrics(&indexed, &components, config);

    let violations: Vec<ValidateViolation> = metrics
        .violations
        .iter()
        .map(|v| ValidateViolation {
            rule: v.rule.clone(),
            component: v.component.clone(),
            message: v.message.clone(),
            severity: v.severity.clone(),
            level: v.level.clone(),
        })
        .collect();

    let violation_count = violations.len();
    let health = if violation_count == 0 {
        100u32
    } else {
        let penalty = (violation_count * 5).min(100);
        (100 - penalty) as u32
    };

    let meta = yaml.metadata.as_ref();

    Ok(ValidateReport {
        source: source.to_string(),
        language: meta.map(|m| m.language.as_str()).unwrap_or("").to_string(),
        root_dir: meta.map(|m| m.root_dir.as_str()).unwrap_or("").to_string(),
        analyzed_at: meta.map(|m| m.analyzed_at.as_str()).unwrap_or("").to_string(),
        components: components.len(),
        links: links.len(),
        max_fan_out: metrics.max_fan_out,
        max_fan_in: metrics.max_fan_in,
        cycles: metrics.cycles,
        violations,
        violation_count,
        health,
    })
}

/// Compute fan-out, fan-in, cycle, and layer-violation metrics for a graph.
/// This mirrors analyzer::calculate_metrics but works without ParsedFile data,
/// so ISP/DIP rules (which need source-level trait information) are not applied.
fn compute_metrics(graph: &IndexedGraph, components: &[Component], config: &Config) -> Metrics {
    let mut max_fan_out = 0;
    let mut max_fan_in = 0;
    let mut violations: Vec<Violation> = Vec::new();

    let fan_out_threshold = config.fan_out_threshold();
    let fan_in_threshold = config.fan_in_threshold();

    for comp in components {
        let fo = graph.fan_out(&comp.id);
        let fi = graph.fan_in(&comp.id);

        if fo > max_fan_out {
            max_fan_out = fo;
        }
        if fi > max_fan_in {
            max_fan_in = fi;
        }

        if config.rules.fan_out.enabled
            && fo > fan_out_threshold
            && !config.rules.fan_out.exclude.contains(&comp.id)
        {
            violations.push(Violation {
                rule: "fan_out".to_string(),
                component: comp.id.clone(),
                message: format!("fan-out {} exceeds limit {}", fo, fan_out_threshold),
                severity: "warning".to_string(),
                level: config.rules.fan_out.level.as_str().to_string(),
            });
        }

        if config.rules.fan_in.enabled
            && fi > fan_in_threshold
            && !config.rules.fan_in.exclude.contains(&comp.id)
        {
            violations.push(Violation {
                rule: "fan_in".to_string(),
                component: comp.id.clone(),
                message: format!("fan-in {} exceeds limit {}", fi, fan_in_threshold),
                severity: "info".to_string(),
                level: config.rules.fan_in.level.as_str().to_string(),
            });
        }
    }

    // Cycle detection
    let cycles = if config.rules.cycles.enabled {
        detect_cycles(graph)
    } else {
        Vec::new()
    };

    if config.rules.cycles.enabled && !cycles.is_empty() {
        for cycle in &cycles {
            violations.push(Violation {
                rule: "cycle".to_string(),
                component: cycle.first().cloned().unwrap_or_default(),
                message: format!("cycle detected: {}", cycle.join(" -> ")),
                severity: "error".to_string(),
                level: config.rules.cycles.level.as_str().to_string(),
            });
        }
    }

    // Layer dependency violations (if layers are configured)
    if config.has_layer_rules() {
        for comp in components {
            let from_layer = match config.layer_for_module(&comp.id) {
                Some(l) => l,
                None => continue,
            };

            let allowed: &[String] = config
                .allowed_dependencies
                .get(from_layer)
                .map(|v| v.as_slice())
                .unwrap_or(&[]);

            if let Some(&from_idx) = graph.node_indices.get(&comp.id) {
                let neighbors: Vec<String> = graph
                    .graph
                    .neighbors_directed(from_idx, petgraph::Direction::Outgoing)
                    .map(|idx| graph.graph[idx].clone())
                    .collect();

                for neighbor in neighbors {
                    if let Some(to_layer) = config.layer_for_module(&neighbor) {
                        if !allowed.contains(&to_layer.to_string()) {
                            violations.push(Violation {
                                rule: "layer_deps".to_string(),
                                component: comp.id.clone(),
                                message: format!(
                                    "forbidden dependency: {} ({}) -> {} ({})",
                                    comp.id, from_layer, neighbor, to_layer
                                ),
                                severity: "error".to_string(),
                                level: "taboo".to_string(),
                            });
                        }
                    }
                }
            }
        }
    }

    Metrics {
        component_count: components.len(),
        link_count: graph.graph.edge_count(),
        max_fan_out,
        max_fan_in,
        cycles,
        violations,
    }
}

/// Detect cycles in the graph using DFS-based SCC detection (Tarjan-like via petgraph).
/// Returns a list of cycles, each represented as a list of component IDs.
fn detect_cycles(graph: &IndexedGraph) -> Vec<Vec<String>> {
    use petgraph::algo::tarjan_scc;

    let sccs = tarjan_scc(&graph.graph);
    let mut cycles = Vec::new();

    for scc in sccs {
        if scc.len() > 1 {
            // Multi-node SCC: genuine cycle
            let ids: Vec<String> = scc
                .iter()
                .map(|&idx| graph.graph[idx].clone())
                .collect();
            cycles.push(ids);
        } else if scc.len() == 1 {
            // Self-loop check
            let idx = scc[0];
            if graph.graph.contains_edge(idx, idx) {
                cycles.push(vec![graph.graph[idx].clone()]);
            }
        }
    }

    cycles
}

/// Format a ValidateReport as human-readable text (mirrors scan text output).
pub fn format_text(report: &ValidateReport) -> String {
    let mut out = String::new();

    out.push_str(&format!("source:      {}\n", report.source));
    if !report.language.is_empty() {
        out.push_str(&format!("language:    {}\n", report.language));
    }
    if !report.root_dir.is_empty() {
        out.push_str(&format!("root_dir:    {}\n", report.root_dir));
    }
    if !report.analyzed_at.is_empty() {
        out.push_str(&format!("analyzed_at: {}\n", report.analyzed_at));
    }
    out.push_str(&format!("components:  {}\n", report.components));
    out.push_str(&format!("links:       {}\n", report.links));
    out.push_str(&format!("max_fan_out: {}\n", report.max_fan_out));
    out.push_str(&format!("max_fan_in:  {}\n", report.max_fan_in));
    out.push_str(&format!("cycles:      {}\n", report.cycles.len()));
    out.push_str(&format!("violations:  {}\n", report.violation_count));
    out.push_str(&format!("health:      {}/100\n", report.health));

    if !report.violations.is_empty() {
        out.push('\n');

        let taboo: Vec<&ValidateViolation> = report.violations.iter().filter(|v| v.level == "taboo").collect();
        let telemetry: Vec<&ValidateViolation> = report.violations.iter().filter(|v| v.level == "telemetry").collect();
        let personal: Vec<&ValidateViolation> = report.violations.iter().filter(|v| v.level == "personal").collect();

        if !taboo.is_empty() {
            out.push_str("[TABOO - CI BLOCKER]\n");
            for v in &taboo {
                out.push_str(&format!("  [{}] {} - {}\n", v.rule, v.component, v.message));
            }
        }
        if !telemetry.is_empty() {
            out.push_str("[TELEMETRY - track only]\n");
            for v in &telemetry {
                out.push_str(&format!("  [{}] {} - {}\n", v.rule, v.component, v.message));
            }
        }
        if !personal.is_empty() {
            out.push_str("[PERSONAL - informational]\n");
            for v in &personal {
                out.push_str(&format!("  [{}] {} - {}\n", v.rule, v.component, v.message));
            }
        }
    }

    out
}

// ---------------------------------------------------------------------------
// Unit tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;
    use crate::config::Config;

    fn default_config() -> Config {
        Config::default()
    }

    fn make_yaml(components: &[(&str, &str)], links: &[(&str, &str)]) -> String {
        let mut yaml = String::from("components:\n");
        for (id, entity) in components {
            yaml.push_str(&format!("  - id: {}\n    entity: {}\n", id, entity));
        }
        yaml.push_str("links:\n");
        for (from, to) in links {
            yaml.push_str(&format!("  - from: {}\n    to: {}\n", from, to));
        }
        yaml
    }

    #[test]
    fn test_validate_empty_graph() {
        let yaml = "components: []\nlinks: []\n";
        let config = default_config();
        let report = validate_from_str(yaml, "test.yaml", &config).unwrap();
        assert_eq!(report.components, 0);
        assert_eq!(report.links, 0);
        assert_eq!(report.violation_count, 0);
        assert_eq!(report.health, 100);
    }

    #[test]
    fn test_validate_simple_graph_no_violations() {
        let yaml = make_yaml(
            &[("api", "go"), ("service", "go"), ("model", "go")],
            &[("api", "service"), ("service", "model")],
        );
        let config = default_config();
        let report = validate_from_str(&yaml, "arch.yaml", &config).unwrap();
        assert_eq!(report.components, 3);
        assert_eq!(report.links, 2);
        assert_eq!(report.violation_count, 0);
        assert_eq!(report.health, 100);
        assert_eq!(report.max_fan_out, 1);
        assert_eq!(report.max_fan_in, 1);
    }

    #[test]
    fn test_validate_fan_out_violation() {
        // handler depends on 7 modules -> exceeds default threshold (5)
        let yaml = make_yaml(
            &[
                ("handler", "go"),
                ("svc1", "go"), ("svc2", "go"), ("svc3", "go"),
                ("svc4", "go"), ("svc5", "go"), ("svc6", "go"),
            ],
            &[
                ("handler", "svc1"), ("handler", "svc2"), ("handler", "svc3"),
                ("handler", "svc4"), ("handler", "svc5"), ("handler", "svc6"),
            ],
        );
        let config = default_config();
        let report = validate_from_str(&yaml, "arch.yaml", &config).unwrap();
        assert!(report.violation_count > 0, "expected fan_out violation");
        let has_fan_out = report.violations.iter().any(|v| v.rule == "fan_out");
        assert!(has_fan_out, "expected fan_out rule in violations");
        assert_eq!(report.max_fan_out, 6);
    }

    #[test]
    fn test_validate_cycle_detection() {
        // a -> b -> c -> a forms a cycle
        let yaml = make_yaml(
            &[("a", "go"), ("b", "go"), ("c", "go")],
            &[("a", "b"), ("b", "c"), ("c", "a")],
        );
        let config = default_config();
        let report = validate_from_str(&yaml, "arch.yaml", &config).unwrap();
        assert!(!report.cycles.is_empty(), "expected cycle to be detected");
        let has_cycle_violation = report.violations.iter().any(|v| v.rule == "cycle");
        assert!(has_cycle_violation, "expected cycle violation");
    }

    #[test]
    fn test_validate_with_metadata() {
        let yaml = r#"
metadata:
  language: Go
  root_dir: /my/project
  analyzed_at: 2026-01-01T00:00:00Z
components:
  - id: main
    entity: go
links: []
"#;
        let config = default_config();
        let report = validate_from_str(yaml, "arch.yaml", &config).unwrap();
        assert_eq!(report.language, "Go");
        assert_eq!(report.root_dir, "/my/project");
        assert_eq!(report.components, 1);
    }

    #[test]
    fn test_validate_invalid_yaml_returns_error() {
        let yaml = "this: is: not: valid: yaml: ::::";
        let config = default_config();
        let result = validate_from_str(yaml, "bad.yaml", &config);
        assert!(result.is_err(), "expected error for invalid YAML");
    }

    #[test]
    fn test_format_text_no_violations() {
        let report = ValidateReport {
            source: "arch.yaml".to_string(),
            language: "Go".to_string(),
            root_dir: "/project".to_string(),
            analyzed_at: "2026-01-01T00:00:00Z".to_string(),
            components: 3,
            links: 2,
            max_fan_out: 1,
            max_fan_in: 1,
            cycles: vec![],
            violations: vec![],
            violation_count: 0,
            health: 100,
        };
        let text = format_text(&report);
        assert!(text.contains("components:  3"));
        assert!(text.contains("health:      100/100"));
        assert!(text.contains("violations:  0"));
        assert!(!text.contains("TABOO"));
    }

    #[test]
    fn test_format_text_with_violations() {
        let report = ValidateReport {
            source: "arch.yaml".to_string(),
            language: String::new(),
            root_dir: String::new(),
            analyzed_at: String::new(),
            components: 2,
            links: 6,
            max_fan_out: 6,
            max_fan_in: 0,
            cycles: vec![],
            violations: vec![ValidateViolation {
                rule: "fan_out".to_string(),
                component: "handler".to_string(),
                message: "fan-out 6 exceeds limit 5".to_string(),
                severity: "warning".to_string(),
                level: "telemetry".to_string(),
            }],
            violation_count: 1,
            health: 95,
        };
        let text = format_text(&report);
        assert!(text.contains("violations:  1"));
        assert!(text.contains("TELEMETRY"));
        assert!(text.contains("fan_out"));
    }

    #[test]
    fn test_validate_no_cycles_in_dag() {
        // a -> b -> c: no cycle
        let yaml = make_yaml(
            &[("a", "go"), ("b", "go"), ("c", "go")],
            &[("a", "b"), ("b", "c")],
        );
        let config = default_config();
        let report = validate_from_str(&yaml, "arch.yaml", &config).unwrap();
        assert!(report.cycles.is_empty(), "expected no cycles in DAG");
        let no_cycle_viol = report.violations.iter().all(|v| v.rule != "cycle");
        assert!(no_cycle_viol, "expected no cycle violations in DAG");
    }
}
