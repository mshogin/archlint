package mcp

// SeverityClass — ДЕКЛАРИРУЕМЫЙ класс важности метрики. ОТДЕЛЁН от
// ЭФФЕКТИВНОГО gate-level (ViolationLevel): для open-world-ERROR класс=ERROR
// заявлен сейчас (метрика прошла проверку соундности), но боевая БЛОКИРОВКА требует
// дельта-режима + human-in-loop, чья инфраструктура — Фаза 5. До неё эффективный
// уровень держится в АУДИТ-режиме (отчёт, exit 0), не блок.
type SeverityClass struct {
	// Class — заявленный класс: "ERROR" | "WARNING".
	Class string
	// OpenWorld — условно-соундная метрика: ERROR валиден ТОЛЬКО в дельта-режиме
	// (новое нарушение vs baseline), не абсолютным числом. dead-code: реальность
	// «мёртвости» зависит от полноты R (entry points), которая открыта.
	OpenWorld bool
	// RequiresDelta — боевая блокировка требует дельта-инфраструктуры (Фаза 5,
	// общей для всех ERROR). До неё — аудит.
	RequiresDelta bool
	// HumanInLoop — авто-удаление/фикс только через подтверждение человека
	// (destruction-cost: ошибка удаляет живой код).
	HumanInLoop bool
}

