// Package main содержит точку входа для tracelint linter.
package main

import (
	"github.com/mshogin/archlint/internal/linter"
	"github.com/mshogin/archlint/pkg/tracer"
	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() {
	tracer.Enter("main")
	singlechecker.Main(linter.Analyzer)
	tracer.ExitSuccess("main")
}
