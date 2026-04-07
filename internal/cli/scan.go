package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/mshogin/archlint/internal/analyzer"
	"github.com/mshogin/archlint/internal/archlintcfg"
	"github.com/mshogin/archlint/internal/mcp"
	"github.com/spf13/cobra"
)

var (
	scanFormat     string
	scanThreshold  int
	scanConfigFile string
)

var scanCmd = &cobra.Command{
	Use:   "scan [directory]",
	Short: "Scan for architecture violations (quality gate)",
	Long: `Analyze Go source code and report architecture violations.
Supports quality gate mode: exits with code 1 if violations exceed threshold.

Reads .archlint.yaml from the scanned directory (or --config path) to configure
rule thresholds, exclusions, and layer dependency rules. Falls back to built-in
defaults when no config file is found.

Exit codes:
  0 - passed (violations <= threshold)
  1 - failed (violations > threshold)
  2 - error (analysis failed)

Examples:
  archlint scan .
  archlint scan . --format json
  archlint scan . --format json --threshold 5
  archlint scan ./internal --threshold 0
  archlint scan . --config /path/to/.archlint.yaml`,
	Args: cobra.ExactArgs(1),
	RunE: runScan,
}

func init() {
	scanCmd.Flags().StringVar(&scanFormat, "format", "text", "Output format: text or json")
	scanCmd.Flags().IntVar(&scanThreshold, "threshold", -1, "Max violations before failing gate (-1 = any violation fails)")
	scanCmd.Flags().StringVar(&scanConfigFile, "config", "", "Path to .archlint.yaml config file (default: <directory>/.archlint.yaml)")
	rootCmd.AddCommand(scanCmd)
}

// scanGateResult is the JSON output for the scan command.
type scanGateResult struct {
	Passed     bool            `json:"passed"`
	Violations int             `json:"violations"`
	Threshold  int             `json:"threshold"`
	Categories map[string]int  `json:"categories"`
	Details    []mcp.Violation `json:"details"`
	ConfigFile string          `json:"config_file,omitempty"`
}

func runScan(cmd *cobra.Command, args []string) error {
	codeDir := args[0]

	if _, err := os.Stat(codeDir); os.IsNotExist(err) {
		fmt.Fprintf(os.Stderr, "error: %v: %s\n", errDirNotExist, codeDir)
		os.Exit(2)
	}

	// Load .archlint.yaml config.
	var cfg archlintcfg.Config
	var configFile string
	if scanConfigFile != "" {
		cfg = archlintcfg.LoadFile(scanConfigFile)
		configFile = scanConfigFile
	} else {
		absDir, err := filepath.Abs(codeDir)
		if err != nil {
			absDir = codeDir
		}
		cfg = archlintcfg.Load(absDir)
		candidate := filepath.Join(absDir, ".archlint.yaml")
		if _, err := os.Stat(candidate); err == nil {
			configFile = candidate
		}
	}

	a := analyzer.NewGoAnalyzer()

	graph, err := a.Analyze(codeDir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "analysis error: %v\n", err)
		os.Exit(2)
	}

	// Structural violations (coupling, cycles) — config-aware.
	violations := mcp.DetectAllViolationsWithConfig(graph, &cfg)

	// Per-file SOLID and smell violations.
	allMetrics := mcp.ComputeAllFileMetrics(a, graph)

	for _, m := range allMetrics {
		// DIP violations — respect config enabled flag.
		if cfg.Rules.DIP.Enabled {
			violations = append(violations, m.DIPViolations...)
		}
		// ISP violations — respect config enabled flag.
		if cfg.Rules.ISP.Enabled {
			violations = append(violations, m.ISPViolations...)
		}
		// SRP violations — respect config enabled flag and exclusions.
		if cfg.Rules.SRP.Enabled {
			for _, v := range m.SRPViolations {
				if !cfg.IsSRPExcluded(v.Target) {
					violations = append(violations, v)
				}
			}
		}

		// God-class violations — respect config enabled flag and exclusions.
		if cfg.Rules.GodClass.Enabled {
			for _, gc := range m.GodClasses {
				if !cfg.IsGodClassExcluded(gc) {
					violations = append(violations, mcp.Violation{
						Kind:    "god-class",
						Message: fmt.Sprintf("God class detected: %s", gc),
						Target:  gc,
					})
				}
			}
		}

		// Hub-node violations — respect config enabled flag and exclusions.
		if cfg.Rules.HubNode.Enabled {
			for _, hub := range m.HubNodes {
				if !cfg.IsHubNodeExcluded(hub) {
					violations = append(violations, mcp.Violation{
						Kind:    "hub-node",
						Message: fmt.Sprintf("Hub node detected: %s", hub),
						Target:  hub,
					})
				}
			}
		}

		// Feature-envy violations — respect config enabled flag and exclusions.
		if cfg.Rules.FeatureEnvy.Enabled {
			for _, fe := range m.FeatureEnvy {
				if !cfg.IsFeatureEnvyExcluded(fe) {
					violations = append(violations, mcp.Violation{
						Kind:    "feature-envy",
						Message: fmt.Sprintf("Feature envy: %s", fe),
						Target:  fe,
					})
				}
			}
		}

		for _, ss := range m.ShotgunSurgery {
			violations = append(violations, mcp.Violation{
				Kind:    "shotgun-surgery",
				Message: fmt.Sprintf("Shotgun surgery risk: %s", ss),
				Target:  ss,
			})
		}
	}

	// Sort violations by kind then target for stable output.
	sort.Slice(violations, func(i, j int) bool {
		if violations[i].Kind != violations[j].Kind {
			return violations[i].Kind < violations[j].Kind
		}
		return violations[i].Target < violations[j].Target
	})

	// Determine threshold: -1 means any violation fails (equivalent to threshold 0).
	threshold := scanThreshold
	if threshold < 0 {
		threshold = 0
	}

	total := len(violations)
	passed := total <= threshold

	// Build categories map.
	categories := make(map[string]int)
	for _, v := range violations {
		categories[v.Kind]++
	}

	switch scanFormat {
	case "json":
		result := scanGateResult{
			Passed:     passed,
			Violations: total,
			Threshold:  threshold,
			Categories: categories,
			Details:    violations,
			ConfigFile: configFile,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			fmt.Fprintf(os.Stderr, "JSON encoding error: %v\n", err)
			os.Exit(2)
		}
	case "text":
		if configFile != "" {
			fmt.Printf("config: %s\n", configFile)
		}
		if total == 0 {
			fmt.Printf("PASSED: No violations found (threshold: %d)\n", threshold)
		} else {
			status := "PASSED"
			if !passed {
				status = "FAILED"
			}
			fmt.Printf("%s: %d violations found (threshold: %d)\n\n", status, total, threshold)

			for _, v := range violations {
				level := mcp.ViolationLevel(v, &cfg)
				prefix := mcp.LevelPrefix(level)
				fmt.Printf("%s [%s] %s\n", prefix, v.Kind, v.Message)
				if v.Target != "" {
					fmt.Printf("  target: %s\n", v.Target)
				}
				fmt.Println()
			}
		}
	default:
		fmt.Fprintf(os.Stderr, "unknown format: %s (use text or json)\n", scanFormat)
		os.Exit(2)
	}

	if !passed {
		os.Exit(1)
	}

	return nil
}
