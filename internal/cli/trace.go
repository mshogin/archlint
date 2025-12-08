package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/mshogin/archlint/pkg/tracer"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	traceOutputFile string
)

var (
	errTraceDirNotExist = errors.New("директория не существует")
)

var traceCmd = &cobra.Command{
	Use:   "trace [директория с трассировками]",
	Short: "Генерирует контексты из трассировок тестов",
	Long: `Анализирует JSON файлы трассировок и генерирует DocHub контексты.

Каждая трассировка представляет один execution flow (тест) и конвертируется в:
1. DocHub контекст с списком компонентов
2. PlantUML sequence диаграмму

Пример:
  archlint trace ./tests/traces -o contexts.yaml

После запуска тестов с трассировкой:
  go test -v ./...

Сгенерируются файлы:
  - contexts.yaml (контексты для DocHub)
  - *.puml (sequence диаграммы для каждого теста)
`,
	Args: cobra.ExactArgs(1),
	RunE: runTrace,
}

func init() {
	tracer.Enter("init")
	traceCmd.Flags().StringVarP(&traceOutputFile, "output", "o", "contexts.yaml", "Выходной YAML файл для контекстов")
	rootCmd.AddCommand(traceCmd)
	tracer.ExitSuccess("init")
}

func runTrace(cmd *cobra.Command, args []string) error {
	tracer.Enter("runTrace")

	traceDir := args[0]

	if _, err := os.Stat(traceDir); os.IsNotExist(err) {
		tracer.ExitError("runTrace", err)

		return fmt.Errorf("%w: %s", errTraceDirNotExist, traceDir)
	}

	fmt.Printf("Генерация контекстов из трассировок: %s\n", traceDir)

	contexts, err := tracer.GenerateContextsFromTraces(traceDir)
	if err != nil {
		tracer.ExitError("runTrace", err)

		return fmt.Errorf("ошибка генерации контекстов: %w", err)
	}

	if err := saveContexts(contexts); err != nil {
		tracer.ExitError("runTrace", err)

		return err
	}

	printContextsInfo(contexts)
	tracer.ExitSuccess("runTrace")

	return nil
}

func saveContexts(contexts tracer.Contexts) error {
	tracer.Enter("saveContexts")
	fmt.Printf("Найдено трассировок: %d\n", len(contexts))

	output := struct {
		Contexts tracer.Contexts `yaml:"contexts"`
	}{
		Contexts: contexts,
	}

	data, err := yaml.Marshal(output)
	if err != nil {
		tracer.ExitError("saveContexts", err)

		return fmt.Errorf("ошибка сериализации YAML: %w", err)
	}

	if err := os.WriteFile(traceOutputFile, data, 0o600); err != nil {
		tracer.ExitError("saveContexts", err)

		return fmt.Errorf("ошибка записи файла: %w", err)
	}

	fmt.Printf("✓ Контексты сохранены в %s\n", traceOutputFile)
	tracer.ExitSuccess("saveContexts")

	return nil
}

func printContextsInfo(contexts tracer.Contexts) {
	tracer.Enter("printContextsInfo")
	fmt.Println("\nСгенерированные контексты:")

	for id, ctx := range contexts {
		fmt.Printf("  - %s: %s\n", id, ctx.Title)
		fmt.Printf("    Компонентов: %d\n", len(ctx.Components))
		fmt.Printf("    UML: %s\n", ctx.UML.File)
	}

	tracer.ExitSuccess("printContextsInfo")
}
