package callgraph

import "errors"

// Ошибки построения графа вызовов.
var (
	ErrEntryPointNotFound = errors.New("точка входа не найдена в коде")
	ErrInvalidMaxDepth    = errors.New("maxDepth должен быть от 1 до 50")
	ErrAnalyzerRequired   = errors.New("GoAnalyzer обязателен")
)
