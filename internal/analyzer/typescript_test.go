package analyzer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mshogin/archlint/internal/model"
)

// testdataTS returns the absolute path to testdata/typescript.
func testdataTS() string {
	return filepath.Join("testdata", "typescript")
}

// TestTypeScriptAnalyzer_ParseSampleTS verifies class, interface, and function
// extraction from a plain .ts file.
func TestTypeScriptAnalyzer_ParseSampleTS(t *testing.T) {
	ta := NewTypeScriptAnalyzer()
	graph, err := ta.Analyze(testdataTS())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	// Find the root package node (testdata/typescript contains flat files).
	rootPkg := findPackage(ta, "root")
	if rootPkg == nil {
		t.Fatalf("expected 'root' package, got packages: %v", packageKeys(ta))
	}

	wantComponents := []string{"UserService", "IUserService", "User", "UserInput", "parseUserInput"}
	for _, want := range wantComponents {
		if !containsStr(rootPkg.Components, want) {
			t.Errorf("expected component %q in root package, got: %v", want, rootPkg.Components)
		}
	}

	// Graph should have nodes.
	if len(graph.Nodes) == 0 {
		t.Error("expected non-empty graph nodes")
	}
}

// TestTypeScriptAnalyzer_ParseAppTSX verifies React component extraction from a .tsx file.
func TestTypeScriptAnalyzer_ParseAppTSX(t *testing.T) {
	ta := NewTypeScriptAnalyzer()
	_, err := ta.Analyze(testdataTS())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	rootPkg := findPackage(ta, "root")
	if rootPkg == nil {
		t.Fatalf("expected 'root' package")
	}

	if !rootPkg.HasReact {
		t.Error("expected HasReact=true for package containing .tsx file")
	}

	// React function components and class component.
	wantComponents := []string{"App", "UserCard", "ErrorBoundary"}
	for _, want := range wantComponents {
		if !containsStr(rootPkg.Components, want) {
			t.Errorf("expected component %q, got: %v", want, rootPkg.Components)
		}
	}
}

// TestTypeScriptAnalyzer_ES6Imports verifies that ES module imports are extracted.
func TestTypeScriptAnalyzer_ES6Imports(t *testing.T) {
	ta := NewTypeScriptAnalyzer()
	_, err := ta.Analyze(testdataTS())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	rootPkg := findPackage(ta, "root")
	if rootPkg == nil {
		t.Fatalf("expected 'root' package")
	}

	// App.tsx imports 'react' and './sample'.
	wantExternals := []string{"react"}
	for _, want := range wantExternals {
		found := false
		for _, imp := range rootPkg.Imports {
			if imp.From == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected import from %q, got imports: %v", want, importSrcs(rootPkg))
		}
	}
}

// TestTypeScriptAnalyzer_DynamicImports verifies dynamic import() extraction.
func TestTypeScriptAnalyzer_DynamicImports(t *testing.T) {
	ta := NewTypeScriptAnalyzer()
	_, err := ta.Analyze(testdataTS())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	rootPkg := findPackage(ta, "root")
	if rootPkg == nil {
		t.Fatalf("expected 'root' package")
	}

	// hooks.ts has a dynamic import and a require.
	var hasDynamic, hasRequire bool
	for _, imp := range rootPkg.Imports {
		if imp.Kind == "dynamic" {
			hasDynamic = true
		}
		if imp.Kind == "require" {
			hasRequire = true
		}
	}

	if !hasDynamic {
		t.Errorf("expected at least one dynamic import, imports: %v", importKinds(rootPkg))
	}
	if !hasRequire {
		t.Errorf("expected at least one require import, imports: %v", importKinds(rootPkg))
	}
}

// TestTypeScriptAnalyzer_HookCount verifies that React hook usages are counted.
func TestTypeScriptAnalyzer_HookCount(t *testing.T) {
	content, err := os.ReadFile(filepath.Join(testdataTS(), "hooks.ts"))
	if err != nil {
		t.Fatalf("read hooks.ts: %v", err)
	}

	count := CountHooks(string(content))
	// hooks.ts uses: useState(x3), useEffect, useCallback, useRef => at least 6
	if count < 6 {
		t.Errorf("expected at least 6 hook calls in hooks.ts, got %d", count)
	}
}

// TestTypeScriptAnalyzer_HookEdges verifies that hook edges appear in the graph.
func TestTypeScriptAnalyzer_HookEdges(t *testing.T) {
	ta := NewTypeScriptAnalyzer()
	graph, err := ta.Analyze(testdataTS())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	found := false
	for _, e := range graph.Edges {
		if e.To == "ext:react-hooks" && e.Type == "uses-hooks" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected edge to ext:react-hooks with type uses-hooks; edges: %v", edgeSummary(graph.Edges))
	}
}

