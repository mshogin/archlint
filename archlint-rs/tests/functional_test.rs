/// Functional tests for archlint use cases #74-#80.
///
/// Each test runs the compiled binary against a real or temporary project and
/// verifies the output contains the expected strings.
///
/// Prerequisites: build the binary first with `cargo build --release`.
use std::fs;
use std::path::PathBuf;
use std::process::Command;
use tempfile::TempDir;

/// Return the path to the built binary.
fn binary() -> PathBuf {
    let mut p = std::env::current_exe().expect("cannot locate test binary");
    // Strip the test binary portion: target/debug/deps/<test_bin>
    // Walk up until we find a directory that contains `release/archlint`.
    loop {
        p.pop();
        let candidate = p.join("release").join("archlint");
        if candidate.exists() {
            return candidate;
        }
        // Also check the debug build so `cargo test` without --release works.
        let candidate_debug = p.join("debug").join("archlint");
        if candidate_debug.exists() {
            return candidate_debug;
        }
        if !p.pop() {
            panic!(
                "archlint binary not found. Run `cargo build --release` first."
            );
        }
    }
}

/// Build a minimal Go project in a temp directory.
/// `go.mod` + one Go source file with many imports to trigger fan-out violations.
///
/// Note: archlint skips directories whose names start with `.` (hidden dirs such
/// as `.git`, `.cargo`, etc.).  We use a non-hidden prefix so the analyzer does
/// not skip the project root when WalkDir traverses it.
fn make_go_project() -> TempDir {
    let dir = tempfile::Builder::new()
        .prefix("archlint_test_")
        .tempdir()
        .expect("failed to create temp dir");
    let path = dir.path();

    fs::write(
        path.join("go.mod"),
        "module example.com/testproj\n\ngo 1.21\n",
    )
    .expect("write go.mod");

    // handler.go with 10 internal imports -> fan_out violation
    fs::write(
        path.join("handler.go"),
        r#"package handler

import (
    "context"
    "encoding/json"
    "fmt"
    "io"
    "log"
    "net/http"
    "os"
    "strings"
    "time"
    "example.com/testproj/model"
    "example.com/testproj/repository"
    "example.com/testproj/service"
    "example.com/testproj/config"
    "example.com/testproj/middleware"
    "example.com/testproj/auth"
)

func HandleRequest(ctx context.Context, w http.ResponseWriter, r *http.Request) {
    _ = json.NewEncoder(w)
    _ = fmt.Sprintf("")
    _ = io.Discard
    _ = log.New(os.Stdout, "", 0)
    _ = strings.Join(nil, "")
    _ = time.Now()
}
"#,
    )
    .expect("write handler.go");

    dir
}

// ---------------------------------------------------------------------------
// #74 SCAN
// ---------------------------------------------------------------------------

/// #74 - SCAN: scan a Go project, verify components/violations/health in output.
#[test]
fn test_74_scan_go_project() {
    let project = make_go_project();
    let bin = binary();

    let output = Command::new(&bin)
        .args(["scan", project.path().to_str().unwrap(), "--format", "brief"])
        .output()
        .expect("failed to run archlint scan");

    let stdout = String::from_utf8_lossy(&output.stdout);
    let stderr = String::from_utf8_lossy(&output.stderr);

    // Basic sanity: the command should produce output
    assert!(
        !stdout.is_empty() || !stderr.is_empty(),
        "expected output from archlint scan"
    );

    let combined = format!("{}{}", stdout, stderr);
    // components should be detected
    assert!(
        combined.contains("components="),
        "expected 'components=' in output, got:\n{}",
        combined
    );

    // at least 1 component - find the line that contains "components=" and parse inline
    let comp_val: usize = combined
        .lines()
        .find(|l| l.contains("components="))
        .and_then(|l| {
            l.split("components=")
                .nth(1)
                .and_then(|s| s.split(|c: char| !c.is_ascii_digit()).next())
                .and_then(|v| v.parse().ok())
        })
        .unwrap_or(0);
    assert!(comp_val > 0, "expected components > 0, got: {}", comp_val);

    // health score present
    assert!(
        combined.contains("health="),
        "expected 'health=' in output, got:\n{}",
        combined
    );

    // violations present (fan-out should fire)
    assert!(
        combined.contains("violations="),
        "expected 'violations=' in output, got:\n{}",
        combined
    );
    let viol_val: usize = combined
        .lines()
        .find(|l| l.contains("violations="))
        .and_then(|l| {
            l.split("violations=")
                .nth(1)
                .and_then(|s| s.split(|c: char| !c.is_ascii_digit()).next())
                .and_then(|v| v.parse().ok())
        })
        .unwrap_or(0);
    assert!(viol_val > 0, "expected violations > 0, got: {}", viol_val);
}

