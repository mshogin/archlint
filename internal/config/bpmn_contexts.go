// Package config loads and validates BPMN contexts configuration.
package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/mshogin/archlint/pkg/bpmn"
	"gopkg.in/yaml.v3"
)

const maxConfigFileSize = 10 * 1024 * 1024 // 10MB

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

// Configuration validation errors.
var (
	ErrEmptyContexts    = errors.New("contexts section is empty")
	ErrMissingBPMNFile  = errors.New("missing bpmn_file in context")
	ErrEmptyEvents      = errors.New("events list is empty in context")
	ErrMissingEventID   = errors.New("missing event_id in event")
	ErrMissingPackage   = errors.New("missing entry_point.package")
	ErrMissingFunction  = errors.New("missing entry_point.function")
	ErrInvalidType      = errors.New("invalid entry_point.type")
	ErrDuplicateEventID = errors.New("duplicate event_id within context")
	ErrBPMNFileNotFound = errors.New("bpmn_file not found")
	ErrConfigRead       = errors.New("failed to read config file")
	ErrConfigParse      = errors.New("failed to parse YAML config")
	ErrConfigTooLarge   = errors.New("config file is too large")
	ErrPathTraversal    = errors.New("path escapes base directory")
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
	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, fmt.Errorf("%w: %w", ErrConfigRead, err)
	}

	if info.Size() > maxConfigFileSize {
		return nil, nil, fmt.Errorf("%w: %d bytes (limit %d)", ErrConfigTooLarge, info.Size(), maxConfigFileSize)
	}

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
			return fmt.Errorf("%w: context %q", ErrMissingBPMNFile, ctxName)
		}

		bpmnPath, err := resolvePath(baseDir, ctx.BPMNFile)
		if err != nil {
			return fmt.Errorf("context %q: %w", ctxName, err)
		}

		if _, err := os.Stat(bpmnPath); os.IsNotExist(err) {
			return fmt.Errorf("%w: %s (context %q)", ErrBPMNFileNotFound, ctx.BPMNFile, ctxName)
		}

		if len(ctx.Events) == 0 {
			return fmt.Errorf("%w: context %q", ErrEmptyEvents, ctxName)
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
			return fmt.Errorf("%w: context %q, event #%d", ErrMissingEventID, ctxName, i+1)
		}

		if eventIDs[event.EventID] {
			return fmt.Errorf("%w: %q in context %q", ErrDuplicateEventID, event.EventID, ctxName)
		}

		eventIDs[event.EventID] = true

		if event.EntryPoint.Package == "" {
			return fmt.Errorf("%w: context %q, event %q", ErrMissingPackage, ctxName, event.EventID)
		}

		if event.EntryPoint.Function == "" {
			return fmt.Errorf("%w: context %q, event %q", ErrMissingFunction, ctxName, event.EventID)
		}

		if !validEntryPointTypes[event.EntryPoint.Type] {
			return fmt.Errorf(
				"%w: %q in context %q, event %q (allowed: http, kafka, grpc, cron, custom)",
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
				Message: fmt.Sprintf("event_id %q not found in BPMN file %s", event.EventID, ctx.BPMNFile),
			})
		}
	}

	return warnings
}

// resolvePath разрешает путь относительно базовой директории.
// Возвращает ошибку если путь выходит за пределы baseDir.
func resolvePath(baseDir, path string) (string, error) {
	if filepath.IsAbs(path) {
		return path, nil
	}

	resolved := filepath.Clean(filepath.Join(baseDir, path))

	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to resolve base directory: %w", err)
	}

	absResolved, err := filepath.Abs(resolved)
	if err != nil {
		return "", fmt.Errorf("failed to resolve path: %w", err)
	}

	if !strings.HasPrefix(absResolved, absBase+string(filepath.Separator)) && absResolved != absBase {
		return "", fmt.Errorf("%w: %q relative to %q", ErrPathTraversal, path, baseDir)
	}

	return resolved, nil
}
