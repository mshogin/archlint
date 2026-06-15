package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// enumerateModules перечисляет каталоги боевых go.mod (skip-критерий isSkippedModuleDir).
// Симметрия: что НЕ считается multi-module, то и НЕ попадает в per-module перечень.
func TestEnumerateModules(t *testing.T) {
	mkmod := func(dir, rel string) {
		t.Helper()
		p := filepath.Join(dir, rel)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte("module x\ngo 1.21\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// (1) single go.mod -> [корень] (ровно один модуль, как сейчас).
	single := t.TempDir()
	mkmod(single, "go.mod")

	got := enumerateModules(single, nil)
	if len(got) != 1 || got[0] != filepath.Clean(single) {
		t.Errorf("single -> [%q], got %v", single, got)
	}

	// (2) nested 3 модуля -> все 3 каталога, детерминированный порядок (sort).
	nested := t.TempDir()
	mkmod(nested, "go.mod")
	mkmod(nested, "svc-a/go.mod")
	mkmod(nested, "svc-b/go.mod")

	got = enumerateModules(nested, nil)
	want := []string{
		filepath.Clean(nested),
		filepath.Join(nested, "svc-a"),
		filepath.Join(nested, "svc-b"),
	}
	if len(got) != 3 {
		t.Fatalf("nested -> 3 модуля, got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("модуль[%d]: want %q, got %q (порядок должен быть sort-детерминирован)", i, want[i], got[i])
		}
	}

	// (3) фикстуры (testdata/demo/examples) -> только корень (не сканируем фикстурные модули).
	fix := t.TempDir()
	mkmod(fix, "go.mod")
	mkmod(fix, "testdata/sample/go.mod")
	mkmod(fix, "demo/go.mod")
	mkmod(fix, "examples/go.mod")

	got = enumerateModules(fix, nil)
	if len(got) != 1 || got[0] != filepath.Clean(fix) {
		t.Errorf("фикстуры не сканируются per-module: want [%q], got %v", fix, got)
	}

	// (4) excludes -> исключённый модуль не в перечне.
	excl := t.TempDir()
	mkmod(excl, "go.mod")
	mkmod(excl, "thirdparty/go.mod")

	got = enumerateModules(excl, []string{"thirdparty"})
	if len(got) != 1 || got[0] != filepath.Clean(excl) {
		t.Errorf("excluded модуль не сканируется: want [%q], got %v", excl, got)
	}

	// (5) нет go.mod вовсе -> [] (caller фолбэчит на dir as-is).
	empty := t.TempDir()
	if got := enumerateModules(empty, nil); len(got) != 0 {
		t.Errorf("нет go.mod -> пустой перечень, got %v", got)
	}
}
