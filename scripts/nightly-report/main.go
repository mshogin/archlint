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
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/mshogin/archlint/internal/mcp"
	"gopkg.in/yaml.v3"
)

const healthVersion = "v2"

const healthNote = "health v2 = ERROR-class provable core (соундное ядро: SCC/layering/dead-code/ISP/" +
	"forbidden/deprecated/ghost — прошло ворота + self-проверку). WARN-слой (DIP/SRP/clone/ρ) НЕ в формуле " +
	"health: precision очищен (DTO-фильтр + INFO-downgrades), но verified-WARNING доминируют по ОБЪЁМУ — " +
	"линейный вес не масштабируется (десятки WARN -> 0%, ложно-плохой дашборд). Нужна НЕЛИНЕЙНАЯ/" +
	"нормализованная формула (WARN/KLOC, density, log-scale) — отдельный продуманный инкремент (backlog). " +
	"Advanced research-метрики (центральности/спектр — Тир3 музей) и OCP/LSP-эвристики (несоундны) УБРАНЫ " +
	"с дашборда. health% = по блокирующим доказуемым дефектам, НЕ сравним напрямую с прежней (Python) " +
	"методикой. Колонка Warnings — наблюдаема отдельно (категории), в health НЕ входит."

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
	Owner    string `json:"owner"`
	Name     string `json:"name"`
	Language string `json:"language"`
	Health   int    `json:"health"`
	Errors   int    `json:"errors"`
	Warnings int    `json:"warnings"`
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
		// health v2 = ERROR-class only (вариант A, закреплён): WARN-слой исключён из формулы.
		// Golden-замер показал: даже после precision-очистки (DTO-фильтр + INFO-downgrades)
		// verified-WARNING доминируют по ОБЪЁМУ -> линейный вес обнуляет health на любом среднем
		// репо (ложно-плохой дашборд). WARN-слой требует НЕЛИНЕЙНОЙ формулы (density/KLOC) -> backlog.
		// warns считаем для отдельной наблюдаемой колонки, в health НЕ включаем.
		health := 100 - errs*5
		if health < 0 {
			health = 0
		}

		out.Repos = append(out.Repos, repoHealth{
			Owner: owner, Name: name, Language: repo.Language,
			Health: health, Errors: errs, Warnings: warns,
		})

		fmt.Printf("  health=%d%% (v2), errors=%d, warnings=%d\n", health, errs, warns)
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
	b.WriteString("| Repository | Language | Health | Errors | Warnings |\n")
	b.WriteString("|-----------|----------|--------|--------|----------|\n")

	for _, r := range repos {
		link := fmt.Sprintf("[%s/%s](%s-%s/)", r.Owner, r.Name, r.Owner, r.Name)
		fmt.Fprintf(&b, "| %s | %s | %s | %d | %d |\n", link, r.Language, healthBar(r.Health), r.Errors, r.Warnings)
	}

	b.WriteString("\n## How it works\n\n")
	b.WriteString("Every night at 3:00 UTC, archlint clones each monitored repository, builds the " +
		"architecture dependency graph (Go engine), and runs the soundness-gated detectors " +
		"(SCC/cycles, layering, dead-code, ISP, DIP/SRP) — the provable architectural core.\n\n")
	fmt.Fprintf(&b, "Health score (`%s`): `100 - (errors * 5)`, minimum 0 (ERROR-class provable core; warnings informational).\n\n", healthVersion)
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
	fmt.Fprintf(&b, "- WARNING (DIP/SRP/ISP/clone): **%d**\n\n", r.Warnings)
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
