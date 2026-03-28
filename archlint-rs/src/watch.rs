//! Live architecture monitoring with optional auto-fix suggestions.
//!
//! `watch(dir, fix_mode)` sets up a file-system watcher on `dir`, waits for
//! changes to `.go` or `.rs` files, debounces rapid saves (500 ms), then runs
//! `analyzer::analyze` and prints violations with ANSI-colored output.
//! When `fix_mode` is `true` it also calls `fix::suggest_fixes` and prints
//! the suggestions.
//!
//! Color helpers use raw ANSI escape sequences — no extra crate required.

use crate::analyzer;
use crate::fix;
use notify::{Config, Event, EventKind, RecommendedWatcher, RecursiveMode, Watcher};
use std::path::{Path, PathBuf};
use std::sync::mpsc;
use std::time::{Duration, Instant};

// ---------------------------------------------------------------------------
// ANSI colour helpers
// ---------------------------------------------------------------------------

const RED: &str = "\x1b[31m";
const GREEN: &str = "\x1b[32m";
const YELLOW: &str = "\x1b[33m";
const CYAN: &str = "\x1b[36m";
const BOLD: &str = "\x1b[1m";
const RESET: &str = "\x1b[0m";

fn red(s: &str) -> String {
    format!("{RED}{s}{RESET}")
}
fn green(s: &str) -> String {
    format!("{GREEN}{s}{RESET}")
}
fn yellow(s: &str) -> String {
    format!("{YELLOW}{s}{RESET}")
}
fn cyan(s: &str) -> String {
    format!("{CYAN}{s}{RESET}")
}
fn bold(s: &str) -> String {
    format!("{BOLD}{s}{RESET}")
}

// ---------------------------------------------------------------------------
// Debounce constant
// ---------------------------------------------------------------------------

const DEBOUNCE_MS: u64 = 500;

// ---------------------------------------------------------------------------
// Public entry point
// ---------------------------------------------------------------------------

