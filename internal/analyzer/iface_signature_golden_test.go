package analyzer

import (
	"testing"

	"github.com/mshogin/archlint/internal/model"
)

// golden: сигнатуры методов ИНТЕРФЕЙСА -> рёбра usesType/returns ОТ интерфейса
// (DIP по типовому уровню). До правки от интерфейсов было 0 таких рёбер.

func ifaceWithSig(name string, sigs ...model.InterfaceMethodSig) *TypeInfo {
	return &TypeInfo{Name: name, Package: "p", Kind: "interface", MethodSigs: sigs}
}

// (1) Метод интерфейса возвращает КОНКРЕТНЫЙ тип -> returns(I -> Concrete).
// Это и есть видимое DIP-нарушение (абстракция зависит от детали).
func TestIfaceSig_ReturnConcrete(t *testing.T) {
	types := map[string]*TypeInfo{
		"p.Store": ifaceWithSig("Store", model.InterfaceMethodSig{
			Name: "Get", Results: []model.FieldInfo{{TypeName: "Record"}}}),
		"p.Record": {Name: "Record", Package: "p", Kind: "struct"},
	}
	edges := sigEdges(types, nil, nil)
	if !hasEdge(edges, "p.Store", "p.Record", model.EdgeReturns) {
		t.Fatalf("returns I-метод -> Concrete (DIP-нарушение по return) не материализован: %v", edges)
	}
}

// (2) Метод интерфейса принимает КОНКРЕТНЫЙ параметр -> usesType(I -> Concrete).
func TestIfaceSig_ParamConcrete(t *testing.T) {
	types := map[string]*TypeInfo{
		"p.Sink": ifaceWithSig("Sink", model.InterfaceMethodSig{
			Name: "Put", Params: []model.FieldInfo{{TypeName: "Record"}}}),
		"p.Record": {Name: "Record", Package: "p", Kind: "struct"},
	}
	edges := sigEdges(types, nil, nil)
	if !hasEdge(edges, "p.Sink", "p.Record", model.EdgeUses) {
		t.Fatalf("usesType I-метод param-конкрет не материализован: %v", edges)
	}
}

// (3) СОУНДНОСТЬ: примитив/нерезолвимое в сигнатуре I-метода -> НЕ ложит
// ложного abstraction->detail ребра.
func TestIfaceSig_NoEdgeOnPrimitive(t *testing.T) {
	types := map[string]*TypeInfo{
		"p.I": ifaceWithSig("I", model.InterfaceMethodSig{
			Name:    "F",
			Params:  []model.FieldInfo{{TypeName: "int"}},
			Results: []model.FieldInfo{{TypeName: "string"}}}),
	}
	edges := sigEdges(types, nil, nil)
	if len(edges) != 0 {
		t.Fatalf("примитивы в сигнатуре I-метода не должны давать рёбер: %v", edges)
	}
}

// (4) Param-интерфейс (абстракция) -> ребро есть, но цель = интерфейс (не деталь);
// DIP-метрика отфильтрует по kind. Здесь проверяем что ребро ВООБЩЕ материализуется
// на резолвимый тип (полнота), а различение detail/abstraction — задача метрики.
func TestIfaceSig_ParamInterfaceResolves(t *testing.T) {
	types := map[string]*TypeInfo{
		"p.A": ifaceWithSig("A", model.InterfaceMethodSig{
			Name: "Use", Params: []model.FieldInfo{{TypeName: "B"}}}),
		"p.B": {Name: "B", Package: "p", Kind: "interface"},
	}
	edges := sigEdges(types, nil, nil)
	if !hasEdge(edges, "p.A", "p.B", model.EdgeUses) {
		t.Fatalf("usesType I-метод param-интерфейса должен резолвиться (полнота): %v", edges)
	}
}
