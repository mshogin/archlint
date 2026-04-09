package cli

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var initDryRun bool

var initCmd = &cobra.Command{
	Use:   "init [directory]",
	Short: "Generate a .archlint.yaml config by auto-detecting project structure",
	Long: `Scan the target directory, detect the project language and architecture
layers, then generate a tailored .archlint.yaml configuration.

Supported languages:
  Go         (go.mod)
  Rust        (Cargo.toml)
  TypeScript  (package.json)

Detected layer patterns:
  Go:         cmd / internal/{handler,service,repo} / pkg
  Rust:       src/{domain,app,infra}
  TypeScript: src/{controllers,services,repositories,models}

Use --dry-run to preview the generated YAML without writing to disk.

Examples:
  archlint init .
  archlint init ./myproject
  archlint init . --dry-run`,
	Args: cobra.MaximumNArgs(1),
	RunE: runInit,
}

func init() {
	initCmd.Flags().BoolVar(&initDryRun, "dry-run", false, "Print generated YAML to stdout without writing .archlint.yaml")
	rootCmd.AddCommand(initCmd)
}

// ---------------------------------------------------------------------------
// Internal types
// ---------------------------------------------------------------------------

type initLanguage string

const (
	langGo         initLanguage = "Go"
	langRust        initLanguage = "Rust"
	langTypeScript  initLanguage = "TypeScript"
)

type initLayer struct {
	Name  string
	Paths []string
}

// ---------------------------------------------------------------------------
// Command handler
// ---------------------------------------------------------------------------

func runInit(_ *cobra.Command, args []string) error {
	dir := "."
	if len(args) == 1 {
		dir = args[0]
	}

	absDir, err := filepath.Abs(dir)
	if err != nil {
		return fmt.Errorf("cannot resolve directory: %w", err)
	}

	if _, err := os.Stat(absDir); os.IsNotExist(err) {
		return fmt.Errorf("%w: %s", errDirNotExist, absDir)
	}

	langs := detectInitLanguages(absDir)
	layers, allowedDeps := detectInitLayers(absDir, langs)
	yaml := buildInitYAML(langs, layers, allowedDeps)

	if initDryRun {
		fmt.Print(yaml)
		return nil
	}

	outPath := filepath.Join(absDir, ".archlint.yaml")
	if err := os.WriteFile(outPath, []byte(yaml), 0644); err != nil { //nolint:gosec
		return fmt.Errorf("failed to write %s: %w", outPath, err)
	}

	fmt.Printf("Created %s\n", outPath)
	if len(langs) > 0 {
		names := make([]string, len(langs))
		for i, l := range langs {
			names[i] = string(l)
		}
		fmt.Printf("Detected language(s): %s\n", strings.Join(names, ", "))
	}
	if len(layers) > 0 {
		layerNames := make([]string, len(layers))
		for i, l := range layers {
			layerNames[i] = l.Name
		}
		fmt.Printf("Detected layers: %s\n", strings.Join(layerNames, " -> "))
	}

	return nil
}

// ---------------------------------------------------------------------------
// Language detection
// ---------------------------------------------------------------------------

