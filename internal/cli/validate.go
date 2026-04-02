package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/mshogin/archlint/internal/archlintcfg"
	"github.com/mshogin/archlint/internal/mcp"
	"github.com/mshogin/archlint/internal/model"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	validateGraphFile  string
	validateFormat     string
	validateConfigFile string
	validateGroup      string
	validatePython     bool
)

// GraphExport is the standard YAML graph format produced by archlint-rs scan --format yaml.
// It is part of the Unix-pipe multi-language architecture pipeline.
// Field names match archlint-rs ArchGraph: components/links (not nodes/edges).
type GraphExport struct {
	Components []GraphExportNode   `yaml:"components" json:"components"`
	Links      []GraphExportEdge   `yaml:"links"      json:"links"`
	Metadata   GraphExportMetadata `yaml:"metadata"   json:"metadata"`
	Metrics    *GraphExportMetrics `yaml:"metrics,omitempty" json:"metrics,omitempty"`
}

// GraphExportNode represents a component in the exported graph.
// Fields match archlint-rs Component: id, title, entity.
type GraphExportNode struct {
	ID     string `yaml:"id"     json:"id"`
	Title  string `yaml:"title"  json:"title"`
	Entity string `yaml:"entity" json:"entity"`
}

// GraphExportEdge represents a dependency link in the exported graph.
// Fields match archlint-rs Link: from, to, link_type.
type GraphExportEdge struct {
	From     string `yaml:"from"      json:"from"`
	To       string `yaml:"to"        json:"to"`
	LinkType string `yaml:"link_type" json:"link_type"`
}

// GraphExportMetadata contains information about the graph export.
type GraphExportMetadata struct {
	Language   string `yaml:"language"    json:"language"`
	RootDir    string `yaml:"root_dir"    json:"root_dir"`
	AnalyzedAt string `yaml:"analyzed_at" json:"analyzed_at"`
}

// GraphExportMetrics contains architecture metrics from the graph export.
type GraphExportMetrics struct {
	ComponentCount int                    `yaml:"component_count" json:"component_count"`
	LinkCount      int                    `yaml:"link_count"      json:"link_count"`
	MaxFanOut      int                    `yaml:"max_fan_out"     json:"max_fan_out"`
	MaxFanIn       int                    `yaml:"max_fan_in"      json:"max_fan_in"`
	Cycles         [][]string             `yaml:"cycles"          json:"cycles"`
	Violations     []GraphExportViolation `yaml:"violations"      json:"violations"`
}

// GraphExportViolation is a violation entry in the exported graph metrics.
type GraphExportViolation struct {
	Rule      string `yaml:"rule"      json:"rule"`
	Component string `yaml:"component" json:"component"`
	Message   string `yaml:"message"   json:"message"`
	Severity  string `yaml:"severity"  json:"severity"`
}

var validateCmd = &cobra.Command{
	Use:   "validate [directory|architecture.yaml]",
	Short: "Validate architecture: collect graph and run 229 metrics via Python validator",
	Long: `Validate architecture graph using the Python validator with 229 metrics.

If input is a directory, first runs archlint collect to generate architecture.yaml,
then runs the Python validator on it.

If input is a .yaml file, runs the Python validator directly on it.

Without --python flag, runs the built-in Go rule engine only.

Validator groups (--group):
  core         - DAG, cycles, fan-out, coupling, hub nodes
  solid        - SOLID principles (SRP, OCP, LSP, ISP, DIP)
  patterns     - Design smells (god class, shotgun surgery, ...)
  architecture - Clean architecture, domain isolation, ports & adapters
  quality      - Security, observability, testability
  advanced     - Graph centrality, pagerank, modularity (opt-in)
  research     - 142 math metrics: topology, spectral, information theory (opt-in)

Examples:
  archlint validate .
  archlint validate architecture.yaml
  archlint validate . --python --group solid
  archlint validate architecture.yaml --python --group research
  archlint validate . --python --format json
  archlint-rs collect . && archlint validate architecture.yaml --python`,
	Args: cobra.MaximumNArgs(1),
	RunE: runValidate,
}

