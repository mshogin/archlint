use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::path::Path;

/// Configuration for a single rule.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RuleConfig {
    /// Whether the rule is active (default: true).
    #[serde(default = "default_true")]
    pub enabled: bool,

    /// Whether a violation causes a non-zero exit code (default: false).
    #[serde(default)]
    pub error_on_violation: bool,

    /// Numeric threshold for this rule (e.g. max fan-out).
    #[serde(skip_serializing_if = "Option::is_none")]
    pub threshold: Option<usize>,

    /// Component IDs (or glob patterns) to exclude from this rule.
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub exclude: Vec<String>,
}

fn default_true() -> bool {
    true
}

impl Default for RuleConfig {
    fn default() -> Self {
        Self {
            enabled: true,
            error_on_violation: false,
            threshold: None,
            exclude: Vec::new(),
        }
    }
}

/// A single logical layer definition.
/// `paths` lists path prefixes that belong to this layer (e.g. "internal/handler").
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct LayerDef {
    /// Human-readable name used as key in `allowed_dependencies`.
    pub name: String,
    /// Path prefixes (relative to project root) whose files belong to this layer.
    #[serde(default)]
    pub paths: Vec<String>,
}

/// Top-level .archlint.yaml configuration.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct Config {
    /// Rules section keyed by rule name.
    #[serde(default)]
    pub rules: Rules,

    /// Optional layer definitions.
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub layers: Vec<LayerDef>,

    /// Allowed dependency directions between layers.
    /// Key: source layer name. Value: list of target layer names that are allowed.
    /// Any dependency not listed here is a violation.
    #[serde(default, skip_serializing_if = "HashMap::is_empty")]
    pub allowed_dependencies: HashMap<String, Vec<String>>,
}

/// All supported rules.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct Rules {
    #[serde(default = "default_fan_out")]
    pub fan_out: RuleConfig,
    #[serde(default = "default_fan_in")]
    pub fan_in: RuleConfig,
    #[serde(default = "default_cycles")]
    pub cycles: RuleConfig,
    /// ISP: detect traits with too many methods (default threshold: 5).
    #[serde(default = "default_isp")]
    pub isp: RuleConfig,
    /// DIP: detect modules with structs but no trait definitions.
    #[serde(default = "default_dip")]
    pub dip: RuleConfig,
}

fn default_fan_out() -> RuleConfig {
    RuleConfig {
        enabled: true,
        error_on_violation: false,
        threshold: Some(5),
        exclude: Vec::new(),
    }
}

fn default_fan_in() -> RuleConfig {
    RuleConfig {
        enabled: true,
        error_on_violation: false,
        threshold: Some(10),
        exclude: Vec::new(),
    }
}

fn default_cycles() -> RuleConfig {
    RuleConfig {
        enabled: true,
        error_on_violation: false,
        threshold: None,
        exclude: Vec::new(),
    }
}

fn default_isp() -> RuleConfig {
    RuleConfig {
        enabled: true,
        error_on_violation: false,
        threshold: Some(5),
        exclude: Vec::new(),
    }
}

fn default_dip() -> RuleConfig {
    RuleConfig {
        enabled: true,
        error_on_violation: false,
        threshold: None,
        exclude: Vec::new(),
    }
}

impl Default for Rules {
    fn default() -> Self {
        Self {
            fan_out: RuleConfig {
                enabled: true,
                error_on_violation: false,
                threshold: Some(5),
                exclude: Vec::new(),
            },
            fan_in: RuleConfig {
                enabled: true,
                error_on_violation: false,
                threshold: Some(10),
                exclude: Vec::new(),
            },
            cycles: RuleConfig {
                enabled: true,
                error_on_violation: false,
                threshold: None,
                exclude: Vec::new(),
            },
            isp: RuleConfig {
                enabled: true,
                error_on_violation: false,
                threshold: Some(5),
                exclude: Vec::new(),
            },
            dip: RuleConfig {
                enabled: true,
                error_on_violation: false,
                threshold: None,
                exclude: Vec::new(),
            },
        }
    }
}

