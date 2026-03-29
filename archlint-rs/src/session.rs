use crate::model::{ArchGraph, Component, Link};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;

// --- JSONL parsing types ---

/// A single message line in Claude Code JSONL session format.
#[derive(Debug, Deserialize)]
pub struct SessionMessage {
    pub role: Option<String>,
    pub content: Option<serde_json::Value>,
    // Claude Code wraps the message under a "message" key
    pub message: Option<Box<SessionMessage>>,
    // Top-level type field ("assistant" | "user" | ...)
    #[serde(rename = "type")]
    pub msg_type: Option<String>,
}

// --- Output types ---

/// A single tool call event extracted from a session.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ToolCall {
    pub index: usize,
    pub tool: String,
}

/// Frequency count for a tool.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct ToolFrequency {
    pub tool: String,
    pub count: usize,
}

/// A detected workflow pattern.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WorkflowPattern {
    pub name: String,
    pub sequence: Vec<String>,
    pub occurrences: usize,
    pub description: String,
}

/// A workflow flag (fan-out or repetition).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct WorkflowFlag {
    pub kind: String,
    pub tool: Option<String>,
    pub detail: String,
}

/// Full session analysis report.
#[derive(Debug, Serialize, Deserialize)]
pub struct SessionReport {
    pub total_tool_calls: usize,
    pub tool_frequencies: Vec<ToolFrequency>,
    pub bigrams: Vec<BigramFrequency>,
    pub trigrams: Vec<TrigramFrequency>,
    pub patterns: Vec<WorkflowPattern>,
    pub flags: Vec<WorkflowFlag>,
}

/// Bigram (two-tool sequence) with frequency.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct BigramFrequency {
    pub sequence: Vec<String>,
    pub count: usize,
}

/// Trigram (three-tool sequence) with frequency.
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct TrigramFrequency {
    pub sequence: Vec<String>,
    pub count: usize,
}

// --- Parsing ---

/// Extract tool_use names from a content array (serde_json::Value::Array).
fn extract_names_from_content(content: &serde_json::Value) -> Vec<String> {
    let mut names = Vec::new();
    match content {
        serde_json::Value::Array(items) => {
            for item in items {
                if item.get("type").and_then(|t| t.as_str()) == Some("tool_use") {
                    if let Some(name) = item.get("name").and_then(|n| n.as_str()) {
                        names.push(name.to_string());
                    }
                }
            }
        }
        serde_json::Value::String(_) => {
            // Plain string content carries no tool_use blocks.
        }
        _ => {}
    }
    names
}

/// Parse a single JSONL line and extract tool_use names.
///
/// Supports two formats:
/// 1. Claude Code format: `{"type":"assistant","message":{"role":"assistant","content":[...]},...}`
/// 2. Generic format:     `{"role":"assistant","content":[...]}`
fn extract_tool_uses(line: &str) -> Vec<String> {
    let msg: SessionMessage = match serde_json::from_str(line) {
        Ok(m) => m,
        Err(_) => return vec![],
    };

    // --- Claude Code format: top-level "type" + nested "message" ---
    if let Some(inner) = &msg.message {
        // Accept only assistant messages.
        let role = inner.role.as_deref().unwrap_or("");
        let top_type = msg.msg_type.as_deref().unwrap_or("");
        if role != "assistant" && top_type != "assistant" {
            return vec![];
        }
        if let Some(content) = &inner.content {
            return extract_names_from_content(content);
        }
        return vec![];
    }

    // --- Generic format: top-level "role" + "content" ---
    let role = msg.role.as_deref().unwrap_or("");
    let top_type = msg.msg_type.as_deref().unwrap_or("");
    if role != "assistant" && top_type != "assistant" {
        return vec![];
    }

    match &msg.content {
        Some(content) => extract_names_from_content(content),
        None => vec![],
    }
}

/// Parse a JSONL session and return the ordered list of tool calls.
pub fn parse_tool_chain(jsonl: &str) -> Vec<ToolCall> {
    let mut calls = Vec::new();
    let mut index = 0usize;
    for line in jsonl.lines() {
        let line = line.trim();
        if line.is_empty() {
            continue;
        }
        for tool_name in extract_tool_uses(line) {
            calls.push(ToolCall { index, tool: tool_name });
            index += 1;
        }
    }
    calls
}

// --- Frequency analysis ---

fn count_tools(calls: &[ToolCall]) -> Vec<ToolFrequency> {
    let mut counts: HashMap<&str, usize> = HashMap::new();
    for c in calls {
        *counts.entry(c.tool.as_str()).or_insert(0) += 1;
    }
    let mut freq: Vec<ToolFrequency> = counts
        .into_iter()
        .map(|(tool, count)| ToolFrequency { tool: tool.to_string(), count })
        .collect();
    freq.sort_by(|a, b| b.count.cmp(&a.count));
    freq
}

