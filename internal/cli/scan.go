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
	"github.com/mshogin/archlint/internal/archmotifbridge"
	"github.com/mshogin/archlint/internal/graphloader"
	"github.com/mshogin/archlint/internal/mcp"
	"github.com/mshogin/archlint/internal/model"
	"github.com/spf13/cobra"
)

var (
	scanFormat       string
	scanThreshold    int
	scanConfigFile   string
	scanStdin        bool
	scanExclude      []string
	scanBaselineFile string
	scanSignals      bool
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
	scanCmd.Flags().StringSliceVar(&scanExclude, "exclude", nil, "Directory basenames to skip during the source walk (additive on top of built-in defaults). Repeatable.")
	scanCmd.Flags().StringVar(&scanBaselineFile, "baseline", "", "Path to .archlint-baseline.json for delta gating (default: <directory>/.archlint-baseline.json). Absent baseline -> audit mode (no block on ERROR patterns).")
	scanCmd.Flags().BoolVar(&scanSignals, "signals", false, "Audit/slow mode: also compute structural magnitude descriptors (centralities, coupling, smells) and include them under `signals` in JSON. Off by default — the fast gate stays free of magnitudes (speed constitution).")
	rootCmd.AddCommand(scanCmd)
}

// scanGateResult is the JSON output for the scan command.
type scanGateResult struct {
	Passed     bool            `json:"passed"`
	Violations int             `json:"violations"`
	Threshold  int             `json:"threshold"`
	Blocking   int             `json:"blocking"` // НОВЫЕ ERROR-class паттерны vs baseline (регрессии)
	Categories map[string]int  `json:"categories"`
	Details    []mcp.Violation `json:"details"`
	ConfigFile string          `json:"config_file,omitempty"`
	Baseline   string          `json:"baseline,omitempty"` // путь к загруженному snapshot ("" = audit-режим)
	// Signals — структурные магнитудные дескрипторы (--signals, audit/slow). Не часть
	// гейта: магнитуды НЕ блокируют. omitempty -> быстрый гейт их не несёт.
	Signals *mcp.Descriptors `json:"signals,omitempty"`
	// ArchmotifSignals — research-метрики archmotif (modularity, motif_redundancy,
	// spectral/symmetry детекторы) под --signals. Сигналы/наблюдаемость, НИКОГДА ERROR
	// (спектр != proof/).
	ArchmotifSignals *archmotifbridge.Report `json:"archmotifSignals,omitempty"`
	// ContextSignals — INFO-дескрипторы объявленных контекстов (complexity/coupling/
	// depth) под --signals. nil без contexts в конфиге. Не гейт.
	ContextSignals *mcp.ContextSignals `json:"contextSignals,omitempty"`
	// ResearchSignals — медленные структурные дескрипторы (порядок/цепи/замыкание/
	// геодезические) под --signals (research/slow). Сигналы/наблюдаемость, НИКОГДА
	// ERROR. Поля внутри nil, если метрика пропущена (мало узлов / граф > 200).
	ResearchSignals *mcp.ResearchDescriptors `json:"researchSignals,omitempty"`
}

