# Spec 0014: Удаление системы ручной трассировки

**Metadata:**
- Priority: 0014 (Medium)
- Status: Done
- Created: 2026-02-08
- Effort: S
- Parent Spec: -

---

## Overview

### Problem Statement

Система ручной трассировки (specs 0007-0009) требовала инструментирования каждой функции вызовами `tracer.Enter()` / `tracer.ExitSuccess()` / `tracer.ExitError()`. Это создавало три проблемы:

1. **Высокая стоимость внедрения** - каждая функция должна содержать 3+ вызова tracer (Enter + Exit на каждый return path), что загрязняет бизнес-логику.
2. **Хрупкость** - забытый ExitError перед return приводит к некорректной трассировке; tracerlint отлавливал это, но добавлял ещё один шаг в CI.
3. **Замена готова** - specs 0011-0013 реализуют event-driven подход через статический AST-анализ BPMN-событий, не требующий инструментирования кода.

### Solution Summary

Удалить все компоненты ручной трассировки:
- Библиотека `pkg/tracer/`
- Бинарник `cmd/tracelint/`
- Анализатор `internal/linter/`
- CLI-команда `internal/cli/trace.go`
- Конфигурация `tracerlint:` в `.archlint.yaml`
- Ссылки в Makefile

### Success Metrics

- Кодовая база уменьшилась на ~700 строк (4 пакета)
- `go build ./...` проходит без ошибок
- `go test ./...` проходит без ошибок
- `golangci-lint run ./...` не находит dead code
- Никаких импортов `pkg/tracer` или `internal/linter` в проекте

---

## Architecture

### Удаляемые компоненты

```
УДАЛЯЕМ:

pkg/tracer/
  trace.go              - Trace, Call типы; StartTrace, Enter, ExitSuccess, ExitError, StopTrace, Save
  context_generator.go  - GenerateContextsFromTraces, LoadTrace, GenerateContextFromTrace,
                          GenerateSequenceDiagram, MatchComponentPattern

cmd/tracelint/
  main.go               - Entry point (singlechecker.Main)

internal/linter/
  tracerlint.go          - Analyzer: проверка tracer.Enter/Exit в функциях

internal/cli/
  trace.go               - CLI подкоманда "trace" (генерация DocHub контекстов из JSON)

.archlint.yaml           - Секция tracerlint: (exclude_packages)

Makefile                 - TRACELINT переменная, lint target (tracelint часть)
```

### Что остаётся

Все остальные компоненты archlint (collect, analyze, rules engine, 100+ правил) не затронуты. Новый подход к behavioral analysis реализуется в specs 0011-0013.

---

## Requirements

### R1: Удалить библиотеку Tracer

**Описание:** Полное удаление пакета pkg/tracer/

- Удалить директорию `pkg/tracer/` целиком (trace.go, context_generator.go)
- Найти и удалить все `import "archlint/pkg/tracer"` в проекте
- Проверить go.mod на зависимости, используемые только tracer (если есть - удалить через `go mod tidy`)

### R2: Удалить Tracerlint

**Описание:** Удаление статического анализатора и его бинарника

- Удалить директорию `cmd/tracelint/` (main.go)
- Удалить директорию `internal/linter/` (tracerlint.go)
- Удалить секцию `tracerlint:` из `.archlint.yaml` (строки с exclude_packages)
- Обновить Makefile:
  - Удалить переменную `TRACELINT := $(BIN_DIR)/tracelint`
  - Удалить блок `--- tracelint ---` из lint target
  - Убрать `trace` из `.PHONY` (если относится к trace команде)

### R3: Удалить CLI-команду Trace

**Описание:** Удаление подкоманды trace из CLI

- Удалить файл `internal/cli/trace.go`
- Удалить регистрацию команды trace в root command (если есть `addCommand` вызов)
- Обновить help text, если команда trace упоминается

### R4: Обновить документацию

**Описание:** Убрать упоминания трассировки из документации

- Обновить README.md - убрать секции про tracing
- Пометить specs 0007, 0008, 0009 статусом "Superseded by 0011-0013":
  - В каждом файле изменить `Status: Done` на `Status: Superseded`
  - Добавить строку `Superseded by: 0011, 0012, 0013`

### R5: Обеспечить чистую сборку

