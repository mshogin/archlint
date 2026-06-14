package mcp

import (
	"testing"

	"github.com/mshogin/archlint/internal/model"
)

// ACCEPTANCE: дважды на ОДНОМ графе -> Distance ровно 0, Shifted=false
// (детерминизм-защита: округление λ до 1e-9 делает идентичные спектры точно равными).
func TestSpectralDistance_SameGraph_Acceptance(t *testing.T) {
	g := refGraph()

	d1 := ComputeSpectralDistance(g, g)
	if d1.Distance != 0 || d1.Shifted {
		t.Fatalf("один граф -> Distance=0/Shifted=false, got %+v", d1)
	}

	d2 := ComputeSpectralDistance(g, g)
	if d2 != d1 {
		t.Fatalf("недетерминизм: %+v != %+v", d2, d1)
	}
}

// Структурный сдвиг (триугольник -> путь) -> Shifted=true.
func TestSpectralDistance_Shifted(t *testing.T) {
	e := func(f, t string) model.Edge { return model.Edge{From: f, To: t, Type: "import"} }
	triangle := &model.Graph{
		Nodes: []model.Node{{ID: "A"}, {ID: "B"}, {ID: "C"}},
		Edges: []model.Edge{e("A", "B"), e("B", "C"), e("C", "A")},
	}
	path := &model.Graph{
		Nodes: []model.Node{{ID: "A"}, {ID: "B"}, {ID: "C"}},
		Edges: []model.Edge{e("A", "B"), e("B", "C")},
	}

	d := ComputeSpectralDistance(triangle, path)
	if !d.Shifted || d.Distance <= spectralEpsilon {
		t.Fatalf("триугольник vs путь -> spectral shift ожидался, got %+v", d)
	}
}

// Изоморфные по спектру (переименование узлов) -> Distance 0 (спектр инвариантен).
func TestSpectralDistance_RenameInvariant(t *testing.T) {
	e := func(f, t string) model.Edge { return model.Edge{From: f, To: t, Type: "import"} }
	g1 := &model.Graph{
		Nodes: []model.Node{{ID: "A"}, {ID: "B"}, {ID: "C"}},
		Edges: []model.Edge{e("A", "B"), e("B", "C"), e("C", "A")},
	}
	g2 := &model.Graph{
		Nodes: []model.Node{{ID: "X"}, {ID: "Y"}, {ID: "Z"}},
		Edges: []model.Edge{e("X", "Y"), e("Y", "Z"), e("Z", "X")},
	}

	if d := ComputeSpectralDistance(g1, g2); d.Shifted {
		t.Errorf("изоморфные триугольники -> спектр одинаков, Shifted=false; got %+v", d)
	}
}
