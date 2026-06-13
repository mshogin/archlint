package analyzer

import (
	"testing"
)

// Layer A: факт guard1 (ForwardedParams) на РЕАЛЬНОМ AST. Проверяем, что параметр,
// используемый ТОЛЬКО как receiver вызова (p.Foo()), НЕ помечается форвардом, а любая
// value-позиция (аргумент, присвоение, return, type-assert, method-value) — помечается.

func hasForwarded(forwarded []string, name string) bool {
	for _, f := range forwarded {
		if f == name {
			return true
		}
	}

	return false
}

// (A1) Чистый receiver: p.Foo() / p.Bar() -> НЕ форвард (это ISP-числитель).
func TestForwarded_PureReceiver_NotForwarded(t *testing.T) {
	src := `package p
type I interface { Foo(); Bar() }
type C struct{}
func (c C) Use(p I) { p.Foo(); p.Bar() }
`
	m := getMethodByName(parseSnippetOS(t, src).methods, "Use")
	if m == nil {
		t.Fatal("метод Use не распарсен")
	}
	if hasForwarded(m.ForwardedParams, "p") {
		t.Fatalf("p используется только как receiver -> не форвард; got %v", m.ForwardedParams)
	}
}

// (A2) Аргумент чужого вызова helper(p) -> форвард.
func TestForwarded_ArgToHelper_Forwarded(t *testing.T) {
	src := `package p
type I interface { Foo() }
func helper(x I) {}
type C struct{}
func (c C) Use(p I) { p.Foo(); helper(p) }
`
	m := getMethodByName(parseSnippetOS(t, src).methods, "Use")
	if !hasForwarded(m.ForwardedParams, "p") {
		t.Fatalf("p форвардится в helper(p) -> форвард; got %v", m.ForwardedParams)
	}
}

// (A3) Присвоение полю -> форвард.
func TestForwarded_FieldAssign_Forwarded(t *testing.T) {
	src := `package p
type I interface { Foo() }
type C struct{ f I }
func (c *C) Use(p I) { c.f = p }
`
	m := getMethodByName(parseSnippetOS(t, src).methods, "Use")
	if !hasForwarded(m.ForwardedParams, "p") {
		t.Fatalf("p присвоен полю -> форвард; got %v", m.ForwardedParams)
	}
}

// (A4) return p -> форвард.
func TestForwarded_Return_Forwarded(t *testing.T) {
	src := `package p
type I interface { Foo() }
type C struct{}
func (c C) Use(p I) I { return p }
`
	m := getMethodByName(parseSnippetOS(t, src).methods, "Use")
	if !hasForwarded(m.ForwardedParams, "p") {
		t.Fatalf("return p -> форвард; got %v", m.ForwardedParams)
	}
}

// (A5) type-assert p.(T) -> форвард (утечка конкретного типа).
func TestForwarded_TypeAssert_Forwarded(t *testing.T) {
	src := `package p
type I interface { Foo() }
type T struct{}
func (c T) Use(p I) { _, _ = p.(*T) }
`
	m := getMethodByName(parseSnippetOS(t, src).methods, "Use")
	if !hasForwarded(m.ForwardedParams, "p") {
		t.Fatalf("p.(T) -> форвард; got %v", m.ForwardedParams)
	}
}

// (A6) method-value p.Foo (без вызова) -> форвард.
func TestForwarded_MethodValue_Forwarded(t *testing.T) {
	src := `package p
type I interface { Foo() }
type C struct{}
func (c C) Use(p I) { f := p.Foo; _ = f }
`
	m := getMethodByName(parseSnippetOS(t, src).methods, "Use")
	if !hasForwarded(m.ForwardedParams, "p") {
		t.Fatalf("method-value p.Foo -> форвард; got %v", m.ForwardedParams)
	}
}

// (A7) NamedParams: имя+тип параметра доступны для ISP-числителя.
func TestNamedParams_Populated(t *testing.T) {
	src := `package p
type I interface { Foo() }
type C struct{}
func (c C) Use(p I) { p.Foo() }
`
	m := getMethodByName(parseSnippetOS(t, src).methods, "Use")
	if len(m.NamedParams) != 1 || m.NamedParams[0].Name != "p" || m.NamedParams[0].TypeName != "I" {
		t.Fatalf("NamedParams должен дать {p I}; got %+v", m.NamedParams)
	}
}
