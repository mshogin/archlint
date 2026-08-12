package mcp

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestChangedLines_NewSideNumbering(t *testing.T) {
	diff := `--- a/internal/foo.go
+++ b/internal/foo.go
@@ -10,4 +10,6 @@
 context one
 context two
+added A
+added B
 context three
`

	got := changedLines(diff, "/repo/internal/foo.go")

	// Ханк начинается со строки 10 новой стороны: 10, 11 - контекст,
	// 12, 13 - добавленные.
	for _, want := range []int{12, 13} {
		if !got[want] {
			t.Errorf("строка %d должна быть помечена изменённой, есть: %v", want, keys(got))
		}
	}

	for _, notWant := range []int{10, 11, 14} {
		if got[notWant] {
			t.Errorf("строка %d не менялась, но помечена: %v", notWant, keys(got))
		}
	}
}

// Дифф MR содержит много файлов - чужие ханки не должны попадать в свой файл.
func TestChangedLines_OtherFilesIgnored(t *testing.T) {
	diff := `--- a/internal/other.go
+++ b/internal/other.go
@@ -1,2 +1,3 @@
 package internal
+var Other = 1
--- a/internal/foo.go
+++ b/internal/foo.go
@@ -100,2 +100,3 @@
 ctx
+var Mine = 2
`

	got := changedLines(diff, "/repo/internal/foo.go")

	if !got[101] {
		t.Fatalf("своя строка 101 потеряна: %v", keys(got))
	}

	if got[2] {
		t.Fatalf("строка из чужого файла попала в результат: %v", keys(got))
	}
}

func TestChangedLines_EmptyDiff(t *testing.T) {
	if got := changedLines("", "/repo/a.go"); len(got) != 0 {
		t.Fatalf("пустой дифф должен давать пусто, получили %v", keys(got))
	}
}

func TestSameFile(t *testing.T) {
	cases := []struct {
		diffPath, absPath string
		want              bool
	}{
		{"b/internal/foo.go", "/repo/internal/foo.go", true},
		{"a/internal/foo.go", "/repo/internal/foo.go", true},
		{"internal/foo.go", "/repo/internal/foo.go", true},
		{"b/internal/bar.go", "/repo/internal/foo.go", false},
		// Суффикс должен совпадать по границе каталога, а не по символам.
		{"b/oo.go", "/repo/internal/foo.go", false},
		{"/dev/null", "/repo/internal/foo.go", false},
	}

	for _, c := range cases {
		if got := sameFile(c.diffPath, c.absPath); got != c.want {
			t.Errorf("sameFile(%q, %q) = %v, ожидали %v", c.diffPath, c.absPath, got, c.want)
		}
	}
}

func TestDeclsTouched_AttributesToEnclosingDecl(t *testing.T) {
	decls := []decl{
		{id: "pkg.First", line: 10},
		{id: "pkg.Second", line: 40},
		{id: "pkg.Third", line: 80},
	}

	// Строка 45 лежит внутри Second (следующее объявление начинается с 80).
	got := declsTouched(decls, map[int]bool{45: true})
	if len(got) != 1 || got[0] != "pkg.Second" {
		t.Fatalf("ожидали только pkg.Second, получили %v", got)
	}

	// Строка 5 выше первого объявления - это импорты/package-level, ничьё.
	if got := declsTouched(decls, map[int]bool{5: true}); len(got) != 0 {
		t.Fatalf("правка выше первого объявления не принадлежит узлу, получили %v", got)
	}

	got = declsTouched(decls, map[int]bool{12: true, 95: true})
	sort.Strings(got)

	if strings.Join(got, ",") != "pkg.First,pkg.Third" {
		t.Fatalf("ожидали First и Third, получили %v", got)
	}
}

// Сквозной: анализ реального файла с двумя функциями, дифф трогает только одну.
func TestAnalyzeChange_NarrowsToChangedFunction(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "main.go")
	src := `package main

func untouched() {
	println("a")
}

func changed() {
	println("b")
}

func main() {
	untouched()
	changed()
}
`

	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	server := createInitializedServer(t, tmpDir)

	// Строка 8 - тело changed().
	diff := `--- a/main.go
+++ b/main.go
@@ -7,3 +7,3 @@
 func changed() {
-	println("old")
+	println("b")
 }
`

	raw, err := json.Marshal(map[string]any{"path": path, "diff": diff})
	if err != nil {
		t.Fatal(err)
	}

	res, err := handleAnalyzeChange(server.state, raw)
	if err != nil {
		t.Fatalf("handleAnalyzeChange: %v", err)
	}

	if res.Scope != scopeDecls {
		t.Fatalf("scope %q, ожидали %q (дифф передан, сужение должно сработать)", res.Scope, scopeDecls)
	}

	joined := strings.Join(res.AffectedNodes, " ")
	if !strings.Contains(joined, "changed") {
		t.Fatalf("изменённая функция должна быть в affectedNodes: %v", res.AffectedNodes)
	}

	if strings.Contains(joined, "untouched") {
		t.Fatalf("нетронутая функция не должна считаться затронутой: %v", res.AffectedNodes)
	}
}

// Без диффа поведение прежнее: затронут весь файл. Это контракт для
// существующих потребителей.
func TestAnalyzeChange_WithoutDiffKeepsFileScope(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "main.go")

	if err := os.WriteFile(path, []byte("package main\n\nfunc a() {}\n\nfunc b() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	server := createInitializedServer(t, tmpDir)

	raw, err := json.Marshal(map[string]any{"path": path})
	if err != nil {
		t.Fatal(err)
	}

	res, err := handleAnalyzeChange(server.state, raw)
	if err != nil {
		t.Fatalf("handleAnalyzeChange: %v", err)
	}

	if res.Scope != scopeFile {
		t.Fatalf("scope %q, ожидали %q", res.Scope, scopeFile)
	}

	if len(res.AffectedNodes) < 2 {
		t.Fatalf("без диффа затронут весь файл, ожидали обе функции: %v", res.AffectedNodes)
	}
}

func keys(m map[int]bool) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}

	sort.Ints(out)

	return out
}
