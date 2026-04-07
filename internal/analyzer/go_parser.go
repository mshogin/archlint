package analyzer

import (
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"path/filepath"
	"strings"
)

// GoParser отвечает за парсинг Go файлов и извлечение структурной информации.
type GoParser struct {
	packages  map[string]*PackageInfo
	types     map[string]*TypeInfo
	functions map[string]*FunctionInfo
	methods   map[string]*MethodInfo
}

// newGoParser создает новый парсер, работающий с переданными хранилищами данных.
func newGoParser(
	packages map[string]*PackageInfo,
	types map[string]*TypeInfo,
	functions map[string]*FunctionInfo,
	methods map[string]*MethodInfo,
) *GoParser {
	return &GoParser{
		packages:  packages,
		types:     types,
		functions: functions,
		methods:   methods,
	}
}

// parseFile парсит один Go файл.
//
//nolint:funlen // AST parsing inherently requires detailed processing of multiple node types.
func (p *GoParser) parseFile(filename string) error {
	fset := token.NewFileSet()

	node, err := goparser.ParseFile(fset, filename, nil, goparser.ParseComments)
	if err != nil {
		return fmt.Errorf("ошибка парсинга %s: %w", filename, err)
	}

	pkgName := node.Name.Name
	pkgDir := filepath.Dir(filename)
	pkgID := p.getPkgID(pkgDir)

	if _, exists := p.packages[pkgID]; !exists {
		p.packages[pkgID] = &PackageInfo{
			Name:    pkgName,
			Path:    pkgID,
			Dir:     pkgDir,
			Imports: []string{},
		}
	}

	for _, imp := range node.Imports {
		importPath := strings.Trim(imp.Path.Value, `"`)

		if p.isStdLib(importPath) {
			continue
		}

		p.packages[pkgID].Imports = append(p.packages[pkgID].Imports, importPath)
	}

	for _, decl := range node.Decls {
		switch d := decl.(type) {
		case *ast.GenDecl:
			if d.Tok == token.TYPE {
				p.parseTypeDecl(d, pkgID, filename, fset)
			}
		case *ast.FuncDecl:
			p.parseFuncDecl(d, pkgID, filename, fset)
		}
	}

	return nil
}

// parseTypeDecl парсит объявления типов.
func (p *GoParser) parseTypeDecl(decl *ast.GenDecl, pkgID, filename string, fset *token.FileSet) {
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
			fields, embeds = p.parseStructFields(t)
		case *ast.InterfaceType:
			kind = "interface"
			embeds = p.parseInterfaceEmbeds(t)
		}

		pos := fset.Position(typeSpec.Pos())
		p.types[typeID] = &TypeInfo{
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
func (p *GoParser) parseStructFields(structType *ast.StructType) (fields []FieldInfo, embeds []string) {
	if structType.Fields == nil {
		return fields, embeds
	}

	for _, field := range structType.Fields.List {
		typeName, typePkg := p.getTypeName(field.Type)

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
func (p *GoParser) parseInterfaceEmbeds(iface *ast.InterfaceType) []string {
	var embeds []string

	if iface.Methods == nil {
		return embeds
	}

	for _, method := range iface.Methods.List {
		if len(method.Names) == 0 {
			typeName, _ := p.getTypeName(method.Type)
			embeds = append(embeds, typeName)
		}
	}

	return embeds
}

// getTypeName извлекает имя типа из AST выражения.
func (p *GoParser) getTypeName(expr ast.Expr) (typeName, typePkg string) {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name, ""
	case *ast.StarExpr:
		return p.getTypeName(t.X)
	case *ast.SelectorExpr:
		if ident, ok := t.X.(*ast.Ident); ok {
			return ident.Name + "." + t.Sel.Name, ident.Name
		}

		return "", ""
	case *ast.ArrayType:
		return p.getTypeName(t.Elt)
	case *ast.MapType:
		return p.getTypeName(t.Value)
	case *ast.ChanType:
		return p.getTypeName(t.Value)
	case *ast.FuncType:
		return "func", ""
	case *ast.InterfaceType:
		return "interface{}", ""
	}

	return "", ""
}

// parseFuncDecl парсит объявления функций и методов.
func (p *GoParser) parseFuncDecl(decl *ast.FuncDecl, pkgID, filename string, fset *token.FileSet) {
	funcName := decl.Name.Name
	pos := fset.Position(decl.Pos())

	calls := p.collectCalls(decl.Body, pkgID, fset)

	if decl.Recv != nil && len(decl.Recv.List) > 0 {
		receiverType := p.getReceiverType(decl.Recv.List[0].Type)
		methodID := pkgID + "." + receiverType + "." + funcName

		p.methods[methodID] = &MethodInfo{
			Name:     funcName,
			Receiver: receiverType,
			Package:  pkgID,
			File:     filename,
			Line:     pos.Line,
			Calls:    calls,
		}
	} else {
		funcID := pkgID + "." + funcName

		p.functions[funcID] = &FunctionInfo{
			Name:    funcName,
			Package: pkgID,
			File:    filename,
			Line:    pos.Line,
			Calls:   calls,
		}
	}
}

// collectCalls собирает все вызовы функций/методов из тела функции.
func (p *GoParser) collectCalls(body *ast.BlockStmt, pkgID string, fset *token.FileSet) []CallInfo {
	if body == nil {
		return nil
	}

	var calls []CallInfo

	ast.Inspect(body, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case *ast.GoStmt:
			calls = append(calls, p.extractCallInfo(stmt.Call, pkgID, fset, true, false)...)

			return false
		case *ast.DeferStmt:
			calls = append(calls, p.extractCallInfo(stmt.Call, pkgID, fset, false, true)...)

			return false
		case *ast.CallExpr:
			calls = append(calls, p.extractCallInfo(stmt, pkgID, fset, false, false)...)
		}

		return true
	})

	return calls
}

// extractCallInfo извлекает информацию о вызове из AST-узла *ast.CallExpr.
func (p *GoParser) extractCallInfo(
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
		calls = append(calls, p.extractSelectorCall(fun, pos.Line, isGoroutine, isDeferred)...)
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

func (p *GoParser) extractSelectorCall(
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
func (p *GoParser) getReceiverType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return p.getReceiverType(t.X)
	default:
		return "Unknown"
	}
}

// getPkgID генерирует ID пакета из пути.
func (p *GoParser) getPkgID(dir string) string {
	parts := strings.Split(filepath.Clean(dir), string(filepath.Separator))

	if len(parts) > 3 {
		return strings.Join(parts[len(parts)-3:], "/")
	}

	return strings.Join(parts, "/")
}

// isStdLib проверяет является ли пакет стандартной библиотекой Go.
func (p *GoParser) isStdLib(importPath string) bool {
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
