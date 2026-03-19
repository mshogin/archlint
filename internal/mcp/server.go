package mcp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Server implements an MCP (Model Context Protocol) server for archlint.
// It communicates via JSON-RPC 2.0 over stdio using Content-Length headers.
type Server struct {
	state    *State
	executor *ToolExecutor
	logger   *log.Logger
	reader   *bufio.Reader
	writer   io.Writer
	watcher  *Watcher
}

// jsonrpcMessage represents a JSON-RPC 2.0 message.
type jsonrpcMessage struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      *json.RawMessage `json:"id,omitempty"`
	Method  string           `json:"method,omitempty"`
	Params  json.RawMessage  `json:"params,omitempty"`
	Result  interface{}      `json:"result,omitempty"`
	Error   *jsonrpcError    `json:"error,omitempty"`
}

// jsonrpcError represents a JSON-RPC 2.0 error.
type jsonrpcError struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// JSON-RPC error codes.
const (
	codeParseError     = -32700
	codeInvalidParams  = -32602
	codeMethodNotFound = -32601
	codeInternalError  = -32603
)

// NewServer creates a new MCP server.
func NewServer(logFile string) (*Server, error) {
	var logger *log.Logger

	if logFile != "" {
		//nolint:gosec // G304: logFile is provided via CLI flag
		f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o640)
		if err != nil {
			return nil, fmt.Errorf("error opening log file: %w", err)
		}

		logger = log.New(f, "[archlint-mcp] ", log.LstdFlags)
	} else {
		logger = log.New(io.Discard, "", 0)
	}

	state := NewState()

	return &Server{
		state:    state,
		executor: NewToolExecutor(state),
		logger:   logger,
		reader:   bufio.NewReader(os.Stdin),
		writer:   os.Stdout,
	}, nil
}

// NewServerWithIO creates an MCP server with custom I/O streams. Used in tests.
func NewServerWithIO(reader io.Reader, writer io.Writer, logger *log.Logger) *Server {
	state := NewState()

	if logger == nil {
		logger = log.New(io.Discard, "", 0)
	}

	return &Server{
		state:    state,
		executor: NewToolExecutor(state),
		logger:   logger,
		reader:   bufio.NewReader(reader),
		writer:   writer,
	}
}

// Run starts the main loop of the MCP server (read stdin, handle, write stdout).
func (s *Server) Run() error {
	s.logger.Println("MCP server started")

	defer func() {
		if s.watcher != nil {
			s.watcher.Stop()
		}
	}()

	for {
		msg, err := s.readMessage()
		if err != nil {
			if err == io.EOF {
				s.logger.Println("Connection closed (EOF)")

				return nil
			}

			s.logger.Printf("Error reading message: %v", err)

			return fmt.Errorf("read error: %w", err)
		}

		s.logger.Printf("Received request: method=%s", msg.Method)

		response := s.handleMessage(msg)
		if response != nil {
			if err := s.writeMessage(response); err != nil {
				s.logger.Printf("Error writing response: %v", err)

				return fmt.Errorf("write error: %w", err)
			}
		}
	}
}

// readMessage reads one MCP message from stdin.
func (s *Server) readMessage() (*jsonrpcMessage, error) {
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
				return nil, fmt.Errorf("invalid Content-Length: %w", err)
			}

			contentLength = length
		}
	}

	if contentLength == 0 {
		return nil, fmt.Errorf("Content-Length not specified")
	}

	body := make([]byte, contentLength)

	if _, err := io.ReadFull(s.reader, body); err != nil {
		return nil, fmt.Errorf("error reading body: %w", err)
	}

	var msg jsonrpcMessage
	if err := json.Unmarshal(body, &msg); err != nil {
		return nil, fmt.Errorf("error parsing JSON: %w", err)
	}

	return &msg, nil
}

