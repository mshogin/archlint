package analyzer

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/mshogin/archlint/internal/model"
)

// RustAnalyzer analyzes Rust projects and builds dependency graphs.
type RustAnalyzer struct {
	modules  map[string]*RustModule
	crates   map[string]*CrateInfo
	nodes    []model.Node
	edges    []model.Edge
}

// RustModule represents a Rust module (file or directory).
type RustModule struct {
	Name       string
	Path       string
	Parent     string
	Visibility string // pub, pub(crate), private
	Uses       []string
	ModDecls   []string
	Structs    []RustStruct
	Traits     []RustTrait
	Functions  []string
}

// RustStruct represents a Rust struct.
type RustStruct struct {
	Name       string
	Visibility string
	Fields     int
	ImplTraits []string
}

// RustTrait represents a Rust trait.
type RustTrait struct {
	Name    string
	Methods int
}

// CrateInfo represents a crate from Cargo.toml.
type CrateInfo struct {
	Name         string
	Version      string
	Dependencies []string
	Path         string
}

// NewRustAnalyzer creates a new Rust analyzer.
func NewRustAnalyzer() *RustAnalyzer {
	return &RustAnalyzer{
		modules: make(map[string]*RustModule),
		crates:  make(map[string]*CrateInfo),
	}
}

// Analyze scans a Rust project directory and builds the architecture graph.
func (ra *RustAnalyzer) Analyze(dir string) (*model.Graph, error) {
	// Step 1: Parse Cargo.toml for crate dependencies
	cargoPath := filepath.Join(dir, "Cargo.toml")
	if _, err := os.Stat(cargoPath); err == nil {
		if err := ra.parseCargo(cargoPath); err != nil {
			return nil, fmt.Errorf("parse Cargo.toml: %w", err)
		}
	}

	// Check for workspace (multiple crates)
	workspacePath := filepath.Join(dir, "Cargo.toml")
	ra.parseWorkspace(workspacePath)

	// Step 2: Walk src/ directory for .rs files
	srcDir := filepath.Join(dir, "src")
	if _, err := os.Stat(srcDir); err != nil {
		return nil, fmt.Errorf("src directory not found: %w", err)
	}

	err := filepath.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".rs") {
			return nil
		}
		return ra.parseRustFile(path, srcDir)
	})
	if err != nil {
		return nil, fmt.Errorf("walk src: %w", err)
	}

	// Step 3: Build graph
	ra.buildGraph()

	return &model.Graph{
		Nodes: ra.nodes,
		Edges: ra.edges,
	}, nil
}

var (
	reModDecl    = regexp.MustCompile(`^(?:pub(?:\(crate\))?\s+)?mod\s+(\w+)\s*[;{]`)
	reUseDecl    = regexp.MustCompile(`^(?:pub\s+)?use\s+(?:crate::)?(\w+)`)
	reStructDecl = regexp.MustCompile(`^(?:pub(?:\(crate\))?\s+)?struct\s+(\w+)`)
	reTraitDecl  = regexp.MustCompile(`^(?:pub(?:\(crate\))?\s+)?trait\s+(\w+)`)
	reImplDecl   = regexp.MustCompile(`^impl(?:<[^>]*>)?\s+(\w+)\s+for\s+(\w+)`)
	reFnDecl     = regexp.MustCompile(`^(?:pub(?:\(crate\))?\s+)?(?:async\s+)?fn\s+(\w+)`)
	reCargoDepLn = regexp.MustCompile(`^(\w[\w-]*)\s*=`)
	rePubPrefix  = regexp.MustCompile(`^pub(?:\(crate\))?\s+`)
)

func (ra *RustAnalyzer) parseRustFile(path, srcDir string) error {
	relPath, _ := filepath.Rel(srcDir, path)
	modName := pathToModuleName(relPath)

	mod := &RustModule{
		Name: modName,
		Path: path,
	}

	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	inBlockComment := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip comments
		if strings.HasPrefix(line, "//") {
			continue
		}
		if strings.Contains(line, "/*") {
			inBlockComment = true
		}
		if inBlockComment {
			if strings.Contains(line, "*/") {
				inBlockComment = false
			}
			continue
		}

		// Parse mod declarations
		if m := reModDecl.FindStringSubmatch(line); m != nil {
			mod.ModDecls = append(mod.ModDecls, m[1])
		}

		// Parse use declarations
		if m := reUseDecl.FindStringSubmatch(line); m != nil {
			mod.Uses = append(mod.Uses, m[1])
		}

		// Parse struct declarations
		if m := reStructDecl.FindStringSubmatch(line); m != nil {
			vis := "private"
			if strings.HasPrefix(line, "pub(crate)") {
				vis = "pub(crate)"
			} else if strings.HasPrefix(line, "pub") {
				vis = "pub"
			}
			mod.Structs = append(mod.Structs, RustStruct{
				Name:       m[1],
				Visibility: vis,
			})
		}

		// Parse trait declarations
		if m := reTraitDecl.FindStringSubmatch(line); m != nil {
			mod.Traits = append(mod.Traits, RustTrait{
				Name: m[1],
			})
		}

		// Parse impl Trait for Struct
		if m := reImplDecl.FindStringSubmatch(line); m != nil {
			traitName := m[1]
			structName := m[2]
			for i, s := range mod.Structs {
				if s.Name == structName {
					mod.Structs[i].ImplTraits = append(mod.Structs[i].ImplTraits, traitName)
				}
			}
		}

		// Parse function declarations
		if m := reFnDecl.FindStringSubmatch(line); m != nil {
			mod.Functions = append(mod.Functions, m[1])
		}
	}

	ra.modules[modName] = mod
	return nil
}

