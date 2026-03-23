package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mshogin/archlint/internal/analyzer"
)

func TestComputeFileMetricsBasic(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "main.go")

	if err := os.WriteFile(goFile, []byte(`package main

type Service struct {
	Name   string
	Config Config
}

type Config struct {
	Debug bool
}

func NewService() *Service {
	return &Service{}
}

func (s *Service) Run() error {
	return nil
}

func (s *Service) Stop() {}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	a := analyzer.NewGoAnalyzer()

	graph, err := a.Analyze(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	metrics := ComputeFileMetrics(goFile, a, graph)

	if metrics.Types != 2 {
		t.Errorf("expected 2 types, got %d", metrics.Types)
	}

	if metrics.Functions != 1 {
		t.Errorf("expected 1 function, got %d", metrics.Functions)
	}

	if metrics.Methods != 2 {
		t.Errorf("expected 2 methods, got %d", metrics.Methods)
	}

	if metrics.HealthScore <= 0 || metrics.HealthScore > 100 {
		t.Errorf("expected health score between 1 and 100, got %d", metrics.HealthScore)
	}
}

func TestComputeFileMetricsGodClass(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "god.go")

	// Create a type with >15 methods (god class threshold).
	code := `package main

type Monster struct {
	a, b, c, d, e, f, g, h, i, j string
	k, l, m, n, o, p, q, r, s, tt string
	u string
}

func (m *Monster) M1() {}
func (m *Monster) M2() {}
func (m *Monster) M3() {}
func (m *Monster) M4() {}
func (m *Monster) M5() {}
func (m *Monster) M6() {}
func (m *Monster) M7() {}
func (m *Monster) M8() {}
func (m *Monster) M9() {}
func (m *Monster) M10() {}
func (m *Monster) M11() {}
func (m *Monster) M12() {}
func (m *Monster) M13() {}
func (m *Monster) M14() {}
func (m *Monster) M15() {}
func (m *Monster) M16() {}

func main() {}
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

	if len(metrics.GodClasses) == 0 {
		t.Error("expected god class detection for Monster type")
	}

	if len(metrics.SRPViolations) == 0 {
		t.Error("expected SRP violations for Monster type with >7 methods and >10 fields")
	}

	if metrics.HealthScore >= 100 {
		t.Error("expected health score below 100 for file with god class")
	}
}

func TestComputeFileMetricsOrphanNodes(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "orphan.go")

	if err := os.WriteFile(goFile, []byte(`package main

type Unused struct{}

func main() {}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	a := analyzer.NewGoAnalyzer()

	graph, err := a.Analyze(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	metrics := ComputeFileMetrics(goFile, a, graph)

	// main() is connected via package contains, but after filtering containment,
	// the Unused struct and main function may show as orphans.
	if metrics.HealthScore <= 0 {
		t.Errorf("health score should be > 0, got %d", metrics.HealthScore)
	}
}

func TestComputeAllFileMetrics(t *testing.T) {
	tmpDir := t.TempDir()

	if err := os.WriteFile(filepath.Join(tmpDir, "a.go"), []byte(`package main

type Foo struct{}

func main() {}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(filepath.Join(tmpDir, "b.go"), []byte(`package main

type Bar struct{}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	a := analyzer.NewGoAnalyzer()

	graph, err := a.Analyze(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	allMetrics := ComputeAllFileMetrics(a, graph)

	if len(allMetrics) < 2 {
		t.Errorf("expected metrics for at least 2 files, got %d", len(allMetrics))
	}

	for path, m := range allMetrics {
		if m.HealthScore < 0 || m.HealthScore > 100 {
			t.Errorf("file %s has invalid health score: %d", path, m.HealthScore)
		}
	}
}

func TestHealthScoreComputation(t *testing.T) {
	// Test health score directly with known violation counts.
	m := &FileMetrics{
		SRPViolations: []Violation{{Kind: "srp"}},                        // -5
		GodClasses:    []string{"Monster"},                               // -10
		CyclicDeps:    []string{"a -> b -> a"},                           // -10
		HubNodes:      []string{"hub1"},                                  // -5
		ISPViolations: []Violation{{Kind: "isp"}},                        // -3
		DIPViolations: []Violation{{Kind: "dip"}},                        // -3
		FeatureEnvy:   []string{"envious"},                               // -2
		Instability:   0.9,                                               // -5
		MainSeqDistance: 0.6,                                             // -5
	}

	score := computeHealthScore(m)

	// 100 - 5 - 10 - 10 - 5 - 3 - 3 - 2 - 5 - 5 = 52
	expected := 52
	if score != expected {
		t.Errorf("expected health score %d, got %d", expected, score)
	}
}

func TestHealthScoreFloor(t *testing.T) {
	// Test that health score floors at 0.
	m := &FileMetrics{
		GodClasses: make([]string, 15), // -150 total, should floor at 0
	}

	score := computeHealthScore(m)

	if score != 0 {
		t.Errorf("expected health score 0 (floor), got %d", score)
	}
}

func TestIntToStr(t *testing.T) {
	tests := []struct {
		input    int
		expected string
	}{
		{0, "0"},
		{1, "1"},
		{42, "42"},
		{-5, "-5"},
		{100, "100"},
	}

	for _, tt := range tests {
		result := intToStr(tt.input)
		if result != tt.expected {
			t.Errorf("intToStr(%d) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}
