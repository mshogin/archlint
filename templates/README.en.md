# Spec Templates for archlint

**EN** | [RU](README.md)

Templates for creating specifications using Markdown + PlantUML (C4 + UML).

These templates are designed for **spec driven development** with Claude Code - detailed specifications allow AI to effectively implement functionality.

---

## Available Templates

### 1. spec-template.md (Universal)
**Purpose:** Technical specifications of ANY size (XS/S/M/L/XL)

**Key Feature:** One template, but different levels of detail when filling it out

**Approach:**
- Template contains ALL possible sections
- Comments indicate what is needed for each specification size
- Unused sections are simply removed or simplified

**Structure:**
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

**Specification Sizes:**

#### XS specifications (50-100 lines)
**Example:** Fix typo in error message

**What to fill in:**
- Metadata: Effort: XS
- Overview: brief
- Architecture: skip diagrams
- Requirements: 1-2 simple ones
- Acceptance Criteria: 3-5
- Implementation Steps: 2-3 steps
- Notes: minimal

**Template shows:** `<!-- For XS specifications, can skip -->`

---

#### S specifications (100-200 lines)
**Example:** Add new link type to graph

**What to fill in:**
- Metadata: Effort: S
- Overview: brief
- Architecture: Data Model only
- Requirements: 2-3
- Acceptance Criteria: 5-10
- Implementation Steps: 3-5 steps
- Notes: code examples

**Template shows:** `<!-- For S specifications: Data Model only -->`

---

#### M specifications (200-400 lines)
**Example:** Implement JSON exporter

**What to fill in:**
- Metadata: Effort: M
- Overview: detailed
- Architecture: Component + Data Model + Sequence
- Requirements: 3-5 with details
- Acceptance Criteria: 10-15
- Implementation Steps: 5-10 steps
- Testing Strategy: Unit + Integration
- Notes: examples, configurations

**Template shows:** `<!-- For M specifications: Component + Data Model + Sequence -->`

---

#### L specifications (400-700 lines)
**Example:** Implement cycle detection with Tarjan's algorithm

**What to fill in:**
- Metadata: Effort: L
- Overview: very detailed
- Architecture: Context + Container + Component + Data Model + Sequence + Activity
- Requirements: 5-8 detailed with API
- Acceptance Criteria: 15-25
- Implementation Steps: breakdown by phases (10-15 steps)
- Testing Strategy: comprehensive strategy
- Notes: Design decisions, performance, examples

**Template shows:** `<!-- For L specifications: all diagrams -->`

---

#### XL specifications (700-1000 lines)
**Example:** Implement configuration system with TimeGrid (as in aitrader)

**What to fill in:**
- Metadata: Effort: XL
- Overview: maximally detailed with context
- Architecture: ALL diagrams + multiple Sequences for different scenarios
- Requirements: 8-11 maximally detailed (FR + NFR)
- Acceptance Criteria: 25-35
- Implementation Steps: 4-5 phases, 20+ steps
- Testing Strategy: all types of tests
- Notes: expanded examples, formulas, configurations, migration

**Template shows:** `<!-- For XL specifications: maximum detail -->`

---

### 2. adr.md
**Purpose:** Architecture Decision Record

**When to use:** When making important architectural decisions

**Contains:**
- Context and problem
- Considered options (3+)
- Decision made with justification
- C4: Context, Container, Component
- Sequence diagram
- Consequences (positive/negative)
- Alternatives (why rejected)

**Example:** Choosing an algorithm for cycle detection

---

## How to Work with the Universal Template

### Step 1: Determine the specification size

**XS** - typo, simple bug, cosmetic changes
**S** - add field/method, simple functionality
**M** - new feature with integration
**L** - new module, complex algorithm
**XL** - architectural changes, new subsystem

### Step 2: Copy the template

```bash
cp templates/spec-template.md specs/todo/0042-your-spec.md
```

### Step 3: Read the comments

The template has comments:

```markdown
<!--
IMPORTANT: The scope of diagrams depends on the specification size:
- XS/S specifications: Data Model only (UML Class)
- M specifications: Component + Data Model + Sequence
- L/XL specifications: all diagrams
-->
```

```markdown
<!-- For L/XL specifications: shows the system in its environment -->
<!-- For S/M specifications: can skip this section -->
```

### Step 4: Fill in according to recommendations

- For **XS** - remove most diagrams, minimal text
- For **S** - Data Model only, brief Requirements
- For **M** - Component + Data Model + Sequence, more detailed
- For **L** - all diagrams, detailed Requirements
- For **XL** - everything in maximum detail

### Step 5: Remove unused sections

If a section is not needed - just delete it!

### Step 6: Look at examples

At the end of the template there are **5 examples** of specifications of different sizes:
- XS: Fix typo (50-100 lines)
- S: Add link type (100-200 lines)
- M: JSON exporter (200-400 lines)
- L: Cycle detection (400-700 lines)
- XL: Config system (700-1000 lines)

---

## Directory Structure

```
specs/
├── todo/          # Specifications in queue
├── inprogress/    # Specifications in progress
└── done/          # Completed specifications
```

### Specification File Naming