fn count_bigrams(calls: &[ToolCall]) -> Vec<BigramFrequency> {
    let mut counts: HashMap<(&str, &str), usize> = HashMap::new();
    for window in calls.windows(2) {
        let key = (window[0].tool.as_str(), window[1].tool.as_str());
        *counts.entry(key).or_insert(0) += 1;
    }
    let mut freq: Vec<BigramFrequency> = counts
        .into_iter()
        .map(|((a, b), count)| BigramFrequency {
            sequence: vec![a.to_string(), b.to_string()],
            count,
        })
        .collect();
    freq.sort_by(|a, b| b.count.cmp(&a.count));
    freq
}

fn count_trigrams(calls: &[ToolCall]) -> Vec<TrigramFrequency> {
    let mut counts: HashMap<(&str, &str, &str), usize> = HashMap::new();
    for window in calls.windows(3) {
        let key = (
            window[0].tool.as_str(),
            window[1].tool.as_str(),
            window[2].tool.as_str(),
        );
        *counts.entry(key).or_insert(0) += 1;
    }
    let mut freq: Vec<TrigramFrequency> = counts
        .into_iter()
        .map(|((a, b, c), count)| TrigramFrequency {
            sequence: vec![a.to_string(), b.to_string(), c.to_string()],
            count,
        })
        .collect();
    freq.sort_by(|a, b| b.count.cmp(&a.count));
    freq
}

// --- Pattern detection ---

/// Known workflow patterns (trigram-based).
const KNOWN_PATTERNS: &[(&str, &[&str], &str)] = &[
    (
        "refactoring",
        &["Read", "Grep", "Edit"],
        "read->grep->edit = refactoring cycle",
    ),
    (
        "lint_cycle",
        &["Bash", "Edit", "Bash"],
        "bash->edit->bash = lint/build cycle",
    ),
    (
        "search_edit",
        &["Glob", "Read", "Edit"],
        "glob->read->edit = find and modify",
    ),
    (
        "read_write",
        &["Read", "Edit", "Read"],
        "read->edit->read = verify edit",
    ),
    (
        "test_cycle",
        &["Bash", "Read", "Bash"],
        "bash->read->bash = test inspect test cycle",
    ),
];

fn detect_patterns(trigrams: &[TrigramFrequency]) -> Vec<WorkflowPattern> {
    let mut patterns = Vec::new();
    for (name, seq, desc) in KNOWN_PATTERNS {
        let seq_vec: Vec<String> = seq.iter().map(|s| s.to_string()).collect();
        let occurrences: usize = trigrams
            .iter()
            .filter(|t| t.sequence == seq_vec)
            .map(|t| t.count)
            .sum();
        if occurrences > 0 {
            patterns.push(WorkflowPattern {
                name: name.to_string(),
                sequence: seq_vec,
                occurrences,
                description: desc.to_string(),
            });
        }
    }
    patterns.sort_by(|a, b| b.occurrences.cmp(&a.occurrences));
    patterns
}

// --- Flag detection ---

fn detect_flags(calls: &[ToolCall], tool_freq: &[ToolFrequency]) -> Vec<WorkflowFlag> {
    let mut flags = Vec::new();

    // Fan-out: too many distinct tools (> 8 different tools in one session)
    let distinct_tools = tool_freq.len();
    if distinct_tools > 8 {
        flags.push(WorkflowFlag {
            kind: "fan_out".to_string(),
            tool: None,
            detail: format!("{} distinct tools used (threshold: 8)", distinct_tools),
        });
    }

    // Repetition: same tool called > 5 times = possible loop
    for f in tool_freq {
        if f.count > 5 {
            flags.push(WorkflowFlag {
                kind: "repetition".to_string(),
                tool: Some(f.tool.clone()),
                detail: format!("{} called {} times (threshold: 5)", f.tool, f.count),
            });
        }
    }

    // Consecutive same tool (immediate loop indicator)
    let mut consecutive_count = 1usize;
    for i in 1..calls.len() {
        if calls[i].tool == calls[i - 1].tool {
            consecutive_count += 1;
            if consecutive_count >= 3 {
                // Already flagged once for this tool, avoid duplicates
                let already = flags.iter().any(|fl| {
                    fl.kind == "consecutive_loop"
                        && fl.tool.as_deref() == Some(calls[i].tool.as_str())
                });
                if !already {
                    flags.push(WorkflowFlag {
                        kind: "consecutive_loop".to_string(),
                        tool: Some(calls[i].tool.clone()),
                        detail: format!(
                            "{} called {} consecutive times",
                            calls[i].tool, consecutive_count
                        ),
                    });
                }
            }
        } else {
            consecutive_count = 1;
        }
    }

    flags
}