// violationClasses — реестр заявленных классов по Kind нарушения.
//
// Дельта-гейт (EffectiveLevel) активирует в боевом блоке ТОЛЬКО эти
// ERROR-class паттерны и только в дельта-режиме (NEW vs baseline). Градации
// соундности (Ось-1б): closed-world (SCC/слой — безусловно соундны, без замков) и
// open-world (dead-code — условно соунд, обязательны дельта+human-in-loop).
var violationClasses = map[string]SeverityClass{
	// circular-dependency — CLOSED-WORLD ERROR (SCC iff): цикл есть цикл,
	// внешнего допущения нет. Дельта-режим здесь — usability (легаси), не условие
	// соундности -> RequiresDelta=false.
	"circular-dependency": {Class: "ERROR", OpenWorld: false, RequiresDelta: false, HumanInLoop: false},

	// layer-violation — CLOSED-WORLD ERROR относительно объявленного L (back-edge
	// против порядка слоёв Уровень B). Соунд относительно конфига L.
	"layer-violation": {Class: "ERROR", OpenWorld: false, RequiresDelta: false, HumanInLoop: false},

	// dead-code промотирован в ERROR (полная проверка соундности: 0 false-dead на self).
	// open-world: соунден только в дельта-режиме; блокировка — Фаза 5. Удаление —
	// human-in-loop (destruction-cost: ложно-мёртвый удаляет живое).
	"dead-code": {Class: "ERROR", OpenWorld: true, RequiresDelta: true, HumanInLoop: true},

	// forbidden-dependency — CLOSED-WORLD ERROR относительно объявленного запрета
	// (.archlint forbidden: [{from,to}]). Объявленное запрещённое ребро = паттерн по
	// определению (односторонняя импликация). Неактивен без конфига. RequiresDelta=false
	// (соунд относительно конфига; дельта-гейт — usability на легаси, как layer/SCC).
	"forbidden-dependency": {Class: "ERROR", OpenWorld: false, RequiresDelta: false, HumanInLoop: false},

	// deprecated-usage — CLOSED-WORLD ERROR относительно ЯВНЫХ deprecated-маркеров
	// (config-паттерны или атрибут `deprecated`). Использование помеченного устаревшего
	// = дефект по определению. Неактивен без явных маркеров (без широких дефолтов).
	"deprecated-usage": {Class: "ERROR", OpenWorld: false, RequiresDelta: false, HumanInLoop: false},

	// layer-backedge — CLOSED-WORLD ERROR относительно ОБЪЯВЛЕННОГО порядка слоёв
	// (Уровень B). Ребро против порядка (нижний слой -> верхний) = паттерн.
	// Conditional: неактивен без layers-конфига.
	"layer-backedge": {Class: "ERROR", OpenWorld: false, RequiresDelta: false, HumanInLoop: false},

	// isp-usage-subset — промотирован в ERROR после проверки соундности (детерминизм
	// keyed lookup + 0 false-fire стабильно, golden 20x + self дважды NEW=0). CLOSED-WORLD
	// НА ПОДДОМЕНЕ: соунден там, где числитель применим (param-typed свой интерфейс, оба
	// guard'а пройдены), вне поддомена воздерживается (no-verdict). cost=irritation (сужение
	// интерфейса, не destruction) -> HumanInLoop=false. RequiresDelta=true: блокирует только
	// НОВЫЙ запах vs baseline (как dead-code), generic дельта-гейт подхватывает через
	// errorClass/EffectiveLevel без спец-кода. isp-external-narrow НЕ регистрируется
	// (внешний чужой контракт -> всегда WARNING, никогда не ERROR).
	"isp-usage-subset": {Class: "ERROR", OpenWorld: false, RequiresDelta: true, HumanInLoop: false},

	// ghost-component — CLOSED-WORLD ERROR относительно объявленных контекстов
	// (.archlint contexts). Компонент, заявленный в контексте, но отсутствующий в графе
	// = устаревшая декларация (односторонняя импликация ghost⟹дефект). Conditional:
	// неактивен без contexts (self=0). fuzzy-матч консервативен (меньше ложных ghost).
	"ghost-component": {Class: "ERROR", OpenWorld: false, RequiresDelta: false, HumanInLoop: false},

	// ── НЕ-ERROR классификация (ЕДИНЫЙ severity-реестр SSOT) ──
	// Одна точка severity для вывода + health + стража полноты (∀ kind имеет вердикт).
	//
	// WARNING — доказуемые сигналы (прошли WARNING-проверку, precision приемлем):
	"dip-concrete-dependency": {Class: "WARNING"}, // DIP после DTO-фильтра (behavioral concrete)
	"srp-lack-of-cohesion":    {Class: "WARNING"}, // LCOM4>=2, verified WARNING
	"structural-clone":        {Class: "WARNING"}, // изоморфизм формы, canonical fingerprint
	//
	// INFO — магнитуды (порог произволен, не паттерн) / дубли / нет арх-чтения (вердикты по лестнице):
	"srp-multiple-responsibilities": {Class: "INFO"}, // reach-ρ: W1 слабая + W2 провал -> INFO
	"srp-too-many-methods":          {Class: "INFO"}, // размерная магнитуда (#методов > порог)
	"srp-too-many-fields":           {Class: "INFO"}, // размерная магнитуда (#полей > порог)
	"feature-envy":                  {Class: "INFO"}, // магнитуда (Go-фон шумит), не паттерн
	"god-class":                     {Class: "INFO"}, // размерная магнитуда (когезия=LCOM4-дубль)
	"hub-node":                      {Class: "INFO"}, // магнитуда центральности
	"high-efferent-coupling":        {Class: "INFO"}, // магнитуда coupling (порог произволен)
}

// SeverityClassOf возвращает заявленный класс ("ERROR"|"WARNING"|"INFO") или "" если Kind не
// зарегистрирован. ЕДИНЫЙ источник severity (вывод/health/страж полноты читают отсюда).
func SeverityClassOf(kind string) string {
	if c, ok := violationClasses[kind]; ok {
		return c.Class
	}

	return ""
}

// IsInfoClass / IsWarningClass — удобные предикаты над единым реестром.
func IsInfoClass(kind string) bool    { return SeverityClassOf(kind) == "INFO" }
func IsWarningClass(kind string) bool { return SeverityClassOf(kind) == "WARNING" }

// RegisteredKinds — все Kind с объявленным severity-вердиктом (для стража полноты:
// ∀ kind ∈ active_scan_set должен иметь запись здесь).
func RegisteredKinds() map[string]bool {
	out := make(map[string]bool, len(violationClasses))
	for k := range violationClasses {
		out[k] = true
	}

	return out
}

// ClassOf возвращает заявленный класс важности для Kind нарушения (если объявлен).
func ClassOf(kind string) (SeverityClass, bool) {
	c, ok := violationClasses[kind]
	return c, ok
}