```
PPPP-short-description.md
```

- `PPPP` = 4-digit priority (0001-9999)
- Lower number = higher priority

**Examples:**
```
0010-implement-cycle-detection.md      # Critical
0100-add-metrics-calculation.md        # High
0500-improve-error-messages.md         # Medium
```

### Sub-specifications

```
PPPP-XX-subspec-name.md
```

**Example:**
```
0050-graph-analysis.md               # Parent
0050-01-cycle-detection.md           # Sub-specification 1
0050-02-metrics-calculation.md       # Sub-specification 2
```

---

## Workflow

### Creating a specification

1. Choose size (XS/S/M/L/XL)
2. Copy `spec-template.md` to `specs/todo/`
3. Name it: `PPPP-description.md`
4. Follow comments in template
5. Remove unused sections

```bash
cp templates/spec-template.md specs/todo/0042-implement-feature-x.md
```

### Starting work

```bash
mv specs/todo/0042-feature-x.md specs/inprogress/
```

Update: `Status: InProgress`

### Completion

```bash
mv specs/inprogress/0042-feature-x.md specs/done/
```

Update: `Status: Done`

---

## Key Sections (for spec driven development)

### 1. Architecture - Data Model (REQUIRED!)

UML Class diagram with **fields AND methods**:

```plantuml
class Graph {
  +Nodes: []Node           # Fields with types
  --
  +AddNode(node Node)      # Methods with parameters!
  +GetNode(id string) Node # And return values!
  +Validate() error
}
```

**NOT LIKE THIS:** just a list of fields
**LIKE THIS:** fields + methods + types

### 2. Requirements - detail is critical

**XS/S:** brief descriptions
```
R1: Fix typo in error message
```

**M:** with some details
```
R1: JSONExporter type
- Input: Graph
- Output: []byte, error
- Method: Export(g Graph) ([]byte, error)
```

**L/XL:** full API specifications
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

### 3. Acceptance Criteria - quantity depends on size

**XS:** 3-5 criteria
```
- [ ] AC1: Typo fixed
- [ ] AC2: Tests pass
- [ ] AC3: No regressions
```

**S:** 5-10 criteria
```
- [ ] AC1: LinkType supports "implements"
- [ ] AC2: Validation accepts new type
- [ ] AC3: Tests cover new type
- [ ] AC4: Documentation updated
- [ ] AC5: Backward compatible
```

**M:** 10-15 criteria
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

**L/XL:** 20-35 criteria
```
Component Implementation (5)
Functionality (10)
Validation (5)
Performance (3)
Testing (5)
Code Quality (5)
Integration (3)
```

### 4. Notes - critical for Claude Code

**XS/S:** minimal examples
```go
// Location: internal/model/model.go:42
```

**M:** usage examples
```go
exporter := NewJSONExporter()
data, err := exporter.Export(graph)
```

**L/XL:** expanded examples, design decisions, configurations
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

## archlint Components (for examples)

- **CLI** - cmd/archlint (Cobra: collect, trace, analyze)
- **GoAnalyzer** - internal/analyzer/go.go (AST parsing)
- **Graph** - internal/model/model.go (Graph, Node, Edge, DocHub)
- **Tracer** - pkg/tracer (execution tracing)
- **Reporter** - formatting to YAML/PlantUML
- **Linter** - internal/linter (validation)

---

## Viewing PlantUML

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

## Examples

**In template:** 5 examples (XS/S/M/L/XL) at the end of file

**Real examples:**
- `example-spec.md` - filled example
- `../aitrader/specs/done/` - real specifications of different sizes

---

## Best Practices

### 1. Correctly determine the size

Don't overcomplicate! If the specification is simple - use XS/S.

### 2. Follow the comments

The template contains hints for each size.

### 3. Requirements detail

The larger the specification - the more detailed Requirements with API.

### 4. UML Class with methods

Always specify methods, not just fields!

### 5. Many Acceptance Criteria for large specifications

L/XL specifications: 20-35 criteria - this is normal.

### 6. Code examples

For M/L/XL always add examples in Notes.

### 7. Backward Compatibility

Always specify backward compatibility requirements.

---

## FAQ

**Q: How to understand which specification size?**

A: Rough estimate:
- XS: < 50 lines of code, < 1 hour
- S: 50-200 lines, 1-4 hours
- M: 200-500 lines, 4-8 hours
- L: 500-1000 lines, 1-3 days
- XL: > 1000 lines, 3-7 days

**Q: Do I need to fill in all sections?**

A: No! Read the comments in the template and delete unnecessary parts.

**Q: How does this approach differ from two templates?**

A: As in aitrader - one template, different detail levels. Easier to maintain, one structure for all.

**Q: Where to see examples?**

A: At the end of `spec-template.md` there are 5 examples of different sizes. Also see `../aitrader/specs/done/`.

---

## Recommendations

1. **Study examples in template** - they show all 5 sizes
2. **Read comments** - they suggest what is needed
3. **Don't overcomplicate** - for simple specifications use XS/S
4. **Detail for Claude Code** - L/XL specifications with full API
5. **Update as you go** - specification can grow from S to M
