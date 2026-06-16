#!/usr/bin/env bash
#
# archlint-dev-gate.sh — delta-гейт архитектуры для DEV-feat-ветки agents-platform.
#
# ACTIVATION-READY (упреждение #14): артефакт ПОДГОТОВЛЕН, НЕ активирован. По отмашке
# Михаила копируется в agents-platform и вызывается из DEV-pipeline (см. README.md,
# точка вставки dev_engine.go: после getBranchHeadIfAheadOfMain, ПЕРЕД PushBranch).
#
# Что делает: сравнивает feat-ветку DEV-агента с base (origin/main) в ДЕЛЬТА-режиме —
# блокирует ТОЛЬКО НОВЫЕ ERROR-class паттерны (cycles/layering/dead-code/forbidden/
# deprecated/ISP/ghost), введённые DEV в этой ветке. Старые нарушения base НЕ блокируют
# (легаси не красит). WARNING/INFO (DIP/SRP/clone/магнитуды) НЕ блокируют (Ось-1: не блок).
#
# Соундность: baseline снимается с base-состояния во ВРЕМЕННОМ git-worktree -> DEV workdir
# НЕ трогается (никакого checkout/stash в рабочем дереве агента). no-baseline невозможен
# (всегда строим из base) -> детерминированная дельта.
#
# Exit:
#   0 - чисто (нет НОВЫХ ERROR-паттернов vs base) -> push разрешён.
#   1 - регрессия (NEW ERROR-паттерн) -> блок (не пушить / пометить PR / fail task).
#   2 - ошибка гейта (archlint/git сбой) -> fail-open или fail-closed по политике (см. STRICT).
#
# Usage:
#   archlint-dev-gate.sh <workdir> [base-ref]
# Env:
#   ARCHLINT_BIN       путь к archlint (default: archlint из PATH)
#   ARCHLINT_EXCLUDES  каталоги-исключения (default: data,.worktree — worktree-копии agents-platform)
#   GATE_STRICT        1 = ошибка гейта (exit 2) трактуется как блок; 0 = fail-open (default 0)

set -euo pipefail

WORKDIR="${1:?usage: archlint-dev-gate.sh <workdir> [base-ref]}"
BASE_REF="${2:-origin/main}"
ARCHLINT="${ARCHLINT_BIN:-archlint}"
EXCLUDES="${ARCHLINT_EXCLUDES:-data,.worktree}"
STRICT="${GATE_STRICT:-0}"

err() { echo "archlint-dev-gate: $*" >&2; }

# Гейт применим только к Go-репо (archlint боевой = Go). Нет go.mod -> abstain (exit 0, не мешаем).
if ! find "$WORKDIR" -maxdepth 2 -name go.mod -not -path '*/vendor/*' 2>/dev/null | grep -q .; then
	err "no go.mod in $WORKDIR -> abstain (non-Go repo, gate skipped)"
	exit 0
fi

if ! command -v "$ARCHLINT" >/dev/null 2>&1 && [ ! -x "$ARCHLINT" ]; then
	err "archlint binary not found ($ARCHLINT)"
	[ "$STRICT" = "1" ] && exit 1 || exit 2
fi

# 1. base-состояние во ВРЕМЕННОМ worktree (DEV workdir не трогаем).
BASE_WT="$(mktemp -d)"
BASELINE="$(mktemp).json"
cleanup() {
	git -C "$WORKDIR" worktree remove --force "$BASE_WT" >/dev/null 2>&1 || rm -rf "$BASE_WT"
	rm -f "$BASELINE"
}
trap cleanup EXIT

if ! git -C "$WORKDIR" worktree add --detach "$BASE_WT" "$BASE_REF" >/dev/null 2>&1; then
	err "cannot create base worktree for $BASE_REF"
	[ "$STRICT" = "1" ] && exit 1 || exit 2
fi

# 2. baseline ERROR-class паттернов base-состояния (+ ocp-dispatch-site факты).
if ! "$ARCHLINT" baseline "$BASE_WT" --exclude "$EXCLUDES" -o "$BASELINE" >/dev/null 2>&1; then
	err "archlint baseline failed on base $BASE_REF"
	[ "$STRICT" = "1" ] && exit 1 || exit 2
fi

# 3. scan feat-ветки с baseline -> дельта. blocking = НОВЫЕ ERROR-class паттерны (регрессии).
#    threshold большой: магнитуды (WARNING/INFO) не валят count-гейт; блокирует ТОЛЬКО дельта-ERROR.
RESULT="$("$ARCHLINT" scan "$WORKDIR" --baseline "$BASELINE" --exclude "$EXCLUDES" --format json --threshold 1000000 2>/dev/null || true)"
if [ -z "$RESULT" ]; then
	err "archlint scan produced no output"
	[ "$STRICT" = "1" ] && exit 1 || exit 2
fi

BLOCKING="$(printf '%s' "$RESULT" | grep -o '"blocking":[[:space:]]*[0-9]*' | grep -o '[0-9]*' | head -1)"
BLOCKING="${BLOCKING:-0}"

if [ "$BLOCKING" -gt 0 ]; then
	err "BLOCKED: $BLOCKING new ERROR-class architecture regression(s) vs $BASE_REF"
	printf '%s\n' "$RESULT" | grep -o '"kind":[[:space:]]*"[^"]*"' | sort | uniq -c | sed 's/^/  /' >&2 || true
	exit 1
fi

err "OK: no new ERROR-class architecture regressions vs $BASE_REF"
exit 0
