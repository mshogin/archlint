// Package linter содержит анализаторы для проверки качества кода.
package linter

import (
	"go/ast"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/analysis"
	"gopkg.in/yaml.v3"
)

// Config содержит конфигурацию tracerlint.
type Config struct {
	Tracerlint struct {
		ExcludePackages []string `yaml:"exclude_packages"`
	} `yaml:"tracerlint"`
}

// Analyzer проверяет что все функции/методы имеют tracer.Enter и tracer.Exit*.
var Analyzer = &analysis.Analyzer{
	Name: "tracerlint",
	Doc:  "checks that all functions have tracer.Enter() and tracer.ExitSuccess/ExitError() before returns",
	Run:  run,
}

var excludedPackages []string

func init() {
	excludedPackages = loadConfig()
}

// loadConfig читает конфигурацию из .archlint.yaml.
func loadConfig() []string {
	configPath := findConfigFile()
	if configPath == "" {
		return nil
	}

	//nolint:gosec // G304: configPath comes from findConfigFile() which searches upward from cwd for .archlint.yaml
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil
	}

	return config.Tracerlint.ExcludePackages
}

// findConfigFile ищет .archlint.yaml в текущей директории и выше.
func findConfigFile() string {
	dir, err := os.Getwd()
	if err != nil {
		return ""
	}

	for {
		configPath := filepath.Join(dir, ".archlint.yaml")
		if _, err := os.Stat(configPath); err == nil {
			return configPath
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}

		dir = parent
	}

	return ""
}

// isExcluded проверяет, находится ли файл в исключенном пакете.
func isExcluded(filename string) bool {
	for _, pkg := range excludedPackages {
		// Нормализуем путь для сравнения
		pkgPath := filepath.FromSlash(pkg)
		if strings.Contains(filename, string(filepath.Separator)+pkgPath+string(filepath.Separator)) ||
			strings.HasSuffix(filename, string(filepath.Separator)+pkgPath) {
			return true
		}
	}

	return false
}

//nolint:funlen // Linter run function requires iterating files, filtering, and checking all functions.
func run(pass *analysis.Pass) (any, error) {
	for _, file := range pass.Files {
		// Пропускаем файлы из кеша сборки Go.
		filename := pass.Fset.Position(file.Pos()).Filename
		if strings.Contains(filename, "/go-build/") || strings.Contains(filename, "\\go-build\\") {
			continue
		}

		// Пропускаем тестовые файлы.
		if strings.HasSuffix(filename, "_test.go") {
			continue
		}

		// Пропускаем исключенные пакеты из конфигурации.
		if isExcluded(filename) {
			continue
		}

		ast.Inspect(file, func(n ast.Node) bool {
			fn, ok := n.(*ast.FuncDecl)
			if !ok || fn.Body == nil {
				return true
			}

			// Проверяем наличие комментария @skip-tracer.
			if hasSkipTracerComment(fn) {
				return true
			}

			funcName := getFunctionName(fn)

			// Для обычных функций проверяем tracer.Enter и tracer.Exit*.
			// Проверяем наличие tracer.Enter в начале.
			if !hasTracerEnter(fn) {
				pass.Reportf(fn.Pos(),
					"function %s missing tracer.Enter() at the beginning",
					funcName)
			}

			// Проверяем что перед каждым return есть tracer.Exit*.
			checkReturns(pass, fn, funcName)

			return true
		})
	}

	return nil, nil //nolint:nilnil // analysis.Analyzer.Run signature requires (interface{}, error)
}

// hasSkipTracerComment проверяет есть ли комментарий @skip-tracer.
func hasSkipTracerComment(fn *ast.FuncDecl) bool {
	if fn.Doc == nil {
		return false
	}

	for _, comment := range fn.Doc.List {
		if strings.Contains(comment.Text, "@skip-tracer") {
			return true
		}
	}

	return false
}

// getFunctionName возвращает полное имя функции (Type.Method или просто Function).
func getFunctionName(fn *ast.FuncDecl) string {
	if fn.Recv != nil && len(fn.Recv.List) > 0 {
		// Это метод
		receiverType := getReceiverType(fn.Recv.List[0].Type)

		return receiverType + "." + fn.Name.Name
	}

	return fn.Name.Name
}

// getReceiverType извлекает имя типа из receiver.
func getReceiverType(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return getReceiverType(t.X)
	default:
		return "Unknown"
	}
}

// hasTracerEnter проверяет что первый statement это tracer.Enter.
func hasTracerEnter(fn *ast.FuncDecl) bool {
	if len(fn.Body.List) == 0 {
		return false
	}

	firstStmt := fn.Body.List[0]

	exprStmt, ok := firstStmt.(*ast.ExprStmt)
	if !ok {
		return false
	}

	return isTracerCall(exprStmt.X, "Enter")
}