func (ra *RustAnalyzer) parseCargo(path string) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()

	crate := &CrateInfo{Path: path}
	scanner := bufio.NewScanner(file)
	section := ""

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if strings.HasPrefix(line, "[") && strings.HasSuffix(line, "]") {
			section = strings.Trim(line, "[]")
			continue
		}

		switch section {
		case "package":
			if strings.HasPrefix(line, "name") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					crate.Name = strings.Trim(strings.TrimSpace(parts[1]), `"`)
				}
			}
			if strings.HasPrefix(line, "version") {
				parts := strings.SplitN(line, "=", 2)
				if len(parts) == 2 {
					crate.Version = strings.Trim(strings.TrimSpace(parts[1]), `"`)
				}
			}
		case "dependencies", "dev-dependencies":
			if m := reCargoDepLn.FindStringSubmatch(line); m != nil {
				crate.Dependencies = append(crate.Dependencies, m[1])
			}
		}
	}

	if crate.Name != "" {
		ra.crates[crate.Name] = crate
	}
	return nil
}

func (ra *RustAnalyzer) parseWorkspace(cargoPath string) {
	file, err := os.Open(cargoPath)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	inWorkspace := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "[workspace]" {
			inWorkspace = true
			continue
		}
		if inWorkspace && strings.HasPrefix(line, "members") {
			// Parse workspace members - would need TOML parser for full support
			// For now, just note that it's a workspace
			break
		}
		if strings.HasPrefix(line, "[") && line != "[workspace]" {
			inWorkspace = false
		}
	}
}

func (ra *RustAnalyzer) buildGraph() {
	// Add module nodes
	for name, mod := range ra.modules {
		entity := "module"
		if len(mod.Structs) > 0 && len(mod.Traits) > 0 {
			entity = "module+types"
		} else if len(mod.Structs) > 0 {
			entity = "module+structs"
		} else if len(mod.Traits) > 0 {
			entity = "module+traits"
		}

		ra.nodes = append(ra.nodes, model.Node{
			ID:     name,
			Title:  name,
			Entity: entity,
		})
	}

	// Add crate dependency nodes
	for name, crate := range ra.crates {
		for _, dep := range crate.Dependencies {
			// Add external dependency as node
			depID := "ext:" + dep
			ra.nodes = append(ra.nodes, model.Node{
				ID:     depID,
				Title:  dep,
				Entity: "external_crate",
			})
			ra.edges = append(ra.edges, model.Edge{
				From: name,
				To:   depID,
				Type: "depends",
			})
		}
	}

	// Add use-based edges (module -> module)
	for name, mod := range ra.modules {
		for _, usePath := range mod.Uses {
			if _, ok := ra.modules[usePath]; ok {
				ra.edges = append(ra.edges, model.Edge{
					From: name,
					To:   usePath,
					Type: "uses",
				})
			}
		}

		// Add mod declaration edges (parent -> child)
		for _, child := range mod.ModDecls {
			childPath := name + "::" + child
			if _, ok := ra.modules[childPath]; ok {
				ra.edges = append(ra.edges, model.Edge{
					From: name,
					To:   childPath,
					Type: "declares",
				})
			} else if _, ok := ra.modules[child]; ok {
				ra.edges = append(ra.edges, model.Edge{
					From: name,
					To:   child,
					Type: "declares",
				})
			}
		}
	}
}

func pathToModuleName(relPath string) string {
	// Convert file path to Rust module name
	// src/auth/mod.rs -> auth
	// src/auth/handler.rs -> auth::handler
	// src/main.rs -> main
	// src/lib.rs -> lib

	name := strings.TrimSuffix(relPath, ".rs")
	name = strings.ReplaceAll(name, string(filepath.Separator), "::")

	// Remove mod suffix (auth::mod -> auth)
	if strings.HasSuffix(name, "::mod") {
		name = strings.TrimSuffix(name, "::mod")
	}

	return name
}
