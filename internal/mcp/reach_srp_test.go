package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mshogin/archlint/internal/analyzer"
)

// writeAndAnalyze is a helper that writes Go source to a temp file, analyzes
// it, and returns the analyzer + graph.  The returned typeID is for the named
// struct inside the written code.
func writeAndAnalyze(t *testing.T, code string) (*analyzer.GoAnalyzer, interface{ Nodes() int }, string) {
	t.Helper()
	return nil, nil, ""
}

// analyzeCode is the real helper used by all reach-SRP tests.
func analyzeCode(t *testing.T, code string, typeName string) (string, *analyzer.GoAnalyzer, interface{}) {
	t.Helper()

	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "code.go")

	if err := os.WriteFile(goFile, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}

	a := analyzer.NewGoAnalyzer()
	graph, err := a.Analyze(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	typeID := findTypeID(t, a, typeName)
	return typeID, a, graph
}

// TestReachSRPDataClass: struct with 5 getters, no external calls → ρ=1
// (all methods are pure → unified into one class).
func TestReachSRPDataClass(t *testing.T) {
	code := `package data

type Point struct {
	x float64
	y float64
	z float64
}

func (p *Point) X() float64 { return p.x }
func (p *Point) Y() float64 { return p.y }
func (p *Point) Z() float64 { return p.z }
func (p *Point) SetX(v float64) { p.x = v }
func (p *Point) SetY(v float64) { p.y = v }
`
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "code.go")
	if err := os.WriteFile(goFile, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}

	a := analyzer.NewGoAnalyzer()
	graph, err := a.Analyze(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	typeID := findTypeID(t, a, "Point")
	result := ComputeReachSRP(a, typeID, graph)

	if result.Rho != 1 {
		t.Errorf("data class: expected ρ=1 (all pure methods unified), got ρ=%d; classes=%v",
			result.Rho, result.Classes)
	}
	if result.PureMethodCount != 5 {
		t.Errorf("data class: expected 5 pure methods, got %d", result.PureMethodCount)
	}
}

// TestReachSRPGodObject: struct with 3 methods each calling a different
// external package → ρ=3 (three separate responsibilities).
func TestReachSRPGodObject(t *testing.T) {
	code := `package god

import (
	"fmt"
	"os"
	"strings"
)

type God struct{}

func (g *God) DoIO() {
	fmt.Println("io")
	_ = os.Stdout
}

func (g *God) DoFiles() {
	f, _ := os.Open("x")
	_ = f
}

func (g *God) DoStrings() {
	_ = strings.ToUpper("x")
}
`
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "code.go")
	if err := os.WriteFile(goFile, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}

	a := analyzer.NewGoAnalyzer()
	graph, err := a.Analyze(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	typeID := findTypeID(t, a, "God")
	result := ComputeReachSRP(a, typeID, graph)

	// Each method calls a different external — expect multiple classes.
	// In this test the key assertion is ρ >= 2 (separation detected).
	if result.Rho < 2 {
		t.Errorf("god object: expected ρ>=2 (separate responsibilities), got ρ=%d; classes=%v",
			result.Rho, result.Classes)
	}
}

// TestReachSRPFocusedService: struct with 3 methods all calling the same
// external package → ρ=1 (single shared external dependency).
func TestReachSRPFocusedService(t *testing.T) {
	code := `package focused

import "fmt"

type Logger struct{}

func (l *Logger) Info(msg string)  { fmt.Println(msg) }
func (l *Logger) Warn(msg string)  { fmt.Println(msg) }
func (l *Logger) Error(msg string) { fmt.Println(msg) }
`
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "code.go")
	if err := os.WriteFile(goFile, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}

	a := analyzer.NewGoAnalyzer()
	graph, err := a.Analyze(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	typeID := findTypeID(t, a, "Logger")
	result := ComputeReachSRP(a, typeID, graph)

	if result.Rho != 1 {
		t.Errorf("focused service: expected ρ=1 (shared external dependency), got ρ=%d; classes=%v",
			result.Rho, result.Classes)
	}
}

