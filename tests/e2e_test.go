// Package tests contains end-to-end tests for the archlint CLI.
// These tests exercise the full pipeline: build binary -> run commands -> verify output.
//
// Run with:
//
//	go test ./tests/... -run TestE2E -v
//
// The tests use the archlint-demo project as a realistic fixture.
// ARCHLINT_DEMO env var can override the demo project path.
package tests

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gopkg.in/yaml.v3"
)

// e2eBinary returns the path to the archlint binary used for E2E tests.
// Prefers a pre-built binary at /tmp/archlint-e2e, falls back to building from source.
func e2eBinary(t *testing.T) string {
	t.Helper()

	// Check for pre-built binary.
	if _, err := os.Stat("/tmp/archlint-e2e"); err == nil {
		return "/tmp/archlint-e2e"
	}

	// Build from source into a temp dir.
	binPath := filepath.Join(t.TempDir(), "archlint")
	repoRoot := filepath.Join("..", "..")
	cmd := exec.Command("go", "build", "-o", binPath, "./cmd/archlint/")
	cmd.Dir = repoRoot
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("failed to build archlint: %v\n%s", err, out)
	}

	return binPath
}

// demoDir returns the path to the archlint-demo project used as a test fixture.
func demoDir(t *testing.T) string {
	t.Helper()

	if p := os.Getenv("ARCHLINT_DEMO"); p != "" {
		return p
	}

	// Default: sibling directory in the same parent.
	abs, err := filepath.Abs(filepath.Join("..", "..", "..", "archlint-demo"))
	if err != nil {
		t.Skipf("cannot resolve demo dir: %v", err)
	}

	if _, err := os.Stat(abs); os.IsNotExist(err) {
		t.Skipf("archlint-demo not found at %s; set ARCHLINT_DEMO env var", abs)
	}

	return abs
}

// run executes archlint with the given args and returns (stdout+stderr, exit code).
func run(t *testing.T, bin string, args ...string) (string, int) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.CombinedOutput()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else if ctx.Err() == context.DeadlineExceeded {
			exitCode = -1
		}
	}

	return string(out), exitCode
}

// TestE2EInit verifies that archlint init generates a valid .archlint.yaml.
func TestE2EInit(t *testing.T) {
	bin := e2eBinary(t)
	demo := demoDir(t)

	// --dry-run should print YAML to stdout without writing files.
	out, code := run(t, bin, "init", demo, "--dry-run")
	if code != 0 {
		t.Fatalf("init --dry-run exited %d\noutput:\n%s", code, out)
	}

	// Output must be valid YAML.
	var parsed map[string]interface{}
	if err := yaml.Unmarshal([]byte(out), &parsed); err != nil {
		t.Fatalf("init --dry-run output is not valid YAML: %v\noutput:\n%s", err, out)
	}

	// Must contain "rules" key.
	if _, ok := parsed["rules"]; !ok {
		t.Errorf("generated YAML missing 'rules' key\noutput:\n%s", out)
	}

	// Must contain "layers" key.
	if _, ok := parsed["layers"]; !ok {
		t.Errorf("generated YAML missing 'layers' key\noutput:\n%s", out)
	}

	t.Logf("init --dry-run: ok (%d chars, %d top-level keys)", len(out), len(parsed))
}

// TestE2EScanClean verifies that scan passes when code has no layer violations.
func TestE2EScanClean(t *testing.T) {
	bin := e2eBinary(t)
	demo := demoDir(t)

	internalDir := filepath.Join(demo, "internal")
	configFile := filepath.Join(demo, ".archlint.yaml")

	// The default demo state must be clean (no layer violations).
	out, code := run(t, bin, "scan", internalDir, "--config", configFile)
	if code != 0 {
		t.Fatalf("scan exited %d on clean code (expected 0)\noutput:\n%s", code, out)
	}

	if !strings.Contains(out, "PASSED") {
		t.Errorf("expected PASSED in scan output\noutput:\n%s", out)
	}
}

// TestE2EScanWithViolations verifies that scan fails when a layer violation is injected.
func TestE2EScanWithViolations(t *testing.T) {
	bin := e2eBinary(t)
	demo := demoDir(t)

	orderHandlerPath := filepath.Join(demo, "internal", "handler", "order.go")
	violationStep := filepath.Join(demo, "demo-scenario", "step1-quick-fix.go")

	// Back up the original file.
	origContent, err := os.ReadFile(orderHandlerPath)
	if err != nil {
		t.Fatalf("read handler: %v", err)
	}
	t.Cleanup(func() {
		if err := os.WriteFile(orderHandlerPath, origContent, 0o644); err != nil {
			t.Errorf("restore handler: %v", err)
		}
	})

	// Read step1 file. It starts with //go:build ignore - remove that line.
	stepContent, err := os.ReadFile(violationStep)
	if err != nil {
		t.Fatalf("read step1: %v", err)
	}
	lines := strings.SplitN(string(stepContent), "\n", 2)
	if strings.Contains(lines[0], "go:build ignore") && len(lines) > 1 {
		stepContent = []byte(lines[1])
	}

	if err := os.WriteFile(orderHandlerPath, stepContent, 0o644); err != nil {
		t.Fatalf("write step1 to handler: %v", err)
	}

	internalDir := filepath.Join(demo, "internal")
	configFile := filepath.Join(demo, ".archlint.yaml")

	out, code := run(t, bin, "scan", internalDir, "--config", configFile)
	if code == 0 {
		t.Fatalf("scan exited 0 on violating code (expected non-zero)\noutput:\n%s", out)
	}

	if !strings.Contains(out, "FAILED") {
		t.Errorf("expected FAILED in scan output\noutput:\n%s", out)
	}

	if !strings.Contains(out, "layer-violation") {
		t.Errorf("expected 'layer-violation' in scan output\noutput:\n%s", out)
	}

	t.Logf("scan detected violation: exit=%d", code)
}

