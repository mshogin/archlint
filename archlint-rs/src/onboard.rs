/// Adaptive onboarding for new users: detect project structure and generate
/// a tailored `.archlint.yaml` configuration.
///
/// Usage: `archlint init [--dir <path>] [--dry-run]`
use std::collections::HashMap;
use std::path::{Path, PathBuf};

/// Detected project language.
#[derive(Debug, Clone, PartialEq)]
pub enum Language {
    Go,
    Rust,
    TypeScript,
    Python,
}

impl Language {
    pub fn name(&self) -> &'static str {
        match self {
            Language::Go => "Go",
            Language::Rust => "Rust",
            Language::TypeScript => "TypeScript",
            Language::Python => "Python",
        }
    }

    pub fn manifest(&self) -> &'static str {
        match self {
            Language::Go => "go.mod",
            Language::Rust => "Cargo.toml",
            Language::TypeScript => "package.json",
            Language::Python => "pyproject.toml / setup.py",
        }
    }
}

/// Detected project layout style.
#[derive(Debug, Clone, PartialEq)]
pub enum Layout {
    /// Single workspace root, one language, flat or shallow hierarchy.
    Flat,
    /// Standard layered layout (handler/service/repo or domain/app/infra).
    Layered,
    /// Workspace or monorepo: multiple sub-projects at the root.
    Monorepo,
}

impl Layout {
    pub fn name(&self) -> &'static str {
        match self {
            Layout::Flat => "flat",
            Layout::Layered => "layered",
            Layout::Monorepo => "monorepo",
        }
    }
}

/// A single layer definition to be emitted in the YAML config.
#[derive(Debug, Clone)]
pub struct Layer {
    pub name: String,
    pub paths: Vec<String>,
}

/// Result produced by the onboarding scan.
#[derive(Debug)]
pub struct OnboardResult {
    pub languages: Vec<Language>,
    pub layout: Layout,
    pub layers: Vec<Layer>,
    pub allowed_dependencies: HashMap<String, Vec<String>>,
    /// The generated YAML config text.
    pub config_yaml: String,
    /// Human-readable summary of detected structure and suggested next steps.
    pub summary: String,
}

// ---------------------------------------------------------------------------
// Language detection
// ---------------------------------------------------------------------------

/// Detect project languages by scanning for manifest files.
pub fn detect_languages(dir: &Path) -> Vec<Language> {
    let mut langs = Vec::new();
    if dir.join("go.mod").exists() {
        langs.push(Language::Go);
    }
    if dir.join("Cargo.toml").exists() {
        langs.push(Language::Rust);
    }
    if dir.join("package.json").exists() {
        langs.push(Language::TypeScript);
    }
    if dir.join("pyproject.toml").exists() || dir.join("setup.py").exists() {
        langs.push(Language::Python);
    }
    langs
}

// ---------------------------------------------------------------------------
// Layout detection helpers
// ---------------------------------------------------------------------------

fn dir_exists(base: &Path, name: &str) -> bool {
    base.join(name).is_dir()
}

fn any_subdir_has_manifest(dir: &Path) -> bool {
    let Ok(entries) = std::fs::read_dir(dir) else {
        return false;
    };
    for entry in entries.flatten() {
        let p = entry.path();
        if !p.is_dir() {
            continue;
        }
        if p.join("go.mod").exists()
            || p.join("Cargo.toml").exists()
            || p.join("package.json").exists()
        {
            return true;
        }
    }
    false
}

// ---------------------------------------------------------------------------
// Per-language layer detection
// ---------------------------------------------------------------------------

