# Spec Templates для archlint

[EN](README.en.md) | **RU**

Шаблоны для создания спецификаций с использованием Markdown + PlantUML (C4 + UML).

Эти шаблоны предназначены для **spec driven development** с Claude Code - детальные спецификации позволяют AI эффективно реализовывать функциональность.

---

## Доступные шаблоны

### 1. spec-template.md (Универсальный)
**Назначение:** Технические спецификации ЛЮБОГО размера (XS/S/M/L/XL)

**Ключевая особенность:** Один шаблон, но разная детализация при заполнении

**Подход:**
- Шаблон содержит ВСЕ возможные секции
- комментарии указывают, что нужно для каждого размера спецификации
- Неиспользуемые секции просто удаляются или упрощаются

**Структура:**
```
- Metadata
- Overview (Problem, Solution, Success Metrics)
- Architecture (Context, Container, Component, Data Model, Sequence, Activity)
- Requirements
- Acceptance Criteria
- Implementation Steps
- Testing Strategy
- Notes
```

**Размеры спецификаций:**

#### XS спецификации (50-100 строк)
**Пример:** Fix typo in error message

**Что заполняем:**
- Metadata: Effort: XS
- Overview: краткое
- Architecture: пропустить диаграммы
- Requirements: 1-2 простых
- Acceptance Criteria: 3-5
- Implementation Steps: 2-3 шага
- Notes: минимально

**Шаблон показывает:** `<!-- Для XS спецификаций можно пропустить -->`

---

#### S спецификации (100-200 строк)
**Пример:** Add new link type to graph

**Что заполняем:**
- Metadata: Effort: S
- Overview: краткое
- Architecture: только Data Model
- Requirements: 2-3
- Acceptance Criteria: 5-10
- Implementation Steps: 3-5 шагов
- Notes: примеры кода

**Шаблон показывает:** `<!-- Для S спецификаций: только Data Model -->`

---

#### M спецификации (200-400 строк)
**Пример:** Implement JSON exporter

**Что заполняем:**
- Metadata: Effort: M
- Overview: детальное
- Architecture: Component + Data Model + Sequence
- Requirements: 3-5 с деталями
- Acceptance Criteria: 10-15
- Implementation Steps: 5-10 шагов
- Testing Strategy: Unit + Integration
- Notes: примеры, конфигурации

**Шаблон показывает:** `<!-- Для M спецификаций: Component + Data Model + Sequence -->`

---

#### L спецификации (400-700 строк)
**Пример:** Implement cycle detection with Tarjan's algorithm

**Что заполняем:**
- Metadata: Effort: L
- Overview: очень детальное
- Architecture: Context + Container + Component + Data Model + Sequence + Activity
- Requirements: 5-8 детальных с API
- Acceptance Criteria: 15-25
- Implementation Steps: разбивка по фазам (10-15 шагов)
- Testing Strategy: полная стратегия
- Notes: Design decisions, performance, примеры

**Шаблон показывает:** `<!-- Для L спецификаций: все диаграммы -->`

---

#### XL спецификации (700-1000 строк)
**Пример:** Implement configuration system with TimeGrid (как в aitrader)

**Что заполняем:**
- Metadata: Effort: XL
- Overview: максимально детальное с контекстом
- Architecture: ВСЕ диаграммы + несколько Sequence для разных сценариев
- Requirements: 8-11 максимально детальных (FR + NFR)
- Acceptance Criteria: 25-35
- Implementation Steps: 4-5 фаз, 20+ шагов
- Testing Strategy: все типы тестов
- Notes: развернутые примеры, формулы, конфигурации, миграция

**Шаблон показывает:** `<!-- Для XL спецификаций: максимальная детализация -->`

---

### 2. adr.md
**Назначение:** Architecture Decision Record

**Когда использовать:** При принятии важных архитектурных решений

**Содержит:**
- Контекст и проблема
- Рассматриваемые варианты (3+)
- Принятое решение с обоснованием
- C4: Context, Container, Component
- Sequence диаграмма
- Последствия (положительные/отрицательные)
- Альтернативы (почему отклонены)

**Пример:** Выбор алгоритма для поиска циклов

---

## Как работать с универсальным шаблоном

### Шаг 1: Определите размер спецификации

**XS** - опечатка, простой баг, косметические изменения
**S** - добавить поле/метод, простая функциональность
**M** - новая feature с интеграцией
**L** - новый модуль, сложный алгоритм
**XL** - архитектурные изменения, новая подсистема

### Шаг 2: Скопируйте шаблон

```bash
cp templates/spec-template.md specs/todo/0042-your-spec.md
```

