# Spec 0004: Implement Go Code Analyzer

**Metadata:**
- Priority: 0004 (High)
- Status: Done
- Created: 2024-12-01
- Effort: L
- Parent Spec: 0003

---

## Overview

### Problem Statement
Необходимо реализовать анализатор Go кода, который парсит исходный код с помощью AST и строит граф зависимостей между компонентами (пакетами, типами, функциями, методами).

### Solution Summary
Создать GoAnalyzer в пакете internal/analyzer, который использует go/ast и go/parser для анализа Go файлов и построения графа модели.

### Success Metrics
- Анализатор корректно парсит Go код
- Все типы компонентов извлекаются (package, struct, interface, function, method)
- Все типы связей определяются (contains, calls, uses, embeds, import)
- Внешние зависимости идентифицируются

---

## Architecture

### Component Overview (C4 Component)

```plantuml
@startuml spec-0004-component
!theme toy
!include <C4/C4_Component>

title Component Diagram: Go Analyzer

Container_Boundary(analyzer, "internal/analyzer") {
  Component(goanalyzer, "GoAnalyzer", "Go", "Main analyzer struct")
  Component(parser, "parseFile", "Go", "AST parsing")
  Component(builder, "buildGraph", "Go", "Graph construction")
}

Container_Boundary(stdlib, "Go Standard Library") {
  Component(ast, "go/ast", "Go", "AST types")
  Component(goparser, "go/parser", "Go", "Parser")
  Component(token, "go/token", "Go", "Tokenizer")
}

Container_Boundary(model, "internal/model") {
  Component(graph, "Graph", "Go", "Data model")
}

Rel(goanalyzer, parser, "Uses")
Rel(goanalyzer, builder, "Uses")
Rel(parser, ast, "Uses")
Rel(parser, goparser, "Uses")
Rel(parser, token, "Uses")
Rel(builder, graph, "Creates")

@enduml
```

### Data Model

```plantuml
@startuml spec-0004-model
!theme toy

title Data Model: Go Analyzer

class GoAnalyzer {
  -packages: map[string]*PackageInfo
  -types: map[string]*TypeInfo
  -functions: map[string]*FunctionInfo
  -methods: map[string]*MethodInfo
  -nodes: []model.Node
  -edges: []model.Edge
  --
  +NewGoAnalyzer() *GoAnalyzer
  +Analyze(dir string) (*model.Graph, error)
  -parseFile(filename string) error
  -parseTypeDecl(decl *ast.GenDecl, ...)
  -parseFuncDecl(decl *ast.FuncDecl, ...)
  -buildGraph()
  -buildPackageNodes()
  -buildTypeNodes()
  -buildFunctionNodes()
  -buildMethodNodes()
  -buildImportEdges()
  -buildFunctionCallEdges()
  -buildMethodCallEdges()
  -buildTypeDependencyEdges()
}

class PackageInfo {
  +Name: string
  +Path: string
  +Dir: string
  +Imports: []string
}

class TypeInfo {
  +Name: string
  +Package: string
  +Kind: string
  +File: string
  +Line: int
  +Fields: []FieldInfo
  +Embeds: []string
  +Implements: []string
}

class FieldInfo {
  +Name: string
  +TypeName: string
  +TypePkg: string
}

class FunctionInfo {
  +Name: string
  +Package: string
  +File: string
  +Line: int
  +Calls: []CallInfo
}

class MethodInfo {
  +Name: string
  +Receiver: string
  +Package: string
  +File: string
  +Line: int
  +Calls: []CallInfo
}

class CallInfo {
  +Target: string
  +IsMethod: bool
  +Receiver: string
  +Line: int
}

GoAnalyzer "1" *-- "*" PackageInfo
GoAnalyzer "1" *-- "*" TypeInfo
GoAnalyzer "1" *-- "*" FunctionInfo
GoAnalyzer "1" *-- "*" MethodInfo
TypeInfo "1" *-- "*" FieldInfo
FunctionInfo "1" *-- "*" CallInfo
MethodInfo "1" *-- "*" CallInfo

@enduml
```

### Sequence Flow

```plantuml
@startuml spec-0004-sequence
!theme toy

title Sequence: Code Analysis

actor User
participant "GoAnalyzer" as GA
participant "filepath.Walk" as FW
participant "go/parser" as GP
participant "buildGraph" as BG
participant "Graph" as G

User -> GA: Analyze(dir)
activate GA

GA -> FW: Walk(dir)
activate FW

loop For each .go file
  FW -> GA: parseFile(path)
  activate GA

  GA -> GP: ParseFile(fset, filename)
  activate GP
  GP --> GA: *ast.File
  deactivate GP

  GA -> GA: Extract packages
  GA -> GA: Extract types
  GA -> GA: Extract functions
  GA -> GA: Extract methods
  GA -> GA: Extract calls

  deactivate GA
end

FW --> GA: nil
deactivate FW

GA -> BG: buildGraph()
activate BG

BG -> BG: buildPackageNodes()
BG -> BG: buildTypeNodes()
BG -> BG: buildFunctionNodes()
BG -> BG: buildMethodNodes()
BG -> BG: buildImportEdges()
BG -> BG: buildFunctionCallEdges()
BG -> BG: buildMethodCallEdges()
BG -> BG: buildTypeDependencyEdges()

BG --> GA:
deactivate BG

GA -> G: Create Graph
activate G
G --> GA: *Graph
deactivate G

GA --> User: *Graph, nil
deactivate GA

@enduml
```

---

## Requirements

### R1: GoAnalyzer Type
**Description:** Основной тип анализатора

