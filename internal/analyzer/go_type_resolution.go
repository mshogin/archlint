package analyzer

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// resolveFileTypes binds qualifiers in their source file before graph construction.
// Package IDs remain relative to the scan root; import paths remain module-qualified.
func (p *GoParser) resolveFileTypes() {
	imports := make(map[string]*PackageInfo)
	for _, pkg := range p.packages {
		if path := packageImportPath(pkg.Dir); path != "" {
			// At the scan root, external tests have a separate package ID but
			// share the directory. Plain imports must use the production name.
			if existing := imports[path]; existing == nil ||
				(strings.HasSuffix(existing.Name, "_test") && !strings.HasSuffix(pkg.Name, "_test")) {
				imports[path] = pkg
			}
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
			pkg := imports[path]
			name := ""
			if spec.Name != nil {
				name = spec.Name.Name
			} else if pkg != nil {
				name = pkg.Name
			}
			if name == "" || name == "_" || name == "." {
				continue
			}
			b[name] = "?" + path // Unknown imports must never match local types.
			if pkg != nil {
				b[name] = p.getPkgID(pkg.Dir, pkg.Name)
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
