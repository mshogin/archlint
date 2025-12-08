// Package tracer содержит функции для генерации контекстов и диаграмм из трассировок выполнения.
package tracer

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

var (
	errNoTraceFiles = errors.New("no trace files found")
)

// UMLConfig представляет конфигурацию UML с поддержкой инъекций.
type UMLConfig struct {
	Before string `yaml:"$before,omitempty"`
	After  string `yaml:"$after,omitempty"`
	File   string `yaml:"file,omitempty"`
}

// Context представляет DocHub контекст.
type Context struct {
	Title        string     `yaml:"title"`
	Location     string     `yaml:"location,omitempty"`
	Presentation string     `yaml:"presentation,omitempty"`
	ExtraLinks   bool       `yaml:"extra-links,omitempty"`
	Components   []string   `yaml:"components"`
	UML          *UMLConfig `yaml:"uml,omitempty"`
}

// Contexts коллекция контекстов.
type Contexts map[string]Context

// SequenceDiagram представляет PlantUML sequence диаграмму.
type SequenceDiagram struct {
	TestName     string
	Participants []string
	Calls        []SequenceCall
}

// SequenceCall представляет вызов в sequence диаграмме.
type SequenceCall struct {
	From    string
	To      string
	Success bool
	Error   string
}

// GenerateContextsFromTraces генерирует контексты из трассировочных файлов.
func GenerateContextsFromTraces(traceDir string) (Contexts, error) {
	contexts := make(Contexts)

	files, err := filepath.Glob(filepath.Join(traceDir, "*.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to find trace files: %w", err)
	}

	if len(files) == 0 {
		return nil, fmt.Errorf("%w in %s", errNoTraceFiles, traceDir)
	}

	for _, file := range files {
		trace, err := LoadTrace(file)
		if err != nil {
			return nil, fmt.Errorf("failed to load trace %s: %w", file, err)
		}

		ctx, err := GenerateContextFromTrace(trace)
		if err != nil {
			return nil, fmt.Errorf("failed to generate context from %s: %w", file, err)
		}

		pumlFile := strings.TrimSuffix(file, ".json") + ".puml"
		if err := GenerateSequenceDiagram(trace, pumlFile); err != nil {
			return nil, fmt.Errorf("failed to generate sequence diagram: %w", err)
		}

		contextID := toHierarchicalID("tests", sanitizeContextID(trace.TestName))
		ctx.UML = &UMLConfig{File: pumlFile}
		contexts[contextID] = ctx
	}

	return contexts, nil
}

// LoadTrace загружает трассировку из JSON файла.
func LoadTrace(filename string) (*Trace, error) {
	//nolint:gosec // G304: filename is controlled by GenerateContextsFromTraces via filepath.Glob
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("не удалось прочитать файл трассировки: %w", err)
	}

	var trace Trace
	if err := json.Unmarshal(data, &trace); err != nil {
		return nil, fmt.Errorf("не удалось распарсить JSON трассировки: %w", err)
	}

	return &trace, nil
}

// GenerateContextFromTrace генерирует контекст из трассировки.
func GenerateContextFromTrace(trace *Trace) (Context, error) {
	componentsMap := make(map[string]bool)

	for _, call := range trace.Calls {
		if call.Event == "enter" {
			hierarchicalID := toHierarchicalComponentID(call.Function)
			componentsMap[hierarchicalID] = true
		}
	}

	components := make([]string, 0, len(componentsMap))
	for comp := range componentsMap {
		components = append(components, comp)
	}

	return Context{
		Title:        humanizeTestName(trace.TestName),
		Location:     "Tests/" + humanizeTestName(trace.TestName),
		Presentation: "plantuml",
		ExtraLinks:   false,
		Components:   components,
	}, nil
}

// GenerateSequenceDiagram генерирует PlantUML sequence диаграмму из трассировки.
func GenerateSequenceDiagram(trace *Trace, outputFile string) error {
	diagram := buildSequenceDiagram(trace)
	puml := generatePlantUML(diagram)

	if err := os.WriteFile(outputFile, []byte(puml), 0o600); err != nil {
		return fmt.Errorf("не удалось записать PlantUML файл: %w", err)
	}

	return nil
}

// buildSequenceDiagram строит структуру sequence диаграммы из трассировки.
func buildSequenceDiagram(trace *Trace) *SequenceDiagram {
	diagram := &SequenceDiagram{
		TestName:     trace.TestName,
		Participants: []string{},
		Calls:        []SequenceCall{},
	}

	callStack := []string{}
	participantsMap := make(map[string]bool)

	for _, call := range trace.Calls {
		processCall(call, &callStack, participantsMap, diagram)
	}

	for participant := range participantsMap {
		diagram.Participants = append(diagram.Participants, participant)
	}

	return diagram
}

func processCall(call Call, callStack *[]string, participants map[string]bool, diagram *SequenceDiagram) {
	switch call.Event {
	case "enter":
		processEnterEvent(call, callStack, participants, diagram)
	case "exit_success", "exit_error":
		processExitEvent(call, callStack, diagram)
	}
}

