package cli

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/mshogin/archlint/internal/analyzer"
	"github.com/mshogin/archlint/internal/archlintcfg"
	"github.com/mshogin/archlint/internal/optimizer"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	optimizeConfigFile string
	optimizeTopN       int
)

var optimizeCmd = &cobra.Command{
	Use:   "optimize [directory]",
	Short: "Suggest dependency changes to hit metric targets",
	Long: `Analyze the architecture graph and suggest which import edges to remove
so that the project reaches the metric targets defined in .archlint.yaml.

The optimizer reads the 'optimize' section of .archlint.yaml:

  optimize:
    targets:
      max_fan_out: "<= 7"
      spanning_tree_coverage: ">= 0.4"
    constraints:
      preserve_components: ["internal/model/*"]

For each import edge the optimizer simulates its removal, recomputes the
affected metrics, and scores how many targets move closer to satisfied.
Suggestions are ranked by impact score (highest first).

Examples:
  archlint optimize .
  archlint optimize . --config .archlint.yaml
  archlint optimize ./internal --top 5`,
	Args: cobra.ExactArgs(1),
	RunE: runOptimize,
}

func init() {
	optimizeCmd.Flags().StringVar(&optimizeConfigFile, "config", "", "Path to .archlint.yaml (default: <directory>/.archlint.yaml)")
	optimizeCmd.Flags().IntVar(&optimizeTopN, "top", 10, "Maximum number of suggestions to show")
	rootCmd.AddCommand(optimizeCmd)
}

// optimizeConfig is the yaml schema for the 'optimize' section.
type optimizeConfig struct {
	Targets struct {
		MaxFanOut            string `yaml:"max_fan_out"`
		SpanningTreeCoverage string `yaml:"spanning_tree_coverage"`
	} `yaml:"targets"`
	Constraints struct {
		PreserveComponents []string `yaml:"preserve_components"`
	} `yaml:"constraints"`
}

// fullConfig is a minimal envelope so we can unmarshal only the optimize section.
type fullConfig struct {
	Optimize optimizeConfig `yaml:"optimize"`
}

func runOptimize(cmd *cobra.Command, args []string) error {
	codeDir := args[0]

	if _, err := os.Stat(codeDir); os.IsNotExist(err) {
		return fmt.Errorf("%w: %s", errDirNotExist, codeDir)
	}

	// Resolve config path.
	cfgPath := optimizeConfigFile
	if cfgPath == "" {
		absDir, err := filepath.Abs(codeDir)
		if err != nil {
			absDir = codeDir
		}
		cfgPath = filepath.Join(absDir, ".archlint.yaml")
	}

	// Load archlintcfg for the fan-out threshold default.
	_ = archlintcfg.LoadFile(cfgPath) // load to trigger any warnings; result unused here

	// Load the optimize section.
	optCfg, err := loadOptimizeConfig(cfgPath)
	if err != nil {
		return fmt.Errorf("config error: %w", err)
	}

	// Parse targets.
	targets, err := parseTargets(optCfg)
	if err != nil {
		return fmt.Errorf("target parse error: %w", err)
	}

	if targets.MaxFanOut == nil && targets.SpanningTreeCoverage == nil {
		fmt.Println("No optimize targets configured in .archlint.yaml.")
		fmt.Println()
		fmt.Println("Add an 'optimize' section, for example:")
		fmt.Println()
		fmt.Println("  optimize:")
		fmt.Println("    targets:")
		fmt.Println("      max_fan_out: \"<= 7\"")
		fmt.Println("      spanning_tree_coverage: \">= 0.4\"")
		return nil
	}

	// Analyze the project.
	a := analyzer.NewGoAnalyzer()
	graph, err := a.Analyze(codeDir)
	if err != nil {
		return fmt.Errorf("analysis error: %w", err)
	}

	// Compute baseline metrics.
	baseline := optimizer.ComputeMetrics(graph)

	// Print current metrics.
	fmt.Println("Current metrics:")
	if targets.MaxFanOut != nil {
		status := "OK"
		if !targets.MaxFanOut.Satisfied(float64(baseline.MaxFanOut)) {
			status = fmt.Sprintf("target: %s%g", targets.MaxFanOut.Op, targets.MaxFanOut.Value)
		}
		fmt.Printf("  max_fan_out:            %d (%s)\n", baseline.MaxFanOut, status)
	}
	if targets.SpanningTreeCoverage != nil {
		status := "OK"
		if !targets.SpanningTreeCoverage.Satisfied(baseline.SpanningTreeCoverage) {
			status = fmt.Sprintf("target: %s%g", targets.SpanningTreeCoverage.Op, targets.SpanningTreeCoverage.Value)
		}
		fmt.Printf("  spanning_tree_coverage: %.2f (%s)\n", baseline.SpanningTreeCoverage, status)
	}
	fmt.Println()

	// Run optimizer.
	preserved := optCfg.Constraints.PreserveComponents
	opt := optimizer.New(targets, preserved, optimizeTopN)
	suggestions := opt.Optimize(graph)

	if len(suggestions) == 0 {
		fmt.Println("All targets already satisfied — no suggestions.")
		return nil
	}

	fmt.Printf("Suggestions (ranked by impact, top %d):\n\n", len(suggestions))

	for i, s := range suggestions {
		fmt.Printf("%d. REMOVE EDGE  %s -> %s\n", i+1, s.From, s.To)

		metricParts := ""
		if s.FanOutDelta != 0 {
			metricParts += fmt.Sprintf("max_fan_out %+d", s.FanOutDelta)
		}
		if s.CoverageDelta != 0 {
			if metricParts != "" {
				metricParts += ", "
			}
			metricParts += fmt.Sprintf("spanning_tree_coverage %+.2f", s.CoverageDelta)
		}
		if metricParts != "" {
			fmt.Printf("   Impact: %s\n", metricParts)
		}
		fmt.Printf("   Reason: %s\n", s.Reason)
		fmt.Println()
	}

	// Total impact if all applied.
	if len(suggestions) > 1 {
		printTotalImpact(baseline, targets, suggestions)
	}

	return nil
}

