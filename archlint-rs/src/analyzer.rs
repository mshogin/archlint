use crate::config::Config;
use crate::model::{
    ArchGraph, Component, GraphEdge, GraphExport, GraphMetadata, GraphMetrics, GraphNode,
    GraphViolation, IndexedGraph, Link, Metrics, Violation,
};
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

/// Load the Go module name from go.mod in the given directory.
/// Returns empty string if go.mod is not found or cannot be parsed.
fn load_go_module_name(dir: &Path) -> String {
    let gomod_path = dir.join("go.mod");
    let content = match fs::read_to_string(&gomod_path) {
        Ok(c) => c,
        Err(_) => return String::new(),
    };
    for line in content.lines() {
        let trimmed = line.trim();
        if let Some(rest) = trimmed.strip_prefix("module ") {
            return rest.trim().to_string();
        }
    }
    String::new()
}

/// Analyze a project directory using the provided config.
pub fn analyze_with_config(dir: &Path, config: &Config) -> Result<ArchGraph, String> {
    let files = collect_source_files(dir);
    let external_deps = load_cargo_external_deps(dir);
    let go_module_name = load_go_module_name(dir);

    // Parse files in parallel using rayon
    let parsed: Vec<ParsedFile> = files
        .par_iter()
        .filter_map(|path| parse_file(path, dir, &external_deps, &go_module_name).ok())
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
    let metrics = calculate_metrics(&graph, &components, &parsed, config);

    Ok(ArchGraph {
        components,
        links,
        metrics: Some(metrics),
    })
}

