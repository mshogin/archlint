package mcp

import (
	"github.com/mshogin/archlint/internal/analyzer"
	"github.com/mshogin/archlint/internal/archlintcfg"
	"github.com/mshogin/archlint/internal/model"
)

// metricDetector — единичный ERROR-class детектор в active_metric_registry (SSOT, корень №5).
// Унифицированная сигнатура (g, a, cfg) поглощает разные исходные формы детекторов через замыкания.
type metricDetector func(g *model.Graph, a *analyzer.GoAnalyzer, cfg *archlintcfg.Config) []Violation

// activeErrorClassRegistry — ЕДИНЫЙ реестр ERROR-class детекторов (корень №5, C2 по конструкции).
// baseline (cli/gate.go) И scan (cli/scan.go) берут набор ОТСЮДА -> симметрия baseline<->scan
// гарантирована КОНСТРУКЦИЕЙ, а не дисциплиной синхронизации двух списков. Добавление новой
// ERROR-метрики В ОДНОМ месте автоматически попадает в оба пути (нет дубль-пути, корень №2).
//
// Историческая причина: scan собирал DetectAll+Forbidden+Deprecated+LayerBackedge+Ghost+DeadCode+ISP,
// а gate.go errorClassViolations — лишь DetectAll+DeadCode+ISP -> baseline не снимал forbidden/
// deprecated/layer-backedge/ghost -> ложные NEW в дельте. Реестр устраняет рассинхрон.
func activeErrorClassRegistry() []metricDetector {
	return []metricDetector{
		func(g *model.Graph, _ *analyzer.GoAnalyzer, cfg *archlintcfg.Config) []Violation {
			return DetectAllViolationsWithConfig(g, cfg)
		},
		func(g *model.Graph, _ *analyzer.GoAnalyzer, cfg *archlintcfg.Config) []Violation {
			return ForbiddenDependencies(g, cfg)
		},
		func(g *model.Graph, _ *analyzer.GoAnalyzer, cfg *archlintcfg.Config) []Violation {
			return DeprecatedUsage(g, cfg)
		},
		func(g *model.Graph, _ *analyzer.GoAnalyzer, cfg *archlintcfg.Config) []Violation {
			return LayerBackedge(g, cfg)
		},
		func(g *model.Graph, _ *analyzer.GoAnalyzer, cfg *archlintcfg.Config) []Violation {
			return GhostComponents(g, cfg)
		},
		func(g *model.Graph, a *analyzer.GoAnalyzer, cfg *archlintcfg.Config) []Violation {
			// DeadCode reachability — ГРАФ-ONLY, но analyzer прокидываем для ГРАНИЦЫ prod/_test.go:
			// символы, ОБЪЯВЛЕННЫЕ в _test.go, не помечаются мёртвыми (их область достижимости =
			// тестовые entry-points, не prod-R; иначе ложный ERROR на тест-инфраструктуре). a==nil
			// (stdin/TS/Rust) -> fileOf пуст -> фильтр не активен; для TS/Rust находок 0 по
			// конструкции (нет function|method-узлов). stdin-Go-граф больше НЕ глушит dead-code.
			return DeadCode(g, a, cfg.EntryPoints)
		},
		func(g *model.Graph, a *analyzer.GoAnalyzer, cfg *archlintcfg.Config) []Violation {
			if a == nil || !cfg.Rules.ISP.IsEnabled() {
				return nil
			}

			return ComputeISPUsageSubset(g, a)
		},
		func(g *model.Graph, a *analyzer.GoAnalyzer, _ *archlintcfg.Config) []Violation {
			if a == nil {
				return nil
			}
			// test-only-prod-symbol (под-класс dead-code): возвращает ERROR
			// (unexported, RequiresDelta -> baseline-tracked) + WARNING (exported,
			// open-world). exported в ERROR-class collect безвреден: gate смотрит
			// ClassOf (WARNING != Taboo -> не блокирует), baseline-tracking WARNING
			// нейтрален. self-проверка соундности: 0 ложных ERROR (intToStr — реальный quasi-dead).
			return TestOnlyProdSymbol(g, a)
		},
	}
}

// CollectErrorClassViolations — ЕДИНЫЙ сборщик ERROR-class нарушений (корень №2, «один сборщик,
// два входа»). gate(A,B) = delta(collect(A), collect(B)); scan = collect + сопутствующие сигналы.
// Оба пути вызывают ЭТУ функцию -> набор детекторов один по конструкции (страж №4).
func CollectErrorClassViolations(g *model.Graph, a *analyzer.GoAnalyzer, cfg *archlintcfg.Config) []Violation {
	var out []Violation
	for _, detect := range activeErrorClassRegistry() {
		out = append(out, detect(g, a, cfg)...)
	}

	return out
}
