// Package analyzer contains source code analyzers for building architecture graphs.
//
// Rust analyzer (MVP, regex-based).
// Supports structs, enums, traits, functions, impl blocks, modules, use statements.
// Workspace support via Cargo.toml [workspace] members.
//
// Credit: @dklohgs for the original issue specification.
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
	modules map[string]*RustModule
	crates  map[string]*CrateInfo
	nodes   []model.Node
	edges   []model.Edge
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
	Enums      []RustEnum
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

// RustEnum represents a Rust enum.
type RustEnum struct {
	Name       string
	Visibility string
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
	// Step 1: Parse Cargo.toml for crate dependencies and workspace members.
	cargoPath := filepath.Join(dir, "Cargo.toml")
	if _, err := os.Stat(cargoPath); err == nil {
		if err := ra.parseCargo(cargoPath); err != nil {
			return nil, fmt.Errorf("parse Cargo.toml: %w", err)
		}
		ra.parseWorkspace(cargoPath, dir)
	}

	// Step 2: Walk directory for .rs files (support both src/ layout and flat layout).
	srcDir := filepath.Join(dir, "src")
	walkDir := dir
	if _, err := os.Stat(srcDir); err == nil {
		walkDir = srcDir
	}

	err := filepath.Walk(walkDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			name := info.Name()
			if name == "target" || name == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".rs") {
			return nil
		}
		return ra.parseRustFile(path, walkDir)
	})
	if err != nil {
		return nil, fmt.Errorf("walk src: %w", err)
	}

	// Step 3: Build graph.
	ra.buildGraph()

	return &model.Graph{
		Nodes: ra.nodes,
		Edges: ra.edges,
	}, nil
}

var (
	reModDecl    = regexp.MustCompile(`^(?:pub(?:\(crate\))?\s+)?mod\s+(\w+)\s*[;{]`)
	reUseDecl    = regexp.MustCompile(`^(?:pub\s+)?use\s+(?:(?:crate|super|self)::)?(\w+)`)
	reStructDecl = regexp.MustCompile(`^(?:pub(?:\(crate\))?\s+)?struct\s+(\w+)`)
	reEnumDecl   = regexp.MustCompile(`^(?:pub(?:\(crate\))?\s+)?enum\s+(\w+)`)
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

		// Skip line comments.
		if strings.HasPrefix(line, "//") {
			continue
		}
		// Handle block comments.
		if strings.Contains(line, "/*") {
			inBlockComment = true
		}
		if inBlockComment {
			if strings.Contains(line, "*/") {
				inBlockComment = false
			}
			continue
		}

		// Parse mod declarations.
		if m := reModDecl.FindStringSubmatch(line); m != nil {
			mod.ModDecls = append(mod.ModDecls, m[1])
		}

		// Parse use declarations.
		if m := reUseDecl.FindStringSubmatch(line); m != nil {
			mod.Uses = append(mod.Uses, m[1])
		}

		// Parse struct declarations.
		if m := reStructDecl.FindStringSubmatch(line); m != nil {
			vis := visibilityFromLine(line)
			mod.Structs = append(mod.Structs, RustStruct{
				Name:       m[1],
				Visibility: vis,
			})
		}

		// Parse enum declarations.
		if m := reEnumDecl.FindStringSubmatch(line); m != nil {
			vis := visibilityFromLine(line)
			mod.Enums = append(mod.Enums, RustEnum{
				Name:       m[1],
				Visibility: vis,
			})
		}

		// Parse trait declarations.
		if m := reTraitDecl.FindStringSubmatch(line); m != nil {
			mod.Traits = append(mod.Traits, RustTrait{
				Name: m[1],
			})
		}

		// Parse impl Trait for Struct — add to struct's impl list.
		if m := reImplDecl.FindStringSubmatch(line); m != nil {
			traitName := m[1]
			structName := m[2]
			for i, s := range mod.Structs {
				if s.Name == structName {
					mod.Structs[i].ImplTraits = append(mod.Structs[i].ImplTraits, traitName)
				}
			}
			// Also check enums.
			for i, e := range mod.Enums {
				if e.Name == structName {
					mod.Enums[i].ImplTraits = append(mod.Enums[i].ImplTraits, traitName)
				}
			}
		}

		// Parse function declarations.
		if m := reFnDecl.FindStringSubmatch(line); m != nil {
			mod.Functions = append(mod.Functions, m[1])
		}
	}

	ra.modules[modName] = mod
	return nil
}