/// Convert an ArchGraph to the standard GraphExport format for Unix-pipe pipeline.
/// The GraphExport is compatible with Go's model.Graph and can be consumed by Go validators.
pub fn to_graph_export(graph: &ArchGraph, root_dir: &std::path::Path) -> GraphExport {
    // Detect the dominant language from components
    let language = graph
        .components
        .iter()
        .map(|c| c.entity.as_str())
        .fold(std::collections::HashMap::new(), |mut acc, lang| {
            *acc.entry(lang).or_insert(0usize) += 1;
            acc
        })
        .into_iter()
        .max_by_key(|(_, count)| *count)
        .map(|(lang, _)| lang.to_string())
        .unwrap_or_else(|| "unknown".to_string());

    let nodes: Vec<GraphNode> = graph
        .components
        .iter()
        .map(|c| {
            // Extract package and name from the module id (e.g. "src::analyzer" -> package="src", name="analyzer")
            let parts: Vec<&str> = c.id.rsplitn(2, "::").collect();
            let (name, package) = if parts.len() == 2 {
                (parts[0].to_string(), parts[1].to_string())
            } else {
                (c.id.clone(), String::new())
            };
            GraphNode {
                id: c.id.clone(),
                node_type: c.entity.clone(),
                package,
                name,
                file: format!("{}.{}", c.id.replace("::", "/"), if c.entity == "rust" { "rs" } else { "go" }),
                line: 0,
            }
        })
        .collect();

    let edges: Vec<GraphEdge> = graph
        .links
        .iter()
        .map(|l| GraphEdge {
            from: l.from.clone(),
            to: l.to.clone(),
            edge_type: l.link_type.clone().unwrap_or_else(|| "depends".to_string()),
        })
        .collect();

    let metrics = graph.metrics.as_ref().map(|m| GraphMetrics {
        component_count: m.component_count,
        link_count: m.link_count,
        max_fan_out: m.max_fan_out,
        max_fan_in: m.max_fan_in,
        cycles: m.cycles.clone(),
        violations: m
            .violations
            .iter()
            .map(|v| GraphViolation {
                rule: v.rule.clone(),
                component: v.component.clone(),
                message: v.message.clone(),
                severity: v.severity.clone(),
            })
            .collect(),
    });

    let analyzed_at = {
        use std::time::{SystemTime, UNIX_EPOCH};
        let secs = SystemTime::now()
            .duration_since(UNIX_EPOCH)
            .map(|d| d.as_secs())
            .unwrap_or(0);
        // Format as RFC3339-like: YYYY-MM-DDTHH:MM:SSZ
        let s = secs;
        let days = s / 86400;
        let time = s % 86400;
        let h = time / 3600;
        let m = (time % 3600) / 60;
        let sec = time % 60;
        // Days since epoch to date (simplified - approximate)
        let year = 1970 + days / 365;
        let day_of_year = days % 365;
        let month = day_of_year / 30 + 1;
        let day = day_of_year % 30 + 1;
        format!("{:04}-{:02}-{:02}T{:02}:{:02}:{:02}Z", year, month, day, h, m, sec)
    };

    GraphExport {
        nodes,
        edges,
        metadata: GraphMetadata {
            language,
            root_dir: root_dir.to_string_lossy().to_string(),
            analyzed_at,
        },
        metrics,
    }
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

/// A single trait detected in a Rust source file.
struct TraitDef {
    name: String,
    method_count: usize,
}

struct ParsedFile {
    module_name: String,
    language: String,
    dependencies: Vec<String>,
    structs: Vec<String>,
    functions: Vec<String>,
    /// Rust trait definitions (name + method count). Empty for Go files.
    traits: Vec<TraitDef>,
}

/// Parse a single source file for dependencies and declarations.
fn parse_file(path: &Path, base_dir: &Path, external_deps: &HashSet<String>, go_module_name: &str) -> Result<ParsedFile, String> {
    let ext = path.extension().and_then(|e| e.to_str()).unwrap_or("");
    let content = fs::read_to_string(path).map_err(|e| e.to_string())?;
    let rel_path = path.strip_prefix(base_dir).unwrap_or(path);
    let module_name = path_to_module(rel_path, ext);

    match ext {
        "go" => parse_go_file(&content, &module_name, go_module_name),
        "rs" => parse_rust_file(&content, &module_name, external_deps),
        _ => Err(format!("unsupported extension: {}", ext)),
    }
}

fn parse_go_file(content: &str, module_name: &str, go_module_prefix: &str) -> Result<ParsedFile, String> {
    let re_import = Regex::new(r#""([^"]+)""#).unwrap();
    let re_struct = Regex::new(r"type\s+(\w+)\s+struct").unwrap();
    let re_func = Regex::new(r"func\s+(?:\([^)]+\)\s+)?(\w+)").unwrap();

    let mut deps = Vec::new();
    let mut structs = Vec::new();
    let mut functions = Vec::new();
    let mut in_import = false;

    // Build the module prefix to strip: "module/" (e.g. "demo/")
    let strip_prefix = if go_module_prefix.is_empty() {
        String::new()
    } else {
        format!("{}/", go_module_prefix)
    };

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
                // Preserve path relative to module root for internal imports.
                // e.g. "demo/internal/repo" with module "demo" -> "internal/repo"
                // External imports (not starting with module prefix) are skipped.
                if !strip_prefix.is_empty() && imp.starts_with(&strip_prefix) {
                    let relative = imp[strip_prefix.len()..].to_string();
                    deps.push(relative);
                } else if strip_prefix.is_empty() {
                    // No module info: fall back to last segment
                    if let Some(last) = imp.rsplit('/').next() {
                        deps.push(last.to_string());
                    }
                }
                // else: external import (different module), skip it
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
        traits: Vec::new(),
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
    // Matches trait definitions: `pub trait Foo {` or `trait Foo {`
    let re_trait = Regex::new(r"^(?:pub(?:\(crate\))?\s+)?trait\s+(\w+)").unwrap();
    // Matches trait method signatures (fn inside a trait body, with optional visibility/async).
    let re_trait_fn = Regex::new(r"^\s*(?:pub\s+)?(?:async\s+)?fn\s+\w+").unwrap();

    let mut deps = Vec::new();
    let mut structs = Vec::new();
    let mut functions = Vec::new();
    let mut traits: Vec<TraitDef> = Vec::new();

    // Simple brace-depth tracker to detect when we are inside a trait body.
    let mut in_trait = false;
    let mut current_trait_name = String::new();
    let mut current_trait_methods: usize = 0;
    let mut brace_depth: i32 = 0;
    let mut trait_entry_depth: i32 = 0;

    for line in content.lines() {
        let trimmed = line.trim();

        if trimmed.starts_with("//") {
            continue;
        }

        // Track brace depth for trait body detection.
        let opens = trimmed.chars().filter(|&c| c == '{').count() as i32;
        let closes = trimmed.chars().filter(|&c| c == '}').count() as i32;

        if in_trait {
            // Count method signatures inside the trait body.
            if re_trait_fn.is_match(trimmed) {
                current_trait_methods += 1;
            }

            brace_depth += opens - closes;

            if brace_depth <= trait_entry_depth {
                // Exited trait body.
                traits.push(TraitDef {
                    name: current_trait_name.clone(),
                    method_count: current_trait_methods,
                });
                in_trait = false;
                current_trait_name.clear();
                current_trait_methods = 0;
            }
            continue;
        }

        // Detect trait definition start.
        if let Some(cap) = re_trait.captures(trimmed) {
            let net = opens - closes;
            if net > 0 {
                // Trait body spans multiple lines — enter tracking mode.
                in_trait = true;
                current_trait_name = cap[1].to_string();
                current_trait_methods = 0;
                trait_entry_depth = brace_depth;
                brace_depth += net;
            } else {
                // Trait opens and closes on the same line (e.g. `trait Foo {}`).
                // Record it with zero methods; no need to enter in_trait mode.
                traits.push(TraitDef {
                    name: cap[1].to_string(),
                    method_count: 0,
                });
                brace_depth += net;
            }
            continue;
        }

        // Track depth for non-trait code.
        brace_depth += opens - closes;

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

    // Handle a trait that was never closed (malformed file — still record it).
    if in_trait && !current_trait_name.is_empty() {
        traits.push(TraitDef {
            name: current_trait_name,
            method_count: current_trait_methods,
        });
    }

    Ok(ParsedFile {
        module_name: module_name.to_string(),
        language: "rust".to_string(),
        dependencies: deps,
        structs,
        functions,
        traits,
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

fn calculate_metrics(graph: &IndexedGraph, components: &[Component], parsed: &[ParsedFile], config: &Config) -> Metrics {
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

    // ISP: detect traits with too many methods (Rust only).
    if config.rules.isp.enabled {
        let isp_threshold = config.isp_threshold();
        for pf in parsed {
            if pf.language != "rust" {
                continue;
            }
            if config.rules.isp.exclude.contains(&pf.module_name) {
                continue;
            }
            for t in &pf.traits {
                if t.method_count > isp_threshold {
                    violations.push(Violation {
                        rule: "isp".to_string(),
                        component: pf.module_name.clone(),
                        message: format!(
                            "trait `{}` has {} methods, exceeds ISP threshold of {}",
                            t.name, t.method_count, isp_threshold
                        ),
                        severity: "warning".to_string(),
                    });
                }
            }
        }
    }

    // DIP: detect Rust modules that have structs but no trait definitions (missing abstraction layer).
    if config.rules.dip.enabled {
        for pf in parsed {
            if pf.language != "rust" {
                continue;
            }
            if config.rules.dip.exclude.contains(&pf.module_name) {
                continue;
            }
            if pf.structs.len() > 2 && pf.traits.is_empty() {
                violations.push(Violation {
                    rule: "dip".to_string(),
                    component: pf.module_name.clone(),
                    message: format!(
                        "module has {} structs but no trait definitions; consider introducing traits to enforce dependency inversion",
                        pf.structs.len()
                    ),
                    severity: "info".to_string(),
                });
            }
        }
    }

    // Layer dependency rule: detect forbidden cross-layer dependencies.
    if config.has_layer_rules() {
        for comp in components {
            let from_layer = match config.layer_for_module(&comp.id) {
                Some(l) => l,
                None => continue, // component not in any defined layer - skip
            };

            // Collect allowed targets for this layer (empty vec means no deps allowed).
            let allowed: &[String] = config
                .allowed_dependencies
                .get(from_layer)
                .map(|v| v.as_slice())
                .unwrap_or(&[]);

            // Iterate over all direct dependencies of this component.
            if let Some(&from_idx) = graph.node_indices.get(&comp.id) {
                let neighbors: Vec<String> = graph
                    .graph
                    .neighbors_directed(from_idx, petgraph::Direction::Outgoing)
                    .map(|idx| graph.graph[idx].clone())
                    .collect();

                for dep_id in neighbors {
                    let to_layer = match config.layer_for_module(&dep_id) {
                        Some(l) => l,
                        None => continue, // dep not in any layer - ignore
                    };

                    // Same layer is always fine.
                    if to_layer == from_layer {
                        continue;
                    }

                    if !allowed.iter().any(|a| a == to_layer) {
                        let allowed_list = if allowed.is_empty() {
                            "none".to_string()
                        } else {
                            allowed
                                .iter()
                                .map(|s| s.as_str())
                                .collect::<Vec<_>>()
                                .join(", ")
                        };
                        violations.push(Violation {
                            rule: "layer".to_string(),
                            component: comp.id.clone(),
                            message: format!(
                                "VIOLATION: {} -> {} (forbidden, allowed: [{}])",
                                from_layer, to_layer, allowed_list
                            ),
                            severity: "error".to_string(),
                        });
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

    #[test]
    fn test_trait_parsing_single_trait() {
        let empty = make_deps(&[]);
        let code = "\
pub trait Repository {\n\
    fn find(&self, id: u64) -> Option<Entity>;\n\
    fn save(&mut self, entity: Entity);\n\
    fn delete(&mut self, id: u64);\n\
}\n\
";
        let pf = parse_rust_file(code, "mymod", &empty).unwrap();
        assert_eq!(pf.traits.len(), 1);
        assert_eq!(pf.traits[0].name, "Repository");
        assert_eq!(pf.traits[0].method_count, 3);
    }

    #[test]
    fn test_trait_parsing_multiple_traits() {
        let empty = make_deps(&[]);
        let code = "\
trait Reader {\n\
    fn read(&self) -> Vec<u8>;\n\
}\n\
\n\
pub trait Writer {\n\
    fn write(&mut self, data: &[u8]);\n\
    fn flush(&mut self);\n\
}\n\
";
        let pf = parse_rust_file(code, "mymod", &empty).unwrap();
        assert_eq!(pf.traits.len(), 2);
        let names: Vec<&str> = pf.traits.iter().map(|t| t.name.as_str()).collect();
        assert!(names.contains(&"Reader"));
        assert!(names.contains(&"Writer"));
        let reader = pf.traits.iter().find(|t| t.name == "Reader").unwrap();
        assert_eq!(reader.method_count, 1);
        let writer = pf.traits.iter().find(|t| t.name == "Writer").unwrap();
        assert_eq!(writer.method_count, 2);
    }

    #[test]
    fn test_isp_violation_detected() {
        let empty = make_deps(&[]);
        // Trait with 6 methods exceeds default threshold of 5.
        let code = "\
pub trait GodTrait {\n\
    fn method_a(&self);\n\
    fn method_b(&self);\n\
    fn method_c(&self);\n\
    fn method_d(&self);\n\
    fn method_e(&self);\n\
    fn method_f(&self);\n\
}\n\
";
        let pf = parse_rust_file(code, "mymod", &empty).unwrap();
        assert_eq!(pf.traits.len(), 1);
        assert_eq!(pf.traits[0].method_count, 6);

        let config = Config::default(); // ISP threshold = 5
        let graph = IndexedGraph::new();
        let components = vec![crate::model::Component {
            id: "mymod".to_string(),
            title: "mymod".to_string(),
            entity: "rust".to_string(),
        }];
        let metrics = calculate_metrics(&graph, &components, &[pf], &config);
        let isp_violations: Vec<&Violation> = metrics.violations.iter()
            .filter(|v| v.rule == "isp")
            .collect();
        assert_eq!(isp_violations.len(), 1);
        assert!(isp_violations[0].message.contains("GodTrait"));
        assert_eq!(isp_violations[0].severity, "warning");
    }

    #[test]
    fn test_isp_no_violation_within_threshold() {
        let empty = make_deps(&[]);
        // Trait with exactly 5 methods should NOT trigger ISP.
        let code = "\
pub trait SmallTrait {\n\
    fn a(&self);\n\
    fn b(&self);\n\
    fn c(&self);\n\
    fn d(&self);\n\
    fn e(&self);\n\
}\n\
";
        let pf = parse_rust_file(code, "mymod", &empty).unwrap();
        assert_eq!(pf.traits[0].method_count, 5);

        let config = Config::default();
        let graph = IndexedGraph::new();
        let components = vec![crate::model::Component {
            id: "mymod".to_string(),
            title: "mymod".to_string(),
            entity: "rust".to_string(),
        }];
        let metrics = calculate_metrics(&graph, &components, &[pf], &config);
        let isp_violations: Vec<&Violation> = metrics.violations.iter()
            .filter(|v| v.rule == "isp")
            .collect();
        assert!(isp_violations.is_empty());
    }

    #[test]
    fn test_dip_violation_structs_no_traits() {
        let empty = make_deps(&[]);
        // 3 structs, no traits -> DIP violation.
        let code = "\
pub struct Worker {}\n\
pub struct Agent {}\n\
pub struct Dispatcher {}\n\
";
        let pf = parse_rust_file(code, "concrete_module", &empty).unwrap();
        assert_eq!(pf.structs.len(), 3);
        assert!(pf.traits.is_empty());

        let config = Config::default();
        let graph = IndexedGraph::new();
        let components = vec![crate::model::Component {
            id: "concrete_module".to_string(),
            title: "concrete_module".to_string(),
            entity: "rust".to_string(),
        }];
        let metrics = calculate_metrics(&graph, &components, &[pf], &config);
        let dip_violations: Vec<&Violation> = metrics.violations.iter()
            .filter(|v| v.rule == "dip")
            .collect();
        assert_eq!(dip_violations.len(), 1);
        assert_eq!(dip_violations[0].severity, "info");
    }

    #[test]
    fn test_dip_no_violation_when_traits_present() {
        let empty = make_deps(&[]);
        // 3 structs + 1 trait -> no DIP violation.
        let code = "\
pub trait Bus {}\n\
pub struct Worker {}\n\
pub struct Agent {}\n\
pub struct Dispatcher {}\n\
";
        let pf = parse_rust_file(code, "abstracted_module", &empty).unwrap();
        assert_eq!(pf.structs.len(), 3);
        assert_eq!(pf.traits.len(), 1);

        let config = Config::default();
        let graph = IndexedGraph::new();
        let components = vec![crate::model::Component {
            id: "abstracted_module".to_string(),
            title: "abstracted_module".to_string(),
            entity: "rust".to_string(),
        }];
        let metrics = calculate_metrics(&graph, &components, &[pf], &config);
        let dip_violations: Vec<&Violation> = metrics.violations.iter()
            .filter(|v| v.rule == "dip")
            .collect();
        assert!(dip_violations.is_empty());
    }

    #[test]
    fn test_dip_no_violation_two_or_fewer_structs() {
        let empty = make_deps(&[]);
        // Only 2 structs, no traits -> NOT a DIP violation (threshold is >2).
        let code = "\
pub struct Worker {}\n\
pub struct Agent {}\n\
";
        let pf = parse_rust_file(code, "small_module", &empty).unwrap();
        assert_eq!(pf.structs.len(), 2);

        let config = Config::default();
        let graph = IndexedGraph::new();
        let components = vec![crate::model::Component {
            id: "small_module".to_string(),
            title: "small_module".to_string(),
            entity: "rust".to_string(),
        }];
        let metrics = calculate_metrics(&graph, &components, &[pf], &config);
        let dip_violations: Vec<&Violation> = metrics.violations.iter()
            .filter(|v| v.rule == "dip")
            .collect();
        assert!(dip_violations.is_empty());
    }

    #[test]
    fn test_to_graph_export_nodes_and_edges() {
        use crate::model::{ArchGraph, Component, Link, Metrics, Violation};
        use std::path::Path;

        let graph = ArchGraph {
            components: vec![
                Component { id: "src::main".to_string(), title: "src::main".to_string(), entity: "rust".to_string() },
                Component { id: "src::analyzer".to_string(), title: "src::analyzer".to_string(), entity: "rust".to_string() },
            ],
            links: vec![
                Link { from: "src::main".to_string(), to: "src::analyzer".to_string(), method: None, link_type: Some("depends".to_string()) },
            ],
            metrics: Some(Metrics {
                component_count: 2,
                link_count: 1,
                max_fan_out: 1,
                max_fan_in: 1,
                cycles: vec![],
                violations: vec![],
            }),
        };

        let export = to_graph_export(&graph, Path::new("/tmp/myproject"));

        assert_eq!(export.nodes.len(), 2);
        assert_eq!(export.edges.len(), 1);
        assert_eq!(export.metadata.language, "rust");
        assert_eq!(export.metadata.root_dir, "/tmp/myproject");
        assert!(!export.metadata.analyzed_at.is_empty());

        // Check node structure
        let main_node = export.nodes.iter().find(|n| n.id == "src::main").expect("main node not found");
        assert_eq!(main_node.node_type, "rust");
        assert_eq!(main_node.name, "main");
        assert_eq!(main_node.package, "src");

        // Check edge structure
        let edge = &export.edges[0];
        assert_eq!(edge.from, "src::main");
        assert_eq!(edge.to, "src::analyzer");
        assert_eq!(edge.edge_type, "depends");

        // Check metrics included
        let metrics = export.metrics.expect("metrics should be present");
        assert_eq!(metrics.component_count, 2);
        assert_eq!(metrics.link_count, 1);
        assert_eq!(metrics.max_fan_out, 1);
        assert!(metrics.cycles.is_empty());
        assert!(metrics.violations.is_empty());
    }

    #[test]
    fn test_to_graph_export_yaml_serializable() {
        use crate::model::{ArchGraph, Component, Metrics};
        use std::path::Path;

        let graph = ArchGraph {
            components: vec![
                Component { id: "pkg::service".to_string(), title: "pkg::service".to_string(), entity: "rust".to_string() },
            ],
            links: vec![],
            metrics: Some(Metrics {
                component_count: 1,
                link_count: 0,
                max_fan_out: 0,
                max_fan_in: 0,
                cycles: vec![],
                violations: vec![],
            }),
        };

        let export = to_graph_export(&graph, Path::new("."));
        let yaml = serde_yaml::to_string(&export).expect("should serialize to YAML");

        // Verify key fields appear in YAML output
        assert!(yaml.contains("nodes:"));
        assert!(yaml.contains("edges:"));
        assert!(yaml.contains("metadata:"));
        assert!(yaml.contains("language:"));
        assert!(yaml.contains("root_dir:"));
        assert!(yaml.contains("analyzed_at:"));
        assert!(yaml.contains("metrics:"));
        assert!(yaml.contains("type:"));
        assert!(yaml.contains("package:"));
        assert!(yaml.contains("name:"));
    }

    #[test]
    fn test_to_graph_export_language_detection_rust() {
        use crate::model::{ArchGraph, Component, Metrics};
        use std::path::Path;

        let graph = ArchGraph {
            components: vec![
                Component { id: "a".to_string(), title: "a".to_string(), entity: "rust".to_string() },
                Component { id: "b".to_string(), title: "b".to_string(), entity: "rust".to_string() },
                Component { id: "c".to_string(), title: "c".to_string(), entity: "go".to_string() },
            ],
            links: vec![],
            metrics: Some(Metrics { component_count: 3, link_count: 0, max_fan_out: 0, max_fan_in: 0, cycles: vec![], violations: vec![] }),
        };

        let export = to_graph_export(&graph, Path::new("."));
        // rust appears twice, go once -> rust wins
        assert_eq!(export.metadata.language, "rust");
    }

    #[test]
    fn test_to_graph_export_no_metrics() {
        use crate::model::{ArchGraph, Component};
        use std::path::Path;

        let graph = ArchGraph {
            components: vec![
                Component { id: "mod_a".to_string(), title: "mod_a".to_string(), entity: "rust".to_string() },
            ],
            links: vec![],
            metrics: None,
        };

        let export = to_graph_export(&graph, Path::new("."));
        assert!(export.metrics.is_none());

        // YAML should still be valid without metrics field
        let yaml = serde_yaml::to_string(&export).expect("should serialize");
        assert!(!yaml.contains("metrics:"));
    }

    // --- Tarjan SCC cycle detection tests ---

    fn build_cycle_graph(edges: &[(&str, &str)]) -> IndexedGraph {
        let mut g = IndexedGraph::new();
        for (from, to) in edges {
            g.add_edge(from, to, "depends");
        }
        g
    }

    fn sorted_cycle(mut v: Vec<String>) -> Vec<String> {
        v.sort();
        v
    }

    /// A -> B -> A  (simple mutual dependency, 2-node cycle)
    #[test]
    fn test_detect_cycles_simple_two_node() {
        let g = build_cycle_graph(&[("a", "b"), ("b", "a")]);
        let cycles = detect_cycles(&g);
        assert_eq!(cycles.len(), 1, "expected exactly one SCC");
        let members = sorted_cycle(cycles[0].clone());
        assert_eq!(members, vec!["a".to_string(), "b".to_string()]);
    }

    /// A -> B -> C -> A  (3-node cycle starting from the root)
    #[test]
    fn test_detect_cycles_three_node_cycle() {
        let g = build_cycle_graph(&[("a", "b"), ("b", "c"), ("c", "a")]);
        let cycles = detect_cycles(&g);
        assert_eq!(cycles.len(), 1, "expected exactly one SCC");
        let members = sorted_cycle(cycles[0].clone());
        assert_eq!(members, vec!["a".to_string(), "b".to_string(), "c".to_string()]);
    }

    /// A -> B -> C -> B  (cycle that does NOT include the entry node A)
    /// Simple DFS from A would miss this; Tarjan's SCC must find it.
    #[test]
    fn test_detect_cycles_non_root_cycle() {
        let g = build_cycle_graph(&[("a", "b"), ("b", "c"), ("c", "b")]);
        let cycles = detect_cycles(&g);
        assert_eq!(cycles.len(), 1, "expected exactly one SCC (b <-> c)");
        let members = sorted_cycle(cycles[0].clone());
        assert_eq!(members, vec!["b".to_string(), "c".to_string()]);
    }

    /// Acyclic graph: A -> B -> C  (no cycles expected)
    #[test]
    fn test_detect_cycles_acyclic_graph() {
        let g = build_cycle_graph(&[("a", "b"), ("b", "c")]);
        let cycles = detect_cycles(&g);
        assert!(cycles.is_empty(), "acyclic graph should have no cycles");
    }

    /// Multiple independent cycles: A->B->A and C->D->C
    #[test]
    fn test_detect_cycles_multiple_independent_cycles() {
        let g = build_cycle_graph(&[("a", "b"), ("b", "a"), ("c", "d"), ("d", "c")]);
        let cycles = detect_cycles(&g);
        assert_eq!(cycles.len(), 2, "expected two independent SCCs");

        let mut all_members: Vec<Vec<String>> = cycles.into_iter().map(sorted_cycle).collect();
        all_members.sort();
        assert_eq!(all_members[0], vec!["a".to_string(), "b".to_string()]);
        assert_eq!(all_members[1], vec!["c".to_string(), "d".to_string()]);
    }

    /// Single node with self-loop: A -> A
    #[test]
    fn test_detect_cycles_self_loop() {
        let g = build_cycle_graph(&[("a", "a")]);
        // petgraph tarjan_scc reports self-loops as SCC of size 1 (the node is its own SCC).
        // Our detect_cycles only reports SCCs with len > 1, so no cycle reported.
        // This matches the expected behaviour: self-loops are filtered out.
        let cycles = detect_cycles(&g);
        assert!(cycles.is_empty(), "self-loop on a single node should not be reported as a cycle by Tarjan SCC (size == 1)");
    }

    // ---- layer rule tests ----

    fn build_layer_config() -> Config {
        use crate::config::LayerDef;
        use std::collections::HashMap;

        let layers = vec![
            LayerDef { name: "handler".to_string(), paths: vec!["internal/handler".to_string()] },
            LayerDef { name: "service".to_string(), paths: vec!["internal/service".to_string()] },
            LayerDef { name: "repo".to_string(),    paths: vec!["internal/repo".to_string()] },
            LayerDef { name: "model".to_string(),   paths: vec!["internal/model".to_string()] },
        ];

        let mut allowed = HashMap::new();
        allowed.insert("handler".to_string(), vec!["service".to_string(), "model".to_string()]);
        allowed.insert("service".to_string(), vec!["repo".to_string(), "model".to_string()]);
        allowed.insert("repo".to_string(),    vec!["model".to_string()]);
        allowed.insert("model".to_string(),   vec![]);

        let mut cfg = Config::default();
        cfg.layers = layers;
        cfg.allowed_dependencies = allowed;
        cfg
    }

    /// Helper: build a minimal ArchGraph from (from, to) edges and run calculate_metrics.
    fn run_metrics_with_config(edges: &[(&str, &str)], config: &Config) -> Vec<Violation> {
        let mut graph = IndexedGraph::new();
        let mut components = Vec::new();
        let mut parsed = Vec::new();

        // Collect unique node ids
        let mut nodes: Vec<String> = Vec::new();
        for (f, t) in edges {
            for id in &[*f, *t] {
                if !nodes.contains(&id.to_string()) {
                    nodes.push(id.to_string());
                }
            }
        }

        for id in &nodes {
            graph.add_node(id);
            components.push(Component { id: id.clone(), title: id.clone(), entity: "go".to_string() });
            parsed.push(ParsedFile {
                module_name: id.clone(),
                language: "go".to_string(),
                dependencies: Vec::new(),
                structs: Vec::new(),
                functions: Vec::new(),
                traits: Vec::new(),
            });
        }
        for (f, t) in edges {
            graph.add_edge(f, t, "depends");
        }

        let metrics = calculate_metrics(&graph, &components, &parsed, config);
        metrics.violations
    }

    #[test]
    fn test_layer_allowed_dependency_no_violation() {
        // handler -> service: allowed
        let cfg = build_layer_config();
        let violations = run_metrics_with_config(
            &[("internal/handler/users", "internal/service/users")],
            &cfg,
        );
        let layer_violations: Vec<_> = violations.iter().filter(|v| v.rule == "layer").collect();
        assert!(layer_violations.is_empty(), "handler -> service should be allowed, got: {:?}", layer_violations);
    }

    #[test]
    fn test_layer_forbidden_handler_to_repo() {
        // handler -> repo: FORBIDDEN (allowed: service, model)
        let cfg = build_layer_config();
        let violations = run_metrics_with_config(
            &[("internal/handler/orders", "internal/repo/orders")],
            &cfg,
        );
        let layer_violations: Vec<_> = violations.iter().filter(|v| v.rule == "layer").collect();
        assert_eq!(layer_violations.len(), 1);
        assert!(
            layer_violations[0].message.contains("VIOLATION: handler -> repo"),
            "unexpected message: {}",
            layer_violations[0].message
        );
        assert!(
            layer_violations[0].message.contains("allowed: ["),
            "message should list allowed layers: {}",
            layer_violations[0].message
        );
        assert_eq!(layer_violations[0].severity, "error");
    }

    #[test]
    fn test_layer_allowed_service_to_repo() {
        // service -> repo: allowed
        let cfg = build_layer_config();
        let violations = run_metrics_with_config(
            &[("internal/service/orders", "internal/repo/orders")],
            &cfg,
        );
        let layer_violations: Vec<_> = violations.iter().filter(|v| v.rule == "layer").collect();
        assert!(layer_violations.is_empty(), "service -> repo should be allowed");
    }

    #[test]
    fn test_layer_model_no_deps_allowed() {
        // model -> service: FORBIDDEN (model allowed deps: none)
        let cfg = build_layer_config();
        let violations = run_metrics_with_config(
            &[("internal/model/user", "internal/service/users")],
            &cfg,
        );
        let layer_violations: Vec<_> = violations.iter().filter(|v| v.rule == "layer").collect();
        assert_eq!(layer_violations.len(), 1);
        assert!(
            layer_violations[0].message.contains("VIOLATION: model -> service"),
            "unexpected message: {}",
            layer_violations[0].message
        );
        assert!(
            layer_violations[0].message.contains("allowed: [none]"),
            "should show 'none' when no deps allowed: {}",
            layer_violations[0].message
        );
    }

    #[test]
    fn test_layer_dep_not_in_any_layer_is_ignored() {
        // handler -> pkg/utils (not in any layer): should be ignored
        let cfg = build_layer_config();
        let violations = run_metrics_with_config(
            &[("internal/handler/users", "pkg/utils")],
            &cfg,
        );
        let layer_violations: Vec<_> = violations.iter().filter(|v| v.rule == "layer").collect();
        assert!(layer_violations.is_empty(), "dep to unlayered module should be ignored");
    }

    #[test]
    fn test_layer_same_layer_is_allowed() {
        // handler -> handler: within same layer, always fine
        let cfg = build_layer_config();
        let violations = run_metrics_with_config(
            &[("internal/handler/users", "internal/handler/orders")],
            &cfg,
        );
        let layer_violations: Vec<_> = violations.iter().filter(|v| v.rule == "layer").collect();
        assert!(layer_violations.is_empty(), "same-layer dep should not be flagged");
    }

    #[test]
    fn test_layer_rules_not_checked_when_no_config() {
        // No layers configured -> no layer violations
        let cfg = Config::default();
        let violations = run_metrics_with_config(
            &[("internal/handler/users", "internal/repo/orders")],
            &cfg,
        );
        let layer_violations: Vec<_> = violations.iter().filter(|v| v.rule == "layer").collect();
        assert!(layer_violations.is_empty(), "no layer config -> no layer violations");
    }

    // ---- Go import path tests ----

    #[test]
    fn test_go_import_stripped_to_relative_path() {
        // "demo/internal/repo" with module "demo" -> dep stored as "internal/repo"
        let code = r#"package handler

import (
	"demo/internal/repo"
	"demo/internal/service"
	"fmt"
)
"#;
        let pf = parse_go_file(code, "internal/handler/users", "demo").unwrap();
        assert!(
            pf.dependencies.contains(&"internal/repo".to_string()),
            "expected internal/repo in deps, got: {:?}", pf.dependencies
        );
        assert!(
            pf.dependencies.contains(&"internal/service".to_string()),
            "expected internal/service in deps, got: {:?}", pf.dependencies
        );
        // External stdlib import "fmt" should NOT be included
        assert!(
            !pf.dependencies.contains(&"fmt".to_string()),
            "external import fmt should not be in deps: {:?}", pf.dependencies
        );
    }

    #[test]
    fn test_go_import_external_modules_skipped() {
        // Imports from other modules (not the project module) should be skipped
        let code = r#"package handler

import (
	"demo/internal/service"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)
"#;
        let pf = parse_go_file(code, "internal/handler/users", "demo").unwrap();
        assert_eq!(pf.dependencies, vec!["internal/service".to_string()]);
    }

    #[test]
    fn test_go_import_no_module_prefix_fallback() {
        // When no module prefix known, fall back to last segment (old behavior)
        let code = r#"package handler

import (
	"demo/internal/repo"
)
"#;
        let pf = parse_go_file(code, "internal/handler/users", "").unwrap();
        assert!(
            pf.dependencies.contains(&"repo".to_string()),
            "without module prefix, should fall back to last segment: {:?}", pf.dependencies
        );
    }

    #[test]
    fn test_go_layer_violation_detected_via_parse() {
        // End-to-end: parse handler importing repo -> should produce layer violation
        let handler_code = r#"package handler

import (
	"demo/internal/repo"
)
"#;
        let handler_pf = parse_go_file(handler_code, "internal/handler/orders", "demo").unwrap();
        // handler imports "internal/repo" directly, bypassing service layer

        let cfg = build_layer_config();
        let mut graph = IndexedGraph::new();
        let mut components = Vec::new();
        let mut parsed = Vec::new();

        graph.add_node("internal/handler/orders");
        graph.add_node("internal/repo");
        components.push(Component {
            id: "internal/handler/orders".to_string(),
            title: "internal/handler/orders".to_string(),
            entity: "go".to_string(),
        });
        components.push(Component {
            id: "internal/repo".to_string(),
            title: "internal/repo".to_string(),
            entity: "go".to_string(),
        });

        // Add the edge from handler to repo (from parsed deps)
        for dep in &handler_pf.dependencies {
            graph.add_edge("internal/handler/orders", dep, "depends");
        }

        parsed.push(handler_pf);
        parsed.push(ParsedFile {
            module_name: "internal/repo".to_string(),
            language: "go".to_string(),
            dependencies: Vec::new(),
            structs: Vec::new(),
            functions: Vec::new(),
            traits: Vec::new(),
        });

        let metrics = calculate_metrics(&graph, &components, &parsed, &cfg);
        let layer_violations: Vec<_> = metrics.violations.iter().filter(|v| v.rule == "layer").collect();
        assert_eq!(layer_violations.len(), 1, "handler -> repo should be a violation, got: {:?}", layer_violations);
        assert!(
            layer_violations[0].message.contains("handler -> repo"),
            "unexpected violation message: {}", layer_violations[0].message
        );
    }

    #[test]
    fn test_load_go_module_name() {
        use tempfile::TempDir;
        use std::io::Write;

        let dir = TempDir::new().unwrap();
        let gomod_path = dir.path().join("go.mod");
        let mut f = std::fs::File::create(&gomod_path).unwrap();
        f.write_all(b"module demo\n\ngo 1.21\n").unwrap();

        let name = load_go_module_name(dir.path());
        assert_eq!(name, "demo");
    }

    #[test]
    fn test_load_go_module_name_missing_file() {
        use tempfile::TempDir;

        let dir = TempDir::new().unwrap();
        let name = load_go_module_name(dir.path());
        assert_eq!(name, "");
    }
}
