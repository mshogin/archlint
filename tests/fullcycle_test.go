package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/mshogin/archlint/internal/analyzer"
	"github.com/mshogin/archlint/pkg/callgraph"
	"gopkg.in/yaml.v3"
)

// TestFullCycle проверяет полный цикл работы:
// 1. Сбор архитектуры (структурный граф)
// 2. Построение графа вызовов (статический AST анализ)
// 3. Генерация PlantUML и YAML экспорт
func TestFullCycle(t *testing.T) {
	outputDir := "output"
	archFile := filepath.Join(outputDir, "architecture.yaml")

	if err := os.RemoveAll(outputDir); err != nil && !os.IsNotExist(err) {
		t.Fatalf("Failed to remove output dir: %v", err)
	}

	if err := os.MkdirAll(outputDir, 0o750); err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}

	defer os.RemoveAll(outputDir)

	t.Logf("Output directory: %s", outputDir)

	// Step 1: Собираем архитектуру тестового кода
	t.Log("Step 1: Collecting architecture from sample code")
	sampleDir := filepath.Join("testdata", "sample")

	goAnalyzer := analyzer.NewGoAnalyzer()

	graph, err := goAnalyzer.Analyze(sampleDir)
	if err != nil {
		t.Fatalf("Failed to analyze code: %v", err)
	}

	file, err := os.Create(archFile)
	if err != nil {
		t.Fatalf("Failed to create arch file: %v", err)
	}
	defer file.Close()

	encoder := yaml.NewEncoder(file)
	encoder.SetIndent(2)
	defer encoder.Close()

	if err := encoder.Encode(graph); err != nil {
		t.Fatalf("Failed to save architecture: %v", err)
	}

	t.Logf("Architecture saved to %s", archFile)
	t.Logf("Found %d components and %d links", len(graph.Nodes), len(graph.Edges))

	// Step 2: Строим граф вызовов через статический AST анализ
	t.Log("Step 2: Building call graph via static AST analysis")

	builder, err := callgraph.NewBuilder(goAnalyzer, callgraph.BuildOptions{
		MaxDepth:          10,
		ResolveInterfaces: true,
		TrackGoroutines:   true,
	})
	if err != nil {
		t.Fatalf("Failed to create builder: %v", err)
	}

	entryPoint := "testdata/sample.Calculator.Calculate"

	cg, err := builder.Build(entryPoint)
	if err != nil {
		t.Fatalf("Failed to build call graph: %v", err)
	}

	t.Logf("Call graph: %d nodes, %d edges, depth %d",
		cg.Stats.TotalNodes, cg.Stats.TotalEdges, cg.ActualDepth)

	if cg.Stats.TotalNodes == 0 {
		t.Fatal("Call graph has no nodes")
	}

	if cg.Stats.TotalEdges == 0 {
		t.Fatal("Call graph has no edges")
	}

	// Step 3: Генерируем PlantUML диаграмму
	t.Log("Step 3: Generating PlantUML sequence diagram")

	gen := callgraph.NewSequenceGenerator(callgraph.SequenceOptions{
		MaxDepth:       5,
		ShowPackages:   true,
		MarkAsync:      true,
		MarkInterface:  true,
		GroupByPackage: true,
		Title:          "Full Cycle Test: Calculator.Calculate",
	})

	puml, err := gen.Generate(cg)
	if err != nil {
		t.Fatalf("Failed to generate PlantUML: %v", err)
	}

	if puml == "" {
		t.Fatal("PlantUML output is empty")
	}

	pumlPath := filepath.Join(outputDir, "callgraph.puml")

	if err := os.WriteFile(pumlPath, []byte(puml), 0o644); err != nil {
		t.Fatalf("Failed to write PlantUML: %v", err)
	}

	t.Logf("PlantUML saved to %s", pumlPath)

	// Step 4: Экспортируем YAML
	t.Log("Step 4: Exporting call graph as YAML")

	exporter := callgraph.NewYAMLExporter()
	yamlPath := filepath.Join(outputDir, "callgraph.yaml")

	if err := exporter.ExportCallGraph(cg, yamlPath); err != nil {
		t.Fatalf("Failed to export YAML: %v", err)
	}

	t.Logf("YAML saved to %s", yamlPath)

	// Verify
	t.Log("Step 5: Verifying outputs")

	for _, path := range []string{archFile, pumlPath, yamlPath} {
		info, err := os.Stat(path)
		if err != nil {
			t.Errorf("Output file missing: %s", path)
		} else if info.Size() == 0 {
			t.Errorf("Output file empty: %s", path)
		}
	}

	t.Log("Full cycle test completed successfully!")
}
