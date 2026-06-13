package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mshogin/archlint/internal/analyzer"
	"github.com/mshogin/archlint/internal/model"
)

// Goldens ISP usage-subset (golden id:isp). Прогон через РЕАЛЬНЫЙ анализатор
// (parser ForwardedParams/NamedParams -> метрика), не конструированные структуры —
// честная проверка всей цепочки. Критерий горнила = 0 false-ERROR; здесь фиксируем
// 5 базовых вердиктов из спеки.

func runISP(t *testing.T, src string) []Violation {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "fixture.go"), []byte(src), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	a := analyzer.NewGoAnalyzer()

	graph, err := a.Analyze(dir)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	return ComputeISPUsageSubset(graph, a)
}

func ofKind(vs []Violation, kind string) []Violation {
	var out []Violation

	for _, v := range vs {
		if v.Kind == kind {
			out = append(out, v)
		}
	}

	return out
}

// (1) param-интерфейс, строгое подмножество, не форвард -> ISP (own, ERROR-кандидат).
func TestISP_Golden1_SubsetNotForwarded_ISP(t *testing.T) {
	src := `package demo
type I interface { Foo(); Bar() }
type C struct{}
func (c C) Use(p I) { p.Foo() }
`
	got := ofKind(runISP(t, src), KindISPUsageSubset)
	if len(got) != 1 {
		t.Fatalf("ожидался 1 isp-usage-subset (own), got %d: %+v", len(got), got)
	}
}

// (2) форвард i в helper -> NO-VERDICT (guard1).
func TestISP_Golden2_Forwarded_NoVerdict(t *testing.T) {
	src := `package demo
type I interface { Foo(); Bar() }
func helper(x I) {}
type C struct{}
func (c C) Use(p I) { p.Foo(); helper(p) }
`
	if got := runISP(t, src); len(got) != 0 {
		t.Fatalf("форвард -> воздержание, ожидался 0, got %d: %+v", len(got), got)
	}
}

// (3) C.Use реализует интерфейс (контракт) -> SUPPRESS (guard2).
func TestISP_Golden3_ContractBound_Suppress(t *testing.T) {
	src := `package demo
type I interface { Foo(); Bar() }
type Handler interface { Use(p I) }
type C struct{}
func (c C) Use(p I) { p.Foo() }
var _ Handler = C{}
`
	if got := runISP(t, src); len(got) != 0 {
		t.Fatalf("контракт-связан -> suppress, ожидался 0, got %d: %+v", len(got), got)
	}
}

// (4) внешний io.ReadWriteCloser, узкое использование -> WARNING (external).
func TestISP_Golden4_ExternalNarrow_Warning(t *testing.T) {
	src := `package demo
import "io"
type C struct{}
func (c C) Use(p io.ReadWriteCloser) { p.Close() }
`
	vs := runISP(t, src)
	if own := ofKind(vs, KindISPUsageSubset); len(own) != 0 {
		t.Fatalf("внешний интерфейс не должен давать own-ERROR; got %+v", own)
	}
	if ext := ofKind(vs, KindISPExternalNarrow); len(ext) != 1 {
		t.Fatalf("ожидался 1 isp-external-narrow (WARNING), got %d: %+v", len(ext), ext)
	}
}

// analyzeMulti анализирует МНОГОПАКЕТНЫЙ корень один раз: files[relpath]=src.
func analyzeMulti(t *testing.T, files map[string]string) (*analyzer.GoAnalyzer, *model.Graph) {
	t.Helper()

	dir := t.TempDir()

	for rel, src := range files {
		full := filepath.Join(dir, rel)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatalf("MkdirAll: %v", err)
		}

		if err := os.WriteFile(full, []byte(src), 0o600); err != nil {
			t.Fatalf("WriteFile: %v", err)
		}
	}

	a := analyzer.NewGoAnalyzer()

	graph, err := a.Analyze(dir)
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	return a, graph
}

// (BLOCKER-фикс: детерминизм) ПАКЕТО-КОРРЕКТНОСТЬ + ДЕТЕРМИНИЗМ при ДУБЛЯХ имён интерфейсов
// между пакетами. Два пакета с интерфейсом I разной ширины; клиент в pkga использует
// СВОЙ I (2 метода) узко. Keyed lookup по пакету клиента обязан: (1) резолвить в pkga.I
// (Total=2), НЕ в pkgb.I; (2) давать БАЙТ-идентичный результат на ПОВТОРНЫХ прогонах
// метрики НА ОДНОМ графе (инвариант снапшота дельта-гейта). Перебор-по-имени
// тут давал случайный вердикт (Go map-iteration) -> спонтанный NEW в дельте. Анализ
// делаем ОДИН раз (qname несёт путь tmp-dir -> сравниваем метрику на фикс. графе).
func TestISP_GoldenDeterminism_DuplicateInterfaceNames(t *testing.T) {
	a, graph := analyzeMulti(t, map[string]string{
		"pkga/a.go": `package pkga
type I interface { Foo(); Bar() }
type C struct{}
func (c C) Use(p I) { p.Foo() }
`,
		"pkgb/b.go": `package pkgb
type I interface { Foo() }
`,
	})

	first := ComputeISPUsageSubset(graph, a)

	own := ofKind(first, KindISPUsageSubset)
	if len(own) != 1 {
		t.Fatalf("ожидался 1 own-кандидат (pkga.C.Use), got %d: %+v", len(own), own)
	}
	// Резолв В СВОЙ пакет: pkga.I имеет 2 метода -> "of 2 methods" (не pkgb.I с 1).
	if !strings.Contains(own[0].Message, "of 2 methods") {
		t.Fatalf("должен резолвиться в pkga.I (2 метода), а не pkgb.I (1); msg=%q", own[0].Message)
	}

	want, _ := json.Marshal(first)

	for i := 0; i < 20; i++ {
		got, _ := json.Marshal(ComputeISPUsageSubset(graph, a))
		if string(got) != string(want) {
			t.Fatalf("недетерминизм метрики на фикс. графе:\n want=%s\n got =%s", want, got)
		}
	}
}

// (5) использует ВСЕ методы интерфейса -> НЕ ISP.
func TestISP_Golden5_UsesAll_NotISP(t *testing.T) {
	src := `package demo
type I interface { Foo(); Bar() }
type C struct{}
func (c C) Use(p I) { p.Foo(); p.Bar() }
`
	if got := runISP(t, src); len(got) != 0 {
		t.Fatalf("использует все методы -> не ISP, ожидался 0, got %d: %+v", len(got), got)
	}
}