fn detect_go_layers(dir: &Path) -> (Vec<Layer>, HashMap<String, Vec<String>>) {
    // Classic Go project layout: cmd, internal, pkg
    let has_cmd = dir_exists(dir, "cmd");
    let has_internal = dir_exists(dir, "internal");
    let has_pkg = dir_exists(dir, "pkg");

    // Layered internal subpackages
    let has_handler = dir_exists(dir, "internal/handler")
        || dir_exists(dir, "internal/api")
        || dir_exists(dir, "internal/delivery");
    let has_service = dir_exists(dir, "internal/service")
        || dir_exists(dir, "internal/usecase")
        || dir_exists(dir, "internal/domain");
    let has_repo = dir_exists(dir, "internal/repo")
        || dir_exists(dir, "internal/repository")
        || dir_exists(dir, "internal/storage");

    if has_cmd && has_internal && (has_handler || has_service || has_repo) {
        // Full Clean/Hexagonal layout
        let mut layers = Vec::new();
        let mut allowed: HashMap<String, Vec<String>> = HashMap::new();

        layers.push(Layer {
            name: "cmd".into(),
            paths: vec!["cmd".into()],
        });

        if has_handler {
            let handler_path = if dir_exists(dir, "internal/handler") {
                "internal/handler"
            } else if dir_exists(dir, "internal/api") {
                "internal/api"
            } else {
                "internal/delivery"
            };
            layers.push(Layer {
                name: "handler".into(),
                paths: vec![handler_path.into()],
            });
        }

        if has_service {
            let service_path = if dir_exists(dir, "internal/service") {
                "internal/service"
            } else if dir_exists(dir, "internal/usecase") {
                "internal/usecase"
            } else {
                "internal/domain"
            };
            layers.push(Layer {
                name: "service".into(),
                paths: vec![service_path.into()],
            });
        }

        if has_repo {
            let repo_path = if dir_exists(dir, "internal/repo") {
                "internal/repo"
            } else if dir_exists(dir, "internal/repository") {
                "internal/repository"
            } else {
                "internal/storage"
            };
            layers.push(Layer {
                name: "repo".into(),
                paths: vec![repo_path.into()],
            });
        }

        if has_pkg {
            layers.push(Layer {
                name: "pkg".into(),
                paths: vec!["pkg".into()],
            });
        }

        // Build allowed_dependencies
        let has_h = layers.iter().any(|l| l.name == "handler");
        let has_s = layers.iter().any(|l| l.name == "service");
        let has_r = layers.iter().any(|l| l.name == "repo");
        let has_p = layers.iter().any(|l| l.name == "pkg");

        allowed.insert("cmd".into(), {
            let mut v = Vec::new();
            if has_h { v.push("handler".into()); }
            if has_s { v.push("service".into()); }
            v
        });
        if has_h {
            allowed.insert("handler".into(), {
                let mut v = Vec::new();
                if has_s { v.push("service".into()); }
                if has_p { v.push("pkg".into()); }
                v
            });
        }
        if has_s {
            allowed.insert("service".into(), {
                let mut v = Vec::new();
                if has_r { v.push("repo".into()); }
                if has_p { v.push("pkg".into()); }
                v
            });
        }
        if has_r {
            allowed.insert("repo".into(), {
                let mut v = Vec::new();
                if has_p { v.push("pkg".into()); }
                v
            });
        }
        if has_p {
            allowed.insert("pkg".into(), vec![]);
        }

        return (layers, allowed);
    }

    if has_cmd && has_internal {
        // Simple cmd/internal layout
        let layers = vec![
            Layer { name: "cmd".into(), paths: vec!["cmd".into()] },
            Layer { name: "internal".into(), paths: vec!["internal".into()] },
        ];
        let mut allowed = HashMap::new();
        allowed.insert("cmd".into(), vec!["internal".into()]);
        allowed.insert("internal".into(), vec![]);
        return (layers, allowed);
    }

    // Fallback: no recognised layers
    (Vec::new(), HashMap::new())
}

fn detect_rust_layers(dir: &Path) -> (Vec<Layer>, HashMap<String, Vec<String>>) {
    let src = dir.join("src");
    if !src.is_dir() {
        return (Vec::new(), HashMap::new());
    }

    // Domain / App / Infra pattern
    let has_domain = dir_exists(&src, "domain");
    let has_app = dir_exists(&src, "app") || dir_exists(&src, "application");
    let has_infra = dir_exists(&src, "infra") || dir_exists(&src, "infrastructure");

    if has_domain || has_app || has_infra {
        let mut layers = Vec::new();
        let mut allowed: HashMap<String, Vec<String>> = HashMap::new();

        if has_domain {
            layers.push(Layer {
                name: "domain".into(),
                paths: vec!["src/domain".into()],
            });
        }
        if has_app {
            let app_path = if dir_exists(&src, "app") { "src/app" } else { "src/application" };
            layers.push(Layer {
                name: "app".into(),
                paths: vec![app_path.into()],
            });
        }
        if has_infra {
            let infra_path = if dir_exists(&src, "infra") { "src/infra" } else { "src/infrastructure" };
            layers.push(Layer {
                name: "infra".into(),
                paths: vec![infra_path.into()],
            });
        }

        let has_d = layers.iter().any(|l| l.name == "domain");
        let has_a = layers.iter().any(|l| l.name == "app");
        let has_i = layers.iter().any(|l| l.name == "infra");

        if has_d {
            allowed.insert("domain".into(), vec![]);
        }
        if has_a {
            let mut v = Vec::new();
            if has_d { v.push("domain".into()); }
            allowed.insert("app".into(), v);
        }
        if has_i {
            let mut v = Vec::new();
            if has_d { v.push("domain".into()); }
            if has_a { v.push("app".into()); }
            allowed.insert("infra".into(), v);
        }

        return (layers, allowed);
    }

    // Flat src/ structure - no layer suggestions
    (Vec::new(), HashMap::new())
}

