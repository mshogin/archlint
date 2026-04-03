use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::path::Path;

/// Metric level for a rule - controls exit code and output presentation.
#[derive(Debug, Clone, PartialEq, Eq, Serialize, Deserialize, Default)]
#[serde(rename_all = "lowercase")]
pub enum Level {
    /// Blocks CI: exit code 1, shown in RED.
    Taboo,
    /// Track only: exit code 0, shown in YELLOW.
    #[default]
    Telemetry,
    /// Informational: exit code 0, shown in default color.
    Personal,
}

impl Level {
    pub fn as_str(&self) -> &'static str {
        match self {
            Level::Taboo => "taboo",
            Level::Telemetry => "telemetry",
            Level::Personal => "personal",
        }
    }
}

impl std::fmt::Display for Level {
    fn fmt(&self, f: &mut std::fmt::Formatter<'_>) -> std::fmt::Result {
        write!(f, "{}", self.as_str())
    }
}

/// Configuration for a single rule.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct RuleConfig {
    /// Whether the rule is active (default: true).
    #[serde(default = "default_true")]
    pub enabled: bool,

    /// Whether a violation causes a non-zero exit code (default: false).
    #[serde(default)]
    pub error_on_violation: bool,

    /// Metric level: taboo (CI blocker), telemetry (track only), personal (informational).
    #[serde(default)]
    pub level: Level,

    /// Numeric threshold for this rule (e.g. max fan-out).
    /// Accepts both integers and floating-point values in YAML.
    #[serde(skip_serializing_if = "Option::is_none")]
    pub threshold: Option<f64>,

    /// Component IDs (or glob patterns) to exclude from this rule.
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub exclude: Vec<String>,

    /// Known violations: component paths that are allowed to violate this rule temporarily.
    /// These are shown in output as TODO items (not counted as real violations).
    /// Use for gradual migration: list legacy components that will be fixed later.
    /// With --strict flag, todo items are treated as real violations.
    #[serde(default, skip_serializing_if = "Vec::is_empty")]
    pub todo: Vec<String>,
}

fn default_true() -> bool {
    true
}

impl Default for RuleConfig {
    fn default() -> Self {
        Self {
            enabled: true,
            error_on_violation: false,
            level: Level::Telemetry,
            threshold: None,
            exclude: Vec::new(),
            todo: Vec::new(),
        }
    }
}

