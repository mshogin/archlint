mod analyzer;
mod badge;
mod language_analyzer;
mod config;
mod costlint;
mod diagram;
mod diff;
mod fix;
mod migrate;
mod model;
mod nightly;
mod onboard;
mod orchestrator;
mod perflint;
mod promptlint;
mod seclint;
mod server;
mod session;
mod validate;
mod watch;

use clap::{Parser, Subcommand};
use std::path::PathBuf;

#[derive(Parser)]
#[command(name = "archlint")]
#[command(about = "Architecture linter - structural graphs, SOLID violations, cycle detection")]
struct Cli {
    #[command(subcommand)]
    command: Commands,
}

#[derive(Subcommand)]
enum Commands {
    /// Scan a project directory for architecture analysis
    Scan {
        /// Project directory to scan
        #[arg(default_value = ".")]
        dir: PathBuf,

        /// Output format: yaml, json, brief
        #[arg(long, default_value = "yaml")]
        format: String,

        /// Maximum violations before exit code 1
        #[arg(long)]
        threshold: Option<usize>,

        /// Show what config migration would do without writing changes
        #[arg(long)]
        dry_run: bool,
    },
    /// Analyze prompt complexity and suggest model routing
    Prompt {
        /// Output only model name
        #[arg(long)]
        model_only: bool,

        /// Output format: json, brief
        #[arg(long, default_value = "json")]
        format: String,

        /// Append telemetry record to this JSONL file
        #[arg(long)]
        log: Option<std::path::PathBuf>,
    },
    /// Show telemetry summary from a JSONL log file
    Telemetry {
        /// Path to the JSONL telemetry file
        log: std::path::PathBuf,
    },
    /// Performance analysis - complexity, nesting, allocation patterns
    Perf {
        /// Project directory to scan
        #[arg(default_value = ".")]
        dir: PathBuf,

        /// Output format: json, brief
        #[arg(long, default_value = "json")]
        format: String,
    },
    /// Rate content safety (6+/12+/16+/18+)
    Rate {
        /// Maximum allowed rating (6, 12, 16, 18)
        #[arg(long)]
        max_rating: Option<u8>,
    },
    /// Estimate token cost for a prompt
    Cost {
        /// Model to estimate for
        #[arg(long, default_value = "sonnet")]
        model: String,

        /// Compare all models
        #[arg(long)]
        compare: bool,
    },
    /// Compare architecture between two git commits
    Diff {
        /// Git range (e.g., HEAD~5..HEAD, main..feature)
        range: String,

        /// Project directory
        #[arg(long, default_value = ".")]
        dir: PathBuf,

        /// Output format: json, text
        #[arg(long, default_value = "text")]
        format: String,
    },
    /// Manage Docker-based Claude Code workers
    Worker {
        #[command(subcommand)]
        action: WorkerAction,
    },
    /// Collect architecture graph from source code (Unix-pipe format)
    Collect {
        /// Project directory to analyze
        #[arg(default_value = ".")]
        dir: PathBuf,

        /// Output format: json or text
        #[arg(long, default_value = "text")]
        format: String,
    },
    /// Start HTTP API server
    Serve {
        /// Port to listen on
        #[arg(long, default_value = "8080")]
        port: u16,
    },
    /// Quality-gate check with escalation cost metadata
    QualityGate {
        /// Project directory to scan
        #[arg(default_value = ".")]
        dir: PathBuf,

        /// Maximum violations before quality gate fails (default 0 = any violation fails)
        #[arg(long, default_value = "0")]
        threshold: usize,

        /// Current model tier (haiku, sonnet, opus) for cost delta calculation
        #[arg(long, default_value = "haiku")]
        model: String,

        /// Estimated input tokens for this request (used for cost calculation)
        #[arg(long, default_value = "1000")]
        input_tokens: usize,

        /// Estimated output tokens for this request (used for cost calculation)
        #[arg(long, default_value = "1000")]
        output_tokens: usize,

        /// Append escalation event to this JSONL file when gate fails
        #[arg(long)]
        log: Option<PathBuf>,

        /// Output format: json, brief
        #[arg(long, default_value = "json")]
        format: String,
    },
    /// Show escalation cost report from a JSONL escalation log
    EscalationReport {
        /// Path to the JSONL escalation log file
        log: PathBuf,
    },
    /// Suggest fixes for architecture violations (suggestion-only, does not modify files)
    Fix {
        /// Project directory to scan
        #[arg(default_value = ".")]
        dir: PathBuf,

        /// Output format: text, json
        #[arg(long, default_value = "text")]
        format: String,
    },
    /// Live architecture monitoring - watch for file changes and scan automatically
    Watch {
        /// Directory to watch for changes
        #[arg(default_value = ".")]
        dir: PathBuf,

        /// Automatically show fix suggestions when violations are detected
        #[arg(long)]
        fix: bool,
    },
    /// Validate an external architecture.yaml graph file (fan-out, fan-in, cycles, layer violations)
    Validate {
        /// Path to the YAML graph file (use - for stdin)
        #[arg(long)]
        graph: String,

        /// Output format: text, json, yaml
        #[arg(long, default_value = "text")]
        format: String,
    },
    /// Generate an architecture diagram from scan results
    Diagram {
        /// Project directory to scan
        #[arg(default_value = ".")]
        dir: PathBuf,

        /// Output format: mermaid, d2
        #[arg(long, default_value = "mermaid")]
        format: String,
    },
    /// Generate a health badge SVG for a project directory
    Badge {
        /// Project directory to analyze
        #[arg(default_value = ".")]
        dir: PathBuf,

        /// Output file path (use "-" or omit for stdout)
        #[arg(long)]
        output: Option<PathBuf>,
    },
    /// Adaptive onboarding: detect project structure and generate .archlint.yaml
    Init {
        /// Project directory to scan (default: current directory)
        #[arg(default_value = ".")]
        dir: PathBuf,

        /// Print the generated config without writing it to disk
        #[arg(long)]
        dry_run: bool,
    },
    /// Nightly scan: clone popular open-source projects and generate an architecture health report
    Nightly {
        /// Comma-separated list of GitHub repos to scan (owner/repo). Overrides built-in list.
        #[arg(long)]
        repos: Option<String>,

        /// Output file path for the markdown report (default: stdout)
        #[arg(long)]
        output: Option<PathBuf>,

        /// Show only the worst N projects (sorted by health score ascending)
        #[arg(long)]
        top: Option<usize>,

        /// Print progress to stderr during scan
        #[arg(long)]
        verbose: bool,
    },
    /// Analyze a Claude Code session JSONL file for workflow patterns
    Session {
        /// Path to the JSONL session file
        file: PathBuf,

        /// Detect and show workflow patterns
        #[arg(long)]
        patterns: bool,

        /// Output architecture graph as YAML (components = tool types, links = sequential calls)
        #[arg(long)]
        graph: bool,

        /// Output format: json, text
        #[arg(long, default_value = "text")]
        format: String,
    },
}

