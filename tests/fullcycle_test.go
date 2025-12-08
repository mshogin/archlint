package tests

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mshogin/archlint/internal/analyzer"
	"github.com/mshogin/archlint/pkg/tracer"
	"gopkg.in/yaml.v3"
)

// TestFullCycle проверяет полный цикл работы:
// 1. Сбор архитектуры
// 2. Запуск теста с трассировкой
// 3. Генерация контекста из трейса
// 4. Проверка соответствия компонентов
func TestFullCycle(t *testing.T) {
	// Подготовка: создаем директорию для сохранения результатов
	outputDir := "output"
	archFile := filepath.Join(outputDir, "architecture.yaml")
	traceDir := filepath.Join(outputDir, "traces")
	contextsFile := filepath.Join(outputDir, "contexts.yaml")

	// Очищаем и создаем директорию
	if err := os.RemoveAll(outputDir); err != nil && !os.IsNotExist(err) {
		t.Fatalf("Failed to remove output dir: %v", err)
	}
	if err := os.MkdirAll(traceDir, 0755); err != nil {
		t.Fatalf("Failed to create output dir: %v", err)
	}

	t.Logf("Output directory: %s", outputDir)

	// Шаг 1: Собираем архитектуру тестового кода
	t.Log("Step 1: Collecting architecture from sample code")
	sampleDir := filepath.Join("testdata", "sample")

	goAnalyzer := analyzer.NewGoAnalyzer()
	graph, err := goAnalyzer.Analyze(sampleDir)
	if err != nil {
		t.Fatalf("Failed to analyze code: %v", err)
	}

	// Сохраняем архитектуру
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

	// Шаг 2: Запускаем тест с трассировкой
	t.Log("Step 2: Running traced test")

	// Создаем директорию для трейсов в testdata/sample
	sampleTraceDir := filepath.Join(sampleDir, "traces")
	if err := os.MkdirAll(sampleTraceDir, 0755); err != nil {
		t.Fatalf("Failed to create sample trace dir: %v", err)
	}
	defer os.RemoveAll(sampleTraceDir)

	// Получаем абсолютный путь к sample директории
	absSampleDir, err := filepath.Abs(sampleDir)
	if err != nil {
		t.Fatalf("Failed to get absolute path: %v", err)
	}

	// Запускаем тест из sample директории
	cmd := exec.Command("go", "test", "-v", "-run", "TestCalculateWithTrace")
	cmd.Dir = absSampleDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Logf("Test output: %s", output)
		t.Fatalf("Failed to run traced test: %v", err)
	}
	t.Logf("Test output: %s", output)

	// Шаг 3: Проверяем что трейс-файл создан
	t.Log("Step 3: Checking trace file exists")
	traceFile := filepath.Join(sampleTraceDir, "test_calculate.json")
	if _, err := os.Stat(traceFile); os.IsNotExist(err) {
		t.Fatalf("Trace file not created: %s", traceFile)
	}
	t.Logf("Trace file found: %s", traceFile)

	// Загружаем и проверяем трейс
	traceData, err := os.ReadFile(traceFile)
	if err != nil {
		t.Fatalf("Failed to read trace file: %v", err)
	}

	var trace tracer.Trace
	if err := json.Unmarshal(traceData, &trace); err != nil {
		t.Fatalf("Failed to parse trace: %v", err)
	}

	t.Logf("Trace contains %d calls", len(trace.Calls))

	// Извлекаем уникальные функции из трейса
	tracedFunctions := make(map[string]bool)
	for _, call := range trace.Calls {
		if call.Event == "enter" {
			tracedFunctions[call.Function] = true
		}
	}

	t.Logf("Traced functions: %v", tracedFunctions)

	// Ожидаемые функции в трейсе
	expectedFunctions := []string{
		"sample.NewCalculator",
		"sample.Calculator.Calculate",
	}

	for _, fn := range expectedFunctions {
		if !tracedFunctions[fn] {
			t.Errorf("Expected function %s not found in trace", fn)
		}
	}

	// Шаг 4: Генерируем контекст из трейса
	t.Log("Step 4: Generating context from trace")

	// Копируем трейс во временную директорию
	tempTraceFile := filepath.Join(traceDir, "test_calculate.json")
	if err := copyFile(traceFile, tempTraceFile); err != nil {
		t.Fatalf("Failed to copy trace file: %v", err)
	}

	contexts, err := tracer.GenerateContextsFromTraces(traceDir)
	if err != nil {
		t.Fatalf("Failed to generate contexts: %v", err)
	}

	if len(contexts) == 0 {
		t.Fatal("No contexts generated")
	}

	t.Logf("Generated %d contexts", len(contexts))

	// Сохраняем контексты
	contextsOutput := struct {
		Contexts tracer.Contexts `yaml:"contexts"`
	}{
		Contexts: contexts,
	}

	ctxFile, err := os.Create(contextsFile)
	if err != nil {
		t.Fatalf("Failed to create contexts file: %v", err)
	}
	defer ctxFile.Close()

	ctxEncoder := yaml.NewEncoder(ctxFile)
	ctxEncoder.SetIndent(2)
	defer ctxEncoder.Close()

	if err := ctxEncoder.Encode(contextsOutput); err != nil {
		t.Fatalf("Failed to save contexts: %v", err)
	}

	// Шаг 5: Проверяем что все компоненты из трейса есть в контексте
	t.Log("Step 5: Verifying all traced components are in context")

	// Получаем первый контекст
	var contextID string
	var context tracer.Context
	for id, ctx := range contexts {
		contextID = id
		context = ctx
		break
	}

	t.Logf("Context ID: %s", contextID)
	t.Logf("Context title: %s", context.Title)
	t.Logf("Context components: %v", context.Components)

	// Проверяем что все трассированные функции присутствуют в компонентах контекста
	contextComponentsMap := make(map[string]bool)
	for _, comp := range context.Components {
		contextComponentsMap[comp] = true
	}

	// Проверяем каждую трассированную функцию
	for fn := range tracedFunctions {
		// Преобразуем имя функции в иерархический ID
		hierarchicalID := toHierarchicalComponentID(fn)

		found := false
		for comp := range contextComponentsMap {
			// Проверяем точное совпадение или что компонент является частью функции
			if comp == hierarchicalID || strings.Contains(comp, hierarchicalID) || strings.Contains(hierarchicalID, comp) {
				found = true
				break
			}
		}

		if !found {
			t.Errorf("Traced function %s (hierarchical ID: %s) not found in context components", fn, hierarchicalID)
		} else {
			t.Logf("✓ Function %s found in context", fn)
		}
	}

	// Проверяем что создана PlantUML диаграмма
	if context.UML == nil || context.UML.File == "" {
		t.Error("PlantUML diagram not generated")
	} else {
		t.Logf("✓ PlantUML diagram: %s", context.UML.File)

		// Проверяем что файл существует
		if _, err := os.Stat(context.UML.File); os.IsNotExist(err) {
			t.Errorf("PlantUML file does not exist: %s", context.UML.File)
		}
	}

	t.Log("Full cycle test completed successfully!")
	t.Log("")
	t.Log("Saved files:")
	t.Logf("  - Architecture: %s", archFile)
	t.Logf("  - Contexts: %s", contextsFile)
	t.Logf("  - Trace: %s", tempTraceFile)
	if context.UML != nil && context.UML.File != "" {
		t.Logf("  - PlantUML: %s", context.UML.File)
	}
}

// copyFile копирует файл из src в dst
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// toHierarchicalComponentID преобразует имя функции в иерархический ID компонента
// Копия из pkg/tracer/context_generator.go для использования в тесте
func toHierarchicalComponentID(functionName string) string {
	name := strings.ReplaceAll(functionName, "/", ".")
	parts := strings.Split(name, ".")
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		snakePart := camelToSnake(part)
		if snakePart != "" {
			result = append(result, snakePart)
		}
	}

	return strings.Join(result, ".")
}

// camelToSnake преобразует CamelCase в snake_case
func camelToSnake(s string) string {
	if s == "" {
		return ""
	}

	var result strings.Builder
	for i, r := range s {
		if i > 0 && r >= 'A' && r <= 'Z' {
			result.WriteRune('_')
		}
		result.WriteRune(r)
	}
	return strings.ToLower(result.String())
}
