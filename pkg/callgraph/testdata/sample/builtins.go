package sample

// FuncWithBuiltins calls builtins and type conversions that must not appear as external nodes.
func FuncWithBuiltins(items []string) int {
	n := len(items)
	result := make([]string, 0, n)
	_ = append(result, items...)

	return int(n)
}
