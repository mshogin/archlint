package cli

import (
	"fmt"

	"github.com/mshogin/archlint/internal/lsp"
	"github.com/spf13/cobra"
)

var (
	lspLogFile string
	lspVerbose bool
)

var lspCmd = &cobra.Command{
	Use:   "lsp",
	Short: "Запуск LSP-сервера для архитектурного анализа",
	Long: `Запускает Language Server Protocol (LSP) сервер через stdio.

LSP-сервер парсит проект при инициализации, хранит граф зависимостей в памяти
и обновляет его инкрементально при изменении файлов.

Поддерживаемые команды (workspace/executeCommand):
  archlint.analyzeFile    — полный анализ файла (типы, функции, зависимости)
  archlint.analyzeChange  — анализ влияния изменения файла на архитектуру
  archlint.getGraph       — получение текущего графа зависимостей (или подмножества)
  archlint.getMetrics     — метрики для файла/пакета (coupling, instability)

Пример:
  archlint lsp --log-file /tmp/archlint-lsp.log`,
	RunE: runLSP,
}

func init() {
	lspCmd.Flags().StringVar(&lspLogFile, "log-file", "", "Файл для логирования (по умолчанию логи отключены)")
	lspCmd.Flags().BoolVar(&lspVerbose, "verbose", false, "Подробное логирование")
	rootCmd.AddCommand(lspCmd)
}

func runLSP(_ *cobra.Command, _ []string) error {
	server, err := lsp.NewServer(lspLogFile)
	if err != nil {
		return fmt.Errorf("ошибка создания LSP-сервера: %w", err)
	}

	if err := server.Run(); err != nil {
		return fmt.Errorf("ошибка LSP-сервера: %w", err)
	}

	return nil
}
