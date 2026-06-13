package mcp

import "github.com/mshogin/archlint/internal/archlintcfg"

// EffectiveLevel — ГЕЙТ-уровень нарушения с учётом дельта-режима (Фаза 5, DR-0034).
// Привязка к severity_class через errorClass/ClassOf:
//
//	ERROR-class + baseline == nil      -> Telemetry  (NO-BASELINE -> NO-BLOCK, п.2:
//	                                                   первый прогон НЕ absolute, не блокирует всё)
//	ERROR-class + baseline + EXISTING  -> Telemetry  (давнее нарушение, не регрессия)
//	ERROR-class + baseline + NEW       -> Taboo       (регрессия -> hard-block; для dead-code
//	                                                   блок = сигнал, удаление human-in-loop отдельно)
//	НЕ ERROR-class                     -> ViolationLevel(v,cfg)  (WARNING/INFO как раньше)
//
// fail-safe (п.4): строгий fingerprint -> ошибки в безопасную сторону (над-блок =
// irritation, не под-блок = проскок регрессии).
func EffectiveLevel(v Violation, cfg *archlintcfg.Config, baseline *Baseline) archlintcfg.Level {
	if errorClass(v.Kind) {
		if baseline == nil {
			return archlintcfg.LevelTelemetry // no-baseline -> no-block (audit fallback)
		}
		if baseline.Contains(v) {
			return archlintcfg.LevelTelemetry // существующее -> аудит
		}
		return archlintcfg.LevelTaboo // NEW ERROR-паттерн -> блок
	}
	return ViolationLevel(v, cfg)
}
