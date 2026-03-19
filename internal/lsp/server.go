package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/mshogin/archlint/internal/model"
)

// Server реализует LSP-сервер для archlint.
type Server struct {
	state  *State
	bridge *AnalyzerBridge
	logger *log.Logger
	reader *bufio.Reader
	writer io.Writer
}

// jsonrpcMessage представляет JSON-RPC 2.0 сообщение.
type jsonrpcMessage struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Result  interface{}      `json:"result,omitempty"`
	Error   *jsonrpcError    `json:"error,omitempty"`
}

// jsonrpcError представляет ошибку JSON-RPC 2.0.
type jsonrpcError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// JSON-RPC error codes.
const (
	codeParseError     = -32700
	codeMethodNotFound = -32601
	codeInternalError  = -32603
)

// Список поддерживаемых команд.
var supportedCommands = []string{
	"archlint.analyzeFile",
	"archlint.analyzeChange",
	"archlint.getGraph",
	"archlint.getMetrics",
}

// NewServer создаёт новый LSP-сервер.
func NewServer(logFile string) (*Server, error) {
	var logger *log.Logger

	if logFile != "" {
		//nolint:gosec // G304: logFile is provided via CLI flag
		f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
		if err != nil {
			return nil, fmt.Errorf("ошибка открытия лог-файла: %w", err)
		}

		logger = log.New(f, "[archlint-lsp] ", log.LstdFlags)
	} else {
		logger = log.New(io.Discard, "", 0)
	}

	state := NewState()

	return &Server{
		state:  state,
		bridge: NewAnalyzerBridge(state),
		logger: logger,
		reader: bufio.NewReader(os.Stdin),
		writer: os.Stdout,
	}, nil
}

// NewServerWithIO создаёт LSP-сервер с указанными потоками ввода-вывода.
// Используется в тестах.
func NewServerWithIO(reader io.Reader, writer io.Writer, logger *log.Logger) *Server {
	state := NewState()

	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}

	return &Server{
		state:  state,
		bridge: NewAnalyzerBridge(state),
		logger: logger,
		reader: bufio.NewReader(reader),
		writer: writer,
	}
}

// Run запускает главный цикл LSP-сервера (чтение stdin, обработка, запись stdout).
func (s *Server) Run() error {
	s.logger.Println("LSP-сервер запущен")

	for {
		msg, err := s.readMessage()
		if err != nil {
			if err == io.EOF {
				s.logger.Println("Соединение закрыто (EOF)")

				return nil
			}

			s.logger.Printf("Ошибка чтения сообщения: %v", err)

			return fmt.Errorf("ошибка чтения: %w", err)
		}

		s.logger.Printf("Получен запрос: method=%s", msg.Method)

		response := s.handleMessage(msg)
		if response != nil {
			if err := s.writeMessage(response); err != nil {
				s.logger.Printf("Ошибка записи ответа: %v", err)

				return fmt.Errorf("ошибка записи: %w", err)
			}
		}
	}
}

// readMessage читает одно LSP-сообщение из stdin.
func (s *Server) readMessage() (*jsonrpcMessage, error) {
	// Читаем заголовки.
	contentLength := 0

	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			return nil, err
		}

		line = strings.TrimSpace(line)

		if line == "" {
			break
		}

		if strings.HasPrefix(line, "Content-Length:") {
			val := strings.TrimSpace(strings.TrimPrefix(line, "Content-Length:"))

			length, err := strconv.Atoi(val)
			if err != nil {
				return nil, fmt.Errorf("некорректный Content-Length: %w", err)
			}

			contentLength = length
		}
	}

	if contentLength == 0 {
		return nil, fmt.Errorf("Content-Length не указан")
	}

	// Читаем тело.
	body := make([]byte, contentLength)

	if _, err := io.ReadFull(s.reader, body); err != nil {
		return nil, fmt.Errorf("ошибка чтения тела: %w", err)
	}

	var msg jsonrpcMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, fmt.Errorf("ошибка разбора JSON: %w", err)
	}

	return &msg, nil
}

// writeMessage записывает LSP-сообщение в stdout.
func (s *Server) writeMessage(msg *jsonrpcMessage) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("ошибка сериализации: %w", err)
	}

	header := fmt.Sprintf("Content-Length: %d\r\n\r\n", len(body))

	if _, err := fmt.Fprint(s.writer, header); err != nil {
		return err
	}

	if _, err := s.writer.Write(body); err != nil {
		return err
	}

	return nil
}

