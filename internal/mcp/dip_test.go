package mcp

import (
	"testing"

	"github.com/mshogin/archlint/internal/model"
)

func kindNode(id, kind string) model.Node {
	return model.Node{ID: id, Entity: "type", Attrs: map[string]any{"kind": kind}}
}

func dipGraph(nodes []model.Node, edges []model.Edge) *model.Graph {
	return &model.Graph{Nodes: nodes, Edges: edges}
}

func dipHas(vs []Violation, iface, concrete string) bool {
	for _, v := range vs {
		if v.Kind == "dip-abstraction-to-detail" && v.Target == iface &&
			containsSub(v.Message, concrete) {
			return true
		}
	}
	return false
}
func containsSub(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// (1) Интерфейс возвращает/принимает СВОЙ concrete -> нарушение DIP.
func TestDIP_InterfaceToConcrete(t *testing.T) {
	g := dipGraph(
		[]model.Node{kindNode("p.Store", model.KindInterface), kindNode("p.Record", model.KindConcrete)},
		[]model.Edge{
			{From: "p.Store", To: "p.Record", Type: model.EdgeReturns},
			{From: "p.Store", To: "p.Record", Type: model.EdgeUses},
		},
	)
	vs := DetectDIP(g)
	if !dipHas(vs, "p.Store", "p.Record") {
		t.Fatalf("DIP: интерфейс->свой concrete не пойман; %v", vs)
	}
	if len(vs) != 1 {
		t.Fatalf("дедуп: одно нарушение на пару (I,C), got %d: %v", len(vs), vs)
	}
}

// (2) Интерфейс ссылается на ДРУГУЮ абстракцию (interface) -> НЕ нарушение.
func TestDIP_InterfaceToInterface_NoViolation(t *testing.T) {
	g := dipGraph(
		[]model.Node{kindNode("p.A", model.KindInterface), kindNode("p.B", model.KindInterface)},
		[]model.Edge{{From: "p.A", To: "p.B", Type: model.EdgeUses}},
	)
	if vs := DetectDIP(g); len(vs) != 0 {
		t.Fatalf("abstraction->abstraction НЕ нарушение DIP: %v", vs)
	}
}

// (3) Интерфейс ссылается на внешний/нерезолвимый узел (нет kind=concrete) -> НЕ нарушение.
func TestDIP_ExternalTarget_NoViolation(t *testing.T) {
	g := dipGraph(
		// p.Ext без Attrs.kind (внешний/нерезолвимый); p.I интерфейс
		[]model.Node{kindNode("p.I", model.KindInterface), {ID: "ext.Thing", Entity: "external"}},
		[]model.Edge{{From: "p.I", To: "ext.Thing", Type: model.EdgeReturns}},
	)
	if vs := DetectDIP(g); len(vs) != 0 {
		t.Fatalf("внешний/нерезолвимый таргет НЕ нарушение (соундность): %v", vs)
	}
}

// (4) calls-ребро (не usesType/returns) от интерфейса не считается DIP.
func TestDIP_OnlySignatureEdges(t *testing.T) {
	g := dipGraph(
		[]model.Node{kindNode("p.I", model.KindInterface), kindNode("p.C", model.KindConcrete)},
		[]model.Edge{{From: "p.I", To: "p.C", Type: "calls"}},
	)
	if vs := DetectDIP(g); len(vs) != 0 {
		t.Fatalf("только usesType/returns -> DIP; calls не считается: %v", vs)
	}
}
