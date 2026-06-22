# Единая канонизация fingerprint (SSOT) — план и разведка

Класс инцидентов «опорные точки сравнения разошлись» = структурная необходимость единого
SSOT канонизации идентичности нарушения, не баг×N. Этот документ фиксирует разведку корней,
эмпирический blast radius корневого фикса, развилку реализации и 5 стражей приёмки.

## Корни класса (разведка по коду)

1. Асимметрия baseline<->scan по набору метрик: `cli/gate.go errorClassViolations` собирал
   SCC+layer+dead-code, но НЕ ISP; `cli/scan.go` ISP собирал -> baseline пуст по ISP -> ложные NEW.
   Точечно закрыто (ISP добавлен в errorClassViolations), но корень — ДВА пути сбора.
2. Дубль-путь сбора violations: `cli/scan.go` (полный набор) vs `cli/gate.go errorClassViolations`
   (подмножество). Один и тот же набор детекторов описан в двух местах -> расходится при добавлении.
3. qname зависит от корня скана: `internal/analyzer/go_parser.go getPkgID` берёт «последние 3
   сегмента абсолютного пути» -> pkgID нестабилен относительно способа передачи корня
   («.» vs абсолютный путь vs worktree). Эмпирика: collect(абс) = `archlint-repo/internal/mcp.X`,
   collect(`.`) = `internal/mcp.X` -> РАЗНЫЕ fingerprints. Был ложный always-red в arch-diff PR
   (обойдён `cd worktree && baseline .`, но корень в модели).
4. (latent) baseline `Fingerprint` для circular/layer/forbidden/deprecated/layer-backedge завязан
   на `v.Message` (display-строка) -> переформулировка сообщения / сдвиг -> ложный NEW на
   несвязанном рефакторинге. Нужен СЕМАНТИЧЕСКИЙ якорь (структурные поля), не Message.

## Целевой SSOT

- `canonical_fingerprint`: f(v) = (metric_kind, canonical_qname [module-relative, инвариант корню],
  semantic_discriminator [структурный якорь, НЕ line:col/Message]). Все сборы/сравнения через него.
- единый `active_metric_registry`: baseline и scan берут набор детекторов оттуда (один набор).
- один `collect(tree) -> []canonical_violation`; `gate(A,B) = delta(collect(A), collect(B))`.
  Слить `scan.go` + `gate.go errorClassViolations` в один collect («один сборщик, два входа»).

## 5 стражей приёмки

1. (★главный) `delta(collect(X), collect(X)) = ∅` на любом X — дельта дерева с собой пуста.
   СТАТУС: ЗЕЛЁНЫЙ (TestCanonical_Guard1_DeltaTreeWithSelfEmpty) — на одном пути идентичность
   детерминирована.
2. `collect(path-аргумент) == collect(".")` — t_root-инвариантность fingerprint.
   СТАТУС: ЗЕЛЁНЫЙ (TestCanonical_Guard2_TRootInvariance, корень №3 закрыт — module-relative pkgID).
3. discriminator = семантический якорь, не line:col/Message. СТАТУС: ЗЕЛЁНЫЙ
   (TestCanonical_Guard3_DiscriminatorNotMessage, корень №4 — Violation.Anchor).
4. scan.go/gate.go слиты в один collect. СТАТУС: ЗЕЛЁНЫЙ (CollectErrorClassViolations, корень №2 —
   gate.go и scan.go вызывают единый сборщик; TestCanonical_Guard45).
5. единый active_metric_registry. СТАТУС: ЗЕЛЁНЫЙ (activeErrorClassRegistry, корень №5 — C2 по конструкции).
6. delta при НЕСВЯЗАННОМ СДВИГЕ СТРОК / переформулировке = ∅. СТАТУС: ЗЕЛЁНЫЙ
   (TestCanonical_Guard6_UnrelatedMessageShiftDeltaEmpty, корень №4).

## Края module-relative (соундность-ревью; не отменяют R, очерчивают границу)

1. t_root-инвариантность держится ТОЛЬКО при ЕДИНОМ go-module (import path от go.mod, не от
   файловой позиции). ЛОМАЕТСЯ: нет go.mod (GOPATH-legacy); nested go.mod/go.work. ПРЕДУСЛОВИЕ:
   цель скана = единый go-module (для archlint-on-archlint и большинства репо ок). baseline и scan
   ОБЯЗАНЫ резолвить ОДИН module-root. Для общего archlint на чужих nested-репо — отдельный резолв
   module-root (backlog). Текущая реализация: rel от scanRoot (корень скана = предполагаемый module-root).
2. R = НЕОБХОДИМАЯ часть C1 (t_root-ось qname), НЕ весь класс. Полная соундность = R + единый collect
   (корень №2, t_path) + единый registry (корень №5, C2). R не заменяет слияние.