// --- Analyze ---

/// Analyze the full session and return a report.
pub fn analyze(jsonl: &str) -> SessionReport {
    let calls = parse_tool_chain(jsonl);
    let tool_freq = count_tools(&calls);
    let bigrams = count_bigrams(&calls);
    let trigrams = count_trigrams(&calls);
    let patterns = detect_patterns(&trigrams);
    let flags = detect_flags(&calls, &tool_freq);

    SessionReport {
        total_tool_calls: calls.len(),
        tool_frequencies: tool_freq,
        bigrams,
        trigrams,
        patterns,
        flags,
    }
}

// --- ArchGraph export ---

/// Convert a session tool chain to ArchGraph format.
/// Components = unique tool types; links = sequential call edges.
pub fn to_arch_graph(jsonl: &str) -> ArchGraph {
    let calls = parse_tool_chain(jsonl);

    // Collect unique tool names as components.
    let mut tool_set: Vec<String> = {
        let mut seen = std::collections::HashSet::new();
        calls
            .iter()
            .filter(|c| seen.insert(c.tool.clone()))
            .map(|c| c.tool.clone())
            .collect()
    };
    tool_set.sort();

    let components: Vec<Component> = tool_set
        .iter()
        .map(|t| Component {
            id: t.clone(),
            title: t.clone(),
            entity: "tool".to_string(),
        })
        .collect();

    // Count edges between consecutive tool pairs.
    let mut edge_counts: HashMap<(String, String), usize> = HashMap::new();
    for window in calls.windows(2) {
        let key = (window[0].tool.clone(), window[1].tool.clone());
        *edge_counts.entry(key).or_insert(0) += 1;
    }

    let links: Vec<Link> = edge_counts
        .into_iter()
        .map(|((from, to), count)| Link {
            from,
            to,
            method: None,
            link_type: Some(format!("calls:{}", count)),
        })
        .collect();

    ArchGraph {
        components,
        links,
        metrics: None,
    }
}

// --- Tests ---

#[cfg(test)]
mod tests {
    use super::*;

    fn make_tool_use_line(tool: &str) -> String {
        format!(
            r#"{{"role":"assistant","content":[{{"type":"tool_use","name":"{}"}}]}}"#,
            tool
        )
    }

    /// Claude Code JSONL format: top-level "type" + nested "message".
    fn make_claude_code_line(tool: &str) -> String {
        format!(
            r#"{{"parentUuid":"abc","isSidechain":false,"type":"assistant","uuid":"xyz","timestamp":"2026-01-01T00:00:00Z","message":{{"model":"claude-sonnet-4-5","type":"message","role":"assistant","content":[{{"type":"tool_use","id":"tu_1","name":"{}","input":{{}}}}]}}}}"#,
            tool
        )
    }

    #[test]
    fn test_parse_empty() {
        let calls = parse_tool_chain("");
        assert!(calls.is_empty());
    }

    #[test]
    fn test_parse_single_tool() {
        let line = make_tool_use_line("Read");
        let calls = parse_tool_chain(&line);
        assert_eq!(calls.len(), 1);
        assert_eq!(calls[0].tool, "Read");
    }

    #[test]
    fn test_parse_multiple_tools_in_one_message() {
        let line = r#"{"role":"assistant","content":[{"type":"tool_use","name":"Glob"},{"type":"tool_use","name":"Read"}]}"#;
        let calls = parse_tool_chain(line);
        assert_eq!(calls.len(), 2);
        assert_eq!(calls[0].tool, "Glob");
        assert_eq!(calls[1].tool, "Read");
    }

    #[test]
    fn test_user_messages_ignored() {
        let user_line = r#"{"role":"user","content":[{"type":"tool_result","content":"ok"}]}"#;
        let calls = parse_tool_chain(user_line);
        assert!(calls.is_empty());
    }

    #[test]
    fn test_invalid_json_ignored() {
        let calls = parse_tool_chain("not json");
        assert!(calls.is_empty());
    }

    #[test]
    fn test_refactoring_pattern_detected() {
        let session = [
            make_tool_use_line("Read"),
            make_tool_use_line("Grep"),
            make_tool_use_line("Edit"),
        ]
        .join("\n");
        let report = analyze(&session);
        assert_eq!(report.total_tool_calls, 3);
        let found = report.patterns.iter().any(|p| p.name == "refactoring");
        assert!(found, "refactoring pattern should be detected");
    }

