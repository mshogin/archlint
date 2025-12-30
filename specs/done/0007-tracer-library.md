# Spec 0007: Implement Tracer Library

**Metadata:**
- Priority: 0007 (High)
- Status: Done
- Created: 2024-12-01
- Effort: M
- Parent Spec: 0001

---

## Overview

### Problem Statement
Необходимо реализовать библиотеку трассировки для записи execution flow в тестах. Это позволит строить behavioral graphs на основе реального выполнения кода.

### Solution Summary
Создать пакет pkg/tracer с функциями для записи входа/выхода из функций и сохранения трассировки в JSON формат.

### Success Metrics
- StartTrace() начинает новую трассировку
- Enter()/ExitSuccess()/ExitError() записывают события
- Save() сохраняет трассировку в JSON файл
- Thread-safe операции

---

## Architecture

### Component Overview (C4 Component)

```plantuml
@startuml spec-0007-component
!theme toy
!include <C4/C4_Component>

title Component Diagram: Tracer Library

Container_Boundary(tracer, "pkg/tracer") {
  Component(trace, "trace.go", "Go", "Core tracing functions")
  Component(generator, "context_generator.go", "Go", "Context and diagram generation")
}

Container_Boundary(user, "User Code") {
  Component(test, "test_*.go", "Go", "Tests with tracing")
  Component(func, "functions", "Go", "Traced functions")
}

Rel(test, trace, "StartTrace, StopTrace, Save")
Rel(func, trace, "Enter, ExitSuccess, ExitError")
Rel(generator, trace, "Reads Trace")

@enduml
```

### Data Model

```plantuml
@startuml spec-0007-model
!theme toy

title Data Model: Tracer

class Trace {
  +TestName: string
  +StartTime: time.Time
  +EndTime: time.Time
  +Calls: []Call
  -mu: sync.Mutex
  --
  +Save(filename string) error
}
note right of Trace
  Представляет трассировку выполнения теста.
  JSON tags для сериализации.
  Mutex для thread-safety.
end note

class Call {
  +Event: string
  +Function: string
  +Timestamp: time.Time
  +Error: string
  +Depth: int
}
note right of Call
  Представляет вызов функции.

  Event values:
  - "enter"
  - "exit_success"
  - "exit_error"
end note

package "Global State" {
  class currentTrace {
    -currentTrace: *Trace
    -traceMu: sync.Mutex
    -callDepth: int
  }
}

package "Functions" {
  class TracerFunctions {
    +StartTrace(testName string) *Trace
    +StopTrace() *Trace
    +Enter(fn string)
    +Exit(fn string, err error)
    +ExitSuccess(fn string)
    +ExitError(fn string, err error)
  }
}

Trace "1" *-- "*" Call
currentTrace --> Trace

@enduml
```

### Sequence Flow

```plantuml
@startuml spec-0007-sequence
!theme toy

title Sequence: Tracing a Test

actor Test
participant "tracer" as T
participant "Trace" as TR
participant "File System" as FS

== Start Tracing ==
Test -> T: StartTrace("TestProcessOrder")
activate T
T -> TR: Create new Trace
T -> T: Set currentTrace
T -> T: Reset callDepth = 0
T --> Test: *Trace
deactivate T

== Record Calls ==
Test -> T: Enter("OrderService.ProcessOrder")
activate T
T -> T: Lock traceMu
T -> TR: Append Call{event: "enter", depth: 0}
T -> T: callDepth++
T --> Test:
deactivate T

Test -> T: Enter("Repository.Save")
activate T
T -> TR: Append Call{event: "enter", depth: 1}
T -> T: callDepth++
T --> Test:
deactivate T

Test -> T: ExitSuccess("Repository.Save")
activate T
T -> T: callDepth--
T -> TR: Append Call{event: "exit_success", depth: 1}
T --> Test:
deactivate T

Test -> T: ExitSuccess("OrderService.ProcessOrder")
activate T
T -> T: callDepth--
T -> TR: Append Call{event: "exit_success", depth: 0}
T --> Test:
deactivate T

== Save Trace ==
Test -> TR: Save("traces/test.json")
activate TR
TR -> TR: Set EndTime
TR -> TR: json.MarshalIndent
TR -> FS: WriteFile
FS --> TR:
TR --> Test: nil
deactivate TR

== Stop Tracing ==
Test -> T: StopTrace()
activate T
T -> T: currentTrace = nil
T --> Test: *Trace
deactivate T

@enduml
```

