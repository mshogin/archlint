//! Auto-fix suggestions for architecture violations.
//!
//! This module analyses an [`ArchGraph`] and produces human-readable fix
//! suggestions for each detected violation.  It does **not** modify any
//! files; all output is text-only so a human (or a downstream worker) can
//! decide what to apply.

use crate::model::{ArchGraph, Violation};
use serde::{Deserialize, Serialize};

/// A single fix suggestion attached to a violation.
#[derive(Debug, Clone, Serialize, Deserialize, PartialEq)]
pub struct FixSuggestion {
    /// The rule that produced the violation (e.g. "fan_out", "cycle").
    pub rule: String,
    /// The component / module where the violation was detected.
    pub component: String,
    /// Human-readable description of the original violation.
    pub violation: String,
    /// Suggested action to resolve the violation.
    pub suggestion: String,
    /// Specific items to act on (e.g. imports to remove, modules to extract).
    #[serde(skip_serializing_if = "Vec::is_empty")]
    pub action_items: Vec<String>,
}

/// Full fix report for a project scan.
#[derive(Debug, Serialize, Deserialize)]
pub struct FixReport {
    /// Total number of violations analysed.
    pub total_violations: usize,
    /// Number of violations for which a fix suggestion was produced.
    pub fixable: usize,
    /// Individual fix suggestions.
    pub suggestions: Vec<FixSuggestion>,
}

/// Generate fix suggestions from an [`ArchGraph`] produced by `analyzer::analyze`.
pub fn suggest_fixes(graph: &ArchGraph) -> FixReport {
    let metrics = match &graph.metrics {
        Some(m) => m,
        None => {
            return FixReport {
                total_violations: 0,
                fixable: 0,
                suggestions: Vec::new(),
            }
        }
    };

    let mut suggestions: Vec<FixSuggestion> = Vec::new();

    // Build a lookup: component -> outgoing dependencies (from links).
    let mut outgoing: std::collections::HashMap<&str, Vec<&str>> = std::collections::HashMap::new();
    for link in &graph.links {
        outgoing.entry(link.from.as_str()).or_default().push(link.to.as_str());
    }

    for violation in &metrics.violations {
        if let Some(s) = suggest_for_violation(violation, &outgoing, &metrics.cycles) {
            suggestions.push(s);
        }
    }

    // Cycle violations are stored separately in metrics.cycles, not in metrics.violations.
    // Generate fix suggestions for each detected cycle.
    for cycle in &metrics.cycles {
        if let Some(s) = suggest_for_cycle(cycle) {
            suggestions.push(s);
        }
    }

    let fixable = suggestions.len();
    FixReport {
        total_violations: metrics.violations.len() + metrics.cycles.len(),
        fixable,
        suggestions,
    }
}

/// Produce a fix suggestion for a single violation record.
fn suggest_for_violation(
    violation: &Violation,
    outgoing: &std::collections::HashMap<&str, Vec<&str>>,
    _cycles: &[Vec<String>],
) -> Option<FixSuggestion> {
    match violation.rule.as_str() {
        "fan_out" => suggest_fan_out_fix(violation, outgoing),
        "fan_in" => suggest_fan_in_fix(violation),
        "isp" => suggest_isp_fix(violation),
        "dip" => suggest_dip_fix(violation),
        _ => None,
    }
}

// ---------------------------------------------------------------------------
// Fan-out: too many outgoing dependencies
// ---------------------------------------------------------------------------

fn suggest_fan_out_fix(
    violation: &Violation,
    outgoing: &std::collections::HashMap<&str, Vec<&str>>,
) -> Option<FixSuggestion> {
    let deps: Vec<String> = outgoing
        .get(violation.component.as_str())
        .map(|v| v.iter().map(|s| s.to_string()).collect())
        .unwrap_or_default();

    // Heuristic: group deps by common prefix to suggest which ones to extract.
    let groups = group_by_prefix(&deps);
    let mut action_items: Vec<String> = Vec::new();

    if groups.len() > 1 {
        // Multiple groups -> suggest extracting each group into a facade module.
        for (prefix, members) in &groups {
            if members.len() > 1 {
                action_items.push(format!(
                    "extract [{deps}] into a '{prefix}_facade' module",
                    deps = members.join(", "),
                    prefix = prefix,
                ));
            }
        }
    }

    if action_items.is_empty() {
        // No clear grouping: suggest introducing an interface per dependency.
        for dep in &deps {
            action_items.push(format!(
                "introduce an interface (trait) for '{dep}' and depend on the abstraction"
            ));
        }
    }

    Some(FixSuggestion {
        rule: "fan_out".to_string(),
        component: violation.component.clone(),
        violation: violation.message.clone(),
        suggestion: format!(
            "Reduce outgoing dependencies in '{}'. \
             Consider splitting into smaller modules or introducing facade / interface layers.",
            violation.component
        ),
        action_items,
    })
}

