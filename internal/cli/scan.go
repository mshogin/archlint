package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"

	"github.com/mshogin/archlint/internal/analyzer"
	"github.com/mshogin/archlint/internal/archlintcfg"
	"github.com/mshogin/archlint/internal/mcp"
	"github.com/mshogin/archlint/internal/model"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	scanFormat     string
	scanThreshold  int
	scanConfigFile string
	scanStdin      bool
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
  archlint scan . --config /path/to/.archlint.yaml
  archlint collect . -o - | archlint scan --stdin
  cat architecture.yaml | archlint scan --stdin`,
	Args: cobra.RangeArgs(0, 1),
	RunE: runScan,
}

func init() {
	scanCmd.Flags().StringVar(&scanFormat, "format", "text", "Output format: text or json")
	scanCmd.Flags().IntVar(&scanThreshold, "threshold", -1, "Max violations before failing gate (-1 = any violation fails)")
	scanCmd.Flags().StringVar(&scanConfigFile, "config", "", "Path to .archlint.yaml config file (default: <directory>/.archlint.yaml)")
	scanCmd.Flags().BoolVar(&scanStdin, "stdin", false, "Read architecture YAML graph from stdin instead of analyzing a directory")
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
	// Load .archlint.yaml config.
	var cfg archlintcfg.Config
	var configFile string

	var graph *model.Graph
	var a *analyzer.GoAnalyzer

	if scanStdin {
		// Read YAML graph from stdin.
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading stdin: %v\n", err)
			os.Exit(2)
		}
		var g model.Graph
		if err := yaml.Unmarshal(data, &g); err != nil {
			fmt.Fprintf(os.Stderr, "error parsing YAML from stdin: %v\n", err)
			os.Exit(2)
		}
		graph = &g

		// Load config from --config flag if provided; otherwise use defaults.
		if scanConfigFile != "" {
			cfg = archlintcfg.LoadFile(scanConfigFile)
			configFile = scanConfigFile
		}
	} else {
		if len(args) == 0 {
			fmt.Fprintf(os.Stderr, "error: directory argument required when --stdin is not set\n")
			os.Exit(2)
		}
		codeDir := args[0]

		if _, err := os.Stat(codeDir); os.IsNotExist(err) {
			fmt.Fprintf(os.Stderr, "error: %v: %s\n", errDirNotExist, codeDir)
			os.Exit(2)
		}

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

		if analyzer.DetectRustProject(codeDir) {
			rustAnalyzer := analyzer.NewRustAnalyzer()
			g, err := rustAnalyzer.Analyze(codeDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "analysis error: %v\n", err)
				os.Exit(2)
			}
			graph = g
		} else if analyzer.DetectTypeScriptProject(codeDir) {
			tsAnalyzer := analyzer.NewTypeScriptAnalyzer()
			g, err := tsAnalyzer.Analyze(codeDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "analysis error: %v\n", err)
				os.Exit(2)
			}
			graph = g
		} else {
			a = analyzer.NewGoAnalyzer()
			g, err := a.Analyze(codeDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "analysis error: %v\n", err)
				os.Exit(2)
			}
			graph = g
		}
	}

	// Structural violations (coupling, cycles) — config-aware.
	violations := mcp.DetectAllViolationsWithConfig(graph, &cfg)

	// Per-file SOLID and smell violations (Go projects only).
	var allMetrics map[string]*mcp.FileMetrics
	if a != nil {
		allMetrics = mcp.ComputeAllFileMetrics(a, graph)
	}

	for _, m := range allMetrics {
		// DIP violations — respect config enabled flag.
		if cfg.Rules.DIP.IsEnabled() {
			violations = append(violations, m.DIPViolations...)
		}
		// ISP violations — respect config enabled flag.
		if cfg.Rules.ISP.IsEnabled() {
			violations = append(violations, m.ISPViolations...)
		}
		// SRP violations — respect config enabled flag and exclusions.
		if cfg.Rules.SRP.IsEnabled() {
			for _, v := range m.SRPViolations {
				if !cfg.IsSRPExcluded(v.Target) {
					violations = append(violations, v)
				}
			}
		}

		// God-class violations — respect config enabled flag and exclusions.
		if cfg.Rules.GodClass.IsEnabled() {
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
		if cfg.Rules.HubNode.IsEnabled() {
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
		if cfg.Rules.FeatureEnvy.IsEnabled() {
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
