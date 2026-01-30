# Spec 0001: Implement Dependency Graph Analyzer

[EN](example-spec.en.md) | **RU**

**Metadata:**
- Priority: 0001 (High)
- Status: Todo
- Created: 2025-12-07
- Owner: mshogin
- Parent Spec: -
- Estimated Effort: L

---

## Overview

### Problem Statement
Текущий анализатор кода строит базовый граф зависимостей, но не предоставляет инструменты для анализа этого графа. Необходимо добавить функциональность для поиска циклических зависимостей, анализа coupling и вычисления метрик сложности.

### Solution Summary
Создать пакет `pkg/analyzer/graph` с набором алгоритмов для анализа графа зависимостей:
- Поиск циклов (алгоритм Тарьяна)
- Расчет метрик coupling
- Топологическая сортировка
- Расчет метрик сложности (cyclomatic complexity на уровне графа)

### Success Metrics
- Обнаружение всех циклических зависимостей
- Время анализа < 1 сек для проектов до 1000 узлов
- Coverage > 85%

---

## Architecture Context (C4 Level 1: System Context)

```plantuml
@startuml spec-0001-context
!include https://raw.githubusercontent.com/plantuml-stdlib/C4-PlantUML/master/C4_Context.puml

title System Context: Dependency Graph Analyzer

Person(developer, "Developer", "Analyzes code architecture")
System(archlint, "Archlint", "Architecture linting tool")
System_Ext(codebase, "Go Codebase", "Source code to analyze")

Rel(developer, archlint, "Runs analysis", "CLI")
Rel(archlint, codebase, "Reads & parses", "AST")

@enduml
```

---

## Architecture Design (C4 Level 2: Container)

```plantuml
@startuml spec-0001-container
!include https://raw.githubusercontent.com/plantuml-stdlib/C4-PlantUML/master/C4_Container.puml

title Container Diagram: Archlint Components

Container(cli, "CLI", "Cobra", "Command-line interface")
Container(analyzer, "Code Analyzer", "Go", "AST parsing & graph building")
Container(graph, "Graph Analyzer", "Go", "Graph algorithms & metrics")
Container(reporter, "Reporter", "Go", "Results formatting")
ContainerDb(cache, "Cache", "Files", "AST cache")

Rel(cli, analyzer, "Analyzes code", "Go API")
Rel(analyzer, graph, "Builds graph", "Go API")
Rel(graph, reporter, "Sends results", "Go API")
Rel(analyzer, cache, "Caches AST", "File I/O")

@enduml
```

---

## Component Design (C4 Level 3: Component)

```plantuml
@startuml spec-0001-component
!include https://raw.githubusercontent.com/plantuml-stdlib/C4-PlantUML/master/C4_Component.puml

title Component Diagram: Graph Analyzer Package

Component(detector, "Cycle Detector", "Go", "Tarjan's algorithm")
Component(metrics, "Metrics Calculator", "Go", "Coupling & complexity")
Component(sorter, "Topological Sorter", "Go", "Dependency ordering")
Component(walker, "Graph Walker", "Go", "DFS/BFS traversal")

Rel(detector, walker, "Uses for traversal")
Rel(metrics, walker, "Uses for traversal")
Rel(sorter, walker, "Uses for traversal")

@enduml
```

---

## Data Model (UML Class Diagram)

```plantuml
@startuml spec-0001-classes
!theme toy

title Data Model: Graph Analysis

class Graph {
  +Nodes: []Node
  +Edges: []Edge
  --
  +AddNode(node Node)
  +AddEdge(edge Edge)
  +GetNode(id string) Node
  +GetOutEdges(nodeID string) []Edge
}

class Node {
  +ID: string
  +Title: string
  +Entity: string
  +Package: string
  +Metrics: NodeMetrics
}

class Edge {
  +From: string
  +To: string
  +Type: string
  +Weight: int
}

class NodeMetrics {
  +InDegree: int
  +OutDegree: int
  +CouplingScore: float64
}

class CycleDetector {
  +graph: Graph
  --
  +FindCycles() [][]string
  +HasCycle() bool
}

class MetricsCalculator {
  +graph: Graph
  --
  +CalculateAfferentCoupling(nodeID string) int
  +CalculateEfferentCoupling(nodeID string) int
  +CalculateInstability(nodeID string) float64
}

enum EdgeType {
  IMPORT
  CALLS
  CONTAINS
  USES
  EMBEDS
}

Graph "1" *-- "many" Node
Graph "1" *-- "many" Edge
Node "1" -- "1" NodeMetrics
Edge --> EdgeType
CycleDetector --> Graph
MetricsCalculator --> Graph

@enduml
```