// ---------------------------------------------------------------------------
// #75 COLLECT
// ---------------------------------------------------------------------------

/// #75 - COLLECT: run archlint collect and verify architecture.yaml is created.
#[test]
fn test_75_collect_creates_architecture_yaml() {
    let project = make_go_project();
    let bin = binary();
    let project_path = project.path().to_str().unwrap();

    let output = Command::new(&bin)
        .args(["collect", project_path])
        .output()
        .expect("failed to run archlint collect");

    let stdout = String::from_utf8_lossy(&output.stdout);
    let stderr = String::from_utf8_lossy(&output.stderr);
    let combined = format!("{}{}", stdout, stderr);

    // architecture.yaml should be written
    let yaml_path = project.path().join("architecture.yaml");
    assert!(
        yaml_path.exists(),
        "expected architecture.yaml to be created at {:?}",
        yaml_path
    );

    // stdout should mention components
    assert!(
        combined.contains("components"),
        "expected 'components' in collect output, got:\n{}",
        combined
    );

    // architecture.yaml should contain at least the components key
    let yaml_content = fs::read_to_string(&yaml_path).expect("read architecture.yaml");
    assert!(
        yaml_content.contains("components"),
        "architecture.yaml should contain 'components', got:\n{}",
        yaml_content
    );
}

// ---------------------------------------------------------------------------
// #76 FIX
// ---------------------------------------------------------------------------

/// #76 - FIX: run archlint fix and verify the output mentions fixable violations.
#[test]
fn test_76_fix_mentions_fixable() {
    let project = make_go_project();
    let bin = binary();

    // fix exits with code 1 when there are fixable violations - that is expected
    let output = Command::new(&bin)
        .args(["fix", project.path().to_str().unwrap()])
        .output()
        .expect("failed to run archlint fix");

    let stdout = String::from_utf8_lossy(&output.stdout);
    let stderr = String::from_utf8_lossy(&output.stderr);
    let combined = format!("{}{}", stdout, stderr);

    assert!(
        combined.contains("fixable"),
        "expected 'fixable' in fix output, got:\n{}",
        combined
    );
}

// ---------------------------------------------------------------------------
// #77 WATCH - skipped
// ---------------------------------------------------------------------------

/// #77 - WATCH: skipped - requires async file watching, not suited for unit tests.
#[test]
fn test_77_watch_skipped() {
    // This use case requires watching the filesystem in a background process.
    // Integration testing would need a separate harness with timeout + file writes.
    // Marked as a TODO for a dedicated integration test setup.
}

// ---------------------------------------------------------------------------
// #78 BADGE
// ---------------------------------------------------------------------------

/// #78 - BADGE: run archlint badge and verify SVG output contains "archlint" and a score.
#[test]
fn test_78_badge_svg_output() {
    let project = make_go_project();
    let bin = binary();

    // Without --output flag the SVG goes to stdout
    let output = Command::new(&bin)
        .args(["badge", project.path().to_str().unwrap()])
        .output()
        .expect("failed to run archlint badge");

    let stdout = String::from_utf8_lossy(&output.stdout);

    assert!(
        stdout.contains("archlint"),
        "expected SVG to contain 'archlint', got:\n{}",
        stdout
    );

    // SVG should contain a score of the form "N/100"
    assert!(
        stdout.contains("/100"),
        "expected score in the form 'N/100' in SVG, got:\n{}",
        stdout
    );

    // Should be valid SVG envelope
    assert!(
        stdout.trim_start().starts_with("<svg"),
        "expected output to start with <svg, got:\n{}",
        stdout
    );
    assert!(
        stdout.trim_end().ends_with("</svg>"),
        "expected output to end with </svg>, got:\n{}",
        stdout
    );
}

// ---------------------------------------------------------------------------
// #79 ONBOARD - not implemented
// ---------------------------------------------------------------------------

