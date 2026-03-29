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

/// A detected session phase (segment of tool chain with coherent behavior).
#[derive(Debug, Clone, Serialize, Deserialize)]
pub struct SessionPhase {
    pub start_idx: usize,
    pub end_idx: usize,
    pub dominant_tool: String,
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
    /// #82 Transition matrix: P(B|A) for each tool pair (row-normalized).
    pub transition_matrix: HashMap<(String, String), f64>,
    /// #83 Shannon entropy over tool frequency distribution. Higher = more diverse.
    pub entropy: f64,
    /// #84 Conditional entropy H(Y|X). Lower = more predictable next tool.
    pub conditional_entropy: f64,
    /// #85 PageRank scores per tool node.
    pub pagerank: HashMap<String, f64>,
    /// #87 Betweenness centrality approximation: (tool, centrality_score).
    pub bottlenecks: Vec<(String, f64)>,
    /// #88 Session phases: detected change-point segments.
    pub phases: Vec<SessionPhase>,
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

// --- #82 Transition matrix ---

/// Build row-normalized transition matrix P(B|A) for each consecutive tool pair.
fn compute_transition_matrix(calls: &[ToolCall]) -> HashMap<(String, String), f64> {
    // Count raw co-occurrences.
    let mut raw: HashMap<(String, String), usize> = HashMap::new();
    let mut row_sums: HashMap<String, usize> = HashMap::new();
    for window in calls.windows(2) {
        let from = window[0].tool.clone();
        let to = window[1].tool.clone();
        *raw.entry((from.clone(), to)).or_insert(0) += 1;
        *row_sums.entry(from).or_insert(0) += 1;
    }
    // Normalize each row.
    let mut matrix = HashMap::new();
    for ((from, to), count) in raw {
        let total = *row_sums.get(&from).unwrap_or(&1) as f64;
        matrix.insert((from, to), count as f64 / total);
    }
    matrix
}

// --- #83 Shannon entropy ---

/// Compute Shannon entropy over tool frequency distribution.
/// H = -sum(p * log2(p))
fn compute_entropy(tool_freq: &[ToolFrequency]) -> f64 {
    let total: usize = tool_freq.iter().map(|f| f.count).sum();
    if total == 0 {
        return 0.0;
    }
    let total_f = total as f64;
    tool_freq.iter().fold(0.0, |acc, f| {
        if f.count == 0 {
            acc
        } else {
            let p = f.count as f64 / total_f;
            acc - p * p.log2()
        }
    })
}

// --- #84 Conditional entropy ---

/// Compute H(Y|X) = -sum over pairs p(x,y) * log2(p(y|x))
fn compute_conditional_entropy(calls: &[ToolCall]) -> f64 {
    if calls.len() < 2 {
        return 0.0;
    }
    // Count joint occurrences p(x,y) and marginal p(x).
    let mut joint: HashMap<(String, String), usize> = HashMap::new();
    let mut marginal_x: HashMap<String, usize> = HashMap::new();
    let total_pairs = (calls.len() - 1) as f64;

    for window in calls.windows(2) {
        let x = window[0].tool.clone();
        let y = window[1].tool.clone();
        *joint.entry((x.clone(), y)).or_insert(0) += 1;
        *marginal_x.entry(x).or_insert(0) += 1;
    }

    let mut h = 0.0;
    for ((x, _y), joint_count) in &joint {
        let pxy = *joint_count as f64 / total_pairs;
        let px = *marginal_x.get(x).unwrap_or(&1) as f64 / total_pairs;
        let pyx = *joint_count as f64 / *marginal_x.get(x).unwrap_or(&1) as f64;
        if pyx > 0.0 && pxy > 0.0 {
            h -= pxy * pyx.log2() * (px / px); // simplifies to -pxy * log2(p(y|x))
        }
    }
    h
}

// --- #85 PageRank ---

/// Compute PageRank on the tool transition graph.
/// damping = 0.85, 100 iterations.
fn compute_pagerank(calls: &[ToolCall]) -> HashMap<String, f64> {
    let damping = 0.85_f64;
    let iterations = 100;

    // Collect unique nodes.
    let mut nodes: Vec<String> = {
        let mut seen = std::collections::HashSet::new();
        calls.iter().filter(|c| seen.insert(c.tool.clone())).map(|c| c.tool.clone()).collect()
    };
    nodes.sort();

    let n = nodes.len();
    if n == 0 {
        return HashMap::new();
    }

    // Build adjacency: out_edges[i] = list of target node indices.
    let node_idx: HashMap<&str, usize> =
        nodes.iter().enumerate().map(|(i, name)| (name.as_str(), i)).collect();

    // Count raw edges.
    let mut out_weights: Vec<HashMap<usize, f64>> = vec![HashMap::new(); n];
    let mut out_total: Vec<f64> = vec![0.0; n];
    for window in calls.windows(2) {
        let from = *node_idx.get(window[0].tool.as_str()).unwrap();
        let to = *node_idx.get(window[1].tool.as_str()).unwrap();
        *out_weights[from].entry(to).or_insert(0.0) += 1.0;
        out_total[from] += 1.0;
    }
    // Normalize out-edges.
    for (i, weights) in out_weights.iter_mut().enumerate() {
        if out_total[i] > 0.0 {
            for v in weights.values_mut() {
                *v /= out_total[i];
            }
        }
    }

    // Initialize scores.
    let mut scores = vec![1.0 / n as f64; n];

    for _ in 0..iterations {
        let mut new_scores = vec![(1.0 - damping) / n as f64; n];
        for (from, weights) in out_weights.iter().enumerate() {
            for (to, weight) in weights {
                new_scores[*to] += damping * scores[from] * weight;
            }
        }
        // Dangling nodes: redistribute uniformly.
        let dangling_sum: f64 = nodes
            .iter()
            .enumerate()
            .filter(|(i, _)| out_total[*i] == 0.0)
            .map(|(i, _)| scores[i])
            .sum();
        if dangling_sum > 0.0 {
            for s in &mut new_scores {
                *s += damping * dangling_sum / n as f64;
            }
        }
        scores = new_scores;
    }

    nodes.into_iter().enumerate().map(|(i, name)| (name, scores[i])).collect()
}

// --- #87 Bottleneck: betweenness centrality approximation ---

/// Approximate betweenness centrality using BFS shortest paths.
/// Returns sorted list of (tool, centrality) descending.
fn compute_bottlenecks(calls: &[ToolCall]) -> Vec<(String, f64)> {
    // Collect unique nodes.
    let mut nodes: Vec<String> = {
        let mut seen = std::collections::HashSet::new();
        calls.iter().filter(|c| seen.insert(c.tool.clone())).map(|c| c.tool.clone()).collect()
    };
    nodes.sort();
    let n = nodes.len();
    if n <= 2 {
        return nodes.into_iter().map(|name| (name, 0.0)).collect();
    }

    let node_idx: HashMap<&str, usize> =
        nodes.iter().enumerate().map(|(i, name)| (name.as_str(), i)).collect();

    // Build undirected adjacency list (unweighted, for shortest path counting).
    let mut adj: Vec<std::collections::HashSet<usize>> = vec![std::collections::HashSet::new(); n];
    for window in calls.windows(2) {
        let from = *node_idx.get(window[0].tool.as_str()).unwrap();
        let to = *node_idx.get(window[1].tool.as_str()).unwrap();
        adj[from].insert(to);
        adj[to].insert(from);
    }

    let mut betweenness = vec![0.0_f64; n];

    // BFS from each source.
    for s in 0..n {
        let mut stack: Vec<usize> = Vec::new();
        let mut pred: Vec<Vec<usize>> = vec![Vec::new(); n];
        let mut sigma = vec![0.0_f64; n];
        let mut dist = vec![-1i64; n];

        sigma[s] = 1.0;
        dist[s] = 0;

        let mut queue = std::collections::VecDeque::new();
        queue.push_back(s);

        while let Some(v) = queue.pop_front() {
            stack.push(v);
            for &w in &adj[v] {
                if dist[w] < 0 {
                    queue.push_back(w);
                    dist[w] = dist[v] + 1;
                }
                if dist[w] == dist[v] + 1 {
                    sigma[w] += sigma[v];
                    pred[w].push(v);
                }
            }
        }

        let mut delta = vec![0.0_f64; n];
        while let Some(w) = stack.pop() {
            for &v in &pred[w] {
                if sigma[w] > 0.0 {
                    delta[v] += (sigma[v] / sigma[w]) * (1.0 + delta[w]);
                }
            }
            if w != s {
                betweenness[w] += delta[w];
            }
        }
    }

    // Normalize (undirected: divide by 2, then by (n-1)(n-2)/2 for normalization).
    let norm = if n > 2 { ((n - 1) * (n - 2)) as f64 } else { 1.0 };
    let mut result: Vec<(String, f64)> = nodes
        .into_iter()
        .enumerate()
        .map(|(i, name)| (name, betweenness[i] / norm))
        .collect();
    result.sort_by(|a, b| b.1.partial_cmp(&a.1).unwrap_or(std::cmp::Ordering::Equal));
    result
}

// --- #88 Session segmentation ---

/// Split tool chain into phases using sliding-window entropy comparison.
/// A change point occurs when the local tool distribution shifts significantly.
fn compute_phases(calls: &[ToolCall]) -> Vec<SessionPhase> {
    let n = calls.len();
    // Need at least a few calls to make segmentation meaningful.
    let window_size = 5;
    if n < window_size * 2 {
        // Not enough data - return a single phase.
        if n == 0 {
            return vec![];
        }
        let dominant = calls
            .iter()
            .fold(HashMap::<&str, usize>::new(), |mut m, c| {
                *m.entry(c.tool.as_str()).or_insert(0) += 1;
                m
            })
            .into_iter()
            .max_by_key(|(_, v)| *v)
            .map(|(k, _)| k.to_string())
            .unwrap_or_default();
        return vec![SessionPhase { start_idx: 0, end_idx: n - 1, dominant_tool: dominant }];
    }

    // Compute local entropy for a window of tool names.
    let window_entropy = |slice: &[ToolCall]| -> f64 {
        let mut counts: HashMap<&str, usize> = HashMap::new();
        for c in slice {
            *counts.entry(c.tool.as_str()).or_insert(0) += 1;
        }
        let total = slice.len() as f64;
        counts.values().fold(0.0, |acc, &cnt| {
            let p = cnt as f64 / total;
            acc - p * p.log2()
        })
    };

    // Dominant tool in a window.
    let dominant_in = |slice: &[ToolCall]| -> String {
        let mut counts: HashMap<&str, usize> = HashMap::new();
        for c in slice {
            *counts.entry(c.tool.as_str()).or_insert(0) += 1;
        }
        counts
            .into_iter()
            .max_by_key(|(_, v)| *v)
            .map(|(k, _)| k.to_string())
            .unwrap_or_default()
    };

    // Detect change points: positions where entropy difference between adjacent windows is high.
    let threshold = 0.5;
    let mut change_points: Vec<usize> = vec![0];

    for i in window_size..=(n - window_size) {
        let left = &calls[i - window_size..i];
        let right = &calls[i..i + window_size];
        let diff = (window_entropy(left) - window_entropy(right)).abs();
        // Also check if the dominant tool changes.
        let left_dom = dominant_in(left);
        let right_dom = dominant_in(right);
        if diff > threshold || left_dom != right_dom {
            let last = *change_points.last().unwrap();
            if i - last >= window_size {
                change_points.push(i);
            }
        }
    }
    change_points.push(n);

    // Build phases from change points.
    let mut phases = Vec::new();
    for pair in change_points.windows(2) {
        let start = pair[0];
        let end = pair[1] - 1;
        if start <= end {
            let dominant = dominant_in(&calls[start..=end]);
            phases.push(SessionPhase { start_idx: start, end_idx: end, dominant_tool: dominant });
        }
    }
    phases
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

    let transition_matrix = compute_transition_matrix(&calls);
    let entropy = compute_entropy(&tool_freq);
    let conditional_entropy = compute_conditional_entropy(&calls);
    let pagerank = compute_pagerank(&calls);
    let bottlenecks = compute_bottlenecks(&calls);
    let phases = compute_phases(&calls);

    SessionReport {
        total_tool_calls: calls.len(),
        tool_frequencies: tool_freq,
        bigrams,
        trigrams,
        patterns,
        flags,
        transition_matrix,
        entropy,
        conditional_entropy,
        pagerank,
        bottlenecks,
        phases,
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

    // --- #82 Transition matrix tests ---

    #[test]
    fn test_transition_matrix_probabilities_sum_to_one() {
        // Read always followed by Edit, then Bash. Each row must sum to 1.0.
        let session = [
            make_tool_use_line("Read"),
            make_tool_use_line("Edit"),
            make_tool_use_line("Read"),
            make_tool_use_line("Edit"),
            make_tool_use_line("Read"),
            make_tool_use_line("Bash"),
        ]
        .join("\n");
        let calls = parse_tool_chain(&session);
        let matrix = compute_transition_matrix(&calls);

        // Sum probabilities for rows starting with "Read".
        let read_sum: f64 = matrix
            .iter()
            .filter(|((from, _), _)| from == "Read")
            .map(|(_, p)| p)
            .sum();
        assert!((read_sum - 1.0).abs() < 1e-9, "Read row should sum to 1.0, got {}", read_sum);
    }

    #[test]
    fn test_transition_matrix_probability_values() {
        // Read -> Edit (2 times), Read -> Bash (1 time). P(Edit|Read) = 2/3.
        let session = [
            make_tool_use_line("Read"),
            make_tool_use_line("Edit"),
            make_tool_use_line("Read"),
            make_tool_use_line("Edit"),
            make_tool_use_line("Read"),
            make_tool_use_line("Bash"),
        ]
        .join("\n");
        let calls = parse_tool_chain(&session);
        let matrix = compute_transition_matrix(&calls);
        let p_edit_given_read = matrix
            .get(&("Read".to_string(), "Edit".to_string()))
            .copied()
            .unwrap_or(0.0);
        assert!((p_edit_given_read - 2.0 / 3.0).abs() < 1e-9, "P(Edit|Read) should be 2/3");
    }

    #[test]
    fn test_transition_matrix_empty_calls() {
        let matrix = compute_transition_matrix(&[]);
        assert!(matrix.is_empty());
    }

    // --- #83 Shannon entropy tests ---

    #[test]
    fn test_entropy_uniform_distribution_is_max() {
        // Two tools used equally -> H = 1.0 (max for 2 symbols in bits).
        let freq = vec![
            ToolFrequency { tool: "A".to_string(), count: 5 },
            ToolFrequency { tool: "B".to_string(), count: 5 },
        ];
        let h = compute_entropy(&freq);
        assert!((h - 1.0).abs() < 1e-9, "Uniform 2-symbol entropy should be 1.0, got {}", h);
    }

    #[test]
    fn test_entropy_single_tool_is_zero() {
        let freq = vec![ToolFrequency { tool: "Read".to_string(), count: 10 }];
        let h = compute_entropy(&freq);
        assert!((h - 0.0).abs() < 1e-9, "Single-tool entropy should be 0.0");
    }

    #[test]
    fn test_entropy_empty_is_zero() {
        let h = compute_entropy(&[]);
        assert_eq!(h, 0.0);
    }

    #[test]
    fn test_entropy_more_tools_higher_entropy() {
        let freq2 = vec![
            ToolFrequency { tool: "A".to_string(), count: 5 },
            ToolFrequency { tool: "B".to_string(), count: 5 },
        ];
        let freq4 = vec![
            ToolFrequency { tool: "A".to_string(), count: 5 },
            ToolFrequency { tool: "B".to_string(), count: 5 },
            ToolFrequency { tool: "C".to_string(), count: 5 },
            ToolFrequency { tool: "D".to_string(), count: 5 },
        ];
        assert!(compute_entropy(&freq4) > compute_entropy(&freq2));
    }

    // --- #84 Conditional entropy tests ---

    #[test]
    fn test_conditional_entropy_deterministic_chain_is_zero() {
        // A always followed by B: H(Y|X) = 0.
        let calls = vec![
            ToolCall { index: 0, tool: "A".to_string() },
            ToolCall { index: 1, tool: "B".to_string() },
            ToolCall { index: 2, tool: "A".to_string() },
            ToolCall { index: 3, tool: "B".to_string() },
        ];
        let h = compute_conditional_entropy(&calls);
        assert!(h.abs() < 1e-9, "Deterministic chain should have H(Y|X)=0, got {}", h);
    }

    #[test]
    fn test_conditional_entropy_empty_is_zero() {
        let h = compute_conditional_entropy(&[]);
        assert_eq!(h, 0.0);
    }

    #[test]
    fn test_conditional_entropy_non_negative() {
        let session = [
            make_tool_use_line("Read"),
            make_tool_use_line("Edit"),
            make_tool_use_line("Bash"),
            make_tool_use_line("Read"),
            make_tool_use_line("Bash"),
        ]
        .join("\n");
        let calls = parse_tool_chain(&session);
        let h = compute_conditional_entropy(&calls);
        assert!(h >= 0.0, "Conditional entropy must be non-negative");
    }

    // --- #85 PageRank tests ---

    #[test]
    fn test_pagerank_scores_sum_to_one() {
        let session = [
            make_tool_use_line("Read"),
            make_tool_use_line("Edit"),
            make_tool_use_line("Bash"),
            make_tool_use_line("Read"),
        ]
        .join("\n");
        let calls = parse_tool_chain(&session);
        let pr = compute_pagerank(&calls);
        let sum: f64 = pr.values().sum();
        assert!((sum - 1.0).abs() < 1e-3, "PageRank scores should sum to ~1.0, got {}", sum);
    }

    #[test]
    fn test_pagerank_central_node_has_higher_score() {
        // Read is reached from both Edit and Bash; it should rank highest.
        let session = [
            make_tool_use_line("Edit"),
            make_tool_use_line("Read"),
            make_tool_use_line("Bash"),
            make_tool_use_line("Read"),
            make_tool_use_line("Edit"),
            make_tool_use_line("Read"),
        ]
        .join("\n");
        let calls = parse_tool_chain(&session);
        let pr = compute_pagerank(&calls);
        let read_score = pr.get("Read").copied().unwrap_or(0.0);
        let bash_score = pr.get("Bash").copied().unwrap_or(0.0);
        assert!(read_score > bash_score, "Read (hub) should rank higher than Bash");
    }

    #[test]
    fn test_pagerank_empty_is_empty() {
        let pr = compute_pagerank(&[]);
        assert!(pr.is_empty());
    }

    // --- #87 Bottleneck tests ---

    #[test]
    fn test_bottlenecks_bridge_node_has_highest_centrality() {
        // Linear chain: A -> B -> C. B is the only bridge.
        let calls = vec![
            ToolCall { index: 0, tool: "A".to_string() },
            ToolCall { index: 1, tool: "B".to_string() },
            ToolCall { index: 2, tool: "C".to_string() },
        ];
        let bt = compute_bottlenecks(&calls);
        let b_score = bt.iter().find(|(t, _)| t == "B").map(|(_, s)| *s).unwrap_or(0.0);
        let a_score = bt.iter().find(|(t, _)| t == "A").map(|(_, s)| *s).unwrap_or(0.0);
        assert!(b_score >= a_score, "B (bridge) should have centrality >= A endpoints");
    }

    #[test]
    fn test_bottlenecks_non_negative() {
        let session = [
            make_tool_use_line("Read"),
            make_tool_use_line("Edit"),
            make_tool_use_line("Bash"),
        ]
        .join("\n");
        let calls = parse_tool_chain(&session);
        let bt = compute_bottlenecks(&calls);
        for (_, score) in &bt {
            assert!(*score >= 0.0, "Betweenness centrality must be non-negative");
        }
    }

    #[test]
    fn test_bottlenecks_sorted_descending() {
        let session = [
            make_tool_use_line("A"),
            make_tool_use_line("B"),
            make_tool_use_line("C"),
            make_tool_use_line("B"),
            make_tool_use_line("D"),
        ]
        .join("\n");
        let calls = parse_tool_chain(&session);
        let bt = compute_bottlenecks(&calls);
        for i in 1..bt.len() {
            assert!(bt[i - 1].1 >= bt[i].1, "Bottlenecks should be sorted descending");
        }
    }

    // --- #88 Session phases tests ---

    #[test]
    fn test_phases_single_tool_is_one_phase() {
        let lines: Vec<String> = (0..6).map(|_| make_tool_use_line("Read")).collect();
        let session = lines.join("\n");
        let calls = parse_tool_chain(&session);
        let phases = compute_phases(&calls);
        assert!(!phases.is_empty(), "Should have at least one phase");
        assert_eq!(phases[0].dominant_tool, "Read");
    }

    #[test]
    fn test_phases_cover_all_calls() {
        let session = [
            make_tool_use_line("Read"),
            make_tool_use_line("Read"),
            make_tool_use_line("Read"),
            make_tool_use_line("Read"),
            make_tool_use_line("Read"),
            make_tool_use_line("Bash"),
            make_tool_use_line("Bash"),
            make_tool_use_line("Bash"),
            make_tool_use_line("Bash"),
            make_tool_use_line("Bash"),
        ]
        .join("\n");
        let calls = parse_tool_chain(&session);
        let n = calls.len();
        let phases = compute_phases(&calls);
        assert!(!phases.is_empty());
        // First phase starts at 0.
        assert_eq!(phases[0].start_idx, 0);
        // Last phase ends at n-1.
        assert_eq!(phases.last().unwrap().end_idx, n - 1);
        // Phases are contiguous.
        for i in 1..phases.len() {
            assert_eq!(
                phases[i].start_idx,
                phases[i - 1].end_idx + 1,
                "Phases must be contiguous"
            );
        }
    }

    #[test]
    fn test_phases_empty_calls() {
        let phases = compute_phases(&[]);
        assert!(phases.is_empty());
    }

    #[test]
    fn test_analyze_includes_all_metrics() {
        let session = [
            make_tool_use_line("Read"),
            make_tool_use_line("Edit"),
            make_tool_use_line("Bash"),
            make_tool_use_line("Read"),
        ]
        .join("\n");
        let report = analyze(&session);
        assert!(!report.transition_matrix.is_empty(), "transition_matrix should be populated");
        assert!(report.entropy > 0.0, "entropy should be positive for diverse tool use");
        assert!(report.conditional_entropy >= 0.0, "conditional_entropy must be non-negative");
        assert!(!report.pagerank.is_empty(), "pagerank should be populated");
        assert!(!report.bottlenecks.is_empty(), "bottlenecks should be populated");
        assert!(!report.phases.is_empty(), "phases should be populated");
    }
}
