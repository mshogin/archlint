use crate::config::Config;
use crate::model::{ArchGraph, Component, IndexedGraph, Link, Metrics, Violation};
use rayon::prelude::*;
use regex::Regex;
use std::collections::HashSet;
use std::fs;
use std::path::{Path, PathBuf};
use walkdir::WalkDir;

/// Load external crate names from Cargo.toml in the given directory.
/// Returns empty set if Cargo.toml is not found or cannot be parsed.
fn load_cargo_external_deps(dir: &Path) -> HashSet<String> {
    let cargo_path = dir.join("Cargo.toml");
    let content = match fs::read_to_string(&cargo_path) {
        Ok(c) => c,
        Err(_) => return HashSet::new(),
    };
    let doc: toml::Value = match toml::from_str(&content) {
        Ok(v) => v,
        Err(_) => return HashSet::new(),
    };
    let mut deps = HashSet::new();
    for section in &["dependencies", "dev-dependencies", "build-dependencies"] {
        if let Some(table) = doc.get(section).and_then(|v| v.as_table()) {
            for name in table.keys() {
                // Cargo dep names use hyphens; crate names use underscores
                deps.insert(name.replace('-', "_"));
            }
        }
    }
    deps
}

/// Analyze a project directory and return architecture graph.
/// Config is loaded from `.archlint.yaml` in the directory; defaults are used when absent.
pub fn analyze(dir: &Path) -> Result<ArchGraph, String> {
    let config = Config::load(dir);
    analyze_with_config(dir, &config)
}

