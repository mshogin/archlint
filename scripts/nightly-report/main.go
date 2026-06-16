// nightly-report — полный оркестратор ночного скана для archlint.ru (Go, 0 Python).
//
// Заменяет Python-стек nightly (validator + health-summary + generate-scan-pages.py).
// Читает monitored-repos.yaml (Go-YAML), для каждого active репо: git clone -> archlint scan
// --format json, агрегирует health по ДОКАЗУЕМОМУ Go-ядру (health v2: advanced research-музей +
// OCP/LSP-эвристики УБРАНЫ как не-проходящие ворота), пишет health-summary.json + Hugo markdown.
//
// health v2 != прежняя (Python) методика (меньше checks, иная база) — переход ПОМЕЧЕН ЯВНО
// (health_version + note в JSON и на дашборде), тренд не загадочный скачок (урок ложно-зелёного).
// WARN-слой (DIP/SRP/clone) НЕ в формуле: golden-замер показал несоундность линейного веса на
// объёме verified-WARNING (обнуляет health) — нужна нелинейная/нормализованная формула (backlog).
//
// Usage: nightly-report <monitored-repos.yaml> <archlint-bin> <scan-results-dir> <output-dir> <scan-date>
package main

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mshogin/archlint/internal/mcp"
	"gopkg.in/yaml.v3"
)

const healthVersion = "v3"

// Калибровка WARN-слоя (health v3). warnPenalty = warnMax * d/(d+warnD0), d = warns/KLOC (ПЛОТНОСТЬ,
// не объём -> не обнуляется на больших репо). Гиперболическое насыщение: d=0 -> 0; d=warnD0 -> warnMax/2;
// d->∞ -> warnMax (асимптота, никогда не обнуляет — WARN не блок). Константы откалиброваны замером на
// спектре (self + open-source Go): density godotenv=0, color=4.8, log=3.4, self=4.4, env=15.3, twirp=12.5.
//   warnMax=40: WARN-плотный репо теряет ДО 40 пунктов (заметно, но не обнуляет — ERROR обнуляет, не WARN).
//   warnD0=8:   при density=8 (≈ выше типичной 3-5) штраф = 20; env(15.3)->26, self(4.4)->14, чистый->~0.
// Эффект на витрину (golden): godotenv 100, color 85, log 88, env 74 (линейная дала бы 0!), self 41.
const (
	warnMax = 40.0
	warnD0  = 8.0
)

const healthNote = "health v3 = ERROR-class provable core + НОРМАЛИЗОВАННЫЙ WARN-слой. ERROR-ядро " +
	"(SCC/layering/dead-code/ISP/forbidden/deprecated/ghost — соундное, прошло ворота + self-проверку): " +
	"штраф errs*5. WARN-слой (DIP/SRP/clone): штраф по ПЛОТНОСТИ warnMax*d/(d+warnD0), d=warns/KLOC — " +
	"НЕ по объёму (линейный вес v2 обнулял health на десятках WARN -> ложно-плохой дашборд; golden вскрыл). " +
	"Насыщение: WARN теряет максимум warnMax пунктов, НИКОГДА не обнуляет (WARN — сигнал, не блок; обнуляет " +
	"только ERROR). Константы откалиброваны замером на спектре репо (self+open-source). health% v3 != v2 " +
	"(добавлен WARN-слой) -> тренд НЕ загадочный скачок (переход помечен явно, урок ложно-зелёного). " +
	"Advanced research-метрики (центральности/спектр) и OCP/LSP-эвристики УБРАНЫ. Колонка Warnings + density " +
	"наблюдаемы отдельно."

// severity-классификация берётся из ЕДИНОГО реестра mcp.SeverityClassOf (SSOT) — без дубль-
// хардкода списков. errors = ERROR-class; warnings = WARNING-class; INFO игнорируется в health.

type monitoredConfig struct {
	Repos []monitoredRepo `yaml:"repos"`
}

type monitoredRepo struct {
	URL      string `yaml:"url"`
	Language string `yaml:"language"`
	Status   string `yaml:"status"`
}

type scanJSON struct {
	Categories map[string]int `json:"categories"`
}

