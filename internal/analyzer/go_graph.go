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
	// pkgRefs — package-level function-value-use (cobra RunE и т.п.), (а)-фикс.
	// Выставляется go.go (= a.pkgRefs). nil у тест-билдеров -> обрабатывается мягко.
	pkgRefs map[string][]CallInfo
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
	g.buildImplementsEdges()
	g.buildSignatureEdges()
	g.buildReferenceEdges()
	g.buildFieldAccessNodes()
	g.buildFieldAccessEdges()
}

// buildReferenceEdges материализует ребро owner -> функция/метод, использованные
// как ЗНАЧЕНИЕ (callback): символ в НЕ-call позиции (assigned/passed/address-taken),
// собранный парсером (collectFuncRefs).
//
// Резолв цели = ИСТИННАЯ OVER-APPROXIMATION ПО ИМЕНИ: символ с именем N (последний
// сегмент) -> ребро на ВСЕ функции/методы с этим именем, без var-type inference
// (value-flow ниже арх-уровня, исключён). Это ДОБАВЛЯЕТ достижимость -> ложно-живой
// (дёшево), ложно-мёртвый НЕВОЗМОЖЕН (тот же линчпин, что implements; destruction-
// безопасно для dead-code Фазы 3). Имена, не совпадающие ни с одной функцией/методом
// (локальные переменные dir/err/...), просто не дают рёбер.
func (g *GoGraphBuilder) buildReferenceEdges() {
	// индекс: имя (последний сегмент ID) -> все функции/методы с этим именем.
	byName := make(map[string][]string)
	index := func(id string) {
		byName[lastSegment(id)] = append(byName[lastSegment(id)], id)
	}

	for id := range g.functions {
		index(id)
	}
	for id := range g.methods {
		index(id)
	}

	seen := make(map[[2]string]bool)
	emit := func(from, to string) {
		if from == "" || to == "" || from == to {
			return
		}

		k := [2]string{from, to}
		if seen[k] {
			return
		}

		seen[k] = true
		*g.edges = append(*g.edges, model.Edge{From: from, To: to, Type: model.EdgeReferences})
	}

	link := func(ownerID string, refs []CallInfo) {
		for _, ref := range refs {
			for _, target := range byName[lastSegment(ref.Target)] {
				emit(ownerID, target)
			}
		}
	}

	for funcID, fi := range g.functions {
		link(funcID, fi.Refs)
	}
	for methodID, mi := range g.methods {
		link(methodID, mi.Refs)
	}

	// (а)-фикс: package-level var/const function-value-use. Источник = <pkg>.init
	// (выполняется при загрузке пакета, default-entry в R), а если init нет —
	// package-узел (ссылка не теряется). g.pkgRefs nil у тест-билдеров -> пропуск.
	for pkgID, refs := range g.pkgRefs {
		src := pkgID
		if _, ok := g.functions[pkgID+".init"]; ok {
			src = pkgID + ".init"
		}

		link(src, refs)
	}
}

// lastSegment возвращает имя после последней точки (символ из ID или ref.Target).
func lastSegment(s string) string {
	if i := strings.LastIndex(s, "."); i >= 0 {
		return s[i+1:]
	}

	return s
}

// buildSignatureEdges материализует type-refs из СИГНАТУР функций/методов (Фаза 1):
//   - usesType (model.EdgeUses): owner -> тип ПАРАМЕТРА (расширяет "uses", который
//     раньше покрывал только типы полей struct). Ключ полноты DIP: у интерфейса нет
//     тела -> param-типы единственный сигнал param-нарушений.
//   - returns (model.EdgeReturns): owner -> тип ВОЗВРАТА (type-flow).
//
// Соундность приоритетна: resolveTypeDependency эмитит ребро ТОЛЬКО на известный
// тип-узел (примитивы/внешние/нерезолвимые -> нет ребра), поэтому ложного ребра на
// легальном коде не будет. Неполнота (пропуск нерезолвимого) — в дешёвую сторону.
func (g *GoGraphBuilder) buildSignatureEdges() {
	seen := make(map[[3]string]bool)

	emit := func(from, to, etype string) {
		if from == "" || to == "" || from == to {
			return
		}

		k := [3]string{from, to, etype}
		if seen[k] {
			return
		}

		seen[k] = true
		*g.edges = append(*g.edges, model.Edge{From: from, To: to, Type: etype})
	}

	sig := func(ownerID, pkg string, params, results []model.FieldInfo) {
		for _, p := range params {
			emit(ownerID, g.resolveTypeDependency(p.TypeName, p.TypePkg, pkg), model.EdgeUses)
		}

		for _, r := range results {
			emit(ownerID, g.resolveTypeDependency(r.TypeName, r.TypePkg, pkg), model.EdgeReturns)
		}
	}

	for funcID, fi := range g.functions {
		sig(funcID, fi.Package, fi.Params, fi.Results)
	}

	for methodID, mi := range g.methods {
		sig(methodID, mi.Package, mi.Params, mi.Results)
	}

	// Сигнатуры методов ИНТЕРФЕЙСА: ребро ОТ интерфейса (абстракция) к типу в
	// param/return метода. DIP по типовому уровню: интерфейс, ссылающийся на
	// КОНКРЕТ в сигнатуре своего метода = нарушение (источник=абстракция,
	// цель=деталь). resolveTypeDependency -> только известный тип (соундность).
	for ifaceID, ti := range g.types {
		if ti.Kind != "interface" {
			continue
		}

		for _, ms := range ti.MethodSigs {
			sig(ifaceID, ti.Package, ms.Params, ms.Results)
		}
	}
}

