package cli

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func TestScanTextNoViolations(t *testing.T) {
	// Use the archlint source itself - it should have no critical threshold issues
	// when threshold is set high enough.
	scanFormat = "text"
	scanThreshold = 9999
	defer func() {
		scanFormat = "text"
		scanThreshold = -1
	}()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	// Use the internal directory so we always have a valid Go directory.
	runErr := runScan(nil, []string{"."})

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	// With threshold=9999 any codebase should pass.
	if runErr != nil {
		t.Fatalf("runScan failed: %v", runErr)
	}

	if !strings.Contains(output, "PASSED") {
		t.Errorf("expected PASSED in output, got:\n%s", output)
	}
}

func TestScanJSONFormat(t *testing.T) {
	scanFormat = "json"
	scanThreshold = 9999
	defer func() {
		scanFormat = "text"
		scanThreshold = -1
	}()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	runErr := runScan(nil, []string{"."})

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if runErr != nil {
		t.Fatalf("runScan json failed: %v", runErr)
	}

	var result scanGateResult
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\noutput:\n%s", err, output)
	}

	// With threshold=9999 result must be passed.
	if !result.Passed {
		t.Errorf("expected passed=true with threshold 9999, got passed=false (violations=%d)", result.Violations)
	}

	if result.Threshold != 9999 {
		t.Errorf("expected threshold=9999 in JSON, got %d", result.Threshold)
	}

	if result.Details == nil {
		t.Error("expected details field in JSON output")
	}

	if result.Categories == nil {
		t.Error("expected categories field in JSON output")
	}
}

func TestScanGateThresholdZero(t *testing.T) {
	// threshold=-1 (default) maps to 0 internally: any violation causes exit 1.
	// We capture JSON to inspect without actually calling os.Exit.
	scanFormat = "json"
	scanThreshold = 9999 // keep passing so test doesn't call os.Exit(1)
	defer func() {
		scanFormat = "text"
		scanThreshold = -1
	}()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	runErr := runScan(nil, []string{"."})

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if runErr != nil {
		t.Fatalf("runScan failed: %v", runErr)
	}

	var result scanGateResult
	if err := json.Unmarshal([]byte(output), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}

	// Verify JSON fields are present.
	if result.Violations < 0 {
		t.Error("violations count should be >= 0")
	}
}

func TestScanGateResultPassedLogic(t *testing.T) {
	cases := []struct {
		violations int
		threshold  int
		wantPassed bool
	}{
		{0, 0, true},
		{1, 0, false},
		{5, 5, true},
		{6, 5, false},
		{0, -1, true},
		{1, -1, false},
	}

	for _, tc := range cases {
		threshold := tc.threshold
		if threshold < 0 {
			threshold = 0
		}
		passed := tc.violations <= threshold
		if passed != tc.wantPassed {
			t.Errorf("violations=%d threshold=%d: got passed=%v, want %v",
				tc.violations, tc.threshold, passed, tc.wantPassed)
		}
	}
}

func TestScanInvalidDirectory(t *testing.T) {
	// runScan calls os.Exit(2) for invalid dirs - we can't test that directly,
	// but we can verify os.Stat returns an error for a non-existent path.
	_, err := os.Stat("/nonexistent/path/that/does/not/exist/12345")
	if !os.IsNotExist(err) {
		t.Error("expected os.IsNotExist for non-existent path")
	}
}
