package analyzer

import (
	"path/filepath"
	"testing"

	"github.com/mshogin/archlint/internal/model"
)

// testdataRust returns the absolute path to testdata/rust.
func testdataRust() string {
	return filepath.Join("testdata", "rust")
}

// TestRustAnalyzer_Structs verifies struct extraction from Rust source files.
func TestRustAnalyzer_Structs(t *testing.T) {
	ra := NewRustAnalyzer()
	graph, err := ra.Analyze(testdataRust())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	if len(graph.Nodes) == 0 {
		t.Fatal("expected non-empty graph nodes")
	}

	// service.rs should produce UserService, InternalCache, PrivateHelper structs.
	wantStructs := []string{"UserService", "InternalCache", "PrivateHelper"}
	for _, want := range wantStructs {
		found := false
		for _, n := range graph.Nodes {
			if n.Title == want && n.Entity == "struct" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected struct node %q in graph; nodes: %v", want, rustNodesSummary(graph.Nodes))
		}
	}
}

// TestRustAnalyzer_Enums verifies enum extraction from Rust source files.
func TestRustAnalyzer_Enums(t *testing.T) {
	ra := NewRustAnalyzer()
	graph, err := ra.Analyze(testdataRust())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	wantEnums := []string{"Status", "Role", "PrivateState"}
	for _, want := range wantEnums {
		found := false
		for _, n := range graph.Nodes {
			if n.Title == want && n.Entity == "enum" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected enum node %q in graph; nodes: %v", want, rustNodesSummary(graph.Nodes))
		}
	}
}

// TestRustAnalyzer_Traits verifies trait extraction from Rust source files.
func TestRustAnalyzer_Traits(t *testing.T) {
	ra := NewRustAnalyzer()
	graph, err := ra.Analyze(testdataRust())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	wantTraits := []string{"Repository", "Notifier", "Displayable"}
	for _, want := range wantTraits {
		found := false
		for _, n := range graph.Nodes {
			if n.Title == want && n.Entity == "trait" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected trait node %q in graph; nodes: %v", want, rustNodesSummary(graph.Nodes))
		}
	}
}

// TestRustAnalyzer_ImplEdges verifies that impl Trait for Struct produces edges.
func TestRustAnalyzer_ImplEdges(t *testing.T) {
	ra := NewRustAnalyzer()
	graph, err := ra.Analyze(testdataRust())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	// service.rs: UserService implements Repository.
	// model.rs: User implements Displayable, Status implements fmt::Display.
	foundImpl := false
	for _, e := range graph.Edges {
		if e.Type == "implements" {
			foundImpl = true
			break
		}
	}
	if !foundImpl {
		t.Errorf("expected at least one 'implements' edge; edges: %v", rustEdgesSummary(graph.Edges))
	}
}

// TestRustAnalyzer_Modules verifies module node extraction.
func TestRustAnalyzer_Modules(t *testing.T) {
	ra := NewRustAnalyzer()
	graph, err := ra.Analyze(testdataRust())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	// Expect module nodes: main, service, model.
	wantModules := []string{"main", "service", "model"}
	for _, want := range wantModules {
		found := false
		for _, n := range graph.Nodes {
			if n.ID == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected module node %q; nodes: %v", want, rustNodesSummary(graph.Nodes))
		}
	}
}

// TestRustAnalyzer_Functions verifies function extraction.
func TestRustAnalyzer_Functions(t *testing.T) {
	ra := NewRustAnalyzer()
	_, err := ra.Analyze(testdataRust())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	// service module should have handle_request and internal_helper.
	svcMod := ra.modules["service"]
	if svcMod == nil {
		t.Fatal("expected 'service' module")
	}

	wantFns := []string{"handle_request", "internal_helper"}
	for _, want := range wantFns {
		if !containsStr(svcMod.Functions, want) {
			t.Errorf("expected function %q in service module; got: %v", want, svcMod.Functions)
		}
	}
}

// TestRustAnalyzer_UseStatements verifies use declaration extraction.
func TestRustAnalyzer_UseStatements(t *testing.T) {
	ra := NewRustAnalyzer()
	_, err := ra.Analyze(testdataRust())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	// main.rs has: use crate::service::UserService -> extracts "service".
	mainMod := ra.modules["main"]
	if mainMod == nil {
		t.Fatal("expected 'main' module")
	}

	if !containsStr(mainMod.Uses, "service") {
		t.Errorf("expected 'service' in main module uses; got: %v", mainMod.Uses)
	}
}

// TestRustAnalyzer_Visibility verifies public vs private visibility parsing.
func TestRustAnalyzer_Visibility(t *testing.T) {
	ra := NewRustAnalyzer()
	_, err := ra.Analyze(testdataRust())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	svcMod := ra.modules["service"]
	if svcMod == nil {
		t.Fatal("expected 'service' module")
	}

	for _, s := range svcMod.Structs {
		switch s.Name {
		case "UserService":
			if s.Visibility != "pub" {
				t.Errorf("UserService should be pub, got %q", s.Visibility)
			}
		case "InternalCache":
			if s.Visibility != "pub(crate)" {
				t.Errorf("InternalCache should be pub(crate), got %q", s.Visibility)
			}
		case "PrivateHelper":
			if s.Visibility != "private" {
				t.Errorf("PrivateHelper should be private, got %q", s.Visibility)
			}
		}
	}
}

