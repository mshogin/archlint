package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mshogin/archlint/internal/analyzer"
	"github.com/mshogin/archlint/internal/config"
	"github.com/mshogin/archlint/pkg/callgraph"
	"github.com/spf13/cobra"
)

var (
	cgBPMNContexts string
	cgEntryPoint   string
	cgOutputDir    string
	cgMaxDepth     int
	cgNoPuml       bool
	cgPumlDepth    int
	cgLanguage     string
)

var (
	errNoModeSelected = errors.New("must specify --bpmn-contexts or --entry")
)

var callgraphCmd = &cobra.Command{
	Use:   "callgraph [directory]",
	Short: "Build call graphs from entry points",
	Long: `Builds call graphs from source code.

Two modes:
  1. Full mode (BPMN contexts): --bpmn-contexts bpmn-contexts.yaml
  2. Single mode: --entry "internal/service.OrderService.ProcessOrder"

Example:
  archlint callgraph ./src --bpmn-contexts bpmn-contexts.yaml -o callgraphs/
  archlint callgraph ./src --entry "internal/service.OrderService.ProcessOrder"`,
	Args: cobra.ExactArgs(1),
	RunE: runCallgraph,
}

func init() {
	callgraphCmd.Flags().StringVar(&cgBPMNContexts, "bpmn-contexts", "", "Contexts config file (bpmn-contexts.yaml)")
	callgraphCmd.Flags().StringVar(&cgEntryPoint, "entry", "", "Entry point (single mode)")
	callgraphCmd.Flags().StringVarP(&cgOutputDir, "output", "o", "callgraphs", "Output directory")
	callgraphCmd.Flags().IntVar(&cgMaxDepth, "max-depth", 10, "Maximum analysis depth")
	callgraphCmd.Flags().BoolVar(&cgNoPuml, "no-puml", false, "Do not generate PlantUML diagrams")
	callgraphCmd.Flags().IntVar(&cgPumlDepth, "puml-depth", 5, "Depth for PlantUML diagrams")
	callgraphCmd.Flags().StringVarP(&cgLanguage, "language", "l", "go", "Programming language (go)")
	rootCmd.AddCommand(callgraphCmd)
}

func runCallgraph(cmd *cobra.Command, args []string) error {
	codeDir := args[0]

	if _, err := os.Stat(codeDir); os.IsNotExist(err) {
		return fmt.Errorf("%w: %s", errDirNotExist, codeDir)
	}

	if cgBPMNContexts == "" && cgEntryPoint == "" {
		return errNoModeSelected
	}

	fmt.Printf("Analyzing code: %s (language: %s)\n", codeDir, cgLanguage)

	goAnalyzer := analyzer.NewGoAnalyzer()

	if _, err := goAnalyzer.Analyze(codeDir); err != nil {
		return fmt.Errorf("code analysis error: %w", err)
	}

	opts := callgraph.BuildOptions{
		MaxDepth:          cgMaxDepth,
		ResolveInterfaces: true,
		TrackGoroutines:   true,
	}

	if err := os.MkdirAll(cgOutputDir, 0o750); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", cgOutputDir, err)
	}

	if cgEntryPoint != "" {
		return runSingleMode(goAnalyzer, opts)
	}

	return runFullMode(goAnalyzer, opts)
}

func runSingleMode(goAnalyzer *analyzer.GoAnalyzer, opts callgraph.BuildOptions) error {
	builder, err := callgraph.NewBuilder(goAnalyzer, opts)
	if err != nil {
		return fmt.Errorf("failed to create builder: %w", err)
	}

	cg, err := builder.Build(cgEntryPoint)
	if err != nil {
		return fmt.Errorf("failed to build call graph: %w", err)
	}

	if err := exportSingleGraph(cg); err != nil {
		return err
	}

	return nil
}

