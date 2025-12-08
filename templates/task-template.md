# Task XXX: [Task Title]

**Metadata:**
- Priority: XXX (High/Medium/Low)
- Status: Todo/InProgress/Done
- Created: YYYY-MM-DD
- Owner: [Name]
- Parent Task: [Parent ID if subtask]
- Estimated Effort: [S/M/L/XL]

---

## Overview

### Problem Statement
[Описание проблемы, которую решает задача]

### Solution Summary
[Краткое описание предлагаемого решения]

### Success Metrics
- [Метрика 1]
- [Метрика 2]

---

## Architecture Context (C4 Level 1: System Context)

```plantuml
@startuml task-xxx-context
!theme toy
!include <C4/C4_Context>

title System Context Diagram for [Task Name]

Person(user, "User", "End user of the system")
System(system, "Target System", "System being modified")
System_Ext(external, "External System", "Dependencies")

Rel(user, system, "Uses")
Rel(system, external, "Calls", "API")

@enduml
```

---

## Architecture Design (C4 Level 2: Containers)

```plantuml
@startuml task-xxx-container
!theme toy
!include <C4/C4_Container>

title Container Diagram for [Task Name]

Container(app, "Application", "Go", "Main application")
ContainerDb(db, "Database", "PostgreSQL", "Stores data")
Container(cache, "Cache", "Redis", "Caching layer")

Rel(app, db, "Reads/Writes", "SQL")
Rel(app, cache, "Uses", "Redis Protocol")

@enduml
```

---

## Component Design (C4 Level 3: Component)

```plantuml
@startuml task-xxx-component
!theme toy
!include <C4/C4_Component>

title Component Diagram for [Task Name]

Component(handler, "Handler", "Go", "HTTP request handler")
Component(service, "Service", "Go", "Business logic")
Component(repo, "Repository", "Go", "Data access")

Rel(handler, service, "Calls")
Rel(service, repo, "Uses")

@enduml
```

---

## Data Model (UML Class Diagram)

```plantuml
@startuml task-xxx-classes
!theme toy

title Data Model for [Task Name]

class Entity1 {
  +ID: string
  +Name: string
  +CreatedAt: time.Time
  --
  +Validate() error
  +ToJSON() string
}

class Entity2 {
  +ID: string
  +Entity1ID: string
  +Value: float64
  --
  +Calculate() float64
}

enum Status {
  PENDING
  ACTIVE
  COMPLETED
}

Entity1 "1" -- "many" Entity2
Entity2 --> Status

@enduml
```

---

## Sequence Flow (UML Sequence Diagram)

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

---

## Process Flow (UML Activity Diagram)

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

### Functional Requirements

**FR1: [Requirement Name]**
- Description: [Detailed description]
- Input: [Expected input]
- Output: [Expected output]
- Dependencies: [Other components/tasks]

**FR2: [Requirement Name]**
- Description: [Detailed description]
- Input: [Expected input]
- Output: [Expected output]
- Dependencies: [Other components/tasks]

### Non-Functional Requirements

**NFR1: Performance**
- [Specific performance criteria]

**NFR2: Security**
- [Security requirements]

**NFR3: Scalability**
- [Scalability requirements]

---

## Acceptance Criteria

### AC1: [Criterion Name]
- [ ] Condition 1
- [ ] Condition 2
- [ ] Condition 3

### AC2: [Criterion Name]
- [ ] Condition 1
- [ ] Condition 2

### AC3: [Criterion Name]
- [ ] Condition 1
- [ ] Condition 2

---

## Implementation Plan

### Phase 1: Foundation
**Step 1.1: [Step Name]**
- Files: `path/to/file.go`
- Action: Create/Modify
- Details: [Implementation details]
- Tests: [Test requirements]

**Step 1.2: [Step Name]**
- Files: `path/to/file.go`
- Action: Create/Modify
- Details: [Implementation details]
- Tests: [Test requirements]

### Phase 2: Core Logic
**Step 2.1: [Step Name]**
- Files: `path/to/file.go`
- Action: Create/Modify
- Details: [Implementation details]
- Tests: [Test requirements]

### Phase 3: Integration & Testing
**Step 3.1: Integration Tests**
- Test scenarios
- Expected behavior

**Step 3.2: Documentation**
- Update README
- Add code comments
- Update API docs

---

## Dependencies

### Internal Dependencies
- Task XXX: [Description]
- Task YYY: [Description]

### External Dependencies
- Package/Library: [Name] ([Version])
- API: [Name] ([Endpoint])

---

## Risks & Mitigations

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| [Risk description] | High/Medium/Low | High/Medium/Low | [Mitigation strategy] |

---

## Testing Strategy

### Unit Tests
- [ ] Test [Component A]
- [ ] Test [Component B]
- Coverage target: 80%+

### Integration Tests
- [ ] Test [Integration scenario 1]
- [ ] Test [Integration scenario 2]

### Manual Testing
- [ ] Test [Scenario 1]
- [ ] Test [Scenario 2]

---

## Technical Notes

### Design Decisions
- [Decision 1]: Rationale
- [Decision 2]: Rationale

### Performance Considerations
- [Consideration 1]
- [Consideration 2]

### Security Considerations
- [Consideration 1]
- [Consideration 2]

### Code Examples

```go
// Example implementation
package example

type Service struct {
    repo Repository
}

func NewService(repo Repository) *Service {
    return &Service{repo: repo}
}

func (s *Service) Process(input string) error {
    // Implementation
    return nil
}
```

---

## References

- [Related Documentation](link)
- [API Documentation](link)
- [Architecture Decision Record](link)

---

## Progress Log

### YYYY-MM-DD
- [Update 1]
- [Update 2]

### YYYY-MM-DD
- [Update 3]
