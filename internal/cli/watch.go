package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/mshogin/archlint/internal/analyzer"
	"github.com/mshogin/archlint/internal/archlintcfg"
	"github.com/mshogin/archlint/internal/mcp"
	"github.com/spf13/cobra"
)

var watchCmd = &cobra.Command{
	Use:   "watch [directory]",
	Short: "Watch directory for .go file changes and re-run scan",
	Long: `Watch a directory for .go file changes and re-run architecture scan on each change.
Shows violations in real-time. Press Ctrl+C to stop.

Examples:
  archlint watch .
  archlint watch ./internal/handler/`,
	Args: cobra.ExactArgs(1),
	RunE: runWatch,
}

func init() {
	rootCmd.AddCommand(watchCmd)
}

func runWatch(cmd *cobra.Command, args []string) error {
	dir := args[0]

	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return fmt.Errorf("%w: %s", errDirNotExist, dir)
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		absDir = dir
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("failed to create watcher: %w", err)
	}
	defer watcher.Close()

	// Add directory recursively.
	if err := addDirRecursive(watcher, absDir); err != nil {
		return fmt.Errorf("failed to watch directory: %w", err)
	}

	fmt.Printf("[archlint] Watching %s ...\n", dir)

	// Run initial scan.
	runWatchScan(absDir)

	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				return nil
			}

			// Only react to .go files.
			if filepath.Ext(event.Name) != ".go" {
				continue
			}

			if event.Has(fsnotify.Write) || event.Has(fsnotify.Create) || event.Has(fsnotify.Remove) {
				rel, relErr := filepath.Rel(absDir, event.Name)
				if relErr != nil {
					rel = event.Name
				}
				fmt.Printf("\n[archlint] File changed: %s\n", rel)
				runWatchScan(absDir)
			}

		case watchErr, ok := <-watcher.Errors:
			if !ok {
				return nil
			}
			fmt.Fprintf(os.Stderr, "[archlint] watcher error: %v\n", watchErr)
		}
	}
}

// runWatchScan performs a scan of dir and prints results in watch output format.
func runWatchScan(dir string) {
	fmt.Printf("[archlint] Scanning...\n")

	cfg := archlintcfg.Load(dir)

	a := analyzer.NewGoAnalyzer()

	graph, err := a.Analyze(dir)
	if err != nil {
		fmt.Fprintf(os.Stderr, "[archlint] analysis error: %v\n", err)
		return
	}

	violations := mcp.DetectAllViolationsWithConfig(graph, &cfg)

	allMetrics := mcp.ComputeAllFileMetrics(a, graph)

	for _, m := range allMetrics {
		if cfg.Rules.DIP.IsEnabled() {
			violations = append(violations, m.DIPViolations...)
		}
		if cfg.Rules.ISP.IsEnabled() {
			violations = append(violations, m.ISPViolations...)
		}
		if cfg.Rules.SRP.IsEnabled() {
			for _, v := range m.SRPViolations {
				if !cfg.IsSRPExcluded(v.Target) {
					violations = append(violations, v)
				}
			}
		}
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

	sort.Slice(violations, func(i, j int) bool {
		if violations[i].Kind != violations[j].Kind {
			return violations[i].Kind < violations[j].Kind
		}
		return violations[i].Target < violations[j].Target
	})

	ts := time.Now().Format("15:04:05")

	if len(violations) == 0 {
		fmt.Printf("[archlint] OK - no violations found\n")
		fmt.Printf("[archlint] 0 errors, 0 warnings\n")
		fmt.Printf("[archlint] Last scan: %s\n", ts)
		return
	}

	errors := 0
	warnings := 0

	for _, v := range violations {
		level := mcp.ViolationLevel(v, &cfg)
		if level == "error" {
			errors++
		} else {
			warnings++
		}
		prefix := mcp.LevelPrefix(level)
		fmt.Printf("[archlint] VIOLATION: %s\n", v.Message)
		if v.Target != "" {
			fmt.Printf("  %s %s\n", prefix, v.Target)
		}
	}

	fmt.Printf("[archlint] %d errors, %d warnings\n", errors, warnings)
	fmt.Printf("[archlint] Last scan: %s\n", ts)
}

// addDirRecursive adds dir and all subdirectories to the watcher.
func addDirRecursive(watcher *fsnotify.Watcher, root string) error {
	return filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // skip unreadable paths
		}
		if info.IsDir() {
			return watcher.Add(path)
		}
		return nil
	})
}
