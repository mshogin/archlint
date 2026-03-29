package bot_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/mshogin/archlint/internal/bot"
	"github.com/mshogin/archlint/internal/mcp"
)

func TestFormatResultComment_NoViolations(t *testing.T) {
	result := &bot.ScanResult{
		TotalViolations: 0,
		Categories:      map[string]int{},
		TopViolations:   nil,
		HealthScore:     100,
	}
	comment := bot.FormatResultComment("alice", "myrepo", result)

	if !strings.Contains(comment, "alice/myrepo") {
		t.Error("comment missing owner/repo")
	}
	if !strings.Contains(comment, "100/100") {
		t.Error("comment missing health score 100/100")
	}
	if !strings.Contains(comment, "PASSED") {
		t.Error("comment missing PASSED status")
	}
	if !strings.Contains(comment, "Add to CI") {
		t.Error("comment missing CI section")
	}
}

func TestFormatResultComment_WithViolations(t *testing.T) {
	result := &bot.ScanResult{
		TotalViolations: 3,
		Categories:      map[string]int{"coupling": 2, "god-class": 1},
		TopViolations: []mcp.Violation{
			{Kind: "coupling", Message: "too coupled", Target: "pkg/a"},
			{Kind: "coupling", Message: "also coupled", Target: "pkg/b"},
			{Kind: "god-class", Message: "God class", Target: "MyService"},
		},
		HealthScore: 94,
	}
	comment := bot.FormatResultComment("alice", "myrepo", result)

	if !strings.Contains(comment, "3 violation(s)") {
		t.Error("comment missing violation count")
	}
	if !strings.Contains(comment, "coupling") {
		t.Error("comment missing coupling category")
	}
	if !strings.Contains(comment, "god-class") {
		t.Error("comment missing god-class category")
	}
	if !strings.Contains(comment, "Top violations") {
		t.Error("comment missing Top violations section")
	}
	if !strings.Contains(comment, "94/100") {
		t.Error("comment missing health score 94/100")
	}
}

func TestFormatResultComment_MoreThan5Violations(t *testing.T) {
	violations := make([]mcp.Violation, 7)
	for i := range violations {
		violations[i] = mcp.Violation{Kind: "coupling", Message: "coupled", Target: "pkg"}
	}
	result := &bot.ScanResult{
		TotalViolations: 7,
		Categories:      map[string]int{"coupling": 7},
		TopViolations:   violations[:5],
		HealthScore:     86,
	}
	comment := bot.FormatResultComment("alice", "myrepo", result)

	if !strings.Contains(comment, "2 more") {
		t.Errorf("expected '2 more' in comment, got: %s", comment)
	}
}

func TestFormatErrorComment(t *testing.T) {
	comment := bot.FormatErrorComment("alice", "badrepo", errors.New("network timeout"))
	// fmt not imported in test, use strings
	if !strings.Contains(comment, "alice/badrepo") {
		t.Error("error comment missing owner/repo")
	}
	if !strings.Contains(comment, "scan failed") {
		t.Error("error comment missing 'scan failed'")
	}
}
