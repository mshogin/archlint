use crate::model::{ArchGraph, Component, IndexedGraph, Link, Metrics, Violation};
use rayon::prelude::*;
use regex::Regex;
use std::collections::HashMap;
use std::fs;
use std::path::{Path, PathBuf};
use walkdir::WalkDir;

/// Analyze a project directory and return architecture graph.
pub fn analyze(dir: &Path) -> Result<ArchGraph, String> {
    let files = collect_source_files(dir);

    // Parse files in parallel using rayon
    let parsed: Vec<ParsedFile> = files
        .par_iter()
        .filter_map(|path| parse_file(path, dir).ok())
        .collect();

    // Build graph from parsed files
    let mut graph = IndexedGraph::new();
    let mut components = Vec::new();
    let mut links = Vec::new();

    for pf in &parsed {
        // Add component node
        graph.add_node(&pf.module_name);
        components.push(Component {
            id: pf.module_name.clone(),
            title: pf.module_name.clone(),
            entity: pf.language.clone(),
        });

        // Add dependency edges
        for dep in &pf.dependencies {
            graph.add_edge(&pf.module_name, dep, "depends");
            links.push(Link {
                from: pf.module_name.clone(),
                to: dep.clone(),
                method: None,
                link_type: Some("depends".to_string()),
            });
        }
    }

    // Calculate metrics
    let metrics = calculate_metrics(&graph, &components);

    Ok(ArchGraph {
        components,
        links,
        metrics: Some(metrics),
    })
}

/// Collect all source files (.go, .rs) from directory.
fn collect_source_files(dir: &Path) -> Vec<PathBuf> {
    WalkDir::new(dir)
        .into_iter()
        .filter_map(|e| e.ok())
        .filter(|e| {
            let path = e.path();
            !path.components().any(|c| {
                let s = c.as_os_str().to_string_lossy();
                s.starts_with('.') || s == "vendor" || s == "target" || s == "node_modules"
            })
        })
        .filter(|e| {
            let ext = e.path().extension().and_then(|e| e.to_str()).unwrap_or("");
            ext == "go" || ext == "rs"
        })
        .map(|e| e.path().to_path_buf())
        .collect()
}

struct ParsedFile {
    module_name: String,
    language: String,
    dependencies: Vec<String>,
    structs: Vec<String>,
    functions: Vec<String>,
}

/// Parse a single source file for dependencies and declarations.
fn parse_file(path: &Path, base_dir: &Path) -> Result<ParsedFile, String> {
    let ext = path.extension().and_then(|e| e.to_str()).unwrap_or("");
    let content = fs::read_to_string(path).map_err(|e| e.to_string())?;
    let rel_path = path.strip_prefix(base_dir).unwrap_or(path);
    let module_name = path_to_module(rel_path, ext);

    match ext {
        "go" => parse_go_file(&content, &module_name),
        "rs" => parse_rust_file(&content, &module_name),
        _ => Err(format!("unsupported extension: {}", ext)),
    }
}

