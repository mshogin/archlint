package callgraph

import (
	"strings"

	"github.com/mshogin/archlint/internal/model"
)

// CallResolver разрешает цели вызовов: прямые, через интерфейс, горутины.
type CallResolver struct {
	analyzer Analyzer
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
func NewCallResolver(a Analyzer) *CallResolver {
	return &CallResolver{analyzer: a}
}

// builtinFuncs is the set of Go builtin functions and common type conversions
// that should be silently ignored during call resolution.
var builtinFuncs = map[string]bool{
	"make": true, "new": true, "len": true, "cap": true,
	"append": true, "copy": true, "delete": true, "close": true,
	"panic": true, "recover": true, "print": true, "println": true,
	// type conversions
	"string": true, "int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
	"float32": true, "float64": true, "bool": true, "byte": true, "rune": true,
	"complex64": true, "complex128": true, "uintptr": true,
	"error": true, "any": true,
}

// isBuiltinCall returns true if call.Target refers to a Go builtin or type conversion.
func isBuiltinCall(target string) bool {
	parts := strings.Split(target, ".")
	last := parts[len(parts)-1]

	return builtinFuncs[last]
}

// Resolve разрешает вызов в конкретную цель.
//
//nolint:funlen // Call resolution requires checking multiple target types.
func (r *CallResolver) Resolve(call model.CallInfo, callerPkg string) *ResolvedCall {
	targetID := r.analyzer.ResolveCallTarget(call, callerPkg)

	callType := r.determineCallType(call)
	async := call.IsGoroutine

	if targetID == "" {
		if call.Target == "" || strings.HasPrefix(call.Target, "().") {
			return nil
		}

		// Filter out builtins and type conversions - they are not meaningful call graph nodes.
		if isBuiltinCall(call.Target) {
			return nil
		}

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
func (r *CallResolver) determineCallType(call model.CallInfo) CallType {
	if call.IsGoroutine {
		return CallGoroutine
	}

	if call.IsDeferred {
		return CallDeferred
	}

	return CallDirect
}

// determineMethodNodeType определяет тип узла для метода.
func (r *CallResolver) determineMethodNodeType(methodInfo *model.MethodInfo) CallNodeType {
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