func runScan(cmd *cobra.Command, args []string) error {
	// Load .archlint.yaml config.
	var cfg archlintcfg.Config
	var configFile string

	var graph *model.Graph
	var a *analyzer.GoAnalyzer
	var baselineDir string // каталог для дефолтного пути baseline (пусто в stdin-режиме)

	if scanStdin {
		// Read YAML graph from stdin.
		data, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error reading stdin: %v\n", err)
			os.Exit(2)
		}
		// Универсальный загрузчик (порт graph_loader.py): archlint/DocHub/callgraph
		// форматы + автоопределение. Заменяет наивный Unmarshal (только archlint).
		g, err := graphloader.ParseYAML(data)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error parsing YAML from stdin: %v\n", err)
			os.Exit(2)
		}
		graph = g

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
		baselineDir = codeDir

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

		excludes := mergeExcludes(cfg.ExcludePaths, scanExclude)

		if analyzer.DetectRustProject(codeDir) {
			rustAnalyzer := analyzer.NewRustAnalyzer().WithExcludeDirs(excludes)
			g, err := rustAnalyzer.Analyze(codeDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "analysis error: %v\n", err)
				os.Exit(2)
			}
			graph = g
		} else if analyzer.DetectTypeScriptProject(codeDir) {
			tsAnalyzer := analyzer.NewTypeScriptAnalyzer().WithExcludeDirs(excludes)
			g, err := tsAnalyzer.Analyze(codeDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "analysis error: %v\n", err)
				os.Exit(2)
			}
			graph = g
		} else {
			a = analyzer.NewGoAnalyzer().WithExcludeDirs(excludes)
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

	// Forbidden dependencies (ERROR, closed-world relative to declared config).
	// Inactive when no forbidden rules are configured.
	violations = append(violations, mcp.ForbiddenDependencies(graph, &cfg)...)

	// Deprecated usage (ERROR, closed-world relative to explicit deprecated markers).
	// Inactive when no deprecated patterns/attributes are present.
	violations = append(violations, mcp.DeprecatedUsage(graph, &cfg)...)

	// Layer back-edges against declared layer order (ERROR level B).
	// Inactive when no layers are configured.
	violations = append(violations, mcp.LayerBackedge(graph, &cfg)...)

	// Ghost components: declared in a context but absent from the graph (ERROR,
	// closed-world relative to declared contexts). Inactive without contexts.
	violations = append(violations, mcp.GhostComponents(graph, &cfg)...)

	// NB: soundness candidates (articulation/bridge/stability) are NOT gate violations —
	// the soundness check demoted all three to signals (DIP-class confound). They are
	// surfaced under --signals via ComputeDescriptors, never in severity_class/ the gate.

	// Dead-code (ERROR-class, open-world) — Go-граф only. Участвует в дельта-гейте:
	// НОВЫЙ мёртвый узел vs baseline = регрессия (блок + удаление human-in-loop).
	if a != nil {
		violations = append(violations, mcp.DeadCode(graph, cfg.EntryPoints)...)
	}

	// ISP usage-subset — клиент-центричный «жирный интерфейс» (узкое
	// использование param-интерфейса + 2 guard'а). НЕ блокирующий до прохождения
	// проверки соундности: Kind'ы не в severity_class как ERROR -> аудит-уровень.
	if a != nil && cfg.Rules.ISP.IsEnabled() {
		violations = append(violations, mcp.ComputeISPUsageSubset(graph, a)...)
	}

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

	// --- Delta gate (Фаза 5) ---
	// Загружаем baseline-снимок: отсутствует -> nil -> ERROR-class паттерны
	// деградируют в audit (NO-BASELINE -> NO-BLOCK). Дельта-гейт блокирует ТОЛЬКО
	// НОВЫЕ vs baseline ERROR-class паттерны (SCC/layer/dead-code); магнитуды
	// (WARNING/INFO) дельта-гейтом не блокируются (Ось-1).
	baselinePath := scanBaselineFile
	if baselinePath == "" && baselineDir != "" {
		baselinePath = filepath.Join(baselineDir, defaultBaselineName)
	}
	var baseline *mcp.Baseline
	if baselinePath != "" {
		b, err := loadBaseline(baselinePath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %v\n", err)
			os.Exit(2)
		}
		baseline = b
	}

	// Threshold count gate applies ТОЛЬКО к не-ERROR-class нарушениям (магнитуды/
	// WARNING): ERROR-class управляются дельта-гейтом, не абсолютным счётом.
	threshold := scanThreshold
	if threshold < 0 {
		threshold = 0
	}

	isErrorClass := func(kind string) bool {
		c, ok := mcp.ClassOf(kind)
		return ok && c.Class == "ERROR"
	}

	blocking := 0      // НОВЫЕ ERROR-class паттерны (регрессия) -> блок
	nonErrorCount := 0 // не-ERROR нарушения -> подлежат threshold-гейту
	for _, v := range violations {
		if mcp.EffectiveLevel(v, &cfg, baseline) == archlintcfg.LevelTaboo {
			blocking++
		}
		if !isErrorClass(v.Kind) {
			nonErrorCount++
		}
	}

	countPassed := nonErrorCount <= threshold
	passed := countPassed && blocking == 0

	total := len(violations)

	// Путь baseline для отчёта: показываем только при реально загруженном снимке
	// (nil = audit-режим, no-baseline -> no-block).
	loadedBaseline := ""
	if baseline != nil {
		loadedBaseline = baselinePath
	}

	// Build categories map.
	categories := make(map[string]int)
	for _, v := range violations {
		categories[v.Kind]++
	}

	// Магнитудные дескрипторы — только в audit/slow (--signals); НЕ в быстром гейте.
	var signals *mcp.Descriptors

	var archmotifSignals *archmotifbridge.Report

	var researchSignals *mcp.ResearchDescriptors

	if scanSignals {
		dd := mcp.ComputeDescriptors(graph)
		signals = &dd

		rep := archmotifbridge.Compute(graph)
		archmotifSignals = &rep

		rd := mcp.ComputeResearchDescriptors(graph)
		researchSignals = &rd
	}

	var contextSignals *mcp.ContextSignals
	if scanSignals {
		contextSignals = mcp.ComputeContextSignals(&cfg)
		// context_coverage — WARNING-сигнал (вердикт соундности), под --signals, не гейт.
		if contextSignals != nil {
			cov := mcp.ComputeContextCoverage(graph, &cfg)
			if cov.Active {
				contextSignals.Coverage = &cov
			}
		}
	}

	switch scanFormat {
	case "json":
		result := scanGateResult{
			Passed:           passed,
			Violations:       total,
			Threshold:        threshold,
			Blocking:         blocking,
			Categories:       categories,
			Details:          violations,
			ConfigFile:       configFile,
			Baseline:         loadedBaseline,
			Signals:          signals,
			ArchmotifSignals: archmotifSignals,
			ContextSignals:   contextSignals,
			ResearchSignals:  researchSignals,
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
		if loadedBaseline != "" {
			fmt.Printf("baseline: %s (delta gate)\n", loadedBaseline)
		} else {
			fmt.Printf("baseline: none (audit mode — ERROR patterns reported, not blocked)\n")
		}
		if total == 0 {
			fmt.Printf("PASSED: No violations found (threshold: %d)\n", threshold)
		} else {
			status := "PASSED"
			if !passed {
				status = "FAILED"
			}
			fmt.Printf("%s: %d violations found (threshold: %d, blocking regressions: %d)\n\n", status, total, threshold, blocking)

			for _, v := range violations {
				// Дельта-уровень: НОВЫЙ ERROR-паттерн -> [ERROR]; существующий/без
				// baseline -> аудит; магнитуды -> их обычный уровень.
				level := mcp.EffectiveLevel(v, &cfg, baseline)
				prefix := mcp.LevelPrefix(level)
				fmt.Printf("%s [%s] %s\n", prefix, v.Kind, v.Message)
				if v.Target != "" {
					fmt.Printf("  target: %s\n", v.Target)
				}
				fmt.Println()
			}
		}

		if signals != nil {
			fmt.Printf("signals (audit): nodes=%d edges=%d density=%.4f maxKCore=%d godClass=%d shotgun=%d (use --format json for full)\n",
				signals.NodeCount, signals.EdgeCount, signals.Density, signals.MaxKCore, signals.GodClass, signals.ShotgunSurgery)
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