impl Config {
    /// Load config from `.archlint.yaml` in the given directory.
    /// Falls back to defaults if the file does not exist or cannot be parsed.
    pub fn load(dir: &Path) -> Self {
        let config_path = dir.join(".archlint.yaml");
        if !config_path.exists() {
            return Self::default();
        }

        let content = match std::fs::read_to_string(&config_path) {
            Ok(c) => c,
            Err(e) => {
                eprintln!(
                    "Warning: could not read {}: {}",
                    config_path.display(),
                    e
                );
                return Self::default();
            }
        };

        match serde_yaml::from_str::<Config>(&content) {
            Ok(mut cfg) => {
                // Fill missing thresholds with defaults.
                if cfg.rules.fan_out.threshold.is_none() {
                    cfg.rules.fan_out.threshold = Some(5);
                }
                if cfg.rules.fan_in.threshold.is_none() {
                    cfg.rules.fan_in.threshold = Some(10);
                }
                cfg
            }
            Err(e) => {
                eprintln!(
                    "Warning: could not parse {}: {}. Using defaults.",
                    config_path.display(),
                    e
                );
                Self::default()
            }
        }
    }

    /// Fan-out threshold (default 5).
    pub fn fan_out_threshold(&self) -> usize {
        self.rules.fan_out.threshold.unwrap_or(5)
    }

    /// Fan-in threshold (default 10).
    pub fn fan_in_threshold(&self) -> usize {
        self.rules.fan_in.threshold.unwrap_or(10)
    }

    /// ISP: maximum number of methods allowed per trait (default 5).
    pub fn isp_threshold(&self) -> usize {
        self.rules.isp.threshold.unwrap_or(5)
    }

    /// Resolve which layer name the given module path belongs to.
    /// `module_id` uses `::` as separator (e.g. "internal::handler::users").
    /// The corresponding file path segment is derived by replacing `::` with `/`.
    /// Returns `None` if no layer matches.
    pub fn layer_for_module(&self, module_id: &str) -> Option<&str> {
        // Convert module id to a path-like string for prefix matching.
        let as_path = module_id.replace("::", "/");
        for layer in &self.layers {
            for prefix in &layer.paths {
                // Normalize prefix slashes
                let norm = prefix.trim_end_matches('/');
                if as_path == norm || as_path.starts_with(&format!("{}/", norm)) {
                    return Some(&layer.name);
                }
            }
        }
        None
    }