// TestTypeScriptAnalyzer_TypeImport verifies that type-only imports are tagged correctly.
func TestTypeScriptAnalyzer_TypeImport(t *testing.T) {
	// sample.ts has: import type { Logger } from '../logger';
	ta := NewTypeScriptAnalyzer()
	_, err := ta.Analyze(testdataTS())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	rootPkg := findPackage(ta, "root")
	if rootPkg == nil {
		t.Fatalf("expected 'root' package")
	}

	for _, imp := range rootPkg.Imports {
		if strings.Contains(imp.From, "logger") && imp.Kind != "type" {
			t.Errorf("import from %q should be kind 'type', got %q", imp.From, imp.Kind)
		}
	}
}

// TestTypeScriptAnalyzer_Decorators verifies that decorator regex compiles and matches.
func TestTypeScriptAnalyzer_Decorators(t *testing.T) {
	src := `
@Injectable()
export class MyService {
    @Inject(TOKEN)
    private dep: Dep;
}
`
	matches := reTSDecorator.FindAllStringSubmatch(src, -1)
	if len(matches) < 2 {
		t.Errorf("expected at least 2 decorator matches, got %d: %v", len(matches), matches)
	}
	names := make([]string, 0, len(matches))
	for _, m := range matches {
		names = append(names, m[1])
	}
	if !containsStr(names, "Injectable") || !containsStr(names, "Inject") {
		t.Errorf("expected Injectable and Inject in decorator names, got %v", names)
	}
}

// TestTypeScriptAnalyzer_ReactClassComponent verifies React class component detection.
func TestTypeScriptAnalyzer_ReactClassComponent(t *testing.T) {
	src := `
import React from 'react';

export class Counter extends React.Component<{}, {count: number}> {
    state = { count: 0 };
    render() {
        return <div>{this.state.count}</div>;
    }
}
`
	matches := reTSReactClass.FindAllStringSubmatch(src, -1)
	if len(matches) == 0 {
		t.Fatal("expected to match React class component")
	}
	if matches[0][1] != "Counter" {
		t.Errorf("expected component name 'Counter', got %q", matches[0][1])
	}
}

// TestDetectTypeScriptProject verifies project detection logic.
func TestDetectTypeScriptProject(t *testing.T) {
	// testdata/typescript has .ts/.tsx files but no package.json.
	// DetectTypeScriptProject should still return true.
	detected := DetectTypeScriptProject(testdataTS())
	if !detected {
		t.Error("expected DetectTypeScriptProject to return true for testdata/typescript")
	}
}

// TestTypeScriptAnalyzer_ArrowFunction verifies arrow function extraction.
func TestTypeScriptAnalyzer_ArrowFunction(t *testing.T) {
	ta := NewTypeScriptAnalyzer()
	_, err := ta.Analyze(testdataTS())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	rootPkg := findPackage(ta, "root")
	if rootPkg == nil {
		t.Fatalf("expected 'root' package")
	}

	// App.tsx has: export const UserCard = (...) => (...)
	// UserCard starts with uppercase so it's captured from the .tsx file.
	// hooks.ts uses lowercase names (useToggle) which are not captured as components
	// because hooks are not visual components; only uppercase-first names in .tsx
	// files and all names in .tsx are eligible.
	wantArrows := []string{"UserCard"}
	for _, want := range wantArrows {
		if !containsStr(rootPkg.Components, want) {
			t.Errorf("expected arrow component %q, got: %v", want, rootPkg.Components)
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func findPackage(ta *TypeScriptAnalyzer, key string) *TSPackage {
	return ta.packages[key]
}

func packageKeys(ta *TypeScriptAnalyzer) []string {
	keys := make([]string, 0, len(ta.packages))
	for k := range ta.packages {
		keys = append(keys, k)
	}
	return keys
}

func containsStr(slice []string, s string) bool {
	for _, v := range slice {
		if v == s {
			return true
		}
	}
	return false
}

func importSrcs(pkg *TSPackage) []string {
	srcs := make([]string, 0, len(pkg.Imports))
	for _, imp := range pkg.Imports {
		srcs = append(srcs, imp.From)
	}
	return srcs
}

func importKinds(pkg *TSPackage) []string {
	kinds := make([]string, 0, len(pkg.Imports))
	for _, imp := range pkg.Imports {
		kinds = append(kinds, imp.Kind)
	}
	return kinds
}

// edgeSummary returns a compact string representation of edges for test error output.
func edgeSummary(edges []model.Edge) []string {
	out := make([]string, 0, len(edges))
	for _, e := range edges {
		out = append(out, e.From+"->"+e.To+"["+e.Type+"]")
	}
	return out
}
