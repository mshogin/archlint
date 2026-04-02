package cli

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mshogin/archlint/internal/analyzer"
	"github.com/mshogin/archlint/internal/archlintcfg"
	"github.com/mshogin/archlint/internal/mcp"
	"github.com/spf13/cobra"
)

var (
	batchFormat     string
	batchOutput     string
	batchConfigFile string
	batchStdin      bool
)

var batchCmd = &cobra.Command{
	Use:   "batch [directory | dir1 dir2 dir3...]",
	Short: "Scan multiple repositories and produce a consolidated report",
	Long: `Scan multiple Go project directories for architecture violations and
produce a consolidated health report sorted by violation count (worst first).

Input modes:
  - Single parent directory: archlint batch /path/to/repos/
    (scans every immediate subdirectory that contains Go files)
  - Explicit list:           archlint batch dir1 dir2 dir3
  - From stdin:              find /repos -name go.mod -exec dirname {} \; | archlint batch --stdin

Output formats:
  - text  (default) - markdown table
  - json            - structured JSON, for further processing
  - csv             - comma-separated, for Excel import

Health score: 100 - (violations * 2), minimum 0.

Examples:
  archlint batch /home/user/repos/
  archlint batch ./service-a ./service-b --format json
  archlint batch ./service-a --output report.md
  find ~/repos -maxdepth 1 -type d | archlint batch --stdin --format csv --output report.csv`,
	RunE: runBatch,
}

func init() {
	batchCmd.Flags().StringVar(&batchFormat, "format", "text", "Output format: text, json, or csv")
	batchCmd.Flags().StringVar(&batchOutput, "output", "", "Write output to file (default: stdout)")
	batchCmd.Flags().StringVar(&batchConfigFile, "config", "", "Shared .archlint.yaml config for all repos")
	batchCmd.Flags().BoolVar(&batchStdin, "stdin", false, "Read directory paths from stdin (one per line)")
	rootCmd.AddCommand(batchCmd)
}

// batchRepoResult holds the scan result for a single repository.
type batchRepoResult struct {
	Repository  string         `json:"repository"`
	Path        string         `json:"path"`
	Violations  int            `json:"violations"`
	SOLID       int            `json:"solid"`
	GodClass    int            `json:"god_class"`
	FanOut      int            `json:"fan_out"`
	Cycles      int            `json:"cycles"`
	FeatureEnvy int            `json:"feature_envy"`
	Coupling    int            `json:"coupling"`
	Health      int            `json:"health"`
	Error       string         `json:"error,omitempty"`
	Details     []mcp.Violation `json:"details,omitempty"`
}

// batchReport is the full consolidated report.
type batchReport struct {
	TotalRepos  int               `json:"total_repos"`
	ScannedOK   int               `json:"scanned_ok"`
	Errors      int               `json:"errors"`
	AvgHealth   int               `json:"avg_health"`
	Worst5      []string          `json:"worst_5"`
	Results     []batchRepoResult `json:"results"`
}

// calcHealth converts a violation count to a health score.
func calcHealth(violations int) int {
	h := 100 - violations*2
	if h < 0 {
		h = 0
	}
	return h
}

// collectDirs builds the list of directories to scan based on flags/args.
func collectDirs(args []string) ([]string, error) {
	if batchStdin {
		var dirs []string
		scanner := bufio.NewScanner(os.Stdin)
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if line != "" {
				dirs = append(dirs, line)
			}
		}
		if err := scanner.Err(); err != nil {
			return nil, fmt.Errorf("reading stdin: %w", err)
		}
		return dirs, nil
	}

	if len(args) == 0 {
		return nil, fmt.Errorf("provide at least one directory, or use --stdin")
	}

	// If a single argument and it's a directory that does not itself contain
	// Go files at the top level, treat it as a parent directory and enumerate
	// its immediate subdirectories.
	if len(args) == 1 {
		dir := args[0]
		info, err := os.Stat(dir)
		if err != nil {
			return nil, fmt.Errorf("cannot stat %s: %w", dir, err)
		}
		if info.IsDir() && !dirHasGoFiles(dir) {
			return subdirs(dir)
		}
	}

	return args, nil
}

// dirHasGoFiles returns true if dir contains at least one *.go file directly.
func dirHasGoFiles(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".go") {
			return true
		}
	}
	return false
}