func init() {
	validateCmd.Flags().StringVar(&validateGraphFile, "graph", "", "Path to YAML graph file (use - for stdin); legacy flag, prefer positional arg")
	validateCmd.Flags().StringVar(&validateFormat, "format", "text", "Output format: text, json, or yaml")
	validateCmd.Flags().StringVar(&validateConfigFile, "config", "", "Path to .archlint.yaml config file (default: ./.archlint.yaml)")
	validateCmd.Flags().StringVar(&validateGroup, "group", "", "Validator group: core, solid, patterns, architecture, quality, advanced, research")
	validateCmd.Flags().BoolVar(&validatePython, "python", false, "Run Python validator (229 metrics) instead of built-in Go engine")
	rootCmd.AddCommand(validateCmd)
}

func runValidate(cmd *cobra.Command, args []string) error {
	// Resolve input: positional arg, --graph flag, or current dir
	input := "."
	if len(args) > 0 {
		input = args[0]
	} else if validateGraphFile != "" {
		input = validateGraphFile
	}

	// Determine if input is a directory (needs collect first) or a yaml file
	isDir := false
	if input != "-" {
		info, err := os.Stat(input)
		if err == nil && info.IsDir() {
			isDir = true
		}
	}

	// If directory: collect architecture.yaml first
	archFile := input
	if isDir {
		archFile = filepath.Join(input, "architecture.yaml")
		if err := runCollectForValidate(input, archFile); err != nil {
			return err
		}
	}

	// Route to Python validator or built-in Go engine
	if validatePython {
		return runPythonValidator(archFile)
	}

	return runGoValidator(archFile)
}

// runCollectForValidate runs `archlint collect <dir> -o <outfile>`.
func runCollectForValidate(dir, outFile string) error {
	fmt.Fprintf(os.Stderr, "Collecting architecture graph: %s -> %s\n", dir, outFile)

	// Use the Go collect logic directly by calling the collect command's logic
	// We invoke os.Executable-relative binary to avoid circular dependency
	exe, err := os.Executable()
	if err != nil {
		exe = "archlint"
	}

	//nolint:gosec // G204: exe and dir come from validated user input
	cmd := exec.Command(exe, "collect", dir, "-o", outFile)
	cmd.Stdout = os.Stderr // collect prints status to stderr-equivalent
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("collect failed: %w", err)
	}
	return nil
}

// findPythonValidatorDir returns the path to the validator/ package.
// Search order: ARCHLINT_VALIDATOR_PATH env, dir relative to binary, cwd.
func findPythonValidatorDir() (string, error) {
	if p := os.Getenv("ARCHLINT_VALIDATOR_PATH"); p != "" {
		return p, nil
	}

	// Try relative to binary
	exe, _ := os.Executable()
	if exe != "" {
		candidate := filepath.Join(filepath.Dir(exe), "validator")
		if _, err := os.Stat(candidate); err == nil {
			return filepath.Dir(candidate), nil
		}
		// One level up (binary in bin/)
		candidate = filepath.Join(filepath.Dir(exe), "..", "validator")
		if _, err := os.Stat(candidate); err == nil {
			return filepath.Dir(candidate), nil
		}
	}

	// Try cwd
	cwd, _ := os.Getwd()
	candidate := filepath.Join(cwd, "validator")
	if _, err := os.Stat(candidate); err == nil {
		return cwd, nil
	}

	return "", fmt.Errorf("validator/ package not found; set ARCHLINT_VALIDATOR_PATH or run from repo root")
}