impl RuleConfig {
    /// Returns true if the given component ID is in the todo list for gradual migration.
    /// The component is considered a "known violation" that should not fail the gate.
    pub fn is_todo(&self, component_id: &str) -> bool {
        self.todo.iter().any(|t| {
            // Normalize separators for comparison: both :: and / treated as equivalent.
            let norm_component = component_id.replace("::", "/");
            let norm_todo = t.replace("::", "/");
            // Exact match or prefix match (module path prefix).
            norm_component == norm_todo
                || norm_component.starts_with(&format!("{}/", norm_todo))
        })
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
    /// God-class: detect Go structs with too many methods/fields (default threshold: 20 methods, 15 fields).
    #[serde(default = "default_god_class")]
    pub god_class: RuleConfig,
    /// Feature-envy: detect Go methods that use more of another type than their own (default threshold: 3 foreign calls).
    #[serde(default = "default_feature_envy")]
    pub feature_envy: RuleConfig,
    /// SRP: detect structs/modules with too many methods (Single Responsibility Principle).
    /// Works for both Go and Rust. threshold: max methods per struct/module (default: 10).
    #[serde(default = "default_srp")]
    pub srp: RuleConfig,
    /// Shotgun Surgery: detect modules with high afferent coupling (blast radius).
    /// A change in this module forces changes in many others.
    /// threshold: max number of modules that depend on this one (default: 10).
    #[serde(default = "default_shotgun")]
    pub shotgun_surgery: RuleConfig,
    /// Coupling instability: detect modules with dangerously high instability Ce/(Ca+Ce).
    /// threshold is integer 0-100 representing percentage, default 80 means instability > 0.80.
    /// Only fires when the module has at least 2 dependents (Ca) to avoid noise on leaf modules.
    #[serde(default = "default_coupling")]
    pub coupling: RuleConfig,
}

fn default_fan_out() -> RuleConfig {
    RuleConfig {
        enabled: true,
        error_on_violation: false,
        level: Level::Telemetry,
        threshold: Some(5.0),
        exclude: Vec::new(),
        todo: Vec::new(),
    }
}

fn default_fan_in() -> RuleConfig {
    RuleConfig {
        enabled: true,
        error_on_violation: false,
        level: Level::Telemetry,
        threshold: Some(10.0),
        exclude: Vec::new(),
        todo: Vec::new(),
    }
}

fn default_cycles() -> RuleConfig {
    RuleConfig {
        enabled: true,
        error_on_violation: false,
        level: Level::Telemetry,
        threshold: None,
        exclude: Vec::new(),
        todo: Vec::new(),
    }
}

fn default_isp() -> RuleConfig {
    RuleConfig {
        enabled: true,
        error_on_violation: false,
        level: Level::Telemetry,
        threshold: Some(5.0),
        exclude: Vec::new(),
        todo: Vec::new(),
    }
}

fn default_dip() -> RuleConfig {
    RuleConfig {
        enabled: true,
        error_on_violation: false,
        level: Level::Telemetry,
        threshold: None,
        exclude: Vec::new(),
        todo: Vec::new(),
    }
}

fn default_god_class() -> RuleConfig {
    RuleConfig {
        enabled: true,
        error_on_violation: false,
        level: Level::Telemetry,
        // threshold here is the method count limit; field limit is threshold * 3/4 (see analyzer)
        threshold: Some(20.0),
        exclude: Vec::new(),
        todo: Vec::new(),
    }
}

fn default_feature_envy() -> RuleConfig {
    RuleConfig {
        enabled: true,
        error_on_violation: false,
        level: Level::Telemetry,
        // minimum number of foreign calls to trigger feature-envy
        threshold: Some(3.0),
        exclude: Vec::new(),
        todo: Vec::new(),
    }
}

fn default_srp() -> RuleConfig {
    RuleConfig {
        enabled: true,
        error_on_violation: false,
        level: Level::Telemetry,
        threshold: Some(10.0),
        exclude: Vec::new(),
        todo: Vec::new(),
    }
}

fn default_shotgun() -> RuleConfig {
    RuleConfig {
        enabled: true,
        error_on_violation: false,
        level: Level::Telemetry,
        threshold: Some(10.0),
        exclude: Vec::new(),
        todo: Vec::new(),
    }
}

fn default_coupling() -> RuleConfig {
    RuleConfig {
        enabled: true,
        error_on_violation: false,
        level: Level::Personal,
        // 80 = instability > 0.80 threshold (as integer percentage)
        threshold: Some(80.0),
        exclude: Vec::new(),
        todo: Vec::new(),
    }
}

impl Default for Rules {
    fn default() -> Self {
        Self {
            fan_out: RuleConfig {
                enabled: true,
                error_on_violation: false,
                level: Level::Telemetry,
                threshold: Some(5.0),
                exclude: Vec::new(),
                todo: Vec::new(),
            },
            fan_in: RuleConfig {
                enabled: true,
                error_on_violation: false,
                level: Level::Telemetry,
                threshold: Some(10.0),
                exclude: Vec::new(),
                todo: Vec::new(),
            },
            cycles: RuleConfig {
                enabled: true,
                error_on_violation: false,
                level: Level::Telemetry,
                threshold: None,
                exclude: Vec::new(),
                todo: Vec::new(),
            },
            isp: RuleConfig {
                enabled: true,
                error_on_violation: false,
                level: Level::Telemetry,
                threshold: Some(5.0),
                exclude: Vec::new(),
                todo: Vec::new(),
            },
            dip: RuleConfig {
                enabled: true,
                error_on_violation: false,
                level: Level::Telemetry,
                threshold: None,
                exclude: Vec::new(),
                todo: Vec::new(),
            },
            god_class: RuleConfig {
                enabled: true,
                error_on_violation: false,
                level: Level::Telemetry,
                threshold: Some(20.0),
                exclude: Vec::new(),
                todo: Vec::new(),
            },
            feature_envy: RuleConfig {
                enabled: true,
                error_on_violation: false,
                level: Level::Telemetry,
                threshold: Some(3.0),
                exclude: Vec::new(),
                todo: Vec::new(),
            },
            srp: RuleConfig {
                enabled: true,
                error_on_violation: false,
                level: Level::Telemetry,
                threshold: Some(10.0),
                exclude: Vec::new(),
                todo: Vec::new(),
            },
            shotgun_surgery: RuleConfig {
                enabled: true,
                error_on_violation: false,
                level: Level::Telemetry,
                threshold: Some(10.0),
                exclude: Vec::new(),
                todo: Vec::new(),
            },
            coupling: RuleConfig {
                enabled: true,
                error_on_violation: false,
                level: Level::Personal,
                threshold: Some(80.0),
                exclude: Vec::new(),
                todo: Vec::new(),
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
                    cfg.rules.fan_out.threshold = Some(5.0);
                }
                if cfg.rules.fan_in.threshold.is_none() {
                    cfg.rules.fan_in.threshold = Some(10.0);
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
        self.rules.fan_out.threshold.unwrap_or(5.0) as usize
    }

    /// Fan-in threshold (default 10).
    pub fn fan_in_threshold(&self) -> usize {
        self.rules.fan_in.threshold.unwrap_or(10.0) as usize
    }

    /// ISP: maximum number of methods allowed per trait (default 5).
    pub fn isp_threshold(&self) -> usize {
        self.rules.isp.threshold.unwrap_or(5.0) as usize
    }

    /// God-class: maximum number of methods allowed per Go struct (default 20).
    pub fn god_class_method_threshold(&self) -> usize {
        self.rules.god_class.threshold.unwrap_or(20.0) as usize
    }

    /// God-class: maximum number of fields allowed per Go struct (default 15).
    pub fn god_class_field_threshold(&self) -> usize {
        // Field threshold is 3/4 of method threshold, minimum 15.
        let m = self.god_class_method_threshold();
        (m * 3 / 4).max(15)
    }

    /// Feature-envy: minimum foreign call count to flag a method (default 3).
    /// Accepts float thresholds from config (e.g. 0.5 rounds down to 0).
    pub fn feature_envy_threshold(&self) -> usize {
        self.rules.feature_envy.threshold.unwrap_or(3.0) as usize
    }

    /// SRP: max methods per struct/module (default 10).
    pub fn srp_method_threshold(&self) -> usize {
        self.rules.srp.threshold.unwrap_or(10.0) as usize
    }

    /// Shotgun Surgery: max number of dependents before triggering (default 10).
    pub fn shotgun_threshold(&self) -> usize {
        self.rules.shotgun_surgery.threshold.unwrap_or(10.0) as usize
    }

    /// Coupling instability threshold as fraction (threshold/100).
    /// Default: 0.80 (modules more unstable than 80% are flagged).
    pub fn coupling_instability_threshold(&self) -> f64 {
        self.rules.coupling.threshold.unwrap_or(80.0) / 100.0
    }

    /// Resolve which layer name the given module path belongs to.
    ///
    /// `module_id` uses `::` as separator (e.g. "src::bus", "internal::handler::users").
    /// Config `paths` may use either `/` (e.g. "src/bus") or `::` (e.g. "src::bus").
    ///
    /// Both separators are normalised to `/` before matching so that flat `src/`
    /// structures (where each file is its own module — "src::bus", "src::agent")
    /// are matched correctly against config entries like "src/bus" or "src/agent".
    ///
    /// Returns `None` if no layer matches.
    pub fn layer_for_module(&self, module_id: &str) -> Option<&str> {
        // Normalise module id: replace `::` separator with `/`.
        let as_path = module_id.replace("::", "/");
        for layer in &self.layers {
            for prefix in &layer.paths {
                // Normalise config prefix: replace `::` with `/` and strip trailing `/`.
                let norm = prefix.replace("::", "/");
                let norm = norm.trim_end_matches('/');
                // Exact match (flat module — "src/bus" == "src/bus") or
                // prefix match (nested module — "src/handler/users".starts_with("src/handler/")).
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

    #[test]
    fn test_default_level_is_telemetry() {
        let dir = TempDir::new().unwrap();
        let cfg = Config::load(dir.path());
        assert_eq!(cfg.rules.fan_out.level, Level::Telemetry);
        assert_eq!(cfg.rules.fan_in.level, Level::Telemetry);
        assert_eq!(cfg.rules.cycles.level, Level::Telemetry);
        assert_eq!(cfg.rules.isp.level, Level::Telemetry);
        assert_eq!(cfg.rules.dip.level, Level::Telemetry);
    }

    #[test]
    fn test_level_parsed_from_config() {
        let dir = TempDir::new().unwrap();
        write_config(
            &dir,
            r#"
rules:
  fan_out:
    threshold: 5
    level: taboo
  cycles:
    level: personal
  isp:
    threshold: 5
    level: telemetry
"#,
        );
        let cfg = Config::load(dir.path());
        assert_eq!(cfg.rules.fan_out.level, Level::Taboo);
        assert_eq!(cfg.rules.cycles.level, Level::Personal);
        assert_eq!(cfg.rules.isp.level, Level::Telemetry);
        // fan_in not specified -> default telemetry
        assert_eq!(cfg.rules.fan_in.level, Level::Telemetry);
    }

    #[test]
    fn test_level_as_str() {
        assert_eq!(Level::Taboo.as_str(), "taboo");
        assert_eq!(Level::Telemetry.as_str(), "telemetry");
        assert_eq!(Level::Personal.as_str(), "personal");
    }

    /// Flat src/ structure: each file is a top-level module (e.g. src/bus.rs -> src::bus).
    /// Config paths use slash notation ("src/bus"). Verify that layer_for_module matches
    /// flat modules correctly via exact match and does not accidentally match siblings.
    #[test]
    fn test_layer_flat_src_modules_slash_paths() {
        let dir = TempDir::new().unwrap();
        write_config(
            &dir,
            r#"
layers:
  - name: domain
    paths: ["src/message", "src/context"]
  - name: infra
    paths: ["src/bus", "src/agent", "src/config"]
  - name: app
    paths: ["src/worker"]

allowed_dependencies:
  domain: []
  infra: [domain]
  app: [domain, infra]
"#,
        );
        let cfg = Config::load(dir.path());
        assert_eq!(cfg.layers.len(), 3);
        assert!(cfg.has_layer_rules());

        // Flat modules resolve to the correct layer.
        assert_eq!(cfg.layer_for_module("src::message"), Some("domain"));
        assert_eq!(cfg.layer_for_module("src::context"), Some("domain"));
        assert_eq!(cfg.layer_for_module("src::bus"),     Some("infra"));
        assert_eq!(cfg.layer_for_module("src::agent"),   Some("infra"));
        assert_eq!(cfg.layer_for_module("src::config"),  Some("infra"));
        assert_eq!(cfg.layer_for_module("src::worker"),  Some("app"));

        // Module not listed in any layer returns None.
        assert_eq!(cfg.layer_for_module("src::unknown"), None);

        // Allowed-dependency map is correct.
        let domain_allowed = cfg.allowed_dependencies.get("domain").unwrap();
        assert!(domain_allowed.is_empty(), "domain must not depend on anything");

        let infra_allowed = cfg.allowed_dependencies.get("infra").unwrap();
        assert!(infra_allowed.contains(&"domain".to_string()));
    }

    /// Config paths that use `::` notation (e.g. "src::bus") should also be normalised
    /// and match module ids written with either separator.
    #[test]
    fn test_layer_flat_src_modules_colonsep_paths() {
        let dir = TempDir::new().unwrap();
        write_config(
            &dir,
            r#"
layers:
  - name: domain
    paths: ["src::message", "src::context"]
  - name: infra
    paths: ["src::bus"]

allowed_dependencies:
  domain: []
  infra: [domain]
"#,
        );
        let cfg = Config::load(dir.path());
        // Config paths written with `::` must still match module ids.
        assert_eq!(cfg.layer_for_module("src::message"), Some("domain"));
        assert_eq!(cfg.layer_for_module("src::context"), Some("domain"));
        assert_eq!(cfg.layer_for_module("src::bus"),     Some("infra"));
        assert_eq!(cfg.layer_for_module("src::other"),   None);
    }

    #[test]
    fn test_todo_list_parsed_from_config() {
        let dir = TempDir::new().unwrap();
        write_config(
            &dir,
            r#"
rules:
  fan_out:
    threshold: 5
    todo:
      - internal/handler/legacy
      - internal/service/monolith
"#,
        );
        let cfg = Config::load(dir.path());
        assert_eq!(
            cfg.rules.fan_out.todo,
            vec!["internal/handler/legacy", "internal/service/monolith"]
        );
        // Other rules should have empty todo.
        assert!(cfg.rules.fan_in.todo.is_empty());
        assert!(cfg.rules.cycles.todo.is_empty());
    }

    #[test]
    fn test_rule_config_is_todo_exact_match() {
        let rule = RuleConfig {
            todo: vec!["internal/handler/legacy".to_string()],
            ..RuleConfig::default()
        };
        assert!(rule.is_todo("internal/handler/legacy"));
        assert!(rule.is_todo("internal::handler::legacy"));
    }

    #[test]
    fn test_rule_config_is_todo_prefix_match() {
        let rule = RuleConfig {
            todo: vec!["internal/handler".to_string()],
            ..RuleConfig::default()
        };
        // Prefix match: sub-components of handler should be in todo.
        assert!(rule.is_todo("internal/handler/legacy"));
        assert!(rule.is_todo("internal::handler::legacy"));
        // Exact match.
        assert!(rule.is_todo("internal::handler"));
        // Sibling is NOT in todo.
        assert!(!rule.is_todo("internal::service"));
    }

    #[test]
    fn test_rule_config_is_todo_empty() {
        let rule = RuleConfig::default();
        assert!(!rule.is_todo("internal::handler::legacy"));
        assert!(!rule.is_todo("anything"));
    }

    #[test]
    fn test_rule_config_is_todo_colon_separator_in_todo() {
        // Config uses :: notation in todo list.
        let rule = RuleConfig {
            todo: vec!["internal::handler::legacy".to_string()],
            ..RuleConfig::default()
        };
        // Should match regardless of separator used.
        assert!(rule.is_todo("internal/handler/legacy"));
        assert!(rule.is_todo("internal::handler::legacy"));
        // Should not match unrelated module.
        assert!(!rule.is_todo("internal::service::legacy"));
    }

    #[test]
    fn test_todo_default_is_empty_for_all_rules() {
        let rules = Rules::default();
        assert!(rules.fan_out.todo.is_empty());
        assert!(rules.fan_in.todo.is_empty());
        assert!(rules.cycles.todo.is_empty());
        assert!(rules.isp.todo.is_empty());
        assert!(rules.dip.todo.is_empty());
        assert!(rules.god_class.todo.is_empty());
        assert!(rules.feature_envy.todo.is_empty());
        assert!(rules.srp.todo.is_empty());
        assert!(rules.shotgun_surgery.todo.is_empty());
        assert!(rules.coupling.todo.is_empty());
    }
}