/// Group a list of module names by their first path segment (e.g. `src::foo::bar` -> `src`).
fn group_by_prefix(names: &[String]) -> std::collections::HashMap<String, Vec<String>> {
    let mut map: std::collections::HashMap<String, Vec<String>> = std::collections::HashMap::new();
    for name in names {
        let prefix = name.split("::").next().unwrap_or(name.as_str()).to_string();
        map.entry(prefix).or_default().push(name.clone());
    }
    map
}

// ---------------------------------------------------------------------------
// Fan-in: too many incoming dependencies (high coupling)
// ---------------------------------------------------------------------------

fn suggest_fan_in_fix(violation: &Violation) -> Option<FixSuggestion> {
    Some(FixSuggestion {
        rule: "fan_in".to_string(),
        component: violation.component.clone(),
        violation: violation.message.clone(),
        suggestion: format!(
            "'{}' is depended upon by many callers. \
             Consider extracting a stable public API (trait/interface) so callers depend on the \
             abstraction rather than the concrete module.",
            violation.component
        ),
        action_items: vec![
            format!(
                "define a public trait in a new module (e.g. '{}_api') that captures the surface used by callers",
                last_segment(&violation.component)
            ),
            format!(
                "move concrete implementation of '{}' behind the trait",
                violation.component
            ),
        ],
    })
}

// ---------------------------------------------------------------------------
// ISP: trait has too many methods
// ---------------------------------------------------------------------------

fn suggest_isp_fix(violation: &Violation) -> Option<FixSuggestion> {
    // Extract trait name from message: "trait `Foo` has N methods ..."
    let trait_name = extract_trait_name(&violation.message).unwrap_or("this trait");

    Some(FixSuggestion {
        rule: "isp".to_string(),
        component: violation.component.clone(),
        violation: violation.message.clone(),
        suggestion: format!(
            "Split '{trait_name}' in '{}' into smaller role-based traits (ISP).",
            violation.component
        ),
        action_items: vec![
            format!(
                "identify logical groups of methods in '{trait_name}' (e.g. read vs write, query vs command)"
            ),
            format!("create one focused trait per group (e.g. '{trait_name}Reader', '{trait_name}Writer')"),
            format!("update implementors to implement the individual traits"),
            format!("update call-sites to depend only on the trait they actually need"),
        ],
    })
}

fn extract_trait_name(message: &str) -> Option<&str> {
    // Message format: "trait `Foo` has N methods..."
    let start = message.find('`')? + 1;
    let end = message[start..].find('`')? + start;
    Some(&message[start..end])
}

// ---------------------------------------------------------------------------
// DIP: module has structs but no trait definitions
// ---------------------------------------------------------------------------

fn suggest_dip_fix(violation: &Violation) -> Option<FixSuggestion> {
    let mod_name = last_segment(&violation.component);

    Some(FixSuggestion {
        rule: "dip".to_string(),
        component: violation.component.clone(),
        violation: violation.message.clone(),
        suggestion: format!(
            "'{}' exposes concrete structs without traits. \
             Introduce trait abstractions so callers depend on behaviour, not implementation (DIP).",
            violation.component
        ),
        action_items: vec![
            format!("add a trait (e.g. `{mod_name}Service` or `{mod_name}Store`) that describes the public contract"),
            "move the concrete struct(s) to an `impl` block for that trait".to_string(),
            "update direct struct references in callers to use the trait type".to_string(),
        ],
    })
}

// ---------------------------------------------------------------------------
// Cycle violations (stored separately in metrics.cycles)
// ---------------------------------------------------------------------------

