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

var metricsFormat string

var metricsCmd = &cobra.Command{
	Use:   "metrics [directory]",
	Short: "Print coupling and complexity metrics per package",
	Long: `Analyze Go source code and print architecture metrics for each file:
coupling (Ca/Ce/instability/abstractness), size, fan-in/out, SOLID violations, code smells, and health score.

Examples:
  archlint metrics .
  archlint metrics ./internal --format json
  archlint metrics . --format text`,
	Args: cobra.ExactArgs(1),
	RunE: runMetrics,
}

func init() {
	metricsCmd.Flags().StringVar(&metricsFormat, "format", "text", "Output format: text or json")
	rootCmd.AddCommand(metricsCmd)
}

// packageMetrics aggregates file metrics at the package level for output.
type packageMetrics struct {
	Package          string  `json:"package"`
	Files            int     `json:"files"`
	AfferentCoupling int     `json:"afferentCoupling"`
	EfferentCoupling int     `json:"efferentCoupling"`
	Instability      float64 `json:"instability"`
	Abstractness     float64 `json:"abstractness"`
	MainSeqDistance  float64 `json:"mainSeqDistance"`
	Types            int     `json:"types"`
	Functions        int     `json:"functions"`
	Methods          int     `json:"methods"`
	Fields           int     `json:"fields"`
	FanIn            int     `json:"fanIn"`
	FanOut           int     `json:"fanOut"`
	HealthScore      int     `json:"healthScore"`
	Violations       int     `json:"violations"`
	Smells           int     `json:"smells"`
}

type metricsResult struct {
	Packages []packageMetrics `json:"packages"`
}

func runMetrics(cmd *cobra.Command, args []string) error {
	codeDir := args[0]

	if _, err := os.Stat(codeDir); os.IsNotExist(err) {
		return fmt.Errorf("%w: %s", errDirNotExist, codeDir)
	}

	a := analyzer.NewGoAnalyzer()

	graph, err := a.Analyze(codeDir)
	if err != nil {
		return fmt.Errorf("analysis error: %w", err)
	}

	allMetrics := mcp.ComputeAllFileMetrics(a, graph)

	// Aggregate by package.
	pkgMap := make(map[string]*packageMetrics)

	for _, m := range allMetrics {
		pkg := m.Package
		if pkg == "" {
			continue
		}

		pm, ok := pkgMap[pkg]
		if !ok {
			pm = &packageMetrics{
				Package:          pkg,
				AfferentCoupling: m.AfferentCoupling,
				EfferentCoupling: m.EfferentCoupling,
				Instability:      m.Instability,
				Abstractness:     m.Abstractness,
				MainSeqDistance:  m.MainSeqDistance,
			}
			pkgMap[pkg] = pm
		}

		pm.Files++
		pm.Types += m.Types
		pm.Functions += m.Functions
		pm.Methods += m.Methods
		pm.Fields += m.Fields
		pm.FanIn += m.FanIn
		pm.FanOut += m.FanOut
		pm.HealthScore += m.HealthScore
		pm.Violations += len(m.SRPViolations) + len(m.DIPViolations) + len(m.ISPViolations)
		pm.Smells += len(m.GodClasses) + len(m.HubNodes) + len(m.FeatureEnvy) + len(m.ShotgunSurgery)
	}

	// Average health score per package.
	packages := make([]packageMetrics, 0, len(pkgMap))
	for _, pm := range pkgMap {
		if pm.Files > 0 {
			pm.HealthScore = pm.HealthScore / pm.Files
		}
		packages = append(packages, *pm)
	}

	sort.Slice(packages, func(i, j int) bool {
		return packages[i].Package < packages[j].Package
	})

	switch metricsFormat {
	case "json":
		result := metricsResult{Packages: packages}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(result); err != nil {
			return fmt.Errorf("JSON encoding error: %w", err)
		}
	case "text":
		if len(packages) == 0 {
			fmt.Println("No packages found.")
			return nil
		}

		fmt.Printf("=== Package Metrics (%d packages) ===\n\n", len(packages))

		for _, pm := range packages {
			fmt.Printf("%s\n", pm.Package)
			fmt.Printf("  Coupling:    Ca=%-3d Ce=%-3d I=%.2f A=%.2f D=%.2f\n",
				pm.AfferentCoupling, pm.EfferentCoupling,
				pm.Instability, pm.Abstractness, pm.MainSeqDistance)
			fmt.Printf("  Size:        types=%-3d functions=%-3d methods=%-3d fields=%-3d\n",
				pm.Types, pm.Functions, pm.Methods, pm.Fields)
			fmt.Printf("  Fan:         in=%-3d out=%-3d\n", pm.FanIn, pm.FanOut)
			fmt.Printf("  Health:      %d/100\n", pm.HealthScore)
			if pm.Violations > 0 || pm.Smells > 0 {
				fmt.Printf("  Issues:      violations=%d smells=%d\n", pm.Violations, pm.Smells)
			}
			fmt.Println()
		}
	default:
		return fmt.Errorf("unknown format: %s (use text or json)", metricsFormat)
	}

	return nil
}