// sendNotification отправляет уведомление клиенту.
func (s *Server) sendNotification(method string, params interface{}) {
	msg := &jsonrpcMessage{
		JSONRPC: "2.0",
		Method:  method,
	}

	if params != nil {
		data, err := json.Marshal(params)
		if err != nil {
			s.logger.Printf("Ошибка сериализации уведомления: %v", err)

			return
		}

		msg.Params = data
	}

	if err := s.writeMessage(msg); err != nil {
		s.logger.Printf("Ошибка отправки уведомления: %v", err)
	}
}

// handleMessage обрабатывает входящее JSON-RPC сообщение.
func (s *Server) handleMessage(msg *jsonrpcMessage) *jsonrpcMessage {
	switch msg.Method {
	case "initialize":
		return s.handleInitialize(msg)
	case "initialized":
		return nil // уведомление, ответ не требуется
	case "shutdown":
		return s.handleShutdown(msg)
	case "exit":
		os.Exit(0)

		return nil
	case "textDocument/didOpen":
		s.handleDidOpen(msg)

		return nil
	case "textDocument/didChange":
		s.handleDidChange(msg)

		return nil
	case "textDocument/didSave":
		s.handleDidSave(msg)

		return nil
	case "textDocument/didClose":
		s.handleDidClose(msg)

		return nil
	case "workspace/didChangeWatchedFiles":
		s.handleDidChangeWatchedFiles(msg)

		return nil
	case "workspace/executeCommand":
		return s.handleExecuteCommand(msg)
	default:
		if msg.ID != nil {
			return s.makeErrorResponse(msg.ID, codeMethodNotFound,
				fmt.Sprintf("метод не поддерживается: %s", msg.Method))
		}

		return nil
	}
}

// handleInitialize обрабатывает запрос initialize.
func (s *Server) handleInitialize(msg *jsonrpcMessage) *jsonrpcMessage {
	var params InitializeParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return s.makeErrorResponse(msg.ID, codeParseError, "ошибка разбора параметров")
	}

	rootDir := uriToPath(params.RootURI)
	if rootDir == "" {
		rootDir = params.RootPath
	}

	s.logger.Printf("Инициализация: rootDir=%s", rootDir)

	// Парсим проект в фоне (но блокируемся до завершения для простоты).
	if rootDir != "" {
		if err := s.state.Initialize(rootDir); err != nil {
			s.logger.Printf("Ошибка инициализации: %v", err)

			s.sendNotification("window/logMessage", LogMessageParams{
				Type:    LogWarning,
				Message: fmt.Sprintf("Частичная инициализация: %v", err),
			})
		} else {
			stats := s.state.Stats()
			s.logger.Printf("Инициализация завершена: %d nodes, %d edges", stats.TotalNodes, stats.TotalEdges)
		}
	}

	result := InitializeResult{
		Capabilities: ServerCapabilities{
			TextDocumentSync: 1, // Full sync
			ExecuteCommandProvider: &ExecuteCommandOptions{
				Commands: supportedCommands,
			},
			HoverProvider: false,
		},
		ServerInfo: ServerInfo{
			Name:    "archlint-lsp",
			Version: "0.1.0",
		},
	}

	return s.makeResponse(msg.ID, result)
}

// handleShutdown обрабатывает запрос shutdown.
func (s *Server) handleShutdown(msg *jsonrpcMessage) *jsonrpcMessage {
	s.logger.Println("Получен запрос shutdown")

	return s.makeResponse(msg.ID, nil)
}

// handleDidOpen обрабатывает уведомление textDocument/didOpen.
func (s *Server) handleDidOpen(msg *jsonrpcMessage) {
	var params DidOpenTextDocumentParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		s.logger.Printf("Ошибка разбора didOpen: %v", err)

		return
	}

	filePath := uriToPath(params.TextDocument.URI)
	s.state.SetFileVersion(params.TextDocument.URI, params.TextDocument.Version)
	s.logger.Printf("Открыт файл: %s", filePath)

	if strings.HasSuffix(filePath, ".go") {
		s.reparseAndPublishDiagnostics(filePath, params.TextDocument.URI)
	}
}

