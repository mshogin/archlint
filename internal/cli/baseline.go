package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mshogin/archlint/internal/archlintcfg"
	"github.com/mshogin/archlint/internal/mcp"
	"github.com/spf13/cobra"
)

var (
	baselineOutputFile string
	baselineConfigFile string
	baselineExclude    []string
)

var baselineCmd = &cobra.Command{
	Use:   "baseline [directory]",
	Short: "Snapshot current ERROR-class architecture patterns for the delta gate",
	Long: `Build a baseline snapshot of the code's current ERROR-class pattern facts
(SCC cycles, layer back-edges, dead code) into .archlint-baseline.json.

The delta gate (archlint scan) compares live patterns against this baseline and
blocks only NEW regressions, not pre-existing (legacy) violations. Without a
baseline file scan degrades to audit (telemetry, no block).

The snapshot is deterministic: two runs over identical code are byte-identical
(sorted, stable keys). Commit it to lock in the current architecture as the floor.

Examples:
  archlint baseline .
  archlint baseline ./internal -o ./internal/.archlint-baseline.json`,
	Args: cobra.ExactArgs(1),
	RunE: runBaseline,
}

func init() {
	baselineCmd.Flags().StringVarP(&baselineOutputFile, "output", "o", "", "Output path (default: <directory>/.archlint-baseline.json)")
	baselineCmd.Flags().StringVar(&baselineConfigFile, "config", "", "Path to .archlint.yaml config file (default: <directory>/.archlint.yaml)")
	baselineCmd.Flags().StringSliceVar(&baselineExclude, "exclude", nil, "Directory basenames to skip during the source walk (additive). Repeatable.")
	rootCmd.AddCommand(baselineCmd)
}

func runBaseline(_ *cobra.Command, args []string) error {
	codeDir := args[0]

	var cfg archlintcfg.Config
	if baselineConfigFile != "" {
		cfg = archlintcfg.LoadFile(baselineConfigFile)
	} else {
		absDir, err := filepath.Abs(codeDir)
		if err != nil {
			absDir = codeDir
		}
		cfg = archlintcfg.Load(absDir)
	}

	excludes := mergeExcludes(cfg.ExcludePaths, baselineExclude)

	graph, a, err := analyzeForGate(codeDir, excludes)
	if err != nil {
		return err
	}

	violations := errorClassViolations(graph, a, &cfg)
	baseline := mcp.BuildBaseline(violations)

	data, err := json.MarshalIndent(baseline, "", "  ")
	if err != nil {
		return fmt.Errorf("baseline serialization error: %w", err)
	}
	data = append(data, '\n')

	outPath := baselineOutputFile
	if outPath == "" {
		outPath = filepath.Join(codeDir, defaultBaselineName)
	}

	//nolint:gosec // G304: outPath is a user-provided CLI argument
	if err := os.WriteFile(outPath, data, 0o644); err != nil {
		return fmt.Errorf("failed to write baseline %s: %w", outPath, err)
	}

	total := 0
	for _, fps := range baseline.Patterns {
		total += len(fps)
	}
	fmt.Fprintf(os.Stderr, "baseline written: %s (%d ERROR-class patterns across %d kinds)\n", outPath, total, len(baseline.Patterns))

	return nil
}

// loadBaseline читает снимок дельта-гейта. Отсутствие файла -> (nil, nil): гейт
// деградирует в audit (no-baseline -> no-block, DR-0034 п.2), это НЕ ошибка.
func loadBaseline(path string) (*mcp.Baseline, error) {
	data, err := os.ReadFile(path) //nolint:gosec // G304: path derived from scanned dir / --baseline flag
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to read baseline %s: %w", path, err)
	}
	var b mcp.Baseline
	if err := json.Unmarshal(data, &b); err != nil {
		return nil, fmt.Errorf("failed to parse baseline %s: %w", path, err)
	}
	return &b, nil
}
