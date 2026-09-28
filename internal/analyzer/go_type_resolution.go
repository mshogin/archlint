package analyzer

import (
	"go/build"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// resolveFileTypes binds qualifiers in their source file before graph construction.
// Package IDs remain relative to the scan root; import paths remain module-qualified.
// Call after all files have been parsed; repeated calls leave the bindings unchanged.
func (p *GoParser) resolveFileTypes() {
	if p.fileTypesResolved {
		return
	}
	// A directory may contain an ignored generator (package main) or an
	// external test package. Neither may determine the name of a plain import.
	// Retain every buildable production name so the qualifier selects its own
	// package even when several names share the scan root.
	imports := make(map[string]map[string]string)
	importPaths := make(map[string]string)
	for file, name := range p.filePackageNames {
		if strings.HasSuffix(name, "_test") {
			continue
		}
		dir := filepath.Dir(file)
		matches, err := build.Default.MatchFile(dir, filepath.Base(file))
		if err != nil || !matches {
			continue
		}
		path, ok := importPaths[dir]
		if !ok {
			path = packageImportPath(dir)
			importPaths[dir] = path
		}
		if path == "" {
			continue
		}
		if imports[path] == nil {
			imports[path] = make(map[string]string)
		}
		if pkg := p.packages[p.getPkgID(dir, name)]; pkg != nil {
			imports[path][name] = pkg.Path
		}
	}
	files := make(map[string]map[string]string)
	bindings := func(file string) map[string]string {
		if b, ok := files[file]; ok {
			return b
		}
		b := make(map[string]string)
		for _, spec := range p.fileImports[file] {
			path, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				continue
			}
			if spec.Name != nil {
				alias := spec.Name.Name
				if alias == "_" || alias == "." {
					continue
				}
				// An alias does not identify the package name. Bind it only when
				// the import path has one buildable production package.
				if len(imports[path]) == 1 {
					for _, pkgID := range imports[path] {
						b[alias] = pkgID
					}
				} else {
					b[alias] = "?" + path
				}
				continue
			}
			// For a plain import, the source qualifier must match the package's
			// declared name (which may differ from the directory name).
			for name, pkgID := range imports[path] {
				b[name] = pkgID
			}
		}
		files[file] = b
		return b
	}
	resolve := func(fields []FieldInfo, file string) {
		b := bindings(file)
		for i := range fields {
			if fields[i].TypePkg == "" {
				continue
			}
			qualifier := fields[i].TypePkg
			fields[i].TypePkg = "?" + qualifier
			if pkg, ok := b[qualifier]; ok {
				fields[i].TypePkg = pkg
			}
		}
	}
	for _, t := range p.types {
		resolve(t.Fields, t.File)
		for i := range t.MethodSigs {
			resolve(t.MethodSigs[i].Params, t.File)
			resolve(t.MethodSigs[i].Results, t.File)
		}
		for i, name := range t.Embeds {
			if dot := strings.IndexByte(name, '.'); dot >= 0 {
				pkg, ok := bindings(t.File)[name[:dot]]
				if !ok {
					pkg = "?" + name[:dot]
				}
				t.Embeds[i] = pkg + name[dot:]
			}
		}
	}
	for _, f := range p.functions {
		resolve(f.Params, f.File)
		resolve(f.Results, f.File)
		resolve(f.NamedParams, f.File)
	}
	for _, m := range p.methods {
		resolve(m.Params, m.File)
		resolve(m.Results, m.File)
		resolve(m.NamedParams, m.File)
	}
	p.fileTypesResolved = true
}

// packageImportPath uses the closest go.mod, including when scanning a subdirectory.
func packageImportPath(dir string) string {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return ""
	}
	for parent := abs; ; parent = filepath.Dir(parent) {
		data, err := os.ReadFile(filepath.Join(parent, "go.mod"))
		if err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				fields := strings.Fields(line)
				if len(fields) >= 2 && fields[0] == "module" {
					module := fields[1]
					if strings.HasPrefix(module, "\"") {
						if value, err := strconv.Unquote(module); err == nil {
							module = value
						}
					}
					rel, err := filepath.Rel(parent, abs)
					if err != nil {
						return ""
					}
					if rel == "." {
						return module
					}
					return module + "/" + filepath.ToSlash(rel)
				}
			}
			return ""
		}
		if filepath.Dir(parent) == parent {
			return ""
		}
	}
}
