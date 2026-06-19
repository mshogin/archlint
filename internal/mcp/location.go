package mcp

import (
	"fmt"
	"strings"

	"github.com/mshogin/archlint/internal/analyzer"
)

// ApplyLocations дописывает Location (file:line) каждому нарушению по Target-qname из analyzer
// (methods/functions/types несут File+Line). Для починки: не лезть искать строку по qname вручную.
// Display-обогащение — НЕ трогает Fingerprint/Anchor (идентичность нарушения). a==nil -> no-op.
func ApplyLocations(a *analyzer.GoAnalyzer, violations []Violation) {
	if a == nil {
		return
	}

	idx := buildLocationIndex(a)
	for i := range violations {
		if loc := resolveLocation(idx, violations[i].Target); loc != "" {
			violations[i].Location = loc
		}
	}
}

// buildLocationIndex — qname -> "file:line" из всех именованных сущностей анализатора.
func buildLocationIndex(a *analyzer.GoAnalyzer) map[string]string {
	idx := make(map[string]string)

	for id, f := range a.AllFunctions() {
		idx[id] = fmt.Sprintf("%s:%d", f.File, f.Line)
	}
	for id, m := range a.AllMethods() {
		idx[id] = fmt.Sprintf("%s:%d", m.File, m.Line)
	}
	for id, t := range a.AllTypes() {
		idx[id] = fmt.Sprintf("%s:%d", t.File, t.Line)
	}

	return idx
}

// resolveLocation резолвит Target в file:line. Прямой lookup; если Target = OCP-сайт "qname|operand",
// отрезает |operand и резолвит qname. "" если не именованная сущность (напр. target = пакет).
func resolveLocation(idx map[string]string, target string) string {
	if loc, ok := idx[target]; ok {
		return loc
	}

	if i := strings.IndexByte(target, '|'); i >= 0 {
		if loc, ok := idx[target[:i]]; ok {
			return loc
		}
	}

	return ""
}