fn detect_ts_layers(dir: &Path) -> (Vec<Layer>, HashMap<String, Vec<String>>) {
    // Common TypeScript/Node project layouts
    let src = if dir.join("src").is_dir() { dir.join("src") } else { dir.to_path_buf() };

    let has_controllers = dir_exists(&src, "controllers") || dir_exists(&src, "routes");
    let has_services = dir_exists(&src, "services");
    let has_models = dir_exists(&src, "models") || dir_exists(&src, "entities");
    let has_repos = dir_exists(&src, "repositories") || dir_exists(&src, "repos");

    if has_controllers || has_services || has_models {
        let mut layers = Vec::new();
        let mut allowed: HashMap<String, Vec<String>> = HashMap::new();

        let src_prefix = if dir.join("src").is_dir() { "src/" } else { "" };

        if has_controllers {
            let path = if dir_exists(&src, "controllers") {
                format!("{}controllers", src_prefix)
            } else {
                format!("{}routes", src_prefix)
            };
            layers.push(Layer { name: "controller".into(), paths: vec![path] });
        }
        if has_services {
            layers.push(Layer {
                name: "service".into(),
                paths: vec![format!("{}services", src_prefix)],
            });
        }
        if has_repos {
            let path = if dir_exists(&src, "repositories") {
                format!("{}repositories", src_prefix)
            } else {
                format!("{}repos", src_prefix)
            };
            layers.push(Layer { name: "repository".into(), paths: vec![path] });
        }
        if has_models {
            let path = if dir_exists(&src, "models") {
                format!("{}models", src_prefix)
            } else {
                format!("{}entities", src_prefix)
            };
            layers.push(Layer { name: "model".into(), paths: vec![path] });
        }

        let has_c = layers.iter().any(|l| l.name == "controller");
        let has_s = layers.iter().any(|l| l.name == "service");
        let has_r = layers.iter().any(|l| l.name == "repository");
        let has_m = layers.iter().any(|l| l.name == "model");

        if has_c {
            let mut v = Vec::new();
            if has_s { v.push("service".into()); }
            if has_m { v.push("model".into()); }
            allowed.insert("controller".into(), v);
        }
        if has_s {
            let mut v = Vec::new();
            if has_r { v.push("repository".into()); }
            if has_m { v.push("model".into()); }
            allowed.insert("service".into(), v);
        }
        if has_r {
            let mut v = Vec::new();
            if has_m { v.push("model".into()); }
            allowed.insert("repository".into(), v);
        }
        if has_m {
            allowed.insert("model".into(), vec![]);
        }

        return (layers, allowed);
    }

    (Vec::new(), HashMap::new())
}

