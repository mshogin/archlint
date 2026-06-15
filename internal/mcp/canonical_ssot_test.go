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

// СТРАЖ №2 (t_root-инвариантность): collect(из абсолютного пути) == collect(из ".") —
// fingerprint-наборы побитово равны. Корень №3 (module-relative pkgID). Предусловие (край
// Сократа): цель скана = ЕДИНЫЙ go-module (для archlint-on-archlint и большинства репо ок;
// nested go.work — отдельный резолв module-root, см. canonical-fingerprint-ssot-plan.md).
func TestCanonical_Guard2_TRootInvariance(t *testing.T) {
	dir := t.TempDir()
	code := `package sample

type Store interface {
	Get() int
	Put(x int)
}

func client(s Store) { s.Get() }
`
	if err := os.WriteFile(filepath.Join(dir, "s.go"), []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}

	// collect из АБСОЛЮТНОГО пути.
	abs := fingerprintSet(t, collectErrorClass(t, dir))

	// collect из "." (cwd = то же дерево).
	t.Chdir(dir)

	dot := fingerprintSet(t, collectErrorClass(t, "."))

	if len(abs) == 0 {
		t.Fatal("ожидались нарушения в эталоне (иначе тест пуст)")
	}

	if len(abs) != len(dot) {
		t.Fatalf("СТРАЖ №2 НАРУШЕН: |collect(абс)|=%d != |collect(.)|=%d", len(abs), len(dot))
	}

	for fp := range abs {
		if !dot[fp] {
			t.Fatalf("СТРАЖ №2 НАРУШЕН (t_root): fingerprint «%s» есть в collect(абс), нет в collect(.) — qname зависит от корня", fp)
		}
	}
}

// fingerprintSet — множество (Kind|Fingerprint) для сравнения наборов независимо от порядка.
func fingerprintSet(t *testing.T, vs []Violation) map[string]bool {
	t.Helper()

	set := make(map[string]bool, len(vs))
	for _, v := range vs {
		set[v.Kind+"|"+Fingerprint(v)] = true
	}

	return set
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
