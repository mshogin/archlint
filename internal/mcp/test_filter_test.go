package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mshogin/archlint/internal/analyzer"
)

// DropTestTargets убирает находки, чей Target объявлен в _test.go (заглушки, сьюты,
// тест-функции) или лежит во внешнем тест-пакете (<pkg>_test). Прод-символы и
// пакетные находки остаются: тесты участвуют в графе, но не в отчёте.
func TestDropTestTargets(t *testing.T) {
	dir := t.TempDir()
	files := map[string]string{
		"p.go": `package p

type Widget struct{ n int }

func (w *Widget) Do() int { return w.n }
`,
		"p_test.go": `package p

import "testing"

type stubWidget struct{ a, b int }

func (s *stubWidget) A() int { return s.a }

func TestDo(t *testing.T) { _ = (&Widget{}).Do() }
`,
		"ext_test.go": `package p_test

import "testing"

func TestExt(t *testing.T) {}
`,
	}
	for name, src := range files {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(src), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	a := analyzer.NewGoAnalyzer()
	if _, err := a.Analyze(dir); err != nil {
		t.Fatal(err)
	}

	vs := []Violation{
		{Kind: "srp-lack-of-cohesion", Target: "p.Widget"},          // прод-тип -> остаётся
		{Kind: "feature-envy", Target: "p.Widget.Do"},               // прод-метод -> остаётся
		{Kind: "srp-lack-of-cohesion", Target: "p.stubWidget"},      // тип из _test.go -> убрать
		{Kind: "feature-envy", Target: "p.stubWidget.A"},            // метод из _test.go -> убрать
		{Kind: "structural-clone", Target: "p.TestDo"},              // тест-функция -> убрать
		{Kind: "high-efferent-coupling", Target: "p_test"},          // внешний тест-пакет -> убрать
		{Kind: "hub-node", Target: "p"},                             // пакет -> остаётся
		{Kind: KindOCPOpenModification, Target: "p.stubWidget.A|x"}, // OCP-сайт в тесте -> убрать
		{Kind: "cycles", Anchor: "p,q"},                             // без Target -> остаётся
	}

	got := DropTestTargets(a, vs)

	want := map[string]bool{"p.Widget": true, "p.Widget.Do": true, "p": true, "": true}
	if len(got) != len(want) {
		t.Fatalf("got %d violations, want %d: %+v", len(got), len(want), got)
	}
	for _, v := range got {
		if !want[v.Target] {
			t.Errorf("unexpected violation kept: %s %s", v.Kind, v.Target)
		}
	}
}

// Без Go-анализатора (stdin/TS/Rust) фильтр по файлам невозможен: убираются только
// находки на внешних тест-пакетах, остальное не трогаем (honest: не знаем файл).
func TestDropTestTargetsNilAnalyzer(t *testing.T) {
	vs := []Violation{
		{Kind: "hub-node", Target: "p"},
		{Kind: "high-efferent-coupling", Target: "p_test"},
		{Kind: "god-class", Target: "p.stubWidget"},
	}

	got := DropTestTargets(nil, vs)
	if len(got) != 2 || got[0].Target != "p" || got[1].Target != "p.stubWidget" {
		t.Fatalf("unexpected result: %+v", got)
	}
}
