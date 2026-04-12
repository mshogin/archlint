package cli

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mshogin/archlint/internal/analyzer"
	"github.com/mshogin/archlint/internal/archlintcfg"
	"github.com/spf13/cobra"
)

var (
	contractsFormat  string
	contractsOrphans bool
	contractsUnused  bool
)

var contractsCmd = &cobra.Command{
	Use:   "contracts [directory]",
	Short: "Analyze external contract boundaries in the architecture graph",
	Long: `Load external_contracts from .archlint.yaml and cross-reference them with
the architecture graph to identify:

  - OK:     contract module exists and has internal dependents
  - UNUSED: contract module exists but no internal nodes depend on it
  - ORPHAN: contract module not found in the graph at all

External contracts are declared in .archlint.yaml:

  external_contracts:
    - name: user_api
      module: src::api::users
      type: rest
    - name: bus_events
      module: src::infra::unix_bus
      type: stream
      schema: schemas/bus.proto

Output format (text):

  External Contracts (5 total, 1 orphan, 1 unused):

    agent_list     query    src::app::workflow      3 dependents  OK
    bus_events     stream   src::infra::unix_bus    0 dependents  UNUSED
    old_endpoint   rpc      src::legacy::v1         -             ORPHAN

Examples:
  archlint contracts .
  archlint contracts . --format json
  archlint contracts . --orphans
  archlint contracts . --unused`,
	Args: cobra.ExactArgs(1),
	RunE: runContracts,
}

func init() {
	contractsCmd.Flags().StringVar(&contractsFormat, "format", "text", "Output format: text or json")
	contractsCmd.Flags().BoolVar(&contractsOrphans, "orphans", false, "Show only orphan contracts (module not found in graph)")
	contractsCmd.Flags().BoolVar(&contractsUnused, "unused", false, "Show only unused contracts (no internal dependents)")
	rootCmd.AddCommand(contractsCmd)
}

// contractJSONEntry is a single contract entry in JSON output.
type contractJSONEntry struct {
	Name         string `json:"name"`
	Type         string `json:"type"`
	Module       string `json:"module"`
	Schema       string `json:"schema,omitempty"`
	Dependents   int    `json:"dependents"`
	Dependencies int    `json:"dependencies"`
	Status       string `json:"status"` // "ok", "unused", "orphan"
}

// contractsJSONResult is the top-level JSON output structure.
type contractsJSONResult struct {
	Total   int                 `json:"total"`
	Orphans int                 `json:"orphans"`
	Unused  int                 `json:"unused"`
	Entries []contractJSONEntry `json:"entries"`
}

func runContracts(cmd *cobra.Command, args []string) error {
	codeDir := args[0]

	if _, err := os.Stat(codeDir); os.IsNotExist(err) {
		return fmt.Errorf("%w: %s", errDirNotExist, codeDir)
	}

	// Resolve and load config.
	absDir, err := filepath.Abs(codeDir)
	if err != nil {
		absDir = codeDir
	}
	cfg := archlintcfg.Load(absDir)

	// Analyze the codebase to build the graph.
	var graph interface{ /* *model.Graph */ } = nil
	_ = graph // used below via typed var

	var graphResult *analyzer.ContractAnalysis

	if analyzer.DetectTypeScriptProject(codeDir) {
		tsAnalyzer := analyzer.NewTypeScriptAnalyzer()
		g, analysisErr := tsAnalyzer.Analyze(codeDir)
		if analysisErr != nil {
			return fmt.Errorf("analysis error: %w", analysisErr)
		}
		graphResult = analyzer.AnalyzeContracts(g, cfg.ExternalContracts)
	} else {
		goAnalyzer := analyzer.NewGoAnalyzer()
		g, analysisErr := goAnalyzer.Analyze(codeDir)
		if analysisErr != nil {
			return fmt.Errorf("analysis error: %w", analysisErr)
		}
		graphResult = analyzer.AnalyzeContracts(g, cfg.ExternalContracts)
	}

	// Build status lookup sets for quick membership test.
	orphanSet := make(map[string]bool, len(graphResult.OrphanContracts))
	for _, name := range graphResult.OrphanContracts {
		orphanSet[name] = true
	}
	unusedSet := make(map[string]bool, len(graphResult.UnusedContracts))
	for _, name := range graphResult.UnusedContracts {
		unusedSet[name] = true
	}

	// Apply filters.
	contracts := graphResult.Contracts
	if contractsOrphans {
		filtered := contracts[:0]
		for _, c := range contracts {
			if orphanSet[c.Name] {
				filtered = append(filtered, c)
			}
		}
		contracts = filtered
	} else if contractsUnused {
		filtered := contracts[:0]
		for _, c := range contracts {
			if unusedSet[c.Name] {
				filtered = append(filtered, c)
			}
		}
		contracts = filtered
	}

	total := len(graphResult.Contracts)
	orphanCount := len(graphResult.OrphanContracts)
	unusedCount := len(graphResult.UnusedContracts)

	switch contractsFormat {
	case "json":
		entries := make([]contractJSONEntry, 0, len(contracts))
		for _, c := range contracts {
			status := "ok"
			if orphanSet[c.Name] {
				status = "orphan"
			} else if unusedSet[c.Name] {
				status = "unused"
			}
			entries = append(entries, contractJSONEntry{
				Name:         c.Name,
				Type:         c.Type,
				Module:       c.Module,
				Schema:       c.Schema,
				Dependents:   c.Dependents,
				Dependencies: c.Dependencies,
				Status:       status,
			})
		}
		result := contractsJSONResult{
			Total:   total,
			Orphans: orphanCount,
			Unused:  unusedCount,
			Entries: entries,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(result)

	case "text":
		if total == 0 {
			fmt.Println("No external contracts defined in .archlint.yaml.")
			fmt.Println("Add an external_contracts section to declare API boundaries.")
			return nil
		}

		fmt.Printf("External Contracts (%d total, %d orphan, %d unused):\n\n",
			total, orphanCount, unusedCount)

		for _, c := range contracts {
			status := "OK"
			dependentsStr := fmt.Sprintf("%d dependents", c.Dependents)
			if orphanSet[c.Name] {
				status = "ORPHAN (module not found)"
				dependentsStr = "-"
			} else if unusedSet[c.Name] {
				status = "UNUSED"
			}
			fmt.Printf("  %-18s %-8s %-30s %-16s %s\n",
				c.Name, c.Type, c.Module, dependentsStr, status)
		}
		fmt.Println()

	default:
		return fmt.Errorf("unknown format: %s (use text or json)", contractsFormat)
	}

	return nil
}
