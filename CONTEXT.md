# Archlint - Контекст проекта

Обновлено: 2026-02-12

---

## Что это

Archlint - линтер архитектуры Go проектов. Анализирует исходный код через AST, строит графы (структурные и поведенческие), валидирует по 200+ правилам.

Три основных режима:
1. **Структурный анализ** - какие пакеты, типы, функции есть и как связаны
2. **Поведенческий анализ** - что вызывает что от точки входа (call graphs)
3. **Валидация** - проверка архитектуры по правилам (Python валидаторы)

---

## Быстрый старт

```bash
# Собрать
make build

# Структурный граф (все пакеты, типы, функции, связи)
./bin/archlint collect ./my-project -o architecture.yaml

# Граф вызовов от одной точки входа
./bin/archlint callgraph ./my-project --entry "internal/service.OrderService.Process" -o callgraphs/

# Граф вызовов для всех BPMN-событий
./bin/archlint callgraph ./my-project --bpmn-contexts bpmn-contexts.yaml -o callgraphs/

# Парсинг BPMN процесса
./bin/archlint bpmn order-process.bpmn -o process-graph.yaml

# Валидация архитектуры (200+ правил)
./bin/archlint validate architecture.yaml

# Тесты
make test

# Линт
make lint
```

---

## Что реализовано (спринт 8-12 февраля 2026)

### Spec 0011: BPMN Business Process Graph

Парсинг BPMN 2.0 XML файлов в структурированные графы процессов.

**Пакет:** `pkg/bpmn/`
**CLI:** `archlint bpmn <file.bpmn> [-o output.yaml]`

Поддерживает:
- StartEvent, EndEvent (timer, message, signal, error)
- IntermediateCatchEvent, IntermediateThrowEvent
- Task, ServiceTask, UserTask, ScriptTask, SendTask, ReceiveTask
- ExclusiveGateway, ParallelGateway, InclusiveGateway, EventBasedGateway
- SequenceFlow

Валидация: отсутствие start/end events, broken references, изолированные элементы.

**Файлы:**
- `pkg/bpmn/types.go` - типы (BPMNProcess, BPMNElement, BPMNFlow)
- `pkg/bpmn/adapter.go` - парсинг XML через olive-io/bpmn/schema
- `pkg/bpmn/graph.go` - построение и валидация графа
- `internal/cli/bpmn.go` - CLI команда

### Spec 0013: Event Mapping Configuration

Конфигурация маппинга BPMN-событий на точки входа в код.

**Пакет:** `internal/config/`

Формат `bpmn-contexts.yaml`:
```yaml
contexts:
  order_process:
    bpmn_file: ./processes/order.bpmn
    events:
      - event_id: "ORDER_CREATED"
        event_name: "Order Created"
        entry_point:
          package: "internal/service"
          function: "OrderService.CreateOrder"
          type: "http"   # http | kafka | grpc | cron | custom
```

Валидация: обязательные поля, уникальность event_id, допустимые типы entry_point, существование BPMN файлов.

**Файлы:**
- `internal/config/bpmn_contexts.go` - загрузка и валидация
- `internal/config/bpmn_contexts_test.go` - 16 тестов

### Spec 0012: Event Call Graph

Статический AST-анализ для построения графов вызовов от точек входа.

**Пакет:** `pkg/callgraph/`
**CLI:** `archlint callgraph <dir> [--entry | --bpmn-contexts]`

Ключевые возможности:
- Рекурсивный обход вызовов (DFS) с ограничением глубины
- Разрешение вызовов: прямые, через интерфейс, goroutine, defer
- Детекция циклов
- Генерация PlantUML sequence диаграмм (группировка по пакетам, маркеры async/interface)
- Экспорт в YAML
- Два режима: одиночная точка входа и полный BPMN

