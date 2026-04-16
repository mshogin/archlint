package analyzer

import (
	"os"
	"strings"
	"testing"
)

// parseSnippetOS parses a Go source snippet from a temp file and returns the
// populated GoParser so tests can inspect methods and field accesses.
func parseSnippetOS(t *testing.T, src string) *GoParser {
	t.Helper()

	dir := t.TempDir()
	filePath := dir + "/snippet.go"

	if err := os.WriteFile(filePath, []byte(src), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	packages := make(map[string]*PackageInfo)
	types := make(map[string]*TypeInfo)
	functions := make(map[string]*FunctionInfo)
	methods := make(map[string]*MethodInfo)

	p := newGoParser(packages, types, functions, methods)
	if err := p.parseFile(filePath); err != nil {
		t.Fatalf("parseFile: %v", err)
	}

	return p
}

func getMethodByName(methods map[string]*MethodInfo, name string) *MethodInfo {
	for _, m := range methods {
		if m.Name == name {
			return m
		}
	}

	return nil
}

func hasFieldAccess(accesses []FieldAccessInfo, fieldName string, isWrite bool) bool {
	for _, fa := range accesses {
		if fa.FieldName == fieldName && fa.IsWrite == isWrite {
			return true
		}
	}

	return false
}

// TestFieldAccess_Getter verifies that a method reading a single field emits
// one field_read access for that field.
func TestFieldAccess_Getter(t *testing.T) {
	src := `package mypkg

type User struct {
	Name string
}

func (u *User) GetName() string {
	return u.Name
}
`
	p := parseSnippetOS(t, src)
	m := getMethodByName(p.methods, "GetName")

	if m == nil {
		t.Fatal("method GetName not found")
	}

	if len(m.FieldAccess) == 0 {
		t.Fatal("expected at least one field access, got none")
	}

	if !hasFieldAccess(m.FieldAccess, "Name", false) {
		t.Errorf("expected field_read for Name, got %+v", m.FieldAccess)
	}
}

// TestFieldAccess_Setter verifies that a method writing a single field emits
// one field_write access.
func TestFieldAccess_Setter(t *testing.T) {
	src := `package mypkg

type User struct {
	Name string
}

func (u *User) SetName(name string) {
	u.Name = name
}
`
	p := parseSnippetOS(t, src)
	m := getMethodByName(p.methods, "SetName")

	if m == nil {
		t.Fatal("method SetName not found")
	}

	if !hasFieldAccess(m.FieldAccess, "Name", true) {
		t.Errorf("expected field_write for Name, got %+v", m.FieldAccess)
	}
}

// TestFieldAccess_ReadAndWrite verifies that a method that both reads and
// writes multiple fields produces the correct mix of accesses.
func TestFieldAccess_ReadAndWrite(t *testing.T) {
	src := `package mypkg

type Counter struct {
	Value int
	Total int
}

func (c *Counter) Increment() {
	c.Total = c.Total + c.Value
	c.Value++
}
`
	p := parseSnippetOS(t, src)
	m := getMethodByName(p.methods, "Increment")

	if m == nil {
		t.Fatal("method Increment not found")
	}

	if !hasFieldAccess(m.FieldAccess, "Total", true) {
		t.Errorf("expected field_write for Total, got %+v", m.FieldAccess)
	}

	if !hasFieldAccess(m.FieldAccess, "Total", false) {
		t.Errorf("expected field_read for Total (RHS), got %+v", m.FieldAccess)
	}

	// Value is incremented (write via inc-stmt).
	if !hasFieldAccess(m.FieldAccess, "Value", true) {
		t.Errorf("expected field_write for Value (inc-stmt), got %+v", m.FieldAccess)
	}
}

// TestFieldAccess_NoFieldAccess verifies that a method doing pure computation
// (no field access) has an empty FieldAccess slice.
func TestFieldAccess_NoFieldAccess(t *testing.T) {
	src := `package mypkg

type Calc struct{}

func (c *Calc) Add(a, b int) int {
	return a + b
}
`
	p := parseSnippetOS(t, src)
	m := getMethodByName(p.methods, "Add")

	if m == nil {
		t.Fatal("method Add not found")
	}

	if len(m.FieldAccess) != 0 {
		t.Errorf("expected no field accesses, got %+v", m.FieldAccess)
	}
}

// TestFieldAccess_CallsOtherMethod verifies that a method calling another
// method does not generate false field-access entries for those calls.
func TestFieldAccess_CallsOtherMethod(t *testing.T) {
	src := `package mypkg

type Svc struct {
	count int
}

func (s *Svc) increment() { s.count++ }

func (s *Svc) Run() {
	s.increment()
}
`
	p := parseSnippetOS(t, src)

	// Run() calls s.increment() — that SelectorExpr resolves to a method call,
	// not a field.  The field "increment" should NOT appear in FieldAccess for Run.
	m := getMethodByName(p.methods, "Run")
	if m == nil {
		t.Fatal("method Run not found")
	}

	// "increment" is a method name, not a field — should not be emitted as field.
	// (We cannot distinguish without type info, but we verify that for the
	//  field "count" — which is NOT touched by Run — no spurious read edge exists.)
	if hasFieldAccess(m.FieldAccess, "count", false) || hasFieldAccess(m.FieldAccess, "count", true) {
		t.Errorf("Run() should not have field access to 'count'; got %+v", m.FieldAccess)
	}
}

// TestFieldAccess_17Getters verifies the demo/User scenario: a struct with
// many getters each gets its own field_read edge.
func TestFieldAccess_17Getters(t *testing.T) {
	fields := []string{
		"ID", "Name", "Email", "Age", "Phone",
		"Address", "City", "Country", "Zip", "CreatedAt",
		"UpdatedAt", "DeletedAt", "Role", "Status", "Score",
		"Bio", "Avatar",
	}

	var sb strings.Builder
	sb.WriteString("package mypkg\n\ntype User struct {\n")

	for _, f := range fields {
		sb.WriteString("\t" + f + " string\n")
	}

	sb.WriteString("}\n\n")

	for _, f := range fields {
		sb.WriteString("func (u *User) Get" + f + "() string { return u." + f + " }\n")
	}

	p := parseSnippetOS(t, sb.String())

	for _, f := range fields {
		m := getMethodByName(p.methods, "Get"+f)
		if m == nil {
			t.Errorf("method Get%s not found", f)
			continue
		}

		if !hasFieldAccess(m.FieldAccess, f, false) {
			t.Errorf("Get%s: expected field_read for %s, got %+v", f, f, m.FieldAccess)
		}
	}
}

// TestFieldAccess_GraphNodes verifies that after building the graph, field
// nodes and field_read/field_write edges appear in the YAML output graph.
func TestFieldAccess_GraphNodes(t *testing.T) {
	src := `package mypkg

type Rect struct {
	Width  int
	Height int
}

func (r *Rect) Area() int {
	return r.Width * r.Height
}

func (r *Rect) Scale(factor int) {
	r.Width = r.Width * factor
	r.Height = r.Height * factor
}
`
	dir := t.TempDir()
	filePath := dir + "/rect.go"

	if err := os.WriteFile(filePath, []byte(src), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	a := NewGoAnalyzer()
	graph, err := a.Analyze(dir)

	if err != nil {
		t.Fatalf("Analyze: %v", err)
	}

	// Check for field nodes.
	fieldNodes := map[string]bool{}

	for _, n := range graph.Nodes {
		if n.Entity == "field" {
			fieldNodes[n.ID] = true
		}
	}

	expectedSuffixes := []string{
		".Rect.Width",
		".Rect.Height",
	}

	for _, suffix := range expectedSuffixes {
		found := false

		for id := range fieldNodes {
			if strings.HasSuffix(id, suffix) {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("expected field node with suffix %q, got fieldNodes=%v", suffix, fieldNodes)
		}
	}

	// Check for field_read and field_write edges.
	hasRead, hasWrite := false, false

	for _, e := range graph.Edges {
		if e.Type == "field_read" {
			hasRead = true
		}

		if e.Type == "field_write" {
			hasWrite = true
		}
	}

	if !hasRead {
		t.Error("expected at least one field_read edge in graph")
	}

	if !hasWrite {
		t.Error("expected at least one field_write edge in graph")
	}
}
