# ADR-XXXX: [Architecture Decision Name]

**EN** | [RU](adr.md)

**Metadata:**
- Status: [Proposed/Accepted/Deprecated/Superseded]
- Date: YYYY-MM-DD
- Deciders: [List of decision makers]
- Related ADRs: [Links to related ADRs]

---

## Context and Problem

[Description of the context in which the decision is being made and the problem that needs to be solved]

### Current Situation
[How it works now]

### Requirements
[Functional and non-functional requirements for the solution]

### Constraints
- [Constraint 1]
- [Constraint 2]

---

## Considered Options

### Option 1: [Name]
**Description:** [Brief description of the approach]

**Pros:**
- [Advantage 1]
- [Advantage 2]

**Cons:**
- [Disadvantage 1]
- [Disadvantage 2]

### Option 2: [Name]
**Description:** [Brief description of the approach]

**Pros:**
- [Advantage 1]
- [Advantage 2]

**Cons:**
- [Disadvantage 1]
- [Disadvantage 2]

### Option 3: [Name]
**Description:** [Brief description of the approach]

**Pros:**
- [Advantage 1]
- [Advantage 2]

**Cons:**
- [Disadvantage 1]
- [Disadvantage 2]

---

## Decision

**Chosen option:** [Name of the selected option]

**Rationale:**
[Why this particular option was chosen]

---

## Solution Architecture

### C4 Context (Level 1: System Context)

```plantuml
@startuml adr-xxxx-context
!theme toy
!include <C4/C4_Context>

title ADR-XXXX: System Context

Person(user, "User", "System user")
System(system, "System", "System description")
System_Ext(external, "External System", "External system")

Rel(user, system, "Uses")
Rel(system, external, "Integrates with")

SHOW_LEGEND()
@enduml
```

### C4 Container (Level 2: Containers)

```plantuml
@startuml adr-xxxx-container
!theme toy
!include <C4/C4_Container>

title ADR-XXXX: Container Diagram

Person(user, "User", "User")

System_Boundary(system, "System") {
  Container(cli, "CLI", "Go/Cobra", "Command line interface")
  Container(analyzer, "Analyzer", "Go", "Code analysis")
  ContainerDb(db, "Storage", "Files/DB", "Data storage")
}

System_Ext(external, "External System", "External system")

Rel(user, cli, "Executes commands")
Rel(cli, analyzer, "Uses")
Rel(analyzer, db, "Reads/Writes")
Rel(analyzer, external, "Calls")

SHOW_LEGEND()
@enduml
```

### C4 Component (Level 3: Components)

```plantuml
@startuml adr-xxxx-component
!theme toy
!include <C4/C4_Component>

title ADR-XXXX: Component Diagram

Container_Boundary(analyzer, "Analyzer") {
  Component(parser, "Parser", "Go", "Source code parsing")
  Component(builder, "Graph Builder", "Go", "Graph construction")
  Component(validator, "Validator", "Go", "Rules validation")
}

ContainerDb(db, "Storage", "Files", "Storage")

Rel(parser, builder, "Passes AST")
Rel(builder, validator, "Passes graph")
Rel(validator, db, "Saves results")

@enduml
```

### Sequence Diagram (Interaction Sequence)

```plantuml
@startuml adr-xxxx-sequence
!theme toy

title ADR-XXXX: Sequence Diagram

actor User
participant "CLI" as CLI
participant "Analyzer" as A
participant "Parser" as P
participant "Builder" as B
database "Storage" as S

User -> CLI: Execute command
activate CLI

CLI -> A: Analyze(path)
activate A

A -> P: Parse(files)
activate P
P --> A: AST
deactivate P

A -> B: BuildGraph(AST)
activate B
B --> A: Graph
deactivate B

A -> S: Save(graph)
activate S
S --> A: OK
deactivate S

A --> CLI: Result
deactivate A

CLI --> User: Display result
deactivate CLI

@enduml
```

---

## Consequences

### Positive

- [Positive consequence 1]
- [Positive consequence 2]
- [Positive consequence 3]

### Negative

- [Negative consequence 1]
- [Negative consequence 2]

### Neutral

- [Neutral consequence 1]
- [Neutral consequence 2]

---

## Implementation Details

### Technologies and Libraries
- [Technology 1]: [purpose]
- [Technology 2]: [purpose]

### Changes to Codebase
- [Package/module 1]: [changes]
- [Package/module 2]: [changes]

### Migration Plan
1. [Step 1]
2. [Step 2]
3. [Step 3]

---

## Alternatives (Rejected)

### Why Option 1 Was Not Chosen
[Specific reasons for rejection]

### Why Option 2 Was Not Chosen
[Specific reasons for rejection]

---

## Related Decisions

- [ADR-YYYY]: [name and relationship]
- [ADR-ZZZZ]: [name and relationship]

---

## References

- [Link to documentation 1]
- [Link to issue/PR]
- [Link to external resource]

---

## Example for archlint

**ADR-0001: Choosing an Algorithm for Detecting Circular Dependencies**

Context: Need to detect circular dependencies in the Go package graph

Options:
1. DFS with stack tracking (simple, but only finds simple cycles)
2. Tarjan's algorithm (O(V+E), finds all SCCs)
3. Floyd-Warshall (O(V^3), too slow)

Decision: Chose Tarjan's algorithm
- Optimal complexity O(V+E)
- Finds all strongly connected components in a single pass
- Standard algorithm for this task

Consequences:
+ Efficient detection of all cycles
+ Single pass through the graph
- More complex implementation than DFS
