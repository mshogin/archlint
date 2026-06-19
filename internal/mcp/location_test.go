package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mshogin/archlint/internal/analyzer"
)

// ApplyLocations дописывает file:line по Target-qname (функции/методы/типы) — чтобы чинить, не
// искать строку вручную. Резолв OCP-сайта "qname|operand" -> qname. Не-сущность (пакет) -> "".
func TestApplyLocations(t *testing.T) {
	dir := t.TempDir()
	src := `package p

type Widget struct{ n int }

func (w *Widget) Do() int { return w.n }

func Free() int { return 1 }
`
	if err := os.WriteFile(filepath.Join(dir, "p.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	a := analyzer.NewGoAnalyzer()
	if _, err := a.Analyze(dir); err != nil {
		t.Fatal(err)
	}

	vs := []Violation{
		{Kind: "dead-code", Target: "p.Free"},
		{Kind: "srp-lack-of-cohesion", Target: "p.Widget"},
		{Kind: "feature-envy", Target: "p.Widget.Do"},
		{Kind: KindOCPOpenModification, Target: "p.Free|x"}, // OCP-сайт: резолв до p.Free
		{Kind: "hub-node", Target: "p"},                     // пакет — не именованная сущность -> ""
	}
	ApplyLocations(a, vs)

	// Функция, тип, метод, OCP-сайт -> непустой file:line; пакет -> пусто.
	for i, want := range []bool{true, true, true, true, false} {
		got := vs[i].Location != ""
		if got != want {
			t.Errorf("vs[%d] (%s target=%s): location=%q, ожидалось непусто=%v", i, vs[i].Kind, vs[i].Target, vs[i].Location, want)
		}
	}

	// Location НЕ влияет на Fingerprint (display-поле).
	withLoc := Violation{Kind: "dead-code", Target: "p.Free", Location: "p.go:7"}
	noLoc := Violation{Kind: "dead-code", Target: "p.Free"}
	if Fingerprint(withLoc) != Fingerprint(noLoc) {
		t.Errorf("Location не должна влиять на Fingerprint: %q != %q", Fingerprint(withLoc), Fingerprint(noLoc))
	}

	// a==nil -> no-op (не паникует).
	ApplyLocations(nil, vs)
}
