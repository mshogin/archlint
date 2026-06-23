package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
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
	scanDiff         string
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
	scanCmd.Flags().StringVar(&scanDiff, "diff", "", "Self-audit delta: mark findings INTRODUCED by the working tree vs git <ref> as NEW (all severity, same canonical fingerprint as the ERROR gate). Empty value (--diff alone) compares against HEAD. Requires a git repo.")
	scanCmd.Flags().Lookup("diff").NoOptDefVal = "HEAD"
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
	// Language — детектированный язык фронта (go|typescript|rust|graph). Часть honest-na:
	// агент видит, ЧЕМ анализировали.
	Language string `json:"language,omitempty"`
	// SymbolLevel — true: символьные детекторы (dead-code/ISP/clone/SRP) реально прогнаны
	// (Go-анализатор присутствует). false: ТОЛЬКО package-level (TS/Rust/stdin-граф) — символьная
	// архитектура НЕ анализирована. Честная граница против ложно-зелёного: PASSED при false НЕ
	// означает «чисто», означает «package-level чисто, символьный уровень не смотрели».
	SymbolLevel bool `json:"symbol_level"`
	// Scope — человекочитаемая маркировка покрытия (что реально проверено / что DISABLED).
	Scope string `json:"scope,omitempty"`
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
	// MultiModule — ИНФО-сигнал per-module скана (репо monorepo просканирован помодульно).
	// nil = single-module. Detected=true -> результат ПОЛОН (агрегат по всем модулям), не абстейн.
	MultiModule *multiModuleInfo `json:"multi_module,omitempty"`
	// Modules — per-module разбивка (только в multi-module агрегате; nil в single). Каждый модуль
	// со своим scanRoot/baseline; верхний уровень (Violations/Blocking/Categories) — агрегат-сумма,
	// Passed — AND по модулям (worst). omitempty -> single-module JSON не несёт.
	Modules []moduleScanResult `json:"modules,omitempty"`
}

// moduleScanResult — результат скана ОДНОГО модуля monorepo (свой scanRoot/baseline).
type moduleScanResult struct {
	Module     string          `json:"module"` // путь модуля относительно корня скана (корневой = ".")
	Passed     bool            `json:"passed"`
	Violations int             `json:"violations"`
	Blocking   int             `json:"blocking"`
	Categories map[string]int  `json:"categories"`
	Baseline   string          `json:"baseline,omitempty"`
	Details    []mcp.Violation `json:"details"`
}

// multiModuleInfo — ИНФО-сигнал per-module скана (репо просканирован помодульно, НЕ абстейн).
// Detected=true + GoModCount: monorepo обработан полностью, каждый модуль — отдельно.
type multiModuleInfo struct {
	Detected   bool   `json:"detected"`
	GoModCount int    `json:"go_mod_count"`
	HasGoWork  bool   `json:"has_go_work"`
	Warning    string `json:"warning"`
}

// isSkippedModuleDir — каталог НЕ несёт боевой go-модуль: build-арт (vendor/node_modules/.git/bin)
// + ФИКСТУРНЫЕ конвенции (testdata — Go-стандарт, инструменты игнорируют; demo/examples — примеры,
// не боевой код) + пользовательские excludes. ЕДИНЫЙ критерий перечня модулей -> симметрия: что
// НЕ считаем модулем, то и НЕ сканируем per-module.
func isSkippedModuleDir(name string, excludes []string) bool {
	return name == "vendor" || name == "node_modules" || name == ".git" || name == "bin" ||
		name == "testdata" || name == "demo" || name == "examples" || analyzer.MatchesExclude(name, excludes)
}

// enumerateModules возвращает каталоги боевых go.mod (skip-критерий isSkippedModuleDir).
// Каждый каталог -> отдельный scanRoot для per-module скана (module-relative qname внутри модуля,
// t_root-инвариантность per-module). Порядок детерминирован (sort) -> стабильный объединённый вывод.
// single-module репо -> [каталог с go.mod]; нет go.mod вовсе -> [] (caller-фолбэк на dir as-is).
func enumerateModules(dir string, excludes []string) []string {
	var mods []string

	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr // best-effort обход; ошибки путей не валят перечень
		}

		if info.IsDir() {
			if isSkippedModuleDir(info.Name(), excludes) {
				return filepath.SkipDir
			}

			return nil
		}

		if info.Name() == "go.mod" {
			mods = append(mods, filepath.Dir(path))
		}

		return nil
	})

	sort.Strings(mods)

	return mods
}

