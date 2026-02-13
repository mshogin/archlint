package bpmn

import (
	"fmt"
	"os"

	"github.com/mshogin/archlint/pkg/tracer"
	"github.com/olive-io/bpmn/schema"
)

// ParseFile читает BPMN 2.0 XML файл и возвращает BPMNProcess.
func ParseFile(filename string) (*BPMNProcess, error) {
	tracer.Enter("ParseFile")
	//nolint:gosec // G304: filename is a user-provided CLI argument, file path control is expected
	data, err := os.ReadFile(filename)
	if err != nil {
		tracer.ExitError("ParseFile", err)

		return nil, fmt.Errorf("чтение BPMN файла: %w", err)
	}

	tracer.ExitSuccess("ParseFile")

	return Parse(data)
}

// Parse парсит BPMN 2.0 XML через schema.Parse() и конвертирует в BPMNProcess.
func Parse(data []byte) (*BPMNProcess, error) {
	tracer.Enter("Parse")

	defs, err := schema.Parse(data)
	if err != nil {
		tracer.ExitError("Parse", err)

		return nil, fmt.Errorf("парсинг BPMN XML: %w", err)
	}

	tracer.ExitSuccess("Parse")

	return convertDefinitions(defs)
}

// convertDefinitions конвертирует schema.Definitions -> BPMNProcess.
// Берет первый процесс из definitions (типичный кейс).
func convertDefinitions(defs *schema.Definitions) (*BPMNProcess, error) {
	tracer.Enter("convertDefinitions")

	processes := defs.Processes()
	if processes == nil || len(*processes) == 0 {
		tracer.ExitError("convertDefinitions", ErrNoProcesses)

		return nil, ErrNoProcesses
	}

	tracer.ExitSuccess("convertDefinitions")

	return convertProcess(&(*processes)[0])
}

// convertProcess конвертирует schema.Process -> BPMNProcess.
func convertProcess(proc *schema.Process) (*BPMNProcess, error) {
	tracer.Enter("convertProcess")

	result := &BPMNProcess{
		ID:   getID(proc),
		Name: getName(proc),
	}

	result.Elements = collectElements(proc)
	result.Flows = collectFlows(proc)

	tracer.ExitSuccess("convertProcess")

	return result, nil
}

// collectElements собирает все элементы процесса.
//
//nolint:funlen,gocyclo // Collecting all BPMN element types requires iterating each type.
func collectElements(proc *schema.Process) []BPMNElement {
	tracer.Enter("collectElements")

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

	tracer.ExitSuccess("collectElements")

	return elements
}

// collectFlows собирает все sequence flows процесса.
func collectFlows(proc *schema.Process) []BPMNFlow {
	tracer.Enter("collectFlows")

	seqFlows := proc.SequenceFlows()
	if seqFlows == nil {
		tracer.ExitSuccess("collectFlows")

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

	tracer.ExitSuccess("collectFlows")

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
	tracer.Enter("getID")

	if id, present := elem.Id(); present && id != nil {
		tracer.ExitSuccess("getID")

		return *id
	}

	tracer.ExitSuccess("getID")

	return ""
}

// getName извлекает Name из элемента.
func getName(elem nameGetter) string {
	tracer.Enter("getName")

	if name, present := elem.Name(); present && name != nil {
		tracer.ExitSuccess("getName")

		return *name
	}

	tracer.ExitSuccess("getName")

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
	tracer.Enter("detectCatchEventType")

	if defs := event.TimerEventDefinitions(); defs != nil && len(*defs) > 0 {
		tracer.ExitSuccess("detectCatchEventType")

		return EventTimer
	}

	if defs := event.MessageEventDefinitions(); defs != nil && len(*defs) > 0 {
		tracer.ExitSuccess("detectCatchEventType")

		return EventMessage
	}

	if defs := event.SignalEventDefinitions(); defs != nil && len(*defs) > 0 {
		tracer.ExitSuccess("detectCatchEventType")

		return EventSignal
	}

	if defs := event.ErrorEventDefinitions(); defs != nil && len(*defs) > 0 {
		tracer.ExitSuccess("detectCatchEventType")

		return EventError
	}

	tracer.ExitSuccess("detectCatchEventType")

	return EventNone
}

// detectThrowEventType определяет подтип throw-события.
func detectThrowEventType(event throwEventDefGetter) EventType {
	tracer.Enter("detectThrowEventType")

	if defs := event.MessageEventDefinitions(); defs != nil && len(*defs) > 0 {
		tracer.ExitSuccess("detectThrowEventType")

		return EventMessage
	}

	if defs := event.SignalEventDefinitions(); defs != nil && len(*defs) > 0 {
		tracer.ExitSuccess("detectThrowEventType")

		return EventSignal
	}

	if defs := event.ErrorEventDefinitions(); defs != nil && len(*defs) > 0 {
		tracer.ExitSuccess("detectThrowEventType")

		return EventError
	}

	tracer.ExitSuccess("detectThrowEventType")

	return EventNone
}
