# archlint-гейт для agents-platform DEV-pipeline (ACTIVATION-READY)

Статус: **ПОДГОТОВЛЕНО, НЕ АКТИВИРОВАНО** (упреждение #14). Артефакт держится в archlint.
По отмашке Михаила копируется в agents-platform и подключается к DEV-pipeline. До отмашки —
agents-platform НЕ трогается (его pipeline = санкция Михаила).

## Что это

`archlint-dev-gate.sh` — delta-гейт архитектуры для feat-ветки DEV-агента. Блокирует push/PR,
если DEV ввёл **НОВЫЕ** ERROR-class нарушения архитектуры (dead-code / ISP / forbidden /
layer-backedge / deprecated / ghost) относительно base (origin/main). Старые нарушения base НЕ
блокируют (легаси не красит). WARNING/INFO (DIP/SRP/clone/магнитуды) НЕ блокируют (Ось-1: сигнал, не блок).

## DEV-workflow agents-platform (изучено read-only)

`internal/pipeline/dev_engine.go` `runOne()`:
1. `PrepareWorkdir` — clone целевого `p.Repo`, checkout `feat/<EXT_KEY>` от base.
2. LLM (DEV-агент) пишет Go в workdir.
3. `AutoCommitWIP` -> `PushBranch` (WIP, даже при сбое LLM).
4. Status=done -> `getBranchHeadIfAheadOfMain` (реальный HEAD feat-ветки).
5. `PushBranch` -> `gh pr create/update`.

## Точка вставки гейта (при активации)

`dev_engine.go`, между получением реального SHA и push — после строки
`sha, branchErr := getBranchHeadIfAheadOfMain(...)` (feat подтверждённо ahead of base),
ПЕРЕД `PushBranch(ctx, workdir, p.ExtKey)`:

```go
// archlint architecture delta-gate: блок push при НОВЫХ ERROR-регрессиях vs base.
if gateErr := runArchlintGate(ctx, workdir, baseRef); gateErr != nil {
    return fmt.Errorf("archlint gate: %w", gateErr) // fail task -> rework, без push
}
```

где `runArchlintGate` запускает `archlint-dev-gate.sh <workdir> <baseRef>` и трактует exit-код.
Альтернатива (мягче): не fail task, а пометить PR лейблом/комментом «arch-regression» (gate exit 1).

## Использование

```bash
archlint-dev-gate.sh <workdir> [base-ref]      # base-ref default: origin/main
```

Env:
- `ARCHLINT_BIN` — путь к archlint (default: `archlint` из PATH). Для личного контура
  agents-platform — локальный бинарь (без push-наружу).
- `ARCHLINT_EXCLUDES` — каталоги-исключения (default: `data,.worktree` — worktree-копии
  agents-platform, чтобы гейт не сканировал чужие изолированные деревья агентов).
- `GATE_STRICT` — `1`: ошибка гейта (archlint/git сбой) = блок (exit 2 трактуется строго);
  `0` (default): fail-open (сбой инфраструктуры не блокирует DEV).

Exit-коды:
- `0` — чисто (нет НОВЫХ ERROR vs base) -> push разрешён.
- `1` — регрессия (NEW ERROR-паттерн) -> блок.
- `2` — ошибка гейта (archlint/git) -> по политике `GATE_STRICT`.

## Соундность delta-режима

- baseline снимается с base-состояния во **временном git-worktree** (`git worktree add --detach`)
  -> DEV workdir НЕ трогается (никакого checkout/stash в рабочем дереве агента).
- Блокирует ТОЛЬКО дельту (NEW vs base), не абсолютный счёт -> на легаси-репо с давними
  нарушениями гейт не «всё красное», ловит именно регрессию DEV.
- Не-Go репо (нет go.mod) -> abstain (exit 0, гейт пропускается; archlint боевой = Go).

## Failing-case (доказано локально, такт-2 ready)

base (чистый) -> feat вводит регрессии (unreachable dead-code + ISP-narrow интерфейса):

```bash
# feat с регрессиями vs base:
archlint-dev-gate: BLOCKED: 2 new ERROR-class architecture regression(s) vs main
     1 "kind": "dead-code"
     1 "kind": "isp-usage-subset"
GATE EXIT=1

# контр — feat без регрессий (= base-код):
archlint-dev-gate: OK: no new ERROR-class architecture regressions vs main
GATE EXIT=0
```

Воспроизведение: см. историю прогона (git base/feat с unreachable-func + сужением интерфейса
Store до одного метода) — гейт даёт exit 1 на регрессии, exit 0 на чистом.

## Наблюдение (для сведения, НЕ блокер гейта)

Go-репозитории НЕ имеют import-циклов пакетов (запрет компилятора Go) -> `circular-dependency`
для Go-целей неактуален. Гейт опирается на работающие на Go ERROR-детекторы: dead-code, ISP,
forbidden, layer-backedge, deprecated, ghost. Для `layer-violation`/`forbidden` нужен `.archlint`
конфиг в целевом репо (без него — эти правила неактивны, гейт ловит остальные).