// handleDidChange обрабатывает уведомление textDocument/didChange.
func (s *Server) handleDidChange(msg *jsonrpcMessage) {
	var params DidChangeTextDocumentParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		s.logger.Printf("Ошибка разбора didChange: %v", err)

		return
	}

	s.state.SetFileVersion(params.TextDocument.URI, params.TextDocument.Version)

	// Не перепарсиваем при каждом изменении — ждём save.
	s.logger.Printf("Изменён файл: %s (version %d)", params.TextDocument.URI, params.TextDocument.Version)
}

// handleDidSave обрабатывает уведомление textDocument/didSave.
func (s *Server) handleDidSave(msg *jsonrpcMessage) {
	var params DidSaveTextDocumentParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		s.logger.Printf("Ошибка разбора didSave: %v", err)

		return
	}

	filePath := uriToPath(params.TextDocument.URI)
	s.logger.Printf("Сохранён файл: %s", filePath)

	if strings.HasSuffix(filePath, ".go") {
		s.reparseAndPublishDiagnostics(filePath, params.TextDocument.URI)
	}
}

// handleDidClose обрабатывает уведомление textDocument/didClose.
func (s *Server) handleDidClose(msg *jsonrpcMessage) {
	var params DidCloseTextDocumentParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		s.logger.Printf("Ошибка разбора didClose: %v", err)

		return
	}

	// Очищаем диагностики при закрытии файла.
	s.sendNotification("textDocument/publishDiagnostics", PublishDiagnosticsParams{
		URI:         params.TextDocument.URI,
		Diagnostics: []Diagnostic{},
	})

	s.logger.Printf("Закрыт файл: %s", params.TextDocument.URI)
}

// handleDidChangeWatchedFiles обрабатывает уведомление workspace/didChangeWatchedFiles.
func (s *Server) handleDidChangeWatchedFiles(msg *jsonrpcMessage) {
	var params DidChangeWatchedFilesParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		s.logger.Printf("Ошибка разбора didChangeWatchedFiles: %v", err)

		return
	}

	var goFiles []string

	for _, change := range params.Changes {
		filePath := uriToPath(change.URI)
		if strings.HasSuffix(filePath, ".go") {
			goFiles = append(goFiles, filePath)
		}
	}

	if len(goFiles) > 0 {
		s.logger.Printf("Изменены файлы workspace: %v", goFiles)

		if err := s.state.ReparseFiles(goFiles); err != nil {
			s.logger.Printf("Ошибка перепарсинга файлов: %v", err)
		}
	}
}

// handleExecuteCommand обрабатывает запрос workspace/executeCommand.
//
//nolint:funlen // Command dispatch requires handling multiple commands.
func (s *Server) handleExecuteCommand(msg *jsonrpcMessage) *jsonrpcMessage {
	var params ExecuteCommandParams
	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return s.makeErrorResponse(msg.ID, codeParseError, "ошибка разбора параметров команды")
	}

	s.logger.Printf("Выполнение команды: %s args=%v", params.Command, params.Arguments)

	switch params.Command {
	case "archlint.analyzeFile":
		return s.executeAnalyzeFile(msg.ID, params.Arguments)
	case "archlint.analyzeChange":
		return s.executeAnalyzeChange(msg.ID, params.Arguments)
	case "archlint.getGraph":
		return s.executeGetGraph(msg.ID, params.Arguments)
	case "archlint.getMetrics":
		return s.executeGetMetrics(msg.ID, params.Arguments)
	default:
		return s.makeErrorResponse(msg.ID, codeMethodNotFound,
			fmt.Sprintf("неизвестная команда: %s", params.Command))
	}
}

// executeAnalyzeFile выполняет команду archlint.analyzeFile.
func (s *Server) executeAnalyzeFile(id *json.RawMessage, args []interface{}) *jsonrpcMessage {
	if len(args) < 1 {
		return s.makeErrorResponse(id, codeParseError, "команда требует аргумент: путь к файлу")
	}

	filePath, ok := args[0].(string)
	if !ok {
		return s.makeErrorResponse(id, codeParseError, "аргумент должен быть строкой")
	}

	// Поддержка URI и обычных путей.
	if strings.HasPrefix(filePath, "file://") {
		filePath = uriToPath(filePath)
	}

	result, err := s.bridge.AnalyzeFile(filePath)
	if err != nil {
		return s.makeErrorResponse(id, codeInternalError, err.Error())
	}

	return s.makeResponse(id, result)
}

