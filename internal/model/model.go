// Package model содержит структуры данных для представления архитектурного графа.
package model

// Entity kind constants.
const (
	EntityPackage  = "package"
	EntityStruct   = "struct"
	EntityInterface = "interface"
	EntityFunction = "function"
	EntityMethod   = "method"
	EntityField    = "field"
	EntityExternal = "external"
)

// Edge type constants.
const (
	EdgeContains   = "contains"
	EdgeImport     = "import"
	EdgeCalls      = "calls"
	EdgeUses       = "uses"
	EdgeEmbeds     = "embeds"
	EdgeFieldRead  = "field_read"
	EdgeFieldWrite = "field_write"
)

// Graph представляет архитектурный граф.
type Graph struct {
	Nodes []Node `yaml:"components"`
	Edges []Edge `yaml:"links"`
}

// Node представляет узел графа (компонент).
type Node struct {
	ID     string `yaml:"id"`
	Title  string `yaml:"title"`
	Entity string `yaml:"entity"`
}

// Edge представляет ребро графа (связь между компонентами).
type Edge struct {
	From   string `yaml:"from"`
	To     string `yaml:"to"`
	Method string `yaml:"method,omitempty"`
	Type   string `yaml:"type,omitempty"`
}

// TypeInfo содержит информацию о типе (struct/interface).
type TypeInfo struct {
	Name       string
	Package    string
	Kind       string
	File       string
	Line       int
	Fields     []FieldInfo
	Embeds     []string
	Implements []string
}

// FieldInfo содержит информацию о поле структуры.
type FieldInfo struct {
	Name     string
	TypeName string
	TypePkg  string
}

// FunctionInfo содержит информацию о функции.
type FunctionInfo struct {
	Name    string
	Package string
	File    string
	Line    int
	Calls   []CallInfo
}

// MethodInfo содержит информацию о методе.
type MethodInfo struct {
	Name        string
	Receiver    string
	Package     string
	File        string
	Line        int
	Calls       []CallInfo
	FieldAccess []FieldAccessInfo
}

// FieldAccessInfo contains information about a field access within a method.
type FieldAccessInfo struct {
	// FieldName is the bare field name (e.g. "Name").
	FieldName string
	// IsWrite is true when the field is on the LHS of an assignment,
	// increment/decrement, or its address is taken.
	IsWrite bool
	// Line is the source line of the access.
	Line int
}

// CallInfo содержит информацию о вызове.
type CallInfo struct {
	Target      string
	IsMethod    bool
	Receiver    string
	Line        int
	IsGoroutine bool
	IsDeferred  bool
}
