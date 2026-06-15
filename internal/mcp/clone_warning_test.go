package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mshogin/archlint/internal/analyzer"
)

// WARNING-проверка соундности structural-clone (DRY), шаблон как у SRP/DIP. Нотация ДВЕ ОСИ:
//   - W1 construct-validity [интенсионал/элементы]: term(m) ⊆ term(Def_DRY) — метрика читает
//     структуру фрагментов (число/виды вызовов, арность, профиль полей = язык DRY), не магнитуду.
//   - W2 не молчит: больной эталон (точная копипаста — изоморфные фрагменты >=k) -> fire=1.
//   - не always-fire: уникальный фрагмент / одиночка после extract -> НЕ fire.
//   - W3 направленность: устранение дубля (extract common -> один фрагмент) -> метрика ПАДАЕТ
//     (группа размера 1 не fire); «ложное сходство» (изоморфно, семантика разная) = legal FP, НЕ инверсия.
//   - precision [экстенсионал/исходы]: fire ⊇ viol (ложное структурное сходство в FP) -> precision<1 -> WARNING.

func clonesIn(t *testing.T, code string) []Violation {
	t.Helper()

	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "x.go"), []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}

	a := analyzer.NewGoAnalyzer()
	if _, err := a.Analyze(tmpDir); err != nil {
		t.Fatal(err)
	}

	return StructuralClone(a)
}

// БОЛЬНОЙ: два метода с изоморфной формой (по 5 method-вызовов >= cloneMinSize, одинаковая
// арность) — точная структурная копипаста. Семантически РАЗНЫЕ цели -> заодно legal FP.
const cloneSick = `package clone

type T struct{}

func (t *T) CloneA() { t.x(); t.y(); t.z(); t.w(); t.v() }
func (t *T) CloneB() { t.p(); t.q(); t.r(); t.s(); t.u() }

func (t *T) x() {}
func (t *T) y() {}
func (t *T) z() {}
func (t *T) w() {}
func (t *T) v() {}
func (t *T) p() {}
func (t *T) q() {}
func (t *T) r() {}
func (t *T) s() {}
func (t *T) u() {}
`

// ЗДОРОВЫЙ / W3-после-extract: один фрагмент той же формы (дубль устранён) -> группа размера 1.
const cloneHealthy = `package clone

type T struct{}

func (t *T) Only() { t.x(); t.y(); t.z(); t.w(); t.v() }

func (t *T) x() {}
func (t *T) y() {}
func (t *T) z() {}
func (t *T) w() {}
func (t *T) v() {}
`

// Усиленный fingerprint: (1) копипаста с ПЕРЕИМЕНОВАННЫМИ переменными -> всё ещё клон (canonical
// receiver-нумерация устойчива к переименованию); (2) разные функции схожей грубой формы (тот же
// размер/виды, но иной receiver-паттерн) -> НЕ склеиваются ложно (precision++).
const cloneRenameVsDistinct = `package clone

type S struct{}
type U struct{}

// Orig и Copy: идентичная форма, ПЕРЕИМЕНОВАННЫЕ переменные (a/b -> x/y) -> должны склеиться (клон).
func Orig(a *S, b *U) { a.m1(); a.m2(); b.n1(); a.m3(); b.n2() }
func Copy(x *S, y *U) { x.m1(); x.m2(); y.n1(); x.m3(); y.n2() }

// DiffR: тот же размер (5 method-вызовов) и арность, но ИНОЙ receiver-паттерн -> НЕ клон.
func DiffR(a *S, b *U) { a.m1(); b.n1(); a.m2(); b.n2(); a.m3() }

func (s *S) m1() {}
func (s *S) m2() {}
func (s *S) m3() {}
func (u *U) n1() {}
func (u *U) n2() {}
`

func TestStructuralClone_FingerprintPrecision(t *testing.T) {
	vs := clonesIn(t, cloneRenameVsDistinct)

	// qname несёт path-префикс tmpdir (та самая qname-проблема, которую закроет канонизация) ->
	// сверяем по СУФФИКСУ имени функции, а не полному qname.
	has := func(name string) bool {
		for _, v := range vs {
			if strings.HasSuffix(v.Target, "."+name) {
				return true
			}
		}

		return false
	}

	// (1) rename-robustness: Orig+Copy склеены (копипаста с переименованием -> клон).
	if !has("Orig") || !has("Copy") {
		t.Errorf("rename-robustness нарушена: Orig+Copy должны склеиться как клон, got=%+v", vs)
	}

	// (2) precision: DiffR (иной receiver-паттерн) НЕ склеен с Orig/Copy.
	if has("DiffR") {
		t.Errorf("ЛОЖНАЯ СКЛЕЙКА: DiffR (иной receiver-паттерн) ошибочно помечен клоном — fingerprint слишком груб")
	}
}

func TestStructuralClone_WarningSoundness(t *testing.T) {
	// W2 0-FALSE-SILENCE: точная копипаста (2 изоморфных фрагмента >=k) ОБЯЗАНА fire.
	sick := clonesIn(t, cloneSick)
	if len(sick) == 0 {
		t.Fatal("FALSE-SILENCE: точная структурная копипаста не сработала — WARNING обязан fire")
	}

	// fire покрывает ОБА члена пары (детерминированно).
	if len(sick) != 2 {
		t.Errorf("ожидалось 2 структурных клона (CloneA+CloneB), got %d: %+v", len(sick), sick)
	}

	// W3 НАПРАВЛЕННОСТЬ + не always-fire: после extract common остаётся ОДИН фрагмент -> НЕ fire.
	healthy := clonesIn(t, cloneHealthy)
	if len(healthy) != 0 {
		t.Errorf("W3-ИНВЕРСИЯ/always-fire: одиночный фрагмент (дубль устранён) сработал (%d) — метрика должна падать при extract", len(healthy))
	}

	// W3 монотонность: устранение дубля снижает метрику (2 -> 0), инверсии нет.
	if len(healthy) >= len(sick) {
		t.Errorf("W3: extract common не снизил метрику: sick=%d, after-extract=%d", len(sick), len(healthy))
	}
}
