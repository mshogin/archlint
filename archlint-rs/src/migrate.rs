/// Config schema migration: old format -> current flat schema.
///
/// Old keys detected:
///   rules.max_fan_out       -> rules.fan_out.threshold
///   rules.coupling.params   -> rules.fan_in.threshold (ce_threshold) / rules.fan_out.threshold (ca_threshold)
///   rules.layer_violations  -> layers + allowed_dependencies
///   rules.forbidden_dependencies -> layers.allowed_dependencies (inverted/added)
use std::collections::HashMap;
use std::path::Path;

/// Result of a migration check.
#[derive(Debug, PartialEq)]
pub enum MigrateResult {
    /// Config is already up-to-date, no action taken.
    UpToDate,
    /// Old schema detected. In dry-run mode, changes were NOT written.
    DryRun(String),
    /// Old schema detected and config was rewritten. Backup created at path.
    Migrated { backup: String, summary: String },
}

/// Inspect raw YAML content and return true if it looks like old schema.
pub fn needs_migration(content: &str) -> bool {
    let doc: serde_yaml::Value = match serde_yaml::from_str(content) {
        Ok(v) => v,
        Err(_) => return false,
    };

    let rules = match doc.get("rules") {
        Some(r) => r,
        None => return false,
    };

    rules.get("max_fan_out").is_some()
        || rules.get("coupling").is_some()
        || rules.get("layer_violations").is_some()
        || rules.get("forbidden_dependencies").is_some()
}

/// Perform migration on the config file at `config_path`.
///
/// If `dry_run` is true, nothing is written to disk - only the diff description
/// is returned.
pub fn migrate(config_path: &Path, dry_run: bool) -> Result<MigrateResult, String> {
    let content = std::fs::read_to_string(config_path)
        .map_err(|e| format!("cannot read {}: {}", config_path.display(), e))?;

    if !needs_migration(&content) {
        return Ok(MigrateResult::UpToDate);
    }

    let (new_content, summary) = transform(&content)?;

    if dry_run {
        return Ok(MigrateResult::DryRun(summary));
    }

    // Create backup before overwriting.
    let backup_path = config_path.with_extension("yaml.bak");
    std::fs::copy(config_path, &backup_path)
        .map_err(|e| format!("cannot create backup {}: {}", backup_path.display(), e))?;

    std::fs::write(config_path, &new_content)
        .map_err(|e| format!("cannot write migrated config: {}", e))?;

    Ok(MigrateResult::Migrated {
        backup: backup_path.to_string_lossy().into_owned(),
        summary,
    })
}