    #[test]
    fn test_lint_cycle_pattern() {
        let session = [
            make_tool_use_line("Bash"),
            make_tool_use_line("Edit"),
            make_tool_use_line("Bash"),
        ]
        .join("\n");
        let report = analyze(&session);
        let found = report.patterns.iter().any(|p| p.name == "lint_cycle");
        assert!(found, "lint_cycle pattern should be detected");
    }

    #[test]
    fn test_repetition_flag() {
        let lines: Vec<String> = (0..7).map(|_| make_tool_use_line("Bash")).collect();
        let session = lines.join("\n");
        let report = analyze(&session);
        let flagged = report.flags.iter().any(|f| f.kind == "repetition" && f.tool.as_deref() == Some("Bash"));
        assert!(flagged, "repetition flag should be set for Bash called 7 times");
    }

    #[test]
    fn test_fan_out_flag() {
        let tools = ["Read", "Grep", "Edit", "Bash", "Glob", "Write", "LS", "Cat", "Find"];
        let lines: Vec<String> = tools.iter().map(|t| make_tool_use_line(t)).collect();
        let session = lines.join("\n");
        let report = analyze(&session);
        let flagged = report.flags.iter().any(|f| f.kind == "fan_out");
        assert!(flagged, "fan_out flag should be set for 9 distinct tools");
    }

    #[test]
    fn test_to_arch_graph() {
        let session = [
            make_tool_use_line("Read"),
            make_tool_use_line("Edit"),
            make_tool_use_line("Read"),
        ]
        .join("\n");
        let graph = to_arch_graph(&session);
        assert_eq!(graph.components.len(), 2); // Read, Edit
        // Edges: Read->Edit, Edit->Read
        assert_eq!(graph.links.len(), 2);
    }

    #[test]
    fn test_bigrams_counted() {
        let session = [
            make_tool_use_line("Read"),
            make_tool_use_line("Edit"),
            make_tool_use_line("Read"),
            make_tool_use_line("Edit"),
        ]
        .join("\n");
        let report = analyze(&session);
        let re_bigram = report
            .bigrams
            .iter()
            .find(|b| b.sequence == vec!["Read", "Edit"]);
        assert!(re_bigram.is_some());
        assert_eq!(re_bigram.unwrap().count, 2);
    }

    #[test]
    fn test_consecutive_loop_flag() {
        let lines: Vec<String> = (0..3).map(|_| make_tool_use_line("Bash")).collect();
        let session = lines.join("\n");
        let report = analyze(&session);
        let flagged = report.flags.iter().any(|f| f.kind == "consecutive_loop");
        assert!(flagged, "consecutive_loop flag should be set");
    }

    // --- Claude Code format tests ---

    #[test]
    fn test_claude_code_format_single_tool() {
        let line = make_claude_code_line("Read");
        let calls = parse_tool_chain(&line);
        assert_eq!(calls.len(), 1);
        assert_eq!(calls[0].tool, "Read");
    }

    #[test]
    fn test_claude_code_format_multiple_lines() {
        let session = [
            make_claude_code_line("Read"),
            make_claude_code_line("Edit"),
            make_claude_code_line("Bash"),
        ]
        .join("\n");
        let calls = parse_tool_chain(&session);
        assert_eq!(calls.len(), 3);
        assert_eq!(calls[0].tool, "Read");
        assert_eq!(calls[1].tool, "Edit");
        assert_eq!(calls[2].tool, "Bash");
    }

    #[test]
    fn test_claude_code_format_non_assistant_ignored() {
        // A user message in Claude Code format should be ignored.
        let user_line = r#"{"type":"user","uuid":"x","message":{"role":"user","content":[{"type":"tool_result","content":"ok"}]}}"#;
        let calls = parse_tool_chain(user_line);
        assert!(calls.is_empty());
    }

    #[test]
    fn test_mixed_formats_parsed_together() {
        // Generic format line followed by Claude Code format line.
        let generic = make_tool_use_line("Glob");
        let cc = make_claude_code_line("Grep");
        let session = format!("{}\n{}", generic, cc);
        let calls = parse_tool_chain(&session);
        assert_eq!(calls.len(), 2);
        assert_eq!(calls[0].tool, "Glob");
        assert_eq!(calls[1].tool, "Grep");
    }

    #[test]
    fn test_claude_code_format_pattern_detected() {
        let session = [
            make_claude_code_line("Read"),
            make_claude_code_line("Grep"),
            make_claude_code_line("Edit"),
        ]
        .join("\n");
        let report = analyze(&session);
        assert_eq!(report.total_tool_calls, 3);
        let found = report.patterns.iter().any(|p| p.name == "refactoring");
        assert!(found, "refactoring pattern should be detected in Claude Code format");
    }
}
