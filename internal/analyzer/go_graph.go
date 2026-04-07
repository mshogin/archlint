package analyzer

import (
	"strings"

	"github.com/mshogin/archlint/internal/model"
)

// GoGraphBuilder is responsible for building a dependency graph from collected data.
type GoGraphBuilder struct {
	packages  map[string]*PackageInfo
	types     map[string]*TypeInfo
	functions map[string]*FunctionInfo
	methods   map[string]*MethodInfo
	nodes     *[]model.Node
	edges     *[]model.Edge
}

// newGoGraphBuilder создает новый построитель графа.
func newGoGraphBuilder(
	packages map[string]*PackageInfo,
	types map[string]*TypeInfo,
	functions map[string]*FunctionInfo,
	methods map[string]*MethodInfo,
	nodes *[]model.Node,
	edges *[]model.Edge,
) *GoGraphBuilder {
	return &GoGraphBuilder{
		packages:  packages,
		types:     types,
		functions: functions,
		methods:   methods,
		nodes:     nodes,
		edges:     edges,
	}
}

// buildGraph строит граф из собранной информации о пакетах.
func (g *GoGraphBuilder) buildGraph() {
	g.buildPackageNodes()
	g.buildTypeNodes()
	g.buildFunctionNodes()
	g.buildMethodNodes()
	g.buildImportEdges()
	g.buildFunctionCallEdges()
	g.buildMethodCallEdges()
	g.buildTypeDependencyEdges()
}

func (g *GoGraphBuilder) buildPackageNodes() {
	for pkgID, pkg := range g.packages {
		*g.nodes = append(*g.nodes, model.Node{
			ID:     pkgID,
			Title:  pkg.Name,
			Entity: "package",
		})
	}
}

func (g *GoGraphBuilder) buildTypeNodes() {
	for typeID, typeInfo := range g.types {
		*g.nodes = append(*g.nodes, model.Node{
			ID:     typeID,
			Title:  typeInfo.Name,
			Entity: typeInfo.Kind,
		})

		*g.edges = append(*g.edges, model.Edge{
			From: typeInfo.Package,
			To:   typeID,
			Type: "contains",
		})
	}
}

func (g *GoGraphBuilder) buildFunctionNodes() {
	for funcID, funcInfo := range g.functions {
		*g.nodes = append(*g.nodes, model.Node{
			ID:     funcID,
			Title:  funcInfo.Name,
			Entity: "function",
		})

		*g.edges = append(*g.edges, model.Edge{
			From: funcInfo.Package,
			To:   funcID,
			Type: "contains",
		})
	}
}

func (g *GoGraphBuilder) buildMethodNodes() {
	for methodID, methodInfo := range g.methods {
		*g.nodes = append(*g.nodes, model.Node{
			ID:     methodID,
			Title:  methodInfo.Name,
			Entity: "method",
		})

		receiverID := methodInfo.Package + "." + methodInfo.Receiver
		*g.edges = append(*g.edges, model.Edge{
			From: receiverID,
			To:   methodID,
			Type: "contains",
		})
	}
}

func (g *GoGraphBuilder) buildImportEdges() {
	for pkgID, pkg := range g.packages {
		for _, imp := range pkg.Imports {
			targetID := g.findPackageByImport(imp)
			if targetID != "" {
				*g.edges = append(*g.edges, model.Edge{
					From: pkgID,
					To:   targetID,
					Type: "import",
				})
			} else {
				*g.nodes = append(*g.nodes, model.Node{
					ID:     imp,
					Title:  g.getLastPathComponent(imp),
					Entity: "external",
				})
				*g.edges = append(*g.edges, model.Edge{
					From: pkgID,
					To:   imp,
					Type: "import",
				})
			}
		}
	}
}

func (g *GoGraphBuilder) buildFunctionCallEdges() {
	for funcID, funcInfo := range g.functions {
		for _, call := range funcInfo.Calls {
			targetID := g.resolveCallTarget(call, funcInfo.Package)
			if targetID != "" && targetID != funcID {
				*g.edges = append(*g.edges, model.Edge{
					From: funcID,
					To:   targetID,
					Type: "calls",
				})
			}
		}
	}
}

