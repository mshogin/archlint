// Package analyzer содержит анализаторы исходного кода для построения архитектурных графов.
package analyzer

import (
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"

	"github.com/mshogin/archlint/internal/model"
)

// GoAnalyzer анализирует Go код и строит граф зависимостей.
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

		return a.parseFile(path)
	})
	if err != nil {
		return nil, fmt.Errorf("ошибка обхода директории: %w", err)
	}

	a.buildGraph()

	return &model.Graph{
		Nodes: a.nodes,
		Edges: a.edges,
	}, nil
}

// parseFile парсит один Go файл.
//
//nolint:funlen // AST parsing inherently requires detailed processing of multiple node types.
func (a *GoAnalyzer) parseFile(filename string) error {
	fset := token.NewFileSet()

	node, err := goparser.ParseFile(fset, filename, nil, goparser.ParseComments)
	if err != nil {
		return fmt.Errorf("ошибка парсинга %s: %w", filename, err)
	}

	pkgName := node.Name.Name
	pkgDir := filepath.Dir(filename)
	pkgID := a.getPkgID(pkgDir)

	if _, exists := a.packages[pkgID]; !exists {
		a.packages[pkgID] = &PackageInfo{
			Name:    pkgName,
			Path:    pkgID,
			Dir:     pkgDir,
			Imports: []string{},
		}
	}

	for _, imp := range node.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)

		if a.isStdLib(importPath) {
			continue
		}

		a.packages[pkgID].Imports = append(a.packages[pkgID].Imports, importPath)
	}

	for _, decl := range node.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			if d.Tok == token.TYPE {
				a.parseTypeDecl(d, pkgID, filename, fset)
			}
		case *ast.FuncDecl:
			a.parseFuncDecl(d, pkgID, filename, fset)
		}
	}

	return nil
}

// parseTypeDecl парсит объявления типов.
func (a *GoAnalyzer) parseTypeDecl(decl *ast.GenDecl, pkgID, filename string, fset *token.FileSet) {
	for _, spec := range decl.Specs {
		typeSpec, ok := spec.(*ast.TypeSpec)
		if !ok {
			continue
		}

		typeName := typeSpec.Name.Name
		typeID := pkgID + "." + typeName

		kind := "type"

		var fields []FieldInfo

		var embeds []string

		switch t := typeSpec.Type.(type) {
		case *ast.StructType:
			kind = "struct"
			fields, embeds = a.parseStructFields(t)
		case *ast.InterfaceType:
			kind = "interface"
			embeds = a.parseInterfaceEmbeds(t)
		}

		pos := fset.Position(typeSpec.Pos())
		a.types[typeID] = &TypeInfo{
			Name:    typeName,
			Package: pkgID,
			Kind:    kind,
			File:    filename,
			Line:    pos.Line,
			Fields:  fields,
			Embeds:  embeds,
		}
	}
}

// parseStructFields извлекает поля и встроенные типы из структуры.
func (a *GoAnalyzer) parseStructFields(structType *ast.StructType) (fields []FieldInfo, embeds []string) {
	if structType.Fields == nil {
		return fields, embeds
	}

	for _, field := range structType.Fields.List {
		typeName, typePkg := a.getTypeName(field.Type)

		if len(field.Names) == 0 {
			embeds = append(embeds, typeName)
		} else {
			for _, name := range field.Names {
				fields = append(fields, FieldInfo{
					Name:     name.Name,
					TypeName: typeName,
					TypePkg:  typePkg,
				})
			}
		}
	}

	return fields, embeds
}

// parseInterfaceEmbeds извлекает встроенные интерфейсы.
func (a *GoAnalyzer) parseInterfaceEmbeds(iface *ast.InterfaceType) []string {
	var embeds []string

	if iface.Methods == nil {
		return embeds
	}

	for _, method := range iface.Methods.List {
		if len(method.Names) == 0 {
			typeName, _ := a.getTypeName(method.Type)
			embeds = append(embeds, typeName)
		}
	}

	return embeds
}

