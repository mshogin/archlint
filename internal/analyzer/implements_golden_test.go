package analyzer

import (
	"testing"

	"github.com/mshogin/archlint/internal/model"
)

// golden для материализации implements-ребра (Фаза 1, ADR-0002).
// Особый акцент — ПОЛНОТА с embeds-промоушеном (struct/interface embedding).

func implementsEdges(types map[string]*TypeInfo, methods map[string]*MethodInfo) []model.Edge {
	var nodes []model.Node
	var edges []model.Edge
	g := newGoGraphBuilder(nil, types, nil, methods, &nodes, &edges)
	g.buildImplementsEdges()
	return edges
}

func hasImpl(edges []model.Edge, from, to string) bool {
	for _, e := range edges {
		if e.Type == model.EdgeImplements && e.From == from && e.To == to {
			return true
		}
	}
	return false
}

func iface(name string, methods ...string) *TypeInfo {
	var sigs []model.InterfaceMethodSig
	for _, m := range methods {
		sigs = append(sigs, model.InterfaceMethodSig{Name: m})
	}
	return &TypeInfo{Name: name, Package: "p", Kind: "interface", MethodSigs: sigs}
}
func strct(name string, embeds ...string) *TypeInfo {
	return &TypeInfo{Name: name, Package: "p", Kind: "struct", Embeds: embeds}
}
func meth(recv, name string) *MethodInfo {
	return &MethodInfo{Package: "p", Receiver: recv, Name: name}
}

// (1) Прямая реализация: S.Foo удовлетворяет I{Foo}.
func TestImplements_Direct(t *testing.T) {
	edges := implementsEdges(
		map[string]*TypeInfo{"p.I": iface("I", "Foo"), "p.S": strct("S")},
		map[string]*MethodInfo{"p.S.Foo": meth("S", "Foo")},
	)
	if !hasImpl(edges, "p.S", "p.I") {
		t.Fatalf("S должен реализовывать I; рёбра: %v", edges)
	}
}

// (2) КРИТИЧНЫЙ: embeds-промоушен struct->struct. Derived встроил Base (метод Foo),
// сам Foo не объявляет -> реализует I через промоушен. Неполнота здесь = Фаза 3
// удалит живую реализацию.
func TestImplements_EmbedsPromotion_StructInStruct(t *testing.T) {
	edges := implementsEdges(
		map[string]*TypeInfo{
			"p.I":       iface("I", "Foo"),
			"p.Base":    strct("Base"),
			"p.Derived": strct("Derived", "Base"),
		},
		map[string]*MethodInfo{"p.Base.Foo": meth("Base", "Foo")},
	)
	if !hasImpl(edges, "p.Derived", "p.I") {
		t.Fatalf("Derived должен реализовывать I через промоушен Base.Foo; рёбра: %v", edges)
	}
	if !hasImpl(edges, "p.Base", "p.I") {
		t.Fatalf("Base тоже реализует I")
	}
}

// (3) embeds-промоушен interface-in-struct: T встроил интерфейс E{Foo} -> промоутит Foo.
func TestImplements_EmbedsPromotion_InterfaceInStruct(t *testing.T) {
	edges := implementsEdges(
		map[string]*TypeInfo{
			"p.I": iface("I", "Foo"),
			"p.E": iface("E", "Foo"),
			"p.T": strct("T", "E"),
		},
		map[string]*MethodInfo{},
	)
	if !hasImpl(edges, "p.T", "p.I") {
		t.Fatalf("T должен реализовывать I через встроенный интерфейс E; рёбра: %v", edges)
	}
}

// (4) Интерфейс встраивает интерфейс: req(I)=I.Foo ∪ J.Bar. S с обоими -> implements I и J.
func TestImplements_InterfaceEmbedsInterface(t *testing.T) {
	I := iface("I", "Foo")
	I.Embeds = []string{"J"}
	edges := implementsEdges(
		map[string]*TypeInfo{"p.I": I, "p.J": iface("J", "Bar"), "p.S": strct("S")},
		map[string]*MethodInfo{"p.S.Foo": meth("S", "Foo"), "p.S.Bar": meth("S", "Bar")},
	)
	if !hasImpl(edges, "p.S", "p.I") {
		t.Fatalf("S (Foo+Bar) должен реализовывать I(embeds J); рёбра: %v", edges)
	}
	if !hasImpl(edges, "p.S", "p.J") {
		t.Fatalf("S должен реализовывать и J")
	}
}

// (5) Негатив: X без нужного метода НЕ реализует; пустой интерфейс не плодит рёбер.
func TestImplements_NoFalsePositive(t *testing.T) {
	edges := implementsEdges(
		map[string]*TypeInfo{
			"p.I":     iface("I", "Foo"),
			"p.X":     strct("X"),
			"p.Empty": iface("Empty"), // пустой интерфейс
		},
		map[string]*MethodInfo{"p.X.Bar": meth("X", "Bar")}, // не Foo
	)
	if hasImpl(edges, "p.X", "p.I") {
		t.Fatalf("X (только Bar) НЕ должен реализовывать I{Foo}")
	}
	for _, e := range edges {
		if e.To == "p.Empty" {
			t.Fatalf("пустой интерфейс не должен плодить implements-рёбер: %v", e)
		}
	}
}
