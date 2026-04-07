package callgraph

import "github.com/mshogin/archlint/internal/model"

// Analyzer определяет интерфейс для получения информации об анализируемом коде.
// Интерфейс позволяет pkg/callgraph не зависеть напрямую от internal/analyzer.
type Analyzer interface {
	LookupFunction(funcID string) *model.FunctionInfo
	LookupMethod(methodID string) *model.MethodInfo
	LookupType(typeID string) *model.TypeInfo
	ResolveCallTarget(call model.CallInfo, callerPkg string) string
}