---

## Requirements

### R1: Trace Type
**Description:** Тип для хранения трассировки

```go
// Package: pkg/tracer
// File: trace.go

type Trace struct {
    TestName  string    `json:"test_name"`
    StartTime time.Time `json:"start_time"`
    EndTime   time.Time `json:"end_time"`
    Calls     []Call    `json:"calls"`
    mu        sync.Mutex
}
```

### R2: Call Type
**Description:** Тип для записи вызова

```go
type Call struct {
    Event     string    `json:"event"`
    Function  string    `json:"function"`
    Timestamp time.Time `json:"timestamp"`
    Error     string    `json:"error,omitempty"`
    Depth     int       `json:"depth"`
}
```

### R3: Global State
**Description:** Глобальное состояние для текущей трассировки

```go
var (
    currentTrace *Trace
    traceMu      sync.Mutex
    callDepth    int
)
```

### R4: StartTrace Function
**Description:** Начало новой трассировки

```go
func StartTrace(testName string) *Trace {
    traceMu.Lock()
    defer traceMu.Unlock()

    currentTrace = &Trace{
        TestName:  testName,
        StartTime: time.Now(),
        Calls:     []Call{},
    }
    callDepth = 0

    return currentTrace
}
```

### R5: Enter Function
**Description:** Запись входа в функцию

```go
func Enter(fn string) {
    traceMu.Lock()
    defer traceMu.Unlock()

    if currentTrace == nil {
        return
    }

    currentTrace.mu.Lock()
    defer currentTrace.mu.Unlock()

    currentTrace.Calls = append(currentTrace.Calls, Call{
        Event:     "enter",
        Function:  fn,
        Timestamp: time.Now(),
        Depth:     callDepth,
    })

    callDepth++
}
```

### R6: ExitSuccess Function
**Description:** Запись успешного выхода

```go
func ExitSuccess(fn string) {
    traceMu.Lock()
    defer traceMu.Unlock()

    if currentTrace == nil {
        return
    }

    callDepth--
    if callDepth < 0 {
        callDepth = 0
    }

    currentTrace.mu.Lock()
    defer currentTrace.mu.Unlock()

    currentTrace.Calls = append(currentTrace.Calls, Call{
        Event:     "exit_success",
        Function:  fn,
        Timestamp: time.Now(),
        Depth:     callDepth,
    })
}
```

### R7: ExitError Function
**Description:** Запись выхода с ошибкой

```go
func ExitError(fn string, err error) {
    // Similar to ExitSuccess but with error message
    // Event: "exit_error"
    // Error: err.Error()
}
```

### R8: Exit Function (Deprecated)
**Description:** Обертка для обратной совместимости

```go
func Exit(fn string, err error) {
    if err != nil {
        ExitError(fn, err)
    } else {
        ExitSuccess(fn)
    }
}
```

### R9: Save Method
**Description:** Сохранение трассировки в файл

```go
func (t *Trace) Save(filename string) error {
    t.mu.Lock()
    defer t.mu.Unlock()

    t.EndTime = time.Now()

    data, err := json.MarshalIndent(t, "", "  ")
    if err != nil {
        return fmt.Errorf("ошибка сериализации трассировки: %w", err)
    }

    if err := os.WriteFile(filename, data, 0o600); err != nil {
        return fmt.Errorf("ошибка записи файла трассировки: %w", err)
    }

    return nil
}
```

### R10: StopTrace Function
**Description:** Остановка текущей трассировки

