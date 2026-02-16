// Package callgraph строит графы вызовов от точек входа в Go-коде.
package callgraph

import "time"

// CallNodeType тип узла графа вызовов.
type CallNodeType string

// Допустимые типы узлов.
const (
	NodeFunction        CallNodeType = "function"
	NodeMethod          CallNodeType = "method"
	NodeInterfaceMethod CallNodeType = "interface_method"
	NodeClosure         CallNodeType = "closure"
	NodeExternal        CallNodeType = "external"
)

// CallType тип вызова.
type CallType string

// Допустимые типы вызовов.
const (
	CallDirect    CallType = "direct"
	CallInterface CallType = "interface"
	CallGoroutine CallType = "goroutine"
	CallClosure   CallType = "closure"
	CallDeferred  CallType = "deferred"
)

// CallNode представляет узел графа вызовов (функцию или метод).
type CallNode struct {
	ID       string       `yaml:"id"`
	Package  string       `yaml:"package"`
	Function string       `yaml:"function"`
	Receiver string       `yaml:"receiver,omitempty"`
	Type     CallNodeType `yaml:"type"`
	File     string       `yaml:"file,omitempty"`
	Line     int          `yaml:"line,omitempty"`
	Depth    int          `yaml:"depth"`
}

// CallEdge представляет ребро графа вызовов (вызов).
type CallEdge struct {
	From     string   `yaml:"from"`
	To       string   `yaml:"to"`
	CallType CallType `yaml:"call_type"`
	Line     int      `yaml:"line,omitempty"`
	Async    bool     `yaml:"async,omitempty"`
	Cycle    bool     `yaml:"cycle,omitempty"`
}

// Stats статистика по графу.
type Stats struct {
	TotalNodes      int `yaml:"total_nodes"`
	TotalEdges      int `yaml:"total_edges"`
	MaxDepthReached int `yaml:"max_depth_reached"`
	InterfaceCalls  int `yaml:"interface_calls"`
	GoroutineCalls  int `yaml:"goroutine_calls"`
	CyclesDetected  int `yaml:"cycles_detected"`
	UnresolvedCalls int `yaml:"unresolved_calls"`
}

// CallGraph представляет граф вызовов от одной точки входа.
type CallGraph struct {
	EventID     string            `yaml:"event_id,omitempty"`
	EventName   string            `yaml:"event_name,omitempty"`
	EntryPoint  string            `yaml:"entry_point"`
	Nodes       []CallNode        `yaml:"nodes"`
	Edges       []CallEdge        `yaml:"edges"`
	MaxDepth    int               `yaml:"max_depth"`
	ActualDepth int               `yaml:"actual_depth"`
	Stats       Stats             `yaml:"stats"`
	Warnings    []string          `yaml:"warnings,omitempty"`
	BuildTime   time.Duration     `yaml:"-"`
	Metadata    map[string]string `yaml:"metadata,omitempty"`
}

// SetStats статистика по набору графов.
type SetStats struct {
	TotalEvents  int `yaml:"total_events"`
	MappedEvents int `yaml:"mapped_events"`
	BuiltGraphs  int `yaml:"built_graphs"`
	FailedGraphs int `yaml:"failed_graphs"`
	TotalNodes   int `yaml:"total_nodes"`
	TotalEdges   int `yaml:"total_edges"`
}

// EventCallGraphSet набор графов вызовов по бизнес-процессу.
type EventCallGraphSet struct {
	ProcessID   string               `yaml:"process_id"`
	ProcessName string               `yaml:"process_name"`
	Graphs      map[string]CallGraph `yaml:"graphs"`
	GeneratedAt time.Time            `yaml:"generated_at"`
	Stats       SetStats             `yaml:"stats"`
	Warnings    []string             `yaml:"warnings,omitempty"`
}
