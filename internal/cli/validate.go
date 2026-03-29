package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mshogin/archlint/internal/model"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	validateGraphFile string
	validateFormat    string
)

// GraphExport is the standard YAML graph format produced by archlint-rs scan --format yaml.
// It is part of the Unix-pipe multi-language architecture pipeline.
type GraphExport struct {
	Nodes    []GraphExportNode   `yaml:"nodes"    json:"nodes"`
	Edges    []GraphExportEdge   `yaml:"edges"    json:"edges"`
	Metadata GraphExportMetadata `yaml:"metadata" json:"metadata"`
	Metrics  *GraphExportMetrics `yaml:"metrics,omitempty" json:"metrics,omitempty"`
}

// GraphExportNode represents a component node in the exported graph.
type GraphExportNode struct {
	ID      string `yaml:"id"      json:"id"`
	Type    string `yaml:"type"    json:"type"`
	Package string `yaml:"package" json:"package"`
	Name    string `yaml:"name"    json:"name"`
	File    string `yaml:"file"    json:"file"`
	Line    int    `yaml:"line"    json:"line"`
}

// GraphExportEdge represents a dependency edge in the exported graph.
type GraphExportEdge struct {
	From string `yaml:"from" json:"from"`
	To   string `yaml:"to"   json:"to"`
	Type string `yaml:"type" json:"type"`
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

Examples:
  archlint validate --graph graph.yaml
  archlint validate --graph graph.yaml --format yaml
  archlint-rs scan . --format yaml | archlint validate --graph -`,
	RunE: runValidate,
}

func init() {
	validateCmd.Flags().StringVar(&validateGraphFile, "graph", "", "Path to YAML graph file (use - for stdin)")
	validateCmd.Flags().StringVar(&validateFormat, "format", "text", "Output format: text or json")
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

	// Convert GraphExport to model.Graph for validation
	graph := exportToModelGraph(&export)

	switch validateFormat {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(graph)
	case "text":
		fmt.Printf("language:   %s\n", export.Metadata.Language)
		fmt.Printf("root_dir:   %s\n", export.Metadata.RootDir)
		fmt.Printf("analyzed_at: %s\n", export.Metadata.AnalyzedAt)
		fmt.Printf("nodes:      %d\n", len(graph.Nodes))
		fmt.Printf("edges:      %d\n", len(graph.Edges))
		if export.Metrics != nil {
			fmt.Printf("violations: %d\n", len(export.Metrics.Violations))
			fmt.Printf("cycles:     %d\n", len(export.Metrics.Cycles))
			fmt.Printf("max_fan_out: %d\n", export.Metrics.MaxFanOut)
		}
	default:
		return fmt.Errorf("unknown format: %s (use text or json)", validateFormat)
	}

	return nil
}

// exportToModelGraph converts a GraphExport to the internal model.Graph.
func exportToModelGraph(export *GraphExport) *model.Graph {
	nodes := make([]model.Node, 0, len(export.Nodes))
	for _, n := range export.Nodes {
		nodes = append(nodes, model.Node{
			ID:     n.ID,
			Title:  n.Name,
			Entity: n.Type,
		})
	}

	edges := make([]model.Edge, 0, len(export.Edges))
	for _, e := range export.Edges {
		edges = append(edges, model.Edge{
			From: e.From,
			To:   e.To,
			Type: e.Type,
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
