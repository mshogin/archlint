package analyzer

import (
	"os"
	"path/filepath"
	"testing"
)

func TestImportedTypesUseFileScopedBindings(t *testing.T) {
	root := t.TempDir()
	files := map[string]string{
		"go.mod":             "module example.com/fixture\n\ngo 1.26.1\n",
		"a/models/types.go":  "package models\ntype Filter struct{}\ntype Base interface { Run() }\n",
		"b/renamed/types.go": "package models\ntype Filter struct{}\n",
		"business/a.go": `package business
import "example.com/fixture/a/models"
type Holder struct { F *models.Filter; models.Base }
func Search(f *models.Filter) *models.Filter { return f }
type Service interface { Search(*models.Filter) *models.Filter }
`,
		"business/b.go": `package business
import "example.com/fixture/b/renamed"
func Other(f *models.Filter) {}
`,
		"business/c.go": `package business
import alias "example.com/fixture/a/models"
func Aliased(f *alias.Filter) {}
`,
		"business/d.go": `package business
import models "example.org/external/models"
func External(f *models.Filter) {}
func Unknown(f *Filter) {}
`,
	}
	for name, body := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	expected := map[string]string{
		"business.Search": "a/models.Filter", "business.Other": "b/renamed.Filter",
		"business.Aliased": "a/models.Filter", "business.Holder": "a/models.Filter",
		"business.Service": "a/models.Filter",
	}
	for run := 0; run < 10; run++ {
		graph, err := NewGoAnalyzer().Analyze(root)
		if err != nil {
			t.Fatal(err)
		}
		seen := map[string]bool{}
		embedded := false
		for _, e := range graph.Edges {
			if e.Type == "embeds" && e.From == "business.Holder" {
				if e.To != "a/models.Base" {
					t.Errorf("wrong embed: %+v", e)
				}
				embedded = true
			}
			if e.Type != "uses" && e.Type != "returns" {
				continue
			}
			if e.From == "business.External" || e.From == "business.Unknown" {
				t.Errorf("guessed type: %+v", e)
			}
			if want, ok := expected[e.From]; ok {
				if e.To != want {
					t.Errorf("%s: got %s, want %s", e.From, e.To, want)
				}
				seen[e.From] = true
			}
		}
		for from := range expected {
			if !seen[from] {
				t.Errorf("missing edge from %s", from)
			}
		}
		if !embedded {
			t.Error("missing embedded interface")
		}
	}
}

func TestExternalTestPackageBeforeProductionPackage(t *testing.T) {
	for _, tc := range []struct {
		name, dir, target string
	}{
		{name: "subdirectory", dir: "foo", target: "foo.Thing"},
		{name: "scan root", dir: "", target: "foo.Thing"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			importPath := "example.com/fixture"
			if tc.dir != "" {
				importPath += "/" + tc.dir
			}
			files := map[string]string{
				"go.mod":                           "module example.com/fixture\n\ngo 1.26.1\n",
				filepath.Join(tc.dir, "a_test.go"): "package foo_test\nimport foo \"" + importPath + "\"\ntype External struct { T foo.Thing }\n",
				filepath.Join(tc.dir, "foo.go"):    "package foo\ntype Thing struct{}\n",
				"bar/bar.go":                       "package bar\nimport \"" + importPath + "\"\ntype Holder struct { T foo.Thing }\nfunc Use(t *foo.Thing) *foo.Thing { return t }\n",
			}
			for name, body := range files {
				path := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0644); err != nil {
					t.Fatal(err)
				}
			}

			graph, err := NewGoAnalyzer().Analyze(root)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []struct{ from, kind string }{
				{from: "bar.Holder", kind: "uses"},
				{from: "bar.Use", kind: "uses"},
				{from: "bar.Use", kind: "returns"},
			} {
				found := false
				for _, edge := range graph.Edges {
					if edge.From == want.from && edge.To == tc.target && edge.Type == want.kind {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("missing %s edge from %s to %s", want.kind, want.from, tc.target)
				}
			}
		})
	}
}

