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

var responsibilityFormat string

var responsibilityCmd = &cobra.Command{
	Use:   "responsibility [directory]",
	Short: "Analyze types for reach-based SRP metric (responsibility count ρ)",
	Long: `Analyze all struct types in a Go project and compute the reach-based SRP
metric ρ (rho) alongside LCOM4.

ρ = 1  → single responsibility (OK)
ρ >= 2 → multiple responsibilities detected (consider splitting)
ρ = 0  → no methods (trivial type, skipped)

Output is sorted by ρ descending (worst offenders first).

Examples:
  archlint responsibility .
  archlint responsibility ./internal --json`,
	Args: cobra.ExactArgs(1),
	RunE: runResponsibility,
}

func init() {
	responsibilityCmd.Flags().StringVar(&responsibilityFormat, "format", "text", "Output format: text or json")
	responsibilityCmd.Flags().BoolP("json", "", false, "Shorthand for --format json")
	rootCmd.AddCommand(responsibilityCmd)
}

// responsibilityRow is a single row in the responsibility table.
type responsibilityRow struct {
	Type            string     `json:"type"`
	Rho             int        `json:"rho"`
	LCOM4           int        `json:"lcom4"`
	PureMethodCount int        `json:"pureMethodCount"`
	Verdict         string     `json:"verdict"`
	Classes         [][]string `json:"classes,omitempty"`
}

func runResponsibility(cmd *cobra.Command, args []string) error {
	codeDir := args[0]

	if _, err := os.Stat(codeDir); os.IsNotExist(err) {
		return fmt.Errorf("%w: %s", errDirNotExist, codeDir)
	}

	// Honour --json shorthand.
	jsonFlag, _ := cmd.Flags().GetBool("json")
	if jsonFlag {
		responsibilityFormat = "json"
	}

	a := analyzer.NewGoAnalyzer()

	graph, err := a.Analyze(codeDir)
	if err != nil {
		return fmt.Errorf("analysis error: %w", err)
	}

	// Collect results for all struct types.
	var rows []responsibilityRow

	for typeID, t := range a.AllTypes() {
		if t.Kind != "struct" {
			continue
		}

		result := mcp.ComputeReachSRP(a, typeID, graph)
		if result.Rho == 0 {
			// Skip trivial types with no methods.
			continue
		}

		row := responsibilityRow{
			Type:            typeID,
			Rho:             result.Rho,
			LCOM4:           result.LCOM4,
			PureMethodCount: result.PureMethodCount,
			Classes:         result.Classes,
			Verdict:         buildVerdict(result),
		}

		rows = append(rows, row)
	}

	// Sort by ρ descending, then by type name for stable output.
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Rho != rows[j].Rho {
			return rows[i].Rho > rows[j].Rho
		}
		return rows[i].Type < rows[j].Type
	})

	switch responsibilityFormat {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rows); err != nil {
			return fmt.Errorf("JSON encoding error: %w", err)
		}
	case "text":
		if len(rows) == 0 {
			fmt.Println("No struct types found.")
			return nil
		}

		printResponsibilityTable(rows)
	default:
		return fmt.Errorf("unknown format: %s (use text or json)", responsibilityFormat)
	}

	return nil
}

// buildVerdict produces a human-readable verdict string.
func buildVerdict(r *mcp.ReachSRPResult) string {
	if r.Rho <= 1 {
		if r.PureMethodCount > 0 && r.LCOM4 > 1 {
			return fmt.Sprintf("OK (pure data carrier, LCOM4=%d is misleading)", r.LCOM4)
		}
		return "OK"
	}

	// Build class summary: list first method of each class.
	summary := ""
	for i, cls := range r.Classes {
		if i > 0 {
			summary += " / "
		}
		if len(cls) == 0 {
			continue
		}
		// Show up to 3 method names per class.
		shown := cls
		if len(shown) > 3 {
			shown = append(shown[:3], "...")
		}
		summary += "{"
		for j, m := range shown {
			if j > 0 {
				summary += ","
			}
			summary += m
		}
		summary += "}"
	}

	return fmt.Sprintf("SPLIT: %s", summary)
}

// printResponsibilityTable renders the result as a formatted text table.
func printResponsibilityTable(rows []responsibilityRow) {
	// Determine column widths.
	typeWidth := len("Type")
	for _, r := range rows {
		if len(r.Type) > typeWidth {
			typeWidth = len(r.Type)
		}
	}
	if typeWidth > 60 {
		typeWidth = 60
	}

	verdictWidth := len("Verdict")
	for _, r := range rows {
		if len(r.Verdict) > verdictWidth {
			verdictWidth = len(r.Verdict)
		}
	}
	if verdictWidth > 80 {
		verdictWidth = 80
	}

	// Header.
	fmt.Printf("%-*s | %-5s | %-5s | %s\n", typeWidth, "Type", "rho", "LCOM4", "Verdict")
	fmt.Printf("%s-+-%s-+-%s-+-%s\n",
		repeat("-", typeWidth),
		repeat("-", 5),
		repeat("-", 5),
		repeat("-", verdictWidth))

	for _, r := range rows {
		typeTrunc := r.Type
		if len(typeTrunc) > typeWidth {
			typeTrunc = "..." + typeTrunc[len(typeTrunc)-typeWidth+3:]
		}

		verdictTrunc := r.Verdict
		if len(verdictTrunc) > verdictWidth {
			verdictTrunc = verdictTrunc[:verdictWidth-3] + "..."
		}

		fmt.Printf("%-*s | %-5d | %-5d | %s\n",
			typeWidth, typeTrunc, r.Rho, r.LCOM4, verdictTrunc)
	}

	// Summary line.
	violations := 0
	for _, r := range rows {
		if r.Rho >= 2 {
			violations++
		}
	}
	fmt.Printf("\nTotal: %d types analyzed, %d with ρ >= 2\n", len(rows), violations)
}

// repeat returns a string of n copies of s.
func repeat(s string, n int) string {
	result := ""
	for i := 0; i < n; i++ {
		result += s
	}
	return result
}
