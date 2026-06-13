package analyzer

import (
	"fmt"
	"go/ast"
	goparser "go/parser"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
)

// GoParser is responsible for parsing Go files and extracting structural information.
type GoParser struct {
	packages  map[string]*PackageInfo
	types     map[string]*TypeInfo
	functions map[string]*FunctionInfo
	methods   map[string]*MethodInfo
	// pkgRefs — package-level function-value-use по пакетам (а)-фикс. Делится с
	// builder через analyzer (go.go выставляет parser.pkgRefs = a.pkgRefs).
	pkgRefs map[string][]CallInfo
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
		return fmt.Errorf("parse error %s: %w", filename, err)
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
			switch d.Tok {
			case token.TYPE:
				p.parseTypeDecl(d, pkgID, filename, fset)
			case token.VAR, token.CONST:
				// (а)-фикс: package-level var/const инициализаторы тоже несут
				// function-value-use (cobra `var c=&Command{RunE:H}`, slices, maps).
				p.collectPackageLevelRefs(d, pkgID)
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

		var methodSigs []InterfaceMethodSig

		switch t := typeSpec.Type.(type) {
		case *ast.StructType:
			kind = "struct"
			fields, embeds = p.parseStructFields(t)
		case *ast.InterfaceType:
			kind = "interface"
			embeds, methodSigs = p.parseInterfaceEmbeds(t)
		}

		pos := fset.Position(typeSpec.Pos())
		p.types[typeID] = &TypeInfo{
			Name:       typeName,
			Package:    pkgID,
			Kind:       kind,
			File:       filename,
			Line:       pos.Line,
			Fields:     fields,
			Embeds:     embeds,
			MethodSigs: methodSigs,
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

// parseInterfaceEmbeds извлекает встроенные интерфейсы (method.Names==0) И
// ПОЛНЫЕ сигнатуры собственных методов интерфейса (method.Names!=0): имя +
// param/return type-refs. Имена -> method-set implements; param/return -> рёбра
// usesType/returns ОТ интерфейса (DIP по типовому уровню).
func (p *GoParser) parseInterfaceEmbeds(iface *ast.InterfaceType) (embeds []string, methodSigs []InterfaceMethodSig) {
	if iface.Methods == nil {
		return embeds, methodSigs
	}

	for _, method := range iface.Methods.List {
		if len(method.Names) == 0 {
			typeName, _ := p.getTypeName(method.Type)
			embeds = append(embeds, typeName)

			continue
		}

		// method.Type для метода интерфейса — *ast.FuncType (та же сигнатура).
		ft, _ := method.Type.(*ast.FuncType)
		params, results := p.parseSignature(ft)

		for _, name := range method.Names {
			methodSigs = append(methodSigs, InterfaceMethodSig{Name: name.Name, Params: params, Results: results})
		}
	}

	return embeds, methodSigs
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

// parseSignature извлекает type-refs параметров и возвратов из сигнатуры (Фаза 1).
// Имя параметра не важно для type-ref. Нерезолвимые/примитивы отсеются позже в
// билдере (resolveTypeDependency) -> ребро только на известный тип (соундность).
func (p *GoParser) parseSignature(ft *ast.FuncType) (params, results []FieldInfo) {
	collect := func(list *ast.FieldList) []FieldInfo {
		var out []FieldInfo
		if list == nil {
			return out
		}

		for _, f := range list.List {
			typeName, typePkg := p.getTypeName(f.Type)
			if typeName == "" {
				continue
			}

			out = append(out, FieldInfo{TypeName: typeName, TypePkg: typePkg})
		}

		return out
	}

	if ft == nil {
		return params, results
	}

	return collect(ft.Params), collect(ft.Results)
}

// collectParamFacts извлекает ISP-фундамент: именованные параметры (имя+тип) и
// множество параметров, форвардящихся в value-позицию (guard1). Отдельно от
// parseSignature, чтобы не трогать usesType/signature-факты.
func (p *GoParser) collectParamFacts(ft *ast.FuncType, body *ast.BlockStmt) (named []FieldInfo, forwarded []string) {
	if ft == nil || ft.Params == nil {
		return nil, nil
	}

	paramSet := make(map[string]bool)

	for _, f := range ft.Params.List {
		typeName, typePkg := p.getTypeName(f.Type)
		for _, name := range f.Names {
			if name.Name == "_" {
				continue
			}

			named = append(named, FieldInfo{Name: name.Name, TypeName: typeName, TypePkg: typePkg})
			paramSet[name.Name] = true
		}
	}

	if body == nil || len(paramSet) == 0 {
		return named, nil
	}

	return named, p.collectForwardedParams(body, paramSet)
}

// collectForwardedParams — синтаксический факт guard1: множество параметров,
// появляющихся в VALUE-позиции (не только как receiver вызова p.Foo()). Реализация —
// три класса вхождений Ident в теле:
//   - Sel-позиция (p в `x.p`) — это селектор-поле, НЕ переменная p -> игнор;
//   - receiver-of-call (p в `p.Foo()`, т.е. X селектора, который Fun у CallExpr) ->
//     это и есть ISP-числитель, НЕ форвард -> игнор;
//   - всё остальное (аргумент helper(p), RHS присвоения, return p, p.(T), method-value
//     p.Foo как значение) -> VALUE-позиция -> форвард.
//
// Over-approx в безопасную для ISP сторону: при сомнении считаем форвардом ->
// воздержание (no-verdict), а не ложный ISP-ERROR. Имя-коллизия (param shadowing,
// param==имя метода) тоже уходит в форвард = воздержание.
func (p *GoParser) collectForwardedParams(body *ast.BlockStmt, paramSet map[string]bool) []string {
	// Sel-иденты (правая часть селектора `x.Sel`) — не переменные-параметры.
	selIdents := make(map[*ast.Ident]bool)
	// receiver-of-call иденты — X селектора, являющегося Fun у CallExpr (p.Foo()).
	receiverIdents := make(map[*ast.Ident]bool)

	ast.Inspect(body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.SelectorExpr:
			selIdents[s.Sel] = true
		case *ast.CallExpr:
			if sel, ok := s.Fun.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok {
					receiverIdents[id] = true
				}
			}
		}

		return true
	})

	fwd := make(map[string]bool)

	ast.Inspect(body, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok || !paramSet[id.Name] {
			return true
		}

		if selIdents[id] || receiverIdents[id] {
			return true // селектор-поле или receiver вызова -> не форвард
		}

		fwd[id.Name] = true

		return true
	})

	if len(fwd) == 0 {
		return nil
	}

	out := make([]string, 0, len(fwd))
	for k := range fwd {
		out = append(out, k)
	}

	sort.Strings(out)

	return out
}