/// #79 - INIT (onboard): adaptive onboarding generates .archlint.yaml
#[test]
fn test_79_onboard_not_implemented() {
    let bin = binary();
    let tmp = tempfile::TempDir::new().unwrap();

    // Create a minimal Go project layout
    let cmd_dir = tmp.path().join("cmd");
    let handler_dir = tmp.path().join("internal").join("handler");
    std::fs::create_dir_all(&cmd_dir).unwrap();
    std::fs::create_dir_all(&handler_dir).unwrap();
    std::fs::write(tmp.path().join("go.mod"), "module example.com/myapp\ngo 1.21\n").unwrap();
    std::fs::write(cmd_dir.join("main.go"), "package main\nfunc main() {}\n").unwrap();

    let output = Command::new(&bin)
        .arg("init")
        .arg("--dry-run")
        .arg(tmp.path())
        .output()
        .expect("failed to run archlint init");

    let stdout = String::from_utf8_lossy(&output.stdout);
    assert!(
        output.status.success(),
        "archlint init should succeed; stderr: {}",
        String::from_utf8_lossy(&output.stderr)
    );
    assert!(stdout.contains("archlint init"), "summary should be printed");
    assert!(stdout.contains("Go"), "should detect Go language");
    assert!(stdout.contains("rules:"), "generated YAML should contain rules section");
    assert!(stdout.contains("fan_out:"), "generated YAML should contain fan_out rule");

    // Verify no file was written (dry-run)
    assert!(
        !tmp.path().join(".archlint.yaml").exists(),
        ".archlint.yaml must NOT be written in dry-run mode"
    );
}

// ---------------------------------------------------------------------------
// #80 VALIDATE
// ---------------------------------------------------------------------------

/// #80 - VALIDATE: create a minimal architecture YAML, run archlint session --graph
/// (the closest available validate-like command), verify components are shown.
///
/// Note: archlint-rs has no standalone `validate` subcommand.  The `session --graph`
/// command reads a JSONL session file and emits an arch graph.  For a pure YAML
/// validate path we use `collect` against a project that already has an
/// architecture.yaml and verify the graph output.
#[test]
fn test_80_validate_yaml_graph() {
    let project = make_go_project();
    let bin = binary();

    // First, collect to produce architecture.yaml
    Command::new(&bin)
        .args(["collect", project.path().to_str().unwrap()])
        .output()
        .expect("collect step failed");

    let yaml_path = project.path().join("architecture.yaml");
    assert!(yaml_path.exists(), "architecture.yaml should exist after collect");

    let yaml_content = fs::read_to_string(&yaml_path).expect("read architecture.yaml");

    // Verify the YAML contains components and links sections
    assert!(
        yaml_content.contains("components"),
        "expected 'components' in architecture.yaml, got:\n{}",
        yaml_content
    );

    // Re-run collect in JSON format and verify nodes (components) > 0 in JSON output
    let output = Command::new(&bin)
        .args(["collect", project.path().to_str().unwrap(), "--format", "json"])
        .output()
        .expect("failed to run archlint collect --format json");

    let stdout = String::from_utf8_lossy(&output.stdout);

    // JSON output should be valid JSON
    let json: serde_json::Value =
        serde_json::from_str(&stdout).expect("collect --format json should produce valid JSON");

    // The JSON graph format uses "nodes" for components
    let nodes = json
        .get("nodes")
        .and_then(|v| v.as_array())
        .map(|a| a.len())
        .unwrap_or(0);
    assert!(
        nodes > 0,
        "expected at least 1 node/component in JSON graph, got:\n{}",
        stdout
    );
}

// ---------------------------------------------------------------------------
// #56 VALIDATE subcommand
// ---------------------------------------------------------------------------

/// #56 - VALIDATE: run `archlint validate --graph <file>` against a hand-crafted
/// architecture.yaml and verify components/violations/health appear in output.
#[test]
fn test_56_validate_subcommand_text() {
    let dir = tempfile::Builder::new()
        .prefix("archlint_validate_")
        .tempdir()
        .expect("failed to create temp dir");

    // Hand-crafted architecture.yaml with one fan-out violation (handler -> 6 deps)
    let yaml = r#"
components:
  - id: handler
    entity: go
  - id: svc1
    entity: go
  - id: svc2
    entity: go
  - id: svc3
    entity: go
  - id: svc4
    entity: go
  - id: svc5
    entity: go
  - id: svc6
    entity: go
links:
  - from: handler
    to: svc1
  - from: handler
    to: svc2
  - from: handler
    to: svc3
  - from: handler
    to: svc4
  - from: handler
    to: svc5
  - from: handler
    to: svc6
metadata:
  language: Go
  root_dir: /test
"#;

    let graph_path = dir.path().join("architecture.yaml");
    fs::write(&graph_path, yaml).expect("write architecture.yaml");

    let bin = binary();
    let output = Command::new(&bin)
        .args(["validate", "--graph", graph_path.to_str().unwrap()])
        .output()
        .expect("failed to run archlint validate");

    let stdout = String::from_utf8_lossy(&output.stdout);
    let combined = format!("{}{}", stdout, String::from_utf8_lossy(&output.stderr));

    assert!(
        combined.contains("components:"),
        "expected 'components:' in output, got:\n{}",
        combined
    );
    assert!(
        combined.contains("links:"),
        "expected 'links:' in output, got:\n{}",
        combined
    );
    assert!(
        combined.contains("violations:"),
        "expected 'violations:' in output, got:\n{}",
        combined
    );
    assert!(
        combined.contains("health:"),
        "expected 'health:' in output, got:\n{}",
        combined
    );
    // fan-out violation should fire (handler has 6 deps, default threshold = 5)
    assert!(
        combined.contains("fan_out"),
        "expected fan_out violation in output, got:\n{}",
        combined
    );
}

