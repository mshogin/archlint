package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// --- Config parsing tests ---

func TestLoadMonitoredRepos_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "monitored-repos.yaml")

	// Write empty-ish yaml.
	if err := os.WriteFile(cfgPath, []byte("repos: []\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := loadMonitoredRepos(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(cfg.Repos) != 0 {
		t.Errorf("expected 0 repos, got %d", len(cfg.Repos))
	}
}

func TestLoadMonitoredRepos_Missing(t *testing.T) {
	cfg, err := loadMonitoredRepos("/nonexistent/path/monitored-repos.yaml")
	if err != nil {
		t.Fatalf("expected no error for missing file, got: %v", err)
	}
	if len(cfg.Repos) != 0 {
		t.Errorf("expected empty config for missing file, got %d repos", len(cfg.Repos))
	}
}

func TestLoadMonitoredRepos_ParsesFields(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "monitored-repos.yaml")

	content := `repos:
  - url: https://github.com/mshogin/archlint
    language: Go
    added: 2026-04-07
    status: active
  - url: https://github.com/kgatilin/deskd
    language: Rust
    added: 2026-04-07
    status: active
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	cfg, err := loadMonitoredRepos(cfgPath)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(cfg.Repos) != 2 {
		t.Fatalf("expected 2 repos, got %d", len(cfg.Repos))
	}

	r0 := cfg.Repos[0]
	if r0.URL != "https://github.com/mshogin/archlint" {
		t.Errorf("unexpected URL: %q", r0.URL)
	}
	if r0.Language != "Go" {
		t.Errorf("unexpected language: %q", r0.Language)
	}
	if r0.Added != "2026-04-07" {
		t.Errorf("unexpected added: %q", r0.Added)
	}
	if r0.Status != "active" {
		t.Errorf("unexpected status: %q", r0.Status)
	}

	r1 := cfg.Repos[1]
	if r1.Language != "Rust" {
		t.Errorf("expected Rust language, got %q", r1.Language)
	}
}

func TestLoadMonitoredRepos_InvalidYAML(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "monitored-repos.yaml")

	if err := os.WriteFile(cfgPath, []byte("not: valid: yaml: [[\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	_, err := loadMonitoredRepos(cfgPath)
	if err == nil {
		t.Error("expected error for invalid YAML, got nil")
	}
}

func TestSaveMonitoredRepos(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "monitored-repos.yaml")

	cfg := MonitoredReposConfig{
		Repos: []MonitoredRepo{
			{URL: "https://github.com/test/repo", Language: "Go", Added: "2026-04-01", Status: "active"},
		},
	}

	if err := saveMonitoredRepos(cfgPath, cfg); err != nil {
		t.Fatalf("save error: %v", err)
	}

	loaded, err := loadMonitoredRepos(cfgPath)
	if err != nil {
		t.Fatalf("load after save: %v", err)
	}

	if len(loaded.Repos) != 1 {
		t.Fatalf("expected 1 repo, got %d", len(loaded.Repos))
	}
	if loaded.Repos[0].URL != "https://github.com/test/repo" {
		t.Errorf("unexpected URL after save: %q", loaded.Repos[0].URL)
	}
}

func TestSaveMonitoredRepos_Header(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "monitored-repos.yaml")

	cfg := MonitoredReposConfig{}
	if err := saveMonitoredRepos(cfgPath, cfg); err != nil {
		t.Fatalf("save: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}

	if !strings.HasPrefix(string(data), "#") {
		t.Error("expected file to start with a comment header")
	}
}

// --- Command tests ---

func TestRunMonitorList_Empty(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "monitored-repos.yaml")
	if err := os.WriteFile(cfgPath, []byte("repos: []\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	origCfg := monitorConfigFile
	monitorConfigFile = cfgPath
	defer func() { monitorConfigFile = origCfg }()

	err := runMonitorList(nil, nil)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(buf.String(), "No monitored") {
		t.Errorf("expected empty message, got: %s", buf.String())
	}
}

func TestRunMonitorList_WithRepos(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "monitored-repos.yaml")
	content := `repos:
  - url: https://github.com/mshogin/archlint
    language: Go
    added: 2026-04-07
    status: active
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	origCfg := monitorConfigFile
	monitorConfigFile = cfgPath
	defer func() { monitorConfigFile = origCfg }()

	err := runMonitorList(nil, nil)

	w.Close()
	os.Stdout = old

	var buf bytes.Buffer
	buf.ReadFrom(r)
	output := buf.String()

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(output, "https://github.com/mshogin/archlint") {
		t.Errorf("expected repo URL in output, got: %s", output)
	}
	if !strings.Contains(output, "Go") {
		t.Errorf("expected language in output, got: %s", output)
	}
}