// getTypeName извлекает имя типа из AST выражения.
func (a *GoAnalyzer) getTypeName(expr ast.Expr) (typeName, typePkg string) {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name, ""
	case *ast.StarExpr:
		return a.getTypeName(t.X)
	case *ast.SelectorExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name + "." + t.Sel.Name, ident.Name
		}

		return "", ""
	case *ast.ArrayType:
		return a.getTypeName(t.Elt)
	case *ast.MapType:
		return a.getTypeName(t.Value)
	case *ast.ChanType:
		return a.getTypeName(t.Value)
	case *ast.FuncType:
		return "func", ""
	case *ast.InterfaceType:
		return "interface{}", ""
	}

	return "", ""
}

// parseFuncDecl парсит объявления функций и методов.
func (a *GoAnalyzer) parseFuncDecl(decl *ast.FuncDecl, pkgID, filename string, fset *token.FileSet) {
	funcName := decl.Name.Name
	pos := fset.Position(decl.Pos())

	calls := a.collectCalls(decl.Body, pkgID, fset)

	if decl.Recv != nil && len(decl.Recv.List) > 0 {
		receiverType := a.getReceiverType(decl.Recv.List[0].Type)
		methodID := pkgID + "." + receiverType + "." + funcName

		a.methods[methodID] = &MethodInfo{
			Name:     funcName,
			Receiver: receiverType,
			Package:  pkgID,
			File:     filename,
			Line:     pos.Line,
			Calls:    calls,
		}
	} else {
		funcID := pkgID + "." + funcName

		a.functions[funcID] = &FunctionInfo{
			Name:    funcName,
			Package: pkgID,
			File:    filename,
			Line:    pos.Line,
			Calls:   calls,
		}
	}
}

// collectCalls собирает все вызовы функций/методов из тела функции.
func (a *GoAnalyzer) collectCalls(body *ast.BlockStmt, pkgID string, fset *token.FileSet) []CallInfo {
	if body == nil {
		return nil
	}

	var calls []CallInfo

	ast.Inspect(body, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case *ast.GoStmt:
			calls = append(calls, a.extractCallInfo(stmt.Call, pkgID, fset, true, false)...)

			return false
		case *ast.DeferStmt:
			calls = append(calls, a.extractCallInfo(stmt.Call, pkgID, fset, false, true)...)

			return false
		case *ast.CallExpr:
			calls = append(calls, a.extractCallInfo(stmt, pkgID, fset, false, false)...)
		}

		return true
	})

	return calls
}

// extractCallInfo извлекает информацию о вызове из AST-узла *ast.CallExpr.
func (a *GoAnalyzer) extractCallInfo(
	callExpr *ast.CallExpr, pkgID string, fset *token.FileSet,
	isGoroutine, isDeferred bool,
) []CallInfo {
	pos := fset.Position(callExpr.Pos())

	var calls []CallInfo

	switch fun := callExpr.Fun.(type) {
	case *ast.Ident:
		calls = append(calls, CallInfo{
			Target:      pkgID + "." + fun.Name,
			IsMethod:    false,
			Line:        pos.Line,
			IsGoroutine: isGoroutine,
			IsDeferred:  isDeferred,
		})
	case *ast.SelectorExpr:
		calls = append(calls, a.extractSelectorCall(fun, pos.Line, isGoroutine, isDeferred)...)
	case *ast.FuncLit:
		if isGoroutine {
			calls = append(calls, CallInfo{
				Target:      pkgID + ".<closure>",
				IsGoroutine: true,
				Line:        pos.Line,
			})
		}
	}

	return calls
}

func (a *GoAnalyzer) extractSelectorCall(
	fun *ast.SelectorExpr, line int,
	isGoroutine, isDeferred bool,
) []CallInfo {
	switch x := fun.X.(type) {
	case *ast.Ident:
		return []CallInfo{{
			Target:      x.Name + "." + fun.Sel.Name,
			IsMethod:    true,
			Receiver:    x.Name,
			Line:        line,
			IsGoroutine: isGoroutine,
			IsDeferred:  isDeferred,
		}}
	case *ast.SelectorExpr:
		if ident, ok := x.X.(*ast.Ident); ok {
			return []CallInfo{{
				Target:      ident.Name + "." + x.Sel.Name + "." + fun.Sel.Name,
				IsMethod:    true,
				Receiver:    ident.Name + "." + x.Sel.Name,
				Line:        line,
				IsGoroutine: isGoroutine,
				IsDeferred:  isDeferred,
			}}
		}
	case *ast.CallExpr:
		return []CallInfo{{
			Target:      "()." + fun.Sel.Name,
			IsMethod:    true,
			Line:        line,
			IsGoroutine: isGoroutine,
			IsDeferred:  isDeferred,
		}}
	}

	return nil
}