fn detect_python_layers(dir: &Path) -> (Vec<Layer>, HashMap<String, Vec<String>>) {
    // Django-style or clean architecture
    let has_views = dir_exists(dir, "views") || dir_exists(dir, "api");
    let has_services = dir_exists(dir, "services");
    let has_models = dir_exists(dir, "models");
    let has_repos = dir_exists(dir, "repositories");

    // Check for src/ layout
    let src = dir.join("src");
    let (base, prefix) = if src.is_dir() {
        (src.as_path(), "src/")
    } else {
        (dir, "")
    };

    let has_views = has_views || dir_exists(base, "views") || dir_exists(base, "api");
    let has_services = has_services || dir_exists(base, "services");
    let has_models = has_models || dir_exists(base, "models");
    let has_repos = has_repos || dir_exists(base, "repositories");

    if has_views || has_services || has_models {
        let mut layers = Vec::new();
        let mut allowed: HashMap<String, Vec<String>> = HashMap::new();

        if has_views {
            let path = if dir_exists(base, "api") {
                format!("{}api", prefix)
            } else {
                format!("{}views", prefix)
            };
            layers.push(Layer { name: "view".into(), paths: vec![path] });
        }
        if has_services {
            layers.push(Layer {
                name: "service".into(),
                paths: vec![format!("{}services", prefix)],
            });
        }
        if has_repos {
            layers.push(Layer {
                name: "repository".into(),
                paths: vec![format!("{}repositories", prefix)],
            });
        }
        if has_models {
            layers.push(Layer {
                name: "model".into(),
                paths: vec![format!("{}models", prefix)],
            });
        }

        let has_v = layers.iter().any(|l| l.name == "view");
        let has_s = layers.iter().any(|l| l.name == "service");
        let has_r = layers.iter().any(|l| l.name == "repository");
        let has_m = layers.iter().any(|l| l.name == "model");

        if has_v {
            let mut v = Vec::new();
            if has_s { v.push("service".into()); }
            if has_m { v.push("model".into()); }
            allowed.insert("view".into(), v);
        }
        if has_s {
            let mut v = Vec::new();
            if has_r { v.push("repository".into()); }
            if has_m { v.push("model".into()); }
            allowed.insert("service".into(), v);
        }
        if has_r {
            let mut v = Vec::new();
            if has_m { v.push("model".into()); }
            allowed.insert("repository".into(), v);
        }
        if has_m {
            allowed.insert("model".into(), vec![]);
        }

        return (layers, allowed);
    }

    (Vec::new(), HashMap::new())
}

// ---------------------------------------------------------------------------
// Layout detection
// ---------------------------------------------------------------------------

pub fn detect_layout(dir: &Path, languages: &[Language]) -> Layout {
    if any_subdir_has_manifest(dir) {
        return Layout::Monorepo;
    }

    // Check if any layered structure is found for detected languages
    for lang in languages {
        let (layers, _) = match lang {
            Language::Go => detect_go_layers(dir),
            Language::Rust => detect_rust_layers(dir),
            Language::TypeScript => detect_ts_layers(dir),
            Language::Python => detect_python_layers(dir),
        };
        if !layers.is_empty() {
            return Layout::Layered;
        }
    }

    Layout::Flat
}

// ---------------------------------------------------------------------------
// YAML generation
// ---------------------------------------------------------------------------

fn write_rule_block(
    out: &mut String,
    name: &str,
    level: &str,
    threshold: Option<usize>,
    comment: &str,
) {
    out.push_str(&format!("  {}:\n", name));
    out.push_str(&format!("    level: {}    # {}\n", level, comment));
    if let Some(t) = threshold {
        out.push_str(&format!("    threshold: {}\n", t));
    }
}

fn generate_yaml(
    languages: &[Language],
    layout: &Layout,
    layers: &[Layer],
    allowed: &HashMap<String, Vec<String>>,
) -> String {
    let mut out = String::new();
    out.push_str("# .archlint.yaml - generated by `archlint init`\n");
    out.push_str("# Metric levels:\n");
    out.push_str("#   taboo     - CI blocker: exit code 1, shown in red\n");
    out.push_str("#   telemetry - track only: exit code 0, shown in yellow\n");
    out.push_str("#   personal  - informational: exit code 0, shown in default color\n");
    out.push_str("\n");

    // Language hint
    let lang_names: Vec<&str> = languages.iter().map(|l| l.name()).collect();
    out.push_str(&format!("# Detected language(s): {}\n", lang_names.join(", ")));
    out.push_str(&format!("# Detected layout: {}\n", layout.name()));
    out.push_str("\n");

    // Rules section - adapt thresholds and levels based on layout
    out.push_str("rules:\n");

    let (fan_out_level, fan_out_threshold) = match layout {
        Layout::Flat => ("telemetry", 7usize),
        Layout::Layered => ("taboo", 5usize),
        Layout::Monorepo => ("telemetry", 8usize),
    };

    write_rule_block(
        &mut out,
        "fan_out",
        fan_out_level,
        Some(fan_out_threshold),
        "too many dependencies from one component",
    );
    write_rule_block(
        &mut out,
        "fan_in",
        "telemetry",
        Some(10),
        "too many callers into one component",
    );
    write_rule_block(
        &mut out,
        "cycles",
        if layout == &Layout::Layered { "taboo" } else { "telemetry" },
        None,
        "circular dependencies between modules",
    );
    write_rule_block(
        &mut out,
        "isp",
        "personal",
        Some(5),
        "interface/trait has too many methods",
    );
    write_rule_block(
        &mut out,
        "dip",
        "personal",
        None,
        "depends on concrete implementations instead of abstractions",
    );

    if !layers.is_empty() {
        out.push_str("\n# Architecture layers\n");
        out.push_str("layers:\n");
        for layer in layers {
            out.push_str(&format!("  - name: {}\n", layer.name));
            out.push_str("    paths:\n");
            for path in &layer.paths {
                out.push_str(&format!("      - \"{}\"\n", path));
            }
        }

        out.push_str("\n# Allowed dependency directions (source: [targets...])\n");
        out.push_str("# Any direction not listed here is a violation.\n");
        out.push_str("allowed_dependencies:\n");

        // Sort for deterministic output
        let mut sorted_keys: Vec<&String> = allowed.keys().collect();
        sorted_keys.sort();
        for key in sorted_keys {
            let targets = &allowed[key];
            if targets.is_empty() {
                out.push_str(&format!("  {}: []\n", key));
            } else {
                out.push_str(&format!("  {}: [{}]\n", key, targets.iter().map(|s| s.as_str()).collect::<Vec<_>>().join(", ")));
            }
        }
    }

    out
}

