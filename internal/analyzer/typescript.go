// Package analyzer contains source code analyzers for building architecture graphs.
//
// TypeScript/React analyzer (MVP, regex-based).
// Full AST upgrade planned via esbuild; see issue #144 Part 2.
//
// Credit: @dklohgs for the original issue specification.
package analyzer

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mshogin/archlint/internal/model"
)

// TypeScriptAnalyzer analyzes TypeScript/JavaScript/React projects and builds
// dependency graphs using regex-based parsing (MVP approach).
type TypeScriptAnalyzer struct {
	packages map[string]*TSPackage
	nodes    []model.Node
	edges    []model.Edge
}

// TSPackage represents a folder-based package in a TS/JS project.
type TSPackage struct {
	// Dir is the directory path of this package.
	Dir string
	// Files lists all source files belonging to this package.
	Files []string
	// Components is the list of discovered component/class/function names.
	Components []string
	// Imports maps import source string to the importing file path.
	Imports []TSImport
	// HasReact is true when the package uses JSX or imports React.
	HasReact bool
}

// TSImport represents a single import statement.
type TSImport struct {
	// From is the import source path (e.g. "../services/api" or "react").
	From string
	// Kind is one of: "es6", "type", "dynamic", "require".
	Kind string
}

// NewTypeScriptAnalyzer creates a new TypeScript/React analyzer.
func NewTypeScriptAnalyzer() *TypeScriptAnalyzer {
	return &TypeScriptAnalyzer{
		packages: make(map[string]*TSPackage),
	}
}

// tsExtensions lists all file extensions handled by this analyzer.
var tsExtensions = map[string]bool{
	".ts":  true,
	".tsx": true,
	".js":  true,
	".jsx": true,
}

// Compiled regexes for component extraction.
var (
	// ES6 class: class Foo [ extends Bar ] {
	reTSClass = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:abstract\s+)?class\s+(\w+)`)
	// interface IFoo {
	reTSInterface = regexp.MustCompile(`(?m)^\s*(?:export\s+)?interface\s+(\w+)`)
	// type Foo = ...
	reTSTypeAlias = regexp.MustCompile(`(?m)^\s*(?:export\s+)?type\s+(\w+)\s*[=<]`)
	// function foo(
	reTSFunction = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:async\s+)?function\s+(\w+)\s*[(<]`)
	// const foo = ( ... ) => or const foo = async ( ... ) =>
	reTSArrow = regexp.MustCompile(`(?m)^\s*(?:export\s+)?(?:const|let|var)\s+(\w+)\s*=\s*(?:async\s*)?\(`)
	// React class component: class Foo extends React.Component or Component
	reTSReactClass = regexp.MustCompile(`(?m)class\s+(\w+)\s+extends\s+(?:React\.)?(?:Component|PureComponent)`)
	// Detects JSX return (crude but sufficient for MVP): return (< or return <
	reTSJSXReturn = regexp.MustCompile(`return\s*\(\s*<|return\s+<[A-Z/]`)

	// React hooks used as edges.
	// Allow optional generic type parameters between hook name and opening paren:
	// useState(  or  useState<T>(
	reTSHook = regexp.MustCompile(`\b(useState|useEffect|useContext|useReducer|useCallback|useMemo|useRef|useLayoutEffect|useImperativeHandle|useDebugValue)(?:<[^>]*>)?\s*\(`)

	// Decorator: @Something
	reTSDecorator = regexp.MustCompile(`(?m)^\s*@(\w+)`)
)

// Compiled regexes for import extraction.
var (
	// import X from 'Y'  /  import { X } from "Y"  /  import * as X from "Y"
	reTSImportES6 = regexp.MustCompile(`(?m)^\s*import\s+(?:type\s+)?(?:\w+|\{[^}]*\}|\*\s+as\s+\w+)\s+from\s+['"]([^'"]+)['"]`)
	// import type { X } from 'Y'
	reTSImportType = regexp.MustCompile(`(?m)^\s*import\s+type\s+(?:\w+|\{[^}]*\})\s+from\s+['"]([^'"]+)['"]`)
	// import('Y')
	reTSImportDynamic = regexp.MustCompile(`\bimport\s*\(\s*['"]([^'"]+)['"]\s*\)`)
	// require('Y')
	reTSRequire = regexp.MustCompile(`\brequire\s*\(\s*['"]([^'"]+)['"]\s*\)`)
)

