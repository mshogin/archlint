package mcp

import (
	"fmt"

	"github.com/mshogin/archlint/internal/analyzer"
	"github.com/mshogin/archlint/internal/model"
)

// OCP (Open-Closed) узкий, baseline-conditional. Доказуемый SOLID РЕАЛИЗАЦИЕЙ (не только
// классификацией): type-dispatch S «закрыт» для расширения, если добавление поддержки нового типа
// требует МОДИФИКАЦИИ S (новой ветки), а не нового полиморфного типа. Нарушение проксируется
// СТРУКТУРОЙ дельты: ветка на тип ∈ Δ⁺ (новый vs baseline) в СУЩЕСТВУЮЩЕМ S = «закрытое изменено».
//
// Два kind, оба через ЕДИНЫЙ Fingerprint/Anchor (canonical S.identity, путь getPkgID — не новый):
//   - ocp-dispatch-site (INFO): ФАКТ существования ветки (S::тип) для baseline-СНИМКА. Эмитится
//     ТОЛЬКО при генерации baseline (CollectDispatchFacts), не в scan-выводе.
//   - ocp-open-modification (WARNING): НАРУШЕНИЕ — новая ветка существующего S vs baseline (П3).
const (
	// KindOCPDispatchSite — baseline-снимок ветки type-dispatch (S::тип). INFO, не нарушение.
	KindOCPDispatchSite = "ocp-dispatch-site"
	// KindOCPOpenModification — новая ветка существующего dispatch vs baseline. WARNING.
	KindOCPOpenModification = "ocp-open-modification"
)

// dispatchSiteID — идентичность сайта S: canonical qname функции/метода (map-ключ analyzer, уже
// module-relative через getPkgID) + локальный Operand. НЕ новый путь канонизации: canonical часть —
// уже-канонический id, Operand path-независим (локальная переменная/поле). monorepo: id module-
// relative внутри модуля (per-module scanRoot) -> S.identity module-relative автоматически.
func dispatchSiteID(qname, operand string) string {
	return qname + "|" + operand
}

// dispatchFingerprint — единый Anchor ветки (S::тип): строгая идентичность экземпляра для дельты.
func dispatchFingerprint(siteID, typ string) string {
	return siteID + "::" + typ
}

// CollectDispatchFacts эмитит ФАКТ-снимок каждой ветки type-dispatch (ocp-dispatch-site, INFO) для
// baseline-генерации. По одному Violation на (S, тип): Anchor = canonical S.identity::тип -> единый
// fingerprint-путь. Эти факты снимаются BuildBaseline (baselineTracked) и образуют слепок «что было»,
// против которого П3 (collectOCP) детектит НОВЫЕ ветки существующего S. Только в baseline-path.
func CollectDispatchFacts(a *analyzer.GoAnalyzer) []Violation {
	if a == nil {
		return nil
	}

	var out []Violation

	emit := func(qname string, disps []model.TypeDispatch) {
		for _, d := range disps {
			site := dispatchSiteID(qname, d.Operand)
			for _, typ := range d.Types {
				out = append(out, Violation{
					Kind:    KindOCPDispatchSite,
					Target:  site,
					Anchor:  dispatchFingerprint(site, typ),
					Message: fmt.Sprintf("type-dispatch %s: branch on %s", site, typ),
				})
			}
		}
	}

	for id, f := range a.AllFunctions() {
		emit(id, f.Dispatches)
	}
	for id, m := range a.AllMethods() {
		emit(id, m.Dispatches)
	}

	return out
}
