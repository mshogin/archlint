package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestCollectWorkflow covers the end-to-end collect scenario:
// CLI collect -> analyzeCode -> GoAnalyzer.Analyze -> build graph -> saveGraph YAML
func TestCollectWorkflow(t *testing.T) {
	tmpDir := t.TempDir()
	outFile := filepath.Join(tmpDir, "arch.yaml")

	// Override CLI flags used by runCollect.
	origOut := collectOutputFile
	origLang := collectLanguage
	defer func() {
		collectOutputFile = origOut
		collectLanguage = origLang
	}()

	collectOutputFile = outFile
	collectLanguage = "go"

	// Capture stdout.
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	// Use the cli package itself as source - always a valid Go directory.
	runErr := runCollect(nil, []string{"."})

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)

	if runErr != nil {
		t.Fatalf("runCollect failed: %v", runErr)
	}

	// Verify YAML output file exists and is non-empty.
	info, err := os.Stat(outFile)
	if err != nil {
		t.Fatalf("output file missing: %v", err)
	}
	if info.Size() == 0 {
		t.Fatal("output file is empty")
	}

	// Parse and verify YAML structure.
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	var graph struct {
		Nodes []interface{} `yaml:"components"`
		Edges []interface{} `yaml:"links"`
	}
	if err := yaml.Unmarshal(data, &graph); err != nil {
		t.Fatalf("YAML parse failed: %v", err)
	}

	if len(graph.Nodes) == 0 {
		t.Error("expected at least one node in the graph")
	}

	output := buf.String()
	if !strings.Contains(output, "Найдено компонентов") {
		t.Errorf("expected stats in stdout, got: %s", output)
	}
}

// TestCollectUnsupportedLanguage verifies that collect returns an error for unknown languages.
func TestCollectUnsupportedLanguage(t *testing.T) {
	origLang := collectLanguage
	origOut := collectOutputFile
	defer func() {
		collectLanguage = origLang
		collectOutputFile = origOut
	}()

	collectLanguage = "rust"
	collectOutputFile = filepath.Join(t.TempDir(), "arch.yaml")

	err := runCollect(nil, []string{"."})
	if err == nil {
		t.Fatal("expected error for unsupported language, got nil")
	}
	if !strings.Contains(err.Error(), "неподдерживаемый язык") {
		t.Errorf("unexpected error message: %v", err)
	}
}

// TestValidateWorkflow covers the validate scenario:
// read YAML -> parse GraphExport -> exportToModelGraph -> print stats
func TestValidateWorkflow(t *testing.T) {
	// Build a minimal GraphExport YAML.
	graphYAML := `components:
  - id: pkg/foo
    title: foo
    entity: package
  - id: pkg/bar
    title: bar
    entity: package
links:
  - from: pkg/foo
    to: pkg/bar
    link_type: import
metadata:
  language: go
  root_dir: /tmp/test
  analyzed_at: "2026-01-01T00:00:00Z"
`

	tmpFile := filepath.Join(t.TempDir(), "graph.yaml")
	if err := os.WriteFile(tmpFile, []byte(graphYAML), 0o644); err != nil {
		t.Fatalf("write graph yaml: %v", err)
	}

	origFile := validateGraphFile
	origFmt := validateFormat
	defer func() {
		validateGraphFile = origFile
		validateFormat = origFmt
	}()

	validateGraphFile = tmpFile
	validateFormat = "text"

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	runErr := runValidate(nil, nil)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if runErr != nil {
		t.Fatalf("runValidate failed: %v", runErr)
	}

	checks := []string{"language:", "components:", "links:"}
	for _, s := range checks {
		if !strings.Contains(output, s) {
			t.Errorf("expected %q in output, got:\n%s", s, output)
		}
	}
}

// TestValidateJSONFormat verifies that validate --format json produces valid JSON.
func TestValidateJSONFormat(t *testing.T) {
	graphYAML := `components:
  - id: pkg/a
    title: a
    entity: package
links: []
metadata:
  language: go
  root_dir: /tmp
  analyzed_at: "2026-01-01T00:00:00Z"
`

	tmpFile := filepath.Join(t.TempDir(), "graph.yaml")
	if err := os.WriteFile(tmpFile, []byte(graphYAML), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	origFile := validateGraphFile
	origFmt := validateFormat
	defer func() {
		validateGraphFile = origFile
		validateFormat = origFmt
	}()

	validateGraphFile = tmpFile
	validateFormat = "json"

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	runErr := runValidate(nil, nil)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)

	if runErr != nil {
		t.Fatalf("runValidate json failed: %v", runErr)
	}

	var out interface{}
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("output is not valid JSON: %v\noutput: %s", err, buf.String())
	}
}

// TestCheckWorkflow covers the check scenario:
// CLI check -> GoAnalyzer.Analyze -> DetectAllViolations -> ComputeAllFileMetrics -> report
//
// Uses the sample testdata dir which has minimal code and no violations,
// so runCheck returns nil without calling os.Exit(1).
func TestCheckWorkflow(t *testing.T) {
	origFmt := checkFormat
	defer func() { checkFormat = origFmt }()

	checkFormat = "json"

	// Use the simple sample directory to avoid triggering os.Exit(1) on violations.
	sampleDir := filepath.Join("..", "..", "tests", "testdata", "sample")
	if _, err := os.Stat(sampleDir); os.IsNotExist(err) {
		t.Skip("testdata/sample not found, skipping check workflow test")
	}

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	runErr := runCheck(nil, []string{sampleDir})

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)

	if runErr != nil {
		t.Fatalf("runCheck failed: %v", runErr)
	}

	if buf.Len() > 0 {
		var result checkResult
		if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
			t.Fatalf("check JSON output invalid: %v\noutput: %s", err, buf.String())
		}
		// Total should match len(violations).
		if result.Total != len(result.Violations) {
			t.Errorf("total=%d != len(violations)=%d", result.Total, len(result.Violations))
		}
	}
}

// TestMetricsWorkflow covers the metrics scenario:
// CLI metrics -> GoAnalyzer.Analyze -> ComputeAllFileMetrics -> aggregate by package -> report
func TestMetricsWorkflow(t *testing.T) {
	origFmt := metricsFormat
	defer func() { metricsFormat = origFmt }()

	metricsFormat = "json"

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w

	runErr := runMetrics(nil, []string{"."})

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)

	if runErr != nil {
		t.Fatalf("runMetrics failed: %v", runErr)
	}

	var result metricsResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("metrics JSON invalid: %v\noutput: %s", err, buf.String())
	}

	if len(result.Packages) == 0 {
		t.Error("expected at least one package in metrics output")
	}

	// Every package must have a non-empty name.
	for _, pm := range result.Packages {
		if pm.Package == "" {
			t.Error("found package entry with empty name")
		}
		if pm.HealthScore < 0 || pm.HealthScore > 100 {
			t.Errorf("health score out of range [0,100]: %d (package: %s)", pm.HealthScore, pm.Package)
		}
	}
}