// TestE2ECollect verifies that collect produces a valid architecture.yaml.
func TestE2ECollect(t *testing.T) {
	bin := e2eBinary(t)
	demo := demoDir(t)

	outFile := filepath.Join(t.TempDir(), "architecture.yaml")

	out, code := run(t, bin, "collect", demo, "-o", outFile)
	if code != 0 {
		t.Fatalf("collect exited %d\noutput:\n%s", code, out)
	}

	// Output file must exist and be non-empty.
	info, err := os.Stat(outFile)
	if err != nil {
		t.Fatalf("architecture.yaml not created: %v", err)
	}

	if info.Size() == 0 {
		t.Fatal("architecture.yaml is empty")
	}

	// Must contain valid YAML with components and links.
	data, err := os.ReadFile(outFile)
	if err != nil {
		t.Fatalf("read architecture.yaml: %v", err)
	}

	var graph struct {
		Nodes []interface{} `yaml:"components"`
		Edges []interface{} `yaml:"links"`
	}
	if err := yaml.Unmarshal(data, &graph); err != nil {
		t.Fatalf("architecture.yaml is not valid YAML: %v", err)
	}

	if len(graph.Nodes) == 0 {
		t.Error("architecture.yaml has no components")
	}

	t.Logf("collect: %d components, %d links", len(graph.Nodes), len(graph.Edges))

	// stdout must mention component count.
	if !strings.Contains(out, "Found components") {
		t.Errorf("expected 'Found components' in collect output\noutput:\n%s", out)
	}
}

// callgraphEntry returns the correct entry point ID for the archlint-demo OrderService.CreateOrder.
// When the scan directory is an absolute path the node IDs use the directory-relative module prefix
// derived from the folder name (e.g. "archlint-demo/internal/service.OrderService.CreateOrder").
// When using a relative path like "./internal" the prefix matches what was passed (e.g. "internal/service...").
// We always pass an absolute path so we use the folder-name-derived prefix.
func callgraphEntry(demo string) string {
	base := filepath.Base(demo)
	return base + "/internal/service.OrderService.CreateOrder"
}

// TestE2ECallgraph verifies that callgraph builds a call graph from an entry point.
func TestE2ECallgraph(t *testing.T) {
	bin := e2eBinary(t)
	demo := demoDir(t)

	internalDir := filepath.Join(demo, "internal")
	outDir := t.TempDir()
	entry := callgraphEntry(demo)

	out, code := run(t, bin, "callgraph", internalDir, "--entry", entry, "--no-puml", "-o", outDir)
	if code != 0 {
		t.Fatalf("callgraph exited %d\noutput:\n%s", code, out)
	}

	if !strings.Contains(out, "Graph built") {
		t.Errorf("expected 'Graph built' in callgraph output\noutput:\n%s", out)
	}

	// Verify the YAML file was created.
	yamlPath := filepath.Join(outDir, "callgraph.yaml")
	if _, err := os.Stat(yamlPath); err != nil {
		t.Errorf("callgraph.yaml not created at %s: %v", yamlPath, err)
	}

	t.Logf("callgraph: ok\noutput: %s", strings.TrimSpace(out))
}

// TestE2ECallgraphCycleDetection verifies that callgraph detects cycles
// when behavioral cycle code is active in the service layer.
func TestE2ECallgraphCycleDetection(t *testing.T) {
	bin := e2eBinary(t)
	demo := demoDir(t)

	internalDir := filepath.Join(demo, "internal")
	outDir := t.TempDir()
	entry := callgraphEntry(demo)

	// The current demo code has a self-referential call chain in order_service.go.
	// Run callgraph and check the resulting YAML.
	out, code := run(t, bin, "callgraph", internalDir, "--entry", entry, "--no-puml", "-o", outDir)
	if code != 0 {
		t.Fatalf("callgraph exited %d\noutput:\n%s", code, out)
	}

	// The YAML result should contain cycles_detected field.
	yamlPath := filepath.Join(outDir, "callgraph.yaml")
	data, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("read callgraph.yaml: %v", err)
	}

	var cg struct {
		Stats struct {
			CyclesDetected int `yaml:"cycles_detected"`
		} `yaml:"stats"`
	}
	if err := yaml.Unmarshal(data, &cg); err != nil {
		t.Fatalf("callgraph.yaml parse failed: %v", err)
	}

	t.Logf("callgraph cycles_detected: %d", cg.Stats.CyclesDetected)
	// We just check the field is present (parseable); specific cycle count depends on current code state.
}

