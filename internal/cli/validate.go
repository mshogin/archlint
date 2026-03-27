package cli

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/mshogin/archlint/internal/model"
	"github.com/spf13/cobra"
)

var (
	validateGraphFile string
	validateFormat    string
)

// GraphExport is the standard JSON graph format produced by archlint-rs collect --format json.
// It is part of the Unix-pipe multi-language architecture pipeline.
type GraphExport struct {
	Nodes    []GraphExportNode    `json:"nodes"`
	Edges    []GraphExportEdge    `json:"edges"`
	Metadata GraphExportMetadata  `json:"metadata"`
	Metrics  *GraphExportMetrics  `json:"metrics,omitempty"`
}

// GraphExportNode represents a component node in the exported graph.
type GraphExportNode struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Package  string `json:"package"`
	Name     string `json:"name"`
	File     string `json:"file"`
	Line     int    `json:"line"`
}

// GraphExportEdge represents a dependency edge in the exported graph.
type GraphExportEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"`
}

// GraphExportMetadata contains information about the graph export.
type GraphExportMetadata struct {
	Language   string `json:"language"`
	RootDir    string `json:"root_dir"`
	AnalyzedAt string `json:"analyzed_at"`
}

// GraphExportMetrics contains architecture metrics from the graph export.
type GraphExportMetrics struct {
	ComponentCount int                    `json:"component_count"`
	LinkCount      int                    `json:"link_count"`
	MaxFanOut      int                    `json:"max_fan_out"`
	MaxFanIn       int                    `json:"max_fan_in"`
	Cycles         [][]string             `json:"cycles"`
	Violations     []GraphExportViolation `json:"violations"`
}

// GraphExportViolation is a violation entry in the exported graph metrics.
type GraphExportViolation struct {
	Rule      string `json:"rule"`
	Component string `json:"component"`
	Message   string `json:"message"`
	Severity  string `json:"severity"`
}

var validateCmd = &cobra.Command{
	Use:   "validate",
	Short: "Validate architecture graph from JSON input (Unix-pipe pipeline)",
	Long: `Read a JSON graph produced by archlint-rs collect --format json and validate it.

This command is part of the Unix-pipe multi-language architecture pipeline:
  archlint-rs collect . --format json | archlint validate --graph -

Examples:
  archlint validate --graph graph.json
  archlint validate --graph graph.json --format json
  archlint-rs collect . --format json | archlint validate --graph -`,
	RunE: runValidate,
}

func init() {
	validateCmd.Flags().StringVar(&validateGraphFile, "graph", "", "Path to JSON graph file (use - for stdin)")
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
	if err := json.Unmarshal(data, &export); err != nil {
		return fmt.Errorf("failed to parse graph JSON: %w", err)
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