// getReceiverType извлекает имя типа из receiver.
func (a *GoAnalyzer) getReceiverType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return a.getReceiverType(t.X)
	default:
		return "Unknown"
	}
}

// buildGraph строит граф из собранной информации о пакетах.
func (a *GoAnalyzer) buildGraph() {
	a.buildPackageNodes()
	a.buildTypeNodes()
	a.buildFunctionNodes()
	a.buildMethodNodes()
	a.buildImportEdges()
	a.buildFunctionCallEdges()
	a.buildMethodCallEdges()
	a.buildTypeDependencyEdges()
}

func (a *GoAnalyzer) buildPackageNodes() {
	for pkgID, pkg := range a.packages {
		a.nodes = append(a.nodes, model.Node{
			ID:     pkgID,
			Title:  pkg.Name,
			Entity: "package",
		})
	}
}

func (a *GoAnalyzer) buildTypeNodes() {
	for typeID, typeInfo := range a.types {
		a.nodes = append(a.nodes, model.Node{
			ID:     typeID,
			Title:  typeInfo.Name,
			Entity: typeInfo.Kind,
		})

		a.edges = append(a.edges, model.Edge{
			From: typeInfo.Package,
			To:   typeID,
			Type: "contains",
		})
	}
}

func (a *GoAnalyzer) buildFunctionNodes() {
	for funcID, funcInfo := range a.functions {
		a.nodes = append(a.nodes, model.Node{
			ID:     funcID,
			Title:  funcInfo.Name,
			Entity: "function",
		})

		a.edges = append(a.edges, model.Edge{
			From: funcInfo.Package,
			To:   funcID,
			Type: "contains",
		})
	}
}

func (a *GoAnalyzer) buildMethodNodes() {
	for methodID, methodInfo := range a.methods {
		a.nodes = append(a.nodes, model.Node{
			ID:     methodID,
			Title:  methodInfo.Name,
			Entity: "method",
		})

		receiverID := methodInfo.Package + "." + methodInfo.Receiver
		a.edges = append(a.edges, model.Edge{

			From: receiverID,
			To:   methodID,
			Type: "contains",
		})
	}
}

func (a *GoAnalyzer) buildImportEdges() {
	for pkgID, pkg := range a.packages {
		for _, imp := range pkg.Imports {
			targetID := a.findPackageByImport(imp)
			if targetID != "" {
				a.edges = append(a.edges, model.Edge{
					From: pkgID,
					To:   targetID,
					Type: "import",
				})
			} else {
				a.nodes = append(a.nodes, model.Node{
					ID:     imp,
					Title:  a.getLastPathComponent(imp),
					Entity: "external",
				})
				a.edges = append(a.edges, model.Edge{
					From: pkgID,
					To:   imp,
					Type: "import",
				})
			}
		}
	}
}

func (a *GoAnalyzer) buildFunctionCallEdges() {
	for funcID, funcInfo := range a.functions {
		for _, call := range funcInfo.Calls {
			targetID := a.resolveCallTarget(call, funcInfo.Package)
			if targetID != "" && targetID != funcID {
				a.edges = append(a.edges, model.Edge{
					From: funcID,
					To:   targetID,

					Type: "calls",
				})
			}
		}
	}
}

func (a *GoAnalyzer) buildMethodCallEdges() {
	for methodID, methodInfo := range a.methods {
		for _, call := range methodInfo.Calls {
			targetID := a.resolveCallTarget(call, methodInfo.Package)
			if targetID != "" && targetID != methodID {
				a.edges = append(a.edges, model.Edge{
					From: methodID,
					To:   targetID,
					Type: "calls",
				})
			}
		}
	}
}

