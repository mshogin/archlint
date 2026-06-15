package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// nested go-module АБСТЕЙН (соундность, защита от ложно-зелёного: молча 0 на monorepo).
// single -> nil (полный скан); >1 go.mod / go.work (не фикстуры) -> detected (скан неполон).
func TestDetectMultiModule(t *testing.T) {
	mkmod := func(dir, rel string) {
		t.Helper()
		p := filepath.Join(dir, rel)
		_ = os.MkdirAll(filepath.Dir(p), 0o755)
		if err := os.WriteFile(p, []byte("module x\ngo 1.21\n"), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	// (1) single go.mod -> nil (полный скан, self-gate цел).
	single := t.TempDir()
	mkmod(single, "go.mod")
	if mm := detectMultiModule(single, nil); mm != nil {
		t.Errorf("single go.mod не должен быть multi-module, got %+v", mm)
	}

	// (2) nested 3 go.mod (реальные подмодули svc-a/svc-b) -> detected (как agents-platform monorepo).
	nested := t.TempDir()
	mkmod(nested, "go.mod")
	mkmod(nested, "svc-a/go.mod")
	mkmod(nested, "svc-b/go.mod")

	mm := detectMultiModule(nested, nil)
	if mm == nil || !mm.Detected || mm.GoModCount != 3 {
		t.Errorf("nested 3 go.mod должен detect (count=3), got %+v", mm)
	}

	// (3) ФИКСТУРЫ (testdata Go-конвенция / demo / examples) -> nil (НЕ ложный абстейн на self).
	fix := t.TempDir()
	mkmod(fix, "go.mod")
	mkmod(fix, "testdata/sample/go.mod")
	mkmod(fix, "demo/go.mod")
	mkmod(fix, "examples/go.mod")

	if mm := detectMultiModule(fix, nil); mm != nil {
		t.Errorf("testdata/demo/examples go.mod = фикстуры, не multi-module, got %+v", mm)
	}

	// (4) go.work -> detected (даже при 1 go.mod).
	work := t.TempDir()
	mkmod(work, "go.mod")
	if err := os.WriteFile(filepath.Join(work, "go.work"), []byte("go 1.21\nuse .\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	mm2 := detectMultiModule(work, nil)
	if mm2 == nil || !mm2.HasGoWork {
		t.Errorf("go.work должен detect, got %+v", mm2)
	}

	// (5) excludes-директория с go.mod -> не считается.
	excl := t.TempDir()
	mkmod(excl, "go.mod")
	mkmod(excl, "thirdparty/go.mod")

	if mm := detectMultiModule(excl, []string{"thirdparty"}); mm != nil {
		t.Errorf("go.mod в excluded-директории не должен считаться, got %+v", mm)
	}
}

// enumerateModules перечисляет каталоги боевых go.mod (тот же skip-критерий, что detectMultiModule).
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
