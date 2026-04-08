/// LanguageAnalyzer trait: pluggable per-language parsing interface.
///
/// Implementors handle a specific language (Rust, Go, …) and expose:
/// - `detect`           – checks whether a project directory belongs to this language
/// - `file_extensions`  – file suffixes to collect during directory walk
/// - `parse_file`       – parses a single source file into a `ParsedFile`
///
/// The main analyzer picks the right implementor at runtime:
///
/// ```
/// let analyzer = pick_analyzer(path);
/// if analyzer.detect(project_dir) { … }
/// ```
use regex::Regex;
use std::collections::HashSet;
use std::fs;
use std::path::Path;

// ---------------------------------------------------------------------------
// Shared data types
// ---------------------------------------------------------------------------

/// A single trait detected in a Rust source file.
pub struct TraitDef {
    pub name: String,
    pub method_count: usize,
}

/// Go struct with method and field counts (for god-class detection).
pub struct GoStructDef {
    pub name: String,
    pub method_count: usize,
    pub field_count: usize,
}

/// Go method with call information (for feature-envy detection).
pub struct GoMethodDef {
    /// The method name (e.g. "MyMethod").
    pub name: String,
    /// The receiver type (e.g. "MyStruct"). Empty for free functions.
    pub receiver: String,
    /// Number of calls to methods on the same receiver type (own calls).
    pub own_calls: usize,
    /// Number of calls to methods on other receiver types (foreign calls).
    pub other_calls: usize,
}

/// Parsed representation of a single source file.
pub struct ParsedFile {
    pub module_name: String,
    pub language: String,
    pub dependencies: Vec<String>,
    pub structs: Vec<String>,
    #[allow(dead_code)]
    pub functions: Vec<String>,
    /// Rust trait definitions (name + method count). Empty for Go files.
    pub traits: Vec<TraitDef>,
    /// True when at least one struct in this file has a service-like impl block.
    pub has_service_structs: bool,
    /// Go struct definitions with method/field counts. Empty for Rust files.
    pub go_structs: Vec<GoStructDef>,
    /// Go method definitions with call counts for feature-envy detection. Empty for Rust files.
    pub go_methods: Vec<GoMethodDef>,
    /// Total number of lines in the source file.
    pub line_count: usize,
}

// ---------------------------------------------------------------------------
// LanguageAnalyzer trait
// ---------------------------------------------------------------------------

/// Pluggable language analysis interface.
#[allow(dead_code)]
pub trait LanguageAnalyzer: Send + Sync {
    /// Returns true when the given project directory contains this language.
    /// Typically checks for a manifest file (Cargo.toml, go.mod, …).
    fn detect(&self, dir: &Path) -> bool;

    /// File extensions handled by this analyzer (without leading dot, e.g. `"rs"`).
    fn file_extensions(&self) -> &[&str];

    /// Parse a single source file.
    ///
    /// * `path`          – absolute or relative path to the source file
    /// * `base_dir`      – project root (used to derive the module name)
    /// * `external_deps` – set of known external dependency names to skip
    /// * `module_prefix` – language-specific prefix (Go module name, ignored for Rust)
    fn parse_file(
        &self,
        path: &Path,
        base_dir: &Path,
        external_deps: &HashSet<String>,
        module_prefix: &str,
    ) -> Result<ParsedFile, String>;
}

// ---------------------------------------------------------------------------
// Helper: path -> module name
// ---------------------------------------------------------------------------

pub fn path_to_module(rel_path: &Path, ext: &str) -> String {
    let s = rel_path.to_string_lossy();
    let name = s.trim_end_matches(&format!(".{}", ext));
    let name = name.replace('/', "::");
    let name = name.replace('\\', "::");
    // Remove mod suffix for Rust
    let name = name.trim_end_matches("::mod");
    name.to_string()
}

// ---------------------------------------------------------------------------
// RustAnalyzer
// ---------------------------------------------------------------------------

/// Rust-language analyzer.  Detects projects by the presence of Cargo.toml.
#[allow(dead_code)]
pub struct RustAnalyzer;

impl LanguageAnalyzer for RustAnalyzer {
    fn detect(&self, dir: &Path) -> bool {
        dir.join("Cargo.toml").exists()
    }

    fn file_extensions(&self) -> &[&str] {
        &["rs"]
    }

