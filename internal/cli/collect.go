package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/mshogin/archlint/internal/analyzer"
	"github.com/mshogin/archlint/internal/model"
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
	collectCmd.Flags().StringVarP(&collectOutputFile, "output", "o", "architecture.yaml", "Выходной YAML файл")
	collectCmd.Flags().StringVarP(&collectLanguage, "language", "l", "go", "Язык программирования (go)")
	rootCmd.AddCommand(collectCmd)
}

func runCollect(cmd *cobra.Command, args []string) error {
	codeDir := args[0]

	if _, err := os.Stat(codeDir); os.IsNotExist(err) {
		return fmt.Errorf("%w: %s", errDirNotExist, codeDir)
	}

	fmt.Printf("Анализ кода: %s (язык: %s)\n", codeDir, collectLanguage)

	graph, err := analyzeCode(codeDir)
	if err != nil {
		return err
	}

	printStats(graph)

	if err := saveGraph(graph); err != nil {
		return err
	}

	fmt.Printf("Граф сохранен в %s\n", collectOutputFile)

	return nil
}

func analyzeCode(codeDir string) (*model.Graph, error) {
	switch collectLanguage {
	case "go":
		goAnalyzer := analyzer.NewGoAnalyzer()

		graph, err := goAnalyzer.Analyze(codeDir)
		if err != nil {
			return nil, fmt.Errorf("ошибка анализа: %w", err)
		}

		return graph, nil
	default:
		return nil, fmt.Errorf("%w: %s", errUnsupportedLang, collectLanguage)
	}
}

func printStats(graph *model.Graph) {
	stats := make(map[string]int)

	for _, node := range graph.Nodes {
		stats[node.Entity]++
	}

	fmt.Printf("Найдено компонентов: %d\n", len(graph.Nodes))

	for entity, count := range stats {
		fmt.Printf("  - %s: %d\n", entity, count)
	}

	fmt.Printf("Найдено связей: %d\n", len(graph.Edges))
}

func saveGraph(graph *model.Graph) error {
	//nolint:gosec // G304: collectOutputFile is a user-provided CLI argument
	file, err := os.OpenFile(collectOutputFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o640)
	if err != nil {
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
		return fmt.Errorf("%w: %w", errYAMLSerialization, err)
	}

	return nil
}
