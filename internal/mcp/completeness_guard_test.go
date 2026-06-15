package mcp

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// СТРАЖ ПОЛНОТЫ (веха №6): ∀ Kind, эмитируемый детекторами, имеет severity-вердикт в ЕДИНОМ
// реестре (RegisteredKinds). Ловит СПЯЩУЮ метрику ДО следующего чужого репо: новый детектор,
// добавивший Kind без записи в severity_class, краснит этот тест (а не молча выстреливает
// неклассифицированным на чужом коде). Self-проверка полноты — как delta(collect(X),collect(X))=∅
// для дельты, но для каталога severity.
//
// Извлекает Kind-литералы из исходников детекторов в ДВУХ формах:
//   - struct-литерал:  Violation{Kind: "kind-name"}
//   - const-объявление: KindXxx = "kind-name"
// и проверяет каждый против RegisteredKinds().
func TestCompleteness_AllEmittedKindsHaveVerdict(t *testing.T) {
	reg := RegisteredKinds()

	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}

	reLit := regexp.MustCompile(`Kind:\s*"([a-z][a-z0-9-]+)"`)
	reConst := regexp.MustCompile(`Kind\w*\s*=\s*"([a-z][a-z0-9-]+)"`)

	// non-gate Kind'ы: НЕ участвуют в severity-гейте (сигналы/исключения), вердикт не требуется.
	// isp-external-narrow — внешний контракт, по дизайну НЕ в реестре (всегда WARNING на гейте отброшен).
	whitelist := map[string]bool{"isp-external-narrow": true}

	emitted := make(map[string]bool)

	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}

		data, err := os.ReadFile(f) //nolint:gosec // тестовое чтение исходников пакета
		if err != nil {
			t.Fatal(err)
		}

		src := string(data)
		for _, re := range []*regexp.Regexp{reLit, reConst} {
			for _, m := range re.FindAllStringSubmatch(src, -1) {
				emitted[m[1]] = true
			}
		}
	}

	if len(emitted) == 0 {
		t.Fatal("не извлечено ни одного Kind — регэксп/glob сломан")
	}

	for kind := range emitted {
		if whitelist[kind] {
			continue
		}

		if !reg[kind] {
			t.Errorf("СПЯЩАЯ МЕТРИКА: Kind %q эмитируется детектором, но НЕ имеет severity-вердикта "+
				"в едином реестре (severity_class). Добавь запись (ERROR/WARNING/INFO) — иначе выстрелит "+
				"неклассифицированным на чужом репо.", kind)
		}
	}
}