---

## Sequence Flow (UML Sequence Diagram)

```plantuml
@startuml spec-0001-sequence
!theme toy

title Sequence: Analyze Dependencies

actor Developer
participant "CLI" as CLI
participant "GoAnalyzer" as Analyzer
participant "Graph" as Graph
participant "CycleDetector" as Detector
participant "Reporter" as Reporter

Developer -> CLI: archlint analyze ./pkg
activate CLI

CLI -> Analyzer: Analyze("./pkg")
activate Analyzer

Analyzer -> Graph: Build graph from AST
activate Graph
Graph --> Analyzer: Graph
deactivate Graph

Analyzer -> Detector: FindCycles(graph)
activate Detector

Detector -> Detector: Tarjan's Algorithm
Detector --> Analyzer: cycles [][]string
deactivate Detector

Analyzer -> Analyzer: CalculateMetrics(graph)

Analyzer --> CLI: AnalysisResult
deactivate Analyzer

CLI -> Reporter: Format(result)
activate Reporter
Reporter --> CLI: FormattedOutput
deactivate Reporter

CLI --> Developer: Display results
deactivate CLI

@enduml
```

---

## Process Flow (UML Activity Diagram)

```plantuml
@startuml spec-0001-activity
!theme toy

title Activity: Cycle Detection (Tarjan's Algorithm)

start

:Initialize stack, index, lowlink;

:For each node in graph;

if (Node visited?) then (no)
  :strongConnect(node);

  partition "strongConnect" {
    :Set node.index = currentIndex++;
    :Set node.lowlink = node.index;
    :Push node to stack;

    :For each successor of node;

    if (Successor visited?) then (no)
      :Recurse strongConnect(successor);
      :node.lowlink = min(node.lowlink, successor.lowlink);
    else (yes on stack)
      :node.lowlink = min(node.lowlink, successor.index);
    endif

    if (node.lowlink == node.index?) then (yes)
      :Pop nodes from stack until node;
      :Add as strongly connected component;

      if (Component size > 1?) then (yes)
        :Report cycle found;
      endif
    endif
  }
else (yes)
  :Skip;
endif

:Return all cycles found;

stop

@enduml
```

---

## Requirements

### Functional Requirements

**FR1: Cycle Detection**
- Description: Обнаружение всех циклических зависимостей в графе
- Input: Graph
- Output: Список циклов [][]string (каждый цикл - список ID узлов)
- Dependencies: Graph Walker

**FR2: Coupling Metrics**
- Description: Расчет метрик связанности (afferent/efferent coupling, instability)
- Input: Graph, NodeID
- Output: CouplingMetrics struct
- Dependencies: Graph Walker

**FR3: Topological Sort**
- Description: Упорядочивание узлов по зависимостям
- Input: Graph
- Output: []string (ordered node IDs) или error если есть циклы
- Dependencies: Cycle Detector

### Non-Functional Requirements

**NFR1: Performance**
- Анализ графа из 1000 узлов должен занимать < 1 секунды
- Память: O(V + E) где V - узлы, E - ребра

**NFR2: Correctness**
- Алгоритм Тарьяна должен находить ВСЕ циклы
- Метрики должны быть математически корректны

**NFR3: Extensibility**
- Легко добавлять новые алгоритмы анализа
- Interface-based дизайн для подмены реализаций

---

## Acceptance Criteria

### AC1: Cycle Detection Works
- [ ] Обнаруживает простые циклы (A->B->A)
- [ ] Обнаруживает сложные циклы (A->B->C->A)
- [ ] Обнаруживает self-loops
- [ ] Не дает false positives на ациклических графах
- [ ] Корректно работает на пустом графе