// collectPackageLevelRefs собирает function-value-use из package-level var/const
// инициализаторов (вложенные composite-literals/slices/maps раскрываются ast.Inspect)
// и складывает в p.pkgRefs[pkgID]. Билдер атрибутирует их узлу <pkg>.init (var-init
// выполняется при загрузке пакета ДО main; если пакет активен — ссылки живые), а
// при отсутствии init — package-узлу. Это (а)-фикс: видимый синтаксис регистрации
// (cobra RunE и т.п.) -> достижимость, БЕЗ config-whitelist (полнота сохраняется).
func (p *GoParser) collectPackageLevelRefs(d *ast.GenDecl, pkgID string) {
	if p.pkgRefs == nil {
		return
	}

	for _, spec := range d.Specs {
		vs, ok := spec.(*ast.ValueSpec)
		if !ok {
			continue
		}

		for _, val := range vs.Values {
			if refs := p.collectValueRefs(val); len(refs) > 0 {
				p.pkgRefs[pkgID] = append(p.pkgRefs[pkgID], refs...)
			}
		}
	}
}

// collectFuncRefs собирает использования функции/метода как ЗНАЧЕНИЯ (callback) в
// ТЕЛЕ функции: Ident/SelectorExpr ВНЕ call-позиции. Принцип направления ошибки:
// пропуск ссылки = ложно-мёртвый код (дорого) -> собираем щедро; лишнее отсеется
// over-approx-резолвом по имени (ложно-живой = дёшево).
func (p *GoParser) collectFuncRefs(body *ast.BlockStmt) []CallInfo {
	if body == nil {
		return nil
	}

	return p.collectValueRefs(body)
}

// collectValueRefs — общий сбор function-value-use по ПРОИЗВОЛЬНОМУ узлу AST
// (тело функции ИЛИ инициализатор package-level var/const). Используется и для
// (а)-фикса cobra: `var c = &cobra.Command{RunE: H}` -> H собран отсюда.
func (p *GoParser) collectValueRefs(root ast.Node) []CallInfo {
	if root == nil {
		return nil
	}

	callFuns := make(map[ast.Expr]bool)

	ast.Inspect(root, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.CallExpr:
			callFuns[s.Fun] = true
		case *ast.GoStmt:
			if s.Call != nil {
				callFuns[s.Call.Fun] = true
			}
		case *ast.DeferStmt:
			if s.Call != nil {
				callFuns[s.Call.Fun] = true
			}
		}

		return true
	})

	var refs []CallInfo

	ast.Inspect(root, func(n ast.Node) bool {
		expr, ok := n.(ast.Expr)
		if !ok || callFuns[expr] {
			return true // не выражение ИЛИ это call-fun (вызов, не значение)
		}

		switch x := expr.(type) {
		case *ast.SelectorExpr:
			if id, ok := x.X.(*ast.Ident); ok {
				refs = append(refs, CallInfo{Target: id.Name + "." + x.Sel.Name, IsMethod: true, Receiver: id.Name})
			}

			return false // не спускаемся в X/Sel (иначе двойной учёт)
		case *ast.Ident:
			refs = append(refs, CallInfo{Target: x.Name})
		}

		return true
	})

	return refs
}