// subdirs returns all immediate subdirectories of parent.
func subdirs(parent string) ([]string, error) {
	entries, err := os.ReadDir(parent)
	if err != nil {
		return nil, fmt.Errorf("reading directory %s: %w", parent, err)
	}
	var dirs []string
	for _, e := range entries {
		if e.IsDir() {
			dirs = append(dirs, filepath.Join(parent, e.Name()))
		}
	}
	if len(dirs) == 0 {
		return nil, fmt.Errorf("no subdirectories found in %s", parent)
	}
	return dirs, nil
}

// scanRepo performs a single-repo scan and returns a batchRepoResult.
func scanRepo(dir string, cfg *archlintcfg.Config) batchRepoResult {
	abs, err := filepath.Abs(dir)
	if err != nil {
		abs = dir
	}
	name := filepath.Base(abs)

	res := batchRepoResult{
		Repository: name,
		Path:       abs,
	}

	if _, err := os.Stat(abs); os.IsNotExist(err) {
		res.Error = "directory does not exist"
		res.Health = calcHealth(0)
		return res
	}

	a := analyzer.NewGoAnalyzer()
	graph, err := a.Analyze(abs)
	if err != nil {
		res.Error = fmt.Sprintf("analysis error: %v", err)
		res.Health = calcHealth(0)
		return res
	}

	// Structural violations using config (or defaults when cfg is zero-value).
	var violations []mcp.Violation
	if cfg != nil {
		violations = mcp.DetectAllViolationsWithConfig(graph, cfg)
	} else {
		violations = mcp.DetectAllViolations(graph)
	}

	// Per-file SOLID and smell violations.
	allMetrics := mcp.ComputeAllFileMetrics(a, graph)
	for _, m := range allMetrics {
		violations = append(violations, m.SRPViolations...)
		if cfg == nil || cfg.Rules.DIP.Enabled {
			violations = append(violations, m.DIPViolations...)
		}
		if cfg == nil || cfg.Rules.ISP.Enabled {
			violations = append(violations, m.ISPViolations...)
		}
		for _, gc := range m.GodClasses {
			violations = append(violations, mcp.Violation{Kind: "god-class", Message: "God class detected: " + gc, Target: gc})
		}
		for _, hub := range m.HubNodes {
			violations = append(violations, mcp.Violation{Kind: "hub-node", Message: "Hub node detected: " + hub, Target: hub})
		}
		for _, fe := range m.FeatureEnvy {
			violations = append(violations, mcp.Violation{Kind: "feature-envy", Message: "Feature envy: " + fe, Target: fe})
		}
		for _, ss := range m.ShotgunSurgery {
			violations = append(violations, mcp.Violation{Kind: "shotgun-surgery", Message: "Shotgun surgery risk: " + ss, Target: ss})
		}
	}

	// Categorise.
	for _, v := range violations {
		switch v.Kind {
		case "srp", "dip", "isp":
			res.SOLID++
		case "god-class":
			res.GodClass++
		case "fan-out", "hub-node":
			res.FanOut++
		case "cycle":
			res.Cycles++
		case "feature-envy", "shotgun-surgery":
			res.FeatureEnvy++
		default:
			// high-coupling, layer-violation, etc.
			res.Coupling++
		}
	}

	res.Violations = len(violations)
	res.Health = calcHealth(res.Violations)
	res.Details = violations
	return res
}

// buildReport assembles batchReport from individual results.
func buildReport(results []batchRepoResult) batchReport {
	report := batchReport{
		TotalRepos: len(results),
		Results:    results,
	}

	sumHealth := 0
	for _, r := range results {
		if r.Error == "" {
			report.ScannedOK++
			sumHealth += r.Health
		} else {
			report.Errors++
		}
	}
	if report.ScannedOK > 0 {
		report.AvgHealth = sumHealth / report.ScannedOK
	}

	// Worst-5: already sorted by violations desc.
	limit := 5
	if len(results) < limit {
		limit = len(results)
	}
	for i := 0; i < limit; i++ {
		if results[i].Error == "" && results[i].Violations > 0 {
			report.Worst5 = append(report.Worst5, results[i].Repository)
		}
	}

	return report
}

