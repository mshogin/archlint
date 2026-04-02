/// Generate architecture diagrams from an ArchGraph.
///
/// Supported formats:
/// - Mermaid: flowchart LR with nodes and edges
/// - D2: simple directed graph format
use crate::model::ArchGraph;
use std::collections::HashMap;

/// Sanitize an ID to be safe for use in diagram node names.
/// Replaces non-alphanumeric characters (except underscores) with underscores.
fn sanitize_id(id: &str) -> String {
    id.chars()
        .map(|c| if c.is_alphanumeric() || c == '_' { c } else { '_' })
        .collect()
}

/// Group components by their layer prefix.
/// Components with IDs like "handler/UserHandler" are grouped under "handler".
/// Components without a prefix go into a "__root__" group.
fn group_by_layer(components: &[crate::model::Component]) -> HashMap<String, Vec<&crate::model::Component>> {
    let mut groups: HashMap<String, Vec<&crate::model::Component>> = HashMap::new();
    for comp in components {
        let layer = if let Some(pos) = comp.id.find('/') {
            comp.id[..pos].to_string()
        } else {
            "__root__".to_string()
        };
        groups.entry(layer).or_default().push(comp);
    }
    groups
}

/// Generate a Mermaid diagram from an ArchGraph.
///
/// Produces a `flowchart LR` with:
/// - Components as nodes labeled with their title
/// - Links as directed edges (with optional method label)
/// - Components grouped by layer prefix as subgraphs
pub fn generate_mermaid(graph: &ArchGraph) -> String {
    let mut out = String::new();
    out.push_str("flowchart LR\n");

    let groups = group_by_layer(&graph.components);
    let mut layer_names: Vec<String> = groups.keys().cloned().collect();
    layer_names.sort();

    for layer in &layer_names {
        let comps = &groups[layer];
        if layer == "__root__" {
            // Nodes without a layer prefix: emit directly
            for comp in comps {
                let node_id = sanitize_id(&comp.id);
                let label = escape_mermaid_label(&comp.title);
                out.push_str(&format!("    {}[\"{}\"]\n", node_id, label));
            }
        } else {
            // Group into a subgraph
            out.push_str(&format!("    subgraph {}\n", layer));
            for comp in comps {
                let node_id = sanitize_id(&comp.id);
                let label = escape_mermaid_label(&comp.title);
                out.push_str(&format!("        {}[\"{}\"]\n", node_id, label));
            }
            out.push_str("    end\n");
        }
    }

    // Edges
    for link in &graph.links {
        let from = sanitize_id(&link.from);
        let to = sanitize_id(&link.to);
        match &link.method {
            Some(method) if !method.is_empty() => {
                let label = escape_mermaid_label(method);
                out.push_str(&format!("    {} -->|\"{}\"| {}\n", from, label, to));
            }
            _ => {
                out.push_str(&format!("    {} --> {}\n", from, to));
            }
        }
    }

    out
}

/// Escape a label string for safe use inside Mermaid node labels.
/// Replaces double-quotes with single-quotes to avoid breaking `["..."]` syntax.
fn escape_mermaid_label(s: &str) -> String {
    s.replace('"', "'")
}

/// Generate a D2 diagram from an ArchGraph.
///
/// Produces a D2 directed graph with:
/// - Components as nodes with label
/// - Links as directed edges
/// - Components grouped by layer prefix
pub fn generate_d2(graph: &ArchGraph) -> String {
    let mut out = String::new();

    let groups = group_by_layer(&graph.components);
    let mut layer_names: Vec<String> = groups.keys().cloned().collect();
    layer_names.sort();

    for layer in &layer_names {
        let comps = &groups[layer];
        if layer == "__root__" {
            for comp in comps {
                let node_id = sanitize_id(&comp.id);
                let label = escape_d2_string(&comp.title);
                out.push_str(&format!("{}: \"{}\"\n", node_id, label));
            }
        } else {
            out.push_str(&format!("{}: {{\n", sanitize_id(layer)));
            for comp in comps {
                // Use short name inside the container (part after the /)
                let short = comp.id.find('/').map(|p| &comp.id[p + 1..]).unwrap_or(&comp.id);
                let node_id = sanitize_id(short);
                let label = escape_d2_string(&comp.title);
                out.push_str(&format!("  {}: \"{}\"\n", node_id, label));
            }
            out.push_str("}\n");
        }
    }

    // Edges
    for link in &graph.links {
        let from = d2_node_ref(&link.from);
        let to = d2_node_ref(&link.to);
        match &link.method {
            Some(method) if !method.is_empty() => {
                let label = escape_d2_string(method);
                out.push_str(&format!("{} -> {}: \"{}\"\n", from, to, label));
            }
            _ => {
                out.push_str(&format!("{} -> {}\n", from, to));
            }
        }
    }

    out
}