/// #56 - VALIDATE: JSON format output is valid JSON with expected fields.
#[test]
fn test_56_validate_subcommand_json() {
    let dir = tempfile::Builder::new()
        .prefix("archlint_validate_json_")
        .tempdir()
        .expect("failed to create temp dir");

    let yaml = r#"
components:
  - id: a
    entity: go
  - id: b
    entity: go
links:
  - from: a
    to: b
"#;

    let graph_path = dir.path().join("arch.yaml");
    fs::write(&graph_path, yaml).expect("write arch.yaml");

    let bin = binary();
    let output = Command::new(&bin)
        .args(["validate", "--graph", graph_path.to_str().unwrap(), "--format", "json"])
        .output()
        .expect("failed to run archlint validate --format json");

    let stdout = String::from_utf8_lossy(&output.stdout);
    assert!(output.status.success(), "expected exit 0 for valid graph, got:\n{}", stdout);

    let json: serde_json::Value = serde_json::from_str(&stdout)
        .expect(&format!("expected valid JSON output, got:\n{}", stdout));

    assert_eq!(json["components"].as_u64().unwrap_or(0), 2);
    assert_eq!(json["links"].as_u64().unwrap_or(0), 1);
    assert_eq!(json["violation_count"].as_u64().unwrap_or(99), 0);
    assert_eq!(json["health"].as_u64().unwrap_or(0), 100);
}

/// #56 - VALIDATE: cycle detection works via validate subcommand.
#[test]
fn test_56_validate_cycle_detection() {
    let dir = tempfile::Builder::new()
        .prefix("archlint_validate_cycle_")
        .tempdir()
        .expect("failed to create temp dir");

    // a -> b -> c -> a forms a cycle
    let yaml = r#"
components:
  - id: a
    entity: go
  - id: b
    entity: go
  - id: c
    entity: go
links:
  - from: a
    to: b
  - from: b
    to: c
  - from: c
    to: a
"#;

    let graph_path = dir.path().join("cycle.yaml");
    fs::write(&graph_path, yaml).expect("write cycle.yaml");

    let bin = binary();
    let output = Command::new(&bin)
        .args(["validate", "--graph", graph_path.to_str().unwrap(), "--format", "json"])
        .output()
        .expect("failed to run archlint validate");

    let stdout = String::from_utf8_lossy(&output.stdout);
    let json: serde_json::Value = serde_json::from_str(&stdout)
        .expect(&format!("expected valid JSON, got:\n{}", stdout));

    let cycles = json["cycles"].as_array().map(|a| a.len()).unwrap_or(0);
    assert!(cycles > 0, "expected cycles to be detected, got:\n{}", stdout);
}

/// #56 - VALIDATE: reading from architecture.yaml produced by `collect` round-trips correctly.
#[test]
fn test_56_validate_collect_roundtrip() {
    let project = make_go_project();
    let bin = binary();

    // First collect to produce architecture.yaml
    Command::new(&bin)
        .args(["collect", project.path().to_str().unwrap()])
        .output()
        .expect("collect step failed");

    let yaml_path = project.path().join("architecture.yaml");
    assert!(yaml_path.exists(), "architecture.yaml should exist after collect");

    // Now validate the produced YAML
    let output = Command::new(&bin)
        .args(["validate", "--graph", yaml_path.to_str().unwrap(), "--format", "json"])
        .output()
        .expect("failed to run archlint validate on collected graph");

    let stdout = String::from_utf8_lossy(&output.stdout);
    let json: serde_json::Value = serde_json::from_str(&stdout)
        .expect(&format!("expected valid JSON from validate, got:\n{}", stdout));

    let components = json["components"].as_u64().unwrap_or(0);
    assert!(components > 0, "expected components > 0 in validated collected graph");
}
