# Spec 0015: LSP-сервер для архитектурного анализа

**Metadata:**
- Priority: 0015 (Medium)
- Status: Done
- Created: 2026-03-19
- Effort: M
- Parent Spec: -

---

## Overview

### Problem Statement

archlint работает как CLI-инструмент: при каждом запуске заново парсит весь проект, строит граф и выводит результат. Это означает:

1. **Высокая задержка** — каждый запуск требует полного парсинга AST всего проекта.
2. **Нет интерактивности** — редактор/IDE не получает архитектурных предупреждений в реальном времени.
3. **Нет интеграции с AI-ассистентами** — Claude Code не может запросить архитектурный анализ файла или изменения без запуска отдельного процесса.

### Solution Summary

Реализовать LSP-сервер (`archlint lsp`), который:
- При инициализации парсит весь проект и строит граф зависимостей в памяти
- Отслеживает изменения файлов и обновляет граф инкрементально
- Публикует диагностики (высокий coupling, циклические зависимости)
- Предоставляет custom-команды для архитектурного анализа через `workspace/executeCommand`

### Success Metrics

- `go build ./...` проходит без ошибок
- `go test ./...` проходит без ошибок
- LSP-сервер запускается через `archlint lsp`
- Поддерживаются 4 custom-команды: analyzeFile, analyzeChange, getGraph, getMetrics

---

## Architecture

### Новые компоненты

```
internal/lsp/
  protocol.go        - LSP protocol types (минимальный набор)
  server.go          - JSON-RPC 2.0 сервер через stdio
  state.go           - Потокобезопасное in-memory хранение графа
  analyzer_bridge.go - Мост между LSP и GoAnalyzer
  server_test.go     - Тесты сервера
  state_test.go      - Тесты состояния

internal/cli/
  lsp.go             - CLI-команда "archlint lsp"
```

### Транспорт

JSON-RPC 2.0 через stdio (стандартный LSP-транспорт). Без внешних зависимостей — реализация протокола inline.

### Потокобезопасность

`sync.RWMutex` для доступа к графу, так как LSP-обработчики могут вызываться конкурентно.

---

## Requirements

### R1: CLI-команда `archlint lsp`

- Новая подкоманда cobra: `archlint lsp`
- Флаги: `--log-file`, `--verbose`
- Запускает LSP-сервер через stdio

### R2: Инициализация проекта

- На `initialize` — парсит `rootUri` через `GoAnalyzer.Analyze()`
- Хранит граф и анализатор в памяти
- Возвращает capabilities с поддержкой textDocumentSync и executeCommand

### R3: Отслеживание изменений файлов

- `textDocument/didOpen` — отслеживание открытых файлов
- `textDocument/didSave` — перепарсинг файла и обновление графа
- `workspace/didChangeWatchedFiles` — batch-обновление при изменениях в workspace

### R4: Публикация диагностик

- При сохранении файла — публикация `textDocument/publishDiagnostics`
- Предупреждение: высокий efferent coupling (> 10 зависимостей)
- Ошибка: циклические зависимости через import-рёбра

### R5: Custom-команды (workspace/executeCommand)

- `archlint.analyzeFile` — полный анализ файла (типы, функции, зависимости, метрики)
- `archlint.analyzeChange` — анализ влияния изменения на архитектуру (затронутые узлы, связи, уровень impact)
- `archlint.getGraph` — текущий граф (с опциональной фильтрацией по пакету)
- `archlint.getMetrics` — метрики coupling/instability для пакета

---

## Acceptance Criteria

- [x] AC1: `go build ./...` проходит
- [x] AC2: `go test ./...` проходит
- [x] AC3: `archlint lsp` запускает LSP-сервер
- [x] AC4: initialize парсит проект и возвращает capabilities
- [x] AC5: didSave перепарсивает файл и публикует диагностики
- [x] AC6: archlint.analyzeFile возвращает типы, функции, зависимости
- [x] AC7: archlint.analyzeChange возвращает затронутые узлы и impact
- [x] AC8: archlint.getGraph возвращает граф (с фильтрацией)
- [x] AC9: archlint.getMetrics возвращает coupling-метрики
- [x] AC10: Все новые файлы покрыты тестами

---

## Testing Strategy

### Unit-тесты

- `TestInitialize` — проверка инициализации с temp-директорией
- `TestExecuteCommandAnalyzeFile` — анализ файла с типами, функциями, методами
- `TestExecuteCommandGetGraph` — получение графа
- `TestExecuteCommandGetMetrics` — получение метрик
- `TestExecuteCommandAnalyzeChange` — анализ влияния изменений
- `TestShutdown` — корректный shutdown
- `TestMethodNotFound` — обработка неподдерживаемых методов
- `TestURIToPath` — конвертация file:// URI в пути
- `TestReadWriteMessage` — JSON-RPC read/write
- `TestStateInitialize` — инициализация состояния
- `TestStateReparseFile` — инкрементальное обновление
- `TestStateFileVersions` — отслеживание версий файлов
- `TestStateRootDir` — хранение rootDir
- `TestStateGetGraphReturnsACopy` — копирование графа

---

## Notes

### Design Decisions

**Почему без внешних LSP-библиотек:**
Реализация LSP-протокола inline (JSON-RPC 2.0 через stdio) минимизирует зависимости и упрощает сборку. Используется только подмножество LSP-протокола, необходимое для архитектурного анализа.

**Стратегия обновления графа:**
Полный ребилд при изменении файла (вместо инкрементального обновления). Для типичных Go-проектов (< 1000 файлов) занимает < 100ms, что достаточно быстро для интерактивного использования. Это значительно проще инкрементального подхода и гарантирует корректность.

### Claude Code Integration

Примеры использования с Claude Code:

```json
// Анализ файла
{"method": "workspace/executeCommand", "params": {"command": "archlint.analyzeFile", "arguments": ["internal/service/order.go"]}}

// Анализ влияния изменения
{"method": "workspace/executeCommand", "params": {"command": "archlint.analyzeChange", "arguments": ["internal/service/order.go"]}}

// Получение графа пакета
{"method": "workspace/executeCommand", "params": {"command": "archlint.getGraph", "arguments": ["internal/service"]}}

// Метрики пакета
{"method": "workspace/executeCommand", "params": {"command": "archlint.getMetrics", "arguments": ["internal/service"]}}
```
