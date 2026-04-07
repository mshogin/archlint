// Package cli provides the command-line interface implementation for archlint.
package cli

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

var (
	version = "0.1.0"
)

var rootCmd = &cobra.Command{
	Use:   "archlint",
	Short: "Architecture graph builder tool",
	Long: `archlint - a tool for building structural graphs and behavioral graphs
from Go source code.`,
	Version: version,
}

// Execute runs the root CLI command.
func Execute() error {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)

		return fmt.Errorf("command execution error: %w", err)
	}

	return nil
}
