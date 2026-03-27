package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/mshogin/archlint/internal/analyzer"
	"github.com/mshogin/archlint/internal/mcp"
	"github.com/spf13/cobra"
)

var selfScanFormat string

var selfScanCmd = &cobra.Command{
	Use:   "self-scan",
	Short: "Run archlint on its own source code and display a health dashboard",
	Long: `Run a full architecture scan on archlint's own source code.
Displays components, links, violations, fan-out, cycles, and overall health score.

Examples:
  archlint self-scan
  archlint self-scan --format markdown`,
	Args: cobra.NoArgs,
	RunE: runSelfScan,
}

func init() {
	selfScanCmd.Flags().StringVar(&selfScanFormat, "format", "text", "Output format: text or markdown")
	rootCmd.AddCommand(selfScanCmd)
}

// selfScanResult holds aggregated self-scan data.
type selfScanResult struct {
	SourceDir   string
	Components  int
	Links       int
	Packages    int
	Structs     int
	Interfaces  int
	Functions   int
	Methods     int
	External    int
	Violations  int
	Cycles      int
	MaxFanOut   int
	HealthScore int
	TopViolations []string
	PackageScores []packageScore
}

type packageScore struct {
	Name        string
	HealthScore int
	Violations  int
	FanOut      int
}

func runSelfScan(_ *cobra.Command, _ []string) error {
	// Locate archlint source root: go up from this file's runtime location.
	sourceDir, err := findSelfSourceDir()
	if err != nil {
		return fmt.Errorf("cannot locate archlint source: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Scanning: %s\n", sourceDir)

	a := analyzer.NewGoAnalyzer()

	graph, err := a.Analyze(sourceDir)
	if err != nil {
		return fmt.Errorf("analysis error: %w", err)
	}

	// Collect component type counts.
	stats := make(map[string]int)
	for _, node := range graph.Nodes {
		stats[node.Entity]++
	}

	// Detect structural violations (cycles, high coupling).
	violations := mcp.DetectAllViolations(graph)

	// Compute per-file metrics.
	allMetrics := mcp.ComputeAllFileMetrics(a, graph)

	// Aggregate by package.
	pkgMap := make(map[string]*packageScore)
	totalHealth := 0
	fileCount := 0
	maxFanOut := 0

	for _, m := range allMetrics {
		pkg := m.Package
		if pkg == "" {
			continue
		}

		ps, ok := pkgMap[pkg]
		if !ok {
			ps = &packageScore{Name: pkg}
			pkgMap[pkg] = ps
		}

		vCount := len(m.SRPViolations) + len(m.DIPViolations) + len(m.ISPViolations) +
			len(m.GodClasses) + len(m.HubNodes) + len(m.FeatureEnvy) + len(m.ShotgunSurgery)
		ps.Violations += vCount
		ps.HealthScore += m.HealthScore
		ps.FanOut += m.FanOut
		fileCount++

		if m.FanOut > maxFanOut {
			maxFanOut = m.FanOut
		}

		totalHealth += m.HealthScore

		// Collect SOLID + smell violations for display.
		for _, v := range m.SRPViolations {
			violations = append(violations, v)
		}
		for _, v := range m.DIPViolations {
			violations = append(violations, v)
		}
		for _, v := range m.ISPViolations {
			violations = append(violations, v)
		}
	}

	// Average health score per package.
	pkgCounts := make(map[string]int)
	for _, m := range allMetrics {
		if m.Package != "" {
			pkgCounts[m.Package]++
		}
	}
	for pkg, ps := range pkgMap {
		if c := pkgCounts[pkg]; c > 0 {
			ps.HealthScore = ps.HealthScore / c
		}
	}

	// Overall health score.
	overallHealth := 100
	if fileCount > 0 {
		overallHealth = totalHealth / fileCount
	}

	// Count cycles.
	cycles := 0
	for _, v := range violations {
		if v.Kind == "circular-dependency" {
			cycles++
		}
	}

	// Top violations (up to 5, deduplicated).
	seen := make(map[string]bool)
	var topViolations []string
	for _, v := range violations {
		msg := fmt.Sprintf("[%s] %s", v.Kind, v.Message)
		if !seen[msg] && len(topViolations) < 5 {
			seen[msg] = true
			topViolations = append(topViolations, msg)
		}
	}

	// Package scores sorted by health ascending (worst first).
	packages := make([]packageScore, 0, len(pkgMap))
	for _, ps := range pkgMap {
		packages = append(packages, *ps)
	}
	sort.Slice(packages, func(i, j int) bool {
		return packages[i].HealthScore < packages[j].HealthScore
	})

	result := &selfScanResult{
		SourceDir:     sourceDir,
		Components:    len(graph.Nodes),
		Links:         len(graph.Edges),
		Packages:      stats["package"],
		Structs:       stats["struct"],
		Interfaces:    stats["interface"],
		Functions:     stats["function"],
		Methods:       stats["method"],
		External:      stats["external"],
		Violations:    len(violations),
		Cycles:        cycles,
		MaxFanOut:     maxFanOut,
		HealthScore:   overallHealth,
		TopViolations: topViolations,
		PackageScores: packages,
	}

	switch selfScanFormat {
	case "markdown":
		printSelfScanMarkdown(result)
	default:
		printSelfScanText(result)
	}

	return nil
}

// findSelfSourceDir locates the archlint module root.
// It walks up from the executable or working directory looking for go.mod with module "github.com/mshogin/archlint".
func findSelfSourceDir() (string, error) {
	// Try working directory first (common case when running from source).
	cwd, err := os.Getwd()
	if err == nil {
		if dir := findModuleRoot(cwd); dir != "" {
			return dir, nil
		}
	}

	// Try executable path.
	exe, err := os.Executable()
	if err == nil {
		dir := filepath.Dir(exe)
		if found := findModuleRoot(dir); found != "" {
			return found, nil
		}
	}

	// Try runtime caller path (only works when built with debug info).
	_, file, _, ok := runtime.Caller(0)
	if ok {
		dir := filepath.Dir(file)
		if found := findModuleRoot(dir); found != "" {
			return found, nil
		}
	}

	return "", fmt.Errorf("archlint source root not found (no go.mod with module github.com/mshogin/archlint)")
}

// findModuleRoot walks up the directory tree looking for go.mod containing the archlint module.
func findModuleRoot(start string) string {
	dir := start
	for {
		modFile := filepath.Join(dir, "go.mod")
		if data, err := os.ReadFile(modFile); err == nil { //nolint:gosec // G304: internal path walk
			if strings.Contains(string(data), "github.com/mshogin/archlint") {
				return dir
			}
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}

		dir = parent
	}

	return ""
}

func healthLabel(score int) string {
	switch {
	case score >= 80:
		return "GOOD"
	case score >= 60:
		return "FAIR"
	case score >= 40:
		return "POOR"
	default:
		return "CRITICAL"
	}
}

func printSelfScanText(r *selfScanResult) {
	fmt.Println("=== archlint Self-Scan Dashboard ===")
	fmt.Println()

	fmt.Println("--- Components ---")
	fmt.Printf("  Total:      %d components, %d links\n", r.Components, r.Links)
	fmt.Printf("  Packages:   %d\n", r.Packages)
	fmt.Printf("  Structs:    %d\n", r.Structs)
	fmt.Printf("  Interfaces: %d\n", r.Interfaces)
	fmt.Printf("  Functions:  %d\n", r.Functions)
	fmt.Printf("  Methods:    %d\n", r.Methods)
	if r.External > 0 {
		fmt.Printf("  External:   %d\n", r.External)
	}
	fmt.Println()

	fmt.Println("--- Quality ---")
	fmt.Printf("  Violations: %d\n", r.Violations)
	fmt.Printf("  Cycles:     %d\n", r.Cycles)
	fmt.Printf("  Max fan-out:%d\n", r.MaxFanOut)
	fmt.Printf("  Health:     %d/100 (%s)\n", r.HealthScore, healthLabel(r.HealthScore))
	fmt.Println()

	if len(r.TopViolations) > 0 {
		fmt.Println("--- Top Violations ---")
		for _, v := range r.TopViolations {
			fmt.Printf("  %s\n", v)
		}
		fmt.Println()
	}

	if len(r.PackageScores) > 0 {
		fmt.Println("--- Package Health (worst first) ---")
		for _, ps := range r.PackageScores {
			bar := healthBar(ps.HealthScore)
			fmt.Printf("  %-35s %s %d/100", ps.Name, bar, ps.HealthScore)
			if ps.Violations > 0 {
				fmt.Printf("  violations=%d", ps.Violations)
			}
			fmt.Println()
		}
		fmt.Println()
	}
}

func printSelfScanMarkdown(r *selfScanResult) {
	fmt.Println("# archlint Self-Scan Dashboard")
	fmt.Println()
	fmt.Printf("Source: `%s`\n", r.SourceDir)
	fmt.Println()

	fmt.Println("## Components")
	fmt.Println()
	fmt.Printf("| Metric | Value |\n")
	fmt.Printf("|--------|-------|\n")
	fmt.Printf("| Total components | %d |\n", r.Components)
	fmt.Printf("| Total links | %d |\n", r.Links)
	fmt.Printf("| Packages | %d |\n", r.Packages)
	fmt.Printf("| Structs | %d |\n", r.Structs)
	fmt.Printf("| Interfaces | %d |\n", r.Interfaces)
	fmt.Printf("| Functions | %d |\n", r.Functions)
	fmt.Printf("| Methods | %d |\n", r.Methods)
	if r.External > 0 {
		fmt.Printf("| External | %d |\n", r.External)
	}
	fmt.Println()

	fmt.Println("## Quality")
	fmt.Println()
	fmt.Printf("| Metric | Value |\n")
	fmt.Printf("|--------|-------|\n")
	fmt.Printf("| Violations | %d |\n", r.Violations)
	fmt.Printf("| Cycles | %d |\n", r.Cycles)
	fmt.Printf("| Max fan-out | %d |\n", r.MaxFanOut)
	fmt.Printf("| Health score | %d/100 (%s) |\n", r.HealthScore, healthLabel(r.HealthScore))
	fmt.Println()

	if len(r.TopViolations) > 0 {
		fmt.Println("## Top Violations")
		fmt.Println()
		for _, v := range r.TopViolations {
			fmt.Printf("- %s\n", v)
		}
		fmt.Println()
	}

	if len(r.PackageScores) > 0 {
		fmt.Println("## Package Health")
		fmt.Println()
		fmt.Printf("| Package | Health | Violations | Fan-out |\n")
		fmt.Printf("|---------|--------|------------|---------|\n")
		for _, ps := range r.PackageScores {
			fmt.Printf("| `%s` | %d/100 | %d | %d |\n", ps.Name, ps.HealthScore, ps.Violations, ps.FanOut)
		}
		fmt.Println()
	}
}

// healthBar returns a simple ASCII progress bar for the health score.
func healthBar(score int) string {
	const width = 10
	filled := score * width / 100
	bar := strings.Repeat("#", filled) + strings.Repeat(".", width-filled)
	return "[" + bar + "]"
}
