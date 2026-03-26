// Package archtest provides architecture test assertions for Go projects.
// It shells out to the archlint binary to analyze code and exposes
// testing-friendly assertions.
//
// Usage in _test.go files:
//
//	func TestArchitecture(t *testing.T) {
//	    arch := archtest.Scan(".")
//	    arch.AssertNoCircularDependencies(t)
//	    arch.AssertMaxFanOut(t, 5)
//	    arch.AssertHealthScore(t, 70)
//	}
//
// The archlint binary is located by checking (in order):
//  1. ARCHLINT_BIN environment variable
//  2. PATH lookup for "archlint"
package archtest

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"testing"
)

// ArchResult holds the aggregated architecture analysis results for a directory.
type ArchResult struct {
	// Components is the total number of packages analyzed.
	Components int
	// Violations is the total number of architecture violations found.
	Violations int
	// Cycles is the number of circular dependency violations.
	Cycles int
	// HealthScore is the average health score across all packages (0-100).
	HealthScore int
	// FanOutMax is the maximum fan-out value across all packages.
	FanOutMax int

	// scanErr holds any error that occurred during scanning so that
	// assertions can report it as a test failure rather than panicking.
	scanErr error
}

// checkOutput is the JSON structure returned by `archlint check --format json`.
type checkOutput struct {
	Violations []checkViolation `json:"violations"`
	Total      int              `json:"total"`
}

type checkViolation struct {
	Kind    string `json:"kind"`
	Message string `json:"message"`
	Target  string `json:"target"`
}

// metricsOutput is the JSON structure returned by `archlint metrics --format json`.
type metricsOutput struct {
	Packages []packageMetrics `json:"packages"`
}

type packageMetrics struct {
	Package     string `json:"package"`
	FanOut      int    `json:"fanOut"`
	HealthScore int    `json:"healthScore"`
}

// archlintBin returns the path to the archlint binary.
// It respects the ARCHLINT_BIN environment variable, falling back to PATH.
func archlintBin() (string, error) {
	if bin := os.Getenv("ARCHLINT_BIN"); bin != "" {
		return bin, nil
	}

	path, err := exec.LookPath("archlint")
	if err != nil {
		return "", fmt.Errorf("archlint binary not found in PATH; set ARCHLINT_BIN to override: %w", err)
	}

	return path, nil
}

// Scan runs archlint on the given directory and returns an ArchResult.
// If the binary cannot be found or execution fails, the error is recorded
// internally and surfaced when any assertion is called.
func Scan(dir string) *ArchResult {
	result := &ArchResult{}

	bin, err := archlintBin()
	if err != nil {
		result.scanErr = err
		return result
	}

	// --- violations (cycles, coupling, SOLID) ---
	checkData, err := runCommand(bin, "check", dir, "--format", "json")
	if err != nil {
		// `archlint check` exits with status 1 when violations are found;
		// that is expected.  We only treat it as an error when no output
		// was produced at all.
		if len(checkData) == 0 {
			result.scanErr = fmt.Errorf("archlint check failed: %w", err)
			return result
		}
	}

	var checkOut checkOutput
	if err := json.Unmarshal(checkData, &checkOut); err != nil {
		result.scanErr = fmt.Errorf("failed to parse archlint check output: %w", err)
		return result
	}

	result.Violations = checkOut.Total

	for _, v := range checkOut.Violations {
		if v.Kind == "cycle" || v.Kind == "circular-dependency" {
			result.Cycles++
		}
	}

	// --- metrics (health score, fan-out) ---
	metricsData, err := runCommand(bin, "metrics", dir, "--format", "json")
	if err != nil {
		result.scanErr = fmt.Errorf("archlint metrics failed: %w", err)
		return result
	}

	var metricsOut metricsOutput
	if err := json.Unmarshal(metricsData, &metricsOut); err != nil {
		result.scanErr = fmt.Errorf("failed to parse archlint metrics output: %w", err)
		return result
	}

	result.Components = len(metricsOut.Packages)

	totalHealth := 0

	for _, pkg := range metricsOut.Packages {
		totalHealth += pkg.HealthScore
		if pkg.FanOut > result.FanOutMax {
			result.FanOutMax = pkg.FanOut
		}
	}

	if result.Components > 0 {
		result.HealthScore = totalHealth / result.Components
	}

	return result
}

// runCommand executes the archlint binary with the given arguments and returns
// its combined stdout output.  Stderr is suppressed because archlint writes
// progress information there that would interfere with JSON parsing.
func runCommand(bin string, args ...string) ([]byte, error) {
	cmd := exec.Command(bin, args...) //nolint:gosec
	out, err := cmd.Output()

	return out, err
}

// AssertNoCircularDependencies fails the test if any circular dependencies
// were detected during the scan.
func (a *ArchResult) AssertNoCircularDependencies(t *testing.T) {
	t.Helper()

	if a.scanErr != nil {
		t.Fatalf("archtest scan error: %v", a.scanErr)
	}

	if a.Cycles > 0 {
		t.Errorf("found %d circular dependencies, expected 0", a.Cycles)
	}
}

// AssertMaxFanOut fails the test if the maximum fan-out across all packages
// exceeds the given limit.
func (a *ArchResult) AssertMaxFanOut(t *testing.T, max int) {
	t.Helper()

	if a.scanErr != nil {
		t.Fatalf("archtest scan error: %v", a.scanErr)
	}

	if a.FanOutMax > max {
		t.Errorf("max fan-out %d exceeds limit %d", a.FanOutMax, max)
	}
}

// AssertMaxViolations fails the test if the total number of violations
// exceeds the given maximum.
func (a *ArchResult) AssertMaxViolations(t *testing.T, max int) {
	t.Helper()

	if a.scanErr != nil {
		t.Fatalf("archtest scan error: %v", a.scanErr)
	}

	if a.Violations > max {
		t.Errorf("found %d violations, max allowed %d", a.Violations, max)
	}
}

// AssertHealthScore fails the test if the average health score is below
// the given minimum (0-100).
func (a *ArchResult) AssertHealthScore(t *testing.T, min int) {
	t.Helper()

	if a.scanErr != nil {
		t.Fatalf("archtest scan error: %v", a.scanErr)
	}

	if a.HealthScore < min {
		t.Errorf("health score %d below minimum %d", a.HealthScore, min)
	}
}
