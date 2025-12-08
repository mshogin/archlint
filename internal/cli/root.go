// Package cli содержит реализацию интерфейса командной строки для archlint.
package cli

import (
	"fmt"
	"os"

	"github.com/mshogin/archlint/pkg/tracer"
	"github.com/spf13/cobra"
)

var (
	version = "0.1.0"
)

var rootCmd = &cobra.Command{
	Use:   "archlint",
	Short: "Инструмент для построения архитектурных графов",
	Long: `archlint - инструмент для построения структурных графов и графов поведения
из исходного кода на языке Go.`,
	Version: version,
}

// Execute запускает выполнение корневой команды CLI.
func Execute() error {
	tracer.Enter("Execute")

	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "Ошибка: %v\n", err)
		tracer.ExitError("Execute", err)

		return fmt.Errorf("ошибка выполнения команды: %w", err)
	}

	tracer.ExitSuccess("Execute")

	return nil
}