// checkReturns проверяет что перед каждым return есть tracer.Exit*.
func checkReturns(pass *analysis.Pass, fn *ast.FuncDecl, funcName string) {
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		// Пропускаем анонимные функции - их return'ы не относятся к родительской функции
		if _, ok := n.(*ast.FuncLit); ok {
			return false // не заходим внутрь анонимной функции
		}

		ret, ok := n.(*ast.ReturnStmt)
		if !ok {
			return true
		}

		// Находим statement перед return
		prevStmt := findPreviousStatement(fn.Body, ret)
		if prevStmt == nil {
			pass.Reportf(ret.Pos(),
				"return in %s must be preceded by tracer.ExitSuccess() or tracer.ExitError()",
				funcName)

			return true
		}

		// Проверяем что это tracer.Exit*.
		if !isTracerExitCall(prevStmt) {
			pass.Reportf(ret.Pos(),
				"return in %s must be preceded by tracer.ExitSuccess() or tracer.ExitError()",
				funcName)
		}

		return true
	})
}

// findPreviousStatement находит предыдущий значимый statement перед данным.
//
//nolint:gocyclo // Statement search requires handling multiple AST node types (case, comm, blocks).
func findPreviousStatement(body *ast.BlockStmt, target ast.Node) ast.Stmt {
	for i, stmt := range body.List {
		if stmt == target && i > 0 {
			// Идем назад, пропуская пустые statements
			for j := i - 1; j >= 0; j-- {
				if !isEmptyStmt(body.List[j]) {
					return body.List[j]
				}
			}

			return nil
		}

		// Специальная обработка для CaseClause и CommClause
		switch s := stmt.(type) {
		case *ast.CaseClause:
			if prev := findInStatementList(s.Body, target); prev != nil {
				return prev
			}
		case *ast.CommClause:
			if prev := findInStatementList(s.Body, target); prev != nil {
				return prev
			}
		default:
			// Рекурсивно ищем в блоках (if, for, etc)
			if block := getBlockStmt(stmt); block != nil {
				if prev := findPreviousStatement(block, target); prev != nil {
					return prev
				}
			}
		}
	}

	return nil
}

// findInStatementList ищет предыдущий statement в списке statements.
func findInStatementList(stmts []ast.Stmt, target ast.Node) ast.Stmt {
	for i, stmt := range stmts {
		if stmt == target && i > 0 {
			// Идем назад, пропуская пустые statements
			for j := i - 1; j >= 0; j-- {
				if !isEmptyStmt(stmts[j]) {
					return stmts[j]
				}
			}

			return nil
		}

		// Рекурсивно ищем в блоках
		if block := getBlockStmt(stmt); block != nil {
			if prev := findPreviousStatement(block, target); prev != nil {
				return prev
			}
		}
	}

	return nil
}

// isEmptyStmt проверяет является ли statement "пустым" (можно пропустить).
func isEmptyStmt(stmt ast.Stmt) bool {
	// ast.EmptyStmt - это явно пустой statement (например, одиночная ;)
	_, isEmpty := stmt.(*ast.EmptyStmt)

	return isEmpty
}

// getBlockStmt извлекает BlockStmt из statement если есть.
func getBlockStmt(stmt ast.Stmt) *ast.BlockStmt {
	switch s := stmt.(type) {
	case *ast.BlockStmt:
		return s
	case *ast.IfStmt:
		return s.Body
	case *ast.ForStmt:
		return s.Body
	case *ast.RangeStmt:
		return s.Body
	case *ast.SwitchStmt:
		return s.Body
	case *ast.TypeSwitchStmt:
		return s.Body
	case *ast.SelectStmt:
		return s.Body
	case *ast.CaseClause:
		// Case clause имеет список statements, оборачиваем в BlockStmt
		return &ast.BlockStmt{List: s.Body}
	case *ast.CommClause:
		// Comm clause (для select) также имеет список statements
		return &ast.BlockStmt{List: s.Body}
	}

	return nil
}

// isTracerExitCall проверяет что это вызов tracer.ExitSuccess или tracer.ExitError.
func isTracerExitCall(stmt ast.Stmt) bool {
	exprStmt, ok := stmt.(*ast.ExprStmt)
	if !ok {
		return false
	}

	return isTracerCall(exprStmt.X, "ExitSuccess") ||
		isTracerCall(exprStmt.X, "ExitError")
}

// isTracerCall проверяет что это вызов tracer.MethodName (без проверки аргументов).
func isTracerCall(expr ast.Expr, methodName string) bool {
	callExpr, ok := expr.(*ast.CallExpr)
	if !ok {
		return false
	}

	// Проверяем что это tracer.MethodName
	sel, ok := callExpr.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	ident, ok := sel.X.(*ast.Ident)
	if !ok || ident.Name != "tracer" {
		return false
	}

	return sel.Sel.Name == methodName
}