/// Start watching `dir` for `.go` / `.rs` file changes.
///
/// Blocks indefinitely (runs the watch loop).  Returns `Err` if the watcher
/// cannot be set up.
pub fn watch(dir: &Path, fix_mode: bool) -> Result<(), String> {
    let (tx, rx) = mpsc::channel::<Result<Event, notify::Error>>();

    let mut watcher = RecommendedWatcher::new(tx, Config::default())
        .map_err(|e| format!("failed to create watcher: {e}"))?;

    watcher
        .watch(dir, RecursiveMode::Recursive)
        .map_err(|e| format!("failed to watch {}: {e}", dir.display()))?;

    println!(
        "{} {}",
        bold("[archlint watch]"),
        cyan(&format!("monitoring {} ...", dir.display()))
    );
    if fix_mode {
        println!("{}", yellow("  --fix mode: fix suggestions will be shown on violations"));
    }
    println!();

    // Debounce: track the last time we received an event and the path that
    // triggered it, then wait until no event arrives for DEBOUNCE_MS before
    // scanning.
    let debounce = Duration::from_millis(DEBOUNCE_MS);
    let mut pending: Option<(Instant, PathBuf)> = None;

    loop {
        // Non-blocking receive with a short timeout so we can check the
        // debounce deadline even when no new events arrive.
        match rx.recv_timeout(Duration::from_millis(50)) {
            Ok(Ok(event)) => {
                if let Some(path) = relevant_path(&event) {
                    pending = Some((Instant::now(), path));
                }
            }
            Ok(Err(e)) => {
                eprintln!("{} watcher error: {e}", yellow("[warn]"));
            }
            Err(mpsc::RecvTimeoutError::Timeout) => {
                // no new event — check if debounce window has elapsed
            }
            Err(mpsc::RecvTimeoutError::Disconnected) => {
                return Err("watcher channel closed unexpectedly".to_string());
            }
        }

        // Fire the scan when the debounce window has elapsed.
        if let Some((last, ref path)) = pending {
            if last.elapsed() >= debounce {
                run_scan(dir, path, fix_mode);
                pending = None;
            }
        }
    }
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

/// Return the first relevant (`.go` / `.rs`) path from a notify event.
fn relevant_path(event: &Event) -> Option<PathBuf> {
    // We care about create / modify events only.
    match event.kind {
        EventKind::Create(_) | EventKind::Modify(_) => {}
        _ => return None,
    }

    event.paths.iter().find_map(|p| {
        let ext = p.extension().and_then(|e| e.to_str()).unwrap_or("");
        if ext == "go" || ext == "rs" {
            Some(p.clone())
        } else {
            None
        }
    })
}

/// Run the architecture scan for `dir` triggered by a change in `changed`.
fn run_scan(dir: &Path, changed: &Path, fix_mode: bool) {
    let ts = timestamp();
    println!(
        "{} {} {}",
        cyan(&format!("[{ts}]")),
        bold("[watching]"),
        changed.display()
    );

    match analyzer::analyze(dir) {
        Ok(graph) => {
            let metrics = graph.metrics.as_ref();
            let violations = metrics.map(|m| m.violations.as_slice()).unwrap_or(&[]);
            let cycles = metrics.map(|m| m.cycles.as_slice()).unwrap_or(&[]);

            if violations.is_empty() && cycles.is_empty() {
                println!("{} no violations", green("[ok]"));
            } else {
                println!(
                    "{} {} violation(s), {} cycle(s)",
                    red("[scan]"),
                    violations.len(),
                    cycles.len()
                );

                for v in violations {
                    println!(
                        "  {} rule={} component={} msg={}",
                        red("VIOLATION"),
                        bold(&v.rule),
                        v.component,
                        v.message
                    );
                }

                for cycle in cycles {
                    println!(
                        "  {} {}",
                        red("CYCLE"),
                        bold(&cycle.join(" -> "))
                    );
                }

                if fix_mode {
                    println!();
                    let report = fix::suggest_fixes(&graph);
                    if report.fixable == 0 {
                        println!("{} no auto-fix suggestions available", yellow("[fix]"));
                    } else {
                        println!("{} {} fix suggestion(s):", green("[fix]"), report.fixable);
                        for s in &report.suggestions {
                            println!(
                                "  {} rule={} component={}",
                                green("FIX"),
                                bold(&s.rule),
                                s.component
                            );
                            println!("    suggestion: {}", s.suggestion);
                            for item in &s.action_items {
                                println!("    - {item}");
                            }
                        }
                    }
                }
            }
        }
        Err(e) => {
            eprintln!("{} scan error: {e}", red("[error]"));
        }
    }

    println!();
}

/// Return the current wall-clock time as a human-readable string.
fn timestamp() -> String {
    use std::time::{SystemTime, UNIX_EPOCH};
    let secs = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs();
    let h = (secs % 86400) / 3600;
    let m = (secs % 3600) / 60;
    let s = secs % 60;
    format!("{h:02}:{m:02}:{s:02}")
}

// ---------------------------------------------------------------------------
// Tests
// ---------------------------------------------------------------------------

#[cfg(test)]
mod tests {
    use super::*;
    use notify::{event::ModifyKind, Event, EventKind};
    use std::path::PathBuf;

    fn make_event(kind: EventKind, paths: Vec<PathBuf>) -> Event {
        Event {
            kind,
            paths,
            attrs: Default::default(),
        }
    }

    #[test]
    fn test_relevant_path_rs_file_modify() {
        let path = PathBuf::from("/project/src/main.rs");
        let event = make_event(
            EventKind::Modify(ModifyKind::Data(notify::event::DataChange::Content)),
            vec![path.clone()],
        );
        let result = relevant_path(&event);
        assert_eq!(result, Some(path));
    }

    #[test]
    fn test_relevant_path_go_file_modify() {
        let path = PathBuf::from("/project/internal/handler.go");
        let event = make_event(
            EventKind::Modify(ModifyKind::Data(notify::event::DataChange::Content)),
            vec![path.clone()],
        );
        let result = relevant_path(&event);
        assert_eq!(result, Some(path));
    }

    #[test]
    fn test_relevant_path_create_rs() {
        let path = PathBuf::from("/project/src/lib.rs");
        let event = make_event(
            EventKind::Create(notify::event::CreateKind::File),
            vec![path.clone()],
        );
        let result = relevant_path(&event);
        assert_eq!(result, Some(path));
    }

    #[test]
    fn test_relevant_path_ignores_toml() {
        let path = PathBuf::from("/project/Cargo.toml");
        let event = make_event(
            EventKind::Modify(ModifyKind::Data(notify::event::DataChange::Content)),
            vec![path],
        );
        let result = relevant_path(&event);
        assert!(result.is_none());
    }

    #[test]
    fn test_relevant_path_ignores_txt() {
        let path = PathBuf::from("/project/README.txt");
        let event = make_event(
            EventKind::Modify(ModifyKind::Data(notify::event::DataChange::Content)),
            vec![path],
        );
        let result = relevant_path(&event);
        assert!(result.is_none());
    }

    #[test]
    fn test_relevant_path_ignores_remove_event() {
        let path = PathBuf::from("/project/src/main.rs");
        let event = make_event(
            EventKind::Remove(notify::event::RemoveKind::File),
            vec![path],
        );
        let result = relevant_path(&event);
        assert!(result.is_none());
    }

    #[test]
    fn test_relevant_path_empty_paths() {
        let event = make_event(
            EventKind::Modify(ModifyKind::Data(notify::event::DataChange::Content)),
            vec![],
        );
        let result = relevant_path(&event);
        assert!(result.is_none());
    }

    #[test]
    fn test_relevant_path_picks_first_relevant() {
        let txt = PathBuf::from("/project/README.txt");
        let rs = PathBuf::from("/project/src/lib.rs");
        let event = make_event(
            EventKind::Modify(ModifyKind::Data(notify::event::DataChange::Content)),
            vec![txt, rs.clone()],
        );
        let result = relevant_path(&event);
        assert_eq!(result, Some(rs));
    }

    #[test]
    fn test_timestamp_format() {
        let ts = timestamp();
        // Should be HH:MM:SS
        let parts: Vec<&str> = ts.split(':').collect();
        assert_eq!(parts.len(), 3);
        for part in &parts {
            assert_eq!(part.len(), 2);
            assert!(part.chars().all(|c| c.is_ascii_digit()));
        }
    }

    #[test]
    fn test_debounce_constant() {
        assert_eq!(DEBOUNCE_MS, 500);
    }
}
