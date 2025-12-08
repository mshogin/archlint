package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/mshogin/archlint/internal/analyzer"
	"github.com/mshogin/archlint/internal/model"
	"github.com/mshogin/archlint/pkg/tracer"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	collectOutputFile string
	collectLanguage   string
)

var (
	errDirNotExist       = errors.New("директория не существует")
	errUnsupportedLang   = errors.New("неподдерживаемый язык")
	errFileCreate        = errors.New("ошибка создания файла")
	errYAMLSerialization = errors.New("ошибка сериализации YAML")
)

var collectCmd = &cobra.Command{
	Use:   "collect [директория]",
	Short: "Сбор архитектуры из исходного кода",
	Long: `Анализирует исходный код и строит граф архитектуры в формате YAML.

Пример:
  archlint collect . -l go -o architecture.yaml`,
	Args: cobra.ExactArgs(1),
	RunE: runCollect,
}

func init() {
	tracer.Enter("init")
	collectCmd.Flags().StringVarP(&collectOutputFile, "output", "o", "architecture.yaml", "Выходной YAML файл")
	collectCmd.Flags().StringVarP(&collectLanguage, "language", "l", "go", "Язык программирования (go)")
	rootCmd.AddCommand(collectCmd)
	tracer.ExitSuccess("init")
}

func runCollect(cmd *cobra.Command, args []string) error {
	tracer.Enter("runCollect")

	codeDir := args[0]

	if _, err := os.Stat(codeDir); os.IsNotExist(err) {
		tracer.ExitError("runCollect", err)

		return fmt.Errorf("%w: %s", errDirNotExist, codeDir)
	}

	fmt.Printf("Анализ кода: %s (язык: %s)\n", codeDir, collectLanguage)

	graph, err := analyzeCode(codeDir)
	if err != nil {
		tracer.ExitError("runCollect", err)

		return err
	}

	printStats(graph)

	if err := saveGraph(graph); err != nil {
		tracer.ExitError("runCollect", err)

		return err
	}

	fmt.Printf("✓ Граф сохранен в %s\n", collectOutputFile)
	tracer.ExitSuccess("runCollect")

	return nil
}

func analyzeCode(codeDir string) (*model.Graph, error) {
	tracer.Enter("analyzeCode")

	switch collectLanguage {
	case "go":
		goAnalyzer := analyzer.NewGoAnalyzer()

		graph, err := goAnalyzer.Analyze(codeDir)
		if err != nil {
			err = fmt.Errorf("ошибка анализа: %w", err)
			tracer.ExitError("analyzeCode", err)

			return nil, err
		}

		tracer.ExitSuccess("analyzeCode")

		return graph, nil
	default:
		err := fmt.Errorf("%w: %s", errUnsupportedLang, collectLanguage)
		tracer.ExitError("analyzeCode", err)

		return nil, err
	}
}

func printStats(graph *model.Graph) {
	tracer.Enter("printStats")

	stats := make(map[string]int)

	for _, node := range graph.Nodes {
		stats[node.Entity]++
	}

	fmt.Printf("Найдено компонентов: %d\n", len(graph.Nodes))

	for entity, count := range stats {
		fmt.Printf("  - %s: %d\n", entity, count)
	}

	fmt.Printf("Найдено связей: %d\n", len(graph.Edges))
	tracer.ExitSuccess("printStats")
}

func saveGraph(graph *model.Graph) error {
	tracer.Enter("saveGraph")
	//nolint:gosec // G304: collectOutputFile is a user-provided CLI argument, file path control is expected
	file, err := os.Create(collectOutputFile)
	if err != nil {
		tracer.ExitError("saveGraph", err)

		return fmt.Errorf("%w: %w", errFileCreate, err)
	}

	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "Предупреждение: ошибка закрытия файла: %v\n", closeErr)
		}
	}()

	encoder := yaml.NewEncoder(file)
	encoder.SetIndent(2)

	defer func() {
		if closeErr := encoder.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "Предупреждение: ошибка закрытия encoder: %v\n", closeErr)
		}
	}()

	if err := encoder.Encode(graph); err != nil {
		tracer.ExitError("saveGraph", err)

		return fmt.Errorf("%w: %w", errYAMLSerialization, err)
	}

	tracer.ExitSuccess("saveGraph")

	return nil
}