/// Analyze a project directory using the provided config.
pub fn analyze_with_config(dir: &Path, config: &Config) -> Result<ArchGraph, String> {
    let files = collect_source_files(dir);
    let external_deps = load_cargo_external_deps(dir);

    // Parse files in parallel using rayon
    let parsed: Vec<ParsedFile> = files
        .par_iter()
        .filter_map(|path| parse_file(path, dir, &external_deps).ok())
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

    // Calculate metrics using thresholds from config
    let metrics = calculate_metrics(&graph, &components, config);

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
fn parse_file(path: &Path, base_dir: &Path, external_deps: &HashSet<String>) -> Result<ParsedFile, String> {
    let ext = path.extension().and_then(|e| e.to_str()).unwrap_or("");
    let content = fs::read_to_string(path).map_err(|e| e.to_string())?;
    let rel_path = path.strip_prefix(base_dir).unwrap_or(path);
    let module_name = path_to_module(rel_path, ext);

    match ext {
        "go" => parse_go_file(&content, &module_name),
        "rs" => parse_rust_file(&content, &module_name, external_deps),
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

fn parse_rust_file(content: &str, module_name: &str, external_deps: &HashSet<String>) -> Result<ParsedFile, String> {
    // Matches: use crate::foo, use self::foo, use super::foo -> internal (crate-local)
    let re_use_internal = Regex::new(r"^(?:pub\s+)?use\s+(crate|self|super)::").unwrap();
    // Matches: use foo::... -> captures foo as the crate root
    let re_use_external = Regex::new(r"^(?:pub\s+)?use\s+(\w+)").unwrap();
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

        if trimmed.starts_with("use ") || trimmed.starts_with("pub use ") {
            // Internal: crate::, self::, super:: — always count
            if re_use_internal.is_match(trimmed) {
                // Extract the module name after the prefix
                let re_internal_name = Regex::new(r"^(?:pub\s+)?use\s+(?:crate|self|super)::(\w+)").unwrap();
                if let Some(cap) = re_internal_name.captures(trimmed) {
                    deps.push(cap[1].to_string());
                }
            } else if let Some(cap) = re_use_external.captures(trimmed) {
                let crate_name = &cap[1];
                // Skip std, core, alloc (language built-ins)
                // Skip anything listed in Cargo.toml as an external dependency
                if !is_external_crate(crate_name, external_deps) {
                    deps.push(crate_name.to_string());
                }
            }
            continue;
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

/// Returns true if the given crate name should be excluded from metrics.
/// External crates are: std, core, alloc (built-ins) and anything in Cargo.toml dependencies.
fn is_external_crate(name: &str, cargo_deps: &HashSet<String>) -> bool {
    matches!(name, "std" | "core" | "alloc") || cargo_deps.contains(name)
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

fn calculate_metrics(graph: &IndexedGraph, components: &[Component], config: &Config) -> Metrics {
    let mut max_fan_out = 0;
    let mut max_fan_in = 0;
    let mut violations = Vec::new();

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

        // Check fan-out violation
        if config.rules.fan_out.enabled
            && fo > fan_out_threshold
            && !config.rules.fan_out.exclude.contains(&comp.id)
        {
            violations.push(Violation {
                rule: "fan_out".to_string(),
                component: comp.id.clone(),
                message: format!("fan-out {} exceeds limit {}", fo, fan_out_threshold),
                severity: "warning".to_string(),
            });
        }

        // Check fan-in violation
        if config.rules.fan_in.enabled
            && fi > fan_in_threshold
            && !config.rules.fan_in.exclude.contains(&comp.id)
        {
            violations.push(Violation {
                rule: "fan_in".to_string(),
                component: comp.id.clone(),
                message: format!("fan-in {} exceeds limit {}", fi, fan_in_threshold),
                severity: "info".to_string(),
            });
        }
    }

    // Detect cycles using petgraph
    let cycles = if config.rules.cycles.enabled {
        detect_cycles(graph)
    } else {
        Vec::new()
    };

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

#[cfg(test)]
mod tests {
    use super::*;

    fn make_deps(names: &[&str]) -> HashSet<String> {
        names.iter().map(|s| s.to_string()).collect()
    }

    #[test]
    fn test_external_crate_filtered_from_deps() {
        let external = make_deps(&["tokio", "serde", "anyhow", "tracing", "regex"]);
        let code = "\
use tokio::runtime::Runtime;\n\
use serde::{Serialize, Deserialize};\n\
use anyhow::Result;\n\
use tracing::info;\n\
use crate::model::Component;\n\
use super::utils;\n\
";
        let pf = parse_rust_file(code, "mymod", &external).unwrap();
        assert!(!pf.dependencies.contains(&"tokio".to_string()), "tokio should be filtered");
        assert!(!pf.dependencies.contains(&"serde".to_string()), "serde should be filtered");
        assert!(!pf.dependencies.contains(&"anyhow".to_string()), "anyhow should be filtered");
        assert!(!pf.dependencies.contains(&"tracing".to_string()), "tracing should be filtered");
        assert!(pf.dependencies.contains(&"model".to_string()), "crate::model should be kept");
        assert!(pf.dependencies.contains(&"utils".to_string()), "super::utils should be kept");
    }

    #[test]
    fn test_std_core_alloc_always_filtered() {
        let external = make_deps(&[]);
        let code = "\
use std::collections::HashMap;\n\
use core::fmt;\n\
use alloc::vec::Vec;\n\
use crate::config;\n\
";
        let pf = parse_rust_file(code, "mymod", &external).unwrap();
        assert!(!pf.dependencies.contains(&"std".to_string()), "std should be filtered");
        assert!(!pf.dependencies.contains(&"core".to_string()), "core should be filtered");
        assert!(!pf.dependencies.contains(&"alloc".to_string()), "alloc should be filtered");
        assert!(pf.dependencies.contains(&"config".to_string()), "crate::config should be kept");
    }

    #[test]
    fn test_internal_crate_prefix_kept() {
        let external = make_deps(&["tokio"]);
        let code = "\
use crate::analyzer;\n\
use crate::model::ArchGraph;\n\
use self::helper;\n\
use super::parent;\n\
";
        let pf = parse_rust_file(code, "mymod", &external).unwrap();
        assert!(pf.dependencies.contains(&"analyzer".to_string()));
        assert!(pf.dependencies.contains(&"model".to_string()));
        assert!(pf.dependencies.contains(&"helper".to_string()));
        assert!(pf.dependencies.contains(&"parent".to_string()));
    }

    #[test]
    fn test_is_external_crate_builtin() {
        let empty = HashSet::new();
        assert!(is_external_crate("std", &empty));
        assert!(is_external_crate("core", &empty));
        assert!(is_external_crate("alloc", &empty));
        assert!(!is_external_crate("mymodule", &empty));
    }

    #[test]
    fn test_is_external_crate_cargo_dep() {
        let deps = make_deps(&["tokio", "serde_json"]);
        assert!(is_external_crate("tokio", &deps));
        assert!(is_external_crate("serde_json", &deps));
        assert!(!is_external_crate("mymodule", &deps));
    }

    #[test]
    fn test_fan_out_excludes_external_crates() {
        let external = make_deps(&["tokio", "serde", "anyhow", "tracing", "regex", "rayon"]);
        let code = "\
use tokio::runtime::Runtime;\n\
use serde::{Serialize, Deserialize};\n\
use anyhow::Result;\n\
use tracing::info;\n\
use regex::Regex;\n\
use rayon::prelude::*;\n\
use crate::model;\n\
";
        let pf = parse_rust_file(code, "mymod", &external).unwrap();
        assert_eq!(pf.dependencies.len(), 1);
        assert_eq!(pf.dependencies[0], "model");
    }
}
