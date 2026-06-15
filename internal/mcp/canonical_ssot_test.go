package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mshogin/archlint/internal/analyzer"
)

// СТРАЖ №1 канонизации fingerprint (главный): дельта дерева С СОБОЙ пуста.
// delta(collect(X), collect(X)) = ∅ на любом X. Ловит ВЕСЬ класс «опорные точки сравнения
// разошлись» одним тестом: если идентичность нарушения недетерминирована/зависит от прохода,
// один и тот же код дал бы ложные NEW сам против себя.
//
// ТЕКУЩЕЕ СОСТОЯНИЕ (до полного SSOT): collect здесь = ERROR-class сбор на ОДНОМ дереве/пути.
// Страж №1 на ОДНОМ пути проходит (Fingerprint детерминирован per-tree). Класс ломается на
// РАЗНЫХ опорных точках (страж №2 t_root-инвариантность — см. canonical-fingerprint-ssot-plan.md),
// что и был инц.3 (qname с path-префиксом worktree). Этот тест фиксирует страж №1 как регрессионный
// якорь; страж №2 требует module-relative pkgID (массовый golden rebaseline -> продуктовое решение).
func collectErrorClass(t *testing.T, dir string) []Violation {
	t.Helper()

	a := analyzer.NewGoAnalyzer()
	g, err := a.Analyze(dir)
	if err != nil {
		t.Fatal(err)
	}

	var vs []Violation
	vs = append(vs, DetectAllViolations(g)...)
	vs = append(vs, DeadCode(g, nil)...)
	vs = append(vs, ComputeISPUsageSubset(g, a)...)

	return vs
}

func TestCanonical_Guard1_DeltaTreeWithSelfEmpty(t *testing.T) {
	dir := t.TempDir()
	code := `package sample

type Store interface {
	Get() int
	Put(x int)
	Del()
}

func client(s Store) { s.Get() }

func orphan() int { return 1 + 1 + 1 }
`
	if err := os.WriteFile(filepath.Join(dir, "s.go"), []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}

	// collect(X) дважды (независимые проходы того же дерева).
	a := collectErrorClass(t, dir)
	b := collectErrorClass(t, dir)

	// baseline из первого -> дельта второго против него ОБЯЗАНА быть пустой.
	base := BuildBaseline(a)
	d := Delta(b, base)

	if len(d.New) != 0 {
		t.Fatalf("СТРАЖ №1 НАРУШЕН: delta(collect(X),collect(X)) не пуста (%d ложных NEW) — идентичность нарушения недетерминирована: %+v", len(d.New), d.New)
	}
}