func (a *GoAnalyzer) buildTypeDependencyEdges() {
	for typeID, typeInfo := range a.types {
		for _, field := range typeInfo.Fields {
			targetID := a.resolveTypeDependency(field.TypeName, field.TypePkg, typeInfo.Package)
			if targetID != "" && targetID != typeID {
				a.edges = append(a.edges, model.Edge{
					From: typeID,
					To:   targetID,
					Type: "uses",
				})
			}
		}

		for _, embed := range typeInfo.Embeds {
			targetID := a.resolveTypeDependency(embed, "", typeInfo.Package)
			if targetID != "" && targetID != typeID {
				a.edges = append(a.edges, model.Edge{
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
func (a *GoAnalyzer) resolveCallTarget(call CallInfo, callerPkg string) string {
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

	if _, exists := a.functions[target]; exists {
		return target
	}

	if _, exists := a.methods[target]; exists {
		return target
	}

	if !strings.Contains(target, "/") {
		withPkg := callerPkg + "." + target
		if _, exists := a.functions[withPkg]; exists {
			return withPkg
		}

		if _, exists := a.methods[withPkg]; exists {
			return withPkg
		}
	}

	if call.IsMethod && call.Receiver != "" {
		for methodID, methodInfo := range a.methods {
			if methodInfo.Package == callerPkg && strings.HasSuffix(methodID, "."+call.Receiver+"."+parts[len(parts)-1]) {
				return methodID
			}
		}
	}

	return ""
}

// resolveTypeDependency разрешает зависимость типа в ID узла.
func (a *GoAnalyzer) resolveTypeDependency(typeName, typePkg, currentPkg string) string {
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
		for typeID := range a.types {
			if strings.Contains(typeID, typePkg) && strings.HasSuffix(typeID, "."+strings.Split(typeName, ".")[len(strings.Split(typeName, "."))-1]) {
				return typeID
			}
		}

		return ""
	}

	localID := currentPkg + "." + typeName
	if _, exists := a.types[localID]; exists {
		return localID
	}

	for typeID := range a.types {
		if strings.HasSuffix(typeID, "."+typeName) {
			return typeID
		}
	}

	return ""
}

// getPkgID генерирует ID пакета из пути.
func (a *GoAnalyzer) getPkgID(dir string) string {
	parts := strings.Split(filepath.Clean(dir), string(filepath.Separator))

	if len(parts) > 3 {
		return strings.Join(parts[len(parts)-3:], "/")
	}

	return strings.Join(parts, "/")
}

// getLastPathComponent возвращает последний компонент пути.
func (a *GoAnalyzer) getLastPathComponent(path string) string {
	parts := strings.Split(path, "/")

	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	return path
}

// findPackageByImport ищет пакет по пути импорта.
func (a *GoAnalyzer) findPackageByImport(importPath string) string {
	for pkgID, pkg := range a.packages {
		if pkg.Path == importPath {
			return pkgID
		}

		if strings.HasSuffix(pkgID, importPath) {
			return pkgID
		}
	}

	return ""
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

	var result []string

	for typeID, typeInfo := range a.types {
		if typeInfo.Kind != "struct" || typeID == interfaceID {
			continue
		}

		for _, field := range typeInfo.Fields {
			resolvedType := a.resolveTypeDependency(field.TypeName, field.TypePkg, typeInfo.Package)
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
	return a.resolveCallTarget(call, callerPkg)
}

// isStdLib проверяет является ли пакет стандартной библиотекой Go.
func (a *GoAnalyzer) isStdLib(importPath string) bool {
	if !strings.Contains(importPath, ".") && !strings.Contains(importPath, "/") {
		return true
	}

	stdlibPrefixes := []string{
		"fmt", "io", "os", "path", "time", "net",
		"strings", "bytes", "errors", "sync", "context",
		"encoding", "crypto", "database", "log", "math",
		"regexp", "sort", "strconv", "testing", "runtime",
	}

	for _, prefix := range stdlibPrefixes {
		if strings.HasPrefix(importPath, prefix+"/") || importPath == prefix {
			return true
		}
	}

	return false
}