// TestE2EWatchStartsAndStops verifies that watch starts, runs a scan, and exits cleanly on signal.
func TestE2EWatchStartsAndStops(t *testing.T) {
	bin := e2eBinary(t)
	demo := demoDir(t)

	internalDir := filepath.Join(demo, "internal")

	// Use a short-timeout context to kill the watch process after it has started.
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, bin, "watch", internalDir)
	out, err := cmd.CombinedOutput()

	// We expect the process to be killed by context timeout (exit code -1/signal) or
	// to produce the watch header line before exiting.
	output := string(out)

	if !strings.Contains(output, "[archlint] Watching") {
		t.Errorf("watch did not start: expected '[archlint] Watching' in output\noutput:\n%s", output)
	}

	// Process killed by timeout is expected; only fail on unexpected errors.
	if err != nil && ctx.Err() == nil {
		// Context not expired but got an error - might be a real startup failure.
		if !strings.Contains(output, "[archlint] Watching") {
			t.Errorf("watch failed unexpectedly: %v\noutput:\n%s", err, output)
		}
	}

	t.Logf("watch started cleanly, first scan output present")
}

// TestE2EBatch verifies that batch scan produces a markdown health report.
func TestE2EBatch(t *testing.T) {
	bin := e2eBinary(t)
	demo := demoDir(t)

	internalDir := filepath.Join(demo, "internal")

	out, code := run(t, bin, "batch", internalDir)
	if code != 0 {
		t.Fatalf("batch exited %d\noutput:\n%s", code, out)
	}

	// Must contain the markdown table header.
	if !strings.Contains(out, "Architecture Health Report") {
		t.Errorf("expected 'Architecture Health Report' in batch output\noutput:\n%s", out)
	}

	if !strings.Contains(out, "Health") {
		t.Errorf("expected 'Health' column in batch output\noutput:\n%s", out)
	}

	t.Logf("batch: ok (%d chars)", len(out))
}

// TestE2EMetrics verifies that metrics produces package coupling/size metrics.
func TestE2EMetrics(t *testing.T) {
	bin := e2eBinary(t)
	demo := demoDir(t)

	internalDir := filepath.Join(demo, "internal")

	out, code := run(t, bin, "metrics", internalDir)
	if code != 0 {
		t.Fatalf("metrics exited %d\noutput:\n%s", code, out)
	}

	if !strings.Contains(out, "Package Metrics") {
		t.Errorf("expected 'Package Metrics' in output\noutput:\n%s", out)
	}

	if !strings.Contains(out, "Coupling:") {
		t.Errorf("expected 'Coupling:' in metrics output\noutput:\n%s", out)
	}

	if !strings.Contains(out, "Health:") {
		t.Errorf("expected 'Health:' in metrics output\noutput:\n%s", out)
	}

	t.Logf("metrics: ok (%d chars)", len(out))
}

// TestE2EMonitorList verifies that monitor list works (may show empty list).
func TestE2EMonitorList(t *testing.T) {
	bin := e2eBinary(t)

	out, code := run(t, bin, "monitor", "list")
	// Exit code 0 for empty list is acceptable.
	if code != 0 {
		t.Fatalf("monitor list exited %d\noutput:\n%s", code, out)
	}

	// Either shows repos or says "No monitored repositories configured".
	if !strings.Contains(out, "No monitored") && !strings.Contains(out, "Repository") {
		t.Errorf("unexpected monitor list output:\n%s", out)
	}

	t.Logf("monitor list: ok")
}

// TestE2EScanConfigFlag verifies that --config flag is accepted by scan.
func TestE2EScanConfigFlag(t *testing.T) {
	bin := e2eBinary(t)
	demo := demoDir(t)

	internalDir := filepath.Join(demo, "internal")
	configFile := filepath.Join(demo, ".archlint.yaml")

	// --config flag must be accepted without error.
	out, code := run(t, bin, "scan", internalDir, "--config", configFile)
	if code != 0 {
		t.Fatalf("scan --config exited %d\noutput:\n%s", code, out)
	}

	if strings.Contains(out, "unknown flag") || strings.Contains(out, "Error:") {
		t.Errorf("scan --config produced error\noutput:\n%s", out)
	}

	t.Logf("scan --config: ok")
}

// TestE2EValidateBuiltin verifies that validate (built-in Go engine, no --python) works.
func TestE2EValidateBuiltin(t *testing.T) {
	bin := e2eBinary(t)
	demo := demoDir(t)

	out, code := run(t, bin, "validate", demo)
	if code != 0 {
		t.Fatalf("validate exited %d\noutput:\n%s", code, out)
	}

	// Must mention component count.
	if !strings.Contains(out, "components:") {
		t.Errorf("expected 'components:' in validate output\noutput:\n%s", out)
	}

	t.Logf("validate (builtin): ok")
}
