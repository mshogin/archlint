package cli

import (
	"fmt"

	"github.com/mshogin/archlint/internal/mcp"
	"github.com/spf13/cobra"
)

var (
	serveLogFile string
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Start MCP server for architecture analysis",
	Long: `Starts a Model Context Protocol (MCP) server over stdio.

The MCP server parses the project on initialization, keeps the architecture
graph in memory, and re-parses on each tool call to pick up file changes.

Available tools:
  analyze_file      — full file analysis (types, functions, dependencies)
  analyze_change    — analyze impact of a file change on the architecture
  get_dependencies  — get dependency graph for a file or package
  get_architecture  — get full architecture graph (or filtered subset)
  check_violations  — check for circular dependencies, high coupling
  get_callgraph     — get call graph from an entry point

Usage with Claude Code:
  claude mcp add archlint -- archlint serve

Example:
  archlint serve --log-file /tmp/archlint-mcp.log`,
	RunE: runServe,
}

func init() {
	serveCmd.Flags().StringVar(&serveLogFile, "log-file", "", "Log file path (logging disabled by default)")
	rootCmd.AddCommand(serveCmd)
}

func runServe(_ *cobra.Command, _ []string) error {
	server, err := mcp.NewServer(serveLogFile)
	if err != nil {
		return fmt.Errorf("error creating MCP server: %w", err)
	}

	if err := server.Run(); err != nil {
		return fmt.Errorf("MCP server error: %w", err)
	}

	return nil
}
