// Package config загружает и валидирует конфигурацию BPMN-контекстов.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/mshogin/archlint/pkg/bpmn"
	"gopkg.in/yaml.v3"
)

// EntryPointType определяет тип точки входа в коде.
type EntryPointType string

// Допустимые типы точек входа.
const (
	EntryPointHTTP   EntryPointType = "http"
	EntryPointKafka  EntryPointType = "kafka"
	EntryPointGRPC   EntryPointType = "grpc"
	EntryPointCron   EntryPointType = "cron"
	EntryPointCustom EntryPointType = "custom"
)

// validEntryPointTypes содержит все допустимые значения EntryPointType.
var validEntryPointTypes = map[EntryPointType]bool{
	EntryPointHTTP:   true,
	EntryPointKafka:  true,
	EntryPointGRPC:   true,
	EntryPointCron:   true,
	EntryPointCustom: true,
}

// Ошибки валидации конфигурации.
var (
	ErrEmptyContexts    = errors.New("секция contexts пуста")
	ErrMissingBPMNFile  = errors.New("отсутствует bpmn_file в контексте")
	ErrEmptyEvents      = errors.New("список events пуст в контексте")
	ErrMissingEventID   = errors.New("отсутствует event_id в событии")
	ErrMissingPackage   = errors.New("отсутствует entry_point.package")
	ErrMissingFunction  = errors.New("отсутствует entry_point.function")
	ErrInvalidType      = errors.New("невалидный entry_point.type")
	ErrDuplicateEventID = errors.New("дубликат event_id внутри контекста")
	ErrBPMNFileNotFound = errors.New("файл bpmn_file не найден")
	ErrConfigRead       = errors.New("ошибка чтения файла конфигурации")
	ErrConfigParse      = errors.New("ошибка парсинга YAML конфигурации")
)

// Warning представляет предупреждение валидации (не блокирует работу).
type Warning struct {
	Context string
	Message string
}

// String возвращает строковое представление предупреждения.
func (w Warning) String() string {
	if w.Context != "" {
		return fmt.Sprintf("[%s] %s", w.Context, w.Message)
	}

	return w.Message
}

// EntryPoint описывает точку входа в Go-коде.
type EntryPoint struct {
	Package  string         `yaml:"package"`
	Function string         `yaml:"function"`
	Type     EntryPointType `yaml:"type"`
}

// EventMapping описывает маппинг BPMN-события на точку входа в коде.
type EventMapping struct {
	EventID    string     `yaml:"event_id"`
	EventName  string     `yaml:"event_name,omitempty"`
	EntryPoint EntryPoint `yaml:"entry_point"`
}

// BPMNContext описывает контекст одного BPMN-файла с набором событий.
type BPMNContext struct {
	BPMNFile string         `yaml:"bpmn_file"`
	Events   []EventMapping `yaml:"events"`
}

// BehavioralConfig - корневая структура конфигурации BPMN-контекстов.
type BehavioralConfig struct {
	Contexts map[string]BPMNContext `yaml:"contexts"`
}

// LoadBPMNContexts загружает и валидирует конфигурацию контекстов из YAML файла.
func LoadBPMNContexts(path string) (*BehavioralConfig, []Warning, error) {
	//nolint:gosec // G304: path is a user-provided CLI argument
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", ErrConfigRead, err)
	}

	var config BehavioralConfig
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, nil, fmt.Errorf("%w: %w", ErrConfigParse, err)
	}

	if len(config.Contexts) == 0 {
		return nil, nil, ErrEmptyContexts
	}

	baseDir := filepath.Dir(path)

	if err := validateContexts(&config, baseDir); err != nil {
		return nil, nil, err
	}

	return &config, nil, nil
}

// validateContexts проверяет все контексты конфигурации.
func validateContexts(config *BehavioralConfig, baseDir string) error {
	for ctxName, ctx := range config.Contexts {
		if ctx.BPMNFile == "" {
			return fmt.Errorf("%w: контекст %q", ErrMissingBPMNFile, ctxName)
		}

		bpmnPath := resolvePath(baseDir, ctx.BPMNFile)
		if _, err := os.Stat(bpmnPath); os.IsNotExist(err) {
			return fmt.Errorf("%w: %s (контекст %q)", ErrBPMNFileNotFound, ctx.BPMNFile, ctxName)
		}

		if len(ctx.Events) == 0 {
			return fmt.Errorf("%w: контекст %q", ErrEmptyEvents, ctxName)
		}

		if err := validateEvents(ctxName, ctx.Events); err != nil {
			return err
		}
	}

	return nil
}

// validateEvents проверяет список событий внутри контекста.
func validateEvents(ctxName string, events []EventMapping) error {
	eventIDs := make(map[string]bool)

	for i, event := range events {
		if event.EventID == "" {
			return fmt.Errorf("%w: контекст %q, событие #%d", ErrMissingEventID, ctxName, i+1)
		}

		if eventIDs[event.EventID] {
			return fmt.Errorf("%w: %q в контексте %q", ErrDuplicateEventID, event.EventID, ctxName)
		}

		eventIDs[event.EventID] = true

		if event.EntryPoint.Package == "" {
			return fmt.Errorf("%w: контекст %q, событие %q", ErrMissingPackage, ctxName, event.EventID)
		}

		if event.EntryPoint.Function == "" {
			return fmt.Errorf("%w: контекст %q, событие %q", ErrMissingFunction, ctxName, event.EventID)
		}

		if !validEntryPointTypes[event.EntryPoint.Type] {
			return fmt.Errorf(
				"%w: %q в контексте %q, событие %q (допустимые: http, kafka, grpc, cron, custom)",
				ErrInvalidType, event.EntryPoint.Type, ctxName, event.EventID,
			)
		}
	}

	return nil
}

// ValidateAgainstBPMN проверяет что event_id из конфигурации существуют в BPMN-файле.
func ValidateAgainstBPMN(ctxName string, ctx *BPMNContext, process *bpmn.BPMNProcess) []Warning {
	elementIDs := make(map[string]bool)
	for _, elem := range process.Elements {
		elementIDs[elem.ID] = true
	}

	var warnings []Warning

	for _, event := range ctx.Events {
		if !elementIDs[event.EventID] {
			warnings = append(warnings, Warning{
				Context: ctxName,
				Message: fmt.Sprintf("event_id %q не найден в BPMN-файле %s", event.EventID, ctx.BPMNFile),
			})
		}
	}

	return warnings
}

// resolvePath разрешает путь относительно базовой директории.
func resolvePath(baseDir, path string) string {
	if filepath.IsAbs(path) {
		return path
	}

	return filepath.Join(baseDir, path)
}
