package mcp

import (
	"testing"

	"github.com/mshogin/archlint/internal/model"
)

func deadGraph(nodes []model.Node, edges []model.Edge) *model.Graph {
	return &model.Graph{Nodes: nodes, Edges: edges}
}

func isDead(vs []Violation, id string) bool {
	for _, v := range vs {
		if v.Kind == "dead-code" && v.Target == id {
			return true
		}
	}
	return false
}

// (1) недостижимая func -> мёртвая; (2) достижимая через calls -> живая;
// (5) entry из R -> всегда живая.
func TestDeadCode_CallsReachAndOrphan(t *testing.T) {
	g := deadGraph(
		[]model.Node{
			fn("p.main", "main"),     // entry (default R)
			fn("p.used", "used"),     // unexported, вызывается main -> жива
			fn("p.orphan", "orphan"), // unexported, никто не зовёт -> мёртвая
		},
		[]model.Edge{{From: "p.main", To: "p.used", Type: model.EdgeCalls}},
	)
	vs := DeadCode(g, nil)
	if !isDead(vs, "p.orphan") {
		t.Fatalf("orphan должна быть мёртвой; %v", vs)
	}
	if isDead(vs, "p.used") {
		t.Fatal("used достижима через calls -> живая")
	}
	if isDead(vs, "p.main") {
		t.Fatal("main = entry R -> всегда живая")
	}
}

// (3) ★callback через references -> живая (не мёртвая).
func TestDeadCode_ReferenceKeepsAlive(t *testing.T) {
	g := deadGraph(
		[]model.Node{fn("p.main", "main"), fn("p.cb", "cb")}, // cb unexported, только referenced
		[]model.Edge{{From: "p.main", To: "p.cb", Type: model.EdgeReferences}},
	)
	if isDead(DeadCode(g, nil), "p.cb") {
		t.Fatal("callback cb достижим через references -> НЕ мёртвый (иначе destruction)")
	}
}

// (4) ★реализация интерфейса, достижимая ТОЛЬКО через dispatch (i.Foo()) -> живая.
// main использует интерфейс I (usesType) -> I достигнут -> dispatch на T (T implements I)
// -> contains -> T.Foo жива, хотя прямого calls на T.Foo нет.
func TestDeadCode_ImplementsDispatch(t *testing.T) {
	// Имена UNEXPORTED -> не попадают в дефолтный R. R = {main}. Достижимость
	// интерфейса iface — ТОЛЬКО через usesType от живого main, реализация impl.do —
	// ТОЛЬКО через dispatch. Так изолируем implements-dispatch.
	g := deadGraph(
		[]model.Node{
			fn("p.main", "main"),
			kindNode("p.iface", model.KindInterface),
			kindNode("p.impl", model.KindConcrete),
			meth("p.impl.do", "do"),       // unexported, только через dispatch
			meth("p.other.bar", "bar"),    // unexported, не реализует iface, никто не зовёт -> мёртв
		},
		[]model.Edge{
			{From: "p.main", To: "p.iface", Type: model.EdgeUses}, // live func использует интерфейс
			{From: "p.impl", To: "p.iface", Type: model.EdgeImplements},
			{From: "p.impl", To: "p.impl.do", Type: model.EdgeContains},
		},
	)
	vs := DeadCode(g, nil)
	if isDead(vs, "p.impl.do") {
		t.Fatalf("impl.do достижима через implements-dispatch (i.do()) -> живая; %v", vs)
	}
	if !isDead(vs, "p.other.bar") {
		t.Fatalf("other.bar не достижим и не реализует используемый интерфейс -> мёртв; %v", vs)
	}
}

// (5) ★функция, вызванная ТОЛЬКО из теста (Test*) -> живая (test-reachability).
// Test* в авто-R -> helper достижим через calls от теста.
func TestDeadCode_TestOnlyReachable(t *testing.T) {
	g := deadGraph(
		[]model.Node{
			fn("p.helper", "helper"),        // prod, unexported, зовётся только тестом
			fn("p.TestHelper", "TestHelper"), // тест-функция -> в R
		},
		[]model.Edge{{From: "p.TestHelper", To: "p.helper", Type: model.EdgeCalls}},
	)
	if isDead(DeadCode(g, nil), "p.helper") {
		t.Fatal("helper вызван из Test* -> живой (test-reachability), не удаляем")
	}
}

// (бонус) exported func -> в R -> жива даже без входящих рёбер.
func TestDeadCode_ExportedIsEntry(t *testing.T) {
	g := deadGraph(
		[]model.Node{fn("p.PublicAPI", "PublicAPI")},
		nil,
	)
	if isDead(DeadCode(g, nil), "p.PublicAPI") {
		t.Fatal("exported PublicAPI = entry R -> живая")
	}
}
