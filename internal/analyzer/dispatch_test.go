package analyzer

import (
	"os"
	"path/filepath"
	"testing"
)

// AST-детектор type-dispatch (OCP П1): из type-switch извлекаются Operand (идентичность сайта)
// и отсортированное множество типов-веток. Указатель сохраняется (*T != T). default/nil не тип.
func TestCollectTypeDispatches(t *testing.T) {
	dir := t.TempDir()
	src := `package disp

type Shape interface{ Area() float64 }
type Circle struct{}
type Square struct{}
type Rect struct{}

func (c Circle) Area() float64 { return 0 }
func (s Square) Area() float64 { return 0 }
func (r *Rect) Area() float64  { return 0 }

// Сайт A: type-switch по операнду s, три ветки (Square, *Rect, Circle).
func describe(s Shape) string {
	switch v := s.(type) {
	case Circle:
		_ = v
		return "circle"
	case Square:
		return "square"
	case *Rect:
		return "rect"
	default:
		return "?"
	}
}

// Метод с type-switch (тоже собирается).
type Visitor struct{}

func (vis *Visitor) Visit(n any) {
	switch n.(type) {
	case Circle:
	case Square:
	}
}

// Функция БЕЗ type-switch (обычный switch по значению) -> не dispatch.
func plain(x int) string {
	switch x {
	case 1:
		return "one"
	default:
		return "other"
	}
}
`
	if err := os.WriteFile(filepath.Join(dir, "disp.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	a := NewGoAnalyzer()
	if _, err := a.Analyze(dir); err != nil {
		t.Fatal(err)
	}

	// Функция describe: один dispatch, операнд "s", типы {Circle, Square, *Rect} (отсортировано).
	var describeFn *FunctionInfo
	for id, f := range a.AllFunctions() {
		if f.Name == "describe" {
			describeFn = f
			_ = id
		}
	}
	if describeFn == nil {
		t.Fatal("функция describe не найдена")
	}
	if len(describeFn.Dispatches) != 1 {
		t.Fatalf("describe: ожидался 1 dispatch, got %d (%+v)", len(describeFn.Dispatches), describeFn.Dispatches)
	}
	d := describeFn.Dispatches[0]
	if d.Operand != "s" {
		t.Errorf("operand: want s, got %q", d.Operand)
	}
	wantTypes := []string{"*Rect", "Circle", "Square"}
	if len(d.Types) != len(wantTypes) {
		t.Fatalf("types: want %v, got %v", wantTypes, d.Types)
	}
	for i := range wantTypes {
		if d.Types[i] != wantTypes[i] {
			t.Errorf("types[%d]: want %q (*Rect сохраняет указатель, sort), got %q", i, wantTypes[i], d.Types[i])
		}
	}

	// Метод Visit: dispatch по операнду "n", типы {Circle, Square}.
	var visit *MethodInfo
	for _, m := range a.AllMethods() {
		if m.Name == "Visit" {
			visit = m
		}
	}
	if visit == nil {
		t.Fatal("метод Visit не найден")
	}
	if len(visit.Dispatches) != 1 || visit.Dispatches[0].Operand != "n" {
		t.Fatalf("Visit: ожидался dispatch operand=n, got %+v", visit.Dispatches)
	}

	// Функция plain: switch по ЗНАЧЕНИЮ (не type-switch) -> 0 dispatches.
	for _, f := range a.AllFunctions() {
		if f.Name == "plain" && len(f.Dispatches) != 0 {
			t.Errorf("plain: switch-по-значению не должен давать dispatch, got %+v", f.Dispatches)
		}
	}
}
