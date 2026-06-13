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
// сам отфильтрует ERROR-class, но dead-code считается отдельно (не входит в
// DetectAllViolationsWithConfig). Это ровно тот набор, по которому строится baseline
// и оценивается регрессия в scan.
func errorClassViolations(graph *model.Graph, a *analyzer.GoAnalyzer, cfg *archlintcfg.Config) []mcp.Violation {
	viols := mcp.DetectAllViolationsWithConfig(graph, cfg)
	if a != nil {
		viols = append(viols, mcp.DeadCode(graph, cfg.EntryPoints)...)
	}
	return viols
}