// TestRustAnalyzer_CargoDependencies verifies Cargo.toml dependency parsing.
func TestRustAnalyzer_CargoDependencies(t *testing.T) {
	ra := NewRustAnalyzer()
	graph, err := ra.Analyze(testdataRust())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	// Cargo.toml has serde and tokio as dependencies.
	wantDeps := []string{"ext:serde", "ext:tokio"}
	for _, want := range wantDeps {
		found := false
		for _, n := range graph.Nodes {
			if n.ID == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected external crate node %q; nodes: %v", want, rustNodesSummary(graph.Nodes))
		}
	}
}

// TestRustAnalyzer_ContainsEdges verifies that modules contain their components.
func TestRustAnalyzer_ContainsEdges(t *testing.T) {
	ra := NewRustAnalyzer()
	graph, err := ra.Analyze(testdataRust())
	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	// model module should contain User, Config, Status, Role, Displayable.
	foundContains := false
	for _, e := range graph.Edges {
		if e.From == "model" && e.Type == "contains" {
			foundContains = true
			break
		}
	}
	if !foundContains {
		t.Errorf("expected 'contains' edges from model module; edges: %v", rustEdgesSummary(graph.Edges))
	}
}

// TestDetectRustProject verifies that DetectRustProject correctly identifies Rust projects.
func TestDetectRustProject(t *testing.T) {
	// testdata/rust has a Cargo.toml.
	if !DetectRustProject(testdataRust()) {
		t.Error("expected DetectRustProject to return true for testdata/rust")
	}

	// testdata/typescript has no Cargo.toml.
	if DetectRustProject(testdataTS()) {
		t.Error("expected DetectRustProject to return false for testdata/typescript")
	}
}

// TestRustAnalyzer_Regexes verifies the core regexes on inline snippets.
func TestRustAnalyzer_Regexes(t *testing.T) {
	type reMatch interface {
		FindStringSubmatch(string) []string
	}

	tests := []struct {
		name    string
		input   string
		re      reMatch
		wantCap string
	}{
		{"pub struct", "pub struct Foo {", reStructDecl, "Foo"},
		{"private struct", "struct Bar;", reStructDecl, "Bar"},
		{"pub enum", "pub enum Color { Red, Green }", reEnumDecl, "Color"},
		{"private enum", "enum State { Open }", reEnumDecl, "State"},
		{"pub trait", "pub trait Animal {", reTraitDecl, "Animal"},
		{"impl for", "impl Display for Foo {", reImplDecl, "Display"},
		{"pub fn", "pub fn do_something() {", reFnDecl, "do_something"},
		{"async fn", "async fn fetch() {", reFnDecl, "fetch"},
		{"pub async fn", "pub async fn handle() {", reFnDecl, "handle"},
		{"mod decl", "mod utils;", reModDecl, "utils"},
		{"pub mod", "pub mod api {", reModDecl, "api"},
		{"use crate", "use crate::service;", reUseDecl, "service"},
		{"use super", "use super::model;", reUseDecl, "model"},
		{"use std", "use std::io;", reUseDecl, "std"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := tt.re.FindStringSubmatch(tt.input)
			if m == nil {
				t.Fatalf("regex did not match %q", tt.input)
			}
			if m[1] != tt.wantCap {
				t.Errorf("got capture %q, want %q", m[1], tt.wantCap)
			}
		})
	}
}

// TestRustAnalyzer_ImplForRegex verifies impl Trait for Struct regex captures both names.
func TestRustAnalyzer_ImplForRegex(t *testing.T) {
	line := "impl Repository for UserService {"
	m := reImplDecl.FindStringSubmatch(line)
	if m == nil {
		t.Fatal("expected reImplDecl to match 'impl Repository for UserService'")
	}
	if m[1] != "Repository" {
		t.Errorf("expected trait name 'Repository', got %q", m[1])
	}
	if m[2] != "UserService" {
		t.Errorf("expected struct name 'UserService', got %q", m[2])
	}
}

// TestPathToModuleName verifies file path to Rust module name conversion.
func TestPathToModuleName(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"main.rs", "main"},
		{"lib.rs", "lib"},
		{"service.rs", "service"},
		{"auth/mod.rs", "auth"},
		{"auth/handler.rs", "auth::handler"},
		{"api/v1/routes.rs", "api::v1::routes"},
	}

	for _, tt := range tests {
		got := pathToModuleName(tt.input)
		if got != tt.want {
			t.Errorf("pathToModuleName(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func rustNodesSummary(nodes []model.Node) []string {
	out := make([]string, 0, len(nodes))
	for _, n := range nodes {
		out = append(out, n.ID+"("+n.Entity+")")
	}
	return out
}

func rustEdgesSummary(edges []model.Edge) []string {
	out := make([]string, 0, len(edges))
	for _, e := range edges {
		out = append(out, e.From+"->"+e.To+"["+e.Type+"]")
	}
	return out
}
