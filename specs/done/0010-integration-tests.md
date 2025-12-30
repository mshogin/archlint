# Spec 0010: Implement Integration Tests

**Metadata:**
- Priority: 0010 (High)
- Status: Done
- Created: 2024-12-01
- Effort: M
- Parent Spec: 0007

---

## Overview

### Problem Statement
Необходимо проверить корректность работы всего pipeline archlint: от анализа кода до генерации контекстов из трассировок.

### Solution Summary
Создать интеграционный тест TestFullCycle, который проверяет полный цикл работы с использованием sample кода.

### Success Metrics
- Тест проверяет collect (анализ кода)
- Тест проверяет trace (генерация контекстов)
- Тест использует реальный sample код с трассировкой
- Все компоненты интегрируются корректно

---

## Architecture

### Component Overview (C4 Component)

```plantuml
@startuml spec-0010-component
!theme toy
!include <C4/C4_Component>

title Component Diagram: Integration Tests

Container_Boundary(tests, "tests/") {
  Component(fullcycle, "fullcycle_test.go", "Go", "Full cycle integration test")
}

Container_Boundary(testdata, "tests/testdata/sample/") {
  Component(calculator, "calculator.go", "Go", "Sample traced code")
  Component(calctest, "calculator_traced_test.go", "Go", "Sample test with tracing")
}

Container_Boundary(internal, "internal/") {
  Component(analyzer, "analyzer", "Go", "Code analyzer")
}

Container_Boundary(pkg, "pkg/") {
  Component(tracer, "tracer", "Go", "Tracing library")
}

Rel(fullcycle, analyzer, "Uses GoAnalyzer")
Rel(fullcycle, tracer, "Uses Contexts generation")
Rel(fullcycle, testdata, "Analyzes and runs")
Rel(calculator, tracer, "Uses tracer calls")
Rel(calctest, tracer, "StartTrace/StopTrace")

@enduml
```

### Sequence Flow

```plantuml
@startuml spec-0010-sequence
!theme toy

title Sequence: Full Cycle Test

actor Test
participant "fullcycle_test" as FC
participant "GoAnalyzer" as GA
participant "exec.Command" as EC
participant "GenerateContexts" as GC

Test -> FC: TestFullCycle(t)
activate FC

== Step 1: Setup ==
FC -> FC: Create output directories
FC -> FC: Clean previous artifacts

== Step 2: Analyze Sample Code ==
FC -> GA: Analyze(testdata/sample)
activate GA
GA --> FC: *Graph
deactivate GA

FC -> FC: Save architecture.yaml
FC -> FC: Verify nodes and edges

== Step 3: Run Traced Test ==
FC -> EC: go test -v -run TestCalculateWithTrace
activate EC
EC -> EC: Run in testdata/sample dir
EC --> FC: Output, trace file created
deactivate EC

FC -> FC: Verify trace file exists

== Step 4: Load and Verify Trace ==
FC -> FC: Read trace JSON
FC -> FC: Verify expected functions traced

== Step 5: Generate Contexts ==
FC -> GC: GenerateContextsFromTraces(traceDir)
activate GC
GC --> FC: Contexts
deactivate GC

FC -> FC: Save contexts.yaml
FC -> FC: Verify components in context

== Step 6: Verify PlantUML ==
FC -> FC: Check UML file exists
FC -> FC: Print summary

FC --> Test: PASS
deactivate FC

@enduml
```

---

## Requirements

### R1: Test Structure
**Description:** Создать структуру тестовых директорий

```
tests/
├── fullcycle_test.go
├── output/           (generated, gitignored)
│   ├── architecture.yaml
│   ├── contexts.yaml
│   └── traces/
└── testdata/
    └── sample/
        ├── calculator.go
        └── calculator_traced_test.go
```

### R2: Sample Code
**Description:** Создать sample код с tracer вызовами

```go
// tests/testdata/sample/calculator.go
package sample

type Calculator struct {
    memory int
}

func NewCalculator() *Calculator {
    tracer.Enter("sample.NewCalculator")
    tracer.ExitSuccess("sample.NewCalculator")
    return &Calculator{memory: 0}
}

func (c *Calculator) Calculate(a, b int) int {
    tracer.Enter("sample.Calculator.Calculate")
    // ... calculations with nested calls
    tracer.ExitSuccess("sample.Calculator.Calculate")
    return result
}
```

