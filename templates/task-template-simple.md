# Task XXX: [Task Title]

**Metadata:**
- Priority: XXX (High/Medium/Low)
- Status: Todo
- Created: YYYY-MM-DD
- Effort: S/M

---

## Обзор

### Problem Statement
[Описание проблемы, которую решает задача]

### Solution Summary
[Краткое описание предлагаемого решения]

### Success Metrics
- [Метрика 1]
- [Метрика 2]

---

## Открытые вопросы
- [Вопрос для обсуждения и принятия решения 1]
- [Вопрос для обсуждения и принятия решения 2]

---

## Architecture

### Component Overview (C4 Component)

```plantuml
@startuml task-xxx-component
!theme toy
!include <C4/C4_Component>

Component(comp1, "Component 1", "Go", "Description")
Component(comp2, "Component 2", "Go", "Description")

Rel(comp1, comp2, "Calls")

@enduml
```

### Data Model

```plantuml
@startuml task-xxx-model
!theme toy

class Model {
  +Field1: type
  +Field2: type
  --
  +Method() error
}

@enduml
```

### Sequence Flow (UML Sequence Diagram)

```plantuml
@startuml task-xxx-sequence
!theme toy

title Sequence Diagram for [Task Name]

actor User
participant "Handler" as H
participant "Service" as S
participant "Repository" as R
database "Database" as DB

User -> H: Request
activate H

H -> S: ProcessRequest()
activate S

S -> R: GetData()
activate R

R -> DB: SELECT
activate DB
DB --> R: Result
deactivate DB

R --> S: Data
deactivate R

S -> S: BusinessLogic()

S --> H: Response
deactivate S

H --> User: HTTP 200
deactivate H

@enduml
```

### Process Flow (UML Activity Diagram)

```plantuml
@startuml task-xxx-activity
!theme toy

title Activity Diagram for [Task Name]

start

:Receive Input;

if (Valid?) then (yes)
  :Process Data;

  fork
    :Step A;
  fork again
    :Step B;
  end fork

  :Combine Results;
else (no)
  :Return Error;
  stop
endif

:Save to Database;

:Return Success;

stop

@enduml
```

---

## Requirements

### R1: [Requirement Name]
- Detail 1
- Detail 2

### R2: [Requirement Name]
- Detail 1
- Detail 2

---

## Acceptance Criteria

- [ ] AC1: [Criterion]
- [ ] AC2: [Criterion]
- [ ] AC3: [Criterion]
- [ ] AC4: All tests pass
- [ ] AC5: Code reviewed

---

## Implementation Steps

**Step 1:** [Step name]
- Action: [What to do]

**Step 2:** [Step name]
- Action: [What to do]

**Step 3:** Tests
- Action: Write unit tests

---

## Testing Strategy

### Unit Tests
- [ ] Test [Component A]
- [ ] Test [Component B]
- Coverage target: 80%+

### Integration Tests
- [ ] Test [Integration scenario 1]
- [ ] Test [Integration scenario 2]

---

## Notes
[Additional notes, links, code snippets]