/// Suggest how to break a dependency cycle.
fn suggest_for_cycle(cycle: &[String]) -> Option<FixSuggestion> {
    if cycle.len() < 2 {
        return None;
    }

    // The weakest link heuristic: suggest breaking the last -> first edge,
    // i.e. extract a shared abstraction that both modules can depend on.
    let first = &cycle[0];
    let last = cycle.last().unwrap();

    // Find the "thinnest" dependency to break: prefer an edge where one
    // module seems to be a lower-level utility (fewest path segments = closer
    // to root = more fundamental).
    let break_from = cycle
        .windows(2)
        .min_by_key(|w| w[1].split("::").count())
        .map(|w| (w[0].as_str(), w[1].as_str()))
        .unwrap_or((last.as_str(), first.as_str()));

    let shared_name = format!(
        "{}_{}",
        last_segment(break_from.0),
        last_segment(break_from.1)
    );

    let members_display = cycle.join(" -> ");

    Some(FixSuggestion {
        rule: "cycle".to_string(),
        component: first.clone(),
        violation: format!("dependency cycle: {}", members_display),
        suggestion: format!(
            "Break the cycle [{members_display}] by extracting shared types/traits \
             into a new neutral module that both '{}' and '{}' can depend on.",
            break_from.0, break_from.1
        ),
        action_items: vec![
            format!(
                "create a new module (e.g. '{}') containing only the shared types or traits",
                shared_name
            ),
            format!(
                "remove the '{}' -> '{}' dependency; instead let both modules import from '{shared_name}'",
                break_from.0, break_from.1
            ),
            format!(
                "verify the cycle is resolved: archlint scan --format brief"
            ),
        ],
    })
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

/// Return the last `::` segment of a module path.
fn last_segment(path: &str) -> &str {
    path.rsplit("::").next().unwrap_or(path)
}

/// Format a fix report as human-readable text.
pub fn format_report(report: &FixReport) -> String {
    let mut out = String::new();

    out.push_str(&format!(
        "fix suggestions: {} violations, {} fixable\n\n",
        report.total_violations, report.fixable
    ));

    if report.suggestions.is_empty() {
        out.push_str("no fixable violations found\n");
        return out;
    }

    for (i, s) in report.suggestions.iter().enumerate() {
        out.push_str(&format!(
            "[{:02}] rule={} component={}\n",
            i + 1,
            s.rule,
            s.component
        ));
        out.push_str(&format!("     violation : {}\n", s.violation));
        out.push_str(&format!("     suggestion: {}\n", s.suggestion));
        if !s.action_items.is_empty() {
            out.push_str("     actions:\n");
            for item in &s.action_items {
                out.push_str(&format!("       - {}\n", item));
            }
        }
        out.push('\n');
    }

    out
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;
    use crate::model::{ArchGraph, Component, Link, Metrics, Violation};

    fn make_graph(violations: Vec<Violation>, cycles: Vec<Vec<String>>, links: Vec<Link>) -> ArchGraph {
        let components: Vec<Component> = violations
            .iter()
            .map(|v| Component {
                id: v.component.clone(),
                title: v.component.clone(),
                entity: "rust".to_string(),
            })
            .collect();

        let metrics = Metrics {
            component_count: components.len(),
            link_count: links.len(),
            max_fan_out: 0,
            max_fan_in: 0,
            cycles,
            violations,
        };

        ArchGraph {
            components,
            links,
            metrics: Some(metrics),
        }
    }

    #[test]
    fn test_suggest_fixes_empty_graph() {
        let graph = ArchGraph {
            components: Vec::new(),
            links: Vec::new(),
            metrics: Some(Metrics {
                component_count: 0,
                link_count: 0,
                max_fan_out: 0,
                max_fan_in: 0,
                cycles: Vec::new(),
                violations: Vec::new(),
            }),
        };
        let report = suggest_fixes(&graph);
        assert_eq!(report.total_violations, 0);
        assert_eq!(report.fixable, 0);
        assert!(report.suggestions.is_empty());
    }

    #[test]
    fn test_suggest_fixes_no_metrics() {
        let graph = ArchGraph {
            components: Vec::new(),
            links: Vec::new(),
            metrics: None,
        };
        let report = suggest_fixes(&graph);
        assert_eq!(report.total_violations, 0);
        assert_eq!(report.fixable, 0);
    }

    #[test]
    fn test_fan_out_suggestion_produced() {
        let v = Violation {
            rule: "fan_out".to_string(),
            component: "src::main".to_string(),
            message: "fan-out 8 exceeds limit 5".to_string(),
            severity: "warning".to_string(),
        };
        let links = vec![
            Link { from: "src::main".to_string(), to: "src::analyzer".to_string(), method: None, link_type: None },
            Link { from: "src::main".to_string(), to: "src::config".to_string(), method: None, link_type: None },
            Link { from: "src::main".to_string(), to: "src::model".to_string(), method: None, link_type: None },
        ];
        let graph = make_graph(vec![v], Vec::new(), links);
        let report = suggest_fixes(&graph);

        assert_eq!(report.total_violations, 1);
        assert_eq!(report.fixable, 1);
        let s = &report.suggestions[0];
        assert_eq!(s.rule, "fan_out");
        assert_eq!(s.component, "src::main");
        assert!(!s.action_items.is_empty());
    }

    #[test]
    fn test_fan_in_suggestion_produced() {
        let v = Violation {
            rule: "fan_in".to_string(),
            component: "src::model".to_string(),
            message: "fan-in 12 exceeds limit 10".to_string(),
            severity: "info".to_string(),
        };
        let graph = make_graph(vec![v], Vec::new(), Vec::new());
        let report = suggest_fixes(&graph);

        assert_eq!(report.fixable, 1);
        let s = &report.suggestions[0];
        assert_eq!(s.rule, "fan_in");
        assert!(s.suggestion.contains("public API"));
    }

    #[test]
    fn test_isp_suggestion_produced() {
        let v = Violation {
            rule: "isp".to_string(),
            component: "src::server".to_string(),
            message: "trait `Handler` has 9 methods, exceeds ISP threshold of 5".to_string(),
            severity: "warning".to_string(),
        };
        let graph = make_graph(vec![v], Vec::new(), Vec::new());
        let report = suggest_fixes(&graph);

        assert_eq!(report.fixable, 1);
        let s = &report.suggestions[0];
        assert_eq!(s.rule, "isp");
        assert!(s.suggestion.contains("Handler"));
        // action items should reference the trait name
        assert!(s.action_items.iter().any(|a| a.contains("Handler")));
    }

    #[test]
    fn test_dip_suggestion_produced() {
        let v = Violation {
            rule: "dip".to_string(),
            component: "src::storage".to_string(),
            message: "module has 4 structs but no trait definitions; consider introducing traits".to_string(),
            severity: "info".to_string(),
        };
        let graph = make_graph(vec![v], Vec::new(), Vec::new());
        let report = suggest_fixes(&graph);

        assert_eq!(report.fixable, 1);
        let s = &report.suggestions[0];
        assert_eq!(s.rule, "dip");
        assert!(s.suggestion.contains("DIP"));
    }

    #[test]
    fn test_cycle_suggestion_produced() {
        let cycle = vec!["src::a".to_string(), "src::b".to_string(), "src::c".to_string()];
        let graph = make_graph(Vec::new(), vec![cycle], Vec::new());
        let report = suggest_fixes(&graph);

        // total_violations includes cycle count
        assert_eq!(report.total_violations, 1);
        assert_eq!(report.fixable, 1);
        let s = &report.suggestions[0];
        assert_eq!(s.rule, "cycle");
        assert!(s.violation.contains("src::a"));
        assert!(!s.action_items.is_empty());
    }

    #[test]
    fn test_cycle_suggestion_names_shared_module() {
        let cycle = vec!["app::service".to_string(), "app::repo".to_string()];
        let graph = make_graph(Vec::new(), vec![cycle], Vec::new());
        let report = suggest_fixes(&graph);

        let s = &report.suggestions[0];
        // The shared module name should combine the last segments
        assert!(s.action_items[0].contains("service_repo") || s.action_items[0].contains("repo_service"));
    }

    #[test]
    fn test_format_report_no_violations() {
        let report = FixReport {
            total_violations: 0,
            fixable: 0,
            suggestions: Vec::new(),
        };
        let text = format_report(&report);
        assert!(text.contains("no fixable violations"));
    }

    #[test]
    fn test_format_report_with_suggestion() {
        let report = FixReport {
            total_violations: 1,
            fixable: 1,
            suggestions: vec![FixSuggestion {
                rule: "fan_out".to_string(),
                component: "src::main".to_string(),
                violation: "fan-out 7 exceeds limit 5".to_string(),
                suggestion: "Reduce outgoing dependencies.".to_string(),
                action_items: vec!["do something".to_string()],
            }],
        };
        let text = format_report(&report);
        assert!(text.contains("[01]"));
        assert!(text.contains("fan_out"));
        assert!(text.contains("src::main"));
        assert!(text.contains("do something"));
    }

    #[test]
    fn test_last_segment() {
        assert_eq!(last_segment("src::foo::bar"), "bar");
        assert_eq!(last_segment("simple"), "simple");
        assert_eq!(last_segment("a::b"), "b");
    }

    #[test]
    fn test_extract_trait_name() {
        assert_eq!(
            extract_trait_name("trait `Handler` has 9 methods, exceeds ISP threshold of 5"),
            Some("Handler")
        );
        assert_eq!(
            extract_trait_name("trait `MyTrait` has 6 methods"),
            Some("MyTrait")
        );
        assert_eq!(extract_trait_name("no backticks here"), None);
    }

    #[test]
    fn test_group_by_prefix() {
        let names: Vec<String> = vec![
            "src::foo".to_string(),
            "src::bar".to_string(),
            "db::store".to_string(),
        ];
        let groups = group_by_prefix(&names);
        assert_eq!(groups["src"].len(), 2);
        assert_eq!(groups["db"].len(), 1);
    }

    #[test]
    fn test_unknown_rule_returns_none() {
        let v = Violation {
            rule: "unknown_rule".to_string(),
            component: "src::foo".to_string(),
            message: "something weird".to_string(),
            severity: "warning".to_string(),
        };
        let outgoing = std::collections::HashMap::new();
        let result = suggest_for_violation(&v, &outgoing, &[]);
        assert!(result.is_none());
    }
}