// Analyze scans a TypeScript/JavaScript project directory and builds the graph.
func (ta *TypeScriptAnalyzer) Analyze(dir string) (*model.Graph, error) {
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			name := info.Name()
			if name == "node_modules" || name == ".git" || name == "dist" || name == "build" || name == ".next" {
				return filepath.SkipDir
			}
			return nil
		}
		if !tsExtensions[strings.ToLower(filepath.Ext(path))] {
			return nil
		}
		return ta.parseFile(path, dir)
	})
	if err != nil {
		return nil, fmt.Errorf("walk error: %w", err)
	}

	ta.buildGraph()

	return &model.Graph{
		Nodes: ta.nodes,
		Edges: ta.edges,
	}, nil
}

func (ta *TypeScriptAnalyzer) parseFile(path, rootDir string) error {
	// Package = directory relative to root.
	dir := filepath.Dir(path)
	relDir, err := filepath.Rel(rootDir, dir)
	if err != nil {
		relDir = dir
	}
	pkgKey := filepath.ToSlash(relDir)
	if pkgKey == "" || pkgKey == "." {
		pkgKey = "root"
	}

	pkg, ok := ta.packages[pkgKey]
	if !ok {
		pkg = &TSPackage{Dir: dir}
		ta.packages[pkgKey] = pkg
	}
	pkg.Files = append(pkg.Files, path)

	content, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	text := string(content)

	// Detect React presence.
	if strings.HasSuffix(strings.ToLower(filepath.Ext(path)), "x") || strings.Contains(text, "from 'react'") || strings.Contains(text, `from "react"`) {
		pkg.HasReact = true
	}

	// Extract components.
	ta.extractComponents(text, pkg, path)

	// Extract imports.
	ta.extractImports(text, pkg)

	return nil
}

func (ta *TypeScriptAnalyzer) extractComponents(text string, pkg *TSPackage, filePath string) {
	seen := make(map[string]bool)
	add := func(name string) {
		if !seen[name] {
			seen[name] = true
			pkg.Components = append(pkg.Components, name)
		}
	}

	// React class components (before generic class to give them priority).
	for _, m := range reTSReactClass.FindAllStringSubmatch(text, -1) {
		add(m[1])
	}

	// ES6 classes.
	for _, m := range reTSClass.FindAllStringSubmatch(text, -1) {
		add(m[1])
	}

	// Interfaces.
	for _, m := range reTSInterface.FindAllStringSubmatch(text, -1) {
		add(m[1])
	}

	// Type aliases.
	for _, m := range reTSTypeAlias.FindAllStringSubmatch(text, -1) {
		add(m[1])
	}

	// Named function declarations.
	for _, m := range reTSFunction.FindAllStringSubmatch(text, -1) {
		add(m[1])
	}

	// Arrow function components (const Foo = () => ...).
	// Check if the function looks like a React component (returns JSX).
	ext := strings.ToLower(filepath.Ext(filePath))
	isTSX := ext == ".tsx" || ext == ".jsx"
	for _, m := range reTSArrow.FindAllStringSubmatch(text, -1) {
		name := m[1]
		// Heuristic: uppercase first letter = component, or .tsx file.
		if isTSX || (len(name) > 0 && name[0] >= 'A' && name[0] <= 'Z') {
			add(name)
		}
	}

	// Decorators as annotations (not components per se, but useful metadata).
	_ = reTSDecorator

	// Detect JSX-returning functions and mark them.
	if reTSJSXReturn.MatchString(text) {
		pkg.HasReact = true
	}
}

func (ta *TypeScriptAnalyzer) extractImports(text string, pkg *TSPackage) {
	// Collect type imports first so they are tagged correctly.
	typeImports := make(map[string]bool)
	for _, m := range reTSImportType.FindAllStringSubmatch(text, -1) {
		typeImports[m[1]] = true
	}

	// ES6 imports (includes type imports — distinguish via typeImports set).
	for _, m := range reTSImportES6.FindAllStringSubmatch(text, -1) {
		src := m[1]
		kind := "es6"
		if typeImports[src] {
			kind = "type"
		}
		pkg.Imports = append(pkg.Imports, TSImport{From: src, Kind: kind})
	}

	// Dynamic imports.
	for _, m := range reTSImportDynamic.FindAllStringSubmatch(text, -1) {
		pkg.Imports = append(pkg.Imports, TSImport{From: m[1], Kind: "dynamic"})
	}

	// CommonJS require.
	for _, m := range reTSRequire.FindAllStringSubmatch(text, -1) {
		pkg.Imports = append(pkg.Imports, TSImport{From: m[1], Kind: "require"})
	}
}