    fn parse_file(
        &self,
        path: &Path,
        base_dir: &Path,
        external_deps: &HashSet<String>,
        _module_prefix: &str,
    ) -> Result<ParsedFile, String> {
        let content = fs::read_to_string(path).map_err(|e| e.to_string())?;
        let rel_path = path.strip_prefix(base_dir).unwrap_or(path);
        let module_name = path_to_module(rel_path, "rs");
        parse_rust_content(&content, &module_name, external_deps)
    }
}

/// Parse Rust source content.
///
/// This is the core Rust parsing logic extracted so it can be unit-tested
/// independently of the filesystem.
pub fn parse_rust_content(
    content: &str,
    module_name: &str,
    external_deps: &HashSet<String>,
) -> Result<ParsedFile, String> {
    // Matches: use crate::foo, use self::foo, use super::foo -> internal (crate-local)
    let re_use_internal = Regex::new(r"^(?:pub\s+)?use\s+(crate|self|super)::").unwrap();
    // Matches: use foo::... -> captures foo as the crate root
    let re_use_external = Regex::new(r"^(?:pub\s+)?use\s+(\w+)").unwrap();
    let re_mod = Regex::new(r"^(?:pub(?:\(crate\))?\s+)?mod\s+(\w+)").unwrap();
    let re_struct = Regex::new(r"^(?:pub(?:\(crate\))?\s+)?struct\s+(\w+)").unwrap();
    let re_fn = Regex::new(r"^(?:pub(?:\(crate\))?\s+)?(?:async\s+)?fn\s+(\w+)").unwrap();
    // Matches trait definitions: `pub trait Foo {` or `trait Foo {`
    let re_trait = Regex::new(r"^(?:pub(?:\(crate\))?\s+)?trait\s+(\w+)").unwrap();
    // Matches trait method signatures (fn inside a trait body).
    let re_trait_fn = Regex::new(r"^\s*(?:pub\s+)?(?:async\s+)?fn\s+\w+").unwrap();
    // Matches any `impl` block start.
    let re_impl = Regex::new(r"^impl\b").unwrap();

    let line_count = content.lines().count();

    let mut deps = Vec::new();
    let mut structs = Vec::new();
    let mut functions = Vec::new();
    let mut traits: Vec<TraitDef> = Vec::new();

    // Brace-depth tracker to detect when inside a trait body.
    let mut in_trait = false;
    let mut current_trait_name = String::new();
    let mut current_trait_methods: usize = 0;
    let mut brace_depth: i32 = 0;
    let mut trait_entry_depth: i32 = 0;

    // Service struct heuristic: scan impl blocks for service-like patterns.
    let mut in_impl = false;
    let mut impl_entry_depth: i32 = 0;
    let mut has_service_structs = false;

    for line in content.lines() {
        let trimmed = line.trim();

        if trimmed.starts_with("//") {
            continue;
        }

        let opens = trimmed.chars().filter(|&c| c == '{').count() as i32;
        let closes = trimmed.chars().filter(|&c| c == '}').count() as i32;

        if in_impl {
            if trimmed.contains("&mut self")
                || trimmed.contains("async fn")
                || trimmed.contains("-> Result<")
                || trimmed.contains("-> anyhow::Result")
            {
                has_service_structs = true;
            }
            brace_depth += opens - closes;
            if brace_depth <= impl_entry_depth {
                in_impl = false;
            }
            continue;
        }

        if in_trait {
            if re_trait_fn.is_match(trimmed) {
                current_trait_methods += 1;
            }
            brace_depth += opens - closes;
            if brace_depth <= trait_entry_depth {
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

        // Detect impl block start.
        if re_impl.is_match(trimmed) {
            let net = opens - closes;
            if net > 0 {
                in_impl = true;
                impl_entry_depth = brace_depth;
                brace_depth += net;
            }
            if trimmed.contains("&mut self")
                || trimmed.contains("async fn")
                || trimmed.contains("-> Result<")
                || trimmed.contains("-> anyhow::Result")
            {
                has_service_structs = true;
            }
            if net <= 0 {
                brace_depth += net;
            }
            continue;
        }

        // Detect trait definition start.
        if let Some(cap) = re_trait.captures(trimmed) {
            let net = opens - closes;
            if net > 0 {
                in_trait = true;
                current_trait_name = cap[1].to_string();
                current_trait_methods = 0;
                trait_entry_depth = brace_depth;
                brace_depth += net;
            } else {
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
            if re_use_internal.is_match(trimmed) {
                let re_internal_name =
                    Regex::new(r"^(?:pub\s+)?use\s+(?:crate|self|super)::(\w+)").unwrap();
                if let Some(cap) = re_internal_name.captures(trimmed) {
                    let short_name = cap[1].to_string();
                    let crate_root = module_name.split("::").next().unwrap_or("");
                    let qualified = if crate_root.is_empty() {
                        short_name
                    } else {
                        format!("{}::{}", crate_root, short_name)
                    };
                    deps.push(qualified);
                }
            } else if let Some(cap) = re_use_external.captures(trimmed) {
                let crate_name = &cap[1];
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

    // Handle a trait that was never closed.
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
        has_service_structs,
        go_structs: Vec::new(),
        go_methods: Vec::new(),
        line_count,
    })
}

/// Returns true if the given crate name should be excluded from metrics.
pub fn is_external_crate(name: &str, cargo_deps: &HashSet<String>) -> bool {
    matches!(name, "std" | "core" | "alloc") || cargo_deps.contains(name)
}

// ---------------------------------------------------------------------------
// GoAnalyzer
// ---------------------------------------------------------------------------

/// Go-language analyzer.  Detects projects by the presence of go.mod.
#[allow(dead_code)]
pub struct GoAnalyzer;

impl LanguageAnalyzer for GoAnalyzer {
    fn detect(&self, dir: &Path) -> bool {
        dir.join("go.mod").exists()
    }

    fn file_extensions(&self) -> &[&str] {
        &["go"]
    }

    fn parse_file(
        &self,
        path: &Path,
        base_dir: &Path,
        _external_deps: &HashSet<String>,
        module_prefix: &str,
    ) -> Result<ParsedFile, String> {
        let content = fs::read_to_string(path).map_err(|e| e.to_string())?;
        let rel_path = path.strip_prefix(base_dir).unwrap_or(path);
        let module_name = path_to_module(rel_path, "go");
        parse_go_content(&content, &module_name, module_prefix)
    }
}

/// Parse Go source content.
pub fn parse_go_content(
    content: &str,
    module_name: &str,
    go_module_prefix: &str,
) -> Result<ParsedFile, String> {
    let re_import = Regex::new(r#""([^"]+)""#).unwrap();
    let re_struct = Regex::new(r"^type\s+(\w+)\s+struct\b").unwrap();
    let re_interface = Regex::new(r"^type\s+(\w+)\s+interface\b").unwrap();
    // Method: func (recvVar *ReceiverType) MethodName(...) - captures both receiver var and type
    let re_method = Regex::new(r"^func\s+\(\s*(\w+)\s+\*?(\w+)\s*\)\s+(\w+)\s*\(").unwrap();
    // Free function (no receiver): func FuncName(...)
    let re_func = Regex::new(r"^func\s+(\w+)\s*\(").unwrap();
    // Method call: receiver.Method( pattern in code - captures receiver name
    let re_method_call = Regex::new(r"\b(\w+)\.\w+\s*\(").unwrap();
    // Struct field lines inside a struct body (lines that start with a word char = field name).
    // Applied to trimmed content (leading whitespace already stripped).
    let re_struct_field = Regex::new(r"^(\w+)\s+\S").unwrap();

    let line_count = content.lines().count();

    let mut deps = Vec::new();
    let mut structs = Vec::new();
    let mut functions = Vec::new();
    let mut in_import = false;

    // State for tracking struct body (to count fields).
    let mut in_struct = false;
    let mut current_struct_name = String::new();
    let mut current_struct_fields = 0usize;
    let mut brace_depth: i32 = 0;
    let mut struct_entry_depth: i32 = 0;

    // Maps struct name -> method count (populated by scanning func declarations).
    let mut struct_method_counts: std::collections::HashMap<String, usize> = std::collections::HashMap::new();
    // Maps struct name -> field count (populated while parsing struct bodies).
    let mut struct_field_counts: std::collections::HashMap<String, usize> = std::collections::HashMap::new();

    // For feature-envy: track each method's receiver and call patterns.
    // We collect (method_name, receiver, own_calls, other_calls).
    let mut go_methods_raw: Vec<GoMethodDef> = Vec::new();

    // Current method context for tracking calls within a method body.
    let mut in_method = false;
    let mut current_method_name = String::new();
    // current_method_receiver_type: the type name (e.g. "Server") - stored in GoMethodDef.receiver
    let mut current_method_receiver_type = String::new();
    // current_method_receiver_var: the variable name (e.g. "s") - used to detect own calls
    let mut current_method_receiver_var = String::new();
    let mut current_method_own_calls = 0usize;
    let mut current_method_other_calls = 0usize;
    let mut method_entry_depth: i32 = 0;

    let strip_prefix = if go_module_prefix.is_empty() {
        String::new()
    } else {
        format!("{}/", go_module_prefix)
    };

    for line in content.lines() {
        let trimmed = line.trim();

        // Skip comments.
        if trimmed.starts_with("//") {
            continue;
        }

        // Handle import blocks.
        if trimmed.starts_with("import (") || trimmed == "import (" {
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
                if !strip_prefix.is_empty() && imp.starts_with(&strip_prefix) {
                    let relative = imp[strip_prefix.len()..].to_string();
                    deps.push(relative);
                } else if strip_prefix.is_empty() {
                    if let Some(last) = imp.rsplit('/').next() {
                        deps.push(last.to_string());
                    }
                }
                // else: external import (different module), skip it
            }
            continue;
        }

        let opens = trimmed.chars().filter(|&c| c == '{').count() as i32;
        let closes = trimmed.chars().filter(|&c| c == '}').count() as i32;

        // Finish method body if we return to entry depth.
        if in_method {
            // Count method calls in this line.
            for cap in re_method_call.captures_iter(trimmed) {
                let recv = cap[1].to_string();
                // Own call: the receiver variable matches the method's receiver variable
                // (e.g. "s" in "func (s *Server) Method()").
                if recv == current_method_receiver_var {
                    current_method_own_calls += 1;
                } else {
                    current_method_other_calls += 1;
                }
            }
            brace_depth += opens - closes;
            if brace_depth <= method_entry_depth {
                // Method body ended - record it.
                go_methods_raw.push(GoMethodDef {
                    name: current_method_name.clone(),
                    receiver: current_method_receiver_type.clone(),
                    own_calls: current_method_own_calls,
                    other_calls: current_method_other_calls,
                });
                in_method = false;
                current_method_name.clear();
                current_method_receiver_type.clear();
                current_method_receiver_var.clear();
                current_method_own_calls = 0;
                current_method_other_calls = 0;
            }
            continue;
        }

        // Track struct body for field counting.
        if in_struct {
            brace_depth += opens - closes;
            if brace_depth <= struct_entry_depth {
                // Struct body ended.
                struct_field_counts.insert(current_struct_name.clone(), current_struct_fields);
                in_struct = false;
                current_struct_name.clear();
                current_struct_fields = 0;
            } else if opens == 0 && !trimmed.is_empty() && trimmed != "{" && trimmed != "}" {
                // Count field lines: non-empty lines inside struct body at depth+1.
                // Skip closing braces and embedded struct starts.
                if re_struct_field.is_match(trimmed) && !trimmed.contains("struct {") && !trimmed.contains("interface {") {
                    current_struct_fields += 1;
                }
            }
            continue;
        }

        // Detect struct type definition.
        if let Some(cap) = re_struct.captures(trimmed) {
            let struct_name = cap[1].to_string();
            structs.push(struct_name.clone());
            if opens > closes {
                // Struct body starts on this line.
                in_struct = true;
                current_struct_name = struct_name;
                current_struct_fields = 0;
                struct_entry_depth = brace_depth;
                brace_depth += opens - closes;
            } else {
                // Empty struct on one line.
                struct_field_counts.insert(struct_name, 0);
                brace_depth += opens - closes;
            }
            continue;
        }

        // Also capture interface types as structs (for component detection).
        if let Some(cap) = re_interface.captures(trimmed) {
            structs.push(cap[1].to_string());
            brace_depth += opens - closes;
            continue;
        }

        // Detect method (has receiver).
        if let Some(cap) = re_method.captures(trimmed) {
            let receiver_var = cap[1].to_string();   // e.g. "s" from "func (s *Server)"
            let receiver_type = cap[2].to_string();  // e.g. "Server"
            let method_name = cap[3].to_string();    // e.g. "Run"
            functions.push(method_name.clone());
            // Increment method count for this receiver type.
            *struct_method_counts.entry(receiver_type.clone()).or_insert(0) += 1;
            if opens > closes {
                // Method body starts on this line.
                in_method = true;
                current_method_name = method_name;
                current_method_receiver_type = receiver_type;
                current_method_receiver_var = receiver_var;
                current_method_own_calls = 0;
                current_method_other_calls = 0;
                method_entry_depth = brace_depth;
                brace_depth += opens - closes;
            } else {
                // Method signature without body (e.g. interface method or one-liner).
                brace_depth += opens - closes;
            }
            continue;
        }

        // Detect free function (no receiver).
        if let Some(cap) = re_func.captures(trimmed) {
            functions.push(cap[1].to_string());
            brace_depth += opens - closes;
            continue;
        }

        brace_depth += opens - closes;
    }

    // If we ended inside a struct (malformed input), save what we have.
    if in_struct && !current_struct_name.is_empty() {
        struct_field_counts.insert(current_struct_name.clone(), current_struct_fields);
    }

    // If we ended inside a method body (malformed input), save what we have.
    if in_method && !current_method_name.is_empty() {
        go_methods_raw.push(GoMethodDef {
            name: current_method_name,
            receiver: current_method_receiver_type,
            own_calls: current_method_own_calls,
            other_calls: current_method_other_calls,
        });
    }

    // Build GoStructDef list combining method counts and field counts.
    let go_structs: Vec<GoStructDef> = structs
        .iter()
        .filter(|_s| {
            // Only include actual structs (not interfaces which were also added to structs).
            // We distinguish by checking if they were added via re_struct (they'll have entries
            // in struct_method_counts or struct_field_counts, or simply appear in struct list).
            true
        })
        .map(|s| GoStructDef {
            name: s.clone(),
            method_count: struct_method_counts.get(s).copied().unwrap_or(0),
            field_count: struct_field_counts.get(s).copied().unwrap_or(0),
        })
        .collect();

    Ok(ParsedFile {
        module_name: module_name.to_string(),
        language: "go".to_string(),
        dependencies: deps,
        structs,
        functions,
        traits: Vec::new(), // Go has interfaces, tracked via structs above
        has_service_structs: false, // Go files not analyzed for DIP heuristic
        go_structs,
        go_methods: go_methods_raw,
        line_count,
    })
}

// ---------------------------------------------------------------------------
// Analyzer registry
// ---------------------------------------------------------------------------

/// Return all built-in language analyzers.
#[allow(dead_code)]
pub fn all_analyzers() -> Vec<Box<dyn LanguageAnalyzer>> {
    vec![Box::new(RustAnalyzer), Box::new(GoAnalyzer)]
}

/// Pick the analyzer that handles the given file extension.
/// Returns `None` if no registered analyzer supports the extension.
#[allow(dead_code)]
pub fn analyzer_for_extension(ext: &str) -> Option<Box<dyn LanguageAnalyzer>> {
    for a in all_analyzers() {
        if a.file_extensions().contains(&ext) {
            return Some(a);
        }
    }
    None
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;
    use std::collections::HashSet;

    fn no_deps() -> HashSet<String> {
        HashSet::new()
    }
    fn make_deps(names: &[&str]) -> HashSet<String> {
        names.iter().map(|s| s.to_string()).collect()
    }

    // ------------------------------------------------------------------
    // RustAnalyzer tests
    // ------------------------------------------------------------------

    #[test]
    fn rust_analyzer_extensions() {
        let a = RustAnalyzer;
        assert!(a.file_extensions().contains(&"rs"));
        assert!(!a.file_extensions().contains(&"go"));
    }

    #[test]
    fn rust_parses_use_crate() {
        let code = "use crate::model;\nuse crate::config::Config;\n";
        let pf = parse_rust_content(code, "src::main", &no_deps()).unwrap();
        assert!(pf.dependencies.contains(&"src::model".to_string()));
        assert!(pf.dependencies.contains(&"src::config".to_string()));
    }

    #[test]
    fn rust_filters_std_core_alloc() {
        let code = "use std::collections::HashMap;\nuse core::fmt;\nuse alloc::vec::Vec;\n";
        let pf = parse_rust_content(code, "mymod", &no_deps()).unwrap();
        assert!(!pf.dependencies.contains(&"std".to_string()));
        assert!(!pf.dependencies.contains(&"core".to_string()));
        assert!(!pf.dependencies.contains(&"alloc".to_string()));
    }

    #[test]
    fn rust_filters_cargo_external_deps() {
        let external = make_deps(&["tokio", "serde", "anyhow"]);
        let code = "use tokio::runtime::Runtime;\nuse serde::Serialize;\nuse crate::model;\n";
        let pf = parse_rust_content(code, "mymod", &external).unwrap();
        assert!(!pf.dependencies.contains(&"tokio".to_string()));
        assert!(!pf.dependencies.contains(&"serde".to_string()));
        assert!(pf.dependencies.contains(&"mymod::model".to_string()));
    }

    #[test]
    fn rust_parses_structs() {
        let code = "pub struct Foo {}\npub(crate) struct Bar;\nstruct Baz;\n";
        let pf = parse_rust_content(code, "mymod", &no_deps()).unwrap();
        assert!(pf.structs.contains(&"Foo".to_string()));
        assert!(pf.structs.contains(&"Bar".to_string()));
        assert!(pf.structs.contains(&"Baz".to_string()));
    }

    #[test]
    fn rust_parses_trait_with_methods() {
        let code = "\
pub trait Repository {\n\
    fn find(&self, id: u64) -> Option<Entity>;\n\
    fn save(&mut self, entity: Entity);\n\
    fn delete(&mut self, id: u64);\n\
}\n";
        let pf = parse_rust_content(code, "mymod", &no_deps()).unwrap();
        assert_eq!(pf.traits.len(), 1);
        assert_eq!(pf.traits[0].name, "Repository");
        assert_eq!(pf.traits[0].method_count, 3);
    }

    #[test]
    fn rust_detects_service_struct_via_mut_self() {
        let code = "\
pub struct Service {}\n\
impl Service {\n\
    pub fn run(&mut self) {}\n\
}\n";
        let pf = parse_rust_content(code, "mymod", &no_deps()).unwrap();
        assert!(pf.has_service_structs);
    }

    #[test]
    fn rust_detects_service_struct_via_async_fn() {
        let code = "\
pub struct Worker {}\n\
impl Worker {\n\
    pub async fn execute(&self) {}\n\
}\n";
        let pf = parse_rust_content(code, "mymod", &no_deps()).unwrap();
        assert!(pf.has_service_structs);
    }

    #[test]
    fn rust_plain_data_struct_not_service() {
        let code = "pub struct Point { x: f64, y: f64 }\n";
        let pf = parse_rust_content(code, "mymod", &no_deps()).unwrap();
        assert!(!pf.has_service_structs);
    }

    #[test]
    fn rust_language_field_is_rust() {
        let code = "";
        let pf = parse_rust_content(code, "mymod", &no_deps()).unwrap();
        assert_eq!(pf.language, "rust");
    }

    // ------------------------------------------------------------------
    // GoAnalyzer tests
    // ------------------------------------------------------------------

    #[test]
    fn go_analyzer_extensions() {
        let a = GoAnalyzer;
        assert!(a.file_extensions().contains(&"go"));
        assert!(!a.file_extensions().contains(&"rs"));
    }

    #[test]
    fn go_parses_internal_imports() {
        let code = "\
package main\n\
\n\
import (\n\
    \"mymodule/internal/repo\"\n\
    \"mymodule/pkg/cache\"\n\
)\n";
        let pf = parse_go_content(code, "cmd::server", "mymodule").unwrap();
        assert!(pf.dependencies.contains(&"internal/repo".to_string()));
        assert!(pf.dependencies.contains(&"pkg/cache".to_string()));
    }

    #[test]
    fn go_skips_external_imports() {
        let code = "\
package main\n\
\n\
import (\n\
    \"fmt\"\n\
    \"net/http\"\n\
    \"mymodule/internal/service\"\n\
)\n";
        let pf = parse_go_content(code, "cmd::main", "mymodule").unwrap();
        // fmt and net/http are not in the module namespace -> skipped
        assert!(!pf.dependencies.contains(&"fmt".to_string()));
        assert!(!pf.dependencies.contains(&"net/http".to_string()));
        assert!(pf.dependencies.contains(&"internal/service".to_string()));
    }

    #[test]
    fn go_parses_structs() {
        let code = "\
type Server struct {\n\
    addr string\n\
}\n\
type Client struct{}\n";
        let pf = parse_go_content(code, "mymod", "").unwrap();
        assert!(pf.structs.contains(&"Server".to_string()));
        assert!(pf.structs.contains(&"Client".to_string()));
    }

    #[test]
    fn go_parses_interfaces() {
        let code = "\
type Repository interface {\n\
    Find(id int) Entity\n\
}\n";
        let pf = parse_go_content(code, "mymod", "").unwrap();
        assert!(pf.structs.contains(&"Repository".to_string()));
    }

    #[test]
    fn go_parses_functions() {
        let code = "\
func NewServer() *Server { return nil }\n\
func (s *Server) Run() error { return nil }\n";
        let pf = parse_go_content(code, "mymod", "").unwrap();
        assert!(pf.functions.contains(&"NewServer".to_string()));
        assert!(pf.functions.contains(&"Run".to_string()));
    }

    #[test]
    fn go_traits_always_empty() {
        let code = "type Repo interface { Find() Entity }\n";
        let pf = parse_go_content(code, "mymod", "").unwrap();
        assert!(pf.traits.is_empty(), "Go parser should never populate traits");
    }

    #[test]
    fn go_language_field_is_go() {
        let code = "";
        let pf = parse_go_content(code, "mymod", "").unwrap();
        assert_eq!(pf.language, "go");
    }

    #[test]
    fn go_has_service_structs_always_false() {
        let code = "func (s *Svc) Run() error { return nil }\n";
        let pf = parse_go_content(code, "mymod", "").unwrap();
        assert!(!pf.has_service_structs);
    }

    // ------------------------------------------------------------------
    // Registry tests
    // ------------------------------------------------------------------

    #[test]
    fn all_analyzers_returns_rust_and_go() {
        let analyzers = all_analyzers();
        let exts: Vec<&str> = analyzers.iter()
            .flat_map(|a| a.file_extensions().iter().copied())
            .collect();
        assert!(exts.contains(&"rs"), "registry must include Rust analyzer");
        assert!(exts.contains(&"go"), "registry must include Go analyzer");
    }

    #[test]
    fn analyzer_for_extension_rs() {
        let a = analyzer_for_extension("rs");
        assert!(a.is_some());
        assert!(a.unwrap().file_extensions().contains(&"rs"));
    }

    #[test]
    fn analyzer_for_extension_go() {
        let a = analyzer_for_extension("go");
        assert!(a.is_some());
        assert!(a.unwrap().file_extensions().contains(&"go"));
    }

    #[test]
    fn analyzer_for_extension_unknown_returns_none() {
        assert!(analyzer_for_extension("py").is_none());
        assert!(analyzer_for_extension("js").is_none());
    }

    // ------------------------------------------------------------------
    // path_to_module helper
    // ------------------------------------------------------------------

    #[test]
    fn path_to_module_rust() {
        use std::path::Path;
        assert_eq!(path_to_module(Path::new("src/analyzer.rs"), "rs"), "src::analyzer");
        assert_eq!(path_to_module(Path::new("src/mod.rs"), "rs"), "src");
    }

    #[test]
    fn path_to_module_go() {
        use std::path::Path;
        assert_eq!(path_to_module(Path::new("cmd/server/main.go"), "go"), "cmd::server::main");
    }

    // ------------------------------------------------------------------
    // Go god-class detection tests
    // ------------------------------------------------------------------

    #[test]
    fn go_struct_method_count_tracked() {
        let code = "\
type Handler struct {}\n\
func (h *Handler) Get() {}\n\
func (h *Handler) Post() {}\n\
func (h *Handler) Put() {}\n\
";
        let pf = parse_go_content(code, "mymod", "").unwrap();
        let handler = pf.go_structs.iter().find(|s| s.name == "Handler");
        assert!(handler.is_some(), "Handler struct should be tracked in go_structs");
        assert_eq!(handler.unwrap().method_count, 3, "Handler should have 3 methods");
    }

    #[test]
    fn go_struct_field_count_tracked() {
        let code = "\
type Config struct {\n\
    Host     string\n\
    Port     int\n\
    Timeout  int\n\
    MaxConns int\n\
}\n\
";
        let pf = parse_go_content(code, "mymod", "").unwrap();
        let cfg = pf.go_structs.iter().find(|s| s.name == "Config");
        assert!(cfg.is_some(), "Config struct should be tracked");
        assert_eq!(cfg.unwrap().field_count, 4, "Config should have 4 fields");
    }

    #[test]
    fn go_struct_no_methods_has_zero_method_count() {
        let code = "\
type Point struct {\n\
    X float64\n\
    Y float64\n\
}\n\
";
        let pf = parse_go_content(code, "mymod", "").unwrap();
        let point = pf.go_structs.iter().find(|s| s.name == "Point");
        assert!(point.is_some(), "Point should be in go_structs");
        assert_eq!(point.unwrap().method_count, 0);
        assert_eq!(point.unwrap().field_count, 2);
    }

    #[test]
    fn go_multiple_structs_tracked_independently() {
        let code = "\
type A struct { x int }\n\
type B struct { y int\n z int }\n\
func (a *A) MethodA1() {}\n\
func (a *A) MethodA2() {}\n\
func (b *B) MethodB1() {}\n\
";
        let pf = parse_go_content(code, "mymod", "").unwrap();
        let a = pf.go_structs.iter().find(|s| s.name == "A").unwrap();
        let b = pf.go_structs.iter().find(|s| s.name == "B").unwrap();
        assert_eq!(a.method_count, 2, "A should have 2 methods");
        assert_eq!(b.method_count, 1, "B should have 1 method");
    }

    // ------------------------------------------------------------------
    // Go feature-envy detection tests
    // ------------------------------------------------------------------

    #[test]
    fn go_method_own_calls_counted() {
        // Method calls methods on the same receiver type.
        // "s.helper()" where receiver is *Server -> own call.
        let code = "\
type Server struct{}\n\
func (s *Server) Process() {\n\
    s.validate()\n\
    s.execute()\n\
}\n\
func (s *Server) validate() {}\n\
func (s *Server) execute() {}\n\
";
        let pf = parse_go_content(code, "mymod", "").unwrap();
        let process = pf.go_methods.iter().find(|m| m.name == "Process");
        assert!(process.is_some(), "Process method should be tracked");
        let p = process.unwrap();
        assert_eq!(p.receiver, "Server");
        // s.validate() and s.execute() -> own calls (receiver matches)
        assert!(p.own_calls >= 2, "Process should have at least 2 own calls, got {}", p.own_calls);
    }

    #[test]
    fn go_method_foreign_calls_counted() {
        // Method calls methods on different types -> foreign calls.
        let code = "\
type OrderService struct{}\n\
func (o *OrderService) CreateOrder() {\n\
    db.Save()\n\
    cache.Set()\n\
    logger.Log()\n\
}\n\
";
        let pf = parse_go_content(code, "mymod", "").unwrap();
        let create = pf.go_methods.iter().find(|m| m.name == "CreateOrder");
        assert!(create.is_some(), "CreateOrder should be tracked");
        let c = create.unwrap();
        // db.Save(), cache.Set(), logger.Log() -> foreign calls
        assert!(c.other_calls >= 3, "CreateOrder should have at least 3 foreign calls, got {}", c.other_calls);
    }

    #[test]
    fn go_method_envy_when_more_foreign_than_own() {
        // A method with more foreign calls than own calls.
        let code = "\
type Processor struct{}\n\
func (p *Processor) Process() {\n\
    external.DoA()\n\
    external.DoB()\n\
    external.DoC()\n\
    external.DoD()\n\
    p.init()\n\
}\n\
func (p *Processor) init() {}\n\
";
        let pf = parse_go_content(code, "mymod", "").unwrap();
        let process = pf.go_methods.iter().find(|m| m.name == "Process");
        assert!(process.is_some());
        let p = process.unwrap();
        assert!(p.other_calls > p.own_calls,
            "Should have more foreign calls ({}) than own calls ({})", p.other_calls, p.own_calls);
    }

    #[test]
    fn go_methods_empty_for_rust_files() {
        let code = "pub struct Foo {}\nimpl Foo { fn bar(&self) {} }\n";
        let pf = parse_rust_content(code, "mymod", &no_deps()).unwrap();
        assert!(pf.go_methods.is_empty(), "Rust files should have empty go_methods");
        assert!(pf.go_structs.is_empty(), "Rust files should have empty go_structs");
    }
}