// writeMessage writes an MCP message to stdout.
func (s *Server) writeMessage(msg *jsonrpcMessage) error {
	body, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("error serializing: %w", err)
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

// handleMessage processes an incoming JSON-RPC message.
func (s *Server) handleMessage(msg *jsonrpcMessage) *jsonrpcMessage {
	switch msg.Method {
	case "initialize":
		return s.handleInitialize(msg)
	case "notifications/initialized":
		return nil // notification, no response needed
	case "tools/list":
		return s.handleToolsList(msg)
	case "tools/call":
		return s.handleToolsCall(msg)
	case "ping":
		return s.makeResponse(msg.ID, map[string]interface{}{})
	default:
		if msg.ID != nil {
			return s.makeErrorResponse(msg.ID, codeMethodNotFound,
				fmt.Sprintf("method not supported: %s", msg.Method))
		}

		return nil
	}
}

// handleInitialize handles the initialize request.
func (s *Server) handleInitialize(msg *jsonrpcMessage) *jsonrpcMessage {
	// Parse rootDir from params if provided.
	var params struct {
		RootDir string `json:"rootDir"`
	}

	if msg.Params != nil {
		// Best-effort parse — rootDir is optional in MCP.
		_ = json.Unmarshal(msg.Params, &params)
	}

	rootDir := params.RootDir
	if rootDir == "" {
		// Default to current working directory.
		var err error

		rootDir, err = os.Getwd()
		if err != nil {
			rootDir = "."
		}
	}

	// Make rootDir absolute.
	absRoot, err := filepath.Abs(rootDir)
	if err == nil {
		rootDir = absRoot
	}

	s.logger.Printf("Initializing: rootDir=%s", rootDir)

	// Parse the project.
	if err := s.state.Initialize(rootDir); err != nil {
		s.logger.Printf("Initialization error: %v", err)
		// Non-fatal: server starts but tools may return errors.
	} else {
		stats := s.state.Stats()
		s.logger.Printf("Initialization complete: %d nodes, %d edges", stats.TotalNodes, stats.TotalEdges)

		// Compute initial metrics baseline for all files.
		s.state.InitializeMetricsBaseline()
		s.logger.Println("Metrics baseline computed")

		// Start file watcher.
		s.startWatcher(rootDir)
	}

	result := map[string]interface{}{
		"protocolVersion": "2024-11-05",
		"capabilities": map[string]interface{}{
			"tools": map[string]interface{}{},
		},
		"serverInfo": map[string]interface{}{
			"name":    "archlint",
			"version": "0.2.0",
		},
	}

	return s.makeResponse(msg.ID, result)
}

// startWatcher starts the file watcher for the project directory.
func (s *Server) startWatcher(rootDir string) {
	handler := func(path string) {
		s.logger.Printf("File change detected: %s, reparsing...", path)

		report, err := s.state.ReparseFile(path)
		if err != nil {
			s.logger.Printf("Reparse error on file change: %v", err)

			return
		}

		s.logger.Printf("Reparse complete for %s (health: %d -> %d, status: %s)",
			path, report.HealthBefore, report.HealthAfter, report.Status)

		// If degradation detected, send a notification (best-effort).
		if report.Status == "degraded" || report.Status == "critical" {
			s.sendDegradationNotification(report)
		}
	}

	watcher, err := NewWatcher(rootDir, handler, s.logger)
	if err != nil {
		s.logger.Printf("Warning: could not start file watcher: %v", err)

		return
	}

	s.watcher = watcher
	watcher.Start()
	s.logger.Println("File watcher started")
}

// sendDegradationNotification sends a server-initiated notification about file degradation.
func (s *Server) sendDegradationNotification(report *DegradationReport) {
	params, err := json.Marshal(report)
	if err != nil {
		return
	}

	notification := &jsonrpcMessage{
		JSONRPC: "2.0",
		Method:  "notifications/fileChanged",
		Params:  params,
	}

	if err := s.writeMessage(notification); err != nil {
		s.logger.Printf("Error sending degradation notification: %v", err)
	}
}

// handleToolsList handles the tools/list request.
func (s *Server) handleToolsList(msg *jsonrpcMessage) *jsonrpcMessage {
	result := map[string]interface{}{
		"tools": toolDefinitions(),
	}

	return s.makeResponse(msg.ID, result)
}

// handleToolsCall handles the tools/call request.
func (s *Server) handleToolsCall(msg *jsonrpcMessage) *jsonrpcMessage {
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}

	if err := json.Unmarshal(msg.Params, &params); err != nil {
		return s.makeErrorResponse(msg.ID, codeParseError, "error parsing tool call parameters")
	}

	s.logger.Printf("Executing tool: %s", params.Name)

	// No per-call Reparse() — the file watcher keeps state fresh.
	// If no watcher is running (e.g. tests), do a reparse for compatibility.
	if s.watcher == nil {
		if err := s.state.Reparse(); err != nil {
			s.logger.Printf("Reparse error: %v", err)
		}
	}

	result, err := s.executor.Execute(params.Name, params.Arguments)
	if err != nil {
		return s.makeErrorResponse(msg.ID, codeInternalError, err.Error())
	}

	// Marshal the result to JSON text for the MCP content response.
	resultJSON, err := json.Marshal(result)
	if err != nil {
		return s.makeErrorResponse(msg.ID, codeInternalError, "error serializing result")
	}

	content := []map[string]interface{}{
		{
			"type": "text",
			"text": string(resultJSON),
		},
	}

	return s.makeResponse(msg.ID, map[string]interface{}{
		"content": content,
	})
}

// makeResponse creates a JSON-RPC response.
func (s *Server) makeResponse(id *json.RawMessage, result interface{}) *jsonrpcMessage {
	return &jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      id,
		Result:  result,
	}
}

// makeErrorResponse creates a JSON-RPC error response.
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
