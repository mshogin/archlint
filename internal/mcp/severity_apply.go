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
		return "разорвать цикл: вынести общую абстракцию в отдельный пакет или инвертировать одну зависимость через интерфейс"
	case "layer-violation", "layer-backedge":
		return "убрать back-edge: зависимость должна идти по объявленному порядку слоёв; инвертировать через интерфейс"
	case "forbidden-dependency":
		return "убрать импорт запрещённого пакета (см. объявленный запрет); вынести общее в разрешённый слой"
	case "deprecated-usage":
		return "заменить вызов deprecated на актуальный API (см. маркер deprecated)"
	case "dead-code":
		return "★подтвердить с человеком (HumanInLoop, не авто): затем удалить неиспользуемый код ЛИБО подключить недостающую точку входа"
	case kindTestOnlyProdSymbol, kindTestOnlyProdSymbolExported:
		return "★подтвердить с человеком (HumanInLoop): убрать символ из prod (если мёртв) ЛИБО перенести в _test.go/testdata, ЛИБО подключить легального prod-юзера (возможна незаконченная интеграция)"
	case "ghost-component":
		return "устаревшая декларация контекста: убрать отсутствующий компонент из .archlint contexts ЛИБО восстановить его в графе"
	case "isp-usage-subset", "isp-fat-interface":
		return "разбить интерфейс по кластерам использования (ISP): дать клиенту узкий интерфейс из реально используемых методов"
	case "dip-concrete-dependency", "dip-abstraction-to-detail":
		return "инвертировать зависимость через интерфейс/абстракцию (DIP): зависеть от абстракции, не от конкретного типа"
	case "ocp-open-modification":
		return "расширять поведение через полиморфизм/новый тип (OCP), не модифицируя существующий type-dispatch"
	case "structural-clone":
		return "вынести дублирующийся код в общую функцию/тип (DRY)"
	case "srp-lack-of-cohesion", "srp-multiple-responsibilities", "srp-too-many-methods", "srp-too-many-fields":
		return "разделить ответственности (SRP): выделить независимые обязанности в отдельные типы/функции"
	case "god-class":
		return "разбить god-class на меньшие типы по ответственностям; вынести группы методов/полей"
	case "hub-node":
		return "снизить центральность hub: разорвать часть связей через посредники/интерфейсы или разбить узел"
	case "high-efferent-coupling":
		return "сократить исходящие зависимости: сгруппировать через фасад/интерфейс, убрать лишние импорты"
	case "shotgun-surgery":
		return "собрать рассеянную логику в один модуль, чтобы изменение не затрагивало много файлов"
	case "articulation-point", "bridge-edge", "stability-violation":
		return "сигнал структурной хрупкости (диагностика): рассмотреть дублирование пути/снижение зависимости в узком месте"
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
