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
