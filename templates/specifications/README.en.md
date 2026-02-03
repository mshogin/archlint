# Specifications

Templates for creating specifications using Markdown + PlantUML (C4 + UML).

**Designed for spec-driven development with Claude Code** - detailed specifications allow AI to effectively implement functionality.

---

## Quick Start

1. Determine specification size (XS/S/M/L/XL)
2. Copy [spec-template.md](spec-template.md):
   ```bash
   cp templates/specifications/spec-template.md specs/todo/0042-your-feature.md
   ```
3. Follow comments in the template
4. Check [example-spec.md](example-spec.md)

---

## Files

### [spec-template.md](spec-template.md)
**Universal template for specifications of ANY size**

**Key feature:** One template, different detail levels when filling

**Approach:**
- Template contains ALL possible sections
- Comments indicate what's needed for each size
- Unused sections simply deleted

**Structure:**
```
- Metadata (priority, size, status)
- Overview (Problem, Solution, Success Metrics)
- Architecture (C4: Context, Container, Component, Data Model, Sequence, Activity)
- Requirements (FR + NFR)
- Acceptance Criteria
- Implementation Steps
- Testing Strategy
- Notes
```

---

### [example-spec.md](example-spec.md)
**Example of filled specification**

Shows how to fill template on real example.

---

## Specification Sizes

| Size | Lines | Effort | When to use |
|--------|-------|--------|-------------------|
| **XS** | 50-100 | < 1 hour | Typo, simple bug, cosmetic changes |
| **S** | 100-200 | 1-4 hours | Add field/method, simple functionality |
| **M** | 200-400 | 4-8 hours | New feature with integration |
| **L** | 400-700 | 1-3 days | New module, complex algorithm |
| **XL** | 700-1000 | 3-7 days | Architectural changes, new subsystem |

---

## What to Fill for Each Size

### XS specifications (50-100 lines)
**Example:** Fix typo in error message

**Fill:**
- Metadata: Effort: XS
- Overview: brief
- Architecture: skip diagrams
- Requirements: 1-2 simple
- Acceptance Criteria: 3-5
- Implementation Steps: 2-3 steps
- Notes: minimal

---

### S specifications (100-200 lines)
**Example:** Add new link type to graph

**Fill:**
- Metadata: Effort: S
- Overview: brief
- Architecture: Data Model only
- Requirements: 2-3
- Acceptance Criteria: 5-10
- Implementation Steps: 3-5 steps
- Notes: code examples

---

### M specifications (200-400 lines)
**Example:** Implement JSON exporter

**Fill:**
- Metadata: Effort: M
- Overview: detailed
- Architecture: Component + Data Model + Sequence
- Requirements: 3-5 with details
- Acceptance Criteria: 10-15
- Implementation Steps: 5-10 steps
- Testing Strategy: Unit + Integration
- Notes: examples, configs

---

### L specifications (400-700 lines)
**Example:** Implement cycle detection with Tarjan's algorithm

**Fill:**
- Metadata: Effort: L
- Overview: very detailed
- Architecture: Context + Container + Component + Data Model + Sequence + Activity
- Requirements: 5-8 detailed with API
- Acceptance Criteria: 15-25
- Implementation Steps: breakdown by phases (10-15 steps)
- Testing Strategy: full strategy
- Notes: Design decisions, performance, examples

---

### XL specifications (700-1000 lines)
**Example:** Implement configuration system with TimeGrid

**Fill:**
- Metadata: Effort: XL
- Overview: maximally detailed with context
- Architecture: ALL diagrams + multiple Sequences for different scenarios
- Requirements: 8-11 maximally detailed (FR + NFR)
- Acceptance Criteria: 25-35
- Implementation Steps: 4-5 phases, 20+ steps
- Testing Strategy: all test types
- Notes: expanded examples, formulas, configs, migration

---

## Working with Universal Template

### Step 1: Determine Size

**XS** - typo, simple bug, cosmetic changes
**S** - add field/method, simple functionality
**M** - new feature with integration
**L** - new module, complex algorithm
**XL** - architectural changes, new subsystem

### Step 2: Copy Template

```bash
cp templates/specifications/spec-template.md specs/todo/0042-your-spec.md
```

### Step 3: Read Comments

Template has comments:

```markdown
<!--
IMPORTANT: Diagram scope depends on specification size:
- XS/S specifications: Data Model only (UML Class)
- M specifications: Component + Data Model + Sequence
- L/XL specifications: all diagrams
-->
```

### Step 4: Fill per Recommendations

- For **XS** - remove most diagrams, minimal text
- For **S** - Data Model only, brief Requirements
- For **M** - Component + Data Model + Sequence, more detailed
- For **L** - all diagrams, detailed Requirements
- For **XL** - everything in maximum detail

### Step 5: Remove Unused Sections

If section not needed - just delete it!

### Step 6: Check Examples

Template end has **5 examples** of different sizes:
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

### Naming

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

---

## Workflow

### Creating Specification

1. Choose size (XS/S/M/L/XL)
2. Copy `spec-template.md` to `specs/todo/`
3. Name: `PPPP-description.md`
4. Follow template comments
5. Remove unused sections

```bash
cp templates/specifications/spec-template.md specs/todo/0042-implement-feature-x.md
```

### Starting Work

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

**NOT THIS:** just fields list
**THIS:** fields + methods + types

### 2. Requirements - Detail Critical

**XS/S:** brief descriptions
**M:** with some details
**L/XL:** full API specifications

### 3. Acceptance Criteria - Quantity Depends on Size

**XS:** 3-5 criteria
**S:** 5-10 criteria
**M:** 10-15 criteria
**L/XL:** 20-35 criteria (grouped by categories)

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

**In template:** 5 examples (XS/S/M/L/XL) at end of file

**Real examples:**
- [example-spec.md](example-spec.md) - filled example
- `../../aitrader/specs/done/` - real specifications of different sizes

---

## Best Practices

### 1. Correctly Determine Size

Don't overcomplicate! If specification simple - use XS/S.

### 2. Follow Comments

Template contains hints for each size.

### 3. Requirements Detail

Larger specification = more detailed Requirements with API.

### 4. UML Class with Methods

Always specify methods, not just fields!

### 5. Many Acceptance Criteria for Large Specs

L/XL specifications: 20-35 criteria - this is normal.

### 6. Code Examples

For M/L/XL always add examples in Notes.

### 7. Backward Compatibility

Always specify backward compatibility requirements.

---

## FAQ

**Q: How to understand which size?**

A: Rough estimate:
- XS: < 50 lines code, < 1 hour
- S: 50-200 lines, 1-4 hours
- M: 200-500 lines, 4-8 hours
- L: 500-1000 lines, 1-3 days
- XL: > 1000 lines, 3-7 days

**Q: Need to fill all sections?**

A: No! Read comments in template and delete unnecessary.

**Q: How does this differ from multiple templates?**

A: One template, different detail. Easier to maintain, one structure for all sizes.

**Q: Where to see examples?**

A: At end of `spec-template.md` are 5 examples of different sizes. Also see `example-spec.md`.

---

## Related Resources

**Methodology:**
- [C4 Model](https://c4model.com/)
- [Spec-Driven Development](https://en.wikipedia.org/wiki/Specification_by_example)

**Tools:**
- [PlantUML](https://plantuml.com/)
- [Markdown](https://www.markdownguide.org/)

---

*Templates created for effective work with Claude Code in spec-driven development mode.*
