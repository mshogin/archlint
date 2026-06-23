package mcp

import "strings"

// severity_apply.go — обогащение MCP-вывода (Violation) severity-классом и
// флагами соундности для ОБЪЯСНИМОСТИ агентского гейта (Горизонт 1). Резолв из
// ЕДИНОГО реестра violationClasses (severity_class.go, SSOT C1) по Kind при
// выводе — как ApplyLocations резолвит Location по Target. Severity/флаги —
// DISPLAY-поля, НЕ участвуют в Fingerprint/Anchor (идентичность нарушения и
// дельта/canonical не затрагиваются).

// ApplySeverity дописывает каждому нарушению Severity (ERROR|WARNING|INFO) +
// флаги условной соундности (OpenWorld/RequiresDelta/HumanInLoop) из реестра
// severity_class + арх-принцип. Mutating in place (как ApplyLocations).
// Kind не в реестре -> Severity="" (omitempty): страж полноты гарантирует, что
// боевые kinds зарегистрированы.
func ApplySeverity(violations []Violation) {
	for i := range violations {
		if c, ok := ClassOf(violations[i].Kind); ok {
			violations[i].Severity = c.Class
			violations[i].OpenWorld = c.OpenWorld
			violations[i].RequiresDelta = c.RequiresDelta
			violations[i].HumanInLoop = c.HumanInLoop
		}
		violations[i].Principle = principleOf(violations[i].Kind)
		violations[i].Remediation = remediationOf(violations[i].Kind)
	}
}

// remediationOf — actionable НАПРАВЛЕНИЕ «как устранить» per Kind. ★GUIDANCE для
// агента (как действовать), НЕ доказательство/гарантия/метрика — DX-слой
// объяснимости. HumanInLoop-нарушения (dead-code) содержат явную оговорку
// «подтвердить с человеком». "" если Kind не сопоставлен.
func remediationOf(kind string) string {
	switch kind {
	case "circular-dependency":
		return "break the cycle: extract the shared abstraction into a separate package, or invert one dependency through an interface"
	case "layer-violation", "layer-backedge":
		return "remove the back-edge: dependencies must follow the declared layer order; invert through an interface"
	case "forbidden-dependency":
		return "remove the import of the forbidden package (see the declared rule); move shared code into an allowed layer"
	case "deprecated-usage":
		return "replace the deprecated call with the current API (see the deprecated marker)"
	case "dead-code":
		return "★confirm with a human (HumanInLoop, not automatic): then delete the unused code OR wire up the missing entry point"
	case kindTestOnlyProdSymbol, kindTestOnlyProdSymbolExported:
		return "★confirm with a human (HumanInLoop): remove the symbol from prod (if dead) OR move it into _test.go/testdata, OR wire up a legitimate prod user (may be an unfinished integration)"
	case "ghost-component":
		return "stale context declaration: remove the missing component from .archlint contexts OR restore it in the graph"
	case "isp-usage-subset", "isp-fat-interface":
		return "split the interface by usage clusters (ISP): give the client a narrow interface of the methods it actually uses"
	case "dip-concrete-dependency", "dip-abstraction-to-detail":
		return "invert the dependency through an interface/abstraction (DIP): depend on the abstraction, not the concrete type"
	case "ocp-open-modification":
		return "extend behavior via polymorphism / a new type (OCP) instead of modifying the existing type-dispatch"
	case "structural-clone":
		return "extract the duplicated code into a shared function/type (DRY)"
	case "srp-lack-of-cohesion", "srp-multiple-responsibilities", "srp-too-many-methods", "srp-too-many-fields":
		return "split responsibilities (SRP): extract independent concerns into separate types/functions"
	case "god-class":
		return "split the god class into smaller types by responsibility; extract groups of methods/fields"
	case "hub-node":
		return "reduce the hub's centrality: break some links via mediators/interfaces, or split the node"
	case "high-efferent-coupling":
		return "reduce outgoing dependencies: group them behind a facade/interface, remove unnecessary imports"
	case "shotgun-surgery":
		return "consolidate the scattered logic into one module so a change does not touch many files"
	case "articulation-point", "bridge-edge", "stability-violation":
		return "structural fragility signal (diagnostic): consider path redundancy or reducing the dependency at the bottleneck"
	default:
		return ""
	}
}

// principleOf — арх-принцип по Kind (объяснимость агенту: к какому принципу
// относится нарушение). Дешёвый mapping по имени Kind; НЕ дублирует severity-
// классификацию (та — в violationClasses), а лишь маркирует семейство принципа.
func principleOf(kind string) string {
	switch {
	case strings.HasPrefix(kind, "srp-"):
		return "SRP"
	case strings.HasPrefix(kind, "dip-"):
		return "DIP"
	case strings.HasPrefix(kind, "isp-"):
		return "ISP"
	case strings.HasPrefix(kind, "ocp-"):
		return "OCP"
	case strings.HasPrefix(kind, "layer"):
		return "layering"
	}
	switch kind {
	case "circular-dependency":
		return "acyclic-dependencies"
	case "forbidden-dependency":
		return "dependency-rules"
	case "deprecated-usage":
		return "deprecation"
	case "dead-code":
		return "reachability"
	case kindTestOnlyProdSymbol, kindTestOnlyProdSymbolExported:
		return "test-hygiene"
	case "ghost-component":
		return "context-integrity"
	case "god-class", "hub-node", "high-efferent-coupling", "shotgun-surgery",
		"articulation-point", "bridge-edge", "stability-violation":
		return "coupling-cohesion"
	case "structural-clone":
		return "DRY"
	default:
		return ""
	}
}
