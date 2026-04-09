package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// mkdirs creates nested directories relative to base.
func mkdirs(t *testing.T, base string, dirs ...string) {
	t.Helper()
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(base, d), 0755); err != nil {
			t.Fatalf("mkdirs: %v", err)
		}
	}
}

// touch creates an empty file at base/name.
func touch(t *testing.T, base, name string) {
	t.Helper()
	p := filepath.Join(base, name)
	if err := os.WriteFile(p, []byte{}, 0644); err != nil {
		t.Fatalf("touch: %v", err)
	}
}

// captureStdout redirects os.Stdout, calls f, then returns captured output.
func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	old := os.Stdout
	os.Stdout = w
	f()
	w.Close()
	os.Stdout = old
	var buf bytes.Buffer
	buf.ReadFrom(r)
	return buf.String()
}

func TestDetectInitLanguages_Go(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "go.mod")

	langs := detectInitLanguages(dir)
	if len(langs) != 1 || langs[0] != langGo {
		t.Errorf("expected [Go], got %v", langs)
	}
}

func TestDetectInitLanguages_Rust(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "Cargo.toml")

	langs := detectInitLanguages(dir)
	if len(langs) != 1 || langs[0] != langRust {
		t.Errorf("expected [Rust], got %v", langs)
	}
}

func TestDetectInitLanguages_TypeScript(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "package.json")

	langs := detectInitLanguages(dir)
	if len(langs) != 1 || langs[0] != langTypeScript {
		t.Errorf("expected [TypeScript], got %v", langs)
	}
}

func TestDetectInitLanguages_Multiple(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "go.mod")
	touch(t, dir, "package.json")

	langs := detectInitLanguages(dir)
	if len(langs) != 2 {
		t.Errorf("expected 2 languages, got %d: %v", len(langs), langs)
	}
}

func TestDetectInitLanguages_None(t *testing.T) {
	dir := t.TempDir()
	langs := detectInitLanguages(dir)
	if len(langs) != 0 {
		t.Errorf("expected no languages, got %v", langs)
	}
}

func TestDetectGoLayers_FullClean(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "cmd", "internal/handler", "internal/service", "internal/repo", "pkg")

	layers, allowed := detectGoLayers(dir)

	wantLayers := []string{"cmd", "handler", "service", "repo", "pkg"}
	if len(layers) != len(wantLayers) {
		t.Fatalf("expected %d layers, got %d: %v", len(wantLayers), len(layers), layers)
	}
	for i, name := range wantLayers {
		if layers[i].Name != name {
			t.Errorf("layer[%d]: want %q got %q", i, name, layers[i].Name)
		}
	}

	if deps := allowed["cmd"]; !containsAll(deps, "handler", "service") {
		t.Errorf("cmd allowed_deps: want handler+service, got %v", deps)
	}
	if deps := allowed["handler"]; !containsAll(deps, "service", "pkg") {
		t.Errorf("handler allowed_deps: want service+pkg, got %v", deps)
	}
	if deps := allowed["service"]; !containsAll(deps, "repo", "pkg") {
		t.Errorf("service allowed_deps: want repo+pkg, got %v", deps)
	}
	if deps := allowed["repo"]; !containsAll(deps, "pkg") {
		t.Errorf("repo allowed_deps: want pkg, got %v", deps)
	}
	if deps := allowed["pkg"]; len(deps) != 0 {
		t.Errorf("pkg allowed_deps: want empty, got %v", deps)
	}
}

func TestDetectGoLayers_CmdInternal(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "cmd", "internal")

	layers, allowed := detectGoLayers(dir)

	if len(layers) != 2 {
		t.Fatalf("expected 2 layers, got %d", len(layers))
	}
	if deps := allowed["cmd"]; !containsAll(deps, "internal") {
		t.Errorf("cmd should allow internal, got %v", deps)
	}
}

func TestDetectGoLayers_NoLayers(t *testing.T) {
	dir := t.TempDir()
	// No Go project dirs
	layers, _ := detectGoLayers(dir)
	if len(layers) != 0 {
		t.Errorf("expected no layers, got %v", layers)
	}
}

func TestDetectGoLayers_AltPaths(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "cmd", "internal/api", "internal/usecase", "internal/storage")

	layers, _ := detectGoLayers(dir)

	wantLayers := []string{"cmd", "handler", "service", "repo"}
	if len(layers) != len(wantLayers) {
		t.Fatalf("expected %d layers, got %d: %v", len(wantLayers), len(layers), layers)
	}
	// Verify the actual paths point to the alt names
	handlerLayer := findLayer(layers, "handler")
	if handlerLayer == nil || handlerLayer.Paths[0] != "internal/api" {
		t.Errorf("handler layer should use internal/api, got %v", handlerLayer)
	}
}

func TestDetectRustLayers(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "src/domain", "src/app", "src/infra")

	layers, allowed := detectRustLayers(dir)

	wantLayers := []string{"domain", "app", "infra"}
	if len(layers) != len(wantLayers) {
		t.Fatalf("expected %d layers, got %d: %v", len(wantLayers), len(layers), layers)
	}

	if deps := allowed["domain"]; len(deps) != 0 {
		t.Errorf("domain should have no deps, got %v", deps)
	}
	if deps := allowed["app"]; !containsAll(deps, "domain") {
		t.Errorf("app should depend on domain, got %v", deps)
	}
	if deps := allowed["infra"]; !containsAll(deps, "domain", "app") {
		t.Errorf("infra should depend on domain+app, got %v", deps)
	}
}

