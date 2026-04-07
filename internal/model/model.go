// Package model содержит структуры данных для представления архитектурного графа.
package model

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
	Name     string
	Receiver string
	Package  string
	File     string
	Line     int
	Calls    []CallInfo
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