func TestIgnoredGeneratorDoesNotNameImportedPackage(t *testing.T) {
	for _, tc := range []struct {
		name, dir string
	}{
		{name: "subdirectory", dir: "foo"},
		{name: "scan root"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			importPath := "example.com/fixture"
			if tc.dir != "" {
				importPath += "/" + tc.dir
			}
			files := map[string]string{
				"go.mod":                          "module example.com/fixture\n\ngo 1.26.1\n",
				filepath.Join(tc.dir, "a_gen.go"): "//go:build ignore\n\npackage main\nfunc main() {}\n",
				filepath.Join(tc.dir, "foo.go"):   "package foo\ntype Thing struct{}\n",
				"bar/bar.go": "package bar\nimport \"" + importPath + "\"\n" +
					"type Holder struct { T foo.Thing }\nfunc Use(t *foo.Thing) *foo.Thing { return t }\n",
				"bar/alias.go": "package bar\nimport alias \"" + importPath + "\"\n" +
					"func Aliased(t *alias.Thing) {}\n",
			}
			for name, body := range files {
				path := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0644); err != nil {
					t.Fatal(err)
				}
			}

			target := "foo.Thing"
			for run := 0; run < 20; run++ {
				graph, err := NewGoAnalyzer().Analyze(root)
				if err != nil {
					t.Fatal(err)
				}
				foundPackage := false
				for _, node := range graph.Nodes {
					if node.ID == "foo" && node.Entity == "package" {
						foundPackage = true
						if node.Title != "foo" {
							t.Errorf("run %d: package foo has title %q", run, node.Title)
						}
					}
				}
				if tc.dir != "" && !foundPackage {
					t.Errorf("run %d: missing package foo", run)
				}
				for _, want := range []struct{ from, kind string }{
					{from: "bar.Holder", kind: "uses"},
					{from: "bar.Use", kind: "uses"},
					{from: "bar.Use", kind: "returns"},
					{from: "bar.Aliased", kind: "uses"},
				} {
					found := false
					for _, edge := range graph.Edges {
						if edge.From == want.from && edge.To == target && edge.Type == want.kind {
							found = true
							break
						}
					}
					if !found {
						t.Errorf("run %d: missing %s edge from %s to %s", run, want.kind, want.from, target)
					}
				}
			}
		})
	}
}

// Imports must resolve against all parsed packages, independently of the
// GOOS, GOARCH, and build tags of the machine running archlint.
func TestImportBindingIndependentOfHostBuildContext(t *testing.T) {
	for _, tc := range []struct {
		name, file, body string
	}{
		{name: "GOOS file suffix", file: "foo_plan9.go", body: "package foo\ntype Thing struct{}\n"},
		{name: "GOARCH file suffix", file: "foo_wasm.go", body: "package foo\ntype Thing struct{}\n"},
		{name: "GOOS build tag", file: "foo.go", body: "//go:build plan9\n\npackage foo\ntype Thing struct{}\n"},
		{name: "custom build tag", file: "foo.go", body: "//go:build integration\n\npackage foo\ntype Thing struct{}\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			files := map[string]string{
				"go.mod":                      "module example.com/fixture\n\ngo 1.26.1\n",
				filepath.Join("foo", tc.file): tc.body,
				"bar/bar.go": "package bar\nimport \"example.com/fixture/foo\"\n" +
					"type Holder struct { T foo.Thing }\nfunc Use(t *foo.Thing) *foo.Thing { return t }\n",
				"bar/alias.go": "package bar\nimport alias \"example.com/fixture/foo\"\n" +
					"func Aliased(t *alias.Thing) {}\n",
			}
			for name, body := range files {
				path := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(body), 0644); err != nil {
					t.Fatal(err)
				}
			}

			graph, err := NewGoAnalyzer().Analyze(root)
			if err != nil {
				t.Fatal(err)
			}
			for _, want := range []struct{ from, kind string }{
				{from: "bar.Holder", kind: "uses"},
				{from: "bar.Use", kind: "uses"},
				{from: "bar.Use", kind: "returns"},
				{from: "bar.Aliased", kind: "uses"},
			} {
				found := false
				for _, edge := range graph.Edges {
					if edge.From == want.from && edge.To == "foo.Thing" && edge.Type == want.kind {
						found = true
						break
					}
				}
				if !found {
					t.Errorf("missing %s edge from %s to foo.Thing", want.kind, want.from)
				}
			}
		})
	}
}