/// Core transformation: parse old YAML, produce new YAML string + human summary.
fn transform(content: &str) -> Result<(String, String), String> {
    let doc: serde_yaml::Value = serde_yaml::from_str(content)
        .map_err(|e| format!("cannot parse yaml: {}", e))?;

    let rules = doc.get("rules").cloned().unwrap_or(serde_yaml::Value::Null);

    let mut changes: Vec<String> = Vec::new();

    // --- rules.fan_out ---
    let mut fan_out_threshold: Option<u64> = None;
    let mut fan_out_exclude: Vec<String> = Vec::new();
    let mut fan_out_error_on_violation = false;

    if let Some(old_fo) = rules.get("max_fan_out") {
        let threshold = old_fo
            .get("threshold")
            .and_then(|v| v.as_u64())
            .unwrap_or(5);
        fan_out_threshold = Some(threshold);
        fan_out_exclude = extract_exclude(old_fo);
        fan_out_error_on_violation = old_fo
            .get("error_on_violation")
            .and_then(|v| v.as_bool())
            .unwrap_or(false);
        changes.push(format!(
            "rules.max_fan_out -> rules.fan_out (threshold: {})",
            threshold
        ));
    }

    // --- rules.fan_in (from coupling.params.ce_threshold) ---
    let mut fan_in_threshold: Option<u64> = None;
    let mut fan_in_exclude: Vec<String> = Vec::new();
    let mut fan_in_error_on_violation = false;

    if let Some(old_coupling) = rules.get("coupling") {
        let ca = old_coupling
            .get("params")
            .and_then(|p| p.get("ca_threshold"))
            .and_then(|v| v.as_u64());
        let ce = old_coupling
            .get("params")
            .and_then(|p| p.get("ce_threshold"))
            .and_then(|v| v.as_u64());

        if let Some(ca_val) = ca {
            if fan_out_threshold.is_none() {
                fan_out_threshold = Some(ca_val);
                changes.push(format!(
                    "rules.coupling.params.ca_threshold -> rules.fan_out (threshold: {})",
                    ca_val
                ));
            }
        }
        if let Some(ce_val) = ce {
            fan_in_threshold = Some(ce_val);
            changes.push(format!(
                "rules.coupling.params.ce_threshold -> rules.fan_in (threshold: {})",
                ce_val
            ));
        }

        let exc = extract_exclude(old_coupling);
        if !exc.is_empty() && fan_in_exclude.is_empty() {
            fan_in_exclude = exc;
        }
        fan_in_error_on_violation = old_coupling
            .get("error_on_violation")
            .and_then(|v| v.as_bool())
            .unwrap_or(false);
    }

    // --- layers + allowed_dependencies (from layer_violations) ---
    let mut layers: Vec<serde_yaml::Value> = Vec::new();
    let mut allowed_deps: HashMap<String, Vec<String>> = HashMap::new();

    if let Some(lv) = rules.get("layer_violations") {
        if let Some(layer_map) = lv.get("params").and_then(|p| p.get("layers")) {
            // layer_map: { handler: 1, service: 2, ... } (name -> rank)
            // Build rank -> [name] mapping to derive allowed deps (lower rank can depend on higher).
            let mut rank_to_names: HashMap<u64, Vec<String>> = HashMap::new();
            if let Some(mapping) = layer_map.as_mapping() {
                for (k, v) in mapping {
                    let name = k.as_str().unwrap_or("").to_string();
                    let rank = v.as_u64().unwrap_or(0);
                    rank_to_names.entry(rank).or_default().push(name.clone());

                    // One layer entry per name.
                    layers.push(serde_yaml::Value::Mapping({
                        let mut m = serde_yaml::Mapping::new();
                        m.insert(
                            serde_yaml::Value::String("name".into()),
                            serde_yaml::Value::String(name),
                        );
                        m.insert(
                            serde_yaml::Value::String("paths".into()),
                            serde_yaml::Value::Sequence(Vec::new()),
                        );
                        m
                    }));
                }
            }

            // Build allowed_deps: a layer at rank R can depend on layers at rank > R.
            if let Some(mapping) = layer_map.as_mapping() {
                for (k, v) in mapping {
                    let name = k.as_str().unwrap_or("").to_string();
                    let rank = v.as_u64().unwrap_or(0);
                    let allowed: Vec<String> = mapping
                        .iter()
                        .filter(|(_, v2)| v2.as_u64().unwrap_or(0) > rank)
                        .map(|(k2, _)| k2.as_str().unwrap_or("").to_string())
                        .collect();
                    allowed_deps.insert(name, allowed);
                }
            }

            changes.push(format!(
                "rules.layer_violations -> layers ({} entries) + allowed_dependencies",
                layers.len()
            ));
        }
    }

    // --- forbidden_dependencies -> remove from allowed_deps ---
    if let Some(fd) = rules.get("forbidden_dependencies") {
        if let Some(fd_rules) = fd
            .get("params")
            .and_then(|p| p.get("rules"))
            .and_then(|r| r.as_sequence())
        {
            let mut count = 0usize;
            for rule in fd_rules {
                let from = rule.get("from").and_then(|v| v.as_str()).unwrap_or("");
                let to = rule.get("to").and_then(|v| v.as_str()).unwrap_or("");
                if from.is_empty() || to.is_empty() {
                    continue;
                }
                if let Some(allowed) = allowed_deps.get_mut(from) {
                    allowed.retain(|t| t != to);
                    count += 1;
                }
            }
            if count > 0 {
                changes.push(format!(
                    "rules.forbidden_dependencies -> removed {} forbidden edges from allowed_dependencies",
                    count
                ));
            }
        }
    }

    // --- Build new Config YAML ---
    let mut new_doc = serde_yaml::Mapping::new();

    // rules section
    let mut new_rules = serde_yaml::Mapping::new();

    // fan_out
    {
        let mut fo = serde_yaml::Mapping::new();
        fo.insert(yaml_str("enabled"), serde_yaml::Value::Bool(true));
        fo.insert(
            yaml_str("error_on_violation"),
            serde_yaml::Value::Bool(fan_out_error_on_violation),
        );
        fo.insert(yaml_str("level"), yaml_str("telemetry"));
        fo.insert(
            yaml_str("threshold"),
            serde_yaml::Value::Number(fan_out_threshold.unwrap_or(5).into()),
        );
        if !fan_out_exclude.is_empty() {
            fo.insert(
                yaml_str("exclude"),
                serde_yaml::Value::Sequence(
                    fan_out_exclude
                        .iter()
                        .map(|s| serde_yaml::Value::String(s.clone()))
                        .collect(),
                ),
            );
        }
        new_rules.insert(yaml_str("fan_out"), serde_yaml::Value::Mapping(fo));
    }

    // fan_in
    {
        let mut fi = serde_yaml::Mapping::new();
        fi.insert(yaml_str("enabled"), serde_yaml::Value::Bool(true));
        fi.insert(
            yaml_str("error_on_violation"),
            serde_yaml::Value::Bool(fan_in_error_on_violation),
        );
        fi.insert(yaml_str("level"), yaml_str("telemetry"));
        fi.insert(
            yaml_str("threshold"),
            serde_yaml::Value::Number(fan_in_threshold.unwrap_or(10).into()),
        );
        if !fan_in_exclude.is_empty() {
            fi.insert(
                yaml_str("exclude"),
                serde_yaml::Value::Sequence(
                    fan_in_exclude
                        .iter()
                        .map(|s| serde_yaml::Value::String(s.clone()))
                        .collect(),
                ),
            );
        }
        new_rules.insert(yaml_str("fan_in"), serde_yaml::Value::Mapping(fi));
    }

    new_doc.insert(yaml_str("rules"), serde_yaml::Value::Mapping(new_rules));

    // layers
    if !layers.is_empty() {
        new_doc.insert(
            yaml_str("layers"),
            serde_yaml::Value::Sequence(layers),
        );
    }

    // allowed_dependencies
    if !allowed_deps.is_empty() {
        let mut ad_map = serde_yaml::Mapping::new();
        let mut sorted_keys: Vec<String> = allowed_deps.keys().cloned().collect();
        sorted_keys.sort();
        for key in sorted_keys {
            let deps = &allowed_deps[&key];
            let mut sorted_deps = deps.clone();
            sorted_deps.sort();
            ad_map.insert(
                serde_yaml::Value::String(key),
                serde_yaml::Value::Sequence(
                    sorted_deps
                        .iter()
                        .map(|s| serde_yaml::Value::String(s.clone()))
                        .collect(),
                ),
            );
        }
        new_doc.insert(
            yaml_str("allowed_dependencies"),
            serde_yaml::Value::Mapping(ad_map),
        );
    }

    let new_yaml = serde_yaml::to_string(&serde_yaml::Value::Mapping(new_doc))
        .map_err(|e| format!("cannot serialize migrated config: {}", e))?;

    let summary = changes.join("; ");
    Ok((new_yaml, summary))
}