// parseFuncDecl парсит объявления функций и методов.
func (p *GoParser) parseFuncDecl(decl *ast.FuncDecl, pkgID, filename string, fset *token.FileSet) {
	funcName := decl.Name.Name
	pos := fset.Position(decl.Pos())

	calls := p.collectCalls(decl.Body, pkgID, fset)
	params, results := p.parseSignature(decl.Type)
	refs := p.collectFuncRefs(decl.Body)
	namedParams, forwarded := p.collectParamFacts(decl.Type, decl.Body)

	if decl.Recv != nil && len(decl.Recv.List) > 0 {
		receiverType := p.getReceiverType(decl.Recv.List[0].Type)
		methodID := pkgID + "." + receiverType + "." + funcName

		// Collect receiver variable name(s) to detect field accesses.
		var receiverVars []string
		for _, field := range decl.Recv.List {
			for _, name := range field.Names {
				receiverVars = append(receiverVars, name.Name)
			}
		}

		fieldAccess := p.collectFieldAccess(decl.Body, receiverVars, fset)

		p.methods[methodID] = &MethodInfo{
			Name:            funcName,
			Receiver:        receiverType,
			Package:         pkgID,
			File:            filename,
			Line:            pos.Line,
			Calls:           calls,
			FieldAccess:     fieldAccess,
			Params:          params,
			Results:         results,
			Refs:            refs,
			ForwardedParams: forwarded,
			NamedParams:     namedParams,
		}
	} else {
		funcID := pkgID + "." + funcName

		p.functions[funcID] = &FunctionInfo{
			Name:            funcName,
			Package:         pkgID,
			File:            filename,
			Line:            pos.Line,
			Calls:           calls,
			Params:          params,
			Results:         results,
			Refs:            refs,
			ForwardedParams: forwarded,
			NamedParams:     namedParams,
		}
	}
}

// collectFieldAccess walks the function body and collects field accesses on the receiver.
//
// A SelectorExpr where X is one of receiverVars is considered a field access.
// We walk the AST and track parent nodes to determine read vs write:
//   - LHS of an assignment -> write
//   - Target of inc/dec stmt -> write
//   - Operand of UnaryExpr with token.AND -> write (address taken)
//   - All other positions -> read
//
//nolint:gocyclo,cyclop // Field-access classification necessarily handles multiple parent-node kinds.
func (p *GoParser) collectFieldAccess(
	body *ast.BlockStmt,
	receiverVars []string,
	fset *token.FileSet,
) []FieldAccessInfo {
	if body == nil || len(receiverVars) == 0 {
		return nil
	}

	receiverSet := make(map[string]bool, len(receiverVars))
	for _, rv := range receiverVars {
		receiverSet[rv] = true
	}

	// Build a set of write-position nodes using a first pass.
	writeNodes := make(map[ast.Node]bool)

	ast.Inspect(body, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case *ast.AssignStmt:
			for _, lhs := range stmt.Lhs {
				p.markSelectorExprsAsWrite(lhs, writeNodes)
			}
		case *ast.IncDecStmt:
			p.markSelectorExprsAsWrite(stmt.X, writeNodes)
		case *ast.UnaryExpr:
			if stmt.Op == token.AND {
				p.markSelectorExprsAsWrite(stmt.X, writeNodes)
			}
		}
		return true
	})

	// Deduplicate: track (field, isWrite) per method to avoid duplicate edges.
	seen := make(map[string]bool)

	var accesses []FieldAccessInfo

	ast.Inspect(body, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}

		// Only consider direct receiver access: r.Field (not r.Sub.Field).
		ident, ok := sel.X.(*ast.Ident)
		if !ok {
			return true
		}

		if !receiverSet[ident.Name] {
			return true
		}

		fieldName := sel.Sel.Name
		isWrite := writeNodes[n]

		key := fieldName + "|" + boolStr(isWrite)
		if seen[key] {
			return true
		}

		seen[key] = true
		pos := fset.Position(sel.Pos())
		accesses = append(accesses, FieldAccessInfo{
			FieldName: fieldName,
			IsWrite:   isWrite,
			Line:      pos.Line,
		})

		return true
	})

	return accesses
}

// markSelectorExprsAsWrite recursively marks all *ast.SelectorExpr nodes
// reachable from expr (through index, deref, paren) as write positions.
func (p *GoParser) markSelectorExprsAsWrite(expr ast.Expr, writeNodes map[ast.Node]bool) {
	switch e := expr.(type) {
	case *ast.SelectorExpr:
		writeNodes[e] = true
	case *ast.IndexExpr:
		p.markSelectorExprsAsWrite(e.X, writeNodes)
	case *ast.StarExpr:
		p.markSelectorExprsAsWrite(e.X, writeNodes)
	case *ast.ParenExpr:
		p.markSelectorExprsAsWrite(e.X, writeNodes)
	}
}

// boolStr converts a bool to a short string for map keys.
func boolStr(b bool) string {
	if b {
		return "w"
	}

	return "r"
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