// buildImplementsEdges материализует ребро concrete-type -> interface по
// method-set сатисфакции, ПОЛНО с embeds-промоушеном (Go-embedding промоутит
// методы встроенного типа/интерфейса). Критерий: requiredMethods(I) ⊆
// providedMethods(T), где оба множества раскрывают embeds рекурсивно.
//
// Сопоставление ПО ИМЕНИ метода (не по полной сигнатуре — сигнатуры пока не в
// модели). Это КОНСЕРВАТИВНАЯ over-approximation: может дать лишние implements,
// но НЕ пропустит реальную реализацию -> для dead-code reach (Фаза 3) это
// БЕЗОПАСНАЯ сторона (не удалит живую реализацию интерфейса). DR-0005: полнота
// implements — критерий, неполнота рушит reach в дорогую сторону.
func (g *GoGraphBuilder) buildImplementsEdges() {
	// methodSet(typeID) — множество имён методов типа с раскрытием embeds-промоушена.
	// memo + placeholder-guard от циклов (в Go embedding-циклов нет, но безопасно).
	memo := make(map[string]map[string]bool)

	var methodSet func(typeID string) map[string]bool
	methodSet = func(typeID string) map[string]bool {
		if m, ok := memo[typeID]; ok {
			return m
		}

		memo[typeID] = map[string]bool{} // placeholder: рвём возможный цикл

		ti := g.types[typeID]
		if ti == nil {
			return memo[typeID]
		}

		m := make(map[string]bool)

		// собственные методы интерфейса (имена из сигнатур)
		for _, ms := range ti.MethodSigs {
			m[ms.Name] = true
		}

		// собственные receiver-методы конкретного типа
		for _, mi := range g.methods {
			if mi.Package == ti.Package && mi.Receiver == ti.Name {
				m[mi.Name] = true
			}
		}

		// промоушен: методы встроенных типов/интерфейсов (рекурсивно)
		for _, emb := range ti.Embeds {
			embID := g.resolveTypeDependency(emb, "", ti.Package)
			if embID == "" || embID == typeID {
				continue
			}

			for name := range methodSet(embID) {
				m[name] = true
			}
		}

		memo[typeID] = m

		return m
	}

	for ifaceID, iface := range g.types {
		if iface.Kind != "interface" {
			continue
		}

		req := methodSet(ifaceID)
		if len(req) == 0 {
			continue // пустой интерфейс (any) — не плодим тривиальные рёбра
		}

		for typeID, t := range g.types {
			if t.Kind != "struct" || typeID == ifaceID {
				continue
			}

			prov := methodSet(typeID)

			implementsAll := true
			for name := range req {
				if !prov[name] {
					implementsAll = false

					break
				}
			}

			if implementsAll {
				*g.edges = append(*g.edges, model.Edge{
					From: typeID,
					To:   ifaceID,
					Type: model.EdgeImplements,
				})
			}
		}
	}
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
		// Attrs.kind — ось абстрактности (interface|concrete) для DIP. Entity
		// оставляем как было (struct/interface) для обратной совместимости.
		kind := model.KindConcrete
		if typeInfo.Kind == "interface" {
			kind = model.KindInterface
		}

		*g.nodes = append(*g.nodes, model.Node{
			ID:     typeID,
			Title:  typeInfo.Name,
			Entity: typeInfo.Kind,
			Attrs:  map[string]any{"kind": kind},
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

// buildFieldAccessNodes creates "field" nodes for every struct field that is
// accessed by at least one method.  The node ID follows the convention
// <pkg>.<TypeName>.<FieldName> and the entity is "field".
// If a node with that ID already exists (e.g. emitted by an earlier run) it is
// not duplicated.
func (g *GoGraphBuilder) buildFieldAccessNodes() {
	existing := make(map[string]bool, len(*g.nodes))
	for _, n := range *g.nodes {
		existing[n.ID] = true
	}

	// Iterate methods and collect field node IDs to emit.
	for _, methodInfo := range g.methods {
		for _, fa := range methodInfo.FieldAccess {
			nodeID := methodInfo.Package + "." + methodInfo.Receiver + "." + fa.FieldName
			if existing[nodeID] {
				continue
			}

			existing[nodeID] = true
			*g.nodes = append(*g.nodes, model.Node{
				ID:     nodeID,
				Title:  fa.FieldName,
				Entity: model.EntityField,
			})
		}
	}
}

// buildFieldAccessEdges emits field_read / field_write edges from each method
// to the field nodes it accesses.
func (g *GoGraphBuilder) buildFieldAccessEdges() {
	// Deduplicate edges per (method, field, edgeType) triple.
	type edgeKey struct {
		from, to, edgeType string
	}

	emitted := make(map[edgeKey]bool)

	for methodID, methodInfo := range g.methods {
		for _, fa := range methodInfo.FieldAccess {
			fieldNodeID := methodInfo.Package + "." + methodInfo.Receiver + "." + fa.FieldName

			edgeType := model.EdgeFieldRead
			if fa.IsWrite {
				edgeType = model.EdgeFieldWrite
			}

			key := edgeKey{from: methodID, to: fieldNodeID, edgeType: edgeType}
			if emitted[key] {
				continue
			}

			emitted[key] = true
			*g.edges = append(*g.edges, model.Edge{
				From: methodID,
				To:   fieldNodeID,
				Type: edgeType,
			})
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
