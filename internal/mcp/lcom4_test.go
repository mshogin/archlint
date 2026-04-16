package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mshogin/archlint/internal/analyzer"
)

// TestLCOM4AllConnected tests that three methods all calling each other produce
// LCOM4 = 1 (single connected component, fully cohesive).
func TestLCOM4AllConnected(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "cohesive.go")

	// Three methods: A calls B, B calls C -> all in one component.
	code := `package cohesive

type Worker struct {
	count int
}

func (w *Worker) A() {
	w.B()
}

func (w *Worker) B() {
	w.C()
}

func (w *Worker) C() {
	_ = w.count
}
`
	if err := os.WriteFile(goFile, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}

	a := analyzer.NewGoAnalyzer()

	graph, err := a.Analyze(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Find the type ID for Worker.
	typeID := findTypeID(t, a, "Worker")

	result := ComputeLCOM4(a, typeID, graph)

	if result.LCOM != 1 {
		t.Errorf("expected LCOM4=1 (all methods connected via calls), got %d; components=%v",
			result.LCOM, result.Components)
	}

	if len(result.Methods) != 3 {
		t.Errorf("expected 3 methods, got %d", len(result.Methods))
	}
}

// TestLCOM4DisjointGroups tests that two groups of methods with no calls
// between groups produce LCOM4 = 2.
func TestLCOM4DisjointGroups(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "disjoint.go")

	// Group 1: A <-> B (A calls B).
	// Group 2: C <-> D (C calls D).
	// No calls between groups.
	code := `package disjoint

type Split struct{}

func (s *Split) A() {
	s.B()
}

func (s *Split) B() {}

func (s *Split) C() {
	s.D()
}

func (s *Split) D() {}
`
	if err := os.WriteFile(goFile, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}

	a := analyzer.NewGoAnalyzer()

	graph, err := a.Analyze(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	typeID := findTypeID(t, a, "Split")

	result := ComputeLCOM4(a, typeID, graph)

	if result.LCOM != 2 {
		t.Errorf("expected LCOM4=2 (two disjoint groups), got %d; components=%v",
			result.LCOM, result.Components)
	}

	if len(result.Methods) != 4 {
		t.Errorf("expected 4 methods, got %d", len(result.Methods))
	}
}

// TestLCOM4AllIsolated tests that N methods with zero calls between them
// produce LCOM4 = N (maximum violation).
func TestLCOM4AllIsolated(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "isolated.go")

	code := `package isolated

type Scattered struct{}

func (s *Scattered) X() {}
func (s *Scattered) Y() {}
func (s *Scattered) Z() {}
`
	if err := os.WriteFile(goFile, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}

	a := analyzer.NewGoAnalyzer()

	graph, err := a.Analyze(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	typeID := findTypeID(t, a, "Scattered")

	result := ComputeLCOM4(a, typeID, graph)

	// Each method is its own component -> LCOM4 = 3.
	if result.LCOM != 3 {
		t.Errorf("expected LCOM4=3 (every method isolated), got %d; components=%v",
			result.LCOM, result.Components)
	}
}

// TestLCOM4ConnectedViaCalls tests that a chain of method calls (A->B->C)
// yields a single connected component even though A and C do not call each other.
func TestLCOM4ConnectedViaCalls(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "chain.go")

	code := `package chain

type Chain struct{}

func (c *Chain) Entry() {
	c.Middle()
}

func (c *Chain) Middle() {
	c.Leaf()
}

func (c *Chain) Leaf() {}
`
	if err := os.WriteFile(goFile, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}

	a := analyzer.NewGoAnalyzer()

	graph, err := a.Analyze(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	typeID := findTypeID(t, a, "Chain")

	result := ComputeLCOM4(a, typeID, graph)

	if result.LCOM != 1 {
		t.Errorf("expected LCOM4=1 (chain of calls), got %d; components=%v",
			result.LCOM, result.Components)
	}
}

// TestLCOM4EmptyType tests that a struct with no methods returns LCOM4 = 0.
func TestLCOM4EmptyType(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "empty.go")

	code := `package empty

type Empty struct {
	value int
}
`
	if err := os.WriteFile(goFile, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}

	a := analyzer.NewGoAnalyzer()

	graph, err := a.Analyze(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	typeID := findTypeID(t, a, "Empty")

	result := ComputeLCOM4(a, typeID, graph)

	if result.LCOM != 0 {
		t.Errorf("expected LCOM4=0 for type with no methods, got %d", result.LCOM)
	}
}

// TestLCOM4UnknownType tests that a nil/unknown type returns a zero result.
func TestLCOM4UnknownType(t *testing.T) {
	a := analyzer.NewGoAnalyzer()

	tmpDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tmpDir, "stub.go"), []byte(`package stub`), 0o644); err != nil {
		t.Fatal(err)
	}

	graph, err := a.Analyze(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	result := ComputeLCOM4(a, "nonexistent.TypeXYZ", graph)

	if result.LCOM != 0 {
		t.Errorf("expected LCOM4=0 for unknown type, got %d", result.LCOM)
	}
}

// TestLCOM4IntegrationSRPViolation tests that LCOM4 >= 2 is reported as an
// SRP violation in the file metrics.
func TestLCOM4IntegrationSRPViolation(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "srp.go")

	// Two completely isolated method groups -> LCOM4=2 -> SRP violation.
	code := `package srp

type Hybrid struct{}

func (h *Hybrid) DoAuth() {
	h.ValidateToken()
}

func (h *Hybrid) ValidateToken() {}

func (h *Hybrid) SendEmail() {
	h.FormatMessage()
}

func (h *Hybrid) FormatMessage() {}
`
	if err := os.WriteFile(goFile, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}

	a := analyzer.NewGoAnalyzer()

	graph, err := a.Analyze(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	metrics := ComputeFileMetrics(goFile, a, graph)

	// Should have at least one srp-lack-of-cohesion violation.
	found := false

	for _, v := range metrics.SRPViolations {
		if v.Kind == "srp-lack-of-cohesion" {
			found = true

			break
		}
	}

	if !found {
		t.Errorf("expected srp-lack-of-cohesion violation for Hybrid type; violations=%v",
			metrics.SRPViolations)
	}

	if len(metrics.LCOMViolations) == 0 {
		t.Error("expected LCOMViolations to be populated")
	}
}

// findTypeID is a test helper that finds the type ID for the named type.
func findTypeID(t *testing.T, a *analyzer.GoAnalyzer, name string) string {
	t.Helper()

	for typeID, ti := range a.AllTypes() {
		if ti.Name == name {
			return typeID
		}
	}

	t.Fatalf("type %q not found in analyzer", name)

	return ""
}
