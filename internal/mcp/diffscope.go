package mcp

import (
	"strconv"
	"strings"
)

// diffscope.go — сужение анализа изменения до РЕАЛЬНО тронутых объявлений.
//
// Зачем: без диффа analyze_change считает затронутым весь файл, то есть на
// файле в 800 строк «затронутыми» оказываются все его функции. Для разбора
// последствий это шум: радиус изменения считается от того, что не менялось.

// hunkHeaderRe разбирать не нужно - формат заголовка ханка фиксирован:
// @@ -<old>,<oldCount> +<new>,<newCount> @@. Берём только новую сторону:
// нас интересуют строки в файле ПОСЛЕ изменения, к ним привязаны объявления
// из текущего среза кода.

// changedLines возвращает номера строк файла path, тронутых унифицированным
// диффом. Дифф может содержать несколько файлов - берутся ханки только своего.
//
// Пустой результат означает «для этого файла в диффе ничего нет», и это НЕ то
// же самое, что «изменений нет»: вызывающий трактует пустоту как повод
// анализировать файл целиком.
func changedLines(diff, path string) map[int]bool {
	if strings.TrimSpace(diff) == "" {
		return nil
	}

	out := make(map[int]bool)
	inFile := false
	lineNo := 0

	for _, l := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(l, "+++ "):
			// «+++ b/internal/foo.go» - начало секции файла.
			inFile = sameFile(strings.TrimSpace(l[4:]), path)
			lineNo = 0

			continue
		case strings.HasPrefix(l, "--- "):
			continue
		case strings.HasPrefix(l, "@@"):
			if !inFile {
				continue
			}

			lineNo = hunkStart(l)

			continue
		}

		if !inFile || lineNo == 0 {
			continue
		}

		switch {
		case strings.HasPrefix(l, "+"):
			out[lineNo] = true
			lineNo++
		case strings.HasPrefix(l, "-"):
			// Удалённая строка новой нумерации не занимает, но объявление,
			// из которого её вырезали, затронуто - помечаем текущую позицию.
			out[lineNo] = true
		case strings.HasPrefix(l, " "), l == "":
			lineNo++
		}
	}

	return out
}

// sameFile сопоставляет путь из диффа (репо-относительный, с префиксом a/ или
// b/) с абсолютным путём анализируемого файла.
//
// Сравнение суффиксом, а не через filepath.Abs: Abs резолвит относительный
// путь от текущего каталога ПРОЦЕССА, который не обязан совпадать с корнем
// анализируемого репозитория - тогда ханки молча не находились бы, и сужение
// впустую откатывалось бы на файл целиком.
func sameFile(diffPath, absPath string) bool {
	p := strings.TrimSpace(diffPath)
	if p == "" || p == "/dev/null" {
		return false
	}

	for _, pref := range []string{"a/", "b/"} {
		p = strings.TrimPrefix(p, pref)
	}

	return absPath == p || strings.HasSuffix(absPath, "/"+p)
}

// hunkStart достаёт начальную строку НОВОЙ стороны из «@@ -a,b +c,d @@».
func hunkStart(header string) int {
	i := strings.Index(header, "+")
	if i < 0 {
		return 0
	}

	rest := header[i+1:]
	end := strings.IndexAny(rest, ", ")

	if end < 0 {
		end = len(rest)
	}

	n, err := strconv.Atoi(strings.TrimSpace(rest[:end]))
	if err != nil || n <= 0 {
		return 0
	}

	return n
}

// decl — объявление файла: узел графа и строка, с которой он начинается.
type decl struct {
	id   string
	line int
}

// declsTouched возвращает id объявлений, в которые попала хотя бы одна
// изменённая строка.
//
// Границы объявления считаются по СЛЕДУЮЩЕМУ объявлению в том же файле: в
// модели есть только строка начала (FunctionInfo.Line и соседи), конца нет.
// Для Go этого достаточно - объявления верхнего уровня идут подряд и не
// вкладываются друг в друга. Изменения ВЫШЕ первого объявления (пакет,
// импорты, package-level переменные) не принадлежат никакому узлу - их
// обрабатывает вызывающий, а не эта функция.
func declsTouched(decls []decl, changed map[int]bool) []string {
	if len(decls) == 0 || len(changed) == 0 {
		return nil
	}

	sorted := make([]decl, len(decls))
	copy(sorted, decls)

	// Сортировка вставками: объявлений в файле десятки, зависимость на sort
	// ради этого не нужна.
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j].line < sorted[j-1].line; j-- {
			sorted[j], sorted[j-1] = sorted[j-1], sorted[j]
		}
	}

	hit := make(map[string]bool)

	for line := range changed {
		idx := -1

		for i, d := range sorted {
			if d.line <= line {
				idx = i

				continue
			}

			break
		}

		if idx >= 0 {
			hit[sorted[idx].id] = true
		}
	}

	out := make([]string, 0, len(hit))
	for id := range hit {
		out = append(out, id)
	}

	return out
}

// Значения ChangeAnalysis.Scope.
const (
	scopeFile  = "file"         // радиус считался от файла целиком
	scopeDecls = "declarations" // радиус сужен до тронутых объявлений
)
