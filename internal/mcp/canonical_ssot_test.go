package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mshogin/archlint/internal/analyzer"
	"github.com/mshogin/archlint/internal/archlintcfg"
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
	vs = append(vs, DeadCode(g, nil, nil)...)
	vs = append(vs, ComputeISPUsageSubset(g, a)...)

	return vs
}

// СТРАЖ №2 (t_root-инвариантность): collect(из абсолютного пути) == collect(из ".") —
// fingerprint-наборы побитово равны. Корень №3 (module-relative pkgID). Предусловие (граничное
// условие, соундность-ревью): цель скана = ЕДИНЫЙ go-module (для archlint-on-archlint и большинства репо ок;
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

// СТРАЖ №3 (discriminator = семантический якорь, НЕ Message): Fingerprint не зависит от
// display-строки. Корень №4. Изменение Message при том же Anchor -> тот же Fingerprint.
func TestCanonical_Guard3_DiscriminatorNotMessage(t *testing.T) {
	v1 := Violation{Kind: "circular-dependency", Message: "Circular dependency detected (SCC size 2): a <-> b", Anchor: "scc:a,b"}
	v2 := Violation{Kind: "circular-dependency", Message: "СОВСЕМ ИНАЧЕ переформулировано на строке 999", Anchor: "scc:a,b"}

	if Fingerprint(v1) != Fingerprint(v2) {
		t.Fatalf("СТРАЖ №3 НАРУШЕН: Fingerprint зависит от Message (%q != %q) — discriminator не семантический", Fingerprint(v1), Fingerprint(v2))
	}
}