### Шаг 3: Читайте комментарии

В шаблоне есть комментарии:

```markdown
<!--
ВАЖНО: Объем диаграмм зависит от размера спецификации:
- XS/S спецификации: только Data Model (UML Class)
- M спецификации: Component + Data Model + Sequence
- L/XL спецификации: все диаграммы
-->
```

```markdown
<!-- Для L/XL спецификаций: показывает систему в окружении -->
<!-- Для S/M спецификаций: можно пропустить эту секцию -->
```

### Шаг 4: Заполните по рекомендациям

- Для **XS** - удалите большинство диаграмм, минимум текста
- Для **S** - только Data Model, краткие Requirements
- Для **M** - Component + Data Model + Sequence, детальнее
- Для **L** - все диаграммы, детальные Requirements
- Для **XL** - все максимально детально

### Шаг 5: Удалите неиспользуемые секции

Если секция не нужна - просто удалите её!

### Шаг 6: Посмотрите примеры

В конце шаблона есть **5 примеров** спецификаций разного размера:
- XS: Fix typo (50-100 строк)
- S: Add link type (100-200 строк)
- M: JSON exporter (200-400 строк)
- L: Cycle detection (400-700 строк)
- XL: Config system (700-1000 строк)

---

## Структура директорий

```
specs/
├── todo/          # Спецификации в очереди
├── inprogress/    # Спецификации в работе
└── done/          # Завершенные спецификации
```

### Именование файлов спецификаций

```
PPPP-short-description.md
```

- `PPPP` = 4-значный приоритет (0001-9999)
- Меньше число = выше приоритет

**Примеры:**
```
0010-implement-cycle-detection.md      # Критическая
0100-add-metrics-calculation.md        # Высокий
0500-improve-error-messages.md         # Средний
```

### Подспецификации

```
PPPP-XX-subspec-name.md
```

**Пример:**
```
0050-graph-analysis.md               # Родительская
0050-01-cycle-detection.md           # Подспецификация 1
0050-02-metrics-calculation.md       # Подспецификация 2
```

---

## Workflow

### Создание спецификации

1. Выберите размер (XS/S/M/L/XL)
2. Скопируйте `spec-template.md` в `specs/todo/`
3. Назовите: `PPPP-description.md`
4. Следуйте комментариям в шаблоне
5. Удалите неиспользуемые секции

```bash
cp templates/spec-template.md specs/todo/0042-implement-feature-x.md
```

### Начало работы

```bash
mv specs/todo/0042-feature-x.md specs/inprogress/
```

Обновите: `Status: InProgress`

### Завершение

```bash
mv specs/inprogress/0042-feature-x.md specs/done/
```

Обновите: `Status: Done`

---

## Ключевые секции (для spec driven development)

### 1. Architecture - Data Model (ОБЯЗАТЕЛЬНО!)

UML Class диаграмма с **полями И методами**:

```plantuml
class Graph {
  +Nodes: []Node           # Поля с типами
  --
  +AddNode(node Node)      # Методы с параметрами!
  +GetNode(id string) Node # И возвращаемыми значениями!
  +Validate() error
}
```

**НЕ ТАК:** просто список полей
**ТАК:** поля + методы + типы

### 2. Requirements - детализация критична

**XS/S:** краткие описания
```
R1: Fix typo in error message
```

**M:** с некоторыми деталями
```
R1: JSONExporter type
- Input: Graph
- Output: []byte, error
- Method: Export(g Graph) ([]byte, error)
```

**L/XL:** полные API спецификации
```go
FR1: CycleDetector Type
Input: Graph
Output: [][]string (cycles)

API:
type CycleDetector struct {
    graph Graph
    visited map[string]bool
    stack []string
}

func NewCycleDetector(g Graph) *CycleDetector {
    // Initialize detector with graph
    return &CycleDetector{graph: g, visited: make(map[string]bool)}
}

func (cd *CycleDetector) FindCycles() [][]string {
    // Find all cycles using Tarjan's algorithm
    // Returns list of cycles, each cycle is list of node IDs
}

func (cd *CycleDetector) HasCycle() bool {
    // Quick check if graph has any cycles
}

Validation Rules:
- Graph must not be nil
- Graph must have at least 2 nodes to form cycle
- Node IDs must be valid

Performance:
- Time complexity: O(V + E)
- Space complexity: O(V)

Error Conditions:
- Returns error if graph is nil: "graph cannot be nil"
- Returns empty list if no cycles found
```

### 3. Acceptance Criteria - количество зависит от размера