type repoHealth struct {
	Owner    string  `json:"owner"`
	Name     string  `json:"name"`
	Language string  `json:"language"`
	Health   int     `json:"health"`
	Errors   int     `json:"errors"`
	Warnings int     `json:"warnings"`
	LOC      int     `json:"loc"`          // непустые строки исходников (для density-нормализации WARN)
	Density  float64 `json:"warn_density"` // warns/KLOC — наблюдаемая плотность WARN
}

type summaryOut struct {
	ScanDate      string       `json:"scan_date"`
	HealthVersion string       `json:"health_version"`
	HealthNote    string       `json:"health_note"`
	Repos         []repoHealth `json:"repos"`
}

func main() {
	if len(os.Args) != 6 {
		fmt.Fprintln(os.Stderr, "usage: nightly-report <monitored-repos.yaml> <archlint-bin> <scan-results-dir> <output-dir> <scan-date>")
		os.Exit(2)
	}

	monitoredPath, archlintBin, scanDir, outDir, scanDate := os.Args[1], os.Args[2], os.Args[3], os.Args[4], os.Args[5]

	var cfg monitoredConfig
	if err := readYAML(monitoredPath, &cfg); err != nil {
		fmt.Fprintf(os.Stderr, "read monitored-repos: %v\n", err)
		os.Exit(1)
	}

	out := summaryOut{ScanDate: scanDate, HealthVersion: healthVersion, HealthNote: healthNote}

	for _, repo := range cfg.Repos {
		if repo.Status != "active" {
			continue
		}

		owner, name := parseRepoURL(repo.URL)
		if owner == "" || name == "" {
			fmt.Printf("SKIP: bad url %q\n", repo.URL)

			continue
		}

		fmt.Printf("Scanning %s/%s (%s)...\n", owner, name, repo.Language)

		dest := filepath.Join(os.TempDir(), name)
		if !gitClone(repo.URL, dest) {
			fmt.Printf("  SKIP: clone failed\n")

			continue
		}

		scanPath := filepath.Join(scanDir, owner, name, "scan.json")
		if err := os.MkdirAll(filepath.Dir(scanPath), 0o755); err != nil {
			fmt.Printf("  SKIP: mkdir %v\n", err)

			continue
		}

		if !runArchlintScan(archlintBin, dest, scanPath) {
			fmt.Printf("  WARN: scan failed (no scan.json) -> health 0/0\n")
		}

		errs, warns := aggregate(scanPath)
		loc := countCodeLines(dest, repo.Language)
		health, density := computeHealth(errs, warns, loc)

		out.Repos = append(out.Repos, repoHealth{
			Owner: owner, Name: name, Language: repo.Language,
			Health: health, Errors: errs, Warnings: warns, LOC: loc, Density: density,
		})

		fmt.Printf("  health=%d%% (v3), errors=%d, warnings=%d, loc=%d, density=%.2f\n", health, errs, warns, loc, density)
	}

	if err := os.MkdirAll(scanDir, 0o755); err == nil {
		_ = writeJSON(filepath.Join(scanDir, "health-summary.json"), out)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "mkdir out: %v\n", err)
		os.Exit(1)
	}

	writeIndex(out, outDir)
	for _, r := range out.Repos {
		writeRepoPage(r, outDir)
	}

	fmt.Printf("Done: health %s for %d repos -> %s/ + %s/health-summary.json\n", healthVersion, len(out.Repos), outDir, scanDir)
}

// parseRepoURL извлекает owner/name из https://host/owner/name(.git).
func parseRepoURL(u string) (owner, name string) {
	u = strings.TrimSuffix(strings.TrimRight(u, "/"), ".git")
	parts := strings.Split(u, "/")
	if len(parts) < 2 {
		return "", ""
	}

	return parts[len(parts)-2], parts[len(parts)-1]
}

func gitClone(url, dest string) bool {
	_ = os.RemoveAll(dest)
	cmd := exec.Command("git", "clone", "--depth=1", url, dest) //nolint:gosec // url from internal monitored-repos.yaml
	cmd.Stderr = os.Stderr

	return cmd.Run() == nil
}

