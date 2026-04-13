package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/mshogin/archlint/internal/analyzer"
	"github.com/mshogin/archlint/internal/model"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	collectOutputFile string
	collectLanguage   string
)

var (
	errDirNotExist       = errors.New("directory does not exist")
	errUnsupportedLang   = errors.New("unsupported language")
	errFileCreate        = errors.New("failed to create file")
	errYAMLSerialization = errors.New("YAML serialization error")
)

var collectCmd = &cobra.Command{
	Use:   "collect [directory]",
	Short: "Collect architecture from source code",
	Long: `Analyzes source code and builds an architecture graph in YAML format.

Example:
  archlint collect . -l go -o architecture.yaml`,
	Args: cobra.ExactArgs(1),
	RunE: runCollect,
}

func init() {
	collectCmd.Flags().StringVarP(&collectOutputFile, "output", "o", "architecture.yaml", "Output YAML file")
	collectCmd.Flags().StringVarP(&collectLanguage, "language", "l", "go", "Programming language (go, typescript, rust) - auto-detected for TypeScript and Rust projects")
	rootCmd.AddCommand(collectCmd)
}

func runCollect(cmd *cobra.Command, args []string) error {
	codeDir := args[0]

	if _, err := os.Stat(codeDir); os.IsNotExist(err) {
		return fmt.Errorf("%w: %s", errDirNotExist, codeDir)
	}

	// Auto-detect project language if not explicitly overridden.
	languageChanged := cmd != nil && cmd.Flags().Changed("language")
	if !languageChanged {
		if analyzer.DetectRustProject(codeDir) {
			collectLanguage = "rust"
		} else if analyzer.DetectTypeScriptProject(codeDir) {
			collectLanguage = "typescript"
		}
	}

	// When writing YAML to stdout (-o -), redirect status messages to stderr
	// so that the output can be piped cleanly.
	statusOut := os.Stdout
	if collectOutputFile == "-" {
		statusOut = os.Stderr
	}

	fmt.Fprintf(statusOut, "Analyzing code: %s (language: %s)\n", codeDir, collectLanguage)

	graph, err := analyzeCode(codeDir)
	if err != nil {
		return err
	}

	printStatsTo(graph, statusOut)

	if err := saveGraph(graph); err != nil {
		return err
	}

	if collectOutputFile != "-" {
		fmt.Fprintf(statusOut, "Graph saved to %s\n", collectOutputFile)
	}

	return nil
}

func analyzeCode(codeDir string) (*model.Graph, error) {
	switch collectLanguage {
	case "go":
		goAnalyzer := analyzer.NewGoAnalyzer()

		graph, err := goAnalyzer.Analyze(codeDir)
		if err != nil {
			return nil, fmt.Errorf("analysis error: %w", err)
		}

		return graph, nil
	case "typescript", "ts", "tsx", "javascript", "js":
		tsAnalyzer := analyzer.NewTypeScriptAnalyzer()

		graph, err := tsAnalyzer.Analyze(codeDir)
		if err != nil {
			return nil, fmt.Errorf("analysis error: %w", err)
		}

		return graph, nil
	case "rust", "rs":
		rustAnalyzer := analyzer.NewRustAnalyzer()

		graph, err := rustAnalyzer.Analyze(codeDir)
		if err != nil {
			return nil, fmt.Errorf("analysis error: %w", err)
		}

		return graph, nil
	default:
		return nil, fmt.Errorf("%w: %s", errUnsupportedLang, collectLanguage)
	}
}

func printStats(graph *model.Graph) {
	printStatsTo(graph, os.Stdout)
}

func printStatsTo(graph *model.Graph, w *os.File) {
	stats := make(map[string]int)

	for _, node := range graph.Nodes {
		stats[node.Entity]++
	}

	fmt.Fprintf(w, "Found components: %d\n", len(graph.Nodes))

	for entity, count := range stats {
		fmt.Fprintf(w, "  - %s: %d\n", entity, count)
	}

	fmt.Fprintf(w, "Found edges: %d\n", len(graph.Edges))
}

func saveGraph(graph *model.Graph) error {
	var file *os.File
	if collectOutputFile == "-" {
		file = os.Stdout
	} else {
		//nolint:gosec // G304: collectOutputFile is a user-provided CLI argument
		f, err := os.OpenFile(collectOutputFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o640)
		if err != nil {
			return fmt.Errorf("%w: %w", errFileCreate, err)
		}
		defer func() {
			if closeErr := f.Close(); closeErr != nil {
				fmt.Fprintf(os.Stderr, "Warning: failed to close file: %v\n", closeErr)
			}
		}()
		file = f
	}

	encoder := yaml.NewEncoder(file)
	encoder.SetIndent(2)

	defer func() {
		if closeErr := encoder.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close encoder: %v\n", closeErr)
		}
	}()

	if err := encoder.Encode(graph); err != nil {
		return fmt.Errorf("%w: %w", errYAMLSerialization, err)
	}

	return nil
}