// ---------------------------------------------------------------------------
// Summary generation
// ---------------------------------------------------------------------------

fn generate_summary(
    dir: &Path,
    languages: &[Language],
    layout: &Layout,
    layers: &[Layer],
) -> String {
    let mut out = String::new();

    out.push_str("archlint init - project onboarding\n");
    out.push_str(&"=".repeat(40));
    out.push('\n');
    out.push('\n');

    out.push_str("Detected:\n");
    if languages.is_empty() {
        out.push_str("  Language: none (no go.mod / Cargo.toml / package.json found)\n");
    } else {
        let manifest_hints: Vec<String> = languages
            .iter()
            .map(|l| format!("{} ({})", l.name(), l.manifest()))
            .collect();
        out.push_str(&format!("  Language(s): {}\n", manifest_hints.join(", ")));
    }
    out.push_str(&format!("  Layout:      {}\n", layout.name()));

    if !layers.is_empty() {
        out.push_str("  Layers:\n");
        for layer in layers {
            out.push_str(&format!("    - {} -> {}\n", layer.name, layer.paths.join(", ")));
        }
    } else {
        out.push_str("  Layers:      none detected (flat layout or unknown structure)\n");
    }

    out.push('\n');
    out.push_str("Generated: .archlint.yaml\n");
    out.push('\n');

    out.push_str("Next steps:\n");
    out.push_str("  1. Review the generated .archlint.yaml\n");
    out.push_str("  2. Run `archlint scan` to check your project\n");
    out.push_str("  3. Adjust `level:` for rules (taboo = CI blocker, telemetry = track only)\n");

    match layout {
        Layout::Flat => {
            out.push_str("  4. When you introduce layers, re-run `archlint init` to update the config\n");
        }
        Layout::Layered => {
            out.push_str("  4. Review `allowed_dependencies` - adjust to match your architecture\n");
            out.push_str("  5. Add `archlint scan` to your CI pipeline\n");
        }
        Layout::Monorepo => {
            out.push_str("  4. Consider running `archlint init` in each sub-project separately\n");
        }
    }

    out.push('\n');
    out.push_str("Tip: run `archlint diagram` to visualise the architecture graph.\n");

    out
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

/// Run the onboarding scan on the given directory and return the result.
pub fn onboard(dir: &Path) -> OnboardResult {
    let languages = detect_languages(dir);
    let layout = detect_layout(dir, &languages);

    // Detect layers for the first recognised language (or all if multilang)
    let mut layers: Vec<Layer> = Vec::new();
    let mut allowed: HashMap<String, Vec<String>> = HashMap::new();

    for lang in &languages {
        let (l, a) = match lang {
            Language::Go => detect_go_layers(dir),
            Language::Rust => detect_rust_layers(dir),
            Language::TypeScript => detect_ts_layers(dir),
            Language::Python => detect_python_layers(dir),
        };
        if layers.is_empty() && !l.is_empty() {
            layers = l;
            allowed = a;
        }
    }

    let config_yaml = generate_yaml(&languages, &layout, &layers, &allowed);
    let summary = generate_summary(dir, &languages, &layout, &layers);

    OnboardResult {
        languages,
        layout,
        layers,
        allowed_dependencies: allowed,
        config_yaml,
        summary,
    }
}

/// Write `.archlint.yaml` to `dir` and return the path.
pub fn write_config(dir: &Path, yaml: &str) -> Result<PathBuf, String> {
    let config_path = dir.join(".archlint.yaml");
    std::fs::write(&config_path, yaml)
        .map_err(|e| format!("cannot write {}: {}", config_path.display(), e))?;
    Ok(config_path)
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;
    use tempfile::TempDir;

    fn mk(dir: &TempDir, path: &str) {
        let full = dir.path().join(path);
        if path.ends_with('/') || !path.contains('.') {
            fs::create_dir_all(&full).unwrap();
        } else {
            if let Some(parent) = full.parent() {
                fs::create_dir_all(parent).unwrap();
            }
            fs::write(&full, "").unwrap();
        }
    }

    // --- Language detection ---

    #[test]
    fn test_detect_go() {
        let dir = TempDir::new().unwrap();
        mk(&dir, "go.mod");
        let langs = detect_languages(dir.path());
        assert_eq!(langs, vec![Language::Go]);
    }

    #[test]
    fn test_detect_rust() {
        let dir = TempDir::new().unwrap();
        mk(&dir, "Cargo.toml");
        let langs = detect_languages(dir.path());
        assert_eq!(langs, vec![Language::Rust]);
    }

    #[test]
    fn test_detect_typescript() {
        let dir = TempDir::new().unwrap();
        mk(&dir, "package.json");
        let langs = detect_languages(dir.path());
        assert_eq!(langs, vec![Language::TypeScript]);
    }

    #[test]
    fn test_detect_python_pyproject() {
        let dir = TempDir::new().unwrap();
        mk(&dir, "pyproject.toml");
        let langs = detect_languages(dir.path());
        assert_eq!(langs, vec![Language::Python]);
    }

    #[test]
    fn test_detect_python_setup() {
        let dir = TempDir::new().unwrap();
        mk(&dir, "setup.py");
        let langs = detect_languages(dir.path());
        assert_eq!(langs, vec![Language::Python]);
    }

    #[test]
    fn test_detect_no_language() {
        let dir = TempDir::new().unwrap();
        let langs = detect_languages(dir.path());
        assert!(langs.is_empty());
    }

    #[test]
    fn test_detect_multilang() {
        let dir = TempDir::new().unwrap();
        mk(&dir, "go.mod");
        mk(&dir, "package.json");
        let langs = detect_languages(dir.path());
        assert!(langs.contains(&Language::Go));
        assert!(langs.contains(&Language::TypeScript));
    }

    // --- Layout detection ---

    #[test]
    fn test_layout_flat_go() {
        let dir = TempDir::new().unwrap();
        mk(&dir, "go.mod");
        // No cmd/internal dirs
        let langs = detect_languages(dir.path());
        let layout = detect_layout(dir.path(), &langs);
        assert_eq!(layout, Layout::Flat);
    }

    #[test]
    fn test_layout_layered_go_cmd_internal() {
        let dir = TempDir::new().unwrap();
        mk(&dir, "go.mod");
        mk(&dir, "cmd/main.go");
        mk(&dir, "internal/handler/users.go");
        let langs = detect_languages(dir.path());
        let layout = detect_layout(dir.path(), &langs);
        assert_eq!(layout, Layout::Layered);
    }

    #[test]
    fn test_layout_layered_rust_domain() {
        let dir = TempDir::new().unwrap();
        mk(&dir, "Cargo.toml");
        mk(&dir, "src/domain/user.rs");
        mk(&dir, "src/app/service.rs");
        let langs = detect_languages(dir.path());
        let layout = detect_layout(dir.path(), &langs);
        assert_eq!(layout, Layout::Layered);
    }

    #[test]
    fn test_layout_monorepo() {
        let dir = TempDir::new().unwrap();
        // Direct subdirectories each have their own manifest - that's a monorepo
        mk(&dir, "auth/go.mod");
        mk(&dir, "users/go.mod");
        let langs = detect_languages(dir.path());
        let layout = detect_layout(dir.path(), &langs);
        assert_eq!(layout, Layout::Monorepo);
    }

    // --- Go layer detection ---

    #[test]
    fn test_go_layers_full_clean_arch() {
        let dir = TempDir::new().unwrap();
        mk(&dir, "go.mod");
        mk(&dir, "cmd/main.go");
        mk(&dir, "internal/handler/api.go");
        mk(&dir, "internal/service/user.go");
        mk(&dir, "internal/repo/pg.go");
        mk(&dir, "pkg/logger/log.go");

        let (layers, allowed) = detect_go_layers(dir.path());
        let names: Vec<&str> = layers.iter().map(|l| l.name.as_str()).collect();
        assert!(names.contains(&"cmd"));
        assert!(names.contains(&"handler"));
        assert!(names.contains(&"service"));
        assert!(names.contains(&"repo"));
        assert!(names.contains(&"pkg"));

        // handler can depend on service and pkg
        let h_allowed = allowed.get("handler").unwrap();
        assert!(h_allowed.contains(&"service".to_string()));
        assert!(h_allowed.contains(&"pkg".to_string()));

        // repo can only depend on pkg
        let r_allowed = allowed.get("repo").unwrap();
        assert!(r_allowed.contains(&"pkg".to_string()));
        assert!(!r_allowed.contains(&"service".to_string()));

        // pkg has no allowed deps
        let p_allowed = allowed.get("pkg").unwrap();
        assert!(p_allowed.is_empty());
    }

    #[test]
    fn test_go_layers_simple_cmd_internal() {
        let dir = TempDir::new().unwrap();
        mk(&dir, "go.mod");
        mk(&dir, "cmd/main.go");
        mk(&dir, "internal/app.go");

        let (layers, allowed) = detect_go_layers(dir.path());
        let names: Vec<&str> = layers.iter().map(|l| l.name.as_str()).collect();
        assert!(names.contains(&"cmd"));
        assert!(names.contains(&"internal"));

        let cmd_allowed = allowed.get("cmd").unwrap();
        assert!(cmd_allowed.contains(&"internal".to_string()));

        let int_allowed = allowed.get("internal").unwrap();
        assert!(int_allowed.is_empty());
    }

    #[test]
    fn test_go_layers_none_flat() {
        let dir = TempDir::new().unwrap();
        mk(&dir, "go.mod");
        mk(&dir, "main.go");

        let (layers, _) = detect_go_layers(dir.path());
        assert!(layers.is_empty());
    }

    // --- Rust layer detection ---

    #[test]
    fn test_rust_layers_domain_app_infra() {
        let dir = TempDir::new().unwrap();
        mk(&dir, "Cargo.toml");
        mk(&dir, "src/domain/user.rs");
        mk(&dir, "src/app/user_service.rs");
        mk(&dir, "src/infra/pg_repo.rs");

        let (layers, allowed) = detect_rust_layers(dir.path());
        let names: Vec<&str> = layers.iter().map(|l| l.name.as_str()).collect();
        assert!(names.contains(&"domain"));
        assert!(names.contains(&"app"));
        assert!(names.contains(&"infra"));

        // domain has no deps
        assert!(allowed.get("domain").unwrap().is_empty());

        // app depends on domain
        let app_allowed = allowed.get("app").unwrap();
        assert!(app_allowed.contains(&"domain".to_string()));

        // infra depends on domain and app
        let infra_allowed = allowed.get("infra").unwrap();
        assert!(infra_allowed.contains(&"domain".to_string()));
        assert!(infra_allowed.contains(&"app".to_string()));
    }

    #[test]
    fn test_rust_layers_flat_src() {
        let dir = TempDir::new().unwrap();
        mk(&dir, "Cargo.toml");
        mk(&dir, "src/main.rs");
        mk(&dir, "src/lib.rs");

        let (layers, _) = detect_rust_layers(dir.path());
        assert!(layers.is_empty());
    }

    // --- TypeScript layer detection ---

    #[test]
    fn test_ts_layers_mvc() {
        let dir = TempDir::new().unwrap();
        mk(&dir, "package.json");
        mk(&dir, "src/controllers/user.ts");
        mk(&dir, "src/services/user.ts");
        mk(&dir, "src/models/user.ts");

        let (layers, allowed) = detect_ts_layers(dir.path());
        let names: Vec<&str> = layers.iter().map(|l| l.name.as_str()).collect();
        assert!(names.contains(&"controller"));
        assert!(names.contains(&"service"));
        assert!(names.contains(&"model"));

        let ctrl_allowed = allowed.get("controller").unwrap();
        assert!(ctrl_allowed.contains(&"service".to_string()));
        assert!(ctrl_allowed.contains(&"model".to_string()));

        let svc_allowed = allowed.get("service").unwrap();
        assert!(svc_allowed.contains(&"model".to_string()));
    }

    // --- YAML generation ---

    #[test]
    fn test_yaml_contains_rules_section() {
        let dir = TempDir::new().unwrap();
        mk(&dir, "go.mod");
        mk(&dir, "cmd/main.go");
        mk(&dir, "internal/handler/api.go");
        mk(&dir, "internal/service/svc.go");
        mk(&dir, "internal/repo/db.go");

        let result = onboard(dir.path());
        assert!(result.config_yaml.contains("rules:"));
        assert!(result.config_yaml.contains("fan_out:"));
        assert!(result.config_yaml.contains("fan_in:"));
        assert!(result.config_yaml.contains("cycles:"));
        assert!(result.config_yaml.contains("isp:"));
        assert!(result.config_yaml.contains("dip:"));
    }

    #[test]
    fn test_yaml_layered_go_has_taboo_cycles() {
        let dir = TempDir::new().unwrap();
        mk(&dir, "go.mod");
        mk(&dir, "cmd/main.go");
        mk(&dir, "internal/handler/api.go");
        mk(&dir, "internal/service/svc.go");

        let result = onboard(dir.path());
        // Layered layout => cycles should be taboo
        assert!(result.config_yaml.contains("taboo"));
        assert!(result.config_yaml.contains("layers:"));
        assert!(result.config_yaml.contains("allowed_dependencies:"));
    }

    #[test]
    fn test_yaml_flat_has_no_layers() {
        let dir = TempDir::new().unwrap();
        mk(&dir, "go.mod");
        mk(&dir, "main.go");

        let result = onboard(dir.path());
        assert!(!result.config_yaml.contains("layers:"));
        assert!(!result.config_yaml.contains("allowed_dependencies:"));
    }

    #[test]
    fn test_yaml_is_valid_serde_yaml() {
        let dir = TempDir::new().unwrap();
        mk(&dir, "Cargo.toml");
        mk(&dir, "src/domain/user.rs");
        mk(&dir, "src/app/service.rs");
        mk(&dir, "src/infra/repo.rs");

        let result = onboard(dir.path());
        // Should parse without error
        let parsed: Result<serde_yaml::Value, _> = serde_yaml::from_str(&result.config_yaml);
        assert!(parsed.is_ok(), "generated YAML is invalid: {:?}", parsed.err());
    }

    // --- write_config ---

    #[test]
    fn test_write_config_creates_file() {
        let dir = TempDir::new().unwrap();
        let yaml = "rules:\n  fan_out:\n    level: telemetry\n    threshold: 5\n";
        let path = write_config(dir.path(), yaml).unwrap();
        assert!(path.exists());
        let content = fs::read_to_string(&path).unwrap();
        assert_eq!(content, yaml);
    }

    #[test]
    fn test_write_config_refuses_on_unwritable() {
        // Use a non-existent nested path to trigger an error
        let result = write_config(
            Path::new("/nonexistent/deeply/nested/path"),
            "rules: {}\n",
        );
        assert!(result.is_err());
    }

    // --- Full onboard flow ---

    #[test]
    fn test_full_onboard_rust_flat() {
        let dir = TempDir::new().unwrap();
        mk(&dir, "Cargo.toml");
        mk(&dir, "src/main.rs");

        let result = onboard(dir.path());
        assert_eq!(result.languages, vec![Language::Rust]);
        assert_eq!(result.layout, Layout::Flat);
        assert!(result.layers.is_empty());
        assert!(result.summary.contains("archlint init"));
        assert!(result.summary.contains("Rust"));
    }

    #[test]
    fn test_full_onboard_go_layered() {
        let dir = TempDir::new().unwrap();
        mk(&dir, "go.mod");
        mk(&dir, "cmd/main.go");
        mk(&dir, "internal/handler/h.go");
        mk(&dir, "internal/service/s.go");
        mk(&dir, "internal/repo/r.go");

        let result = onboard(dir.path());
        assert_eq!(result.languages, vec![Language::Go]);
        assert_eq!(result.layout, Layout::Layered);
        assert!(!result.layers.is_empty());
        assert!(result.summary.contains("Next steps"));
        assert!(result.summary.contains("allowed_dependencies"));
    }

    #[test]
    fn test_summary_contains_next_steps() {
        let dir = TempDir::new().unwrap();
        mk(&dir, "go.mod");

        let result = onboard(dir.path());
        assert!(result.summary.contains("Next steps"));
        assert!(result.summary.contains("archlint scan"));
    }
}
