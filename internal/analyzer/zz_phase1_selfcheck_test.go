package analyzer

import (
	"testing"

	"github.com/mshogin/archlint/internal/model"
)

// Self-оракул Фазы 1 на archlint-на-себе (реальный граф internal/): implements-рёбра
// материализуются, type-узлы несут Attrs.kind. Полнота важнее — проверяем что НЕ ноль.
func TestPhase1_SelfOracle(t *testing.T) {
	g, err := NewGoAnalyzer().Analyze("..")
	if err != nil {
		t.Fatalf("analyze internal/: %v", err)
	}
	var impl, kindAttr, ifaceNodes int
	for _, e := range g.Edges {
		if e.Type == model.EdgeImplements {
			impl++
		}
	}
	for _, n := range g.Nodes {
		if n.Attrs != nil {
			if k, ok := n.Attrs["kind"]; ok {
				kindAttr++
				if k == model.KindInterface {
					ifaceNodes++
				}
			}
		}
	}
	t.Logf("nodes=%d edges=%d implements=%d kind-attr-nodes=%d interfaces=%d",
		len(g.Nodes), len(g.Edges), impl, kindAttr, ifaceNodes)
	if impl == 0 {
		t.Fatal("0 implements-рёбер на реальном коде с интерфейсами — материализация не работает")
	}
	if kindAttr == 0 {
		t.Fatal("ни один type-узел не несёт Attrs.kind")
	}
}