### AC2: Metrics Calculated Correctly
- [ ] Afferent coupling = количество входящих ребер
- [ ] Efferent coupling = количество исходящих ребер
- [ ] Instability = EC / (AC + EC), диапазон [0, 1]
- [ ] Метрики корректны для граничных случаев (изолированные узлы)

### AC3: Performance Requirements Met
- [ ] Граф 100 узлов: < 100ms
- [ ] Граф 1000 узлов: < 1s
- [ ] Граф 10000 узлов: < 10s
- [ ] Бенчмарки документированы

### AC4: Code Quality
- [ ] Test coverage > 85%
- [ ] Все публичные функции документированы
- [ ] golangci-lint passes
- [ ] tracelint passes

---

## Implementation Plan

### Phase 1: Foundation (pkg/analyzer/graph)
**Step 1.1: Create Graph Traversal Utilities**
- Files: `pkg/analyzer/graph/walker.go`
- Action: Create
- Details:
  - Implement DFS (depth-first search)
  - Implement BFS (breadth-first search)
  - Реализовать visitor pattern для обхода
- Tests: `walker_test.go` - тесты на разных графах

**Step 1.2: Create Graph Data Structures**
- Files: `pkg/analyzer/graph/graph.go`
- Action: Create
- Details:
  - Расширить существующий Graph новыми методами
  - GetSuccessors(nodeID) []Node
  - GetPredecessors(nodeID) []Node
  - IsAcyclic() bool
- Tests: `graph_test.go` - базовые операции

### Phase 2: Cycle Detection
**Step 2.1: Implement Tarjan's Algorithm**
- Files: `pkg/analyzer/graph/cycles.go`
- Action: Create
- Details:
  - type CycleDetector struct
  - FindStronglyConnectedComponents() [][]string
  - FindCycles() [][]string
  - HasCycle() bool
- Tests: `cycles_test.go` - различные типы циклов

**Step 2.2: Add Cycle Reporting**
- Files: `pkg/analyzer/graph/report.go`
- Action: Create
- Details:
  - FormatCycle(cycle []string) string
  - Визуализация циклов в удобном формате
- Tests: Проверка форматирования

### Phase 3: Metrics Calculation
**Step 3.1: Implement Coupling Metrics**
- Files: `pkg/analyzer/graph/metrics.go`
- Action: Create
- Details:
  - type MetricsCalculator struct
  - CalculateAfferentCoupling(nodeID string) int
  - CalculateEfferentCoupling(nodeID string) int
  - CalculateInstability(nodeID string) float64
  - CalculateAbstractness(nodeID string) float64
- Tests: `metrics_test.go` - известные метрики

**Step 3.2: Add Aggregate Metrics**
- Files: `pkg/analyzer/graph/metrics.go`
- Action: Modify
- Details:
  - CalculateAllMetrics() map[string]NodeMetrics
  - GetMostCoupled(n int) []Node
  - GetMostUnstable(n int) []Node
- Tests: Сортировка и агрегация

### Phase 4: Topological Sort
**Step 4.1: Implement Kahn's Algorithm**
- Files: `pkg/analyzer/graph/topo.go`
- Action: Create
- Details:
  - TopologicalSort() ([]string, error)
  - Возвращать error если есть циклы
  - Использовать очередь для BFS-подхода
- Tests: `topo_test.go` - DAG и циклические графы

### Phase 5: Integration & CLI
**Step 5.1: Add CLI Command**
- Files: `internal/cli/analyze.go`
- Action: Create
- Details:
  - Команда: archlint analyze [dir]
  - Флаги: --cycles, --metrics, --topo
  - Вызов graph analyzer
- Tests: Integration test

**Step 5.2: Update Reporter**
- Files: `internal/reporter/graph.go`
- Action: Create
- Details:
  - Форматирование результатов анализа
  - Вывод в консоль, JSON, YAML
- Tests: Форматирование различных результатов

### Phase 6: Documentation & Examples
**Step 6.1: Add Examples**
- Files: `examples/graph_analysis/main.go`
- Action: Create
- Details: Примеры использования API

**Step 6.2: Update README**
- Files: `README.md`
- Action: Modify
- Details: Документация новой функциональности

---

## Dependencies