func TestDetectRustLayers_NoSrc(t *testing.T) {
	dir := t.TempDir()
	layers, _ := detectRustLayers(dir)
	if len(layers) != 0 {
		t.Errorf("expected no layers without src/, got %v", layers)
	}
}

func TestDetectTSLayers(t *testing.T) {
	dir := t.TempDir()
	mkdirs(t, dir, "src/controllers", "src/services", "src/repositories", "src/models")

	layers, allowed := detectTSLayers(dir)

	wantLayers := []string{"controller", "service", "repository", "model"}
	if len(layers) != len(wantLayers) {
		t.Fatalf("expected %d layers, got %d: %v", len(wantLayers), len(layers), layers)
	}

	if deps := allowed["controller"]; !containsAll(deps, "service", "model") {
		t.Errorf("controller should depend on service+model, got %v", deps)
	}
	if deps := allowed["model"]; len(deps) != 0 {
		t.Errorf("model should have no deps, got %v", deps)
	}
}

func TestBuildInitYAML_WithLayers(t *testing.T) {
	langs := []initLanguage{langGo}
	layers := []initLayer{
		{Name: "cmd", Paths: []string{"cmd"}},
		{Name: "service", Paths: []string{"internal/service"}},
	}
	allowed := map[string][]string{
		"cmd":     {"service"},
		"service": {},
	}

	yaml := buildInitYAML(langs, layers, allowed)

	checks := []string{
		"fan_out:",
		"threshold: 5",
		"cycles:",
		"level: taboo",
		"dip:",
		"isp:",
		"feature_envy:",
		"god_class:",
		"srp:",
		"layers:",
		"- name: cmd",
		"- name: service",
		"allowed_dependencies:",
		"cmd:",
		"- service",
		"service: []",
		"Detected language(s): Go",
	}
	for _, c := range checks {
		if !strings.Contains(yaml, c) {
			t.Errorf("YAML missing %q\n---\n%s", c, yaml)
		}
	}
}

func TestBuildInitYAML_NoLayers(t *testing.T) {
	yaml := buildInitYAML(nil, nil, nil)

	if !strings.Contains(yaml, "# No layers detected") {
		t.Errorf("expected no-layers comment, got:\n%s", yaml)
	}
	if !strings.Contains(yaml, "fan_out:") {
		t.Errorf("expected rules section, got:\n%s", yaml)
	}
}

func TestRunInit_DryRun(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "go.mod")
	mkdirs(t, dir, "cmd", "internal/service")

	initDryRun = true
	defer func() { initDryRun = false }()

	out := captureStdout(t, func() {
		if err := runInit(nil, []string{dir}); err != nil {
			t.Errorf("runInit error: %v", err)
		}
	})

	if !strings.Contains(out, "fan_out:") {
		t.Errorf("dry-run output missing rules: %s", out)
	}

	// Must NOT have written the file.
	if _, err := os.Stat(filepath.Join(dir, ".archlint.yaml")); err == nil {
		t.Error("dry-run should not create .archlint.yaml")
	}
}

func TestRunInit_WritesFile(t *testing.T) {
	dir := t.TempDir()
	touch(t, dir, "go.mod")
	mkdirs(t, dir, "cmd", "internal/handler", "internal/service")

	initDryRun = false

	out := captureStdout(t, func() {
		if err := runInit(nil, []string{dir}); err != nil {
			t.Errorf("runInit error: %v", err)
		}
	})

	_ = out

	outPath := filepath.Join(dir, ".archlint.yaml")
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf(".archlint.yaml not written: %v", err)
	}

	content := string(data)
	if !strings.Contains(content, "fan_out:") {
		t.Errorf(".archlint.yaml missing rules section: %s", content)
	}
	if !strings.Contains(content, "layers:") {
		t.Errorf(".archlint.yaml missing layers section: %s", content)
	}
}

func TestRunInit_DefaultDir(t *testing.T) {
	// With no args, uses "." - just confirm no error on current dir (archlint repo).
	initDryRun = true
	defer func() { initDryRun = false }()

	out := captureStdout(t, func() {
		if err := runInit(nil, []string{}); err != nil {
			t.Errorf("runInit with no args error: %v", err)
		}
	})

	if !strings.Contains(out, "fan_out:") {
		t.Errorf("expected YAML output, got: %s", out)
	}
}

func TestRunInit_InvalidDir(t *testing.T) {
	initDryRun = false
	err := runInit(nil, []string{"/this/does/not/exist/xyz123"})
	if err == nil {
		t.Error("expected error for non-existent directory")
	}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func containsAll(slice []string, items ...string) bool {
	set := make(map[string]bool, len(slice))
	for _, s := range slice {
		set[s] = true
	}
	for _, item := range items {
		if !set[item] {
			return false
		}
	}
	return true
}

func findLayer(layers []initLayer, name string) *initLayer {
	for i := range layers {
		if layers[i].Name == name {
			return &layers[i]
		}
	}
	return nil
}
