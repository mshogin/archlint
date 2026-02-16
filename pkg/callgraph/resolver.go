package callgraph

import (
	"strings"

	"github.com/mshogin/archlint/internal/analyzer"
	"github.com/mshogin/archlint/pkg/tracer"
)

// CallResolver разрешает цели вызовов: прямые, через интерфейс, горутины.
type CallResolver struct {
	analyzer *analyzer.GoAnalyzer
}

// ResolvedCall содержит разрешенную информацию о вызове.
type ResolvedCall struct {
	TargetID string
	NodeType CallNodeType
	CallType CallType
	Async    bool
	Package  string
	Function string
	Receiver string
	File     string
	Line     int
}

// NewCallResolver создает новый резолвер.
func NewCallResolver(a *analyzer.GoAnalyzer) *CallResolver {
	return &CallResolver{analyzer: a}
}

// Resolve разрешает вызов в конкретную цель.
//
//nolint:funlen // Call resolution requires checking multiple target types.
func (r *CallResolver) Resolve(call analyzer.CallInfo, callerPkg string) *ResolvedCall {
	tracer.Enter("CallResolver.Resolve")

	targetID := r.analyzer.ResolveCallTarget(call, callerPkg)

	callType := r.determineCallType(call)
	async := call.IsGoroutine

	if targetID == "" {
		if call.Target == "" || strings.HasPrefix(call.Target, "().") {
			tracer.ExitSuccess("CallResolver.Resolve")

			return nil
		}

		tracer.ExitSuccess("CallResolver.Resolve")

		return &ResolvedCall{
			TargetID: call.Target,
			NodeType: NodeExternal,
			CallType: callType,
			Async:    async,
			Function: call.Target,
			Line:     call.Line,
		}
	}

	if funcInfo := r.analyzer.LookupFunction(targetID); funcInfo != nil {
		tracer.ExitSuccess("CallResolver.Resolve")

		return &ResolvedCall{
			TargetID: targetID,
			NodeType: NodeFunction,
			CallType: callType,
			Async:    async,
			Package:  funcInfo.Package,
			Function: funcInfo.Name,
			File:     funcInfo.File,
			Line:     funcInfo.Line,
		}
	}

	if methodInfo := r.analyzer.LookupMethod(targetID); methodInfo != nil {
		nodeType := r.determineMethodNodeType(methodInfo)

		tracer.ExitSuccess("CallResolver.Resolve")

		return &ResolvedCall{
			TargetID: targetID,
			NodeType: nodeType,
			CallType: r.adjustCallType(callType, nodeType),
			Async:    async,
			Package:  methodInfo.Package,
			Function: methodInfo.Name,
			Receiver: methodInfo.Receiver,
			File:     methodInfo.File,
			Line:     methodInfo.Line,
		}
	}

	tracer.ExitSuccess("CallResolver.Resolve")

	return &ResolvedCall{
		TargetID: targetID,
		NodeType: NodeExternal,
		CallType: callType,
		Async:    async,
		Function: targetID,
		Line:     call.Line,
	}
}

// determineCallType определяет тип вызова из CallInfo.
func (r *CallResolver) determineCallType(call analyzer.CallInfo) CallType {
	if call.IsGoroutine {
		return CallGoroutine
	}

	if call.IsDeferred {
		return CallDeferred
	}

	return CallDirect
}

// determineMethodNodeType определяет тип узла для метода.
func (r *CallResolver) determineMethodNodeType(methodInfo *analyzer.MethodInfo) CallNodeType {
	receiverTypeID := methodInfo.Package + "." + methodInfo.Receiver
	typeInfo := r.analyzer.LookupType(receiverTypeID)

	if typeInfo != nil && typeInfo.Kind == "interface" {
		return NodeInterfaceMethod
	}

	return NodeMethod
}

// adjustCallType корректирует тип вызова если метод - интерфейсный.
func (r *CallResolver) adjustCallType(original CallType, nodeType CallNodeType) CallType {
	if nodeType == NodeInterfaceMethod && original == CallDirect {
		return CallInterface
	}

	return original
}
