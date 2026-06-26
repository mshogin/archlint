package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mshogin/archlint/internal/archlintcfg"
	"github.com/mshogin/archlint/internal/mcp"
)

// makeMonorepo строит синтетический monorepo: svc-a с structural-clone (T.CloneA/CloneB,
// 5 изоморфных вызовов >= cloneMinSize -> клон + smell), svc-b — чистый. Возвращает корень.
func makeMonorepo(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	write := func(rel, content string) {
		p := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	write("svc-a/go.mod", "module example.com/svca\ngo 1.21\n")
	write("svc-a/internal/a/a.go", `package a

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
`)

	write("svc-b/go.mod", "module example.com/svcb\ngo 1.21\n")
	write("svc-b/internal/b/b.go", "package b\n\nfunc B() int { return 1 }\n")

	return root
}

// monorepo сканируется ПОМОДУЛЬНО (не абстейн): enumerateModules даёт оба модуля, svc-a с клоном
// даёт нарушения, svc-b чистый = 0. Доказывает: per-module скан различает модули, оба обработаны.
func TestPerModule_ScansEachModule(t *testing.T) {
	root := makeMonorepo(t)
	cfg := archlintcfg.Default()

	mods := enumerateModules(root, nil)
	if len(mods) != 2 {
		t.Fatalf("monorepo -> 2 модуля, got %v", mods)
	}

	perMod := map[string]int{}
	for _, m := range mods {
		rel, _ := filepath.Rel(root, m)
		vs, err := collectGoModuleViolations(m, nil, &cfg, nil)
		if err != nil {
			t.Fatalf("collect %s: %v", rel, err)
		}
		perMod[rel] = len(vs)
	}

	if perMod["svc-a"] == 0 {
		t.Errorf("svc-a (с клоном) должен дать нарушения, got 0")
	}
	if perMod["svc-b"] != 0 {
		t.Errorf("svc-b (чистый) должен дать 0, got %d", perMod["svc-b"])
	}
}

// t_root-ИНВАРИАНТНОСТЬ per-module: target нарушений = module-relative qname ВНУТРИ модуля
// (например internal/a.T.CloneA), НЕ содержит абсолютный путь корня monorepo. Каждый модуль —
// свой canonical scanRoot. Без этого qname разных модулей перепутались бы по корню репо.
func TestPerModule_TargetModuleRelative(t *testing.T) {
	root := makeMonorepo(t)
	cfg := archlintcfg.Default()

	svcA := filepath.Join(root, "svc-a")
	vs, err := collectGoModuleViolations(svcA, nil, &cfg, nil)
	if err != nil {
		t.Fatal(err)
	}

	sawClone := false
	for _, v := range vs {
		// target НЕ должен содержать абсолютный путь модуля/корня (module-relative канонизация).
		if strings.Contains(v.Target, root) || filepath.IsAbs(v.Target) {
			t.Errorf("target %q не module-relative (содержит корень/абсолютный путь)", v.Target)
		}
		if v.Kind == "structural-clone" {
			sawClone = true
			// qname внутри модуля начинается с пакета (internal/a...), без префикса svc-a корня.
			if !strings.HasPrefix(v.Target, "internal/a") {
				t.Errorf("clone target %q должен быть module-relative (internal/a...)", v.Target)
			}
		}
	}
	if !sawClone {
		t.Errorf("ожидался structural-clone в svc-a, не найден; got %d нарушений", len(vs))
	}
}

// СТРАЖ per-module: delta(collect(X), collect(X)) = ∅ В КАЖДОМ модуле — два прогона одного модуля
// дают ИДЕНТИЧНЫЙ результат (детерминизм per-module, как канонический страж для single, но на
// уровне модуля monorepo). Если per-module резолв недетерминирован — этот тест краснеет.
func TestPerModule_DeltaSelfEmpty(t *testing.T) {
	root := makeMonorepo(t)
	cfg := archlintcfg.Default()

	for _, rel := range []string{"svc-a", "svc-b"} {
		dir := filepath.Join(root, rel)
		first, err := collectGoModuleViolations(dir, nil, &cfg, nil)
		if err != nil {
			t.Fatalf("collect#1 %s: %v", rel, err)
		}
		second, err := collectGoModuleViolations(dir, nil, &cfg, nil)
		if err != nil {
			t.Fatalf("collect#2 %s: %v", rel, err)
		}

		if len(first) != len(second) {
			t.Fatalf("[%s] delta(collect,collect) != ∅: размеры %d vs %d", rel, len(first), len(second))
		}
		for i := range first {
			if first[i].Kind != second[i].Kind || first[i].Target != second[i].Target {
				t.Errorf("[%s] delta != ∅ на #%d: (%s,%s) vs (%s,%s)",
					rel, i, first[i].Kind, first[i].Target, second[i].Kind, second[i].Target)
			}
		}
	}
}

// Агрегат worst: svc-a (нарушения) под threshold=0 -> !passed; svc-b -> passed; AND = !passed
// (exit worst). Под большим threshold оба passed. Доказывает gateViolations + AND-агрегат.
func TestPerModule_AggregateWorst(t *testing.T) {
	root := makeMonorepo(t)
	cfg := archlintcfg.Default()

	gate := func(rel string, threshold int) bool {
		vs, err := collectGoModuleViolations(filepath.Join(root, rel), nil, &cfg, nil)
		if err != nil {
			t.Fatal(err)
		}
		_, _, passed := gateViolations(vs, &cfg, nil, threshold)
		return passed
	}

	// threshold 0: svc-a проваливает (клон/smell = не-ERROR > 0), svc-b проходит.
	aPass0, bPass0 := gate("svc-a", 0), gate("svc-b", 0)
	if aPass0 {
		t.Errorf("svc-a под threshold=0 должен !passed")
	}
	if !bPass0 {
		t.Errorf("svc-b под threshold=0 должен passed (0 нарушений)")
	}
	if aPass0 && bPass0 {
		t.Errorf("AND-агрегат должен быть !passed (worst)")
	}

	// большой threshold: оба проходят (нет ERROR-blocking, count под порогом).
	if !gate("svc-a", 100000) || !gate("svc-b", 100000) {
		t.Errorf("под threshold=100000 оба модуля должны passed")
	}
}

// TestPerModule_ViolationsCarrySeverity — Применимость (multi-module): per-module нарушения
// ДОЛЖНЫ нести severity (объяснимость). До фикса collectGoModuleViolations возвращал
// collectFromGraph БЕЗ ApplySeverity -> modules[].details приходили с severity="" ->
// агент на monorepo не получал объяснимый вердикт.
func TestPerModule_ViolationsCarrySeverity(t *testing.T) {
	root := makeMonorepo(t)
	cfg := archlintcfg.Default()

	svcA := filepath.Join(root, "svc-a")
	vs, err := collectGoModuleViolations(svcA, nil, &cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(vs) == 0 {
		t.Fatal("svc-a (с клоном) должен дать нарушения")
	}
	withSeverity := 0
	for _, v := range vs {
		if v.Severity != "" {
			withSeverity++
		}
	}
	if withSeverity == 0 {
		t.Errorf("ни одно per-module нарушение не несёт severity (ApplySeverity не применён); %d нарушений", len(vs))
	}
}

// TestAggregateModuleDetails — multi-module delivery: top-level details СОБИРАЕТ modules[].details
// (каждое с .Module). Агент, читающий top-level `details`, на monorepo должен видеть ВСЕ
// нарушения (не пусто при violations>0) с указанием модуля.
func TestAggregateModuleDetails(t *testing.T) {
	results := []moduleScanResult{
		{Module: "svc-a", Details: []mcp.Violation{{Kind: "dead-code", Module: "svc-a"}}},
		{Module: "svc-b", Details: []mcp.Violation{{Kind: "god-class", Module: "svc-b"}, {Kind: "hub-node", Module: "svc-b"}}},
	}
	agg := aggregateModuleDetails(results)
	if len(agg) != 3 {
		t.Fatalf("агрегат = 3 нарушения (1+2), got %d", len(agg))
	}
	for _, v := range agg {
		if v.Module == "" {
			t.Errorf("нарушение %s без .Module в агрегате (агент не узнает модуль)", v.Kind)
		}
	}
}