func processEnterEvent(call Call, callStack *[]string, participants map[string]bool, diagram *SequenceDiagram) {
	participants[call.Function] = true

	if len(*callStack) > 0 {
		from := (*callStack)[len(*callStack)-1]
		diagram.Calls = append(diagram.Calls, SequenceCall{
			From:    from,
			To:      call.Function,
			Success: true,
		})
	}

	*callStack = append(*callStack, call.Function)
}

func processExitEvent(call Call, callStack *[]string, diagram *SequenceDiagram) {
	if len(*callStack) > 0 {
		*callStack = (*callStack)[:len(*callStack)-1]
	}

	if call.Event == "exit_error" && len(diagram.Calls) > 0 {
		lastCall := &diagram.Calls[len(diagram.Calls)-1]
		if lastCall.To == call.Function {
			lastCall.Success = false
			lastCall.Error = call.Error
		}
	}
}

// generatePlantUML генерирует PlantUML код для sequence диаграммы.
func generatePlantUML(diagram *SequenceDiagram) string {
	var b strings.Builder

	b.WriteString("@startuml\n")
	b.WriteString(fmt.Sprintf("title %s\n\n", diagram.TestName))

	for _, participant := range diagram.Participants {
		b.WriteString(fmt.Sprintf("participant %q as %s\n",
			shortName(participant),
			sanitizeAlias(participant)))
	}

	b.WriteString("\n")

	for _, call := range diagram.Calls {
		fromAlias := sanitizeAlias(call.From)
		toAlias := sanitizeAlias(call.To)

		if call.Success {
			b.WriteString(fmt.Sprintf("%s -> %s\n", fromAlias, toAlias))
		} else {
			errorMsg := call.Error
			if errorMsg == "" {
				errorMsg = "error"
			}

			b.WriteString(fmt.Sprintf("%s -> %s: ❌ %s\n", fromAlias, toAlias, errorMsg))
		}
	}

	b.WriteString("\n@enduml\n")

	return b.String()
}

// humanizeTestName конвертирует TestProcessOrder -> Process Order.
func humanizeTestName(testName string) string {
	name := strings.TrimPrefix(testName, "Test")

	var result []string

	var current strings.Builder

	for i, r := range name {
		if i > 0 && r >= 'A' && r <= 'Z' {
			if current.Len() > 0 {
				result = append(result, current.String())
				current.Reset()
			}
		}

		current.WriteRune(r)
	}

	if current.Len() > 0 {
		result = append(result, current.String())
	}

	return strings.Join(result, " ")
}

// sanitizeContextID конвертирует TestProcessOrder -> test-process-order.
func sanitizeContextID(testName string) string {
	name := strings.ToLower(testName)
	name = strings.ReplaceAll(name, "_", "-")

	var result []string

	var current strings.Builder

	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || r == '-' {
			current.WriteRune(r)
		} else if current.Len() > 0 {
			result = append(result, current.String())
			current.Reset()
		}
	}

	if current.Len() > 0 {
		result = append(result, current.String())
	}

	return strings.Join(result, "-")
}

// sanitizeAlias создает безопасный alias для PlantUML.
func sanitizeAlias(name string) string {
	alias := strings.ReplaceAll(name, ".", "_")
	alias = strings.ReplaceAll(alias, " ", "_")

	return alias
}

// shortName возвращает короткое имя для участника (последняя часть).
func shortName(fullName string) string {
	parts := strings.Split(fullName, ".")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	return fullName
}

// toHierarchicalID создает иерархический ID в стиле DocHub (prefix.suffix).
func toHierarchicalID(parts ...string) string {
	return strings.Join(parts, ".")
}

// toHierarchicalComponentID преобразует имя функции в иерархический ID компонента.
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

// camelToSnake преобразует CamelCase в snake_case.
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

// MatchComponentPattern проверяет соответствие компонента паттерну с поддержкой wildcards.
func MatchComponentPattern(componentID, pattern string) bool {
	if componentID == pattern {
		return true
	}

	if strings.HasSuffix(pattern, ".**") {
		prefix := strings.TrimSuffix(pattern, ".**")

		return strings.HasPrefix(componentID, prefix+".")
	}

	if strings.HasSuffix(pattern, ".*") {
		prefix := strings.TrimSuffix(pattern, ".*")
		if !strings.HasPrefix(componentID, prefix+".") {
			return false
		}

		suffix := strings.TrimPrefix(componentID, prefix+".")

		return !strings.Contains(suffix, ".")
	}

	return false
}

// ExpandComponentPatterns раскрывает паттерны компонентов в список конкретных компонентов.
func ExpandComponentPatterns(patterns, allComponents []string) []string {
	result := make(map[string]bool)

	for _, pattern := range patterns {
		for _, comp := range allComponents {
			if MatchComponentPattern(comp, pattern) {
				result[comp] = true
			}
		}
	}

	components := make([]string, 0, len(result))
	for comp := range result {
		components = append(components, comp)
	}

	return components
}
