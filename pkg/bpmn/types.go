// Package bpmn содержит типы и функции для работы с BPMN 2.0 бизнес-процессами.
package bpmn

import "errors"

// ElementType определяет тип BPMN элемента.
type ElementType string

// Типы BPMN элементов.
const (
	StartEvent             ElementType = "startEvent"
	EndEvent               ElementType = "endEvent"
	IntermediateCatchEvent ElementType = "intermediateCatchEvent"
	IntermediateThrowEvent ElementType = "intermediateThrowEvent"
	Task                   ElementType = "task"
	ServiceTask            ElementType = "serviceTask"
	UserTask               ElementType = "userTask"
	ExclusiveGateway       ElementType = "exclusiveGateway"
	ParallelGateway        ElementType = "parallelGateway"
)

// EventType определяет подтип события.
type EventType string

// Подтипы событий.
const (
	EventNone    EventType = "none"
	EventTimer   EventType = "timer"
	EventMessage EventType = "message"
	EventSignal  EventType = "signal"
	EventError   EventType = "error"
)

// Ошибки парсинга и валидации.
var (
	ErrNoProcesses     = errors.New("BPMN файл не содержит процессов")
	ErrNilProcess      = errors.New("процесс не может быть nil")
	ErrNoStartEvent    = errors.New("процесс не содержит startEvent")
	ErrNoEndEvent      = errors.New("процесс не содержит endEvent")
	ErrBrokenRef       = errors.New("ссылка на несуществующий элемент")
	ErrIsolatedElement = errors.New("элемент изолирован")
)

// BPMNProcess представляет бизнес-процесс.
type BPMNProcess struct { //nolint:revive // имя из спецификации, BPMN-префикс отличает от model.Graph
	ID       string        `yaml:"id"`
	Name     string        `yaml:"name"`
	Elements []BPMNElement `yaml:"elements"`
	Flows    []BPMNFlow    `yaml:"flows"`
}

// BPMNElement представляет элемент процесса (событие, задача, шлюз).
type BPMNElement struct { //nolint:revive // имя из спецификации
	ID        string      `yaml:"id"`
	Name      string      `yaml:"name"`
	Type      ElementType `yaml:"type"`
	EventType EventType   `yaml:"event_type,omitempty"`
}

// BPMNFlow представляет поток управления между элементами.
type BPMNFlow struct { //nolint:revive // имя из спецификации
	ID        string `yaml:"id"`
	Name      string `yaml:"name,omitempty"`
	SourceRef string `yaml:"source_ref"`
	TargetRef string `yaml:"target_ref"`
}

// ProcessGraphOutput - формат YAML вывода.
type ProcessGraphOutput struct {
	Process  ProcessMeta   `yaml:"process"`
	Elements []BPMNElement `yaml:"elements"`
	Flows    []BPMNFlow    `yaml:"flows"`
}

// ProcessMeta содержит метаданные процесса для YAML вывода.
type ProcessMeta struct {
	ID   string `yaml:"id"`
	Name string `yaml:"name"`
}