func runBatch(cmd *cobra.Command, args []string) error {
	dirs, err := collectDirs(args)
	if err != nil {
		return err
	}

	// Load shared config if provided; otherwise nil = use defaults.
	var cfg *archlintcfg.Config
	if batchConfigFile != "" {
		c := archlintcfg.LoadFile(batchConfigFile)
		cfg = &c
	}

	// Scan each directory.
	results := make([]batchRepoResult, 0, len(dirs))
	for _, d := range dirs {
		fmt.Fprintf(os.Stderr, "scanning %s...\n", d)
		var repoCfg *archlintcfg.Config
		if cfg != nil {
			repoCfg = cfg
		}
		results = append(results, scanRepo(d, repoCfg))
	}

	// Sort: worst (most violations) first; errors at the end.
	sort.Slice(results, func(i, j int) bool {
		if results[i].Error != "" && results[j].Error == "" {
			return false
		}
		if results[i].Error == "" && results[j].Error != "" {
			return true
		}
		return results[i].Violations > results[j].Violations
	})

	report := buildReport(results)

	// Choose output writer.
	out := os.Stdout
	if batchOutput != "" {
		//nolint:gosec // G304: batchOutput is a CLI argument
		f, err := os.OpenFile(batchOutput, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o640)
		if err != nil {
			return fmt.Errorf("cannot open output file: %w", err)
		}
		defer f.Close()
		out = f
	}

	switch batchFormat {
	case "json":
		enc := json.NewEncoder(out)
		enc.SetIndent("", "  ")
		return enc.Encode(report)

	case "csv":
		return writeBatchCSV(out, report)

	case "text", "":
		return writeBatchMarkdown(out, report)

	default:
		return fmt.Errorf("unknown format: %s (use text, json, or csv)", batchFormat)
	}
}

func writeBatchMarkdown(out *os.File, report batchReport) error {
	fmt.Fprintf(out, "# Architecture Health Report\n\n")
	fmt.Fprintf(out, "Total repos: %d | Scanned OK: %d | Errors: %d | Avg health: %d/100\n\n",
		report.TotalRepos, report.ScannedOK, report.Errors, report.AvgHealth)

	// Table header.
	fmt.Fprintf(out, "| Repository | Violations | SOLID | God-class | Fan-out | Cycles | Feature-envy | Coupling | Health |\n")
	fmt.Fprintf(out, "|------------|-----------|-------|-----------|---------|--------|-------------|----------|--------|\n")

	for _, r := range report.Results {
		if r.Error != "" {
			fmt.Fprintf(out, "| %s | ERROR | - | - | - | - | - | - | - |\n", r.Repository)
			continue
		}
		fmt.Fprintf(out, "| %s | %d | %d | %d | %d | %d | %d | %d | %d/100 |\n",
			r.Repository, r.Violations, r.SOLID, r.GodClass, r.FanOut, r.Cycles, r.FeatureEnvy, r.Coupling, r.Health)
	}

	// Summary section.
	fmt.Fprintf(out, "\n## Summary\n\n")
	fmt.Fprintf(out, "- Total repositories scanned: %d\n", report.ScannedOK)
	fmt.Fprintf(out, "- Average health score: %d/100\n", report.AvgHealth)

	if len(report.Worst5) > 0 {
		fmt.Fprintf(out, "- Worst repos (most violations): %s\n", strings.Join(report.Worst5, ", "))
	}

	if report.Errors > 0 {
		fmt.Fprintf(out, "\n### Scan errors\n\n")
		for _, r := range report.Results {
			if r.Error != "" {
				fmt.Fprintf(out, "- %s: %s\n", r.Repository, r.Error)
			}
		}
	}

	return nil
}

func writeBatchCSV(out *os.File, report batchReport) error {
	w := csv.NewWriter(out)
	defer w.Flush()

	header := []string{"Repository", "Path", "Violations", "SOLID", "God-class", "Fan-out", "Cycles", "Feature-envy", "Coupling", "Health", "Error"}
	if err := w.Write(header); err != nil {
		return err
	}

	for _, r := range report.Results {
		row := []string{
			r.Repository,
			r.Path,
			fmt.Sprintf("%d", r.Violations),
			fmt.Sprintf("%d", r.SOLID),
			fmt.Sprintf("%d", r.GodClass),
			fmt.Sprintf("%d", r.FanOut),
			fmt.Sprintf("%d", r.Cycles),
			fmt.Sprintf("%d", r.FeatureEnvy),
			fmt.Sprintf("%d", r.Coupling),
			fmt.Sprintf("%d", r.Health),
			r.Error,
		}
		if err := w.Write(row); err != nil {
			return err
		}
	}

	return w.Error()
}
