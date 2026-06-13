package analyzer

import (
	"os"
	"strings"
	"testing"

	"github.com/mshogin/archlint/internal/model"
)

// (5) Реальный парс: чистый вызов f() -> calls, НЕ references; value-use var h=f ->
// references. Проверяет синтаксический сбор (collectFuncRefs исключает call-позицию)
// + name-based линковку на настоящем коде.
func TestReferences_CallVsValue_RealParse(t *testing.T) {
	src := `package p
func target() {}
func caller() { target() }
func user() { var h = target; _ = h }
`
	dir := t.TempDir()
	if err := os.WriteFile(dir+"/snippet.go", []byte(src), 0o600); err != nil {
		t.Fatal(err)
	}
	g, err := NewGoAnalyzer().Analyze(dir)
	if err != nil {
		t.Fatal(err)
	}

	edge := func(from, to, etype string) bool {
		for _, e := range g.Edges {
			if e.Type == etype && strings.HasSuffix(e.From, "."+from) && strings.HasSuffix(e.To, "."+to) {
				return true
			}
		}
		return false
	}

	if !edge("caller", "target", "calls") {
		t.Fatal("caller->target: ожидается calls-ребро")
	}
	if edge("caller", "target", model.EdgeReferences) {
		t.Fatal("чистый вызов target() НЕ должен давать references (не дублируем calls)")
	}
	if !edge("user", "target", model.EdgeReferences) {
		t.Fatal("value-use 'var h = target' должен давать references (F не мёртв)")
	}
}

// golden для references-ребра (Фаза 1): функция/метод как значение (callback).
// Принцип: резолв-фильтр оставляет только реальные функции (соундность таргета),
// но собираем щедро (полнота — пропуск ссылки = ложно-мёртвый код, дорого).

func refEdges(funcs map[string]*FunctionInfo, methods map[string]*MethodInfo) []model.Edge {
	var nodes []model.Node
	var edges []model.Edge
	g := newGoGraphBuilder(nil, nil, funcs, methods, &nodes, &edges)
	g.buildReferenceEdges()
	return edges
}

// (1) Функция передана как значение -> references-ребро на неё.
func TestReferences_FuncAsValue(t *testing.T) {
	funcs := map[string]*FunctionInfo{
		"p.handler":  {Name: "handler", Package: "p"},
		"p.register": {Name: "register", Package: "p", Refs: []model.CallInfo{{Target: "p.handler"}}},
	}
	edges := refEdges(funcs, nil)
	if !hasEdge(edges, "p.register", "p.handler", model.EdgeReferences) {
		t.Fatalf("references на функцию-значение не материализован; рёбра: %v", edges)
	}
}

// (2) Метод как значение -> references на метод.
func TestReferences_MethodAsValue(t *testing.T) {
	methods := map[string]*MethodInfo{"p.S.Do": {Name: "Do", Receiver: "S", Package: "p"}}
	funcs := map[string]*FunctionInfo{
		"p.use": {Name: "use", Package: "p", Refs: []model.CallInfo{{Target: "p.S.Do"}}},
	}
	edges := refEdges(funcs, methods)
	if !hasEdge(edges, "p.use", "p.S.Do", model.EdgeReferences) {
		t.Fatalf("references на метод-значение не материализован; рёбра: %v", edges)
	}
}

// (3) Имя НЕ совпадает ни с одной функцией/методом (локальная переменная) -> нет ребра.
func TestReferences_NoEdgeOnNonFunc(t *testing.T) {
	funcs := map[string]*FunctionInfo{
		"p.f": {Name: "f", Package: "p", Refs: []model.CallInfo{{Target: "someLocalVar"}}},
	}
	edges := refEdges(funcs, nil)
	if len(edges) != 0 {
		t.Fatalf("имя без совпадающей функции/метода не должно давать references: %v", edges)
	}
}

// (4) OVER-APPROXIMATION ПО ИМЕНИ: ссылка x.Do (имя Do) -> рёбра на ВСЕ методы Do
// (без var-типа не знаем какой именно -> на все = добавляем достижимость, ложно-мёртвый
// невозможен). Истинная over-approx (на все, не подмножество).
func TestReferences_OverApproxByName(t *testing.T) {
	methods := map[string]*MethodInfo{
		"p.A.Do": {Name: "Do", Receiver: "A", Package: "p"},
		"p.B.Do": {Name: "Do", Receiver: "B", Package: "p"},
	}
	funcs := map[string]*FunctionInfo{
		"p.use": {Name: "use", Package: "p", Refs: []model.CallInfo{{Target: "x.Do", IsMethod: true, Receiver: "x"}}},
	}
	edges := refEdges(funcs, methods)
	if !hasEdge(edges, "p.use", "p.A.Do", model.EdgeReferences) || !hasEdge(edges, "p.use", "p.B.Do", model.EdgeReferences) {
		t.Fatalf("over-approx: x.Do должен дать рёбра на ВСЕ методы Do (A.Do и B.Do); %v", edges)
	}
}