func detectInitLanguages(dir string) []initLanguage {
	var langs []initLanguage
	if fileExists(filepath.Join(dir, "go.mod")) {
		langs = append(langs, langGo)
	}
	if fileExists(filepath.Join(dir, "Cargo.toml")) {
		langs = append(langs, langRust)
	}
	if fileExists(filepath.Join(dir, "package.json")) {
		langs = append(langs, langTypeScript)
	}
	return langs
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func dirExists(base, sub string) bool {
	info, err := os.Stat(filepath.Join(base, sub))
	return err == nil && info.IsDir()
}

// ---------------------------------------------------------------------------
// Layer detection
// ---------------------------------------------------------------------------

func detectInitLayers(dir string, langs []initLanguage) ([]initLayer, map[string][]string) {
	for _, lang := range langs {
		switch lang {
		case langGo:
			layers, allowed := detectGoLayers(dir)
			if len(layers) > 0 {
				return layers, allowed
			}
		case langRust:
			layers, allowed := detectRustLayers(dir)
			if len(layers) > 0 {
				return layers, allowed
			}
		case langTypeScript:
			layers, allowed := detectTSLayers(dir)
			if len(layers) > 0 {
				return layers, allowed
			}
		}
	}
	return nil, nil
}

func detectGoLayers(dir string) ([]initLayer, map[string][]string) {
	hasCmd := dirExists(dir, "cmd")
	hasInternal := dirExists(dir, "internal")
	hasPkg := dirExists(dir, "pkg")

	hasHandler := dirExists(dir, "internal/handler") ||
		dirExists(dir, "internal/api") ||
		dirExists(dir, "internal/delivery")
	hasService := dirExists(dir, "internal/service") ||
		dirExists(dir, "internal/usecase") ||
		dirExists(dir, "internal/domain")
	hasRepo := dirExists(dir, "internal/repo") ||
		dirExists(dir, "internal/repository") ||
		dirExists(dir, "internal/storage")

	if hasCmd && hasInternal && (hasHandler || hasService || hasRepo) {
		var layers []initLayer
		allowed := make(map[string][]string)

		layers = append(layers, initLayer{Name: "cmd", Paths: []string{"cmd"}})

		if hasHandler {
			p := pickFirst(dir, "internal/handler", "internal/api", "internal/delivery")
			layers = append(layers, initLayer{Name: "handler", Paths: []string{p}})
		}
		if hasService {
			p := pickFirst(dir, "internal/service", "internal/usecase", "internal/domain")
			layers = append(layers, initLayer{Name: "service", Paths: []string{p}})
		}
		if hasRepo {
			p := pickFirst(dir, "internal/repo", "internal/repository", "internal/storage")
			layers = append(layers, initLayer{Name: "repo", Paths: []string{p}})
		}
		if hasPkg {
			layers = append(layers, initLayer{Name: "pkg", Paths: []string{"pkg"}})
		}

		hasH := layerExists(layers, "handler")
		hasS := layerExists(layers, "service")
		hasR := layerExists(layers, "repo")
		hasP := layerExists(layers, "pkg")

		var cmdDeps []string
		if hasH {
			cmdDeps = append(cmdDeps, "handler")
		}
		if hasS {
			cmdDeps = append(cmdDeps, "service")
		}
		allowed["cmd"] = cmdDeps

		if hasH {
			var deps []string
			if hasS {
				deps = append(deps, "service")
			}
			if hasP {
				deps = append(deps, "pkg")
			}
			allowed["handler"] = deps
		}
		if hasS {
			var deps []string
			if hasR {
				deps = append(deps, "repo")
			}
			if hasP {
				deps = append(deps, "pkg")
			}
			allowed["service"] = deps
		}
		if hasR {
			var deps []string
			if hasP {
				deps = append(deps, "pkg")
			}
			allowed["repo"] = deps
		}
		if hasP {
			allowed["pkg"] = []string{}
		}

		return layers, allowed
	}

	if hasCmd && hasInternal {
		layers := []initLayer{
			{Name: "cmd", Paths: []string{"cmd"}},
			{Name: "internal", Paths: []string{"internal"}},
		}
		allowed := map[string][]string{
			"cmd":      {"internal"},
			"internal": {},
		}
		return layers, allowed
	}

	return nil, nil
}

func detectRustLayers(dir string) ([]initLayer, map[string][]string) {
	src := filepath.Join(dir, "src")
	if !dirExists(dir, "src") {
		return nil, nil
	}

	hasDomain := dirExists(src, "domain")
	hasApp := dirExists(src, "app") || dirExists(src, "application")
	hasInfra := dirExists(src, "infra") || dirExists(src, "infrastructure")

	if !hasDomain && !hasApp && !hasInfra {
		return nil, nil
	}

	var layers []initLayer
	allowed := make(map[string][]string)

	if hasDomain {
		layers = append(layers, initLayer{Name: "domain", Paths: []string{"src/domain"}})
	}
	if hasApp {
		p := "src/app"
		if dirExists(src, "application") {
			p = "src/application"
		}
		layers = append(layers, initLayer{Name: "app", Paths: []string{p}})
	}
	if hasInfra {
		p := "src/infra"
		if dirExists(src, "infrastructure") {
			p = "src/infrastructure"
		}
		layers = append(layers, initLayer{Name: "infra", Paths: []string{p}})
	}

	hasD := layerExists(layers, "domain")
	hasA := layerExists(layers, "app")
	hasI := layerExists(layers, "infra")

	if hasD {
		allowed["domain"] = []string{}
	}
	if hasA {
		var deps []string
		if hasD {
			deps = append(deps, "domain")
		}
		allowed["app"] = deps
	}
	if hasI {
		var deps []string
		if hasD {
			deps = append(deps, "domain")
		}
		if hasA {
			deps = append(deps, "app")
		}
		allowed["infra"] = deps
	}

	return layers, allowed
}

func detectTSLayers(dir string) ([]initLayer, map[string][]string) {
	base := dir
	prefix := ""
	if dirExists(dir, "src") {
		base = filepath.Join(dir, "src")
		prefix = "src/"
	}

	hasControllers := dirExists(base, "controllers") || dirExists(base, "routes")
	hasServices := dirExists(base, "services")
	hasModels := dirExists(base, "models") || dirExists(base, "entities")
	hasRepos := dirExists(base, "repositories") || dirExists(base, "repos")

	if !hasControllers && !hasServices && !hasModels {
		return nil, nil
	}

	var layers []initLayer
	allowed := make(map[string][]string)

	if hasControllers {
		p := prefix + "controllers"
		if dirExists(base, "routes") {
			p = prefix + "routes"
		}
		layers = append(layers, initLayer{Name: "controller", Paths: []string{p}})
	}
	if hasServices {
		layers = append(layers, initLayer{Name: "service", Paths: []string{prefix + "services"}})
	}
	if hasRepos {
		p := prefix + "repositories"
		if dirExists(base, "repos") {
			p = prefix + "repos"
		}
		layers = append(layers, initLayer{Name: "repository", Paths: []string{p}})
	}
	if hasModels {
		p := prefix + "models"
		if dirExists(base, "entities") {
			p = prefix + "entities"
		}
		layers = append(layers, initLayer{Name: "model", Paths: []string{p}})
	}

	hasC := layerExists(layers, "controller")
	hasS := layerExists(layers, "service")
	hasR := layerExists(layers, "repository")
	hasM := layerExists(layers, "model")

	if hasC {
		var deps []string
		if hasS {
			deps = append(deps, "service")
		}
		if hasM {
			deps = append(deps, "model")
		}
		allowed["controller"] = deps
	}
	if hasS {
		var deps []string
		if hasR {
			deps = append(deps, "repository")
		}
		if hasM {
			deps = append(deps, "model")
		}
		allowed["service"] = deps
	}
	if hasR {
		var deps []string
		if hasM {
			deps = append(deps, "model")
		}
		allowed["repository"] = deps
	}
	if hasM {
		allowed["model"] = []string{}
	}

	return layers, allowed
}

// ---------------------------------------------------------------------------
// YAML generation
// ---------------------------------------------------------------------------

func buildInitYAML(langs []initLanguage, layers []initLayer, allowedDeps map[string][]string) string {
	var sb strings.Builder

	sb.WriteString("# .archlint.yaml - generated by `archlint init`\n")
	if len(langs) > 0 {
		names := make([]string, len(langs))
		for i, l := range langs {
			names[i] = string(l)
		}
		sb.WriteString("# Detected language(s): " + strings.Join(names, ", ") + "\n")
	}
	sb.WriteString("\n")

	// Rules section.
	sb.WriteString("rules:\n")
	sb.WriteString("  fan_out:\n")
	sb.WriteString("    enabled: true\n")
	sb.WriteString("    threshold: 5\n")
	sb.WriteString("    level: telemetry\n")
	sb.WriteString("  cycles:\n")
	sb.WriteString("    enabled: true\n")
	sb.WriteString("    level: taboo\n")
	sb.WriteString("  dip:\n")
	sb.WriteString("    enabled: true\n")
	sb.WriteString("    level: telemetry\n")
	sb.WriteString("  isp:\n")
	sb.WriteString("    enabled: true\n")
	sb.WriteString("    threshold: 5\n")
	sb.WriteString("    level: telemetry\n")
	sb.WriteString("  feature_envy:\n")
	sb.WriteString("    enabled: true\n")
	sb.WriteString("    level: telemetry\n")
	sb.WriteString("  god_class:\n")
	sb.WriteString("    enabled: true\n")
	sb.WriteString("    level: telemetry\n")
	sb.WriteString("  srp:\n")
	sb.WriteString("    enabled: true\n")
	sb.WriteString("    level: telemetry\n")
	sb.WriteString("\n")

	if len(layers) > 0 {
		sb.WriteString("layers:\n")
		for _, l := range layers {
			sb.WriteString(fmt.Sprintf("  - name: %s\n", l.Name))
			sb.WriteString("    paths:\n")
			for _, p := range l.Paths {
				sb.WriteString(fmt.Sprintf("      - %s\n", p))
			}
		}
		sb.WriteString("\n")

		sb.WriteString("allowed_dependencies:\n")
		// Emit in layer declaration order for deterministic output.
		for _, l := range layers {
			deps, ok := allowedDeps[l.Name]
			if !ok {
				deps = []string{}
			}
			if len(deps) == 0 {
				sb.WriteString(fmt.Sprintf("  %s: []\n", l.Name))
			} else {
				sb.WriteString(fmt.Sprintf("  %s:\n", l.Name))
				for _, d := range deps {
					sb.WriteString(fmt.Sprintf("    - %s\n", d))
				}
			}
		}
	} else {
		sb.WriteString("# No layers detected - add them manually if needed:\n")
		sb.WriteString("# layers:\n")
		sb.WriteString("#   - name: handler\n")
		sb.WriteString("#     paths:\n")
		sb.WriteString("#       - internal/handler\n")
		sb.WriteString("#\n")
		sb.WriteString("# allowed_dependencies:\n")
		sb.WriteString("#   handler:\n")
		sb.WriteString("#     - service\n")
	}

	return sb.String()
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func layerExists(layers []initLayer, name string) bool {
	for _, l := range layers {
		if l.Name == name {
			return true
		}
	}
	return false
}

// pickFirst returns the first path (relative to dir) that exists as a directory.
func pickFirst(dir string, candidates ...string) string {
	for _, c := range candidates {
		if dirExists(dir, c) {
			return c
		}
	}
	return candidates[0]
}