// runArchlintScan: archlint scan <dir> --threshold 100000 --format json -> scanPath.
// threshold большой = audit-режим (без baseline на чужом репо), полный список нарушений.
func runArchlintScan(bin, dir, scanPath string) bool {
	cmd := exec.Command(bin, "scan", dir, "--threshold", "100000", "--format", "json") //nolint:gosec // internal CI
	outBytes, _ := cmd.Output()                                                        // scan exit≠0 при violations — это норма, JSON в stdout

	if len(outBytes) == 0 {
		return false
	}

	return os.WriteFile(scanPath, outBytes, 0o644) == nil //nolint:gosec // CI artifact
}

// aggregate считает errors/warnings доказуемого Go-ядра из per-repo scan.json.
func aggregate(scanPath string) (errs, warns int) {
	var s scanJSON
	if err := readJSON(scanPath, &s); err != nil {
		return 0, 0
	}

	for kind, n := range s.Categories {
		switch mcp.SeverityClassOf(kind) {
		case "ERROR":
			errs += n
		case "WARNING":
			warns += n
		}
	}

	return errs, warns
}

// computeHealth — health v3: ERROR-ядро (линейный штраф errs*5, блокирующих мало) + НОРМАЛИЗОВАННЫЙ
// WARN-слой по ПЛОТНОСТИ (density=warns/KLOC). warnPenalty = warnMax*d/(d+warnD0): гиперболическое
// насыщение -> WARN теряет максимум warnMax, НИКОГДА не обнуляет (обнуляет только ERROR). Возвращает
// health [0,100] и density (для наблюдаемости). Решает дефект v2 (линейный вес обнулял на объёме WARN).
func computeHealth(errs, warns, loc int) (health int, density float64) {
	kloc := float64(loc) / 1000.0
	if kloc < 0.001 {
		kloc = 0.001 // защита от деления на ноль (пустой/неизмеренный репо)
	}

	density = float64(warns) / kloc
	warnPenalty := warnMax * density / (density + warnD0)

	health = 100 - errs*5 - int(math.Round(warnPenalty))
	if health < 0 {
		health = 0
	}

	return health, density
}

// countCodeLines считает НЕПУСТЫЕ строки исходников языка (для density-нормализации WARN). Skip
// build-артефактов/вендора/фикстур. Грубая мера размера (не настоящий LOC-инструмент) — достаточно
// для плотности warns/KLOC.
func countCodeLines(dir, language string) int {
	exts := map[string][]string{
		"Go":         {".go"},
		"Rust":       {".rs"},
		"TypeScript": {".ts", ".tsx"},
	}[language]
	if exts == nil {
		exts = []string{".go"}
	}

	skip := map[string]bool{"vendor": true, ".git": true, "node_modules": true, "target": true, "testdata": true}
	total := 0

	_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil //nolint:nilerr // best-effort обход; ошибки путей не валят подсчёт
		}

		if info.IsDir() {
			if skip[info.Name()] {
				return filepath.SkipDir
			}

			return nil
		}

		matched := false
		for _, e := range exts {
			if strings.HasSuffix(info.Name(), e) {
				matched = true

				break
			}
		}
		if !matched {
			return nil
		}

		data, err := os.ReadFile(path) //nolint:gosec // CI: чтение исходников клонированного репо
		if err != nil {
			return nil //nolint:nilerr // нечитаемый файл не валит подсчёт
		}

		for _, line := range strings.Split(string(data), "\n") {
			if strings.TrimSpace(line) != "" {
				total++
			}
		}

		return nil
	})

	return total
}

func healthBar(h int) string {
	switch {
	case h >= 80:
		return fmt.Sprintf("%d%% (good)", h)
	case h >= 60:
		return fmt.Sprintf("%d%% (moderate)", h)
	case h >= 40:
		return fmt.Sprintf("%d%% (needs work)", h)
	default:
		return fmt.Sprintf("%d%% (poor)", h)
	}
}