func exportSingleGraph(cg *callgraph.CallGraph) error {
	fmt.Printf("Граф построен: %d nodes, %d edges, depth %d\n",
		cg.Stats.TotalNodes, cg.Stats.TotalEdges, cg.ActualDepth)

	exporter := callgraph.NewYAMLExporter()
	yamlPath := filepath.Join(cgOutputDir, "callgraph.yaml")

	if err := exporter.ExportCallGraph(cg, yamlPath); err != nil {
		return fmt.Errorf("YAML export error: %w", err)
	}

	fmt.Printf("YAML: %s\n", yamlPath)

	if !cgNoPuml {
		if err := generatePuml(cg, cgEntryPoint); err != nil {
			return fmt.Errorf("генерация PlantUML: %w", err)
		}
	}

	return nil
}

func runFullMode(goAnalyzer *analyzer.GoAnalyzer, opts callgraph.BuildOptions) error {
	set, err := buildEventGraphs(goAnalyzer, opts)
	if err != nil {
		return err
	}

	if err := exportEventGraphs(set); err != nil {
		return err
	}

	return nil
}

func buildEventGraphs(goAnalyzer *analyzer.GoAnalyzer, opts callgraph.BuildOptions) (*callgraph.EventCallGraphSet, error) {
	contexts, warnings, err := config.LoadBPMNContexts(cgBPMNContexts)
	if err != nil {
		return nil, fmt.Errorf("failed to load contexts: %w", err)
	}

	for _, w := range warnings {
		fmt.Printf("Warning: %s\n", w)
	}

	eventCount := 0
	for _, ctx := range contexts.Contexts {
		eventCount += len(ctx.Events)
	}

	fmt.Printf("Loaded contexts: %d, events: %d\n", len(contexts.Contexts), eventCount)

	eventBuilder, err := callgraph.NewEventBuilder(goAnalyzer, contexts, opts)
	if err != nil {
		return nil, fmt.Errorf("создание event builder: %w", err)
	}

	set, err := eventBuilder.BuildAll()
	if err != nil {
		return nil, fmt.Errorf("failed to build graphs: %w", err)
	}

	return set, nil
}

func exportEventGraphs(set *callgraph.EventCallGraphSet) error {
	for _, w := range set.Warnings {
		fmt.Printf("  WARN: %s\n", w)
	}

	fmt.Printf("\nResult: %d graphs built, %d warnings\n",
		set.Stats.BuiltGraphs, set.Stats.FailedGraphs)

	exporter := callgraph.NewYAMLExporter()
	yamlPath := filepath.Join(cgOutputDir, "event-graphs.yaml")

	if err := exporter.ExportEventSet(set, yamlPath); err != nil {
		return fmt.Errorf("YAML export error: %w", err)
	}

	fmt.Printf("YAML: %s\n", yamlPath)

	if !cgNoPuml {
		for eventID := range set.Graphs {
			cg := set.Graphs[eventID]

			if err := generatePuml(&cg, eventID); err != nil {
				fmt.Printf("  Warning: PlantUML for %s: %v\n", eventID, err)
			}
		}
	}

	return nil
}

func generatePuml(cg *callgraph.CallGraph, name string) error {
	gen := callgraph.NewSequenceGenerator(callgraph.SequenceOptions{
		MaxDepth:       cgPumlDepth,
		ShowPackages:   true,
		MarkAsync:      true,
		MarkInterface:  true,
		GroupByPackage: true,
		Title:          fmt.Sprintf("Call Graph: %s", name),
	})

	puml, err := gen.Generate(cg)
	if err != nil {
		return fmt.Errorf("PlantUML generation error: %w", err)
	}

	safeName := sanitizeFilename(name)
	pumlPath := filepath.Join(cgOutputDir, safeName+".puml")

	//nolint:gosec // G306: puml files are not sensitive
	if err := os.WriteFile(pumlPath, []byte(puml), 0o644); err != nil {
		return fmt.Errorf("failed to write PlantUML %s: %w", pumlPath, err)
	}

	fmt.Printf("PlantUML: %s\n", pumlPath)

	return nil
}

var unsafeFilenameChars = regexp.MustCompile(`[^a-zA-Z0-9._-]`)

func sanitizeFilename(name string) string {
	safe := unsafeFilenameChars.ReplaceAllString(name, "_")
	safe = strings.TrimLeft(safe, ".")

	const maxLen = 200
	if len(safe) > maxLen {
		safe = safe[:maxLen]
	}

	if safe == "" {
		safe = "unnamed"
	}

	return safe
}
