package archlintcfg

import (
	"os"
	"path/filepath"
	"testing"
)

// exclude_tests: по умолчанию true; false в конфиге выключает; флаг CLI переопределяет
// конфиг только если задан явно.
func TestExcludeTests(t *testing.T) {
	def := Default()
	if !def.TestsExcluded() {
		t.Fatal("default must exclude tests")
	}

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, ".archlint.yaml"), []byte("exclude_tests: false\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	cfg := Load(dir)
	if cfg.TestsExcluded() {
		t.Fatal("exclude_tests: false must be honoured")
	}

	cfg.ApplyExcludeTestsFlag(false, true)
	if cfg.TestsExcluded() {
		t.Fatal("flag not set explicitly must not override config")
	}

	cfg.ApplyExcludeTestsFlag(true, true)
	if !cfg.TestsExcluded() {
		t.Fatal("explicit flag must override config")
	}
}