3. Корневой пакет скана (rel=".") -> Go-package-name (инвариантно, осмысленно), НЕ "" (пустой ломал
   metrics-агрегацию by package).

## Blast radius корневого фикса (эмпирически измерен)

Фикс №3 (getPkgID -> module-relative `filepath.Rel(scanRoot, dir)`) реализован и прогнан:
build OK, но падают 7 пакетов (callgraph builder ×12, cli metrics, e2e ×13, fullcycle) — они
хардкодят node ID в формате «последние-3-сегмента». Изменение ID-схемы = МАССОВЫЙ golden
rebaseline. Фикс ОТКАЧЕН (не применять вслепую — это продуктовое решение о rebaseline).

## Решение: R (rebaseline) — выбран

R сильнее W: qname канонична В ИСТОЧНИКЕ (модель) -> C1 (одна канонизация) = ТЕОРЕМА КОНСТРУКЦИИ
(нельзя собрать в обход). W (обёртка-strip) порождает ДВА представления qname (модельный +
canonical-strip эвристикой) -> C1 держится дисциплиной «идти через обёртку», не конструкцией +
дубль-представление (то, с чем archlint борется). Цена R (golden-rebaseline) одноразовая.

## КОРЕНЬ №3 — ВЫПОЛНЕН (R-rebaseline)

- module-relative pkgID в модели: `getPkgID` = `filepath.Rel(scanRoot, dir)`; корневой пакет ->
  Go-package-name (не "", инвариантно). go.go: `parser.scanRoot = Clean(dir)`.
- Rebaseline golden-diff = СТРОГО qname-формат (verified, ноль изменений набора/severity):
  - callgraph builder_test.go: 15 entry «testdata/sample.X» -> «sample.X» (pkgID от scanRoot=testdata/sample).
  - tests/fullcycle_test.go: entry «testdata/sample.Calculator.Calculate» -> «sample.Calculator.Calculate».
  - cli metrics: краевой случай пустого pkgID корневого пакета закрыт (Go-package-name).
- Стражи №1, №2 ЗЕЛЁНЫЕ (TestCanonical_Guard1/Guard2). Все тесты зелёные, кроме e2e (предсуществующий
  cwd-артефакт монорепо «cannot find main module», 12 шт — НЕ связан с этим фиксом).

## КОРЕНЬ №4 — ВЫПОЛНЕН (семантический якорь)

- Violation += Anchor (структурный ключ БЕЗ display/line:col). Fingerprint: приоритет Anchor,
  Message только legacy-fallback.
- Anchor заполнен в детекторах: circular-dependency (scc:отсортир.члены), layer-violation
  (layer:from->to), forbidden-dependency (forbidden:src->dst), deprecated-usage (deprecated:u->d),
  layer-backedge (backedge:from->to), degradation circular (scc:cycle).
- Стражи №3 (Fingerprint не зависит от Message) и №6 (delta при переформулировке Message = ∅) ЗЕЛЁНЫЕ.

## КОРНИ №2 + №5 — ВЫПОЛНЕНЫ (единый collect + registry)

- internal/mcp/collect.go: activeErrorClassRegistry (единый реестр ERROR-class детекторов) +
  CollectErrorClassViolations (единый сборщик, «один сборщик, два входа»).
- cli/gate.go errorClassViolations -> делегирует CollectErrorClassViolations.
- cli/scan.go: прямой список 7 детекторов заменён на CollectErrorClassViolations.
- Устранена асимметрия шире ISP: baseline теперь снимает forbidden/deprecated/layer-backedge/ghost
  (раньше gate.go их не включал -> ложные NEW). Симметрия baseline<->scan = C2 по конструкции.

## SSOT ЗАВЕРШЁН — все 6 стражей ЗЕЛЁНЫЕ

| страж | суть | статус |
|-------|------|--------|
| №1 | delta(collect(X),collect(X))=∅ | ЗЕЛЁНЫЙ |
| №2 | t_root: collect(абс)==collect(.) | ЗЕЛЁНЫЙ (корень №3) |
| №3 | discriminator семантический (не Message) | ЗЕЛЁНЫЙ (корень №4) |
| №4 | единый collect (scan+gate) | ЗЕЛЁНЫЙ (корень №2) |
| №5 | единый active_metric_registry | ЗЕЛЁНЫЙ (корень №5) |
| №6 | delta при сдвиге строк/переформулировке=∅ | ЗЕЛЁНЫЙ (корень №4) |

Тесты: TestCanonical_Guard1/Guard2/Guard3/Guard45/Guard6. golden-rebaseline diff verified (только
qname-схема, корень №3). build + все тесты зелёные кроме e2e (предсуществующий cwd-артефакт монорепо).
ПРЕДУСЛОВИЕ (граничное условие, соундность-ревью): цель скана = единый go-module; nested go.work -> backlog.