#[derive(Subcommand)]
enum WorkerAction {
    /// Create a new worker container
    Create {
        /// Model tier: haiku, sonnet, opus
        #[arg(long, default_value = "sonnet")]
        model: String,

        /// Project directory to mount into the container
        #[arg(long, default_value = ".")]
        project: PathBuf,
    },
    /// List all tracked workers
    List,
    /// Stop a running worker
    Stop {
        /// Worker ID to stop
        id: String,
    },
}

#[tokio::main]
async fn main() {
    let cli = Cli::parse();

    match cli.command {
        Commands::Prompt { model_only, format, log } => {
            use std::io::Read;
            let mut input = String::new();
            std::io::stdin().read_to_string(&mut input).expect("failed to read stdin");
            let result = promptlint::analyze(&input);

            // Append telemetry record if --log is specified
            if let Some(ref log_path) = log {
                let record = promptlint::TelemetryRecord::from_analysis(&result);
                promptlint::log_telemetry(&record, log_path);
            }

            if model_only {
                println!("{}", result.suggested_model);
            } else {
                match format.as_str() {
                    "brief" => {
                        println!(
                            "complexity={} model={} words={} action={}",
                            result.complexity, result.suggested_model, result.words, result.action
                        );
                    }
                    _ => {
                        let json = serde_json::to_string_pretty(&result).unwrap();
                        println!("{}", json);
                    }
                }
            }
        }
        Commands::Telemetry { log } => {
            match promptlint::summarize_telemetry(&log) {
                Some(summary) => {
                    let json = serde_json::to_string_pretty(&summary).unwrap();
                    println!("{}", json);
                }
                None => {
                    eprintln!("Could not read telemetry file: {}", log.display());
                    std::process::exit(1);
                }
            }
        }
        Commands::Perf { dir, format } => {
            let report = perflint::analyze(&dir);
            match format.as_str() {
                "brief" => print!("{}", perflint::format_report(&report)),
                _ => {
                    let json = serde_json::to_string_pretty(&report).unwrap();
                    println!("{}", json);
                }
            }
        }
        Commands::Rate { max_rating } => {
            use std::io::Read;
            let mut input = String::new();
            std::io::stdin().read_to_string(&mut input).expect("failed to read stdin");
            let result = seclint::classify(&input);
            let json = serde_json::to_string_pretty(&result).unwrap();
            println!("{}", json);
            if let Some(max) = max_rating {
                let threshold = match max {
                    0..=6 => seclint::Rating::Age6Plus,
                    7..=12 => seclint::Rating::Age12Plus,
                    13..=16 => seclint::Rating::Age16Plus,
                    _ => seclint::Rating::Age18Plus,
                };
                if !seclint::is_safe(&input, &threshold) {
                    std::process::exit(1);
                }
            }
        }
        Commands::Diff { range, dir, format } => {
            let parts: Vec<&str> = range.splitn(2, "..").collect();
            if parts.len() != 2 {
                eprintln!("Invalid range format. Use: FROM..TO (e.g., HEAD~5..HEAD)");
                std::process::exit(1);
            }
            match diff::diff(&dir, parts[0], parts[1]) {
                Ok(d) => {
                    match format.as_str() {
                        "json" => {
                            let json = serde_json::to_string_pretty(&d).unwrap();
                            println!("{}", json);
                        }
                        _ => print!("{}", diff::format_diff(&d)),
                    }
                }
                Err(e) => {
                    eprintln!("Error: {}", e);
                    std::process::exit(1);
                }
            }
        }
        Commands::Cost { model, compare } => {
            use std::io::Read;
            let mut input = String::new();
            std::io::stdin().read_to_string(&mut input).expect("failed to read stdin");
            let tokens = costlint::count_tokens(&input);
            if compare {
                let costs = costlint::compare_models(tokens, tokens);
                let json = serde_json::to_string_pretty(&costs).unwrap();
                println!("{}", json);
            } else {
                let cost = costlint::estimate(&model, tokens, tokens);
                println!("{{\"model\":\"{}\",\"tokens\":{},\"cost_usd\":{:.6}}}", model, tokens * 2, cost);
            }
        }
        Commands::Worker { action } => {
            let rt = || async {
                let mut orch = orchestrator::Orchestrator::new().await.map_err(|e| {
                    format!("failed to connect to Docker: {}", e)
                })?;
                // Discover existing workers from Docker
                let _ = orch.discover_workers().await;

                match action {
                    WorkerAction::Create { model, project } => {
                        let tier: orchestrator::ModelTier = model.parse().map_err(|e: String| e)?;
                        let project_path = std::fs::canonicalize(&project)
                            .map_err(|e| format!("invalid project path: {}", e))?;
                        let project_str = project_path.to_string_lossy().to_string();

                        let worker_id = orch.create_worker(tier, &project_str).await
                            .map_err(|e| format!("failed to create worker: {}", e))?;

                        orch.start_worker(&worker_id).await?;

                        let worker = orch.list_workers().into_iter()
                            .find(|w| w.id == worker_id);
                        if let Some(w) = worker {
                            let json = serde_json::to_string_pretty(&w).unwrap();
                            println!("{}", json);
                        }
                    }
                    WorkerAction::List => {
                        let workers = orch.list_workers();
                        let json = serde_json::to_string_pretty(&workers).unwrap();
                        println!("{}", json);
                    }
                    WorkerAction::Stop { id } => {
                        orch.stop_worker(&id).await?;
                        eprintln!("worker {} stopped", id);
                    }
                }
                Ok::<(), String>(())
            };

            if let Err(e) = rt().await {
                eprintln!("Error: {}", e);
                std::process::exit(1);
            }
        }
        Commands::Scan {
            dir,
            format,
            threshold,
            dry_run,
        } => {
            // Auto-migrate .archlint.yaml if old schema is detected.
            let config_path = dir.join(".archlint.yaml");
            if config_path.exists() {
                match migrate::migrate(&config_path, dry_run) {
                    Ok(migrate::MigrateResult::UpToDate) => {}
                    Ok(migrate::MigrateResult::DryRun(summary)) => {
                        eprintln!("[archlint] dry-run: config migration would apply: {}", summary);
                        eprintln!("[archlint] dry-run: run without --dry-run to apply changes.");
                    }
                    Ok(migrate::MigrateResult::Migrated { backup, summary }) => {
                        eprintln!("[archlint] config migrated: {}", summary);
                        eprintln!("[archlint] backup saved to: {}", backup);
                    }
                    Err(e) => {
                        eprintln!("[archlint] warning: config migration failed: {}", e);
                    }
                }
            }

            match analyzer::analyze_multi_language(&dir) {
                Ok(report) => {
                    match format.as_str() {
                        "json" => {
                            let json = serde_json::to_string_pretty(&report).unwrap();
                            println!("{}", json);
                        }
                        "yaml" => {
                            let yaml = serde_yaml::to_string(&report).unwrap();
                            println!("{}", yaml);
                        }
                        "brief" => {
                            // Count by level across all languages
                            let taboo: usize = report.per_language.iter().map(|r| r.taboo_count).sum();
                            let telemetry: usize = report.per_language.iter().map(|r| r.telemetry_count).sum();
                            let personal: usize = report.per_language.iter().map(|r| r.personal_count).sum();
                            let all_entries: Vec<String> = report.per_language.iter()
                                .flat_map(|r| r.entry_points.iter().cloned())
                                .collect();
                            let entry_points_str = if all_entries.is_empty() {
                                String::new()
                            } else {
                                format!(" entry_points={}", all_entries.join(","))
                            };
                            println!(
                                "languages={} components={} links={} violations={} taboo={} telemetry={} personal={} health={}/100{}",
                                report.languages.join(","),
                                report.total_components,
                                report.total_links,
                                report.total_violations,
                                taboo,
                                telemetry,
                                personal,
                                report.total_health,
                                entry_points_str,
                            );
                        }
                        _ => {
                            // Default human-readable output
                            println!("Project: {}", report.project);
                            if report.languages.is_empty() {
                                println!("Languages: none detected");
                            } else {
                                println!("Languages: {}", report.languages.join(", "));
                            }
                            println!();
                            for lang_report in &report.per_language {
                                println!("{} (architecture-{}.yaml):", lang_report.language, lang_report.language.to_lowercase());
                                println!("  Components: {}, Links: {}", lang_report.components, lang_report.links);
                                println!("  Health: {}/100", lang_report.health);
                                if !lang_report.entry_points.is_empty() {
                                    println!("  Entry points: {}", lang_report.entry_points.join(", "));
                                }
                                if lang_report.violation_count == 0 {
                                    println!("  Violations: 0");
                                } else {
                                    println!("  Violations: {} (taboo: {}, telemetry: {}, personal: {})",
                                        lang_report.violation_count,
                                        lang_report.taboo_count,
                                        lang_report.telemetry_count,
                                        lang_report.personal_count,
                                    );
                                    // Show violations grouped by level
                                    let taboo_viols: Vec<_> = lang_report.violations_detail.iter()
                                        .filter(|v| v.level == "taboo").collect();
                                    let telemetry_viols: Vec<_> = lang_report.violations_detail.iter()
                                        .filter(|v| v.level == "telemetry").collect();
                                    let personal_viols: Vec<_> = lang_report.violations_detail.iter()
                                        .filter(|v| v.level == "personal").collect();
                                    if !taboo_viols.is_empty() {
                                        println!("  [TABOO - CI BLOCKER]");
                                        for v in &taboo_viols {
                                            println!("    [{}] {} - {}", v.rule, v.component, v.message);
                                        }
                                    }
                                    if !telemetry_viols.is_empty() {
                                        println!("  [TELEMETRY - track only]");
                                        for v in &telemetry_viols {
                                            println!("    [{}] {} - {}", v.rule, v.component, v.message);
                                        }
                                    }
                                    if !personal_viols.is_empty() {
                                        println!("  [PERSONAL - informational]");
                                        for v in &personal_viols {
                                            println!("    [{}] {} - {}", v.rule, v.component, v.message);
                                        }
                                    }
                                }
                                println!();
                            }
                            println!("Total:");
                            println!("  Health: {}/100", report.total_health);
                            println!("  Violations: {} (taboo: {}, telemetry: {}, personal: {})",
                                report.total_violations,
                                report.total_taboo,
                                report.per_language.iter().map(|r| r.telemetry_count).sum::<usize>(),
                                report.per_language.iter().map(|r| r.personal_count).sum::<usize>(),
                            );
                        }
                    }

                    // Exit code: taboo violations always cause exit 1.
                    // Legacy threshold flag also triggers exit 1 if total violations exceed it.
                    if report.total_taboo > 0 {
                        std::process::exit(1);
                    }
                    if let Some(max_violations) = threshold {
                        if report.total_violations > max_violations {
                            std::process::exit(1);
                        }
                    }
                }
                Err(e) => {
                    eprintln!("Error: {}", e);
                    std::process::exit(2);
                }
            }
        }
        Commands::Collect { dir, format } => {
            match analyzer::analyze(&dir) {
                Ok(graph) => {
                    match format.as_str() {
                        "json" => {
                            let export = analyzer::to_graph_export(&graph, &dir);
                            let json = serde_json::to_string_pretty(&export).unwrap();
                            println!("{}", json);
                        }
                        _ => {
                            // YAML format (default): matches Go's architecture.yaml
                            let yaml = serde_yaml::to_string(&graph).unwrap();
                            let output_path = dir.join("architecture.yaml");
                            std::fs::write(&output_path, &yaml).unwrap();
                            println!("Граф сохранен в {}", output_path.display());
                            println!("components: {}", graph.components.len());
                            println!("links: {}", graph.links.len());
                            if let Some(ref m) = graph.metrics {
                                println!("violations: {}", m.violations.len());
                            }
                        }
                    }
                }
                Err(e) => {
                    eprintln!("Error: {}", e);
                    std::process::exit(2);
                }
            }
        }
        Commands::Serve { port } => {
            server::run(port).await;
        }
        Commands::QualityGate {
            dir,
            threshold,
            model,
            input_tokens,
            output_tokens,
            log,
            format,
        } => {
            match analyzer::analyze(&dir) {
                Ok(graph) => {
                    let violation_rules: Vec<String> = graph
                        .metrics
                        .as_ref()
                        .map(|m| m.violations.iter().map(|v| v.rule.clone()).collect())
                        .unwrap_or_default();

                    let meta = costlint::QualityGateEscalationMeta::from_violations(
                        &violation_rules,
                        &model,
                        input_tokens,
                        output_tokens,
                    );

                    let gate_failed = violation_rules.len() > threshold;

                    // Log escalation event if gate failed and a log path is given
                    if gate_failed {
                        if let Some(ref log_path) = log {
                            let event = costlint::EscalationEvent::new(
                                &model,
                                &meta.escalate_to,
                                violation_rules.clone(),
                                input_tokens,
                                output_tokens,
                            );
                            costlint::log_escalation(&event, log_path);
                        }
                    }

                    match format.as_str() {
                        "brief" => {
                            println!(
                                "gate={} violations={} escalate_to={} extra_cost=${:.6}",
                                if meta.gate_passed { "pass" } else { "fail" },
                                meta.violation_count,
                                if meta.escalate_to.is_empty() { "none" } else { &meta.escalate_to },
                                meta.estimated_escalation_cost_usd,
                            );
                        }
                        _ => {
                            let json = serde_json::to_string_pretty(&meta).unwrap();
                            println!("{}", json);
                        }
                    }

                    if gate_failed {
                        std::process::exit(1);
                    }
                }
                Err(e) => {
                    eprintln!("Error: {}", e);
                    std::process::exit(2);
                }
            }
        }
        Commands::EscalationReport { log } => {
            match std::fs::read_to_string(&log) {
                Ok(content) => {
                    let report = costlint::escalation_report(&content);
                    let json = serde_json::to_string_pretty(&report).unwrap();
                    println!("{}", json);
                }
                Err(e) => {
                    eprintln!("Could not read escalation log {}: {}", log.display(), e);
                    std::process::exit(1);
                }
            }
        }
        Commands::Fix { dir, format } => {
            match analyzer::analyze(&dir) {
                Ok(graph) => {
                    let report = fix::suggest_fixes(&graph);
                    match format.as_str() {
                        "json" => {
                            let json = serde_json::to_string_pretty(&report).unwrap();
                            println!("{}", json);
                        }
                        _ => {
                            print!("{}", fix::format_report(&report));
                        }
                    }
                    // Exit 1 if there are fixable violations (useful in CI)
                    if report.fixable > 0 {
                        std::process::exit(1);
                    }
                }
                Err(e) => {
                    eprintln!("Error: {}", e);
                    std::process::exit(2);
                }
            }
        }
        Commands::Watch { dir, fix } => {
            if let Err(e) = watch::watch(&dir, fix) {
                eprintln!("Error: {}", e);
                std::process::exit(1);
            }
        }
        Commands::Session { file, patterns, graph, format } => {
            let content = match std::fs::read_to_string(&file) {
                Ok(c) => c,
                Err(e) => {
                    eprintln!("Error reading {}: {}", file.display(), e);
                    std::process::exit(1);
                }
            };

            if graph {
                let arch = session::to_arch_graph(&content);
                let yaml = serde_yaml::to_string(&arch).unwrap();
                println!("{}", yaml);
                return;
            }

            let report = session::analyze(&content);

            match format.as_str() {
                "json" => {
                    let json = serde_json::to_string_pretty(&report).unwrap();
                    println!("{}", json);
                }
                _ => {
                    // Text format
                    println!("Session analysis: {}", file.display());
                    println!("  Total tool calls: {}", report.total_tool_calls);
                    println!();

                    if !report.tool_frequencies.is_empty() {
                        println!("Tool frequencies:");
                        for f in &report.tool_frequencies {
                            println!("  {:20} {}", f.tool, f.count);
                        }
                        println!();
                    }

                    if patterns || !report.patterns.is_empty() {
                        println!("Detected patterns:");
                        if report.patterns.is_empty() {
                            println!("  (none)");
                        } else {
                            for p in &report.patterns {
                                println!(
                                    "  [{}] {} (x{}) - {}",
                                    p.name,
                                    p.sequence.join("->"),
                                    p.occurrences,
                                    p.description
                                );
                            }
                        }
                        println!();
                    }

                    if !report.flags.is_empty() {
                        println!("Flags:");
                        for f in &report.flags {
                            let tool_str = f.tool.as_deref().map(|t| format!(" ({})", t)).unwrap_or_default();
                            println!("  [{}]{} {}", f.kind, tool_str, f.detail);
                        }
                        println!();
                    }

                    // Entropy and conditional entropy (#83, #84)
                    println!("Entropy metrics:");
                    println!("  Shannon entropy:     {:.4}", report.entropy);
                    println!("  Conditional entropy: {:.4}", report.conditional_entropy);
                    println!();

                    // PageRank (#85)
                    if !report.pagerank.is_empty() {
                        let mut pr_sorted: Vec<(&String, &f64)> = report.pagerank.iter().collect();
                        pr_sorted.sort_by(|a, b| b.1.partial_cmp(a.1).unwrap_or(std::cmp::Ordering::Equal));
                        println!("PageRank (top 5):");
                        for (tool, score) in pr_sorted.iter().take(5) {
                            println!("  {:20} {:.4}", tool, score);
                        }
                        println!();
                    }

                    // Bottlenecks / betweenness centrality (#87)
                    if !report.bottlenecks.is_empty() {
                        println!("Bottlenecks (betweenness centrality):");
                        for (tool, score) in report.bottlenecks.iter().take(5) {
                            println!("  {:20} {:.4}", tool, score);
                        }
                        println!();
                    }

                    // Session phases (#88)
                    if !report.phases.is_empty() {
                        println!("Session phases:");
                        for (i, phase) in report.phases.iter().enumerate() {
                            println!(
                                "  Phase {}: [{}-{}] dominant={}",
                                i + 1,
                                phase.start_idx,
                                phase.end_idx,
                                phase.dominant_tool
                            );
                        }
                        println!();
                    }

                    // Transition matrix (#82) - only with --patterns flag
                    if patterns && !report.transition_matrix.is_empty() {
                        println!("Transition matrix (P(B|A), top 10):");
                        let mut tm: Vec<(&(String, String), &f64)> =
                            report.transition_matrix.iter().collect();
                        tm.sort_by(|a, b| {
                            b.1.partial_cmp(a.1).unwrap_or(std::cmp::Ordering::Equal)
                        });
                        for ((from, to), prob) in tm.iter().take(10) {
                            println!("  {:20} -> {:20} {:.3}", from, to, prob);
                        }
                        println!();
                    }

                    if patterns {
                        if !report.bigrams.is_empty() {
                            println!("Top bigrams:");
                            for b in report.bigrams.iter().take(5) {
                                println!("  {} x{}", b.sequence.join("->"), b.count);
                            }
                            println!();
                        }
                        if !report.trigrams.is_empty() {
                            println!("Top trigrams:");
                            for t in report.trigrams.iter().take(5) {
                                println!("  {} x{}", t.sequence.join("->"), t.count);
                            }
                            println!();
                        }
                    }
                }
            }
        }
        Commands::Nightly { repos, output, top, verbose } => {
            let repo_list = match repos {
                Some(ref r) => nightly::parse_repos(r),
                None => nightly::default_repos(),
            };
            if let Err(e) = nightly::run_nightly(repo_list, output.as_deref(), top, verbose) {
                eprintln!("Error: {}", e);
                std::process::exit(1);
            }
        }
        Commands::Init { dir, dry_run } => {
            let result = onboard::onboard(&dir);

            // Print summary
            print!("{}", result.summary);

            if dry_run {
                println!("--- .archlint.yaml (dry-run, not written) ---");
                print!("{}", result.config_yaml);
            } else {
                let config_path = dir.join(".archlint.yaml");
                if config_path.exists() {
                    eprintln!(
                        "[archlint] .archlint.yaml already exists. Use --dry-run to preview without overwriting."
                    );
                    eprintln!(
                        "[archlint] Delete or rename the existing file to re-run init."
                    );
                    std::process::exit(1);
                }
                match onboard::write_config(&dir, &result.config_yaml) {
                    Ok(path) => {
                        println!("Written: {}", path.display());
                    }
                    Err(e) => {
                        eprintln!("Error: {}", e);
                        std::process::exit(1);
                    }
                }
            }
        }
        Commands::Validate { graph, format } => {
            // Read YAML from file or stdin
            let yaml = if graph == "-" {
                use std::io::Read;
                let mut buf = String::new();
                std::io::stdin().read_to_string(&mut buf).expect("failed to read stdin");
                buf
            } else {
                std::fs::read_to_string(&graph).unwrap_or_else(|e| {
                    eprintln!("Error reading {}: {}", graph, e);
                    std::process::exit(1);
                })
            };

            // Load config from current directory (if .archlint.yaml exists)
            let config = config::Config::load(&std::path::PathBuf::from("."));

            match validate::validate_from_str(&yaml, &graph, &config) {
                Ok(report) => {
                    let has_taboo = report.violations.iter().any(|v| v.level == "taboo");

                    match format.as_str() {
                        "json" => {
                            let json = serde_json::to_string_pretty(&report).unwrap();
                            println!("{}", json);
                        }
                        "yaml" => {
                            let yaml_out = serde_yaml::to_string(&report).unwrap();
                            print!("{}", yaml_out);
                        }
                        _ => {
                            // text (default)
                            print!("{}", validate::format_text(&report));
                        }
                    }

                    if has_taboo {
                        std::process::exit(1);
                    }
                }
                Err(e) => {
                    eprintln!("Error: {}", e);
                    std::process::exit(2);
                }
            }
        }
        Commands::Diagram { dir, format } => {
            match analyzer::analyze(&dir) {
                Ok(graph) => {
                    let output = match format.as_str() {
                        "d2" => diagram::generate_d2(&graph),
                        _ => diagram::generate_mermaid(&graph),
                    };
                    print!("{}", output);
                }
                Err(e) => {
                    eprintln!("Error: {}", e);
                    std::process::exit(2);
                }
            }
        }
        Commands::Badge { dir, output } => {
            match analyzer::analyze(&dir) {
                Ok(graph) => {
                    let violation_count = graph
                        .metrics
                        .as_ref()
                        .map(|m| m.violations.len())
                        .unwrap_or(0);
                    let score = badge::score_from_violations(violation_count);
                    let svg = badge::generate_badge(score);

                    match output {
                        Some(ref path) if path.to_str() != Some("-") => {
                            if let Err(e) = std::fs::write(path, &svg) {
                                eprintln!("Error writing badge to {}: {}", path.display(), e);
                                std::process::exit(1);
                            }
                            eprintln!("Badge written to {} (score: {}/100, violations: {})", path.display(), score, violation_count);
                        }
                        _ => {
                            print!("{}", svg);
                        }
                    }
                }
                Err(e) => {
                    eprintln!("Error: {}", e);
                    std::process::exit(2);
                }
            }
        }
    }
}
