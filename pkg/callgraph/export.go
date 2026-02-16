package callgraph

import (
	"fmt"
	"os"

	"github.com/mshogin/archlint/pkg/tracer"
	"gopkg.in/yaml.v3"
)

// YAMLExporter сериализует графы вызовов в YAML.
type YAMLExporter struct{}

// NewYAMLExporter создает новый экспортер.
func NewYAMLExporter() *YAMLExporter {
	return &YAMLExporter{}
}

// ExportCallGraph сериализует один граф вызовов в YAML файл.
func (e *YAMLExporter) ExportCallGraph(cg *CallGraph, path string) error {
	tracer.Enter("YAMLExporter.ExportCallGraph")

	if err := e.writeYAML(cg, path); err != nil {
		tracer.ExitError("YAMLExporter.ExportCallGraph", err)

		return err
	}

	tracer.ExitSuccess("YAMLExporter.ExportCallGraph")

	return nil
}

// ExportEventSet сериализует набор графов вызовов в YAML файл.
func (e *YAMLExporter) ExportEventSet(set *EventCallGraphSet, path string) error {
	tracer.Enter("YAMLExporter.ExportEventSet")

	if err := e.writeYAML(set, path); err != nil {
		tracer.ExitError("YAMLExporter.ExportEventSet", err)

		return err
	}

	tracer.ExitSuccess("YAMLExporter.ExportEventSet")

	return nil
}

// MarshalCallGraph сериализует граф вызовов в YAML bytes.
func (e *YAMLExporter) MarshalCallGraph(cg *CallGraph) ([]byte, error) {
	tracer.Enter("YAMLExporter.MarshalCallGraph")

	data, err := yaml.Marshal(cg)
	if err != nil {
		tracer.ExitError("YAMLExporter.MarshalCallGraph", err)

		return nil, fmt.Errorf("ошибка сериализации CallGraph: %w", err)
	}

	tracer.ExitSuccess("YAMLExporter.MarshalCallGraph")

	return data, nil
}

// MarshalEventSet сериализует набор графов в YAML bytes.
func (e *YAMLExporter) MarshalEventSet(set *EventCallGraphSet) ([]byte, error) {
	tracer.Enter("YAMLExporter.MarshalEventSet")

	data, err := yaml.Marshal(set)
	if err != nil {
		tracer.ExitError("YAMLExporter.MarshalEventSet", err)

		return nil, fmt.Errorf("ошибка сериализации EventCallGraphSet: %w", err)
	}

	tracer.ExitSuccess("YAMLExporter.MarshalEventSet")

	return data, nil
}

func (e *YAMLExporter) writeYAML(data interface{}, path string) error {
	//nolint:gosec // G304: path is user-provided CLI argument
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("ошибка создания файла %s: %w", path, err)
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

	if err := encoder.Encode(data); err != nil {
		return fmt.Errorf("ошибка записи YAML в %s: %w", path, err)
	}

	return nil
}
