package callgraph

import (
	"github.com/mshogin/archlint/internal/analyzer"
	"github.com/mshogin/archlint/pkg/tracer"
)

// CallWalker рекурсивно обходит вызовы от точки входа.
type CallWalker struct {
	analyzer *analyzer.GoAnalyzer
	resolver *CallResolver
	maxDepth int
	visited  map[string]bool
	nodes    []CallNode
	edges    []CallEdge
	stats    Stats
	warnings []string
}

// NewCallWalker создает новый обходчик.
func NewCallWalker(a *analyzer.GoAnalyzer, resolver *CallResolver, maxDepth int) *CallWalker {
	return &CallWalker{
		analyzer: a,
		resolver: resolver,
		maxDepth: maxDepth,
		visited:  make(map[string]bool),
	}
}

// Walk запускает рекурсивный обход от указанной функции.
func (w *CallWalker) Walk(entryPointID string) {
	tracer.Enter("CallWalker.Walk")
	w.walk(entryPointID, 0)
	tracer.ExitSuccess("CallWalker.Walk")
}

// Results возвращает результат обхода.
func (w *CallWalker) Results() ([]CallNode, []CallEdge, Stats, []string) {
	return w.nodes, w.edges, w.stats, w.warnings
}

// walk - рекурсивная функция обхода.
//
//nolint:funlen,gocyclo // Recursive walk handles multiple call types and edge cases.
func (w *CallWalker) walk(functionID string, depth int) {
	if depth > w.maxDepth {
		return
	}

	if w.visited[functionID] {
		return
	}

	w.visited[functionID] = true

	if depth > w.stats.MaxDepthReached {
		w.stats.MaxDepthReached = depth
	}

	funcInfo := w.analyzer.LookupFunction(functionID)
	methodInfo := w.analyzer.LookupMethod(functionID)

	if funcInfo == nil && methodInfo == nil {
		w.addExternalNode(functionID, depth)
		w.stats.UnresolvedCalls++

		return
	}

	var calls []analyzer.CallInfo

	var callerPkg string

	if funcInfo != nil {
		w.addFunctionNode(funcInfo, functionID, depth)
		calls = funcInfo.Calls
		callerPkg = funcInfo.Package
	} else {
		w.addMethodNode(methodInfo, functionID, depth)
		calls = methodInfo.Calls
		callerPkg = methodInfo.Package
	}

	for _, call := range calls {
		resolved := w.resolver.Resolve(call, callerPkg)
		if resolved == nil {
			continue
		}

		if w.visited[resolved.TargetID] && resolved.NodeType != NodeExternal {
			w.addEdge(functionID, resolved, call.Line, true)
			w.stats.CyclesDetected++

			continue
		}

		w.addEdge(functionID, resolved, call.Line, false)
		w.updateStats(resolved)

		if resolved.NodeType == NodeExternal {
			if !w.visited[resolved.TargetID] {
				w.addExternalNode(resolved.TargetID, depth+1)
				w.visited[resolved.TargetID] = true
			}
		} else {
			w.walk(resolved.TargetID, depth+1)
		}
	}
}

func (w *CallWalker) addFunctionNode(info *analyzer.FunctionInfo, id string, depth int) {
	w.nodes = append(w.nodes, CallNode{
		ID:       id,
		Package:  info.Package,
		Function: info.Name,
		Type:     NodeFunction,
		File:     info.File,
		Line:     info.Line,
		Depth:    depth,
	})
	w.stats.TotalNodes++
}

func (w *CallWalker) addMethodNode(info *analyzer.MethodInfo, id string, depth int) {
	nodeType := NodeMethod

	receiverTypeID := info.Package + "." + info.Receiver
	if typeInfo := w.analyzer.LookupType(receiverTypeID); typeInfo != nil && typeInfo.Kind == "interface" {
		nodeType = NodeInterfaceMethod
	}

	w.nodes = append(w.nodes, CallNode{
		ID:       id,
		Package:  info.Package,
		Function: info.Name,
		Receiver: info.Receiver,
		Type:     nodeType,
		File:     info.File,
		Line:     info.Line,
		Depth:    depth,
	})
	w.stats.TotalNodes++
}

func (w *CallWalker) addExternalNode(id string, depth int) {
	w.nodes = append(w.nodes, CallNode{
		ID:       id,
		Function: id,
		Type:     NodeExternal,
		Depth:    depth,
	})
	w.stats.TotalNodes++
}

func (w *CallWalker) addEdge(from string, resolved *ResolvedCall, line int, cycle bool) {
	w.edges = append(w.edges, CallEdge{
		From:     from,
		To:       resolved.TargetID,
		CallType: resolved.CallType,
		Line:     line,
		Async:    resolved.Async,
		Cycle:    cycle,
	})
	w.stats.TotalEdges++
}

func (w *CallWalker) updateStats(resolved *ResolvedCall) {
	if resolved.CallType == CallInterface {
		w.stats.InterfaceCalls++
	}

	if resolved.Async {
		w.stats.GoroutineCalls++
	}

	if resolved.NodeType == NodeExternal {
		w.stats.UnresolvedCalls++
	}
}
