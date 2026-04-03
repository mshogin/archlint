use petgraph::graph::{DiGraph, NodeIndex};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;

/// Architecture graph node (component).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Component {
    pub id: String,
    pub title: String,
    pub entity: String,
}

/// Architecture graph edge (dependency link).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Link {
    pub from: String,
    pub to: String,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub method: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub link_type: Option<String>,
}

/// Architecture graph with metrics.
#[derive(Debug, Serialize, Deserialize)]
pub struct ArchGraph {
    pub components: Vec<Component>,
    pub links: Vec<Link>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub metrics: Option<Metrics>,
}

/// Architecture metrics.
#[derive(Debug, Serialize, Deserialize)]
pub struct Metrics {
    pub component_count: usize,
    pub link_count: usize,
    pub max_fan_out: usize,
    pub max_fan_in: usize,
    pub cycles: Vec<Vec<String>>,
    pub violations: Vec<Violation>,
}

/// Architecture violation.
#[derive(Debug, Serialize, Deserialize)]
pub struct Violation {
    pub rule: String,
    pub component: String,
    pub message: String,
    pub severity: String,
    /// Metric level: taboo (CI blocker), telemetry (track only), personal (informational).
    #[serde(default = "default_telemetry")]
    pub level: String,
}

fn default_telemetry() -> String {
    "telemetry".to_string()
}

/// Standard JSON graph export format compatible with Go's model.Graph.
/// Used for the Unix-pipe multi-language architecture pipeline.
#[derive(Debug, Serialize, Deserialize)]
pub struct GraphExport {
    pub nodes: Vec<GraphNode>,
    pub edges: Vec<GraphEdge>,
    pub metadata: GraphMetadata,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub metrics: Option<GraphMetrics>,
}

/// Node in the exported graph (compatible with Go model.Node).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GraphNode {
    pub id: String,
    #[serde(rename = "type")]
    pub node_type: String,
    pub package: String,
    pub name: String,
    pub file: String,
    pub line: u32,
}

/// Edge in the exported graph (compatible with Go model.Edge).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GraphEdge {
    pub from: String,
    pub to: String,
    #[serde(rename = "type")]
    pub edge_type: String,
}

/// Metadata about the graph export.
#[derive(Debug, Serialize, Deserialize)]
pub struct GraphMetadata {
    pub language: String,
    pub root_dir: String,
    pub analyzed_at: String,
}

/// Metrics summary included in the exported graph.
#[derive(Debug, Serialize, Deserialize)]
pub struct GraphMetrics {
    pub component_count: usize,
    pub link_count: usize,
    pub max_fan_out: usize,
    pub max_fan_in: usize,
    pub cycles: Vec<Vec<String>>,
    pub violations: Vec<GraphViolation>,
}

/// Violation entry in the exported graph metrics.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct GraphViolation {
    pub rule: String,
    pub component: String,
    pub message: String,
    pub severity: String,
    /// Metric level: taboo (CI blocker), telemetry (track only), personal (informational).
    #[serde(default = "default_telemetry_graph")]
    pub level: String,
}

fn default_telemetry_graph() -> String {
    "telemetry".to_string()
}

/// Per-language analysis report (used in multi-language unified scan).
#[derive(Debug, Serialize, Deserialize)]
pub struct LanguageReport {
    pub language: String,
    pub components: usize,
    pub links: usize,
    pub health: u32,
    pub violations: Vec<String>,
    pub violation_count: usize,
    /// Number of taboo-level violations (CI blockers).
    #[serde(default)]
    pub taboo_count: usize,
    /// Number of telemetry-level violations (track only).
    #[serde(default)]
    pub telemetry_count: usize,
    /// Number of personal-level violations (informational).
    #[serde(default)]
    pub personal_count: usize,
    /// Number of todo-level violations (known violations for gradual migration).
    #[serde(default)]
    pub todo_count: usize,
    /// Full violation objects with level info.
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub violations_detail: Vec<ViolationSummary>,
    /// Detected entry points (fan_in=0, fan_out>0 or manifest-declared binaries).
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub entry_points: Vec<String>,
}

/// Compact violation summary for scan output.
#[derive(Debug, Serialize, Deserialize)]
pub struct ViolationSummary {
    pub rule: String,
    pub component: String,
    pub message: String,
    pub level: String,
}

/// Unified multi-language scan report.
#[derive(Debug, Serialize, Deserialize)]
pub struct MultiLanguageReport {
    pub project: String,
    pub languages: Vec<String>,
    pub per_language: Vec<LanguageReport>,
    pub total_components: usize,
    pub total_links: usize,
    pub total_violations: usize,
    pub total_health: u32,
    /// Number of taboo-level violations across all languages.
    #[serde(default)]
    pub total_taboo: usize,
    /// Number of todo-level violations (known violations for gradual migration).
    #[serde(default)]
    pub total_todo: usize,
}

/// Indexed graph for efficient operations.
pub struct IndexedGraph {
    pub graph: DiGraph<String, String>,
    pub node_indices: HashMap<String, NodeIndex>,
}

impl IndexedGraph {
    pub fn new() -> Self {
        Self {
            graph: DiGraph::new(),
            node_indices: HashMap::new(),
        }
    }

    pub fn add_node(&mut self, id: &str) -> NodeIndex {
        if let Some(&idx) = self.node_indices.get(id) {
            return idx;
        }
        let idx = self.graph.add_node(id.to_string());
        self.node_indices.insert(id.to_string(), idx);
        idx
    }

    pub fn add_edge(&mut self, from: &str, to: &str, label: &str) {
        let from_idx = self.add_node(from);
        let to_idx = self.add_node(to);
        self.graph.add_edge(from_idx, to_idx, label.to_string());
    }

    pub fn fan_out(&self, id: &str) -> usize {
        if let Some(&idx) = self.node_indices.get(id) {
            self.graph.neighbors_directed(idx, petgraph::Direction::Outgoing).count()
        } else {
            0
        }
    }

    pub fn fan_in(&self, id: &str) -> usize {
        if let Some(&idx) = self.node_indices.get(id) {
            self.graph.neighbors_directed(idx, petgraph::Direction::Incoming).count()
        } else {
            0
        }
    }
}
