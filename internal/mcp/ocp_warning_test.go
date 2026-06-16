package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mshogin/archlint/internal/analyzer"
)

// analyzeOCP парсит синтетику и возвращает analyzer (для CollectDispatchFacts/CollectOCP).
func analyzeOCP(t *testing.T, src string) *analyzer.GoAnalyzer {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "shape.go"), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	a := analyzer.NewGoAnalyzer()
	if _, err := a.Analyze(dir); err != nil {
		t.Fatal(err)
	}

	return a
}

// Два эталона: type-dispatch на 2 ветки (baseline) и на 3 ветки (+Triangle, расширение S).
const ocpTwoBranches = `package shape

type Shape interface{ Area() float64 }
type Circle struct{}
type Square struct{}

func (Circle) Area() float64 { return 0 }
func (Square) Area() float64 { return 0 }

func describe(s Shape) string {
	switch s.(type) {
	case Circle:
		return "circle"
	case Square:
		return "square"
	}
	return "?"
}
`

const ocpThreeBranches = `package shape

type Shape interface{ Area() float64 }
type Circle struct{}
type Square struct{}
type Triangle struct{}

func (Circle) Area() float64   { return 0 }
func (Square) Area() float64   { return 0 }
func (Triangle) Area() float64 { return 0 }

func describe(s Shape) string {
	switch s.(type) {
	case Circle:
		return "circle"
	case Square:
		return "square"
	case Triangle:
		return "triangle"
	}
	return "?"
}
`

// Рефактор switch->полиморфизм: dispatch УДАЛЁН (Δ⁻ S), поведение через метод — улучшение OCP.
const ocpPolymorphic = `package shape

type Shape interface {
	Area() float64
	Describe() string
}
type Circle struct{}
type Square struct{}
type Triangle struct{}

func (Circle) Area() float64   { return 0 }
func (Square) Area() float64   { return 0 }
func (Triangle) Area() float64 { return 0 }
func (Circle) Describe() string   { return "circle" }
func (Square) Describe() string   { return "square" }
func (Triangle) Describe() string { return "triangle" }

func describe(s Shape) string { return s.Describe() }
`

// WARNING-проверка OCP (узкий) — соундность ТОЛЬКО с выправленным знаком W3.
//   БОЛЬНОЙ: новая ветка (Triangle) в существующем S vs baseline -> fire=1 (закрытое модифицировано).
//   КОНТР (знак W3): рефактор switch->полиморфизм (dispatch удалён) -> fire=0 (улучшение OCP НЕ
//     растит метрику; инверсия снята — рост «лучше» не должен подсвечиваться нарушением).
//   КРАЕВЫЕ: новый S целиком -> no-fire (не модификация закрытого); no-baseline -> abstain;
//     сужение (ветка удалена) -> no-fire (Δ⁻ не нарушение).
func TestOCP_WarningCheck_FireAndSign(t *testing.T) {
	// baseline из 2-веточного эталона (слепок S::Circle, S::Square).
	twoA := analyzeOCP(t, ocpTwoBranches)
	baseline2 := BuildBaseline(CollectDispatchFacts(twoA))
	if len(baseline2.Patterns[KindOCPDispatchSite]) != 2 {
		t.Fatalf("baseline должен снять 2 ветки, got %v", baseline2.Patterns[KindOCPDispatchSite])
	}

	// (1) БОЛЬНОЙ: текущий код 3 ветки vs baseline 2 -> fire=1 на Triangle.
	threeA := analyzeOCP(t, ocpThreeBranches)
	sick := CollectOCP(threeA, baseline2)
	if len(sick) != 1 {
		t.Fatalf("больной (новая ветка Triangle): ожидался fire=1, got %d (%+v)", len(sick), sick)
	}
	if sick[0].Kind != KindOCPOpenModification {
		t.Errorf("kind: want %s, got %s", KindOCPOpenModification, sick[0].Kind)
	}
	if bt := branchType(sick[0].Anchor); bt != "Triangle" {
		t.Errorf("ветка-нарушитель: want Triangle, got %q", bt)
	}

	// (2) КОНТР (знак W3): рефактор switch->полиморфизм vs тот же baseline -> fire=0.
	// Улучшение OCP (dispatch удалён) НЕ должно растить метрику — проверка снятой инверсии.
	polyA := analyzeOCP(t, ocpPolymorphic)
	if fire := CollectOCP(polyA, baseline2); len(fire) != 0 {
		t.Errorf("КОНТР (рефактор switch->полиморфизм): ожидался fire=0 (улучшение OCP не растит метрику), got %d (%+v)", len(fire), fire)
	}

	// (3) НОВЫЙ S ЦЕЛИКОМ: текущий 3 ветки vs ПУСТОЙ baseline (S не существовал) -> no-fire
	// (добавление нового type-dispatch — новый код, не модификация закрытого).
	emptyBaseline := BuildBaseline(nil)
	if fire := CollectOCP(threeA, emptyBaseline); len(fire) != 0 {
		t.Errorf("новый S целиком (S∉baseline): ожидался no-fire, got %d (%+v)", len(fire), fire)
	}

	// (4) NO-BASELINE -> abstain (без слепка расширение неотличимо от легально существовавшего).
	if fire := CollectOCP(threeA, nil); len(fire) != 0 {
		t.Errorf("no-baseline: ожидался abstain (0), got %d", len(fire))
	}

	// (5) СУЖЕНИЕ (Δ⁻): baseline 3 ветки, текущий 2 -> no-fire (удаление ветки не нарушение OCP).
	baseline3 := BuildBaseline(CollectDispatchFacts(threeA))
	if fire := CollectOCP(twoA, baseline3); len(fire) != 0 {
		t.Errorf("сужение (ветка удалена): ожидался no-fire, got %d (%+v)", len(fire), fire)
	}
}
