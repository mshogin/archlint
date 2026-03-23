package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"

	"github.com/mshogin/archlint/internal/analyzer"
	"github.com/mshogin/archlint/internal/mcp"
	"github.com/spf13/cobra"
)

var checkFormat string

var checkCmd = &cobra.Command{
	Use:   "check [directory]",
	Short: "Check for architecture violations",
	Long: `Analyze Go source code and report architecture violations:
SOLID violations, circular dependencies, high coupling, god classes, and code smells.

Examples:
  archlint check .
  archlint check ./internal --format json
  archlint check . --format text`,
	Args: cobra.ExactArgs(1),
	RunE: runCheck,
}

func init() {
	checkCmd.Flags().StringVar(&checkFormat, "format", "text", "Output format: text or json")
	rootCmd.AddCommand(checkCmd)
}

// checkResult is the JSON output structure for the check command.
type checkResult struct {
	Violations []mcp.Violation `json:"violations"`
	Total      int             `json:"total"`
}

func runCheck(cmd *cobra.Command, args []string) error {
	codeDir := args[0]

	if _, err := os.Stat(codeDir); os.IsNotExist(err) {
		return fmt.Errorf("%w: %s", errDirNotExist, codeDir)
	}

	a := analyzer.NewGoAnalyzer()

	graph, err := a.Analyze(codeDir)
	if err != nil {
		return fmt.Errorf("analysis error: %w", err)
	}

	// Structural violations (coupling, cycles).
	violations := mcp.DetectAllViolations(graph)

	// Per-file SOLID and smell violations.
	allMetrics := mcp.ComputeAllFileMetrics(a, graph)

	for _, m := range allMetrics {
		violations = append(violations, m.SRPViolations...)
		violations = append(violations, m.DIPViolations...)
		violations = append(violations, m.ISPViolations...)

		for _, gc := range m.GodClasses {
			violations = append(violations, mcp.Violation{
				Kind:    "god-class",
				Message: fmt.Sprintf("God class detected: %s", gc),
				Target:  gc,
			})
		}

		for _, hub := range m.HubNodes {
			violations = append(violations, mcp.Violation{
				Kind:    "hub-node",
				Message: fmt.Sprintf("Hub node detected: %s", hub),
				Target:  hub,
			})
		}

		for _, fe := range m.FeatureEnvy {
			violations = append(violations, mcp.Violation{
				Kind:    "feature-envy",
				Message: fmt.Sprintf("Feature envy: %s", fe),
				Target:  fe,
			})
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

	switch checkFormat {
	case "json":
		result := checkResult{
			Violations: violations,
			Total:      len(violations),
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			return fmt.Errorf("JSON encoding error: %w", err)
		}
	case "text":
		if len(violations) == 0 {
			fmt.Println("No violations found.")
			return nil
		}

		fmt.Printf("=== Violations (%d found) ===\n\n", len(violations))

		for _, v := range violations {
			fmt.Printf("[%s] %s\n", v.Kind, v.Message)
			if v.Target != "" {
				fmt.Printf("  target: %s\n", v.Target)
			}
			fmt.Println()
		}
	default:
		return fmt.Errorf("unknown format: %s (use text or json)", checkFormat)
	}

	if len(violations) > 0 {
		os.Exit(1)
	}

	return nil
}
