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
