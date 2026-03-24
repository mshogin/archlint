mod analyzer;
mod model;
mod promptlint;

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

        /// Output format: json, yaml, brief
        #[arg(long, default_value = "json")]
        format: String,

        /// Maximum violations before exit code 1
        #[arg(long)]
        threshold: Option<usize>,
    },
    /// Analyze prompt complexity and suggest model routing
    Prompt {
        /// Output only model name
        #[arg(long)]
        model_only: bool,

        /// Output format: json, brief
        #[arg(long, default_value = "json")]
        format: String,
    },
}

fn main() {
    let cli = Cli::parse();

    match cli.command {
        Commands::Prompt { model_only, format } => {
            use std::io::Read;
            let mut input = String::new();
            std::io::stdin().read_to_string(&mut input).expect("failed to read stdin");
            let result = promptlint::analyze(&input);
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
        Commands::Scan {
            dir,
            format,
            threshold,
        } => {
            match analyzer::analyze(&dir) {
                Ok(graph) => {
                    match format.as_str() {
                        "json" => {
                            let json = serde_json::to_string_pretty(&graph).unwrap();
                            println!("{}", json);
                        }
                        "yaml" => {
                            let yaml = serde_yaml::to_string(&graph).unwrap();
                            println!("{}", yaml);
                        }
                        "brief" => {
                            if let Some(ref metrics) = graph.metrics {
                                println!(
                                    "components={} links={} cycles={} violations={} max_fan_out={}",
                                    metrics.component_count,
                                    metrics.link_count,
                                    metrics.cycles.len(),
                                    metrics.violations.len(),
                                    metrics.max_fan_out,
                                );
                            }
                        }
                        _ => {
                            eprintln!("Unknown format: {}", format);
                            std::process::exit(1);
                        }
                    }

                    // Exit code based on threshold
                    if let Some(max_violations) = threshold {
                        if let Some(ref metrics) = graph.metrics {
                            if metrics.violations.len() > max_violations {
                                std::process::exit(1);
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
    }
}