// executeAnalyzeChange выполняет команду archlint.analyzeChange.
func (s *Server) executeAnalyzeChange(id *json.RawMessage, args []interface{}) *jsonrpcMessage {
	if len(args) < 1 {
		return s.makeErrorResponse(id, codeParseError, "команда требует аргумент: путь к файлу")
	}

	filePath, ok := args[0].(string)
	if !ok {
		return s.makeErrorResponse(id, codeParseError, "аргумент должен быть строкой")
	}

	if strings.HasPrefix(filePath, "file://") {
		filePath = uriToPath(filePath)
	}

	result, err := s.bridge.AnalyzeChange(filePath)
	if err != nil {
		return s.makeErrorResponse(id, codeInternalError, err.Error())
	}

	return s.makeResponse(id, result)
}

// executeGetGraph выполняет команду archlint.getGraph.
func (s *Server) executeGetGraph(id *json.RawMessage, args []interface{}) *jsonrpcMessage {
	graph := s.state.GetGraph()

	// Если указан фильтр, фильтруем по пакету.
	if len(args) > 0 {
		filter, ok := args[0].(string)
		if ok && filter != "" {
			graph = filterGraph(graph, filter)
		}
	}

	return s.makeResponse(id, graph)
}

// executeGetMetrics выполняет команду archlint.getMetrics.
func (s *Server) executeGetMetrics(id *json.RawMessage, args []interface{}) *jsonrpcMessage {
	if len(args) < 1 {
		// Возвращаем общую статистику.
		stats := s.state.Stats()

		return s.makeResponse(id, stats)
	}

	target, ok := args[0].(string)
	if !ok {
		return s.makeErrorResponse(id, codeParseError, "аргумент должен быть строкой")
	}

	result, err := s.bridge.GetMetrics(target)
	if err != nil {
		return s.makeErrorResponse(id, codeInternalError, err.Error())
	}

	return s.makeResponse(id, result)
}

// reparseAndPublishDiagnostics перепарсивает файл и публикует диагностики.
func (s *Server) reparseAndPublishDiagnostics(filePath, uri string) {
	if err := s.state.ReparseFile(filePath); err != nil {
		s.logger.Printf("Ошибка перепарсинга %s: %v", filePath, err)

		return
	}

	analysis, err := s.bridge.AnalyzeFile(filePath)
	if err != nil {
		s.logger.Printf("Ошибка анализа %s: %v", filePath, err)

		return
	}

	s.sendNotification("textDocument/publishDiagnostics", PublishDiagnosticsParams{
		URI:         uri,
		Diagnostics: analysis.Diagnostics,
	})
}

// makeResponse создаёт JSON-RPC ответ.
func (s *Server) makeResponse(id *json.RawMessage, result interface{}) *jsonrpcMessage {
	return &jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}

// makeErrorResponse создаёт JSON-RPC ответ с ошибкой.
func (s *Server) makeErrorResponse(id *json.RawMessage, code int, message string) *jsonrpcMessage {
	return &jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      id,
		Error: &jsonrpcError{
			Code:    code,
			Message: message,
		},
	}
}

// uriToPath конвертирует file:// URI в путь файловой системы.
func uriToPath(uri string) string {
	if !strings.HasPrefix(uri, "file://") {
		return uri
	}

	parsed, err := url.Parse(uri)
	if err != nil {
		// Fallback: просто отрезаем file://
		return strings.TrimPrefix(uri, "file://")
	}

	return filepath.FromSlash(parsed.Path)
}

// filterGraph фильтрует граф, оставляя только узлы и рёбра, связанные с filter.
func filterGraph(graph *model.Graph, filter string) *model.Graph {
	nodeSet := make(map[string]bool)

	for _, node := range graph.Nodes {
		if matchesTarget(node.ID, filter) {
			nodeSet[node.ID] = true
		}
	}

	var filteredNodes []model.Node

	for _, node := range graph.Nodes {
		if nodeSet[node.ID] {
			filteredNodes = append(filteredNodes, node)
		}
	}

	var filteredEdges []model.Edge

	for _, edge := range graph.Edges {
		if nodeSet[edge.From] || nodeSet[edge.To] {
			filteredEdges = append(filteredEdges, edge)
		}
	}

	return &model.Graph{
		Nodes: filteredNodes,
		Edges: filteredEdges,
	}
}