func (g *GoGraphBuilder) buildMethodCallEdges() {
	for methodID, methodInfo := range g.methods {
		for _, call := range methodInfo.Calls {
			targetID := g.resolveCallTarget(call, methodInfo.Package)
			if targetID != "" && targetID != methodID {
				*g.edges = append(*g.edges, model.Edge{
					From: methodID,
					To:   targetID,
					Type: "calls",
				})
			}
		}
	}
}

func (g *GoGraphBuilder) buildTypeDependencyEdges() {
	for typeID, typeInfo := range g.types {
		for _, field := range typeInfo.Fields {
			targetID := g.resolveTypeDependency(field.TypeName, field.TypePkg, typeInfo.Package)
			if targetID != "" && targetID != typeID {
				*g.edges = append(*g.edges, model.Edge{
					From: typeID,
					To:   targetID,
					Type: "uses",
				})
			}
		}

		for _, embed := range typeInfo.Embeds {
			targetID := g.resolveTypeDependency(embed, "", typeInfo.Package)
			if targetID != "" && targetID != typeID {
				*g.edges = append(*g.edges, model.Edge{
					From: typeID,
					To:   targetID,
					Type: "embeds",
				})
			}
		}
	}
}

// resolveCallTarget разрешает цель вызова в ID узла.
//
//nolint:funlen,gocyclo // Call resolution requires checking multiple package and type contexts.
func (g *GoGraphBuilder) resolveCallTarget(call CallInfo, callerPkg string) string {
	target := call.Target

	if target == "" || strings.HasPrefix(target, "().") {
		return ""
	}

	builtins := map[string]bool{
		"make": true, "new": true, "len": true, "cap": true,
		"append": true, "copy": true, "delete": true, "close": true,
		"panic": true, "recover": true, "print": true, "println": true,
	}

	parts := strings.Split(target, ".")
	if len(parts) > 0 && builtins[parts[len(parts)-1]] {
		return ""
	}

	if _, exists := g.functions[target]; exists {
		return target
	}

	if _, exists := g.methods[target]; exists {
		return target
	}

	if !strings.Contains(target, "/") {
		withPkg := callerPkg + "." + target
		if _, exists := g.functions[withPkg]; exists {
			return withPkg
		}

		if _, exists := g.methods[withPkg]; exists {
			return withPkg
		}
	}

	if call.IsMethod && call.Receiver != "" {
		for methodID, methodInfo := range g.methods {
			if methodInfo.Package == callerPkg && strings.HasSuffix(methodID, "."+call.Receiver+"."+parts[len(parts)-1]) {
				return methodID
			}
		}
	}

	return ""
}

// resolveTypeDependency разрешает зависимость типа в ID узла.
func (g *GoGraphBuilder) resolveTypeDependency(typeName, typePkg, currentPkg string) string {
	primitives := map[string]bool{
		"string": true, "int": true, "int8": true, "int16": true, "int32": true, "int64": true,
		"uint": true, "uint8": true, "uint16": true, "uint32": true, "uint64": true,
		"float32": true, "float64": true, "bool": true, "byte": true, "rune": true,
		"error": true, "any": true, "interface{}": true, "func": true, "": true,
	}

	if primitives[typeName] {
		return ""
	}

	if typePkg != "" {
		for typeID := range g.types {
			if strings.Contains(typeID, typePkg) && strings.HasSuffix(typeID, "."+strings.Split(typeName, ".")[len(strings.Split(typeName, "."))-1]) {
				return typeID
			}
		}

		return ""
	}

	localID := currentPkg + "." + typeName
	if _, exists := g.types[localID]; exists {
		return localID
	}

	for typeID := range g.types {
		if strings.HasSuffix(typeID, "."+typeName) {
			return typeID
		}
	}

	return ""
}

// findPackageByImport ищет пакет по пути импорта.
func (g *GoGraphBuilder) findPackageByImport(importPath string) string {
	for pkgID, pkg := range g.packages {
		if pkg.Path == importPath {
			return pkgID
		}

		if strings.HasSuffix(pkgID, importPath) {
			return pkgID
		}
	}

	return ""
}

// getLastPathComponent возвращает последний компонент пути.
func (g *GoGraphBuilder) getLastPathComponent(path string) string {
	parts := strings.Split(path, "/")

	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	return path
}
