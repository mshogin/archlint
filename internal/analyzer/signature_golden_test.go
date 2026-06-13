package analyzer

import (
	"testing"

	"github.com/mshogin/archlint/internal/model"
)

// golden для signature-рёбер Фазы 1: usesType(param) + returns. Акцент:
// полнота DIP (param-тип интерфейса ловится) при соундности (примитив не ложит).

func sigEdges(types map[string]*TypeInfo, funcs map[string]*FunctionInfo, methods map[string]*MethodInfo) []model.Edge {
	var nodes []model.Node
	var edges []model.Edge
	g := newGoGraphBuilder(nil, types, funcs, methods, &nodes, &edges)
	g.buildSignatureEdges()
	return edges
}

func hasEdge(edges []model.Edge, from, to, etype string) bool {
	for _, e := range edges {
		if e.From == from && e.To == to && e.Type == etype {
			return true
		}
	}
	return false
}

func ref(typeName string) model.FieldInfo { return model.FieldInfo{TypeName: typeName} }

// (1) usesType: метод с параметром известного типа -> ребро uses на тип.
// Особо — param ИНТЕРФЕЙСА (полнота DIP: param-нарушение видно).
func TestSignature_UsesType_ParamInterface(t *testing.T) {
	types := map[string]*TypeInfo{
		"p.Reader": {Name: "Reader", Package: "p", Kind: "interface", MethodSigs: []model.InterfaceMethodSig{{Name: "Read"}}},
		"p.Svc":    {Name: "Svc", Package: "p", Kind: "struct"},
	}
	methods := map[string]*MethodInfo{
		// func (Svc) Handle(r Reader)  -> param Reader (интерфейс)
		"p.Svc.Handle": {Name: "Handle", Receiver: "Svc", Package: "p", Params: []model.FieldInfo{ref("Reader")}},
	}
	edges := sigEdges(types, nil, methods)
	if !hasEdge(edges, "p.Svc.Handle", "p.Reader", model.EdgeUses) {
		t.Fatalf("usesType param-интерфейса не материализован (DIP-полнота); рёбра: %v", edges)
	}
}

// (2) returns: функция возвращает известный тип -> ребро returns.
func TestSignature_Returns(t *testing.T) {
	types := map[string]*TypeInfo{"p.Config": {Name: "Config", Package: "p", Kind: "struct"}}
	funcs := map[string]*FunctionInfo{
		// func NewConfig() Config
		"p.NewConfig": {Name: "NewConfig", Package: "p", Results: []model.FieldInfo{ref("Config")}},
	}
	edges := sigEdges(types, funcs, nil)
	if !hasEdge(edges, "p.NewConfig", "p.Config", model.EdgeReturns) {
		t.Fatalf("returns не материализован; рёбра: %v", edges)
	}
}

// (3) Соундность: примитивный/нерезолвимый param НЕ ложит ребро (ложный edge -> ложный DIP ERROR).
func TestSignature_NoEdgeOnPrimitive(t *testing.T) {
	types := map[string]*TypeInfo{"p.Svc": {Name: "Svc", Package: "p", Kind: "struct"}}
	methods := map[string]*MethodInfo{
		// func (Svc) F(n int) string  -> int/string примитивы, чужой тип не известен
		"p.Svc.F": {Name: "F", Receiver: "Svc", Package: "p",
			Params:  []model.FieldInfo{ref("int")},
			Results: []model.FieldInfo{ref("string")}},
	}
	edges := sigEdges(types, nil, methods)
	if len(edges) != 0 {
		t.Fatalf("примитивы не должны давать signature-рёбер: %v", edges)
	}
}

// (4) usesType param конкретного типа + returns одновременно.
func TestSignature_ParamAndReturnConcrete(t *testing.T) {
	types := map[string]*TypeInfo{
		"p.In":  {Name: "In", Package: "p", Kind: "struct"},
		"p.Out": {Name: "Out", Package: "p", Kind: "struct"},
	}
	funcs := map[string]*FunctionInfo{
		"p.Transform": {Name: "Transform", Package: "p",
			Params:  []model.FieldInfo{ref("In")},
			Results: []model.FieldInfo{ref("Out")}},
	}
	edges := sigEdges(types, funcs, nil)
	if !hasEdge(edges, "p.Transform", "p.In", model.EdgeUses) {
		t.Fatal("usesType param In не пойман")
	}
	if !hasEdge(edges, "p.Transform", "p.Out", model.EdgeReturns) {
		t.Fatal("returns Out не пойман")
	}
}
