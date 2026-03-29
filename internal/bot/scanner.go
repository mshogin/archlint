package bot

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"

	"github.com/mshogin/archlint/internal/analyzer"
	"github.com/mshogin/archlint/internal/mcp"
)

// ScanResult holds the outcome of a repository scan.
type ScanResult struct {
	// TotalViolations is the total number of violations found.
	TotalViolations int
	// Categories maps violation kind -> count.
	Categories map[string]int
	// TopViolations is up to 5 most representative violations.
	TopViolations []mcp.Violation
	// HealthScore is 0-100 (100 = no violations).
	HealthScore int
}

// Scanner abstracts the scan operation so it can be swapped for tests or Docker.
type Scanner interface {
	Scan(ctx context.Context, repoURL string) (*ScanResult, error)
}

// LocalScanner clones the repository into a temp dir and runs archlint locally.
// TODO: replace with DockerScanner for full network isolation.
type LocalScanner struct {
	// archlintBin is the path to the archlint binary (empty = use current process).
	archlintBin string
}

// NewLocalScanner creates a LocalScanner.
// Pass archlintBin = "" to use the in-process analyzer directly.
func NewLocalScanner(archlintBin string) *LocalScanner {
	return &LocalScanner{archlintBin: archlintBin}
}

// Scan clones repoURL into a temp directory and runs the archlint analyzer on it.
func (s *LocalScanner) Scan(ctx context.Context, repoURL string) (*ScanResult, error) {
	tmpDir, err := os.MkdirTemp("", "archlint-scan-*")
	if err != nil {
		return nil, fmt.Errorf("create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	cloneDir := filepath.Join(tmpDir, "repo")
	if err := gitClone(ctx, repoURL, cloneDir); err != nil {
		return nil, fmt.Errorf("git clone %s: %w", repoURL, err)
	}

	return s.runAnalysis(ctx, cloneDir)
}

// gitClone runs git clone --depth=1 into dest.
func gitClone(ctx context.Context, repoURL, dest string) error {
	cmd := exec.CommandContext(ctx, "git", "clone", "--depth=1", repoURL, dest)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("git clone failed: %w\noutput: %s", err, string(out))
	}
	return nil
}

// runAnalysis runs the in-process analyzer on dir and returns a ScanResult.
func (s *LocalScanner) runAnalysis(ctx context.Context, dir string) (*ScanResult, error) {
	// If an external binary is provided, run it; otherwise use the in-process analyzer.
	if s.archlintBin != "" {
		return s.runExternalBinary(ctx, dir)
	}
	return runInProcessAnalysis(dir)
}

// runInProcessAnalysis uses the embedded analyzer directly.
func runInProcessAnalysis(dir string) (*ScanResult, error) {
	a := analyzer.NewGoAnalyzer()
	graph, err := a.Analyze(dir)
	if err != nil {
		return nil, fmt.Errorf("analyze: %w", err)
	}

	violations := mcp.DetectAllViolations(graph)

	allMetrics := mcp.ComputeAllFileMetrics(a, graph)
	for _, m := range allMetrics {
		violations = append(violations, m.SRPViolations...)
		violations = append(violations, m.DIPViolations...)
		violations = append(violations, m.ISPViolations...)
		for _, gc := range m.GodClasses {
			violations = append(violations, mcp.Violation{
				Kind:    "god-class",
				Message: fmt.Sprintf("God class detected: %s", gc),
				Target:  gc,
			})
		}
		for _, hub := range m.HubNodes {
			violations = append(violations, mcp.Violation{
				Kind:    "hub-node",
				Message: fmt.Sprintf("Hub node detected: %s", hub),
				Target:  hub,
			})
		}
	}

	sort.Slice(violations, func(i, j int) bool {
		if violations[i].Kind != violations[j].Kind {
			return violations[i].Kind < violations[j].Kind
		}
		return violations[i].Target < violations[j].Target
	})

	categories := make(map[string]int)
	for _, v := range violations {
		categories[v.Kind]++
	}

	top := violations
	if len(top) > 5 {
		top = top[:5]
	}

	return &ScanResult{
		TotalViolations: len(violations),
		Categories:      categories,
		TopViolations:   top,
		HealthScore:     HealthScoreFor(len(violations)),
	}, nil
}

// runExternalBinary executes the archlint binary and parses its JSON output.
func (s *LocalScanner) runExternalBinary(ctx context.Context, dir string) (*ScanResult, error) {
	cmd := exec.CommandContext(ctx, s.archlintBin, "scan", dir, "--format", "json")
	out, err := cmd.Output()
	if err != nil {
		// Exit code 1 means violations found, not an error.
		if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
			// parse output below
		} else {
			return nil, fmt.Errorf("archlint binary: %w", err)
		}
	}

	var result struct {
		Passed     bool               `json:"passed"`
		Violations int                `json:"violations"`
		Categories map[string]int     `json:"categories"`
		Details    []mcp.Violation    `json:"details"`
	}
	if err := json.Unmarshal(out, &result); err != nil {
		return nil, fmt.Errorf("parse archlint output: %w", err)
	}

	top := result.Details
	if len(top) > 5 {
		top = top[:5]
	}

	return &ScanResult{
		TotalViolations: result.Violations,
		Categories:      result.Categories,
		TopViolations:   top,
		HealthScore:     HealthScoreFor(result.Violations),
	}, nil
}

// HealthScoreFor converts a violation count to a 0-100 health score.
// 0 violations = 100, each violation reduces by 2, floor at 0.
func HealthScoreFor(violations int) int {
	score := 100 - violations*2
	if score < 0 {
		return 0
	}
	return score
}