func TestImportBindingWithMultiplePackageNames(t *testing.T) {
	root := t.TempDir()
	for name, body := range map[string]string{
		"go.mod":         "module example.com/fixture\n",
		"foo/a_plan9.go": "package foo\ntype Thing struct{}\n",
		"foo/b_linux.go": "package other\ntype Different struct{}\n",
		"bar/foo.go": "package bar\nimport \"example.com/fixture/foo\"\n" +
			"func Use(t foo.Thing) {}\n",
		"bar/other.go": "package bar\nimport \"example.com/fixture/foo\"\n" +
			"func Other(t other.Different) {}\n",
		"bar/alias.go": "package bar\nimport alias \"example.com/fixture/foo\"\n" +
			"func Ambiguous(t alias.Thing) {}\n",
	} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	graph, err := NewGoAnalyzer().Analyze(root)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"bar.Use": "foo.Thing", "bar.Other": "foo.Different"}
	seen := make(map[string]bool)
	for _, edge := range graph.Edges {
		if edge.Type != "uses" {
			continue
		}
		if edge.From == "bar.Ambiguous" {
			t.Errorf("ambiguous alias guessed target %s", edge.To)
		}
		if target, ok := want[edge.From]; ok {
			if edge.To != target {
				t.Errorf("%s points to %s, want %s", edge.From, edge.To, target)
			}
			seen[edge.From] = true
		}
	}
	for from := range want {
		if !seen[from] {
			t.Errorf("missing uses edge from %s", from)
		}
	}
}

func TestResolveFileTypesIsIdempotent(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/fixture\n"), 0644); err != nil {
		t.Fatal(err)
	}
	for name, body := range map[string]string{
		"foo/foo.go": "package foo\ntype Thing struct{}\ntype Base interface { Run() }\n",
		"bar/bar.go": "package bar\nimport \"example.com/fixture/foo\"\n" +
			"type Holder struct { foo.Base }\nfunc Use(t *foo.Thing) {}\n",
	} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	p := newGoParser(make(map[string]*PackageInfo), make(map[string]*TypeInfo),
		make(map[string]*FunctionInfo), make(map[string]*MethodInfo))
	p.scanRoot = root
	for _, name := range []string{"foo/foo.go", "bar/bar.go"} {
		if err := p.parseFile(filepath.Join(root, name)); err != nil {
			t.Fatal(err)
		}
	}
	p.resolveFileTypes()
	p.resolveFileTypes()
	if got := p.functions["bar.Use"].Params[0].TypePkg; got != "foo" {
		t.Errorf("repeated binding changed param package to %q", got)
	}
	if got := p.types["bar.Holder"].Embeds[0]; got != "foo.Base" {
		t.Errorf("repeated binding changed embed to %q", got)
	}
}

func TestPackageImportPathUsesNearestModule(t *testing.T) {
	root := t.TempDir()
	for _, tc := range []struct{ dir, module string }{
		{"", "example.com/root"}, {"nested", "example.com/child/v2"},
	} {
		dir := filepath.Join(root, tc.dir)
		if err := os.MkdirAll(filepath.Join(dir, "sub"), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "go.mod"), []byte("module \""+tc.module+"\"\n"), 0644); err != nil {
			t.Fatal(err)
		}
		if got := packageImportPath(dir); got != tc.module {
			t.Fatalf("got %q, want %q", got, tc.module)
		}
		if got := packageImportPath(filepath.Join(dir, "sub")); got != tc.module+"/sub" {
			t.Fatalf("subdirectory resolved as %q", got)
		}
	}
}