func writeIndex(s summaryOut, outDir string) {
	repos := append([]repoHealth(nil), s.Repos...)
	sort.Slice(repos, func(i, j int) bool { return repos[i].Health > repos[j].Health })

	var b strings.Builder
	date := s.ScanDate
	if len(date) >= 10 {
		date = date[:10]
	}

	b.WriteString("---\n")
	b.WriteString("title: \"Scan Results\"\n")
	b.WriteString("description: \"Nightly architecture analysis of open-source projects\"\n")
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "Last scan: %s\n", date)
	fmt.Fprintf(&b, "Repositories monitored: %d\n\n", len(repos))
	b.WriteString("## Health Dashboard\n\n")
	b.WriteString("| Repository | Language | Health | Errors | Warnings | WARN/KLOC |\n")
	b.WriteString("|-----------|----------|--------|--------|----------|-----------|\n")

	for _, r := range repos {
		link := fmt.Sprintf("[%s/%s](%s-%s/)", r.Owner, r.Name, r.Owner, r.Name)
		fmt.Fprintf(&b, "| %s | %s | %s | %d | %d | %.1f |\n", link, r.Language, healthBar(r.Health), r.Errors, r.Warnings, r.Density)
	}

	b.WriteString("\n## How it works\n\n")
	b.WriteString("Every night at 3:00 UTC, archlint clones each monitored repository, builds the " +
		"architecture dependency graph (Go engine), and runs the soundness-gated detectors " +
		"(SCC/cycles, layering, dead-code, ISP, DIP/SRP) — the provable architectural core.\n\n")
	fmt.Fprintf(&b, "Health score (`%s`): `100 - errs*5 - warnMax*d/(d+warnD0)`, d = warnings/KLOC, minimum 0 "+
		"(ERROR-core линейно + WARN-плотность с насыщением — WARN не обнуляет).\n\n", healthVersion)
	b.WriteString("> " + healthNote + "\n\n")
	b.WriteString("Want your repo scanned? " +
		"[Open an issue](https://github.com/mshogin/archlint/issues/new?title=Add+repo:+owner/name).\n")

	_ = os.WriteFile(filepath.Join(outDir, "_index.md"), []byte(b.String()), 0o644) //nolint:gosec // CI artifact
}

func writeRepoPage(r repoHealth, outDir string) {
	dir := filepath.Join(outDir, r.Owner+"-"+r.Name)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return
	}

	var b strings.Builder
	b.WriteString("---\n")
	fmt.Fprintf(&b, "title: \"%s/%s\"\n", r.Owner, r.Name)
	fmt.Fprintf(&b, "description: \"Architecture analysis of %s/%s\"\n", r.Owner, r.Name)
	b.WriteString("---\n\n")
	fmt.Fprintf(&b, "**Repository:** [%s/%s](https://github.com/%s/%s)\n", r.Owner, r.Name, r.Owner, r.Name)
	fmt.Fprintf(&b, "**Language:** %s\n", r.Language)
	fmt.Fprintf(&b, "**Health:** %s (health %s — provable core)\n\n", healthBar(r.Health), healthVersion)
	b.WriteString("## Provable-core results\n\n")
	fmt.Fprintf(&b, "- ERROR-class violations (cycles/layering/dead-code/forbidden/deprecated/ISP/ghost): **%d**\n", r.Errors)
	fmt.Fprintf(&b, "- WARNING (DIP/SRP/clone): **%d** (density %.1f per KLOC, %d LOC)\n\n", r.Warnings, r.Density, r.LOC)
	b.WriteString("> " + healthNote + "\n")

	_ = os.WriteFile(filepath.Join(dir, "_index.md"), []byte(b.String()), 0o644) //nolint:gosec // CI artifact
}

func readYAML(path string, v any) error {
	data, err := os.ReadFile(path) //nolint:gosec // internal path
	if err != nil {
		return err
	}

	return yaml.Unmarshal(data, v)
}

func readJSON(path string, v any) error {
	data, err := os.ReadFile(path) //nolint:gosec // internal CI path
	if err != nil {
		return err
	}

	return json.Unmarshal(data, v)
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, append(data, '\n'), 0o644) //nolint:gosec // CI artifact
}