/// Convert a component ID to a D2 node reference.
/// IDs with a "/" become `layer.short_name` in D2 notation.
fn d2_node_ref(id: &str) -> String {
    if let Some(pos) = id.find('/') {
        let layer = sanitize_id(&id[..pos]);
        let short = sanitize_id(&id[pos + 1..]);
        format!("{}.{}", layer, short)
    } else {
        sanitize_id(id)
    }
}

/// Escape a string for D2 double-quoted string literals.
fn escape_d2_string(s: &str) -> String {
    s.replace('"', "'").replace('\\', "\\\\")
}

#[cfg(test)]
mod tests {
    use super::*;
    use crate::model::{ArchGraph, Component, Link};

    fn make_graph() -> ArchGraph {
        ArchGraph {
            components: vec![
                Component {
                    id: "handler/UserHandler".to_string(),
                    title: "User Handler".to_string(),
                    entity: "struct".to_string(),
                },
                Component {
                    id: "service/UserService".to_string(),
                    title: "User Service".to_string(),
                    entity: "struct".to_string(),
                },
                Component {
                    id: "repository/UserRepo".to_string(),
                    title: "User Repository".to_string(),
                    entity: "struct".to_string(),
                },
            ],
            links: vec![
                Link {
                    from: "handler/UserHandler".to_string(),
                    to: "service/UserService".to_string(),
                    method: Some("GetUser".to_string()),
                    link_type: None,
                },
                Link {
                    from: "service/UserService".to_string(),
                    to: "repository/UserRepo".to_string(),
                    method: None,
                    link_type: None,
                },
            ],
            metrics: None,
        }
    }

    fn make_flat_graph() -> ArchGraph {
        ArchGraph {
            components: vec![
                Component {
                    id: "Alpha".to_string(),
                    title: "Alpha Component".to_string(),
                    entity: "module".to_string(),
                },
                Component {
                    id: "Beta".to_string(),
                    title: "Beta Component".to_string(),
                    entity: "module".to_string(),
                },
            ],
            links: vec![Link {
                from: "Alpha".to_string(),
                to: "Beta".to_string(),
                method: None,
                link_type: None,
            }],
            metrics: None,
        }
    }

    // --- Mermaid tests ---

    #[test]
    fn test_mermaid_starts_with_flowchart() {
        let g = make_graph();
        let out = generate_mermaid(&g);
        assert!(out.starts_with("flowchart LR"), "should start with flowchart LR");
    }

    #[test]
    fn test_mermaid_contains_nodes() {
        let g = make_graph();
        let out = generate_mermaid(&g);
        assert!(out.contains("handler_UserHandler"), "should have handler node");
        assert!(out.contains("service_UserService"), "should have service node");
        assert!(out.contains("repository_UserRepo"), "should have repo node");
    }

    #[test]
    fn test_mermaid_contains_node_labels() {
        let g = make_graph();
        let out = generate_mermaid(&g);
        assert!(out.contains("User Handler"), "should contain handler label");
        assert!(out.contains("User Service"), "should contain service label");
        assert!(out.contains("User Repository"), "should contain repo label");
    }

