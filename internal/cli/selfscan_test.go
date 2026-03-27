package cli

import (
	"bytes"
	"os"
	"strings"
	"testing"
)

func TestSelfScanText(t *testing.T) {
	// Redirect stdout to capture output.
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	err = runSelfScan(nil, nil)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Fatalf("self-scan failed: %v", err)
	}

	required := []string{
		"archlint Self-Scan Dashboard",
		"Components",
		"components",
		"links",
		"Health",
	}

	for _, s := range required {
		if !strings.Contains(output, s) {
			t.Errorf("expected output to contain %q, got:\n%s", s, output)
		}
	}
}

func TestSelfScanMarkdown(t *testing.T) {
	selfScanFormat = "markdown"
	defer func() { selfScanFormat = "text" }()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("failed to create pipe: %v", err)
	}
	os.Stdout = w

	err = runSelfScan(nil, nil)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Fatalf("self-scan markdown failed: %v", err)
	}

	required := []string{
		"# archlint Self-Scan Dashboard",
		"## Components",
		"## Quality",
		"Health score",
	}

	for _, s := range required {
		if !strings.Contains(output, s) {
			t.Errorf("expected markdown output to contain %q, got:\n%s", s, output)
		}
	}
}

func TestHealthLabel(t *testing.T) {
	cases := []struct {
		score int
		want  string
	}{
		{100, "GOOD"},
		{80, "GOOD"},
		{79, "FAIR"},
		{60, "FAIR"},
		{59, "POOR"},
		{40, "POOR"},
		{39, "CRITICAL"},
		{0, "CRITICAL"},
	}

	for _, c := range cases {
		got := healthLabel(c.score)
		if got != c.want {
			t.Errorf("healthLabel(%d) = %q, want %q", c.score, got, c.want)
		}
	}
}

func TestFindModuleRoot(t *testing.T) {
	// Should find the archlint module root from the current package directory.
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}

	root := findModuleRoot(cwd)
	if root == "" {
		t.Error("findModuleRoot returned empty string, expected archlint source root")
	}
}
