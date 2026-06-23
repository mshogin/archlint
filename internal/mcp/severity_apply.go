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
