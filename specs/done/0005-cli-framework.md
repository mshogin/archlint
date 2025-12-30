# Spec 0005: Implement CLI Framework

**Metadata:**
- Priority: 0005 (High)
- Status: Done
- Created: 2024-12-01
- Effort: S
- Parent Spec: 0001

---

## Overview

### Problem Statement
Необходимо реализовать интерфейс командной строки (CLI) для archlint с поддержкой подкоманд, флагов и справки.

### Solution Summary
Использовать библиотеку Cobra для создания CLI с корневой командой и подкомандами (collect, trace).

### Success Metrics
- CLI запускается без ошибок
- `archlint --help` показывает справку
- `archlint --version` показывает версию
- Подкоманды регистрируются корректно

---

## Architecture

### Component Overview (C4 Component)

```plantuml
@startuml spec-0005-component
!theme toy
!include <C4/C4_Component>

title Component Diagram: CLI Framework

Container_Boundary(cli, "internal/cli") {
  Component(root, "root.go", "Go", "Root command and Execute()")
  Component(collect, "collect.go", "Go", "Collect subcommand")
  Component(trace, "trace.go", "Go", "Trace subcommand")
}

Container_Boundary(cmd, "cmd/archlint") {
  Component(main, "main.go", "Go", "Entry point")
}

Container_Boundary(cobra, "github.com/spf13/cobra") {
  Component(cobraCmd, "cobra.Command", "Go", "Command struct")
}

Rel(main, root, "Calls Execute()")
Rel(root, cobraCmd, "Uses")
Rel(collect, root, "Registers to")
Rel(trace, root, "Registers to")

@enduml
```

### Data Model

```plantuml
@startuml spec-0005-model
!theme toy

title Data Model: CLI

class rootCmd {
  +Use: "archlint"
  +Short: string
  +Long: string
  +Version: string
}

class collectCmd {
  +Use: "collect [dir]"
  +Short: string
  +Long: string
  +Args: ExactArgs(1)
  +RunE: runCollect
  --
  +outputFile: string
  +language: string
}

class traceCmd {
  +Use: "trace [dir]"
  +Short: string
  +Long: string
  +Args: ExactArgs(1)
  +RunE: runTrace
  --
  +outputFile: string
}

package cli {
  class Execute {
    +Execute() error
  }
}

rootCmd "1" *-- "1" collectCmd
rootCmd "1" *-- "1" traceCmd

@enduml
```

---

## Requirements

### R1: Root Command
**Description:** Корневая команда archlint

```go
// Package: internal/cli
// File: root.go

var version = "0.1.0"

var rootCmd = &cobra.Command{
    Use:     "archlint",
    Short:   "Инструмент для построения архитектурных графов",
    Long:    `archlint - инструмент для построения структурных графов и графов поведения
из исходного кода на языке Go.`,
    Version: version,
}

func Execute() error {
    if err := rootCmd.Execute(); err != nil {
        return fmt.Errorf("ошибка выполнения команды: %w", err)
    }
    return nil
}
```

### R2: Entry Point
**Description:** Точка входа приложения

```go
// Package: main
// File: cmd/archlint/main.go

func main() {
    tracer.Enter("main")

    if err := cli.Execute(); err != nil {
        tracer.ExitError("main", err)
        os.Exit(1)
    }

    tracer.ExitSuccess("main")
}
```

### R3: Subcommand Registration
**Description:** Подкоманды регистрируются в init()

```go
func init() {
    rootCmd.AddCommand(collectCmd)
    rootCmd.AddCommand(traceCmd)
}
```

---

## Acceptance Criteria

- [x] AC1: Package internal/cli создан
- [x] AC2: rootCmd определен
- [x] AC3: Execute() функция экспортирована
- [x] AC4: cmd/archlint/main.go вызывает cli.Execute()
- [x] AC5: Version установлен в "0.1.0"
- [x] AC6: archlint --help работает
- [x] AC7: archlint --version работает
- [x] AC8: tracer.Enter/Exit в Execute()
- [x] AC9: tracer.Enter/Exit в main()

---

## Implementation Steps

**Step 1:** Create cli package directory
- Files: internal/cli/
- Action: Create directory
- Details: `mkdir -p internal/cli`

**Step 2:** Add Cobra dependency
- Action: Run
- Details: `go get github.com/spf13/cobra`

**Step 3:** Create root.go
- Files: internal/cli/root.go
- Action: Create
- Details: Define rootCmd and Execute()

**Step 4:** Update main.go
- Files: cmd/archlint/main.go
- Action: Modify
- Details: Call cli.Execute() instead of fmt.Println

---

## Testing Strategy

### Unit Tests
- [x] Execute() не возвращает ошибку при --help
- Coverage target: N/A

---

## Notes

### Cobra Command Structure
```
archlint
├── collect [dir] -o output.yaml -l go
└── trace [dir] -o contexts.yaml
```

### Error Handling
- Все ошибки оборачиваются в fmt.Errorf с контекстом
- main() вызывает os.Exit(1) при ошибке