fn parse_go_file(content: &str, module_name: &str) -> Result<ParsedFile, String> {
    let re_import = Regex::new(r#""([^"]+)""#).unwrap();
    let re_struct = Regex::new(r"type\s+(\w+)\s+struct").unwrap();
    let re_func = Regex::new(r"func\s+(?:\([^)]+\)\s+)?(\w+)").unwrap();
    let re_package = Regex::new(r"^package\s+(\w+)").unwrap();

    let mut deps = Vec::new();
    let mut structs = Vec::new();
    let mut functions = Vec::new();
    let mut in_import = false;

    for line in content.lines() {
        let trimmed = line.trim();

        if trimmed.starts_with("import (") {
            in_import = true;
            continue;
        }
        if in_import {
            if trimmed == ")" {
                in_import = false;
                continue;
            }
            if let Some(cap) = re_import.captures(trimmed) {
                let imp = cap[1].to_string();
                // Extract last segment as dependency name
                if let Some(last) = imp.rsplit('/').next() {
                    deps.push(last.to_string());
                }
            }
            continue;
        }

        if let Some(cap) = re_struct.captures(trimmed) {
            structs.push(cap[1].to_string());
        }
        if let Some(cap) = re_func.captures(trimmed) {
            functions.push(cap[1].to_string());
        }
    }

    Ok(ParsedFile {
        module_name: module_name.to_string(),
        language: "go".to_string(),
        dependencies: deps,
        structs,
        functions,
    })
}

fn parse_rust_file(content: &str, module_name: &str) -> Result<ParsedFile, String> {
    let re_use = Regex::new(r"^(?:pub\s+)?use\s+(?:crate::)?(\w+)").unwrap();
    let re_mod = Regex::new(r"^(?:pub(?:\(crate\))?\s+)?mod\s+(\w+)").unwrap();
    let re_struct = Regex::new(r"^(?:pub(?:\(crate\))?\s+)?struct\s+(\w+)").unwrap();
    let re_fn = Regex::new(r"^(?:pub(?:\(crate\))?\s+)?(?:async\s+)?fn\s+(\w+)").unwrap();

    let mut deps = Vec::new();
    let mut structs = Vec::new();
    let mut functions = Vec::new();

    for line in content.lines() {
        let trimmed = line.trim();

        if trimmed.starts_with("//") {
            continue;
        }

        if let Some(cap) = re_use.captures(trimmed) {
            deps.push(cap[1].to_string());
        }
        if let Some(cap) = re_mod.captures(trimmed) {
            deps.push(cap[1].to_string());
        }
        if let Some(cap) = re_struct.captures(trimmed) {
            structs.push(cap[1].to_string());
        }
        if let Some(cap) = re_fn.captures(trimmed) {
            functions.push(cap[1].to_string());
        }
    }

    Ok(ParsedFile {
        module_name: module_name.to_string(),
        language: "rust".to_string(),
        dependencies: deps,
        structs,
        functions,
    })
}

fn path_to_module(rel_path: &Path, ext: &str) -> String {
    let s = rel_path.to_string_lossy();
    let name = s.trim_end_matches(&format!(".{}", ext));
    let name = name.replace('/', "::");
    let name = name.replace('\\', "::");
    // Remove mod suffix for Rust
    let name = name.trim_end_matches("::mod");
    name.to_string()
}

fn calculate_metrics(graph: &IndexedGraph, components: &[Component]) -> Metrics {
    let mut max_fan_out = 0;
    let mut max_fan_in = 0;
    let mut violations = Vec::new();

    for comp in components {
        let fo = graph.fan_out(&comp.id);
        let fi = graph.fan_in(&comp.id);

        if fo > max_fan_out {
            max_fan_out = fo;
        }
        if fi > max_fan_in {
            max_fan_in = fi;
        }

        // Check fan-out violation (max 5)
        if fo > 5 {
            violations.push(Violation {
                rule: "fan_out".to_string(),
                component: comp.id.clone(),
                message: format!("fan-out {} exceeds limit 5", fo),
                severity: "warning".to_string(),
            });
        }

        // Check fan-in violation (max 10)
        if fi > 10 {
            violations.push(Violation {
                rule: "fan_in".to_string(),
                component: comp.id.clone(),
                message: format!("fan-in {} exceeds limit 10", fi),
                severity: "info".to_string(),
            });
        }
    }

    // Detect cycles using petgraph
    let cycles = detect_cycles(graph);

    Metrics {
        component_count: components.len(),
        link_count: graph.graph.edge_count(),
        max_fan_out,
        max_fan_in,
        cycles,
        violations,
    }
}

fn detect_cycles(graph: &IndexedGraph) -> Vec<Vec<String>> {
    use petgraph::algo::tarjan_scc;

    let sccs = tarjan_scc(&graph.graph);
    let mut cycles = Vec::new();

    for scc in sccs {
        if scc.len() > 1 {
            let cycle: Vec<String> = scc
                .iter()
                .map(|&idx| graph.graph[idx].clone())
                .collect();
            cycles.push(cycle);
        }
    }

    cycles
}