**Описание:** После удаления проект должен собираться и проходить все проверки

- `go build ./...` - без ошибок компиляции
- `go test ./...` - все тесты проходят
- `go mod tidy` - нет лишних зависимостей
- `golangci-lint run ./...` - нет dead code, unused imports
- Makefile targets (build, test, lint, collect) работают корректно

---

## Acceptance Criteria

- [ ] AC1: Директория `pkg/tracer/` удалена
- [ ] AC2: Директория `cmd/tracelint/` удалена
- [ ] AC3: Директория `internal/linter/` удалена
- [ ] AC4: Файл `internal/cli/trace.go` удалён
- [ ] AC5: Секция `tracerlint:` удалена из `.archlint.yaml`
- [ ] AC6: Makefile не содержит ссылок на tracelint
- [ ] AC7: `go build ./...` проходит успешно
- [ ] AC8: `go test ./...` проходит успешно
- [ ] AC9: `golangci-lint run ./...` проходит без ошибок dead code
- [ ] AC10: Specs 0007, 0008, 0009 помечены как Superseded

---

## Implementation Steps

**Step 1:** Удалить код трассировки
- Удалить `pkg/tracer/`, `cmd/tracelint/`, `internal/linter/`
- Удалить `internal/cli/trace.go`
- Удалить регистрацию trace команды в root command

**Step 2:** Обновить конфигурацию
- Удалить секцию `tracerlint:` из `.archlint.yaml`
- Обновить Makefile: убрать TRACELINT переменную, tracelint из lint target
- Запустить `go mod tidy`

**Step 3:** Проверить сборку
- `go build ./...`
- `go test ./...`
- `golangci-lint run ./...`

**Step 4:** Обновить документацию
- Обновить README.md
- Пометить specs 0007, 0008, 0009 как Superseded by 0011-0013

---

## Testing Strategy

### Проверка удаления

- Убедиться, что в проекте нет импортов `pkg/tracer` и `internal/linter`:
  ```bash
  grep -r "pkg/tracer" --include="*.go" .
  grep -r "internal/linter" --include="*.go" .
  ```
- Оба grep должны вернуть пустой результат

### Сборка и тесты

- `go build ./...` - компиляция без ошибок
- `go test ./...` - все существующие тесты проходят
- `go vet ./...` - нет предупреждений

### Функциональная проверка

- `make build` - бинарник собирается
- `make lint` - линтинг работает (без tracelint)
- `make collect` - сбор структурного графа работает

---

## Notes

### Design Decisions

**Почему удаляем, а не deprecate:**
Ручная трассировка полностью заменяется статическим AST-анализом (specs 0011-0013). Оставлять deprecated код - это мертвый груз, увеличивающий cognitive load и время сборки. Чистое удаление предпочтительнее.

**Порядок удаления:**
Сначала код, потом конфигурация, потом документация. Это позволяет на каждом шаге проверять сборку и ловить пропущенные зависимости.

### Breaking Changes

- Удалён пакет `pkg/tracer` - если кто-то импортировал его как библиотеку, импорт перестанет работать
- Удалён бинарник `tracelint` - если использовался в CI pipeline внешних проектов, нужно убрать
- Удалена CLI-команда `archlint trace` - скрипты, вызывающие `archlint trace`, сломаются
- Секция `tracerlint:` в `.archlint.yaml` больше не читается

### Migration Guide

Для пользователей, которые использовали ручную трассировку:

1. **Удалить из кода** все вызовы `tracer.Enter()`, `tracer.ExitSuccess()`, `tracer.ExitError()`
2. **Удалить импорты** `"archlint/pkg/tracer"` из всех файлов
3. **Удалить секцию** `tracerlint:` из `.archlint.yaml`
4. **Убрать tracelint** из CI pipeline (Makefile, GitHub Actions, etc.)
5. **Перейти на specs 0011-0013** - новый подход не требует инструментирования кода; behavioral analysis строится через статический AST-анализ BPMN-событий

### References

- Spec 0007: Tracer Library (Superseded)
- Spec 0008: Trace Command and Context Generator (Superseded)
- Spec 0009: Tracerlint Linter (Superseded)
- Spec 0011: Event-driven AST analysis (replacement)
- Spec 0012: BPMN event mapping (replacement)
- Spec 0013: Behavioral graph generation (replacement)
