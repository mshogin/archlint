package bpmn

import (
	"fmt"
	"os"

	"github.com/olive-io/bpmn/schema"
)

// ParseFile читает BPMN 2.0 XML файл и возвращает BPMNProcess.
func ParseFile(filename string) (*BPMNProcess, error) {
	//nolint:gosec // G304: filename is a user-provided CLI argument, file path control is expected
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("чтение BPMN файла: %w", err)
	}

	return Parse(data)
}

// Parse парсит BPMN 2.0 XML через schema.Parse() и конвертирует в BPMNProcess.
func Parse(data []byte) (*BPMNProcess, error) {
	defs, err := schema.Parse(data)
	if err != nil {
		return nil, fmt.Errorf("парсинг BPMN XML: %w", err)
	}

	return convertDefinitions(defs)
}

// convertDefinitions конвертирует schema.Definitions -> BPMNProcess.
// Берет первый процесс из definitions (типичный кейс).
func convertDefinitions(defs *schema.Definitions) (*BPMNProcess, error) {
	processes := defs.Processes()
	if processes == nil || len(*processes) == 0 {
		return nil, ErrNoProcesses
	}

	return convertProcess(&(*processes)[0])
}

// convertProcess конвертирует schema.Process -> BPMNProcess.
func convertProcess(proc *schema.Process) (*BPMNProcess, error) {
	result := &BPMNProcess{
		ID:   getID(proc),
		Name: getName(proc),
	}

	result.Elements = collectElements(proc)
	result.Flows = collectFlows(proc)

	return result, nil
}

// collectElements собирает все элементы процесса.
//
//nolint:funlen,gocyclo // Collecting all BPMN element types requires iterating each type.
func collectElements(proc *schema.Process) []BPMNElement {
	var elements []BPMNElement

	if starts := proc.StartEvents(); starts != nil {
		for i := range *starts {
			e := &(*starts)[i]
			elements = append(elements, BPMNElement{
				ID:        getID(e),
				Name:      getName(e),
				Type:      StartEvent,
				EventType: detectCatchEventType(e),
			})
		}
	}

	if ends := proc.EndEvents(); ends != nil {
		for i := range *ends {
			e := &(*ends)[i]
			elements = append(elements, BPMNElement{
				ID:        getID(e),
				Name:      getName(e),
				Type:      EndEvent,
				EventType: detectThrowEventType(e),
			})
		}
	}

	if catches := proc.IntermediateCatchEvents(); catches != nil {
		for i := range *catches {
			e := &(*catches)[i]
			elements = append(elements, BPMNElement{
				ID:        getID(e),
				Name:      getName(e),
				Type:      IntermediateCatchEvent,
				EventType: detectCatchEventType(e),
			})
		}
	}

	if throws := proc.IntermediateThrowEvents(); throws != nil {
		for i := range *throws {
			e := &(*throws)[i]
			elements = append(elements, BPMNElement{
				ID:        getID(e),
				Name:      getName(e),
				Type:      IntermediateThrowEvent,
				EventType: detectThrowEventType(e),
			})
		}
	}

	if tasks := proc.Tasks(); tasks != nil {
		for i := range *tasks {
			e := &(*tasks)[i]
			elements = append(elements, BPMNElement{
				ID:   getID(e),
				Name: getName(e),
				Type: Task,
			})
		}
	}

	if stasks := proc.ServiceTasks(); stasks != nil {
		for i := range *stasks {
			e := &(*stasks)[i]
			elements = append(elements, BPMNElement{
				ID:   getID(e),
				Name: getName(e),
				Type: ServiceTask,
			})
		}
	}

	if utasks := proc.UserTasks(); utasks != nil {
		for i := range *utasks {
			e := &(*utasks)[i]
			elements = append(elements, BPMNElement{
				ID:   getID(e),
				Name: getName(e),
				Type: UserTask,
			})
		}
	}

	if egws := proc.ExclusiveGateways(); egws != nil {
		for i := range *egws {
			e := &(*egws)[i]
			elements = append(elements, BPMNElement{
				ID:   getID(e),
				Name: getName(e),
				Type: ExclusiveGateway,
			})
		}
	}

	if pgws := proc.ParallelGateways(); pgws != nil {
		for i := range *pgws {
			e := &(*pgws)[i]
			elements = append(elements, BPMNElement{
				ID:   getID(e),
				Name: getName(e),
				Type: ParallelGateway,
			})
		}
	}

	return elements
}

// collectFlows собирает все sequence flows процесса.
func collectFlows(proc *schema.Process) []BPMNFlow {
	seqFlows := proc.SequenceFlows()
	if seqFlows == nil {
		return nil
	}

	flows := make([]BPMNFlow, 0, len(*seqFlows))

	for i := range *seqFlows {
		sf := &(*seqFlows)[i]
		flow := BPMNFlow{
			ID: getID(sf),
		}

		if name, present := sf.Name(); present {
			flow.Name = *name
		}

		if src := sf.SourceRef(); src != nil {
			flow.SourceRef = *src
		}

		if tgt := sf.TargetRef(); tgt != nil {
			flow.TargetRef = *tgt
		}

		flows = append(flows, flow)
	}

	return flows
}

// idGetter - интерфейс для элементов с Id.
type idGetter interface {
	Id() (*schema.Id, bool)
}

// nameGetter - интерфейс для элементов с Name.
type nameGetter interface {
	Name() (*string, bool)
}

// getID извлекает ID из элемента.
func getID(elem idGetter) string {
	if id, present := elem.Id(); present && id != nil {
		return *id
	}

	return ""
}

// getName извлекает Name из элемента.
func getName(elem nameGetter) string {
	if name, present := elem.Name(); present && name != nil {
		return *name
	}

	return ""
}

// catchEventDefGetter - интерфейс для catch-событий с определениями.
type catchEventDefGetter interface {
	TimerEventDefinitions() *[]schema.TimerEventDefinition
	MessageEventDefinitions() *[]schema.MessageEventDefinition
	SignalEventDefinitions() *[]schema.SignalEventDefinition
	ErrorEventDefinitions() *[]schema.ErrorEventDefinition
}

// throwEventDefGetter - интерфейс для throw-событий с определениями.
type throwEventDefGetter interface {
	MessageEventDefinitions() *[]schema.MessageEventDefinition
	SignalEventDefinitions() *[]schema.SignalEventDefinition
	ErrorEventDefinitions() *[]schema.ErrorEventDefinition
}

// detectCatchEventType определяет подтип catch-события.
func detectCatchEventType(event catchEventDefGetter) EventType {
	if defs := event.TimerEventDefinitions(); defs != nil && len(*defs) > 0 {
		return EventTimer
	}

	if defs := event.MessageEventDefinitions(); defs != nil && len(*defs) > 0 {
		return EventMessage
	}

	if defs := event.SignalEventDefinitions(); defs != nil && len(*defs) > 0 {
		return EventSignal
	}

	if defs := event.ErrorEventDefinitions(); defs != nil && len(*defs) > 0 {
		return EventError
	}

	return EventNone
}

// detectThrowEventType определяет подтип throw-события.
func detectThrowEventType(event throwEventDefGetter) EventType {
	if defs := event.MessageEventDefinitions(); defs != nil && len(*defs) > 0 {
		return EventMessage
	}

	if defs := event.SignalEventDefinitions(); defs != nil && len(*defs) > 0 {
		return EventSignal
	}

	if defs := event.ErrorEventDefinitions(); defs != nil && len(*defs) > 0 {
		return EventError
	}

	return EventNone
}
