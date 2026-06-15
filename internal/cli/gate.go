package cli

import (
	"fmt"
	"os"

	"github.com/mshogin/archlint/internal/analyzer"
	"github.com/mshogin/archlint/internal/archlintcfg"
	"github.com/mshogin/archlint/internal/mcp"
	"github.com/mshogin/archlint/internal/model"
)

// defaultBaselineName — имя файла снимка дельта-гейта рядом со сканируемым кодом.
const defaultBaselineName = ".archlint-baseline.json"

// analyzeForGate строит граф для гейт-команд (baseline/scan-delta). Возвращает
// граф и Go-анализатор (nil для TS/Rust — у них нет dead-code/FileMetrics-фактов).
func analyzeForGate(codeDir string, excludes []string) (*model.Graph, *analyzer.GoAnalyzer, error) {
	if _, err := os.Stat(codeDir); os.IsNotExist(err) {
		return nil, nil, fmt.Errorf("%w: %s", errDirNotExist, codeDir)
	}

	switch {
	case analyzer.DetectRustProject(codeDir):
		g, err := analyzer.NewRustAnalyzer().WithExcludeDirs(excludes).Analyze(codeDir)
		if err != nil {
			return nil, nil, fmt.Errorf("analysis error: %w", err)
		}
		return g, nil, nil
	case analyzer.DetectTypeScriptProject(codeDir):
		g, err := analyzer.NewTypeScriptAnalyzer().WithExcludeDirs(excludes).Analyze(codeDir)
		if err != nil {
			return nil, nil, fmt.Errorf("analysis error: %w", err)
		}
		return g, nil, nil
	default:
		a := analyzer.NewGoAnalyzer().WithExcludeDirs(excludes)
		g, err := a.Analyze(codeDir)
		if err != nil {
			return nil, nil, fmt.Errorf("analysis error: %w", err)
		}
		return g, a, nil
	}
}

// errorClassViolations собирает ВСЕ паттерн-факты, которые участвуют в дельта-гейте:
// структурные (cycles, layer back-edges) + dead-code (только Go-граф). BuildBaseline
// сам отфильтрует ERROR-class, но dead-code И isp-usage-subset считаются отдельно
// (не входят в DetectAllViolationsWithConfig). Это ровно тот набор, по которому
// строится baseline и оценивается регрессия в scan — НАБОР ОБЯЗАН БЫТЬ СИММЕТРИЧЕН
// сбору в scan.go, иначе baseline-снапшот неполон и существующие паттерны ложно
// считаются NEW (инцидент 2026-06-15: ISP не попадал в baseline -> 9 ISP-долгов
// ложно блокировались дельтой на стабильном коде). BuildBaseline отфильтрует
// не-ERROR-class (isp-external-narrow -> WARNING -> отброшен).
func errorClassViolations(graph *model.Graph, a *analyzer.GoAnalyzer, cfg *archlintcfg.Config) []mcp.Violation {
	viols := mcp.DetectAllViolationsWithConfig(graph, cfg)
	if a != nil {
		viols = append(viols, mcp.DeadCode(graph, cfg.EntryPoints)...)
		if cfg.Rules.ISP.IsEnabled() {
			viols = append(viols, mcp.ComputeISPUsageSubset(graph, a)...)
		}
	}
	return viols
}