    /// Returns true when `layers` and `allowed_dependencies` are both configured.
    pub fn has_layer_rules(&self) -> bool {
        !self.layers.is_empty() && !self.allowed_dependencies.is_empty()
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::Write;
    use tempfile::TempDir;

    fn write_config(dir: &TempDir, content: &str) {
        let path = dir.path().join(".archlint.yaml");
        let mut f = std::fs::File::create(path).unwrap();
        f.write_all(content.as_bytes()).unwrap();
    }

    #[test]
    fn test_defaults_when_no_file() {
        let dir = TempDir::new().unwrap();
        let cfg = Config::load(dir.path());
        assert_eq!(cfg.fan_out_threshold(), 5);
        assert_eq!(cfg.fan_in_threshold(), 10);
        assert!(cfg.rules.fan_out.enabled);
        assert!(cfg.rules.fan_in.enabled);
        assert!(cfg.rules.cycles.enabled);
    }

    #[test]
    fn test_custom_thresholds() {
        let dir = TempDir::new().unwrap();
        write_config(
            &dir,
            r#"
rules:
  fan_out:
    threshold: 3
  fan_in:
    threshold: 7
"#,
        );
        let cfg = Config::load(dir.path());
        assert_eq!(cfg.fan_out_threshold(), 3);
        assert_eq!(cfg.fan_in_threshold(), 7);
    }

    #[test]
    fn test_rule_disabled() {
        let dir = TempDir::new().unwrap();
        write_config(
            &dir,
            r#"
rules:
  fan_out:
    enabled: false
    threshold: 3
  fan_in:
    enabled: true
    threshold: 8
"#,
        );
        let cfg = Config::load(dir.path());
        assert!(!cfg.rules.fan_out.enabled);
        assert!(cfg.rules.fan_in.enabled);
        assert_eq!(cfg.fan_in_threshold(), 8);
    }

    #[test]
    fn test_exclude_list() {
        let dir = TempDir::new().unwrap();
        write_config(
            &dir,
            r#"
rules:
  fan_out:
    threshold: 5
    exclude:
      - main
      - lib::utils
"#,
        );
        let cfg = Config::load(dir.path());
        assert_eq!(cfg.rules.fan_out.exclude, vec!["main", "lib::utils"]);
    }

    #[test]
    fn test_error_on_violation_flag() {
        let dir = TempDir::new().unwrap();
        write_config(
            &dir,
            r#"
rules:
  fan_out:
    threshold: 4
    error_on_violation: true
"#,
        );
        let cfg = Config::load(dir.path());
        assert!(cfg.rules.fan_out.error_on_violation);
        assert!(!cfg.rules.fan_in.error_on_violation);
    }

    #[test]
    fn test_fallback_on_invalid_yaml() {
        let dir = TempDir::new().unwrap();
        write_config(&dir, "this: [is: not: valid: yaml");
        let cfg = Config::load(dir.path());
        // Should return defaults without panicking.
        assert_eq!(cfg.fan_out_threshold(), 5);
        assert_eq!(cfg.fan_in_threshold(), 10);
    }

    #[test]
    fn test_partial_config_uses_defaults_for_missing_thresholds() {
        let dir = TempDir::new().unwrap();
        write_config(
            &dir,
            r#"
rules:
  fan_out:
    enabled: true
"#,
        );
        let cfg = Config::load(dir.path());
        // threshold not specified -> default 5
        assert_eq!(cfg.fan_out_threshold(), 5);
        // fan_in not in file -> full default
        assert_eq!(cfg.fan_in_threshold(), 10);
    }

    #[test]
    fn test_layer_config_parsed() {
        let dir = TempDir::new().unwrap();
        write_config(
            &dir,
            r#"
layers:
  - name: handler
    paths: ["internal/handler", "src/handler"]
  - name: service
    paths: ["internal/service", "src/service"]
  - name: repo
    paths: ["internal/repo", "src/repo"]
  - name: model
    paths: ["internal/model", "src/model"]

allowed_dependencies:
  handler: [service, model]
  service: [repo, model]
  repo: [model]
  model: []
"#,
        );
        let cfg = Config::load(dir.path());
        assert_eq!(cfg.layers.len(), 4);
        assert!(cfg.has_layer_rules());

        // Layer name lookup
        assert_eq!(cfg.layer_for_module("internal::handler::users"), Some("handler"));
        assert_eq!(cfg.layer_for_module("src::service::orders"), Some("service"));
        assert_eq!(cfg.layer_for_module("internal::repo::pg"), Some("repo"));
        assert_eq!(cfg.layer_for_module("internal::model::user"), Some("model"));
        assert_eq!(cfg.layer_for_module("pkg::utils"), None);

        // Allowed deps
        let handler_allowed = cfg.allowed_dependencies.get("handler").unwrap();
        assert!(handler_allowed.contains(&"service".to_string()));
        assert!(handler_allowed.contains(&"model".to_string()));
        assert!(!handler_allowed.contains(&"repo".to_string()));
    }

    #[test]
    fn test_no_layers_has_layer_rules_false() {
        let dir = TempDir::new().unwrap();
        let cfg = Config::load(dir.path());
        assert!(!cfg.has_layer_rules());
        assert_eq!(cfg.layer_for_module("internal::handler::foo"), None);
    }

    #[test]
    fn test_layer_path_exact_match() {
        let dir = TempDir::new().unwrap();
        write_config(
            &dir,
            r#"
layers:
  - name: handler
    paths: ["internal/handler"]
allowed_dependencies:
  handler: []
"#,
        );
        let cfg = Config::load(dir.path());
        // Exact match (module id == layer path without slashes)
        assert_eq!(cfg.layer_for_module("internal::handler"), Some("handler"));
        // Prefix match
        assert_eq!(cfg.layer_for_module("internal::handler::users"), Some("handler"));
        // No match for sibling
        assert_eq!(cfg.layer_for_module("internal::repo"), None);
    }
}
