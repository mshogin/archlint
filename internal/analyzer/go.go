// Package analyzer contains source code analyzers for building architecture graphs.
package analyzer

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mshogin/archlint/internal/model"
)

// GoAnalyzer analyzes Go code and builds a dependency graph.
// It orchestrates GoParser and GoGraphBuilder while preserving the public API.
type GoAnalyzer struct {
	packages    map[string]*PackageInfo
	types       map[string]*TypeInfo
	functions   map[string]*FunctionInfo
	methods     map[string]*MethodInfo
	nodes       []model.Node
	edges       []model.Edge
	excludeDirs []string
	pkgRefs     map[string][]CallInfo // package-level function-value-use ((а)-фикс)
}

// PackageInfo holds information about a package.
type PackageInfo struct {
	Name    string
	Path    string
	Dir     string
	Imports []string
}

// TypeInfo is an alias for model.TypeInfo for backward compatibility.
type TypeInfo = model.TypeInfo

// FieldInfo is an alias for model.FieldInfo for backward compatibility.
type FieldInfo = model.FieldInfo

// FunctionInfo is an alias for model.FunctionInfo for backward compatibility.
type FunctionInfo = model.FunctionInfo

// MethodInfo is an alias for model.MethodInfo for backward compatibility.
type MethodInfo = model.MethodInfo

// CallInfo is an alias for model.CallInfo for backward compatibility.
type CallInfo = model.CallInfo

// FieldAccessInfo is an alias for model.FieldAccessInfo for backward compatibility.
type FieldAccessInfo = model.FieldAccessInfo

// InterfaceMethodSig is an alias for model.InterfaceMethodSig.
type InterfaceMethodSig = model.InterfaceMethodSig

// NewGoAnalyzer creates a new Go code analyzer.
func NewGoAnalyzer() *GoAnalyzer {
	return &GoAnalyzer{
		packages:  make(map[string]*PackageInfo),
		types:     make(map[string]*TypeInfo),
		functions: make(map[string]*FunctionInfo),
		methods:   make(map[string]*MethodInfo),
		nodes:     []model.Node{},
		edges:     []model.Edge{},
		pkgRefs:   make(map[string][]CallInfo),
	}
}

// WithExcludeDirs sets additional directory basenames to skip during the walk.
// Additive on top of built-in defaults (vendor, node_modules, .git, bin).
func (a *GoAnalyzer) WithExcludeDirs(dirs []string) *GoAnalyzer {
	a.excludeDirs = dirs
	return a
}

// Analyze analyzes a directory containing Go code.
func (a *GoAnalyzer) Analyze(dir string) (*model.Graph, error) {
	parser := newGoParser(a.packages, a.types, a.functions, a.methods)
	parser.pkgRefs = a.pkgRefs

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			name := info.Name()
			if name == "vendor" || name == "node_modules" || name == ".git" || name == "bin" {
				return filepath.SkipDir
			}
			if MatchesExclude(name, a.excludeDirs) {
				return filepath.SkipDir
			}

			return nil
		}

		// _test.go ТОЖЕ парсим (test-reachability, Фаза 3): тест-вызовы/ссылки в граф,
		// иначе test-only хелперы (вызваны лишь из теста) ложно-мёртвы -> destruction
		// (удалить -> сломать тест). Test*/Benchmark*/Example* в авто-R -> прод-функция,
		// вызванная только из теста, достижима через них.
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		return parser.parseFile(path)
	})
	if err != nil {
		return nil, fmt.Errorf("directory walk error: %w", err)
	}

	builder := newGoGraphBuilder(a.packages, a.types, a.functions, a.methods, &a.nodes, &a.edges)
	builder.pkgRefs = a.pkgRefs
	builder.buildGraph()

	return &model.Graph{
		Nodes: a.nodes,
		Edges: a.edges,
	}, nil
}

// LookupFunction returns function information by ID.
func (a *GoAnalyzer) LookupFunction(funcID string) *FunctionInfo {
	return a.functions[funcID]
}

// LookupMethod returns method information by ID.
func (a *GoAnalyzer) LookupMethod(methodID string) *MethodInfo {
	return a.methods[methodID]
}

// LookupType returns type information by ID.
func (a *GoAnalyzer) LookupType(typeID string) *TypeInfo {
	return a.types[typeID]
}

// FindImplementations searches for concrete types implementing an interface.
// Returns IDs of types that match all interface methods (best-effort).
func (a *GoAnalyzer) FindImplementations(interfaceID string) []string {
	iface := a.types[interfaceID]
	if iface == nil || iface.Kind != "interface" {
		return nil
	}

	builder := newGoGraphBuilder(a.packages, a.types, a.functions, a.methods, &a.nodes, &a.edges)
	builder.pkgRefs = a.pkgRefs

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

// AllFunctions returns all discovered functions.
func (a *GoAnalyzer) AllFunctions() map[string]*FunctionInfo {
	return a.functions
}

// AllMethods returns all discovered methods.
func (a *GoAnalyzer) AllMethods() map[string]*MethodInfo {
	return a.methods
}

// AllTypes returns all discovered types.
func (a *GoAnalyzer) AllTypes() map[string]*TypeInfo {
	return a.types
}

// ResolveCallTarget resolves a call target to a node ID (public access).
func (a *GoAnalyzer) ResolveCallTarget(call CallInfo, callerPkg string) string {
	builder := newGoGraphBuilder(a.packages, a.types, a.functions, a.methods, &a.nodes, &a.edges)
	builder.pkgRefs = a.pkgRefs

	return builder.resolveCallTarget(call, callerPkg)
}
