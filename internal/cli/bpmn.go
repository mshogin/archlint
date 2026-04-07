package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/mshogin/archlint/pkg/bpmn"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

var (
	errBpmnFileNotFound = errors.New("file not found")
	errBpmnParse        = errors.New("parse error")
	errBpmnBuildGraph   = errors.New("failed to build graph")
	errBpmnSave         = errors.New("save error")
	errBpmnFileCreate   = errors.New("failed to create file")
	errBpmnYAMLEncode   = errors.New("YAML serialization error")
)

var bpmnOutputFile string

var bpmnCmd = &cobra.Command{
	Use:   "bpmn <file.bpmn>",
	Short: "Parse BPMN 2.0 file into a business process graph",
	Long: `Analyzes a BPMN 2.0 XML file and builds a business process graph.

Supports files from Camunda Modeler, draw.io, Bizagi.

Example:
  archlint bpmn order-process.bpmn -o process-graph.yaml`,
	Args: cobra.ExactArgs(1),
	RunE: runBpmn,
}

func init() {
	bpmnCmd.Flags().StringVarP(&bpmnOutputFile, "output", "o", "process-graph.yaml", "Output YAML file")
	rootCmd.AddCommand(bpmnCmd)
}

func runBpmn(_ *cobra.Command, args []string) error {
	filename := args[0]

	if _, err := os.Stat(filename); os.IsNotExist(err) {
		return fmt.Errorf("%w: %s", errBpmnFileNotFound, filename)
	}

	fmt.Printf("Parsing BPMN: %s\n", filename)

	process, err := bpmn.ParseFile(filename)
	if err != nil {
		return fmt.Errorf("%w: %w", errBpmnParse, err)
	}

	graph, err := bpmn.BuildGraph(process)
	if err != nil {
		return fmt.Errorf("%w: %w", errBpmnBuildGraph, err)
	}

	validationErrors := graph.Validate()

	printBpmnStats(process, validationErrors)

	if err := saveProcessGraph(process); err != nil {
		return fmt.Errorf("%w: %w", errBpmnSave, err)
	}

	fmt.Printf("Graph saved to %s\n", bpmnOutputFile)

	return nil
}

func printBpmnStats(process *bpmn.BPMNProcess, validationErrors []error) {
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

	fmt.Printf("Process: %s (id: %s)\n", process.Name, process.ID)
	fmt.Printf("Elements: %d (events: %d, tasks: %d, gateways: %d)\n",
		len(process.Elements), events, tasks, gateways)
	fmt.Printf("Flows: %d\n", len(process.Flows))

	if len(validationErrors) == 0 {
		fmt.Println("Validation: OK")
	} else {
		fmt.Printf("Validation: %d errors\n", len(validationErrors))

		for _, e := range validationErrors {
			fmt.Printf("  - %s\n", e.Error())
		}
	}
}

func saveProcessGraph(process *bpmn.BPMNProcess) error {
	output := bpmn.ProcessGraphOutput{
		Process: bpmn.ProcessMeta{
			ID:   process.ID,
			Name: process.Name,
		},
		Elements: process.Elements,
		Flows:    process.Flows,
	}

	//nolint:gosec // G304: bpmnOutputFile is a user-provided CLI argument
	file, err := os.OpenFile(bpmnOutputFile, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o640)
	if err != nil {
		return fmt.Errorf("%w: %w", errBpmnFileCreate, err)
	}

	defer func() {
		if closeErr := file.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close file: %v\n", closeErr)
		}
	}()

	encoder := yaml.NewEncoder(file)
	encoder.SetIndent(2)

	defer func() {
		if closeErr := encoder.Close(); closeErr != nil {
			fmt.Fprintf(os.Stderr, "Warning: failed to close encoder: %v\n", closeErr)
		}
	}()

	if err := encoder.Encode(output); err != nil {
		return fmt.Errorf("%w: %w", errBpmnYAMLEncode, err)
	}

	return nil
}