// CountHooks returns the number of React hook call-sites found in source text.
// Hooks are modelled as edges: the file depends on the React hooks runtime.
func CountHooks(text string) int {
	return len(reTSHook.FindAllString(text, -1))
}

func (ta *TypeScriptAnalyzer) buildGraph() {
	// Index package IDs.
	pkgIDs := make(map[string]string) // dir -> node ID
	for pkgKey := range ta.packages {
		pkgIDs[pkgKey] = pkgKey
	}

	// Add package nodes.
	for pkgKey, pkg := range ta.packages {
		entity := "ts-package"
		if pkg.HasReact {
			entity = "react-package"
		}
		ta.nodes = append(ta.nodes, model.Node{
			ID:     pkgKey,
			Title:  pkgKey,
			Entity: entity,
		})

		// Add component nodes.
		for _, comp := range pkg.Components {
			compID := pkgKey + "." + comp
			ta.nodes = append(ta.nodes, model.Node{
				ID:     compID,
				Title:  comp,
				Entity: "component",
			})
			ta.edges = append(ta.edges, model.Edge{
				From: pkgKey,
				To:   compID,
				Type: "contains",
			})
		}
	}

	// Add import edges (package -> package or package -> external).
	for pkgKey, pkg := range ta.packages {
		for _, imp := range pkg.Imports {
			target := ta.resolveImport(pkgKey, imp.From)
			if target == "" {
				continue
			}
			ta.edges = append(ta.edges, model.Edge{
				From: pkgKey,
				To:   target,
				Type: imp.Kind,
			})
		}
	}

	// Hook usage edges: if a package uses hooks, add edge to "react-hooks".
	for pkgKey, pkg := range ta.packages {
		hookCount := 0
		for _, file := range pkg.Files {
			content, err := os.ReadFile(file)
			if err == nil {
				hookCount += CountHooks(string(content))
			}
		}
		if hookCount > 0 {
			ta.edges = append(ta.edges, model.Edge{
				From:   pkgKey,
				To:     "ext:react-hooks",
				Type:   "uses-hooks",
				Method: fmt.Sprintf("%d calls", hookCount),
			})
		}
	}
}

// resolveImport maps an import source string to a graph node ID.
// Relative imports (starting with . or ..) are resolved to a package key;
// bare specifiers are treated as external dependencies.
func (ta *TypeScriptAnalyzer) resolveImport(fromPkg, importSrc string) string {
	if importSrc == "" {
		return ""
	}

	// External / node_modules dependency.
	if !strings.HasPrefix(importSrc, ".") {
		// Use top-level package name (before first /).
		parts := strings.SplitN(importSrc, "/", 2)
		extID := "ext:" + parts[0]
		// Register as node if not present.
		for _, n := range ta.nodes {
			if n.ID == extID {
				return extID
			}
		}
		ta.nodes = append(ta.nodes, model.Node{
			ID:     extID,
			Title:  parts[0],
			Entity: "external_module",
		})
		return extID
	}

	// Relative import: resolve against the fromPkg directory.
	fromDir := fromPkg
	if fromPkg == "root" {
		fromDir = "."
	}
	resolved := filepath.ToSlash(filepath.Clean(filepath.Join(fromDir, importSrc)))
	if resolved == "." || resolved == "" {
		resolved = "root"
	}

	// Strip file extension if present.
	for ext := range tsExtensions {
		resolved = strings.TrimSuffix(resolved, ext)
	}

	// Check if it maps to a known package (directory).
	if _, ok := ta.packages[resolved]; ok {
		return resolved
	}

	// Check parent directory (e.g. import from ./Button -> components/Button -> components).
	parent := filepath.ToSlash(filepath.Dir(resolved))
	if parent == "." {
		parent = "root"
	}
	if _, ok := ta.packages[parent]; ok {
		return parent
	}

	// Unknown relative target – skip.
	return ""
}

// readFileLines is a helper used in tests and scanning for line-by-line analysis.
func readFileLines(path string) ([]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	var lines []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		lines = append(lines, sc.Text())
	}
	return lines, sc.Err()
}

// DetectTypeScriptProject returns true if dir contains a package.json or tsconfig.json,
// or any .ts/.tsx source files.
func DetectTypeScriptProject(dir string) bool {
	markers := []string{"package.json", "tsconfig.json"}
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(dir, m)); err == nil {
			return true
		}
	}

	// Fall back: look for any .ts/.tsx file in root.
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() {
			ext := strings.ToLower(filepath.Ext(e.Name()))
			if ext == ".ts" || ext == ".tsx" {
				return true
			}
		}
	}
	return false
}
