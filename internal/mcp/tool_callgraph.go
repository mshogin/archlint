package mcp

import (
	"encoding/json"
	"fmt"

	"github.com/mshogin/archlint/internal/model"
)

// Направления обхода графа вызовов.
const (
	callgraphCallees = "callees" // кого зовёт entry (обход по рёбрам calls как есть)
	callgraphCallers = "callers" // кто зовёт entry (обход по инвертированным рёбрам)
)

// handleGetCallgraph implements the get_callgraph tool.
//
// Направление важно для разбора последствий изменения: вопрос «кого зовёт эта
// функция» отвечает на «что она делает», а вопрос «кто зовёт эту функцию» - на
// «кого сломает правка внутри неё». Второй без инверсии рёбер недоступен, хотя
// рёбра calls в графе уже есть.
func handleGetCallgraph(state StateReader, args json.RawMessage) (*CallGraphResult, error) {
	var params struct {
		Entry     string `json:"entry"`
		MaxDepth  int    `json:"max_depth"`
		Direction string `json:"direction"`
	}

	if err := json.Unmarshal(args, &params); err != nil {
		return nil, fmt.Errorf("invalid arguments: %w", err)
	}

	if params.Entry == "" {
		return nil, fmt.Errorf("entry is required")
	}

	if params.MaxDepth <= 0 {
		params.MaxDepth = 10
	}

	// Умолчание - прежнее поведение: у существующих потребителей direction нет.
	switch params.Direction {
	case "":
		params.Direction = callgraphCallees
	case callgraphCallees, callgraphCallers:
	default:
		return nil, fmt.Errorf("direction must be %q or %q, got %q",
			callgraphCallees, callgraphCallers, params.Direction)
	}

	graph := state.GetGraph()

	// Владелец узла и связи типов нужны, чтобы отделить настоящий диспетч
	// через интерфейс от совпадения имён.
	owner := make(map[string]string)              // метод -> его тип
	usesTypes := make(map[string]map[string]bool) // узел -> типы, которые он использует
	ifacesOf := make(map[string]map[string]bool)  // тип -> интерфейсы, которые он реализует

	for _, edge := range graph.Edges {
		switch edge.Type {
		case model.EdgeContains:
			owner[edge.To] = edge.From
		case model.EdgeUses:
			if usesTypes[edge.From] == nil {
				usesTypes[edge.From] = map[string]bool{}
			}

			usesTypes[edge.From][edge.To] = true
		case model.EdgeImplements:
			if ifacesOf[edge.From] == nil {
				ifacesOf[edge.From] = map[string]bool{}
			}

			ifacesOf[edge.From][edge.To] = true
		}
	}

	// Смежность строится по направлению обхода: для callers ребро A calls B
	// читается как «B вызывается из A».
	adj := make(map[string][]string)
	refAdj := make(map[string][]string)
	nodeNames := make(map[string]string)

	for _, edge := range graph.Edges {
		switch edge.Type {
		case "calls":
			if params.Direction == callgraphCallers {
				adj[edge.To] = append(adj[edge.To], edge.From)
			} else {
				adj[edge.From] = append(adj[edge.From], edge.To)
			}
		case model.EdgeReferences:
			// references строятся ПО ИМЕНИ (over-approximation, см.
			// buildReferenceEdges): символ с именем N даёт ребро на все
			// функции и методы с таким именем, без вывода типов.
			//
			// Складывать их с calls нельзя - список «вызывающих» перестанет
			// быть проверяемым фактом. Но и терять нельзя: вызов через
			// интерфейс и регистрация метода обработчиком маршрута попадают
			// именно сюда, а по calls такой метод выглядит никем не
			// вызываемым. Поэтому отдаём отдельным полем и не обходим их в
			// BFS: обход по именам мгновенно вырождается в весь граф.
			if params.Direction == callgraphCallers {
				refAdj[edge.To] = append(refAdj[edge.To], edge.From)
			}
		}
	}

	for _, node := range graph.Nodes {
		nodeNames[node.ID] = node.Title
	}

	result := &CallGraphResult{
		Entry:     params.Entry,
		MaxDepth:  params.MaxDepth,
		Direction: params.Direction,
	}

	// BFS traversal from entry point.
	type queueItem struct {
		id    string
		depth int
	}

	visited := make(map[string]bool)
	queue := []queueItem{{id: params.Entry, depth: 0}}
	visited[params.Entry] = true

	for len(queue) > 0 {
		item := queue[0]
		queue = queue[1:]

		name := nodeNames[item.id]
		if name == "" {
			name = item.id
		}

		var neighbours []string

		if item.depth < params.MaxDepth {
			for _, target := range adj[item.id] {
				neighbours = append(neighbours, target)

				if !visited[target] {
					visited[target] = true
					queue = append(queue, queueItem{id: target, depth: item.depth + 1})
				}
			}
		}

		node := CallGraphNode{
			ID:    item.id,
			Name:  name,
			Depth: item.depth,
		}

		// Поля разные намеренно: одно и то же поле с разным смыслом читается
		// неверно и в отчёте, и моделью.
		if params.Direction == callgraphCallers {
			node.CalledBy = neighbours
			// Приблизительные связи только для самой точки входа: вглубь они
			// не обходятся и в отчёте нужны там, где задан вопрос.
			if item.depth == 0 {
				node.DispatchedBy, node.ReferencedBy =
					splitRefs(item.id, refAdj[item.id], owner, usesTypes, ifacesOf)
			}
		} else {
			node.CallsTo = neighbours
		}

		result.Nodes = append(result.Nodes, node)
	}

	return result, nil
}

// splitRefs делит приблизительные связи на правдоподобный диспетч и шум.
//
// Резолв по имени даёт ребро на КАЖДЫЙ метод с таким именем, поэтому у
// распространённых имён (Get, Create) список наполовину состоит из чужих
// типов. Отличить настоящий вызов через интерфейс от однофамильца можно по
// уже имеющимся рёбрам: связь правдоподобна, если вызывающий (или тип, в
// котором он объявлен) ИСПОЛЬЗУЕТ либо сам тип-владелец цели, либо интерфейс,
// который этот тип реализует. Именно так выглядит внедрение зависимости:
// Logic хранит поле типа HRClient, httpHR реализует HRClient.
//
// Заметим, что при нескольких реализациях одного интерфейса правдоподобными
// окажутся все - и это ВЕРНО: какая именно реализация подставлена, без
// анализа потока значений неизвестно, а ревьюеру важно увидеть их все.
//
// Оставшееся не выбрасывается, а возвращается вторым списком: молча выкинутая
// связь вернула бы слепую зону, ради устранения которой всё и делалось.
func splitRefs(target string, refs []string,
	owner map[string]string,
	usesTypes map[string]map[string]bool,
	ifacesOf map[string]map[string]bool,
) (dispatched, weak []string) {
	targetType := owner[target]

	// Множество типов, использование которых делает связь правдоподобной.
	plausible := map[string]bool{}
	if targetType != "" {
		plausible[targetType] = true

		for iface := range ifacesOf[targetType] {
			plausible[iface] = true
		}
	}

	for _, ref := range refs {
		if usesAny(usesTypes[ref], plausible) || usesAny(usesTypes[owner[ref]], plausible) {
			dispatched = append(dispatched, ref)

			continue
		}

		weak = append(weak, ref)
	}

	return dispatched, weak
}

func usesAny(used, wanted map[string]bool) bool {
	for t := range used {
		if wanted[t] {
			return true
		}
	}

	return false
}