fn yaml_str(s: &str) -> serde_yaml::Value {
    serde_yaml::Value::String(s.to_string())
}

fn extract_exclude(node: &serde_yaml::Value) -> Vec<String> {
    node.get("exclude")
        .and_then(|e| e.as_sequence())
        .map(|seq| {
            seq.iter()
                .filter_map(|v| v.as_str().map(|s| s.to_string()))
                .collect()
        })
        .unwrap_or_default()
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::io::Write;
    use tempfile::TempDir;

    fn write_file(dir: &TempDir, name: &str, content: &str) -> std::path::PathBuf {
        let path = dir.path().join(name);
        let mut f = std::fs::File::create(&path).unwrap();
        f.write_all(content.as_bytes()).unwrap();
        path
    }

    // --- needs_migration ---

    #[test]
    fn test_needs_migration_max_fan_out() {
        let yaml = r#"
rules:
  max_fan_out:
    enabled: true
    threshold: 5
"#;
        assert!(needs_migration(yaml));
    }

    #[test]
    fn test_needs_migration_coupling() {
        let yaml = r#"
rules:
  coupling:
    enabled: true
    params:
      ca_threshold: 10
      ce_threshold: 10
"#;
        assert!(needs_migration(yaml));
    }

    #[test]
    fn test_needs_migration_layer_violations() {
        let yaml = r#"
rules:
  layer_violations:
    enabled: true
    params:
      layers:
        handler: 1
        service: 2
"#;
        assert!(needs_migration(yaml));
    }

    #[test]
    fn test_needs_migration_forbidden_dependencies() {
        let yaml = r#"
rules:
  forbidden_dependencies:
    enabled: true
    params:
      rules:
        - from: handler
          to: repository
"#;
        assert!(needs_migration(yaml));
    }

    #[test]
    fn test_no_migration_needed_current_schema() {
        let yaml = r#"
rules:
  fan_out:
    enabled: true
    threshold: 5
  fan_in:
    threshold: 10
"#;
        assert!(!needs_migration(yaml));
    }

    #[test]
    fn test_no_migration_needed_empty() {
        assert!(!needs_migration(""));
    }

    // --- transform ---

    #[test]
    fn test_transform_max_fan_out() {
        let yaml = r#"
rules:
  max_fan_out:
    enabled: true
    threshold: 7
    error_on_violation: true
    exclude:
      - cmd/*
"#;
        let (new_yaml, summary) = transform(yaml).unwrap();
        let doc: serde_yaml::Value = serde_yaml::from_str(&new_yaml).unwrap();
        let threshold = doc["rules"]["fan_out"]["threshold"].as_u64().unwrap();
        assert_eq!(threshold, 7);
        assert!(summary.contains("max_fan_out"));
        assert!(summary.contains("fan_out"));
    }

    #[test]
    fn test_transform_coupling_thresholds() {
        let yaml = r#"
rules:
  coupling:
    enabled: true
    error_on_violation: true
    params:
      ca_threshold: 8
      ce_threshold: 12
"#;
        let (new_yaml, summary) = transform(yaml).unwrap();
        let doc: serde_yaml::Value = serde_yaml::from_str(&new_yaml).unwrap();
        // ca -> fan_out
        assert_eq!(doc["rules"]["fan_out"]["threshold"].as_u64().unwrap(), 8);
        // ce -> fan_in
        assert_eq!(doc["rules"]["fan_in"]["threshold"].as_u64().unwrap(), 12);
        assert!(summary.contains("fan_out"));
        assert!(summary.contains("fan_in"));
    }

    #[test]
    fn test_transform_layer_violations_builds_layers() {
        let yaml = r#"
rules:
  layer_violations:
    enabled: true
    error_on_violation: true
    params:
      layers:
        handler: 1
        service: 2
        repo: 3
"#;
        let (new_yaml, summary) = transform(yaml).unwrap();
        let doc: serde_yaml::Value = serde_yaml::from_str(&new_yaml).unwrap();
        let layers = doc["layers"].as_sequence().unwrap();
        assert_eq!(layers.len(), 3);
        assert!(summary.contains("layers"));
        // handler (rank 1) should be allowed to depend on service (2) and repo (3)
        let handler_deps = doc["allowed_dependencies"]["handler"]
            .as_sequence()
            .unwrap();
        let deps: Vec<&str> = handler_deps
            .iter()
            .filter_map(|v| v.as_str())
            .collect();
        assert!(deps.contains(&"service"));
        assert!(deps.contains(&"repo"));
    }

    #[test]
    fn test_transform_forbidden_dependencies_removes_edges() {
        let yaml = r#"
rules:
  layer_violations:
    enabled: true
    params:
      layers:
        handler: 1
        service: 2
        repo: 3
  forbidden_dependencies:
    enabled: true
    params:
      rules:
        - from: handler
          to: repo
"#;
        let (new_yaml, _) = transform(yaml).unwrap();
        let doc: serde_yaml::Value = serde_yaml::from_str(&new_yaml).unwrap();
        let handler_deps: Vec<&str> = doc["allowed_dependencies"]["handler"]
            .as_sequence()
            .unwrap()
            .iter()
            .filter_map(|v| v.as_str())
            .collect();
        // repo should have been removed as forbidden
        assert!(!handler_deps.contains(&"repo"));
        // service is still allowed
        assert!(handler_deps.contains(&"service"));
    }

    #[test]
    fn test_transform_defaults_when_no_thresholds() {
        // Old schema present but no explicit thresholds -> use defaults
        let yaml = r#"
rules:
  max_fan_out:
    enabled: true
"#;
        let (new_yaml, _) = transform(yaml).unwrap();
        let doc: serde_yaml::Value = serde_yaml::from_str(&new_yaml).unwrap();
        assert_eq!(doc["rules"]["fan_out"]["threshold"].as_u64().unwrap(), 5);
        assert_eq!(doc["rules"]["fan_in"]["threshold"].as_u64().unwrap(), 10);
    }

    // --- migrate() file-level tests ---

    #[test]
    fn test_migrate_up_to_date_returns_up_to_date() {
        let dir = TempDir::new().unwrap();
        let path = write_file(
            &dir,
            ".archlint.yaml",
            r#"
rules:
  fan_out:
    threshold: 5
"#,
        );
        let result = migrate(&path, false).unwrap();
        assert_eq!(result, MigrateResult::UpToDate);
    }

    #[test]
    fn test_migrate_dry_run_does_not_write() {
        let dir = TempDir::new().unwrap();
        let original = r#"
rules:
  max_fan_out:
    threshold: 6
"#;
        let path = write_file(&dir, ".archlint.yaml", original);
        let result = migrate(&path, true).unwrap();
        match result {
            MigrateResult::DryRun(summary) => {
                assert!(!summary.is_empty());
            }
            other => panic!("expected DryRun, got {:?}", other),
        }
        // File must be unchanged
        let content = std::fs::read_to_string(&path).unwrap();
        assert_eq!(content, original);
    }

    #[test]
    fn test_migrate_creates_backup_and_rewrites() {
        let dir = TempDir::new().unwrap();
        let path = write_file(
            &dir,
            ".archlint.yaml",
            r#"
rules:
  max_fan_out:
    threshold: 6
"#,
        );
        let result = migrate(&path, false).unwrap();
        match result {
            MigrateResult::Migrated { backup, .. } => {
                // Backup must exist
                assert!(std::path::Path::new(&backup).exists());
                // New config must parse with current schema
                let new_content = std::fs::read_to_string(&path).unwrap();
                let doc: serde_yaml::Value = serde_yaml::from_str(&new_content).unwrap();
                assert_eq!(doc["rules"]["fan_out"]["threshold"].as_u64().unwrap(), 6);
            }
            other => panic!("expected Migrated, got {:?}", other),
        }
    }

    #[test]
    fn test_migrate_full_old_schema() {
        // Simulate the deskd .archlint.yaml style
        let dir = TempDir::new().unwrap();
        let path = write_file(
            &dir,
            ".archlint.yaml",
            r#"
rules:
  max_fan_out:
    enabled: true
    threshold: 5
    error_on_violation: true
    exclude:
      - cmd/*
  coupling:
    enabled: true
    error_on_violation: true
    params:
      ca_threshold: 10
      ce_threshold: 10
  layer_violations:
    enabled: true
    error_on_violation: true
    params:
      layers:
        cmd: 0
        handler: 1
        service: 2
        repo: 3
  forbidden_dependencies:
    enabled: true
    error_on_violation: true
    params:
      rules:
        - from: handler
          to: repo
"#,
        );
        let result = migrate(&path, false).unwrap();
        match result {
            MigrateResult::Migrated { .. } => {
                let content = std::fs::read_to_string(&path).unwrap();
                let doc: serde_yaml::Value = serde_yaml::from_str(&content).unwrap();
                assert_eq!(doc["rules"]["fan_out"]["threshold"].as_u64().unwrap(), 5);
                assert_eq!(doc["rules"]["fan_in"]["threshold"].as_u64().unwrap(), 10);
                // layers created
                let layers = doc["layers"].as_sequence().unwrap();
                assert_eq!(layers.len(), 4);
                // forbidden dep removed
                let handler_deps: Vec<&str> = doc["allowed_dependencies"]["handler"]
                    .as_sequence()
                    .unwrap()
                    .iter()
                    .filter_map(|v| v.as_str())
                    .collect();
                assert!(!handler_deps.contains(&"repo"));
            }
            other => panic!("expected Migrated, got {:?}", other),
        }
    }
}
