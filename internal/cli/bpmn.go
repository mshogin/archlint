package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/mshogin/archlint/pkg/bpmn"
	"github.com/mshogin/archlint/pkg/tracer"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	errBpmnFileNotFound = errors.New("файл не найден")
	errBpmnParse        = errors.New("ошибка парсинга")
	errBpmnBuildGraph   = errors.New("ошибка построения графа")
	errBpmnSave         = errors.New("ошибка сохранения")
	errBpmnFileCreate   = errors.New("ошибка создания файла")
	errBpmnYAMLEncode   = errors.New("ошибка сериализации YAML")
)

var bpmnOutputFile string

var bpmnCmd = &cobra.Command{
	Use:   "bpmn <файл.bpmn>",
	Short: "Парсинг BPMN 2.0 файла в граф бизнес-процесса",
	Long: `Анализирует BPMN 2.0 XML файл и строит граф бизнес-процесса.

Поддерживаются файлы из Camunda Modeler, draw.io, Bizagi.

Пример:
  archlint bpmn order-process.bpmn -o process-graph.yaml`,
	Args: cobra.ExactArgs(1),
	RunE: runBpmn,
}

func init() {
	tracer.Enter("init")
	bpmnCmd.Flags().StringVarP(&bpmnOutputFile, "output", "o", "process-graph.yaml", "Выходной YAML файл")
	rootCmd.AddCommand(bpmnCmd)
	tracer.ExitSuccess("init")
}

//nolint:funlen // tracer instrumentation adds lines
func runBpmn(_ *cobra.Command, args []string) error {
	tracer.Enter("runBpmn")

	filename := args[0]

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		tracer.ExitError("runBpmn", err)

		return fmt.Errorf("%w: %s", errBpmnFileNotFound, filename)
	}

	fmt.Printf("Парсинг BPMN: %s\n", filename)

	process, err := bpmn.ParseFile(filename)
	if err != nil {
		tracer.ExitError("runBpmn", err)

		return fmt.Errorf("%w: %w", errBpmnParse, err)
	}

	graph, err := bpmn.BuildGraph(process)
	if err != nil {
		tracer.ExitError("runBpmn", err)

		return fmt.Errorf("%w: %w", errBpmnBuildGraph, err)
	}

	validationErrors := graph.Validate()

	printBpmnStats(process, validationErrors)

	if err := saveProcessGraph(process); err != nil {
		tracer.ExitError("runBpmn", err)

		return fmt.Errorf("%w: %w", errBpmnSave, err)
	}

	fmt.Printf("Граф сохранен в %s\n", bpmnOutputFile)
	tracer.ExitSuccess("runBpmn")

	return nil
}

func printBpmnStats(process *bpmn.BPMNProcess, validationErrors []error) {
	tracer.Enter("printBpmnStats")

	var events, tasks, gateways int

	for _, elem := range process.Elements {
		switch elem.Type {
		case bpmn.StartEvent, bpmn.EndEvent, bpmn.IntermediateCatchEvent, bpmn.IntermediateThrowEvent:
			events++
		case bpmn.Task, bpmn.ServiceTask, bpmn.UserTask:
			tasks++
		case bpmn.ExclusiveGateway, bpmn.ParallelGateway:
			gateways++
		}
	}

	fmt.Printf("Процесс: %s (id: %s)\n", process.Name, process.ID)
	fmt.Printf("Элементов: %d (events: %d, tasks: %d, gateways: %d)\n",
		len(process.Elements), events, tasks, gateways)
	fmt.Printf("Потоков: %d\n", len(process.Flows))

	if len(validationErrors) == 0 {
		fmt.Println("Валидация: OK")
	} else {
		fmt.Printf("Валидация: %d ошибок\n", len(validationErrors))

		for _, e := range validationErrors {
			fmt.Printf("  - %s\n", e.Error())
		}
	}

	tracer.ExitSuccess("printBpmnStats")
}

//nolint:funlen // tracer instrumentation adds lines
func saveProcessGraph(process *bpmn.BPMNProcess) error {
	tracer.Enter("saveProcessGraph")

	output := bpmn.ProcessGraphOutput{
		Process: bpmn.ProcessMeta{
			ID:   process.ID,
			Name: process.Name,
		},
		Elements: process.Elements,
		Flows:    process.Flows,
	}

	//nolint:gosec // G304: bpmnOutputFile is a user-provided CLI argument
	file, err := os.Create(bpmnOutputFile)
	if err != nil {
		tracer.ExitError("saveProcessGraph", err)

		return fmt.Errorf("%w: %w", errBpmnFileCreate, err)
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

	if err := encoder.Encode(output); err != nil {
		tracer.ExitError("saveProcessGraph", err)

		return fmt.Errorf("%w: %w", errBpmnYAMLEncode, err)
	}

	tracer.ExitSuccess("saveProcessGraph")

	return nil
}