// visibilityFromLine extracts Rust visibility from a declaration line.
func visibilityFromLine(line string) string {
	if strings.HasPrefix(line, "pub(crate)") {
		return "pub(crate)"
	} else if rePubPrefix.MatchString(line) {
		return "pub"
	}
	return "private"
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

		if strings.HasPrefix(line, "[") {
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

// parseWorkspace parses [workspace] members from Cargo.toml and processes each member crate.
func (ra *RustAnalyzer) parseWorkspace(cargoPath, rootDir string) {
	file, err := os.Open(cargoPath)
	if err != nil {
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	inWorkspace := false
	inMembers := false

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "[workspace]" {
			inWorkspace = true
			continue
		}

		if inWorkspace {
			if strings.HasPrefix(line, "[") && line != "[workspace]" {
				inWorkspace = false
				inMembers = false
				continue
			}

			if strings.HasPrefix(line, "members") {
				inMembers = true
			}

			if inMembers {
				// Extract quoted member paths: "crate-name" or "path/to/crate"
				re := regexp.MustCompile(`"([^"]+)"`)
				matches := re.FindAllStringSubmatch(line, -1)
				for _, m := range matches {
					memberPath := filepath.Join(rootDir, m[1])
					memberCargo := filepath.Join(memberPath, "Cargo.toml")
					if _, err := os.Stat(memberCargo); err == nil {
						_ = ra.parseCargo(memberCargo)
					}
				}
				// Check if members list ends (closing bracket).
				if strings.Contains(line, "]") && !strings.HasPrefix(line, "members") {
					inMembers = false
				}
			}
		}
	}
}

func (ra *RustAnalyzer) buildGraph() {
	// Add module nodes with their types as entities.
	for name, mod := range ra.modules {
		entity := "module"
		hasStructs := len(mod.Structs) > 0
		hasEnums := len(mod.Enums) > 0
		hasTraits := len(mod.Traits) > 0
		switch {
		case hasStructs && hasTraits:
			entity = "module+types"
		case hasStructs && hasEnums:
			entity = "module+types"
		case hasStructs:
			entity = "module+structs"
		case hasEnums:
			entity = "module+enums"
		case hasTraits:
			entity = "module+traits"
		}

		ra.nodes = append(ra.nodes, model.Node{
			ID:     name,
			Title:  name,
			Entity: entity,
		})

		// Add struct component nodes.
		for _, s := range mod.Structs {
			nodeID := name + "::" + s.Name
			ra.nodes = append(ra.nodes, model.Node{
				ID:     nodeID,
				Title:  s.Name,
				Entity: "struct",
			})
			ra.edges = append(ra.edges, model.Edge{
				From: name,
				To:   nodeID,
				Type: "contains",
			})
			// Add impl-trait edges.
			for _, trait := range s.ImplTraits {
				traitID := name + "::" + trait
				ra.edges = append(ra.edges, model.Edge{
					From: nodeID,
					To:   traitID,
					Type: "implements",
				})
			}
		}

		// Add enum component nodes.
		for _, e := range mod.Enums {
			nodeID := name + "::" + e.Name
			ra.nodes = append(ra.nodes, model.Node{
				ID:     nodeID,
				Title:  e.Name,
				Entity: "enum",
			})
			ra.edges = append(ra.edges, model.Edge{
				From: name,
				To:   nodeID,
				Type: "contains",
			})
			// Add impl-trait edges.
			for _, trait := range e.ImplTraits {
				traitID := name + "::" + trait
				ra.edges = append(ra.edges, model.Edge{
					From: nodeID,
					To:   traitID,
					Type: "implements",
				})
			}
		}

		// Add trait component nodes.
		for _, t := range mod.Traits {
			nodeID := name + "::" + t.Name
			ra.nodes = append(ra.nodes, model.Node{
				ID:     nodeID,
				Title:  t.Name,
				Entity: "trait",
			})
			ra.edges = append(ra.edges, model.Edge{
				From: name,
				To:   nodeID,
				Type: "contains",
			})
		}
	}

	// Add crate dependency nodes and edges.
	for name, crate := range ra.crates {
		for _, dep := range crate.Dependencies {
			depID := "ext:" + dep
			// Register external dependency as node if not present.
			found := false
			for _, n := range ra.nodes {
				if n.ID == depID {
					found = true
					break
				}
			}
			if !found {
				ra.nodes = append(ra.nodes, model.Node{
					ID:     depID,
					Title:  dep,
					Entity: "external_crate",
				})
			}
			ra.edges = append(ra.edges, model.Edge{
				From: name,
				To:   depID,
				Type: "depends",
			})
		}
	}

	// Add use-based edges (module -> module).
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

		// Add mod declaration edges (parent -> child).
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

// pathToModuleName converts a relative file path to a Rust module name.
// Examples:
//   - main.rs -> main
//   - lib.rs -> lib
//   - auth/mod.rs -> auth
//   - auth/handler.rs -> auth::handler
func pathToModuleName(relPath string) string {
	name := strings.TrimSuffix(relPath, ".rs")
	name = strings.ReplaceAll(name, string(filepath.Separator), "::")
	// Remove mod suffix: auth::mod -> auth
	if strings.HasSuffix(name, "::mod") {
		name = strings.TrimSuffix(name, "::mod")
	}
	return name
}

// DetectRustProject returns true if dir contains a Cargo.toml file.
func DetectRustProject(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, "Cargo.toml"))
	return err == nil
}