### Internal Dependencies
- Существующий `internal/analyzer/go.go` - для построения графа
- `pkg/model` - для структур Graph, Node, Edge

### External Dependencies
- Нет новых внешних зависимостей
- Используем только stdlib

---

## Risks & Mitigations

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| Алгоритм Тарьяна сложен в реализации | High | Medium | Использовать проверенные референсные реализации, добавить подробные комментарии |
| Производительность на больших графах | Medium | Low | Ранний benchmarking, оптимизация структур данных (adjacency list) |
| Ложные срабатывания в cycle detection | High | Low | Тщательное тестирование на реальных проектах, добавить фильтрацию допустимых циклов |

---

## Testing Strategy

### Unit Tests
- [ ] Test Walker (DFS/BFS) на различных графах
- [ ] Test CycleDetector на графах с/без циклов
- [ ] Test MetricsCalculator - известные значения
- [ ] Test TopologicalSort - DAG и циклические
- Coverage target: 90%+

### Integration Tests
- [ ] Полный pipeline: code -> graph -> analysis -> report
- [ ] Тест на реальном проекте (archlint сам себя)
- [ ] Performance benchmarks

### Manual Testing
- [ ] Запустить на archlint codebase
- [ ] Запустить на известном проекте с циклами
- [ ] Проверить вывод CLI

---

## Files Modified/Created

```
+ pkg/analyzer/graph/walker.go        (new)
+ pkg/analyzer/graph/walker_test.go   (new)
+ pkg/analyzer/graph/cycles.go        (new)
+ pkg/analyzer/graph/cycles_test.go   (new)
+ pkg/analyzer/graph/metrics.go       (new)
+ pkg/analyzer/graph/metrics_test.go  (new)
+ pkg/analyzer/graph/topo.go          (new)
+ pkg/analyzer/graph/topo_test.go     (new)
+ pkg/analyzer/graph/report.go        (new)
~ pkg/analyzer/graph/graph.go         (modified - add helper methods)
+ internal/cli/analyze.go             (new)
+ internal/reporter/graph.go          (new)
+ examples/graph_analysis/main.go     (new)
~ README.md                           (modified - add documentation)
```

---

## Technical Notes

### Design Decisions
- **Tarjan's Algorithm для cycle detection**: O(V+E), находит все SCC за один проход
- **Adjacency List representation**: Эффективнее для sparse графов (типично для code dependencies)
- **Interface-based design**: Позволит легко добавлять новые алгоритмы

### Performance Considerations
- Использовать map[string][]Edge для adjacency list - O(1) lookup
- Кэшировать метрики если граф не меняется
- Для очень больших графов рассмотреть streaming analysis

### Security Considerations
- Защита от stack overflow на очень глубоких графах
- Лимит на количество узлов для предотвращения DoS

### Code Examples

```go
// Example: Finding cycles
package main

import (
    "fmt"
    "github.com/mshogin/archlint/pkg/analyzer/graph"
)

func main() {
    // Build graph from code
    g := graph.NewGraph()
    // ... populate graph ...

    // Detect cycles
    detector := graph.NewCycleDetector(g)
    cycles := detector.FindCycles()

    if len(cycles) > 0 {
        fmt.Printf("Found %d cycles:\n", len(cycles))
        for i, cycle := range cycles {
            fmt.Printf("Cycle %d: %v\n", i+1, cycle)
        }
    }

    // Calculate metrics
    calc := graph.NewMetricsCalculator(g)
    for _, node := range g.Nodes {
        metrics := calc.Calculate(node.ID)
        fmt.Printf("%s: AC=%d, EC=%d, I=%.2f\n",
            node.ID,
            metrics.AfferentCoupling,
            metrics.EfferentCoupling,
            metrics.Instability)
    }
}
```

---

## References

- [Tarjan's Algorithm](https://en.wikipedia.org/wiki/Tarjan%27s_strongly_connected_components_algorithm)
- [Software Package Metrics](https://en.wikipedia.org/wiki/Software_package_metrics)
- [Coupling and Cohesion](https://en.wikipedia.org/wiki/Coupling_(computer_programming))

---

## Progress Log

### 2025-12-07
- Создана спецификация
- Разработан дизайн архитектуры
- Определены фазы реализации