### R3: Sample Test with Tracing
**Description:** Тест, который создает трассировку

```go
// tests/testdata/sample/calculator_traced_test.go
func TestCalculateWithTrace(t *testing.T) {
    trace := tracer.StartTrace("TestCalculateWithTrace")
    defer func() {
        trace = tracer.StopTrace()
        trace.Save("traces/test_calculate.json")
    }()

    calc := NewCalculator()
    result := calc.Calculate(5, 3)
    // assertions
}
```

### R4: Full Cycle Test
**Description:** Интеграционный тест

```go
// tests/fullcycle_test.go
func TestFullCycle(t *testing.T) {
    // Step 1: Setup output directories
    // Step 2: Analyze sample code with GoAnalyzer
    // Step 3: Run traced test via exec.Command
    // Step 4: Verify trace file and contents
    // Step 5: Generate contexts from traces
    // Step 6: Verify all traced functions in context
    // Step 7: Verify PlantUML diagram generated
}
```

### R5: Assertions
**Description:** Что проверять

- Graph содержит expected nodes и edges
- Trace file создан
- Trace содержит expected functions
- Context содержит all traced components
- PlantUML file существует

---

## Acceptance Criteria

- [x] AC1: tests/ директория создана
- [x] AC2: tests/testdata/sample/ содержит sample код
- [x] AC3: calculator.go с tracer вызовами
- [x] AC4: calculator_traced_test.go с трассировкой
- [x] AC5: fullcycle_test.go реализован
- [x] AC6: Тест создает output директории
- [x] AC7: Тест анализирует sample код
- [x] AC8: Тест запускает traced test
- [x] AC9: Тест проверяет trace file
- [x] AC10: Тест генерирует contexts
- [x] AC11: Тест проверяет PlantUML
- [x] AC12: `go test ./tests/...` проходит
- [x] AC13: Output в tests/output/ (gitignored)

---

## Implementation Steps

### Phase 1: Directory Setup
**Step 1.1:** Create tests directory structure
- Files: tests/, tests/testdata/sample/
- Action: Create directories
- Details: `mkdir -p tests/testdata/sample`

### Phase 2: Sample Code
**Step 2.1:** Create calculator.go
- Files: tests/testdata/sample/calculator.go
- Details: Calculator with tracer calls

**Step 2.2:** Create calculator_traced_test.go
- Files: tests/testdata/sample/calculator_traced_test.go
- Details: Test that creates trace

### Phase 3: Integration Test
**Step 3.1:** Create fullcycle_test.go
- Files: tests/fullcycle_test.go
- Details: Full cycle test

**Step 3.2:** Implement test steps
- Setup, analyze, run, verify

### Phase 4: Gitignore
**Step 4.1:** Update .gitignore
- Details: Add tests/output/

---

## Testing Strategy

### Unit Tests
- N/A (this IS the integration test)

### Integration Tests
- [x] TestFullCycle verifies entire pipeline
- [x] Sample code compiles
- [x] Sample test runs
- Coverage target: N/A

---

## Notes

### Sample Calculator Functions
```go
func Add(a, b int) int
func Multiply(a, b int) int
func (c *Calculator) AddToMemory(value int)
func (c *Calculator) GetMemory() int
func (c *Calculator) Calculate(a, b int) int
```

### Expected Trace Functions
```
sample.NewCalculator
sample.Calculator.Calculate
sample.Add
sample.Multiply
sample.Calculator.AddToMemory
sample.Calculator.GetMemory
```

### Helper Functions in Test
```go
func copyFile(src, dst string) error
func toHierarchicalComponentID(functionName string) string
func camelToSnake(s string) string
```

### Test Output
```
=== RUN   TestFullCycle
Step 1: Collecting architecture from sample code
Architecture saved to output/architecture.yaml
Found 6 components and 8 links
Step 2: Running traced test
Step 3: Checking trace file exists
Trace contains 12 calls
Step 4: Generating context from trace
Generated 1 contexts
Step 5: Verifying all traced components are in context
Function sample.NewCalculator found in context
Function sample.Calculator.Calculate found in context
PlantUML diagram: output/traces/test_calculate.puml
Full cycle test completed successfully!
--- PASS: TestFullCycle
```
