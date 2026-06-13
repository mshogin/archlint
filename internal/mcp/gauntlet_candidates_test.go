package mcp

import (
	"testing"

	"github.com/mshogin/archlint/internal/model"
)

// Golden горнило-кандидатов против networkx на A-F:
// articulation = {A,C,D}; bridges = {A-F, C-D, D-E} (3); stability = 0.
// SEVERITY этих Kind'ов НЕ решён (нет в severity_class) — вердикт после self-горнила.

func TestArticulation_vsPython(t *testing.T) {
	v := ArticulationPoints(refGraph())
	got := map[string]bool{}
	for _, x := range v {
		got[x.Target] = true
	}

	for _, want := range []string{"A", "C", "D"} {
		if !got[want] {
			t.Errorf("ожидалась articulation-точка %s; got %v", want, v)
		}
	}
	if len(v) != 3 {
		t.Errorf("ожидалось 3 articulation points, got %d: %+v", len(v), v)
	}
}

func TestBridges_vsPython(t *testing.T) {
	v := BridgeEdges(refGraph())
	if len(v) != 3 {
		t.Errorf("ожидалось 3 bridge edges (A-F, C-D, D-E), got %d: %+v", len(v), v)
	}
}

func TestStability_None_onAF(t *testing.T) {
	if v := StabilityViolations(refGraph()); len(v) != 0 {
		t.Errorf("на A-F нет SDP-нарушений, ожидался 0, got %d: %+v", len(v), v)
	}
}

// stability: стабильный S (I=0.25) зависит от нестабильного U (I=0.75) -> нарушение.
func TestStability_Detect(t *testing.T) {
	e := func(f, t string) model.Edge { return model.Edge{From: f, To: t, Type: "import"} }
	g := &model.Graph{
		Nodes: []model.Node{
			{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "S"}, {ID: "U"}, {ID: "x"}, {ID: "y"}, {ID: "z"},
		},
		Edges: []model.Edge{
			e("a", "S"), e("b", "S"), e("c", "S"), // S in=3
			e("S", "U"),                           // S out=1 (I=0.25), U in=1
			e("U", "x"), e("U", "y"), e("U", "z"), // U out=3 (I=0.75)
		},
	}

	v := StabilityViolations(g)
	if len(v) != 1 || v[0].Target != "S" {
		t.Fatalf("ожидалось 1 SDP-нарушение S->U, got %d: %+v", len(v), v)
	}
}
