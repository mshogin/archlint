// Package analyzer содержит анализаторы исходного кода для построения архитектурных графов.
package analyzer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mshogin/archlint/internal/model"
)

// GoAnalyzer анализирует Go код и строит граф зависимостей.
// Оркестрирует GoParser и GoGraphBuilder, сохраняя публичный API.
type GoAnalyzer struct {
	packages  map[string]*PackageInfo
	types     map[string]*TypeInfo
	functions map[string]*FunctionInfo
	methods   map[string]*MethodInfo
	nodes     []model.Node
	edges     []model.Edge
}

// PackageInfo содержит информацию о пакете.
type PackageInfo struct {
	Name    string
	Path    string
	Dir     string
	Imports []string
}

// TypeInfo - псевдоним для model.TypeInfo для обратной совместимости.
type TypeInfo = model.TypeInfo

// FieldInfo - псевдоним для model.FieldInfo для обратной совместимости.
type FieldInfo = model.FieldInfo

// FunctionInfo - псевдоним для model.FunctionInfo для обратной совместимости.
type FunctionInfo = model.FunctionInfo

// MethodInfo - псевдоним для model.MethodInfo для обратной совместимости.
type MethodInfo = model.MethodInfo

// CallInfo - псевдоним для model.CallInfo для обратной совместимости.
type CallInfo = model.CallInfo

// NewGoAnalyzer создает новый анализатор Go кода.
func NewGoAnalyzer() *GoAnalyzer {
	return &GoAnalyzer{
		packages:  make(map[string]*PackageInfo),
		types:     make(map[string]*TypeInfo),
		functions: make(map[string]*FunctionInfo),
		methods:   make(map[string]*MethodInfo),
		nodes:     []model.Node{},
		edges:     []model.Edge{},
	}
}

// Analyze анализирует директорию с Go кодом.
func (a *GoAnalyzer) Analyze(dir string) (*model.Graph, error) {
	parser := newGoParser(a.packages, a.types, a.functions, a.methods)

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			name := info.Name()
			if name == "vendor" || name == "node_modules" || name == ".git" || name == "bin" {
				return filepath.SkipDir
			}

			return nil
		}

		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}

		return parser.parseFile(path)
	})
	if err != nil {
		return nil, fmt.Errorf("ошибка обхода директории: %w", err)
	}

	builder := newGoGraphBuilder(a.packages, a.types, a.functions, a.methods, &a.nodes, &a.edges)
	builder.buildGraph()

	return &model.Graph{
		Nodes: a.nodes,
		Edges: a.edges,
	}, nil
}

// LookupFunction возвращает информацию о функции по ID.
func (a *GoAnalyzer) LookupFunction(funcID string) *FunctionInfo {
	return a.functions[funcID]
}

// LookupMethod возвращает информацию о методе по ID.
func (a *GoAnalyzer) LookupMethod(methodID string) *MethodInfo {
	return a.methods[methodID]
}

// LookupType возвращает информацию о типе по ID.
func (a *GoAnalyzer) LookupType(typeID string) *TypeInfo {
	return a.types[typeID]
}

// FindImplementations ищет конкретные типы, реализующие интерфейс.
// Возвращает IDs типов, у которых совпадают все методы интерфейса (best-effort).
func (a *GoAnalyzer) FindImplementations(interfaceID string) []string {
	iface := a.types[interfaceID]
	if iface == nil || iface.Kind != "interface" {
		return nil
	}

	builder := newGoGraphBuilder(a.packages, a.types, a.functions, a.methods, &a.nodes, &a.edges)

	var result []string

	for typeID, typeInfo := range a.types {
		if typeInfo.Kind != "struct" || typeID == interfaceID {
			continue
		}

		for _, field := range typeInfo.Fields {
			resolvedType := builder.resolveTypeDependency(field.TypeName, field.TypePkg, typeInfo.Package)
			if resolvedType == interfaceID {
				result = append(result, typeID)

				break
			}
		}
	}

	return result
}

// AllFunctions возвращает все найденные функции.
func (a *GoAnalyzer) AllFunctions() map[string]*FunctionInfo {
	return a.functions
}

// AllMethods возвращает все найденные методы.
func (a *GoAnalyzer) AllMethods() map[string]*MethodInfo {
	return a.methods
}

// AllTypes возвращает все найденные типы.
func (a *GoAnalyzer) AllTypes() map[string]*TypeInfo {
	return a.types
}

// ResolveCallTarget разрешает цель вызова в ID узла (публичный доступ).
func (a *GoAnalyzer) ResolveCallTarget(call CallInfo, callerPkg string) string {
	builder := newGoGraphBuilder(a.packages, a.types, a.functions, a.methods, &a.nodes, &a.edges)

	return builder.resolveCallTarget(call, callerPkg)
}
