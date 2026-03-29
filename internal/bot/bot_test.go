package bot_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/mshogin/archlint/internal/bot"
	"github.com/mshogin/archlint/internal/mcp"
)

// --- fakes ---

type fakeGitHub struct {
	issues   []bot.Issue
	comments map[int][]string
	closed   []int
	postErr  error
	closeErr error
}

func newFakeGitHub(issues ...bot.Issue) *fakeGitHub {
	return &fakeGitHub{
		issues:   issues,
		comments: make(map[int][]string),
	}
}

func (f *fakeGitHub) ListOpenIssues(_ context.Context, _, _ string) ([]bot.Issue, error) {
	return f.issues, nil
}

func (f *fakeGitHub) PostComment(_ context.Context, _, _ string, number int, body string) error {
	if f.postErr != nil {
		return f.postErr
	}
	f.comments[number] = append(f.comments[number], body)
	return nil
}

func (f *fakeGitHub) CloseIssue(_ context.Context, _, _ string, number int) error {
	if f.closeErr != nil {
		return f.closeErr
	}
	f.closed = append(f.closed, number)
	return nil
}

type fakeScanner struct {
	result *bot.ScanResult
	err    error
}

func (f *fakeScanner) Scan(_ context.Context, _ string) (*bot.ScanResult, error) {
	return f.result, f.err
}

func newOKScanner(violations int) *fakeScanner {
	cats := make(map[string]int)
	var top []mcp.Violation
	if violations > 0 {
		cats["coupling"] = violations
		top = []mcp.Violation{{Kind: "coupling", Message: "test coupling", Target: "pkg/foo"}}
	}
	return &fakeScanner{result: &bot.ScanResult{
		TotalViolations: violations,
		Categories:      cats,
		TopViolations:   top,
		HealthScore:     bot.HealthScoreFor(violations),
	}}
}

// --- ParseScanTarget ---

func TestParseScanTarget(t *testing.T) {
	tests := []struct {
		title     string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{"scan: mshogin/archlint", "mshogin", "archlint", false},
		{"scan:mshogin/archlint", "mshogin", "archlint", false},
		{"Scan: kubernetes/kubernetes", "kubernetes", "kubernetes", false},
		{"SCAN: foo/bar", "foo", "bar", false},
		{"scan: only", "", "", true},
		{"scan: /norepo", "", "", true},
		{"scan: noowner/", "", "", true},
		{"scan: ", "", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			o, r, err := bot.ParseScanTarget(tt.title)
			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil (owner=%q repo=%q)", o, r)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if o != tt.wantOwner || r != tt.wantRepo {
				t.Errorf("got %s/%s, want %s/%s", o, r, tt.wantOwner, tt.wantRepo)
			}
		})
	}
}

// --- Bot.Run: single poll ---

func TestBotProcessesIssueAndCloses(t *testing.T) {
	gh := newFakeGitHub(
		bot.Issue{Number: 42, Title: "scan: alice/myrepo", State: "open"},
	)
	scanner := newOKScanner(3)

	cfg := bot.Config{
		Owner:        "mshogin",
		Repo:         "archlint",
		Token:        "tok",
		PollInterval: 100 * time.Millisecond,
		ScanTimeout:  5 * time.Second,
	}
	b := bot.New(cfg, gh, scanner)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_ = b.Run(ctx)

	if len(gh.comments[42]) == 0 {
		t.Fatal("expected comment on issue #42, got none")
	}
	comment := gh.comments[42][0]
	if !strings.Contains(comment, "alice/myrepo") {
		t.Errorf("comment missing repo name: %s", comment)
	}
	if !strings.Contains(comment, "Health score") {
		t.Errorf("comment missing health score: %s", comment)
	}

	if len(gh.closed) == 0 || gh.closed[0] != 42 {
		t.Errorf("expected issue #42 to be closed, closed=%v", gh.closed)
	}
}

func TestBotSkipsNonScanIssues(t *testing.T) {
	gh := newFakeGitHub(
		bot.Issue{Number: 1, Title: "bug: something broken", State: "open"},
		bot.Issue{Number: 2, Title: "feature request", State: "open"},
	)
	scanner := newOKScanner(0)

	cfg := bot.Config{
		Owner:        "mshogin",
		Repo:         "archlint",
		Token:        "tok",
		PollInterval: 100 * time.Millisecond,
		ScanTimeout:  5 * time.Second,
	}
	b := bot.New(cfg, gh, scanner)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_ = b.Run(ctx)

	if len(gh.closed) != 0 {
		t.Errorf("expected no issues closed, closed=%v", gh.closed)
	}
}

func TestBotHandlesScanError(t *testing.T) {
	gh := newFakeGitHub(
		bot.Issue{Number: 7, Title: "scan: bad/repo", State: "open"},
	)
	scanner := &fakeScanner{err: errors.New("git clone failed")}

	cfg := bot.Config{
		Owner:        "mshogin",
		Repo:         "archlint",
		Token:        "tok",
		PollInterval: 100 * time.Millisecond,
		ScanTimeout:  5 * time.Second,
	}
	b := bot.New(cfg, gh, scanner)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_ = b.Run(ctx)

	if len(gh.comments[7]) == 0 {
		t.Fatal("expected error comment on issue #7")
	}
	if !strings.Contains(gh.comments[7][0], "scan failed") {
		t.Errorf("expected 'scan failed' in comment, got: %s", gh.comments[7][0])
	}
	if len(gh.closed) == 0 || gh.closed[0] != 7 {
		t.Errorf("expected issue #7 to be closed even on error, closed=%v", gh.closed)
	}
}

func TestBotHandlesInvalidTitle(t *testing.T) {
	gh := newFakeGitHub(
		bot.Issue{Number: 9, Title: "scan: no-slash-here", State: "open"},
	)
	scanner := newOKScanner(0)

	cfg := bot.Config{
		Owner:        "mshogin",
		Repo:         "archlint",
		Token:        "tok",
		PollInterval: 100 * time.Millisecond,
		ScanTimeout:  5 * time.Second,
	}
	b := bot.New(cfg, gh, scanner)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	_ = b.Run(ctx)

	if len(gh.comments[9]) == 0 {
		t.Fatal("expected error comment on issue #9")
	}
	if len(gh.closed) == 0 || gh.closed[0] != 9 {
		t.Errorf("expected issue #9 to be closed, closed=%v", gh.closed)
	}
}
