package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

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

// GraphExport is the standard YAML graph format produced by 'archlint scan --format yaml'.
// It is part of the Unix-pipe multi-language architecture pipeline.
// Field names: components/links (not nodes/edges).
type GraphExport struct {
	Components []GraphExportNode   `yaml:"components" json:"components"`
	Links      []GraphExportEdge   `yaml:"links"      json:"links"`
	Metadata   GraphExportMetadata `yaml:"metadata"   json:"metadata"`
	Metrics    *GraphExportMetrics `yaml:"metrics,omitempty" json:"metrics,omitempty"`
}

// GraphExportNode represents a component in the exported graph.
// Fields: id, title, entity.
type GraphExportNode struct {
	ID     string `yaml:"id"     json:"id"`
	Title  string `yaml:"title"  json:"title"`
	Entity string `yaml:"entity" json:"entity"`
}

// GraphExportEdge represents a dependency link in the exported graph.
// Fields: from, to, link_type.
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
	Short: "Validate architecture via the built-in Go engine (--python выпилен из бинаря)",
	Long: `Validate an architecture graph with the built-in Go rule engine.

The production/boevoy path is the native Go detectors (see 'archlint scan'):
SCC/cycles, dead-code, ISP, layering — soundness-gated, used in the agent loop
and CI gate. This command runs that Go engine.

--python ВЫПИЛЕН из бинаря (0 Python в боевом пути). Структурные/доказуемые метрики
портированы в Go; флаг --python теперь возвращает ошибку с указанием на боевой Go-движок.
validator/ остаётся deprecated-музеем (research/математика), бинарь его НЕ вызывает.

If input is a directory, first runs archlint collect to generate architecture.yaml,
then runs the Go engine on it. If input is a .yaml file, runs directly on it.

Examples:
  archlint validate .
  archlint validate architecture.yaml
  archlint scan .                 # боевой дельта-гейт (рекомендуется)`,
	Args: cobra.MaximumNArgs(1),
	RunE: runValidate,
}

func init() {
	validateCmd.Flags().StringVar(&validateGraphFile, "graph", "", "Path to YAML graph file (use - for stdin); legacy flag, prefer positional arg")
	validateCmd.Flags().StringVar(&validateFormat, "format", "text", "Output format: text, json, or yaml")
	validateCmd.Flags().StringVar(&validateConfigFile, "config", "", "Path to .archlint.yaml config file (default: ./.archlint.yaml)")
	validateCmd.Flags().StringVar(&validateGroup, "group", "", "(устар.: применялся к --python музею, который выпилен)")
	validateCmd.Flags().BoolVar(&validatePython, "python", false, "ВЫПИЛЕН: Python-валидатор удалён из бинаря (вернёт ошибку с указанием на Go-движок)")
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

	// --python ОТКЛЮЧЁН: Python-валидатор выпилен из бинаря (0 Python в боевом пути).
	// ЧЕСТНАЯ ошибка (не молча 0/тихий пропуск — урок ложно-зелёного): явно говорим, куда
	// делся музей. validator/ остаётся как deprecated-музей; его судьба (репо vs архив) —
	// отдельное решение, бинарь его НЕ зовёт в любом случае.
	if validatePython {
		return fmt.Errorf("--python отключён: Python-валидатор выпилен из бинаря (0 Python в боевом пути).\n" +
			"  Структурные/доказуемые метрики портированы в Go — используй боевой движок:\n" +
			"    archlint scan .            (дельта-гейт, боевой путь)\n" +
			"    archlint validate <file>   (Go-движок, без --python)\n" +
			"  validator/ остаётся deprecated-музеем (research/математика, не арх-гейт);\n" +
			"  бинарь его не вызывает. Доступ к музею — напрямую: python3 -m validator (вне archlint)")
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

// runGoValidator is the built-in Go validation engine (default; --python выпилен).
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