func TestRunMonitorAdd_NewRepo(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "monitored-repos.yaml")
	if err := os.WriteFile(cfgPath, []byte("repos: []\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	origCfg := monitorConfigFile
	origLang := monitorLanguage
	monitorConfigFile = cfgPath
	monitorLanguage = "Go"
	defer func() {
		monitorConfigFile = origCfg
		monitorLanguage = origLang
	}()

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runMonitorAdd(nil, []string{"https://github.com/example/myrepo"})

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	buf.ReadFrom(r)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg, err := loadMonitoredRepos(cfgPath)
	if err != nil {
		t.Fatalf("load after add: %v", err)
	}
	if len(cfg.Repos) != 1 {
		t.Fatalf("expected 1 repo after add, got %d", len(cfg.Repos))
	}
	if cfg.Repos[0].URL != "https://github.com/example/myrepo" {
		t.Errorf("unexpected URL: %q", cfg.Repos[0].URL)
	}
	if cfg.Repos[0].Language != "Go" {
		t.Errorf("unexpected language: %q", cfg.Repos[0].Language)
	}
	if cfg.Repos[0].Status != "active" {
		t.Errorf("unexpected status: %q", cfg.Repos[0].Status)
	}
}

func TestRunMonitorAdd_Duplicate(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "monitored-repos.yaml")
	content := `repos:
  - url: https://github.com/example/myrepo
    language: Go
    added: 2026-04-01
    status: active
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	origCfg := monitorConfigFile
	monitorConfigFile = cfgPath
	defer func() { monitorConfigFile = origCfg }()

	err := runMonitorAdd(nil, []string{"https://github.com/example/myrepo"})
	if err == nil {
		t.Error("expected error for duplicate repo, got nil")
	}
	if !strings.Contains(err.Error(), "already monitored") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestRunMonitorRemove_Existing(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "monitored-repos.yaml")
	content := `repos:
  - url: https://github.com/example/myrepo
    language: Go
    added: 2026-04-01
    status: active
  - url: https://github.com/example/other
    language: Rust
    added: 2026-04-01
    status: active
`
	if err := os.WriteFile(cfgPath, []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	origCfg := monitorConfigFile
	monitorConfigFile = cfgPath
	defer func() { monitorConfigFile = origCfg }()

	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	err := runMonitorRemove(nil, []string{"https://github.com/example/myrepo"})

	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	buf.ReadFrom(r)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	cfg, err := loadMonitoredRepos(cfgPath)
	if err != nil {
		t.Fatalf("load after remove: %v", err)
	}
	if len(cfg.Repos) != 1 {
		t.Fatalf("expected 1 repo after remove, got %d", len(cfg.Repos))
	}
	if cfg.Repos[0].URL != "https://github.com/example/other" {
		t.Errorf("wrong repo remaining: %q", cfg.Repos[0].URL)
	}
}

func TestRunMonitorRemove_NotFound(t *testing.T) {
	dir := t.TempDir()
	cfgPath := filepath.Join(dir, "monitored-repos.yaml")
	if err := os.WriteFile(cfgPath, []byte("repos: []\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	origCfg := monitorConfigFile
	monitorConfigFile = cfgPath
	defer func() { monitorConfigFile = origCfg }()

	err := runMonitorRemove(nil, []string{"https://github.com/nonexistent/repo"})
	if err == nil {
		t.Error("expected error for non-existent repo, got nil")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("unexpected error: %v", err)
	}
}

// --- parseRepoURLFromBody tests ---

func TestParseRepoURLFromBody(t *testing.T) {
	cases := []struct {
		body string
		want string
	}{
		{
			body: "Please monitor https://github.com/example/myrepo for violations.",
			want: "https://github.com/example/myrepo",
		},
		{
			body: "Repo: github.com/example/myrepo",
			want: "https://github.com/example/myrepo",
		},
		{
			body: "No URL here.",
			want: "",
		},
		{
			body: "Line 1\nhttps://github.com/test/awesome-project\nLine 3",
			want: "https://github.com/test/awesome-project",
		},
		{
			body: "See (https://github.com/user/repo) for details",
			want: "https://github.com/user/repo",
		},
	}

	for _, tc := range cases {
		got := parseRepoURLFromBody(tc.body)
		if got != tc.want {
			t.Errorf("parseRepoURLFromBody(%q) = %q, want %q", tc.body, got, tc.want)
		}
	}
}

// --- truncate tests ---

func TestTruncate(t *testing.T) {
	if got := truncate("hello", 10); got != "hello" {
		t.Errorf("expected 'hello', got %q", got)
	}
	if got := truncate("hello world", 5); got != "hello..." {
		t.Errorf("expected 'hello...', got %q", got)
	}
	if got := truncate("", 10); got != "" {
		t.Errorf("expected empty string, got %q", got)
	}
}