// СТРАЖ №6 (delta при НЕСВЯЗАННОМ сдвиге строк / переформулировке = ∅): приёмка корня №4.
// Тот же структурный паттерн (Anchor стабилен), но иной Message/позиция -> ноль ложных NEW.
func TestCanonical_Guard6_UnrelatedMessageShiftDeltaEmpty(t *testing.T) {
	base := BuildBaseline([]Violation{
		{Kind: "circular-dependency", Message: "Circular dependency (SCC size 2): a <-> b @line10", Anchor: "scc:a,b"},
		{Kind: "layer-violation", Message: "Forbidden: x (app) -> y (infra) @line20", Anchor: "layer:x->y"},
	})

	// Тот же код после несвязанного рефакторинга: Message переформулирован/сдвинут, Anchor тот же.
	cur := []Violation{
		{Kind: "circular-dependency", Message: "ЦИКЛ обнаружен иначе на строке 777: a <-> b", Anchor: "scc:a,b"},
		{Kind: "layer-violation", Message: "Слой нарушен (другой текст) строка 888", Anchor: "layer:x->y"},
	}

	d := Delta(cur, base)
	if len(d.New) != 0 {
		t.Fatalf("СТРАЖ №6 НАРУШЕН: delta при переформулировке Message не пуста (%d ложных NEW) — discriminator-fragility: %+v", len(d.New), d.New)
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

// СТРАЖ №4/№5 (единый collect + единый registry): набор ERROR-class детекторов берётся из
// ОДНОГО реестра (active_metric_registry) через ОДИН сборщик (CollectErrorClassViolations),
// который вызывают и baseline (gate.go), и scan. Реестр непуст; collect детерминирован
// (два вызова на одном графе -> идентичный fingerprint-набор -> симметрия baseline<->scan).
func TestCanonical_Guard45_SingleCollectRegistry(t *testing.T) {
	dir := t.TempDir()
	code := `package sample

type Repo interface {
	Find() int
	Save(x int)
}

func use(r Repo) { r.Find() }
`
	if err := os.WriteFile(filepath.Join(dir, "r.go"), []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}

	a := analyzer.NewGoAnalyzer()
	g, err := a.Analyze(dir)
	if err != nil {
		t.Fatal(err)
	}

	cfg := archlintcfg.Default()

	// Два независимых вызова единого сборщика -> идентичный fingerprint-набор (детерминизм registry).
	s1 := fingerprintSet(t, CollectErrorClassViolations(g, a, &cfg))
	s2 := fingerprintSet(t, CollectErrorClassViolations(g, a, &cfg))

	if len(s1) != len(s2) {
		t.Fatalf("СТРАЖ №4/№5: единый collect недетерминирован (%d != %d)", len(s1), len(s2))
	}

	for fp := range s1 {
		if !s2[fp] {
			t.Fatalf("СТРАЖ №4/№5: единый collect недетерминирован — fingerprint «%s» нестабилен", fp)
		}
	}

	// delta(collect, baseline(collect)) = ∅ через ЕДИНЫЙ источник (симметрия baseline<->scan).
	base := BuildBaseline(CollectErrorClassViolations(g, a, &cfg))
	d := Delta(CollectErrorClassViolations(g, a, &cfg), base)

	if len(d.New) != 0 {
		t.Fatalf("СТРАЖ №4/№5: единый collect не даёт симметрию baseline<->scan (%d ложных NEW)", len(d.New))
	}
}

// dispatchFixture — синтетический type-dispatch (две ветки) для OCP-стражей.
const dispatchFixture = `package sample

type Shape interface{ Area() float64 }
type Circle struct{}
type Square struct{}

func (Circle) Area() float64 { return 0 }
func (Square) Area() float64 { return 0 }

func describe(s Shape) string {
	switch s.(type) {
	case Circle:
		return "c"
	case Square:
		return "s"
	}
	return "?"
}
`

// collectWithDispatch — ERROR-class + ocp-dispatch-site факты (как baseline-генерация).
func collectWithDispatch(t *testing.T, dir string) []Violation {
	t.Helper()

	a := analyzer.NewGoAnalyzer()
	g, err := a.Analyze(dir)
	if err != nil {
		t.Fatal(err)
	}

	cfg := archlintcfg.Default()
	vs := CollectErrorClassViolations(g, a, &cfg)
	vs = append(vs, CollectDispatchFacts(a)...)

	return vs
}

// СТРАЖ OCP (П2, ОБЯЗАТЕЛЬНЫЙ): расширение baseline tracked-set на ocp-dispatch-site (ПЕРВЫЙ
// не-ERROR baseline-tracked kind) НЕ ломает класс «опорные точки сравнения разошлись».
//
//	(1) delta(collect+dispatch(X), collect+dispatch(X)) = ∅ — детерминизм с ocp в tracked-set;
//	(2) baseline РЕАЛЬНО снял dispatch-факты (иначе тест пуст);
//	(3) t_root-инвариантность dispatch-fingerprint — S.identity module-relative (canonical),
//	    НЕ зависит от корня скана (защита от 5-го инцидента: ocp идёт через единый Fingerprint).
func TestCanonical_OCPDispatchFacts_SSOT(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "shape.go"), []byte(dispatchFixture), 0o644); err != nil {
		t.Fatal(err)
	}

	// (1) delta-self = ∅ с ocp в tracked-set.
	base := BuildBaseline(collectWithDispatch(t, dir))
	d := Delta(collectWithDispatch(t, dir), base)
	if len(d.New) != 0 {
		t.Fatalf("СТРАЖ OCP НАРУШЕН: delta-self с ocp-dispatch не пуста (%d ложных NEW): %+v", len(d.New), d.New)
	}

	// (2) baseline реально снял dispatch-факты (Circle/Square ветки сайта describe|s).
	if got := len(base.Patterns[KindOCPDispatchSite]); got == 0 {
		t.Fatalf("СТРАЖ OCP: baseline не снял ocp-dispatch-site факты (got 0) — baseline-tracking не работает")
	}

	// (3) t_root-инвариантность: dispatch-fingerprint(abs) == dispatch-fingerprint(.) (module-relative).
	absA := analyzer.NewGoAnalyzer()
	if _, err := absA.Analyze(dir); err != nil {
		t.Fatal(err)
	}
	absSet := fingerprintSet(t, CollectDispatchFacts(absA))

	t.Chdir(dir)
	dotA := analyzer.NewGoAnalyzer()
	if _, err := dotA.Analyze("."); err != nil {
		t.Fatal(err)
	}
	dotSet := fingerprintSet(t, CollectDispatchFacts(dotA))

	if len(absSet) != len(dotSet) {
		t.Fatalf("СТРАЖ OCP t_root: |dispatch(абс)|=%d != |dispatch(.)|=%d", len(absSet), len(dotSet))
	}
	for fp := range absSet {
		if !dotSet[fp] {
			t.Fatalf("СТРАЖ OCP t_root НАРУШЕН: dispatch-fingerprint «%s» зависит от корня скана (не module-relative)", fp)
		}
	}
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