func runScan(cmd *cobra.Command, args []string) error {
	// Load .archlint.yaml config.
	var cfg archlintcfg.Config
	var configFile string

	var graph *model.Graph
	var a *analyzer.GoAnalyzer
	language := "go"              // детектированный фронт; уточняется по выбранному анализатору ниже
	var baselineDir string        // каталог для дефолтного пути baseline (пусто в stdin-режиме)
	var resolvedExcludes []string // итоговые excludes (для --diff ref-worktree скана)

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
		language = "graph" // импортированный YAML-граф: символьного анализатора нет (package-level)

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
		resolvedExcludes = excludes

		if analyzer.DetectRustProject(codeDir) {
			rustAnalyzer := analyzer.NewRustAnalyzer().WithExcludeDirs(excludes)
			g, err := rustAnalyzer.Analyze(codeDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "analysis error: %v\n", err)
				os.Exit(2)
			}
			graph = g
			language = "rust" // Rust-фронт: символьного Go-анализатора нет (package-level)
		} else if analyzer.DetectTypeScriptProject(codeDir) {
			tsAnalyzer := analyzer.NewTypeScriptAnalyzer().WithExcludeDirs(excludes)
			g, err := tsAnalyzer.Analyze(codeDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "analysis error: %v\n", err)
				os.Exit(2)
			}
			graph = g
			language = "typescript" // regex MVP, package-level; символьные детекторы недоступны
		} else {
			// Multi-module (monorepo / go.work): >1 боевой go.mod -> СКАНИРУЕМ КАЖДЫЙ модуль
			// отдельно (свой scanRoot -> module-relative qname, t_root-инвариантность per-module,
			// per-module baseline). Заменяет прежний абстейн (молча неполный скан) полным сканом.
			if mods := enumerateModules(codeDir, excludes); len(mods) > 1 {
				return runScanPerModule(codeDir, mods, excludes, &cfg, configFile)
			}

			a = analyzer.NewGoAnalyzer().WithExcludeDirs(excludes)
			g, err := a.Analyze(codeDir)
			if err != nil {
				fmt.Fprintf(os.Stderr, "analysis error: %v\n", err)
				os.Exit(2)
			}
			graph = g
		}
	}

	// --- Delta gate (Фаза 5) ---
	// Загружаем baseline-снимок ДО сбора: нужен collectFromGraph для OCP baseline-conditional
	// (новая ветка type-dispatch vs baseline). Отсутствует -> nil -> ERROR-class паттерны
	// деградируют в audit (NO-BASELINE -> NO-BLOCK), OCP -> abstain. Дельта-гейт блокирует ТОЛЬКО
	// НОВЫЕ vs baseline ERROR-class паттерны (SCC/layer/dead-code); магнитуды (WARNING/INFO) не
	// блокируются (Ось-1).
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

	violations := collectFromGraph(graph, a, &cfg, baseline)

	// file:line к каждому нарушению (резолв Target-qname из analyzer) — чтобы чинить, не искать
	// строку вручную. Display-обогащение, не трогает Fingerprint.
	mcp.ApplyLocations(a, violations)

	// severity-класс + флаги соундности (ERROR/WARNING/INFO, HumanInLoop, ...) из
	// SSOT severity_class — объяснимость гейта агенту. Display, не трогает Fingerprint.
	mcp.ApplySeverity(violations)

	// --diff self-аудит: пометить НОВЫЕ нарушения (введены рабочим деревом vs git <ref>) для ВСЕХ
	// severity. Снимает ручную операцию stash+собрать-старый-бинарь+diff-JSON -> одна команда.
	if scanDiff != "" && a != nil && baselineDir != "" {
		if err := markDiffNew(baselineDir, scanDiff, resolvedExcludes, &cfg, violations); err != nil {
			fmt.Fprintf(os.Stderr, "warning: --diff vs %s failed (%v); showing full findings\n", scanDiff, err)
		}
	}

	// Threshold count gate applies ТОЛЬКО к не-ERROR-class нарушениям (магнитуды/
	// WARNING): ERROR-class управляются дельта-гейтом, не абсолютным счётом.
	threshold := scanThreshold
	if threshold < 0 {
		threshold = 0
	}

	blocking, _, passed := gateViolations(violations, &cfg, baseline, threshold)

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

	// honest-na (анти ложно-зелёное): символьные детекторы (dead-code/ISP/structural-clone/SRP/LCOM)
	// прогоняются ТОЛЬКО при наличии Go-анализатора (a != nil). Для TS/Rust/импортированного графа их
	// НЕ было — значит PASSED означает «package-level чисто», НЕ «архитектура чиста». Явно маркируем
	// scope, чтобы агент/человек видел границу покрытия и не доверял пустому результату как полному.
	symbolLevel := a != nil
	scope := ""
	if !symbolLevel {
		scope = fmt.Sprintf(
			"PACKAGE-LEVEL only (%s): symbol-level detectors (dead-code/ISP/structural-clone/SRP) DISABLED — symbol-level architecture NOT analyzed",
			language,
		)
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
			Language:         language,
			SymbolLevel:      symbolLevel,
			Scope:            scope,
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
		if scope != "" {
			fmt.Printf("scope: %s\n", scope)
		}
		if total == 0 {
			if symbolLevel {
				fmt.Printf("PASSED: No violations found (threshold: %d)\n", threshold)
			} else {
				// НЕ голое «No violations found»: было бы ложно-зелёное (символьный уровень не смотрели).
				fmt.Printf("PASSED (package-level): no package-level violations (threshold: %d); symbol-level NOT analyzed — see scope above\n", threshold)
			}
		} else {
			status := "PASSED"
			if !passed {
				status = "FAILED"
			}
			fmt.Printf("%s: %d violations found (threshold: %d, blocking regressions: %d)\n", status, total, threshold, blocking)

			// --diff self-аудит: сводка НОВЫХ (введены рабочим деревом vs ref) — главный сигнал автору.
			diffMode := scanDiff != ""
			if diffMode {
				newCount := 0
				for _, v := range violations {
					if v.IsNew {
						newCount++
					}
				}
				fmt.Printf("diff vs %s: %d NEW (introduced by working tree)\n", scanDiff, newCount)
				// NEW первыми (stable) — заметность; gate/categories уже посчитаны выше, display-сорт безопасен.
				sort.SliceStable(violations, func(i, j int) bool { return violations[i].IsNew && !violations[j].IsNew })
			}
			fmt.Println()

			// UX против alert fatigue (коуч-инсайт на уровне вывода): blocking-регрессии
			// (NEW ERROR -> Taboo) печатаем ВСЕГДА (критичны, их мало); шумные не-блокирующие
			// категории (WARNING/INFO, напр. structural-clone) — ЛИМИТ топ-N на Kind + сводка
			// «…ещё M». Полный список всегда доступен через --format json (машинный путь не урезан).
			// В --diff режиме NEW тоже печатаются ВСЕГДА (не урезаются — это цель self-аудита).
			const perKindLimit = 5

			shownPerKind := make(map[string]int)
			hiddenPerKind := make(map[string]int)

			for _, v := range violations {
				// Дельта-уровень: НОВЫЙ ERROR-паттерн -> [ERROR]; существующий/без
				// baseline -> аудит; магнитуды -> их обычный уровень.
				level := mcp.EffectiveLevel(v, &cfg, baseline)
				prefix := mcp.LevelPrefix(level)

				if level != archlintcfg.LevelTaboo && (!diffMode || !v.IsNew) {
					if shownPerKind[v.Kind] >= perKindLimit {
						hiddenPerKind[v.Kind]++

						continue
					}

					shownPerKind[v.Kind]++
				}

				marker := ""
				if diffMode && v.IsNew {
					marker = "NEW "
				}

				fmt.Printf("%s%s [%s] %s\n", marker, prefix, v.Kind, v.Message)
				if v.Target != "" {
					fmt.Printf("  target: %s\n", v.Target)
				}
				if v.Location != "" {
					fmt.Printf("  at: %s\n", v.Location)
				}
				fmt.Println()
			}

			if len(hiddenPerKind) > 0 {
				kinds := make([]string, 0, len(hiddenPerKind))
				for k := range hiddenPerKind {
					kinds = append(kinds, k)
				}
				sort.Strings(kinds)

				fmt.Printf("…showing top %d per category; hidden (full list: --format json):\n", perKindLimit)
				for _, k := range kinds {
					fmt.Printf("  [%s] +%d more\n", k, hiddenPerKind[k])
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

// collectFromGraph — ЕДИНЫЙ сбор нарушений из графа+анализатора (SSOT, корни №2/№5). Один и тот же
// код для single-module (runScan) и per-module (monorepo loop) -> per-module результаты собраны тем
// же набором метрик, что single (нет расхождения опорных точек). a==nil (stdin/Rust/TS) -> только
// ERROR-class из графа; a!=nil (Go) -> + structural-clone + per-file SOLID/smell. baseline (опц.) ->
// OCP baseline-conditional (новая ветка type-dispatch существующего S); nil baseline -> OCP abstain.
func collectFromGraph(graph *model.Graph, a *analyzer.GoAnalyzer, cfg *archlintcfg.Config, baseline *mcp.Baseline) []mcp.Violation {
	// ERROR-class (structural coupling/cycles + forbidden + deprecated + layer-backedge + ghost +
	// dead-code + ISP). Тот же набор использует baseline (gate.go errorClassViolations) -> симметрия.
	violations := mcp.CollectErrorClassViolations(graph, a, cfg)

	// structural-clone (DRY) — точная изоморфная копипаста >= cloneMinSize. WARNING (не блок).
	if a != nil {
		violations = append(violations, mcp.StructuralClone(a)...)
	}

	// Per-file SOLID and smell violations (Go projects only).
	var allMetrics map[string]*mcp.FileMetrics
	if a != nil {
		allMetrics = mcp.ComputeAllFileMetrics(a, graph)
	}

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

		// feature-envy ДЕМОТИРОВАН из active_scan_set (обоснованный отказ, доказательство в
		// docs/proof-catalog): self-проверка за 3 витка вскрыла, что метрика структурно недоказуема
		// на Go без type-резолва (call.Receiver = синтаксическое ИМЯ != семантический тип: не различает
		// чужой объект / stdlib-пакет / своё поле). НЕ эмитится в боевом scan/gate/health. Вычисление
		// (computeFeatureEnvy с объект-локализацией + own-fix) сохранено в FileMetrics.FeatureEnvy/Envies
		// для диагностики (MCP/signals). Реактивация: при появлении go/types все 3 корня решаемы.

		for _, ss := range m.ShotgunSurgery {
			violations = append(violations, mcp.Violation{
				Kind:    "shotgun-surgery",
				Message: fmt.Sprintf("Shotgun surgery risk: %s", ss),
				Target:  ss,
			})
		}
	}

	// OCP baseline-conditional (ocp-open-modification, WARNING): новая ветка type-dispatch
	// существующего S vs baseline. nil baseline -> abstain (CollectOCP вернёт nil). Go-only (a!=nil).
	if a != nil {
		violations = append(violations, mcp.CollectOCP(a, baseline)...)
	}

	// Стабильный порядок: kind, затем target.
	sort.Slice(violations, func(i, j int) bool {
		if violations[i].Kind != violations[j].Kind {
			return violations[i].Kind < violations[j].Kind
		}
		return violations[i].Target < violations[j].Target
	})

	return violations
}

// collectGoModuleViolations анализирует ОДИН go-модуль (свой scanRoot=moduleDir -> module-relative
// qname, t_root-инвариантность per-module) и собирает все его нарушения тем же collectFromGraph,
// что single. Каждый модуль получает СВЕЖИЙ analyzer (без переноса state между модулями monorepo).
func collectGoModuleViolations(moduleDir string, excludes []string, cfg *archlintcfg.Config, baseline *mcp.Baseline) ([]mcp.Violation, error) {
	a := analyzer.NewGoAnalyzer().WithExcludeDirs(excludes)
	g, err := a.Analyze(moduleDir)
	if err != nil {
		return nil, err
	}

	return collectFromGraph(g, a, cfg, baseline), nil
}

// markDiffNew помечает IsNew нарушения, ВВЕДЁННЫЕ рабочим деревом vs git <ref>. ref-состояние
// собирается во ВРЕМЕННОМ git-worktree (рабочее дерево НЕ трогается — ни checkout, ни stash);
// refSet = {Kind|Fingerprint} через ЕДИНЫЙ canonical Fingerprint (тот же, что baseline-ERROR-гейт,
// C1 — одна канонизация). NEW = (Kind,Fingerprint) ∉ refSet, для ВСЕХ severity (снимаем severity-
// фильтр с уже-соундной дельты). Расширение дельта-механизма, НЕ новый путь канонизации.
func markDiffNew(repoDir, ref string, excludes []string, cfg *archlintcfg.Config, violations []mcp.Violation) error {
	refDir, err := os.MkdirTemp("", "archlint-diff-*")
	if err != nil {
		return fmt.Errorf("temp worktree: %w", err)
	}
	defer func() { _ = os.RemoveAll(refDir) }()

	if out, err := exec.Command("git", "-C", repoDir, "worktree", "add", "--detach", refDir, ref).CombinedOutput(); err != nil { //nolint:gosec // ref из CLI-флага self-аудита
		return fmt.Errorf("git worktree add %s: %v: %s", ref, err, out)
	}
	defer func() { _ = exec.Command("git", "-C", repoDir, "worktree", "remove", "--force", refDir).Run() }() //nolint:gosec // cleanup временного worktree

	// ref-findings ТЕМИ ЖЕ детекторами (collectGoModuleViolations -> collectFromGraph), baseline=nil
	// (полный набор всех severity для сравнения).
	refViolations, err := collectGoModuleViolations(refDir, excludes, cfg, nil)
	if err != nil {
		return fmt.Errorf("scan ref %s: %w", ref, err)
	}

	refSet := make(map[string]bool, len(refViolations))
	for _, v := range refViolations {
		refSet[v.Kind+"|"+mcp.Fingerprint(v)] = true
	}

	for i := range violations {
		if !refSet[violations[i].Kind+"|"+mcp.Fingerprint(violations[i])] {
			violations[i].IsNew = true
		}
	}

	return nil
}

// gateViolations — ЕДИНЫЙ гейт-расчёт (SSOT): blocking = НОВЫЕ ERROR-class паттерны vs baseline
// (Taboo); nonErrorCount = магнитуды/WARNING под threshold-гейтом; passed = нет блока И count<=порог.
// Один код для single (runScan) и per-module (агрегат) -> гейт-семантика не расходится между путями.
func gateViolations(violations []mcp.Violation, cfg *archlintcfg.Config, baseline *mcp.Baseline, threshold int) (blocking, nonErrorCount int, passed bool) {
	isErrorClass := func(kind string) bool {
		c, ok := mcp.ClassOf(kind)
		return ok && c.Class == "ERROR"
	}

	for _, v := range violations {
		if mcp.EffectiveLevel(v, cfg, baseline) == archlintcfg.LevelTaboo {
			blocking++
		}
		if !isErrorClass(v.Kind) {
			nonErrorCount++
		}
	}

	return blocking, nonErrorCount, nonErrorCount <= threshold && blocking == 0
}

// runScanPerModule сканирует monorepo ПОМОДУЛЬНО: каждый go.mod-каталог отдельно (свой scanRoot ->
// module-relative qname; СВОЙ baseline <module>/.archlint-baseline.json -> delta per-module, не
// смешиваем модули). Заменяет прежний абстейн полным сканом. Агрегат: violations/blocking — сумма,
// passed — AND (worst), exit — worst (любой !passed -> exit 1). Вывод — секции с module-префиксом.
func runScanPerModule(codeDir string, mods []string, excludes []string, cfg *archlintcfg.Config, configFile string) error {
	threshold := scanThreshold
	if threshold < 0 {
		threshold = 0
	}

	results := make([]moduleScanResult, 0, len(mods))
	allPassed := true
	totalViolations := 0
	totalBlocking := 0
	aggCategories := make(map[string]int)

	for _, moduleDir := range mods {
		rel, err := filepath.Rel(codeDir, moduleDir)
		if err != nil || rel == "" {
			rel = moduleDir
		}

		// Baseline — СТРОГО per-module (свой файл в каталоге модуля). Глобальный --baseline на
		// monorepo не применяется: один файл не описывает несколько модулей с разными scanRoot.
		// Грузим ДО сбора: нужен для OCP baseline-conditional per-module (delta не смешивает модули).
		baselinePath := filepath.Join(moduleDir, defaultBaselineName)
		var baseline *mcp.Baseline
		if b, err := loadBaseline(baselinePath); err != nil {
			fmt.Fprintf(os.Stderr, "error [module %s]: %v\n", rel, err)
			os.Exit(2)
		} else {
			baseline = b
		}

		violations, err := collectGoModuleViolations(moduleDir, excludes, cfg, baseline)
		if err != nil {
			fmt.Fprintf(os.Stderr, "analysis error [module %s]: %v\n", rel, err)
			os.Exit(2)
		}

		loadedBaseline := ""
		if baseline != nil {
			loadedBaseline = baselinePath
		}

		blocking, _, passed := gateViolations(violations, cfg, baseline, threshold)

		categories := make(map[string]int)
		for _, v := range violations {
			categories[v.Kind]++
			aggCategories[v.Kind]++
		}

		results = append(results, moduleScanResult{
			Module:     rel,
			Passed:     passed,
			Violations: len(violations),
			Blocking:   blocking,
			Categories: categories,
			Baseline:   loadedBaseline,
			Details:    violations,
		})

		totalViolations += len(violations)
		totalBlocking += blocking
		allPassed = allPassed && passed
	}

	info := &multiModuleInfo{
		Detected:   true,
		GoModCount: len(mods),
		Warning:    fmt.Sprintf("multi-module repo: %d modules scanned per-module (each own scanRoot/baseline).", len(mods)),
	}

	printPerModule(results, aggCategories, info, threshold, totalViolations, totalBlocking, allPassed, configFile)

	if !allPassed {
		os.Exit(1)
	}

	return nil
}

// printPerModule печатает агрегат monorepo: JSON (modules[] + верхнеуровневая сумма/AND) либо текст
// (секция на модуль с префиксом + сводка). Верхнеуровневые categories — сумма по kind (health их
// потребляет по kind без изменений). Module-префикс отделяет нарушения разных модулей в выводе.
func printPerModule(results []moduleScanResult, aggCategories map[string]int, info *multiModuleInfo, threshold, totalViolations, totalBlocking int, allPassed bool, configFile string) {
	switch scanFormat {
	case "json":
		result := scanGateResult{
			Passed:      allPassed,
			Violations:  totalViolations,
			Threshold:   threshold,
			Blocking:    totalBlocking,
			Categories:  aggCategories,
			ConfigFile:  configFile,
			MultiModule: info,
			Modules:     results,
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
		fmt.Printf("multi-module: %d modules scanned per-module\n\n", info.GoModCount)

		for _, r := range results {
			status := "PASSED"
			if !r.Passed {
				status = "FAILED"
			}
			fmt.Printf("=== module: %s === %s: %d violations (blocking: %d)\n", r.Module, status, r.Violations, r.Blocking)
			for _, v := range r.Details {
				fmt.Printf("  [%s/%s] %s\n", r.Module, v.Kind, v.Message)
			}
			fmt.Println()
		}

		status := "PASSED"
		if !allPassed {
			status = "FAILED"
		}
		fmt.Printf("SUMMARY: %s — %d violations across %d modules (blocking: %d, threshold: %d)\n",
			status, totalViolations, info.GoModCount, totalBlocking, threshold)
	default:
		fmt.Fprintf(os.Stderr, "unknown format: %s (use text or json)\n", scanFormat)
		os.Exit(2)
	}
}
