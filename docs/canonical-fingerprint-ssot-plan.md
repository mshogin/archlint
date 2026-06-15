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
   СТАТУС: КРАСНЫЙ (эмпирика выше, корень №3). Требует module-relative pkgID.
3. discriminator = семантический якорь, не line:col/Message. СТАТУС: КРАСНЫЙ (корень №4).
4. scan.go/gate.go слиты в один collect. СТАТУС: НЕ СДЕЛАНО (корень №2).
5. единый active_metric_registry. СТАТУС: НЕ СДЕЛАНО.

## Blast radius корневого фикса (эмпирически измерен)

Фикс №3 (getPkgID -> module-relative `filepath.Rel(scanRoot, dir)`) реализован и прогнан:
build OK, но падают 7 пакетов (callgraph builder ×12, cli metrics, e2e ×13, fullcycle) — они
хардкодят node ID в формате «последние-3-сегмента». Изменение ID-схемы = МАССОВЫЙ golden
rebaseline. Фикс ОТКАЧЕН (не применять вслепую — это продуктовое решение о rebaseline).

## Развилка реализации (требует продуктового решения)

- Вариант R (rebaseline): применить module-relative pkgID (корневой, чистый фикс №3 в модели) +
  пересоздать все golden/baseline-снимки. Стоимость: массовый rebaseline (7 пакетов тестов),
  одноразовая. Выигрыш: страж №2 закрыт в модели, qname канонический навсегда.
- Вариант W (canonical-обёртка): не трогать node-ID-схему; нормализовать qname к module-relative
  ТОЛЬКО в canonical_fingerprint-слое (collect принимает scanRoot, strip префикса до module-root).
  Стоимость: canonical-слой + scanRoot проброс. Выигрыш: golden целы; риск — постфактум-strip
  префикса требует знания module-root (эвристика), менее чист чем R.

Discriminator №4 (Message -> структурный якорь) в любом варианте требует расширения Violation
структурными полями (members/from/to) + перевод детекторов — средний рефактор, независим от R/W.

## Что сделано в этом инкременте (безопасно, обратимо)

- Страж №1 как регрессионный тест (зелёный).
- Эмпирическое доказательство страж №2 (красный) + локализация корня (getPkgID).
- Измерен blast radius фикса №3 (7 пакетов) -> развилка R/W вынесена на решение.
- Корневой фикс НЕ применён вслепую (массовый rebaseline = продуктовое решение).
