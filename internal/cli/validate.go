package cli

import (
	"encoding/json"
	"fmt"
	"os"

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
	Use:   "validate",
	Short: "Validate architecture graph from YAML input (Unix-pipe pipeline)",
	Long: `Read a YAML graph produced by archlint-rs scan --format yaml and validate it.

This command is part of the Unix-pipe multi-language architecture pipeline:
  archlint-rs scan . --format yaml | archlint validate --graph -

Config file (.archlint.yaml) is loaded from the current directory by default.
Use --config to specify an explicit path.

Examples:
  archlint validate --graph graph.yaml
  archlint validate --graph graph.yaml --format yaml
  archlint validate --graph graph.yaml --config /path/to/.archlint.yaml
  archlint-rs scan . --format yaml | archlint validate --graph -`,
	RunE: runValidate,
}

func init() {
	validateCmd.Flags().StringVar(&validateGraphFile, "graph", "", "Path to YAML graph file (use - for stdin)")
	validateCmd.Flags().StringVar(&validateFormat, "format", "text", "Output format: text or json")
	validateCmd.Flags().StringVar(&validateConfigFile, "config", "", "Path to .archlint.yaml config file (default: ./.archlint.yaml)")
	_ = validateCmd.MarkFlagRequired("graph")
	rootCmd.AddCommand(validateCmd)
}

func runValidate(cmd *cobra.Command, args []string) error {
	// Read graph JSON from file or stdin
	var data []byte
	var err error

	if validateGraphFile == "-" {
		data, err = readAllStdin()
	} else {
		//nolint:gosec // G304: validateGraphFile is a user-provided CLI argument
		data, err = os.ReadFile(validateGraphFile)
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
		// Default: look in current working directory.
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
		return fmt.Errorf("unknown format: %s (use text or json)", validateFormat)
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
