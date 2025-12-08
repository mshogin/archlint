# archlint

A tool for building architectural graphs from Go source code.

archlint allows you to automatically extract and visualize software system architecture using two types of graphs:
- **Structural graph** - static code analysis showing all components and relationships
- **Behavioral graph** - dynamic analysis through tracing showing actual execution flows

## Features

- ✅ Build structural graphs from Go code
- ✅ Generate behavioral graphs from test traces
- ✅ Export to DocHub YAML format
- ✅ Automatic PlantUML sequence diagram generation
- ✅ Wildcard support for component grouping

## Installation

### From Source

```bash
git clone https://github.com/mshogin/archlint
cd archlint
make install
```

This will install `archlint` to `$GOPATH/bin`.

### Building

```bash
make build
```

The binary will be created at `bin/archlint`.

## Usage

### 1. Building Structural Graph

Analyzes source code and builds a graph of all components (packages, types, functions, methods) and their dependencies.

```bash
archlint collect . -o architecture.yaml
```

**Example output:**
```
Analyzing code: . (language: go)
Found components: 95
  - package: 5
  - struct: 23
  - function: 30
  - method: 21
  - external: 15
Found links: 129
✓ Graph saved to architecture.yaml
```

**Graph structure:**
```yaml
components:
  cmd/archlint:
    title: main
    entity: package
  cmd/archlint.main:
    title: main
    entity: function
  internal/analyzer.GoAnalyzer:
    title: GoAnalyzer
    entity: struct

links:
  cmd/archlint:
    - to: cmd/archlint.main
      type: contains
  cmd/archlint.main:
    - to: internal/analyzer.NewGoAnalyzer
      type: calls

contexts:
  cmd:
    title: cmd
    location: Architecture/cmd
    components:
      - cmd/archlint
      - cmd/archlint.main
```

### 2. Building Behavioral Graph

Generates contexts from test traces, showing actual execution flows.

**Step 1:** Add tracing to your tests:

```go
import "github.com/mshogin/archlint/pkg/tracer"

func TestProcessOrder(t *testing.T) {
    trace := tracer.StartTrace("TestProcessOrder")
    defer func() {
        trace.Save("traces/test_process_order.json")
    }()

    // Traced function
    tracer.Enter("OrderService.ProcessOrder")
    result, err := service.ProcessOrder(order)
    tracer.Exit("OrderService.ProcessOrder", err)

    // assertions...
}
```

**Step 2:** Run tests:

```bash
go test -v ./...
```

**Step 3:** Generate contexts:

```bash
archlint trace ./traces -o contexts.yaml
```

**Result:**
- `contexts.yaml` - contexts for DocHub
- `*.puml` - PlantUML sequence diagrams for each test

### 3. Using Makefile

```bash
# Show help
make help

# Build project
make build

# Build graph for archlint itself
make collect

# Format code
make fmt

# Run tests
make test

# Clean generated files
make clean
```

## Project Structure

```
archlint/
├── cmd/
│   └── archlint/          # CLI application
│       ├── main.go        # Entry point
│       ├── collect.go     # collect command
│       └── trace.go       # trace command
├── internal/
│   ├── model/             # Graph model
│   │   └── model.go       # Graph, Node, Edge, DocHub
│   └── analyzer/          # Code analyzers
│       └── go.go          # GoAnalyzer (AST parsing)
├── pkg/
│   └── tracer/            # Tracing library
│       ├── trace.go       # Trace collection
│       └── context_generator.go  # Context generator
├── go.mod
├── Makefile
└── README.md
```

## Examples

### Analyzing Your Own Project

archlint uses itself as an example:

```bash
make collect
```

Result: `graph/architecture.yaml` with complete project graph.

### Integration with DocHub

Generated YAML files are compatible with [DocHub](https://dochub.info/):

```yaml
# dochub.yaml
contexts:
  $imports:
    - architecture.yaml
    - contexts.yaml
```

## Data Format

### Structural Graph

- **Nodes (components)**: system components
  - `package` - Go packages
  - `struct` - structures
  - `interface` - interfaces
  - `function` - functions
  - `method` - methods
  - `external` - external dependencies

- **Edges (links)**: relationships between components
  - `contains` - containment (package contains type)
  - `calls` - function/method call
  - `uses` - type usage in field
  - `embeds` - type embedding
  - `import` - package import

### Behavioral Graph

- **Trace**: test execution trace
  - `test_name` - test name
  - `calls` - sequence of calls
    - `event`: "enter" | "exit_success" | "exit_error"
    - `function` - function name
    - `depth` - nesting level

## Relationship with aiarch

archlint contains only graph building functionality from the [aiarch](https://github.com/mshogin/aiarch) project.

**What is NOT included in archlint:**
- Graph validation
- Quality metrics (fan-out, coupling, etc.)
- Architecture rule checking

For validation and metrics, use [aiarch](https://github.com/mshogin/aiarch).

## License

MIT

## Contacts

- GitHub: https://github.com/mshogin/archlint
- Related project: https://github.com/mshogin/aiarch