// runPythonValidator runs `python3 -m validator validate <file> --structure-only -f yaml`
// and prints results grouped by taxonomy.
func runPythonValidator(archFile string) error {
	workDir, err := findPythonValidatorDir()
	if err != nil {
		return err
	}

	pyArgs := []string{"-m", "validator", "validate", archFile, "--structure-only", "-f", "yaml"}
	if validateGroup != "" {
		pyArgs = append(pyArgs, "-g", validateGroup)
	}
	if validateFormat == "json" {
		// override format
		for i, a := range pyArgs {
			if a == "yaml" && i > 0 && pyArgs[i-1] == "-f" {
				pyArgs[i] = "json"
			}
		}
	}

	//nolint:gosec // G204: archFile is validated user input
	pyCmd := exec.Command("python3", pyArgs...)
	pyCmd.Dir = workDir
	pyCmd.Stderr = os.Stderr

	out, err := pyCmd.Output()
	if err != nil {
		// exit code 1 = FAILED, 2 = ERROR - still print output
		exitErr, ok := err.(*exec.ExitError)
		if !ok || (exitErr.ExitCode() != 1 && exitErr.ExitCode() != 2) {
			return fmt.Errorf("python3 validator failed: %w", err)
		}
		// print output even on failure
		if len(out) > 0 {
			printPythonResults(out, validateFormat)
		}
		os.Exit(exitErr.ExitCode())
	}

	printPythonResults(out, validateFormat)
	return nil
}

// printPythonResults formats and prints Python validator output.
func printPythonResults(data []byte, format string) {
	if format == "json" || format == "yaml" {
		os.Stdout.Write(data)
		return
	}

	// Parse YAML and display grouped text summary
	var results map[string]interface{}
	if err := yaml.Unmarshal(data, &results); err != nil {
		// Fall back to raw output
		os.Stdout.Write(data)
		return
	}

	// Print header
	status, _ := results["status"].(string)
	fmt.Printf("status: %s\n", status)

	if summary, ok := results["summary"].(map[string]interface{}); ok {
		fmt.Printf("total:    %v checks\n", summary["total_checks"])
		fmt.Printf("passed:   %v\n", summary["passed"])
		fmt.Printf("failed:   %v\n", summary["failed"])
		fmt.Printf("warnings: %v\n", summary["warnings"])
		fmt.Printf("errors:   %v\n", summary["errors"])
	}

	if graph, ok := results["graph"].(map[string]interface{}); ok {
		fmt.Printf("nodes:    %v\n", graph["nodes"])
		fmt.Printf("edges:    %v\n", graph["edges"])
	}

	// Print failed/warning checks
	checks, ok := results["checks"].([]interface{})
	if !ok || len(checks) == 0 {
		return
	}

	var failed, warnings []map[string]interface{}
	for _, c := range checks {
		check, ok := c.(map[string]interface{})
		if !ok {
			continue
		}
		switch check["status"] {
		case "FAILED":
			failed = append(failed, check)
		case "WARNING":
			warnings = append(warnings, check)
		}
	}

	if len(failed) > 0 {
		fmt.Printf("\nFAILED (%d):\n", len(failed))
		for _, c := range failed {
			printCheck(c)
		}
	}
	if len(warnings) > 0 {
		fmt.Printf("\nWARNINGS (%d):\n", len(warnings))
		for _, c := range warnings {
			printCheck(c)
		}
	}

	if validateGroup == "" && len(failed)+len(warnings) > 5 {
		fmt.Printf("\nTip: use --group to focus on a specific area (core, solid, patterns, architecture, quality)\n")
	}
}

func printCheck(c map[string]interface{}) {
	name, _ := c["name"].(string)
	msg, _ := c["message"].(string)
	if msg == "" {
		msg, _ = c["error"].(string)
	}
	if msg != "" {
		fmt.Printf("  [%s] %s\n", name, msg)
	} else {
		fmt.Printf("  [%s]\n", name)
	}
	if details, ok := c["details"].(string); ok && details != "" {
		for _, line := range strings.Split(details, "\n") {
			if line != "" {
				fmt.Printf("    %s\n", line)
			}
		}
	}
}