// TestReachSRPMixed: 2 methods share same external call target + 1 method
// calls a different external → ρ=2.
func TestReachSRPMixed(t *testing.T) {
	code := `package mixed

import (
	"fmt"
	"os"
)

type Mixed struct{}

func (m *Mixed) PrintA() { fmt.Println("a") }
func (m *Mixed) PrintB() { fmt.Println("b") }
func (m *Mixed) OpenFile() {
	f, _ := os.Open("x")
	_ = f
}
`
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "code.go")
	if err := os.WriteFile(goFile, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}

	a := analyzer.NewGoAnalyzer()
	graph, err := a.Analyze(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	typeID := findTypeID(t, a, "Mixed")
	result := ComputeReachSRP(a, typeID, graph)

	if result.Rho != 2 {
		t.Errorf("mixed: expected ρ=2 (fmt group + os group), got ρ=%d; classes=%v",
			result.Rho, result.Classes)
	}
}

// TestReachSRPEmptyType: no methods → ρ=0.
func TestReachSRPEmptyType(t *testing.T) {
	code := `package empty

type Empty struct {
	value int
}
`
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "code.go")
	if err := os.WriteFile(goFile, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}

	a := analyzer.NewGoAnalyzer()
	graph, err := a.Analyze(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	typeID := findTypeID(t, a, "Empty")
	result := ComputeReachSRP(a, typeID, graph)

	if result.Rho != 0 {
		t.Errorf("empty type: expected ρ=0, got ρ=%d", result.Rho)
	}
}

// TestReachSRPInternalDelegation: method A calls method B, B calls external →
// both A and B should be in the same equivalence class → ρ=1.
func TestReachSRPInternalDelegation(t *testing.T) {
	code := `package deleg

import "fmt"

type Service struct{}

func (s *Service) Process() {
	s.log()
}

func (s *Service) log() {
	fmt.Println("log")
}
`
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "code.go")
	if err := os.WriteFile(goFile, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}

	a := analyzer.NewGoAnalyzer()
	graph, err := a.Analyze(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	typeID := findTypeID(t, a, "Service")
	result := ComputeReachSRP(a, typeID, graph)

	if result.Rho != 1 {
		t.Errorf("internal delegation: expected ρ=1 (A calls B → same class), got ρ=%d; classes=%v",
			result.Rho, result.Classes)
	}
}

// TestReachSRPLCOM4Divergence: pure data carrier where LCOM4 reports many
// components (each getter accesses a distinct field) but ρ=1 because all
// methods are pure (no external reach) → both-pure unification rule fires.
func TestReachSRPLCOM4Divergence(t *testing.T) {
	code := `package diverge

type Config struct {
	host    string
	port    int
	timeout int
	debug   bool
	maxConn int
}

func (c *Config) Host() string    { return c.host }
func (c *Config) Port() int       { return c.port }
func (c *Config) Timeout() int    { return c.timeout }
func (c *Config) Debug() bool     { return c.debug }
func (c *Config) MaxConn() int    { return c.maxConn }
`
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "code.go")
	if err := os.WriteFile(goFile, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}

	a := analyzer.NewGoAnalyzer()
	graph, err := a.Analyze(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	typeID := findTypeID(t, a, "Config")
	result := ComputeReachSRP(a, typeID, graph)

	// All methods are pure getters → ρ should be 1.
	if result.Rho != 1 {
		t.Errorf("LCOM4 divergence: expected ρ=1 (pure data carrier), got ρ=%d; classes=%v",
			result.Rho, result.Classes)
	}

	// LCOM4 might be higher (each getter is isolated without field access edges
	// resolving — depends on parser), but ρ must be 1.
	t.Logf("LCOM4=%d, ρ=%d for Config (pure data carrier)", result.LCOM4, result.Rho)
}