```go
// Package: internal/analyzer
// File: go.go

type GoAnalyzer struct {
    packages  map[string]*PackageInfo
    types     map[string]*TypeInfo
    functions map[string]*FunctionInfo
    methods   map[string]*MethodInfo
    nodes     []model.Node
    edges     []model.Edge
}

func NewGoAnalyzer() *GoAnalyzer
func (a *GoAnalyzer) Analyze(dir string) (*model.Graph, error)
```

### R2: File Walking
**Description:** Обход директории с фильтрацией

- Пропускать vendor/, node_modules/, .git/, bin/
- Пропускать не-.go файлы
- Пропускать _test.go файлы

### R3: AST Parsing
**Description:** Парсинг Go файлов

- Извлечение имени пакета
- Извлечение импортов (кроме stdlib)
- Извлечение объявлений типов (struct, interface)
- Извлечение функций и методов
- Извлечение вызовов функций/методов

### R4: Graph Building
**Description:** Построение графа из собранной информации

- Package nodes с типом "package"
- Type nodes с типом "struct"/"interface"
- Function nodes с типом "function"
- Method nodes с типом "method"
- External dependency nodes с типом "external"
- Contains edges (package->type, type->method)
- Import edges (package->package)
- Calls edges (function->function)
- Uses edges (type->type)
- Embeds edges (type->type)

### R5: Call Resolution
**Description:** Разрешение целей вызовов

- Проверка существования цели в known functions/methods
- Фильтрация builtin функций (make, new, len, etc.)
- Пропуск нерезольвленных вызовов

### R6: Type Dependency Resolution
**Description:** Разрешение зависимостей типов

- Фильтрация примитивных типов
- Резолвинг локальных типов
- Резолвинг импортированных типов

---

## Acceptance Criteria

- [x] AC1: NewGoAnalyzer() создает экземпляр
- [x] AC2: Analyze() обходит директорию
- [x] AC3: Пропускаются vendor, .git, bin
- [x] AC4: Пропускаются _test.go файлы
- [x] AC5: Извлекаются пакеты
- [x] AC6: Извлекаются struct типы
- [x] AC7: Извлекаются interface типы
- [x] AC8: Извлекаются функции
- [x] AC9: Извлекаются методы
- [x] AC10: Определяются вызовы
- [x] AC11: Определяются импорты
- [x] AC12: Строятся contains edges
- [x] AC13: Строятся calls edges
- [x] AC14: Строятся uses edges
- [x] AC15: Строятся embeds edges
- [x] AC16: External dependencies создаются
- [x] AC17: Stdlib imports фильтруются
- [x] AC18: Builtin functions фильтруются
- [x] AC19: Примитивные типы фильтруются
- [x] AC20: tracer.Enter/Exit в каждой функции

---

## Implementation Steps

### Phase 1: Core Structure
**Step 1.1:** Create analyzer package directory
- Files: internal/analyzer/
- Action: Create directory
- Details: `mkdir -p internal/analyzer`

**Step 1.2:** Create go.go
- Files: internal/analyzer/go.go
- Action: Create

**Step 1.2:** Define info types
- Details: PackageInfo, TypeInfo, FieldInfo, FunctionInfo, MethodInfo, CallInfo

**Step 1.3:** Define GoAnalyzer type
- Details: Fields and constructor

### Phase 2: Parsing
**Step 2.1:** Implement Analyze()
- Details: filepath.Walk with filtering

**Step 2.2:** Implement parseFile()
- Details: go/parser.ParseFile, extract imports

**Step 2.3:** Implement parseTypeDecl()
- Details: Extract struct/interface info

**Step 2.4:** Implement parseFuncDecl()
- Details: Extract function/method info

**Step 2.5:** Implement collectCalls()
- Details: AST inspect for call expressions

### Phase 3: Graph Building
**Step 3.1:** Implement buildPackageNodes()
**Step 3.2:** Implement buildTypeNodes()
**Step 3.3:** Implement buildFunctionNodes()
**Step 3.4:** Implement buildMethodNodes()
**Step 3.5:** Implement buildImportEdges()
**Step 3.6:** Implement buildFunctionCallEdges()
**Step 3.7:** Implement buildMethodCallEdges()
**Step 3.8:** Implement buildTypeDependencyEdges()

### Phase 4: Resolution
**Step 4.1:** Implement resolveCallTarget()
**Step 4.2:** Implement resolveTypeDependency()
**Step 4.3:** Implement helper functions

---

## Testing Strategy

### Unit Tests
- [ ] Test parsing simple package
- [ ] Test parsing struct
- [ ] Test parsing interface
- [ ] Test parsing function
- [ ] Test parsing method
- [ ] Test call extraction
- Coverage target: 80%+

### Integration Tests
- [x] Test full analysis on sample code
- [x] Verify graph structure

---

## Notes

### Skipped Directories
```go
if name == "vendor" || name == "node_modules" || name == ".git" || name == "bin" {
    return filepath.SkipDir
}
```

### Stdlib Detection
```go
func (a *GoAnalyzer) isStdLib(importPath string) bool {
    // No dots and no slashes = stdlib
    if !strings.Contains(importPath, ".") && !strings.Contains(importPath, "/") {
        return true
    }
    // Known prefixes
    stdlibPrefixes := []string{"fmt", "io", "os", ...}
    // Check prefix match
}
```

### Builtin Functions
```go
builtins := map[string]bool{
    "make": true, "new": true, "len": true, "cap": true,
    "append": true, "copy": true, "delete": true, "close": true,
    "panic": true, "recover": true, "print": true, "println": true,
}
```
