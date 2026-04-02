package cli

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ---- health score ----

func TestCalcHealth(t *testing.T) {
	cases := []struct {
		violations int
		want       int
	}{
		{0, 100},
		{1, 98},
		{5, 90},
		{50, 0},
		{100, 0},
	}
	for _, tc := range cases {
		got := calcHealth(tc.violations)
		if got != tc.want {
			t.Errorf("calcHealth(%d) = %d, want %d", tc.violations, got, tc.want)
		}
	}
}

// ---- dirHasGoFiles ----

func TestDirHasGoFiles(t *testing.T) {
	// Create a temp dir with a .go file.
	dir := t.TempDir()
	f, err := os.Create(filepath.Join(dir, "main.go"))
	if err != nil {
		t.Fatal(err)
	}
	f.Close()

	if !dirHasGoFiles(dir) {
		t.Error("expected dirHasGoFiles=true for dir with main.go")
	}

	// Empty dir should return false.
	empty := t.TempDir()
	if dirHasGoFiles(empty) {
		t.Error("expected dirHasGoFiles=false for empty dir")
	}
}

// ---- subdirs ----

func TestSubdirs(t *testing.T) {
	parent := t.TempDir()
	sub1 := filepath.Join(parent, "repo1")
	sub2 := filepath.Join(parent, "repo2")
	if err := os.Mkdir(sub1, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(sub2, 0o755); err != nil {
		t.Fatal(err)
	}

	dirs, err := subdirs(parent)
	if err != nil {
		t.Fatalf("subdirs error: %v", err)
	}
	if len(dirs) != 2 {
		t.Errorf("expected 2 subdirs, got %d", len(dirs))
	}
}

func TestSubdirsEmpty(t *testing.T) {
	parent := t.TempDir()
	_, err := subdirs(parent)
	if err == nil {
		t.Error("expected error for empty parent dir")
	}
}

// ---- collectDirs ----

func TestCollectDirsExplicitList(t *testing.T) {
	dir1 := t.TempDir()
	dir2 := t.TempDir()
	// Both have go files so single-arg parent-detection is skipped.
	for _, d := range []string{dir1, dir2} {
		f, _ := os.Create(filepath.Join(d, "a.go"))
		f.Close()
	}

	// Multiple args: treated as explicit list.
	oldStdin := batchStdin
	batchStdin = false
	defer func() { batchStdin = oldStdin }()

	dirs, err := collectDirs([]string{dir1, dir2})
	if err != nil {
		t.Fatalf("collectDirs error: %v", err)
	}
	if len(dirs) != 2 {
		t.Errorf("expected 2 dirs, got %d", len(dirs))
	}
}

func TestCollectDirsNoArgs(t *testing.T) {
	oldStdin := batchStdin
	batchStdin = false
	defer func() { batchStdin = oldStdin }()

	_, err := collectDirs(nil)
	if err == nil {
		t.Error("expected error for no args and no stdin flag")
	}
}

// ---- buildReport ----

func TestBuildReport(t *testing.T) {
	results := []batchRepoResult{
		{Repository: "a", Violations: 10, Health: calcHealth(10)},
		{Repository: "b", Violations: 3, Health: calcHealth(3)},
		{Repository: "c", Error: "failed"},
	}
	r := buildReport(results)
	if r.TotalRepos != 3 {
		t.Errorf("expected 3 total repos, got %d", r.TotalRepos)
	}
	if r.ScannedOK != 2 {
		t.Errorf("expected 2 scanned ok, got %d", r.ScannedOK)
	}
	if r.Errors != 1 {
		t.Errorf("expected 1 error, got %d", r.Errors)
	}
	// avg health = (80 + 94) / 2 = 87
	if r.AvgHealth != 87 {
		t.Errorf("expected avg health 87, got %d", r.AvgHealth)
	}
}

func TestBuildReportAllErrors(t *testing.T) {
	results := []batchRepoResult{
		{Repository: "a", Error: "fail1"},
		{Repository: "b", Error: "fail2"},
	}
	r := buildReport(results)
	if r.AvgHealth != 0 {
		t.Errorf("expected 0 avg health when all repos have errors, got %d", r.AvgHealth)
	}
}

// ---- writeBatchMarkdown ----

func TestWriteBatchMarkdown(t *testing.T) {
	r, w, _ := os.Pipe()
	results := []batchRepoResult{
		{Repository: "svc-a", Violations: 5, SOLID: 2, GodClass: 1, FanOut: 1, Health: 90},
		{Repository: "svc-b", Violations: 0, Health: 100},
		{Repository: "broken", Error: "parse error"},
	}
	report := buildReport(results)
	report.Results = results

	if err := writeBatchMarkdown(w, report); err != nil {
		t.Fatalf("writeBatchMarkdown error: %v", err)
	}
	w.Close()

	var buf bytes.Buffer
	buf.ReadFrom(r)
	out := buf.String()

	if !strings.Contains(out, "# Architecture Health Report") {
		t.Error("expected report header")
	}
	if !strings.Contains(out, "svc-a") {
		t.Error("expected svc-a in output")
	}
	if !strings.Contains(out, "ERROR") {
		t.Error("expected ERROR row for broken repo")
	}
}

// ---- writeBatchCSV ----

func TestWriteBatchCSV(t *testing.T) {
	r, w, _ := os.Pipe()
	results := []batchRepoResult{
		{Repository: "svc-a", Path: "/repos/svc-a", Violations: 3, Health: 94},
	}
	report := buildReport(results)
	report.Results = results

	if err := writeBatchCSV(w, report); err != nil {
		t.Fatalf("writeBatchCSV error: %v", err)
	}
	w.Close()

	var buf bytes.Buffer
	buf.ReadFrom(r)

	csvR := csv.NewReader(&buf)
	records, err := csvR.ReadAll()
	if err != nil {
		t.Fatalf("csv parse error: %v", err)
	}
	if len(records) < 2 {
		t.Fatalf("expected header + 1 data row, got %d rows", len(records))
	}
	// Header check.
	if records[0][0] != "Repository" {
		t.Errorf("expected first header to be Repository, got %s", records[0][0])
	}
	// Data row.
	if records[1][0] != "svc-a" {
		t.Errorf("expected svc-a in data row, got %s", records[1][0])
	}
}

// ---- JSON output integration test ----

func TestRunBatchJSONOnSelf(t *testing.T) {
	// Scan the cli package itself as a known Go directory.
	dir := "."

	oldFormat := batchFormat
	oldOutput := batchOutput
	oldConfig := batchConfigFile
	oldStdin := batchStdin
	batchFormat = "json"
	batchOutput = ""
	batchConfigFile = ""
	batchStdin = false
	defer func() {
		batchFormat = oldFormat
		batchOutput = oldOutput
		batchConfigFile = oldConfig
		batchStdin = oldStdin
	}()

	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w

	runErr := runBatch(nil, []string{dir})

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if runErr != nil {
		t.Fatalf("runBatch error: %v", runErr)
	}

	var report batchReport
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("JSON parse error: %v\noutput: %s", err, output)
	}
	if report.TotalRepos != 1 {
		t.Errorf("expected 1 repo, got %d", report.TotalRepos)
	}
	if len(report.Results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(report.Results))
	}
	res := report.Results[0]
	if res.Health < 0 || res.Health > 100 {
		t.Errorf("health score out of range: %d", res.Health)
	}
}

// ---- output to file ----

func TestRunBatchOutputFile(t *testing.T) {
	tmpFile := filepath.Join(t.TempDir(), "report.md")

	oldFormat := batchFormat
	oldOutput := batchOutput
	oldConfig := batchConfigFile
	oldStdin := batchStdin
	batchFormat = "text"
	batchOutput = tmpFile
	batchConfigFile = ""
	batchStdin = false
	defer func() {
		batchFormat = oldFormat
		batchOutput = oldOutput
		batchConfigFile = oldConfig
		batchStdin = oldStdin
	}()

	if err := runBatch(nil, []string{"."}); err != nil {
		t.Fatalf("runBatch error: %v", err)
	}

	data, err := os.ReadFile(tmpFile)
	if err != nil {
		t.Fatalf("output file not created: %v", err)
	}
	if !strings.Contains(string(data), "# Architecture Health Report") {
		t.Error("output file does not contain expected header")
	}
}