```go
func StopTrace() *Trace {
    traceMu.Lock()
    defer traceMu.Unlock()

    trace := currentTrace
    currentTrace = nil
    callDepth = 0

    return trace
}
```

---

## Acceptance Criteria

- [x] AC1: Package pkg/tracer создан
- [x] AC2: Trace тип с JSON тегами
- [x] AC3: Call тип с JSON тегами
- [x] AC4: StartTrace() инициализирует трассировку
- [x] AC5: Enter() записывает "enter" событие
- [x] AC6: ExitSuccess() записывает "exit_success"
- [x] AC7: ExitError() записывает "exit_error" с сообщением
- [x] AC8: Exit() роутит к ExitSuccess/ExitError
- [x] AC9: Save() сохраняет JSON с отступами
- [x] AC10: StopTrace() очищает состояние
- [x] AC11: Thread-safe с sync.Mutex
- [x] AC12: Depth корректно увеличивается/уменьшается
- [x] AC13: Enter/Exit безопасны без StartTrace

---

## Implementation Steps

**Step 1:** Create tracer package directory
- Files: pkg/tracer/
- Action: Create directory
- Details: `mkdir -p pkg/tracer`

**Step 2:** Create trace.go
- Files: pkg/tracer/trace.go
- Action: Create
- Details: Define types and global state

**Step 2:** Implement StartTrace()
- Details: Initialize currentTrace

**Step 3:** Implement Enter()
- Details: Append "enter" call, increment depth

**Step 4:** Implement ExitSuccess()
- Details: Decrement depth, append "exit_success"

**Step 5:** Implement ExitError()
- Details: Decrement depth, append "exit_error" with error

**Step 6:** Implement Exit()
- Details: Route to ExitSuccess or ExitError

**Step 7:** Implement Save()
- Details: Set EndTime, marshal JSON, write file

**Step 8:** Implement StopTrace()
- Details: Reset global state

---

## Testing Strategy

### Unit Tests
- [ ] Test StartTrace creates new trace
- [ ] Test Enter increments depth
- [ ] Test ExitSuccess decrements depth
- [ ] Test ExitError includes error message
- [ ] Test Save creates valid JSON file
- [ ] Test thread-safety with concurrent calls
- Coverage target: 80%+

---

## Notes

### JSON Output Example
```json
{
  "test_name": "TestProcessOrder",
  "start_time": "2024-12-01T10:00:00Z",
  "end_time": "2024-12-01T10:00:01Z",
  "calls": [
    {
      "event": "enter",
      "function": "OrderService.ProcessOrder",
      "timestamp": "2024-12-01T10:00:00.100Z",
      "depth": 0
    },
    {
      "event": "enter",
      "function": "Repository.Save",
      "timestamp": "2024-12-01T10:00:00.200Z",
      "depth": 1
    },
    {
      "event": "exit_success",
      "function": "Repository.Save",
      "timestamp": "2024-12-01T10:00:00.300Z",
      "depth": 1
    },
    {
      "event": "exit_success",
      "function": "OrderService.ProcessOrder",
      "timestamp": "2024-12-01T10:00:00.400Z",
      "depth": 0
    }
  ]
}
```

### Usage in Tests
```go
func TestProcessOrder(t *testing.T) {
    trace := tracer.StartTrace("TestProcessOrder")
    defer func() {
        trace = tracer.StopTrace()
        trace.Save("traces/test_process_order.json")
    }()

    // Call traced functions
    tracer.Enter("OrderService.ProcessOrder")
    result := service.ProcessOrder(order)
    tracer.ExitSuccess("OrderService.ProcessOrder")
}
```

### Usage in Production Code
```go
func (s *OrderService) ProcessOrder(order Order) error {
    tracer.Enter("OrderService.ProcessOrder")

    if err := s.repo.Save(order); err != nil {
        tracer.ExitError("OrderService.ProcessOrder", err)
        return err
    }

    tracer.ExitSuccess("OrderService.ProcessOrder")
    return nil
}
```