**Файлы:**
- `pkg/callgraph/model.go` - типы (CallGraph, CallNode, CallEdge)
- `pkg/callgraph/builder.go` - Builder.Build(entryPoint)
- `pkg/callgraph/walker.go` - CallWalker (DFS + cycle detection)
- `pkg/callgraph/resolver.go` - CallResolver (resolves call targets)
- `pkg/callgraph/event_builder.go` - EventBuilder (BPMN mode)
- `pkg/callgraph/sequence.go` - PlantUML генератор
- `pkg/callgraph/export.go` - YAML экспорт

### Spec 0014: Удаление ручной трассировки

Удалена система runtime-трассировки (tracer.Enter/ExitSuccess/ExitError), заменена статическим AST-анализом.

Удалено:
- `pkg/tracer/` - библиотека трассировки
- `cmd/tracelint/` - бинарник линтера трассировки
- `internal/linter/` - анализатор трассировки
- `internal/cli/trace.go` - CLI команда trace
- Секция `tracerlint:` из `.archlint.yaml`
- 322 вызова tracer из 17 файлов

Specs 0007, 0008, 0009 помечены как Superseded by 0011-0013.

### Расширение GoAnalyzer

В рамках spec 0012 анализатор получил новые возможности:

- Детекция `go func()` (goroutines) и `defer` в вызовах
- Lookup-методы: `LookupFunction`, `LookupMethod`, `LookupType`
- `FindImplementations` - поиск реализаций интерфейса
- `ResolveCallTarget` - разрешение целей вызовов
- `AllFunctions`, `AllMethods`, `AllTypes` - итерация по символам

---

## Архитектура

```
cmd/archlint/main.go
    |
    v
internal/cli/           # Cobra CLI (collect, callgraph, bpmn, validate)
    |
    +-> internal/analyzer/go.go    # AST анализ Go кода
    +-> internal/config/           # Загрузка конфигураций
    +-> internal/model/            # Graph, Node, Edge
    |
    +-> pkg/callgraph/             # Графы вызовов
    |     builder -> walker -> resolver
    |     event_builder (BPMN mode)
    |     sequence (PlantUML) + export (YAML)
    |
    +-> pkg/bpmn/                  # BPMN 2.0 парсинг
          adapter (XML -> model) + graph (валидация)
```

---

## Тесты

61 тест в 4 пакетах:
- `internal/config/` - 16 тестов (BPMN config loading/validation)
- `pkg/bpmn/` - 21 тест (BPMN parsing, graph building, validation)
- `pkg/callgraph/` - 24 теста (builder, sequence, export)
- `tests/` - 1 integration test (full cycle: collect -> callgraph -> PlantUML -> YAML)

```bash
make test   # или go test ./...
```

---

## Конфигурация

### .archlint.yaml

200+ правил валидации, 7 категорий:
- core (DAG, fan-out, coupling, layers)
- solid (SRP, OCP, LSP, ISP, DIP)
- patterns (god class, feature envy, shotgun surgery)
- architecture (clean arch, hexagonal, bounded contexts)
- quality (hotspots, blast radius, security)
- advanced (spectral, graph theory)
- research (topological, algebraic)

### Makefile targets

```
make build     # -> bin/archlint
make test      # go test -v ./...
make lint      # golangci-lint run ./...
make collect   # archlint collect . -> arch/architecture.yaml
make clean     # rm -rf bin/ arch/
make implement # Claude Code с contribution.md
```

---

## Метрики спринта

- Коммитов: 6 (feat + refactor)
- Файлов изменено: 70
- Строк добавлено: +7,786
- Строк удалено: -1,601
- Нетто: +6,185 строк
- Тестов: 61
- Новых пакетов: 2 (pkg/bpmn, pkg/callgraph)
- Удаленных пакетов: 3 (pkg/tracer, cmd/tracelint, internal/linter)

---

## Что дальше (возможные направления)

- Specs `todo/` - пусто (все реализовано)
- `validate.go` имеет 22 lint issue (pre-existing, не блокирует)
- Python валидаторы в `validator/` - не закоммичены
- Можно добавить новые спеки: валидация поведенческих графов, интеграция callgraph + BPMN validation, CI pipeline
