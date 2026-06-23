package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mshogin/archlint/internal/analyzer"
)

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// TestIsTestFile — граница prod/_test.go.
func TestIsTestFile(t *testing.T) {
	if !isTestFile("internal/mcp/foo_test.go") {
		t.Error("_test.go должен быть тестовым")
	}
	if isTestFile("internal/mcp/foo.go") {
		t.Error("foo.go — prod, не тест")
	}
}

// TestIsExportedSymbol — видимость по последнему сегменту qname.
func TestIsExportedSymbol(t *testing.T) {
	cases := map[string]bool{
		"pkg.Foo":            true,  // exported func
		"pkg.bar":            false, // unexported func
		"pkg/sub.Type.Do":    true,  // exported method
		"pkg/sub.Type.do":    false, // unexported method
		"github.com/x/y.Baz": true,
	}
	for q, want := range cases {
		if got := isExportedSymbol(q); got != want {
			t.Errorf("isExportedSymbol(%q)=%v want %v", q, got, want)
		}
	}
}

// TestTestOnlyProdSymbol_FailingCase — синтетический граф через РЕАЛЬНЫЙ analyzer
// на временном пакете: unexported prod-символ, юзаемый только из _test.go ->
// детектор ловит как ERROR+HumanInLoop; exported аналог -> WARNING.
func TestTestOnlyProdSymbol_FailingCase(t *testing.T) {
	dir := t.TempDir()
	// prod-файл: unexported helper (testHelper) + exported (ExportedHelper), оба
	// ВЫЗЫВАЮТСЯ ТОЛЬКО из _test.go (ноль prod-юзеров) -> quasi-dead в проде.
	prod := `package sample

func testHelper() int { return 42 }

func ExportedHelper() int { return 7 }
`
	test := `package sample

import "testing"

func TestA(t *testing.T) {
	if testHelper() != 42 || ExportedHelper() != 7 {
		t.Fail()
	}
}
`
	writeFile(t, dir, "sample.go", prod)
	writeFile(t, dir, "sample_test.go", test)

	a := analyzer.NewGoAnalyzer()
	g, err := a.Analyze(dir)
	if err != nil {
		t.Fatalf("analyze: %v", err)
	}

	vs := TestOnlyProdSymbol(g, a)
	ApplySeverity(vs)

	byTarget := map[string]Violation{}
	for _, v := range vs {
		if strings.HasSuffix(v.Target, ".testHelper") || strings.HasSuffix(v.Target, ".ExportedHelper") {
			byTarget[lastSeg(v.Target)] = v
		}
	}

	// unexported testHelper -> ERROR + HumanInLoop + RequiresDelta.
	if v, ok := byTarget["testHelper"]; !ok {
		t.Errorf("testHelper НЕ пойман детектором (vs=%+v)", vs)
	} else if v.Kind != kindTestOnlyProdSymbol || v.Severity != "ERROR" || !v.HumanInLoop || !v.RequiresDelta {
		t.Errorf("testHelper: %+v (ждали ERROR+HumanInLoop+RequiresDelta)", v)
	} else if !strings.Contains(v.Remediation, "человек") {
		t.Errorf("testHelper remediation без human-in-loop: %q", v.Remediation)
	}

	// exported ExportedHelper -> WARNING (open-world, не ERROR).
	if v, ok := byTarget["ExportedHelper"]; !ok {
		t.Errorf("ExportedHelper НЕ пойман")
	} else if v.Kind != kindTestOnlyProdSymbolExported || v.Severity != "WARNING" {
		t.Errorf("ExportedHelper: %+v (ждали exported-kind WARNING)", v)
	}
}

// TestTestOnlyProdSymbol_DoesNotAffectFingerprint — аддитивность (Anchor-основа).
func TestTestOnlyProdSymbol_DoesNotAffectFingerprint(t *testing.T) {
	v := Violation{Kind: kindTestOnlyProdSymbol, Target: "pkg.foo", Anchor: "test-only:pkg.foo"}
	before := Fingerprint(v)
	vs := []Violation{v}
	ApplySeverity(vs)
	if Fingerprint(vs[0]) != before {
		t.Errorf("Fingerprint изменился: %q -> %q", before, Fingerprint(vs[0]))
	}
}

// TestTestOnlyProdSymbol_SelfCrucible — ★SELF-CRUCIBLE (страж соундности): прогон
// на archlint-self. Контрпример-риск — легальный тест-хелпер намеренно в prod-
// пакете. Тест ЛОГИРУЕТ находки (для честной оценки 0-ложных vs демотация).
func TestTestOnlyProdSymbol_SelfCrucible(t *testing.T) {
	a := analyzer.NewGoAnalyzer()
	g, err := a.Analyze("../..")
	if err != nil {
		t.Skipf("self-analyze: %v", err)
	}
	vs := TestOnlyProdSymbol(g, a)
	for _, v := range vs {
		t.Logf("SELF-FIRE %s [%s]", v.Target, v.Kind)
	}
	t.Logf("SELF-CRUCIBLE: %d находок test-only-prod-symbol на archlint-self", len(vs))
}

func lastSeg(q string) string {
	if i := strings.LastIndexByte(q, '.'); i >= 0 {
		return q[i+1:]
	}
	return q
}