**XS:** 3-5 критериев
```
- [ ] AC1: Typo fixed
- [ ] AC2: Tests pass
- [ ] AC3: No regressions
```

**S:** 5-10 критериев
```
- [ ] AC1: LinkType supports "implements"
- [ ] AC2: Validation accepts new type
- [ ] AC3: Tests cover new type
- [ ] AC4: Documentation updated
- [ ] AC5: Backward compatible
```

**M:** 10-15 критериев
```
- [ ] AC1: JSONExporter.Export() exists
- [ ] AC2: Exports all components
- [ ] AC3: Valid JSON output
- [ ] AC4: CLI --format json works
- [ ] AC5: Backward compatible (yaml)
- [ ] AC6: Error handling
- [ ] AC7: Edge cases covered
- [ ] AC8: Integration test
- [ ] AC9: Documentation
- [ ] AC10: golangci-lint passes
```

**L/XL:** 20-35 критериев
```
Component Implementation (5)
Functionality (10)
Validation (5)
Performance (3)
Testing (5)
Code Quality (5)
Integration (3)
```

### 4. Notes - критично для Claude Code

**XS/S:** минимальные примеры
```go
// Location: internal/model/model.go:42
```

**M:** примеры использования
```go
exporter := NewJSONExporter()
data, err := exporter.Export(graph)
```

**L/XL:** развернутые примеры, design decisions, конфигурации
```go
// Example 1: Basic usage
detector := NewCycleDetector(graph)
cycles := detector.FindCycles()

// Example 2: With error handling
if err := detector.Validate(); err != nil {
    return err
}

// Design Decision: Why Tarjan's algorithm
// - O(V+E) complexity (optimal)
// - Finds all SCC in single pass
// - Standard algorithm for this task

// Performance optimization:
// - Use adjacency list for O(1) lookup
// - Cache visited nodes
```

---

## Компоненты archlint (для примеров)

- **CLI** - cmd/archlint (Cobra: collect, trace, analyze)
- **GoAnalyzer** - internal/analyzer/go.go (AST parsing)
- **Graph** - internal/model/model.go (Graph, Node, Edge, DocHub)
- **Tracer** - pkg/tracer (execution tracing)
- **Reporter** - форматирование в YAML/PlantUML
- **Linter** - internal/linter (validation)

---

## Просмотр PlantUML

**Online:** http://www.plantuml.com/plantuml/

**VS Code:**
```bash
code --install-extension jebbs.plantuml
```

**CLI:**
```bash
brew install plantuml
plantuml specs/todo/0042-spec.md
```

---

## Примеры

**В шаблоне:** 5 примеров (XS/S/M/L/XL) в конце файла

**Реальные примеры:**
- `example-spec.md` - заполненный пример
- `../aitrader/specs/done/` - реальные спецификации разных размеров

---

## Best Practices

### 1. Правильно определите размер

Не переусложняйте! Если спецификация простая - используйте XS/S.

### 2. Следуйте комментариям

Шаблон содержит подсказки для каждого размера.

### 3. Детализация Requirements

Чем больше спецификация - тем детальнее Requirements с API.

### 4. UML Class с методами

Всегда указывайте методы, не только поля!

### 5. Много Acceptance Criteria для больших спецификаций

L/XL спецификации: 20-35 критериев - это нормально.

### 6. Примеры кода

Для M/L/XL обязательно добавляйте примеры в Notes.

### 7. Backward Compatibility

Всегда указывайте требования к обратной совместимости.

---

## FAQ

**Q: Как понять какой размер спецификации?**

A: Примерная оценка:
- XS: < 50 строк кода, < 1 час
- S: 50-200 строк, 1-4 часа
- M: 200-500 строк, 4-8 часов
- L: 500-1000 строк, 1-3 дня
- XL: > 1000 строк, 3-7 дней

**Q: Нужно ли заполнять все секции?**

A: Нет! Читайте комментарии в шаблоне и удаляйте ненужное.

**Q: Чем этот подход отличается от двух шаблонов?**

A: Как в aitrader - один шаблон, разная детализация. Проще поддерживать, одна структура для всех.

**Q: Где посмотреть примеры?**

A: В конце `spec-template.md` есть 5 примеров разных размеров. Плюс смотрите `../aitrader/specs/done/`.

---

## Рекомендации

1. **Изучите примеры в шаблоне** - там показаны все 5 размеров
2. **Читайте комментарии** - они подсказывают что нужно
3. **Не переусложняйте** - для простых спецификаций используйте XS/S
4. **Детализируйте для Claude Code** - L/XL спецификации с полными API
5. **Обновляйте по ходу** - спецификация может вырасти из S в M
