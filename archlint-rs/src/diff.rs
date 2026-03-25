use crate::analyzer;
use crate::model::ArchGraph;
use serde::{Deserialize, Serialize};
use std::collections::{HashMap, HashSet};
use std::path::Path;
use std::process::Command;

/// Result of comparing two architecture snapshots.
#[derive(Debug, Serialize, Deserialize)]
pub struct ArchDiff {
    pub from_ref: String,
    pub to_ref: String,
    pub components: ComponentDiff,
    pub links: LinkDiff,
    pub metrics: MetricsDiff,
    pub new_violations: Vec<String>,
    pub resolved_violations: Vec<String>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ComponentDiff {
    pub added: Vec<String>,
    pub removed: Vec<String>,
    pub modified: Vec<ComponentChange>,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct ComponentChange {
    pub id: String,
    pub change: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct LinkDiff {
    pub added: Vec<LinkEntry>,
    pub removed: Vec<LinkEntry>,
}

#[derive(Debug, Serialize, Deserialize, Hash, Eq, PartialEq, Clone)]
pub struct LinkEntry {
    pub from: String,
    pub to: String,
}

#[derive(Debug, Serialize, Deserialize)]
pub struct MetricsDiff {
    pub components_before: usize,
    pub components_after: usize,
    pub links_before: usize,
    pub links_after: usize,
    pub violations_before: usize,
    pub violations_after: usize,
    pub max_fanout_before: usize,
    pub max_fanout_after: usize,
}

/// Compare architecture between two git refs.
pub fn diff(project_dir: &Path, from_ref: &str, to_ref: &str) -> Result<ArchDiff, String> {
    // Scan current state (to_ref = HEAD usually)
    let graph_after = if to_ref == "HEAD" || to_ref.is_empty() {
        analyzer::analyze(project_dir)?
    } else {
        scan_at_ref(project_dir, to_ref)?
    };

    // Scan previous state
    let graph_before = scan_at_ref(project_dir, from_ref)?;

    Ok(compare_graphs(&graph_before, &graph_after, from_ref, to_ref))
}

/// Checkout a ref in a temp worktree and scan it.
fn scan_at_ref(project_dir: &Path, git_ref: &str) -> Result<ArchGraph, String> {
    let tmp_dir = std::env::temp_dir().join(format!("archlint-diff-{}", git_ref.replace('/', "-")));

    // Create git worktree
    let output = Command::new("git")
        .args(["worktree", "add", "--detach", tmp_dir.to_str().unwrap(), git_ref])
        .current_dir(project_dir)
        .output()
        .map_err(|e| format!("git worktree: {}", e))?;

    if !output.status.success() {
        // Worktree might already exist, try to use stash/checkout approach
        let _ = Command::new("git")
            .args(["worktree", "remove", "--force", tmp_dir.to_str().unwrap()])
            .current_dir(project_dir)
            .output();

        let output = Command::new("git")
            .args(["worktree", "add", "--detach", tmp_dir.to_str().unwrap(), git_ref])
            .current_dir(project_dir)
            .output()
            .map_err(|e| format!("git worktree retry: {}", e))?;

        if !output.status.success() {
            return Err(format!(
                "failed to create worktree for {}: {}",
                git_ref,
                String::from_utf8_lossy(&output.stderr)
            ));
        }
    }

    let result = analyzer::analyze(&tmp_dir);

    // Cleanup worktree
    let _ = Command::new("git")
        .args(["worktree", "remove", "--force", tmp_dir.to_str().unwrap()])
        .current_dir(project_dir)
        .output();

    result
}

fn compare_graphs(before: &ArchGraph, after: &ArchGraph, from_ref: &str, to_ref: &str) -> ArchDiff {
    // Component sets
    let before_ids: HashSet<&str> = before.components.iter().map(|c| c.id.as_str()).collect();
    let after_ids: HashSet<&str> = after.components.iter().map(|c| c.id.as_str()).collect();

    let added: Vec<String> = after_ids.difference(&before_ids).map(|s| s.to_string()).collect();
    let removed: Vec<String> = before_ids.difference(&after_ids).map(|s| s.to_string()).collect();

    // Fan-out changes for existing components
    let before_fanout: HashMap<&str, usize> = compute_fanout(before);
    let after_fanout: HashMap<&str, usize> = compute_fanout(after);

    let mut modified = Vec::new();
    for id in before_ids.intersection(&after_ids) {
        let fo_before = before_fanout.get(id).copied().unwrap_or(0);
        let fo_after = after_fanout.get(id).copied().unwrap_or(0);
        if fo_before != fo_after {
            modified.push(ComponentChange {
                id: id.to_string(),
                change: format!("fan-out {}->{}",fo_before, fo_after),
            });
        }
    }

    // Link sets
    let before_links: HashSet<LinkEntry> = before.links.iter()
        .map(|l| LinkEntry { from: l.from.clone(), to: l.to.clone() })
        .collect();
    let after_links: HashSet<LinkEntry> = after.links.iter()
        .map(|l| LinkEntry { from: l.from.clone(), to: l.to.clone() })
        .collect();

    let added_links: Vec<LinkEntry> = after_links.difference(&before_links).cloned().collect();
    let removed_links: Vec<LinkEntry> = before_links.difference(&after_links).cloned().collect();

    // Metrics
    let violations_before = before.metrics.as_ref().map(|m| m.violations.len()).unwrap_or(0);
    let violations_after = after.metrics.as_ref().map(|m| m.violations.len()).unwrap_or(0);
    let max_fo_before = before.metrics.as_ref().map(|m| m.max_fan_out).unwrap_or(0);
    let max_fo_after = after.metrics.as_ref().map(|m| m.max_fan_out).unwrap_or(0);

    // Violation diff
    let before_violations: HashSet<String> = before.metrics.as_ref()
        .map(|m| m.violations.iter().map(|v| format!("{}: {}", v.component, v.message)).collect())
        .unwrap_or_default();
    let after_violations: HashSet<String> = after.metrics.as_ref()
        .map(|m| m.violations.iter().map(|v| format!("{}: {}", v.component, v.message)).collect())
        .unwrap_or_default();

    let new_violations: Vec<String> = after_violations.difference(&before_violations).cloned().collect();
    let resolved_violations: Vec<String> = before_violations.difference(&after_violations).cloned().collect();

    ArchDiff {
        from_ref: from_ref.to_string(),
        to_ref: to_ref.to_string(),
        components: ComponentDiff { added, removed, modified },
        links: LinkDiff { added: added_links, removed: removed_links },
        metrics: MetricsDiff {
            components_before: before.components.len(),
            components_after: after.components.len(),
            links_before: before.links.len(),
            links_after: after.links.len(),
            violations_before,
            violations_after,
            max_fanout_before: max_fo_before,
            max_fanout_after: max_fo_after,
        },
        new_violations,
        resolved_violations,
    }
}

fn compute_fanout(graph: &ArchGraph) -> HashMap<&str, usize> {
    let mut fanout: HashMap<&str, usize> = HashMap::new();
    for link in &graph.links {
        *fanout.entry(link.from.as_str()).or_insert(0) += 1;
    }
    fanout
}

/// Format diff as human-readable string.
pub fn format_diff(d: &ArchDiff) -> String {
    let mut s = format!("Architecture Diff: {}..{}\n\n", d.from_ref, d.to_ref);

    // Components
    s.push_str("Components:\n");
    for c in &d.components.added {
        s.push_str(&format!("  + {} (NEW)\n", c));
    }
    for c in &d.components.removed {
        s.push_str(&format!("  - {} (REMOVED)\n", c));
    }
    for c in &d.components.modified {
        s.push_str(&format!("  ~ {} ({})\n", c.id, c.change));
    }
    if d.components.added.is_empty() && d.components.removed.is_empty() && d.components.modified.is_empty() {
        s.push_str("  (no changes)\n");
    }

    // Links
    s.push_str(&format!("\nLinks:\n  +{} new, -{} removed\n",
        d.links.added.len(), d.links.removed.len()));

    // Metrics
    let m = &d.metrics;
    s.push_str(&format!(
        "\nMetrics:\n  Components: {} -> {} ({:+})\n  Links: {} -> {} ({:+})\n  Violations: {} -> {} ({:+})\n  Max fan-out: {} -> {} ({:+})\n",
        m.components_before, m.components_after, m.components_after as i64 - m.components_before as i64,
        m.links_before, m.links_after, m.links_after as i64 - m.links_before as i64,
        m.violations_before, m.violations_after, m.violations_after as i64 - m.violations_before as i64,
        m.max_fanout_before, m.max_fanout_after, m.max_fanout_after as i64 - m.max_fanout_before as i64,
    ));

    // New violations
    if !d.new_violations.is_empty() {
        s.push_str("\nNew Violations:\n");
        for v in &d.new_violations {
            s.push_str(&format!("  [new] {}\n", v));
        }
    }

    // Resolved
    if !d.resolved_violations.is_empty() {
        s.push_str("\nResolved Violations:\n");
        for v in &d.resolved_violations {
            s.push_str(&format!("  [fixed] {}\n", v));
        }
    }

    s
}
