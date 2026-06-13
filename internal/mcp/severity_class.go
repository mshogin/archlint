package mcp

// SeverityClass — ДЕКЛАРИРУЕМЫЙ класс важности метрики (DR-0029). ОТДЕЛЁН от
// ЭФФЕКТИВНОГО gate-level (ViolationLevel): для open-world-ERROR класс=ERROR
// заявлен сейчас (метрика прошла горнило), но боевая БЛОКИРОВКА требует
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
// Дельта-гейт (EffectiveLevel, DR-0034) активирует в боевом блоке ТОЛЬКО эти
// ERROR-class паттерны и только в дельта-режиме (NEW vs baseline). Градации
// соундности (Ось-1б): closed-world (SCC/слой — безусловно соундны, без замков) и
// open-world (dead-code — условно соунд, обязательны дельта+human-in-loop).
var violationClasses = map[string]SeverityClass{
	// circular-dependency — CLOSED-WORLD ERROR (SCC iff, DR-0005): цикл есть цикл,
	// внешнего допущения нет. Дельта-режим здесь — usability (легаси), не условие
	// соундности -> RequiresDelta=false.
	"circular-dependency": {Class: "ERROR", OpenWorld: false, RequiresDelta: false, HumanInLoop: false},

	// layer-violation — CLOSED-WORLD ERROR относительно объявленного L (back-edge
	// против порядка слоёв, DR-0009 Уровень B). Соунд относительно конфига L.
	"layer-violation": {Class: "ERROR", OpenWorld: false, RequiresDelta: false, HumanInLoop: false},

	// dead-code промотирован в ERROR (полное горнило соундности: 0 false-dead на self).
	// open-world: соунден только в дельта-режиме; блокировка — Фаза 5. Удаление —
	// human-in-loop (destruction-cost: ложно-мёртвый удаляет живое).
	"dead-code": {Class: "ERROR", OpenWorld: true, RequiresDelta: true, HumanInLoop: true},
}

// ClassOf возвращает заявленный класс важности для Kind нарушения (если объявлен).
func ClassOf(kind string) (SeverityClass, bool) {
	c, ok := violationClasses[kind]
	return c, ok
}