// runGoValidator is the original built-in Go validation engine.
func runGoValidator(archFile string) error {
	var data []byte
	var err error

	if archFile == "-" {
		data, err = readAllStdin()
	} else {
		//nolint:gosec // G304: archFile is a user-provided CLI argument
		data, err = os.ReadFile(archFile)
	}
	if err != nil {
		return fmt.Errorf("failed to read graph: %w", err)
	}

	var export GraphExport
	if err := yaml.Unmarshal(data, &export); err != nil {
		return fmt.Errorf("failed to parse graph YAML: %w", err)
	}

	// Load .archlint.yaml config.
	var cfg archlintcfg.Config
	var configFile string
	if validateConfigFile != "" {
		cfg = archlintcfg.LoadFile(validateConfigFile)
		configFile = validateConfigFile
	} else {
		cwd, err := os.Getwd()
		if err != nil {
			cwd = "."
		}
		cfg = archlintcfg.Load(cwd)
		candidate := cwd + "/.archlint.yaml"
		if _, err := os.Stat(candidate); err == nil {
			configFile = candidate
		}
	}

	// Convert GraphExport to model.Graph for validation
	graph := exportToModelGraph(&export)

	// Run config-aware violation checks on the imported graph.
	configViolations := mcp.DetectAllViolationsWithConfig(graph, &cfg)

	switch validateFormat {
	case "json":
		type validateResult struct {
			Graph      *model.Graph    `json:"graph"`
			Violations []mcp.Violation `json:"violations,omitempty"`
			ConfigFile string          `json:"config_file,omitempty"`
		}
		result := validateResult{
			Graph:      graph,
			Violations: configViolations,
			ConfigFile: configFile,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)
	case "text":
		if configFile != "" {
			fmt.Printf("config:     %s\n", configFile)
		}
		fmt.Printf("language:   %s\n", export.Metadata.Language)
		fmt.Printf("root_dir:   %s\n", export.Metadata.RootDir)
		fmt.Printf("analyzed_at: %s\n", export.Metadata.AnalyzedAt)
		fmt.Printf("components: %d\n", len(graph.Nodes))
		fmt.Printf("links:      %d\n", len(graph.Edges))
		if export.Metrics != nil {
			fmt.Printf("violations: %d\n", len(export.Metrics.Violations))
			fmt.Printf("cycles:     %d\n", len(export.Metrics.Cycles))
			fmt.Printf("max_fan_out: %d\n", export.Metrics.MaxFanOut)
		}
		if len(configViolations) > 0 {
			fmt.Printf("\nconfig violations (%d):\n", len(configViolations))
			for _, v := range configViolations {
				level := mcp.ViolationLevel(v, &cfg)
				prefix := mcp.LevelPrefix(level)
				fmt.Printf("  %s [%s] %s\n", prefix, v.Kind, v.Message)
				if v.Target != "" {
					fmt.Printf("    target: %s\n", v.Target)
				}
			}
		}
	default:
		return fmt.Errorf("unknown format: %s (use text, json, or yaml)", validateFormat)
	}

	return nil
}

// exportToModelGraph converts a GraphExport to the internal model.Graph.
func exportToModelGraph(export *GraphExport) *model.Graph {
	nodes := make([]model.Node, 0, len(export.Components))
	for _, n := range export.Components {
		nodes = append(nodes, model.Node{
			ID:     n.ID,
			Title:  n.Title,
			Entity: n.Entity,
		})
	}

	edges := make([]model.Edge, 0, len(export.Links))
	for _, e := range export.Links {
		edges = append(edges, model.Edge{
			From: e.From,
			To:   e.To,
			Type: e.LinkType,
		})
	}

	return &model.Graph{
		Nodes: nodes,
		Edges: edges,
	}
}

// readAllStdin reads all data from os.Stdin.
func readAllStdin() ([]byte, error) {
	var buf []byte
	tmp := make([]byte, 4096)
	for {
		n, err := os.Stdin.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return nil, err
		}
	}
	return buf, nil
}
