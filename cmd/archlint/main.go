// Package main содержит точку входа для archlint CLI.
package main

import (
	"os"

	"github.com/mshogin/archlint/internal/cli"
	"github.com/mshogin/archlint/pkg/tracer"
)

func main() {
	tracer.Enter("main")

	if err := cli.Execute(); err != nil {
		tracer.ExitError("main", err)
		os.Exit(1)
	}

	tracer.ExitSuccess("main")
}