// printTotalImpact prints the projected metric values after applying all suggestions.
func printTotalImpact(baseline optimizer.Metrics, targets optimizer.Targets, suggestions []optimizer.Suggestion) {
	totalFanOutDelta := 0
	totalCoverageDelta := 0.0

	for _, s := range suggestions {
		totalFanOutDelta += s.FanOutDelta
		totalCoverageDelta += s.CoverageDelta
	}

	fmt.Print("Total impact if all applied:")
	if targets.MaxFanOut != nil {
		after := baseline.MaxFanOut + totalFanOutDelta
		if after < 0 {
			after = 0
		}
		fmt.Printf("  max_fan_out %d -> %d", baseline.MaxFanOut, after)
	}
	if targets.SpanningTreeCoverage != nil {
		after := baseline.SpanningTreeCoverage + totalCoverageDelta
		if after > 1.0 {
			after = 1.0
		}
		fmt.Printf("  spanning_tree_coverage %.2f -> %.2f", baseline.SpanningTreeCoverage, after)
	}
	fmt.Println()
}

// loadOptimizeConfig reads only the 'optimize' section from the yaml file.
// Returns an empty config (no targets) if the file doesn't exist or has no
// optimize section — the command will then show a helpful message.
func loadOptimizeConfig(path string) (optimizeConfig, error) {
	data, err := os.ReadFile(path) //nolint:gosec // user-provided path
	if err != nil {
		if os.IsNotExist(err) {
			return optimizeConfig{}, nil
		}
		return optimizeConfig{}, fmt.Errorf("cannot read %s: %w", path, err)
	}

	var full fullConfig
	if err := yaml.Unmarshal(data, &full); err != nil {
		return optimizeConfig{}, fmt.Errorf("cannot parse %s: %w", path, err)
	}

	return full.Optimize, nil
}

// parseTargets converts the raw string config into typed Constraint values.
func parseTargets(cfg optimizeConfig) (optimizer.Targets, error) {
	var t optimizer.Targets

	if cfg.Targets.MaxFanOut != "" {
		c, err := optimizer.ParseConstraint(cfg.Targets.MaxFanOut)
		if err != nil {
			return t, fmt.Errorf("max_fan_out: %w", err)
		}
		t.MaxFanOut = c
	}

	if cfg.Targets.SpanningTreeCoverage != "" {
		c, err := optimizer.ParseConstraint(cfg.Targets.SpanningTreeCoverage)
		if err != nil {
			return t, fmt.Errorf("spanning_tree_coverage: %w", err)
		}
		t.SpanningTreeCoverage = c
	}

	return t, nil
}
