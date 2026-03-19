package mcp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mshogin/archlint/internal/analyzer"
)

func TestDegradationDetectorBasic(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "main.go")

	// Initial clean code.
	if err := os.WriteFile(goFile, []byte(`package main

type Service struct {
	Name string
}

func main() {}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	a := analyzer.NewGoAnalyzer()

	graph, err := a.Analyze(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	detector := NewDegradationDetector()

	// Set baseline from initial metrics.
	baseline := ComputeFileMetrics(goFile, a, graph)
	detector.SetBaseline(goFile, baseline)

	// Check without change — should be stable.
	report := detector.CheckWithoutUpdate(goFile, a, graph)

	if report.Status != "stable" {
		t.Errorf("expected status 'stable', got %q", report.Status)
	}

	if report.Delta != 0 {
		t.Errorf("expected delta 0, got %d", report.Delta)
	}
}

func TestDegradationDetectorDegraded(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "main.go")

	// Initial clean code.
	if err := os.WriteFile(goFile, []byte(`package main

type Service struct {
	Name string
}

func main() {}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	a1 := analyzer.NewGoAnalyzer()

	graph1, err := a1.Analyze(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	detector := NewDegradationDetector()

	// Set baseline from initial clean metrics.
	baseline := ComputeFileMetrics(goFile, a1, graph1)
	detector.SetBaseline(goFile, baseline)

	initialHealth := baseline.HealthScore

	// Now write code with a god class (many methods and fields).
	code := `package main

type Service struct {
	a, b, c, d, e, f, g, h, i, j string
	k, l, m, n, o, p, q, r, s, tt string
	u string
}

func (s *Service) M1() {}
func (s *Service) M2() {}
func (s *Service) M3() {}
func (s *Service) M4() {}
func (s *Service) M5() {}
func (s *Service) M6() {}
func (s *Service) M7() {}
func (s *Service) M8() {}
func (s *Service) M9() {}
func (s *Service) M10() {}
func (s *Service) M11() {}
func (s *Service) M12() {}
func (s *Service) M13() {}
func (s *Service) M14() {}
func (s *Service) M15() {}
func (s *Service) M16() {}

func main() {}
`

	if err := os.WriteFile(goFile, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}

	a2 := analyzer.NewGoAnalyzer()

	graph2, err := a2.Analyze(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	// Check degradation.
	report := detector.Check(goFile, a2, graph2)

	if report.HealthBefore != initialHealth {
		t.Errorf("expected healthBefore=%d, got %d", initialHealth, report.HealthBefore)
	}

	if report.HealthAfter >= report.HealthBefore {
		t.Errorf("expected health to decrease, before=%d after=%d", report.HealthBefore, report.HealthAfter)
	}

	if report.Delta >= 0 {
		t.Errorf("expected negative delta, got %d", report.Delta)
	}

	if report.Status != "degraded" && report.Status != "critical" {
		t.Errorf("expected 'degraded' or 'critical' status, got %q", report.Status)
	}

	if len(report.NewViolations) == 0 {
		t.Error("expected new violations in degradation report")
	}
}

func TestDegradationDetectorImproved(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "main.go")

	// Initial code with a god class.
	code := `package main

type Service struct {
	a, b, c, d, e, f, g, h, i, j string
	k, l, m, n, o, p, q, r, s, tt string
	u string
}

func (s *Service) M1() {}
func (s *Service) M2() {}
func (s *Service) M3() {}
func (s *Service) M4() {}
func (s *Service) M5() {}
func (s *Service) M6() {}
func (s *Service) M7() {}
func (s *Service) M8() {}
func (s *Service) M9() {}
func (s *Service) M10() {}
func (s *Service) M11() {}
func (s *Service) M12() {}
func (s *Service) M13() {}
func (s *Service) M14() {}
func (s *Service) M15() {}
func (s *Service) M16() {}

func main() {}
`

	if err := os.WriteFile(goFile, []byte(code), 0o644); err != nil {
		t.Fatal(err)
	}

	a1 := analyzer.NewGoAnalyzer()

	graph1, err := a1.Analyze(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	detector := NewDegradationDetector()

	// Set baseline with bad code.
	baseline := ComputeFileMetrics(goFile, a1, graph1)
	detector.SetBaseline(goFile, baseline)

	// Now write clean code.
	if err := os.WriteFile(goFile, []byte(`package main

type Service struct {
	Name string
}

func main() {}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	a2 := analyzer.NewGoAnalyzer()

	graph2, err := a2.Analyze(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	report := detector.Check(goFile, a2, graph2)

	if report.Delta <= 0 {
		t.Errorf("expected positive delta (improvement), got %d", report.Delta)
	}

	if report.Status != "improved" {
		t.Errorf("expected 'improved' status, got %q", report.Status)
	}

	if len(report.FixedViolations) == 0 {
		t.Error("expected fixed violations in improvement report")
	}
}

func TestDegradationDetectorNoBaseline(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "main.go")

	if err := os.WriteFile(goFile, []byte(`package main

func main() {}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	a := analyzer.NewGoAnalyzer()

	graph, err := a.Analyze(tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	detector := NewDegradationDetector()

	// Check without any baseline — should be stable.
	report := detector.CheckWithoutUpdate(goFile, a, graph)

	if report.Status != "stable" {
		t.Errorf("expected 'stable' for no-baseline check, got %q", report.Status)
	}
}

func TestClassifyDelta(t *testing.T) {
	tests := []struct {
		delta    int
		expected string
	}{
		{10, "improved"},
		{6, "improved"},
		{5, "stable"},
		{0, "stable"},
		{-5, "stable"},
		{-6, "degraded"},
		{-15, "degraded"},
		{-16, "critical"},
		{-50, "critical"},
	}

	for _, tt := range tests {
		result := classifyDelta(tt.delta)
		if result != tt.expected {
			t.Errorf("classifyDelta(%d) = %q, want %q", tt.delta, result, tt.expected)
		}
	}
}

func TestSetBaselines(t *testing.T) {
	detector := NewDegradationDetector()

	metrics := map[string]*FileMetrics{
		"/a.go": {FilePath: "/a.go", HealthScore: 90},
		"/b.go": {FilePath: "/b.go", HealthScore: 80},
	}

	detector.SetBaselines(metrics)

	a := detector.GetBaseline("/a.go")
	if a == nil || a.HealthScore != 90 {
		t.Error("expected baseline for /a.go with health 90")
	}

	b := detector.GetBaseline("/b.go")
	if b == nil || b.HealthScore != 80 {
		t.Error("expected baseline for /b.go with health 80")
	}

	c := detector.GetBaseline("/c.go")
	if c != nil {
		t.Error("expected nil baseline for unknown file")
	}
}