    #[test]
    fn test_mermaid_contains_subgraphs() {
        let g = make_graph();
        let out = generate_mermaid(&g);
        assert!(out.contains("subgraph handler"), "should have handler subgraph");
        assert!(out.contains("subgraph service"), "should have service subgraph");
        assert!(out.contains("subgraph repository"), "should have repository subgraph");
    }

    #[test]
    fn test_mermaid_contains_edges() {
        let g = make_graph();
        let out = generate_mermaid(&g);
        // Edge with method label
        assert!(out.contains("-->|"), "should have labeled edge");
        assert!(out.contains("GetUser"), "should contain method name");
        // Edge without method label
        assert!(out.contains("-->"), "should have unlabeled edge");
    }

    #[test]
    fn test_mermaid_flat_graph() {
        let g = make_flat_graph();
        let out = generate_mermaid(&g);
        // No subgraphs when components have no layer prefix
        assert!(!out.contains("subgraph"), "flat graph should have no subgraphs");
        assert!(out.contains("Alpha"), "should have Alpha node");
        assert!(out.contains("Beta"), "should have Beta node");
        assert!(out.contains("Alpha --> Beta"), "should have edge");
    }

    #[test]
    fn test_mermaid_empty_graph() {
        let g = ArchGraph {
            components: vec![],
            links: vec![],
            metrics: None,
        };
        let out = generate_mermaid(&g);
        assert!(out.starts_with("flowchart LR"), "empty graph should still have header");
    }

    #[test]
    fn test_mermaid_special_chars_in_id() {
        let g = ArchGraph {
            components: vec![Component {
                id: "pkg/my-component.go".to_string(),
                title: "My Component".to_string(),
                entity: "file".to_string(),
            }],
            links: vec![],
            metrics: None,
        };
        let out = generate_mermaid(&g);
        // Hyphens and dots become underscores
        assert!(out.contains("my_component_go"), "special chars in id should be sanitized");
    }

    // --- D2 tests ---

    #[test]
    fn test_d2_contains_nodes() {
        let g = make_graph();
        let out = generate_d2(&g);
        assert!(out.contains("UserHandler"), "should have UserHandler");
        assert!(out.contains("UserService"), "should have UserService");
        assert!(out.contains("UserRepo"), "should have UserRepo");
    }

    #[test]
    fn test_d2_contains_containers() {
        let g = make_graph();
        let out = generate_d2(&g);
        assert!(out.contains("handler: {"), "should have handler container");
        assert!(out.contains("service: {"), "should have service container");
        assert!(out.contains("repository: {"), "should have repository container");
    }

    #[test]
    fn test_d2_contains_edges() {
        let g = make_graph();
        let out = generate_d2(&g);
        assert!(out.contains("handler.UserHandler -> service.UserService"), "should have labeled edge");
        assert!(out.contains("service.UserService -> repository.UserRepo"), "should have unlabeled edge");
    }

    #[test]
    fn test_d2_flat_graph() {
        let g = make_flat_graph();
        let out = generate_d2(&g);
        assert!(!out.contains('{'), "flat graph should have no containers");
        assert!(out.contains("Alpha -> Beta"), "should have edge");
    }

    #[test]
    fn test_d2_empty_graph() {
        let g = ArchGraph {
            components: vec![],
            links: vec![],
            metrics: None,
        };
        let out = generate_d2(&g);
        assert!(out.is_empty() || !out.contains("->"), "empty graph should have no edges");
    }

    #[test]
    fn test_d2_edge_with_method_label() {
        let g = make_graph();
        let out = generate_d2(&g);
        assert!(out.contains("GetUser"), "should contain method label in D2 edge");
    }

    // --- sanitize_id tests ---

    #[test]
    fn test_sanitize_id_basic() {
        assert_eq!(sanitize_id("hello_world"), "hello_world");
        assert_eq!(sanitize_id("hello-world"), "hello_world");
        assert_eq!(sanitize_id("pkg/Name"), "pkg_Name");
        assert_eq!(sanitize_id("a.b.c"), "a_b_c");
    }

    #[test]
    fn test_sanitize_id_empty() {
        assert_eq!(sanitize_id(""), "");
    }
}
