// Package main содержит точку входа для archlint CLI.
package main

import (
	"os"

	"github.com/mshogin/archlint/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
