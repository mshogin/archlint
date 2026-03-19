package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// buildMCPMessage formats an MCP message with Content-Length header.
func buildMCPMessage(msg interface{}) []byte {
	body, _ := json.Marshal(msg)

	return []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body))
}

// parseMCPResponse extracts the JSON-RPC response from MCP output.
func parseMCPResponse(data []byte) (*jsonrpcMessage, error) {
	str := string(data)

	var lastResp *jsonrpcMessage

	for {
		idx := strings.Index(str, "Content-Length:")
		if idx < 0 {
			break
		}

		str = str[idx:]
		headerEnd := strings.Index(str, "\r\n\r\n")

		if headerEnd < 0 {
			break
		}

		lengthStr := strings.TrimSpace(strings.TrimPrefix(str[:headerEnd], "Content-Length:"))
		var length int

		if _, err := fmt.Sscanf(lengthStr, "%d", &length); err != nil {
			break
		}

		bodyStart := headerEnd + 4
		if bodyStart+length > len(str) {
			break
		}

		body := str[bodyStart : bodyStart+length]

		var resp jsonrpcMessage
		if err := json.Unmarshal([]byte(body), &resp); err == nil {
			if resp.ID != nil {
				lastResp = &resp
			}
		}

		str = str[bodyStart+length:]
	}

	if lastResp == nil {
		return nil, fmt.Errorf("no response found in data: %s", string(data))
	}

	return lastResp, nil
}

func TestInitialize(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "main.go")

	if err := os.WriteFile(goFile, []byte(`package main

func main() {}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	id := json.RawMessage(`1`)
	initMsg := buildMCPMessage(jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "initialize",
		Params:  mustMarshal(map[string]string{"rootDir": tmpDir}),
	})

	input := bytes.NewReader(initMsg)
	output := &bytes.Buffer{}
	logger := log.New(io.Discard, "", 0)

	server := NewServerWithIO(input, output, logger)

	msg, err := server.readMessage()
	if err != nil {
		t.Fatalf("error reading message: %v", err)
	}

	resp := server.handleMessage(msg)
	if resp == nil {
		t.Fatal("expected response to initialize")
	}

	if err := server.writeMessage(resp); err != nil {
		t.Fatalf("error writing response: %v", err)
	}

	response, err := parseMCPResponse(output.Bytes())
	if err != nil {
		t.Fatalf("error parsing response: %v", err)
	}

	if response.Error != nil {
		t.Fatalf("got error: %s", response.Error.Message)
	}

	resultBytes, _ := json.Marshal(response.Result)

	var result map[string]interface{}
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("error parsing result: %v", err)
	}

	if result["protocolVersion"] != "2024-11-05" {
		t.Errorf("expected protocolVersion 2024-11-05, got %v", result["protocolVersion"])
	}

	serverInfo, ok := result["serverInfo"].(map[string]interface{})
	if !ok {
		t.Fatal("expected serverInfo in result")
	}

	if serverInfo["name"] != "archlint" {
		t.Errorf("expected server name archlint, got %v", serverInfo["name"])
	}
}

func TestToolsList(t *testing.T) {
	server := NewServerWithIO(
		bytes.NewReader(nil),
		&bytes.Buffer{},
		log.New(io.Discard, "", 0),
	)

	id := json.RawMessage(`2`)
	resp := server.handleMessage(&jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "tools/list",
	})

	if resp == nil {
		t.Fatal("expected response")
	}

	if resp.Error != nil {
		t.Fatalf("got error: %s", resp.Error.Message)
	}

	resultBytes, _ := json.Marshal(resp.Result)

	var result struct {
		Tools []ToolDefinition `json:"tools"`
	}

	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("error parsing result: %v", err)
	}

	expectedTools := []string{
		"analyze_file", "analyze_change", "get_dependencies",
		"get_architecture", "check_violations", "get_callgraph",
		"get_file_metrics", "get_degradation_report",
	}

	if len(result.Tools) != len(expectedTools) {
		t.Fatalf("expected %d tools, got %d", len(expectedTools), len(result.Tools))
	}

	toolNames := make(map[string]bool)
	for _, tool := range result.Tools {
		toolNames[tool.Name] = true
	}

	for _, name := range expectedTools {
		if !toolNames[name] {
			t.Errorf("expected tool %s not found", name)
		}
	}
}

func TestToolsCallAnalyzeFile(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "main.go")

	if err := os.WriteFile(goFile, []byte(`package main

type Service struct {
	Name string
}

func NewService() *Service {
	return &Service{Name: "test"}
}

func (s *Service) Run() error {
	return nil
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	server := createInitializedServer(t, tmpDir)

	id := json.RawMessage(`3`)
	resp := server.handleMessage(&jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "tools/call",
		Params: mustMarshal(map[string]interface{}{
			"name":      "analyze_file",
			"arguments": map[string]string{"path": goFile},
		}),
	})

	if resp == nil {
		t.Fatal("expected response")
	}

	if resp.Error != nil {
		t.Fatalf("got error: %s", resp.Error.Message)
	}

	// Parse MCP content response.
	resultBytes, _ := json.Marshal(resp.Result)

	var mcpResult struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.Unmarshal(resultBytes, &mcpResult); err != nil {
		t.Fatalf("error parsing MCP result: %v", err)
	}

	if len(mcpResult.Content) == 0 {
		t.Fatal("expected content in response")
	}

	var analysis FileAnalysis
	if err := json.Unmarshal([]byte(mcpResult.Content[0].Text), &analysis); err != nil {
		t.Fatalf("error parsing analysis: %v", err)
	}

	if len(analysis.Types) == 0 {
		t.Error("expected types in analysis result")
	}

	if len(analysis.Functions) == 0 {
		t.Error("expected functions in analysis result")
	}

	if len(analysis.Methods) == 0 {
		t.Error("expected methods in analysis result")
	}
}

func TestToolsCallAnalyzeChange(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "main.go")

	if err := os.WriteFile(goFile, []byte(`package main

type Config struct {
	Debug bool
}

func main() {}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	server := createInitializedServer(t, tmpDir)

	id := json.RawMessage(`4`)
	resp := server.handleMessage(&jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "tools/call",
		Params: mustMarshal(map[string]interface{}{
			"name":      "analyze_change",
			"arguments": map[string]string{"path": goFile},
		}),
	})

	if resp == nil {
		t.Fatal("expected response")
	}

	if resp.Error != nil {
		t.Fatalf("got error: %s", resp.Error.Message)
	}

	text := extractContentText(t, resp)

	var result ChangeAnalysis
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("error parsing result: %v", err)
	}

	if len(result.AffectedNodes) == 0 {
		t.Error("expected affected nodes")
	}
}

func TestToolsCallGetArchitecture(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "main.go")

	if err := os.WriteFile(goFile, []byte(`package main

type Foo struct{}
type Bar struct{ f Foo }

func main() {}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	server := createInitializedServer(t, tmpDir)

	id := json.RawMessage(`5`)
	resp := server.handleMessage(&jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "tools/call",
		Params: mustMarshal(map[string]interface{}{
			"name":      "get_architecture",
			"arguments": map[string]interface{}{},
		}),
	})

	if resp == nil {
		t.Fatal("expected response")
	}

	if resp.Error != nil {
		t.Fatalf("got error: %s", resp.Error.Message)
	}

	text := extractContentText(t, resp)

	var graph struct {
		Nodes []map[string]interface{} `json:"Nodes"`
		Edges []map[string]interface{} `json:"Edges"`
	}

	if err := json.Unmarshal([]byte(text), &graph); err != nil {
		t.Fatalf("error parsing graph: %v", err)
	}

	if len(graph.Nodes) == 0 {
		t.Error("expected nodes in graph")
	}
}

func TestToolsCallCheckViolations(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "main.go")

	if err := os.WriteFile(goFile, []byte(`package main

func main() {}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	server := createInitializedServer(t, tmpDir)

	id := json.RawMessage(`6`)
	resp := server.handleMessage(&jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "tools/call",
		Params: mustMarshal(map[string]interface{}{
			"name":      "check_violations",
			"arguments": map[string]interface{}{},
		}),
	})

	if resp == nil {
		t.Fatal("expected response")
	}

	if resp.Error != nil {
		t.Fatalf("got error: %s", resp.Error.Message)
	}

	text := extractContentText(t, resp)

	var report ViolationReport
	if err := json.Unmarshal([]byte(text), &report); err != nil {
		t.Fatalf("error parsing violation report: %v", err)
	}
}

func TestToolsCallGetFileMetrics(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "main.go")

	if err := os.WriteFile(goFile, []byte(`package main

type App struct {
	Name string
}

func NewApp() *App {
	return &App{}
}

func main() {}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	server := createInitializedServer(t, tmpDir)

	id := json.RawMessage(`20`)
	resp := server.handleMessage(&jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "tools/call",
		Params: mustMarshal(map[string]interface{}{
			"name":      "get_file_metrics",
			"arguments": map[string]string{"path": goFile},
		}),
	})

	if resp == nil {
		t.Fatal("expected response")
	}

	if resp.Error != nil {
		t.Fatalf("got error: %s", resp.Error.Message)
	}

	text := extractContentText(t, resp)

	var metrics FileMetrics
	if err := json.Unmarshal([]byte(text), &metrics); err != nil {
		t.Fatalf("error parsing metrics: %v", err)
	}

	if metrics.Types != 1 {
		t.Errorf("expected 1 type, got %d", metrics.Types)
	}

	if metrics.Functions != 2 {
		t.Errorf("expected 2 functions (NewApp + main), got %d", metrics.Functions)
	}

	if metrics.HealthScore <= 0 || metrics.HealthScore > 100 {
		t.Errorf("expected valid health score, got %d", metrics.HealthScore)
	}
}

func TestToolsCallGetDegradationReport(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "main.go")

	if err := os.WriteFile(goFile, []byte(`package main

func main() {}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	server := createInitializedServer(t, tmpDir)

	id := json.RawMessage(`21`)
	resp := server.handleMessage(&jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "tools/call",
		Params: mustMarshal(map[string]interface{}{
			"name":      "get_degradation_report",
			"arguments": map[string]string{"path": goFile},
		}),
	})

	if resp == nil {
		t.Fatal("expected response")
	}

	if resp.Error != nil {
		t.Fatalf("got error: %s", resp.Error.Message)
	}

	text := extractContentText(t, resp)

	var report DegradationReport
	if err := json.Unmarshal([]byte(text), &report); err != nil {
		t.Fatalf("error parsing degradation report: %v", err)
	}

	if report.Status == "" {
		t.Error("expected non-empty status in degradation report")
	}
}

func TestToolsCallGetDependencies(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "main.go")

	if err := os.WriteFile(goFile, []byte(`package main

func main() {}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	server := createInitializedServer(t, tmpDir)

	id := json.RawMessage(`7`)
	resp := server.handleMessage(&jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "tools/call",
		Params: mustMarshal(map[string]interface{}{
			"name":      "get_dependencies",
			"arguments": map[string]string{"path": goFile},
		}),
	})

	if resp == nil {
		t.Fatal("expected response")
	}

	if resp.Error != nil {
		t.Fatalf("got error: %s", resp.Error.Message)
	}
}

func TestToolsCallGetCallgraph(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "main.go")

	if err := os.WriteFile(goFile, []byte(`package main

func helper() {}

func main() {
	helper()
}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	server := createInitializedServer(t, tmpDir)

	// We need to find the actual function ID. It will be based on the temp dir path.
	graph := server.state.GetGraph()

	var mainID string

	for _, node := range graph.Nodes {
		if node.Title == "main" && node.Entity == "function" {
			mainID = node.ID

			break
		}
	}

	if mainID == "" {
		t.Fatal("could not find main function in graph")
	}

	id := json.RawMessage(`8`)
	resp := server.handleMessage(&jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "tools/call",
		Params: mustMarshal(map[string]interface{}{
			"name": "get_callgraph",
			"arguments": map[string]interface{}{
				"entry":     mainID,
				"max_depth": 5,
			},
		}),
	})

	if resp == nil {
		t.Fatal("expected response")
	}

	if resp.Error != nil {
		t.Fatalf("got error: %s", resp.Error.Message)
	}

	text := extractContentText(t, resp)

	var result CallGraphResult
	if err := json.Unmarshal([]byte(text), &result); err != nil {
		t.Fatalf("error parsing result: %v", err)
	}

	if len(result.Nodes) == 0 {
		t.Error("expected nodes in call graph")
	}
}

func TestMethodNotFound(t *testing.T) {
	server := NewServerWithIO(
		bytes.NewReader(nil),
		&bytes.Buffer{},
		log.New(io.Discard, "", 0),
	)

	id := json.RawMessage(`10`)
	resp := server.handleMessage(&jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "unknown/method",
	})

	if resp == nil {
		t.Fatal("expected error response")
	}

	if resp.Error == nil {
		t.Fatal("expected methodNotFound error")
	}

	if resp.Error.Code != codeMethodNotFound {
		t.Errorf("expected error code %d, got %d", codeMethodNotFound, resp.Error.Code)
	}
}

func TestPing(t *testing.T) {
	server := NewServerWithIO(
		bytes.NewReader(nil),
		&bytes.Buffer{},
		log.New(io.Discard, "", 0),
	)

	id := json.RawMessage(`11`)
	resp := server.handleMessage(&jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "ping",
	})

	if resp == nil {
		t.Fatal("expected response to ping")
	}

	if resp.Error != nil {
		t.Fatalf("unexpected error: %s", resp.Error.Message)
	}
}

func TestReadWriteMessage(t *testing.T) {
	id := json.RawMessage(`1`)
	original := &jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "test/method",
		Params:  json.RawMessage(`{"key":"value"}`),
	}

	buf := &bytes.Buffer{}
	server := NewServerWithIO(nil, buf, nil)

	if err := server.writeMessage(original); err != nil {
		t.Fatalf("error writing: %v", err)
	}

	server2 := NewServerWithIO(bytes.NewReader(buf.Bytes()), nil, nil)

	msg, err := server2.readMessage()
	if err != nil {
		t.Fatalf("error reading: %v", err)
	}

	if msg.Method != "test/method" {
		t.Errorf("expected method test/method, got %s", msg.Method)
	}
}

// createInitializedServer creates a server and initializes it with the given root dir.
func createInitializedServer(t *testing.T, rootDir string) *Server {
	t.Helper()

	server := NewServerWithIO(
		bytes.NewReader(nil),
		&bytes.Buffer{},
		log.New(io.Discard, "", 0),
	)

	id := json.RawMessage(`0`)
	resp := server.handleMessage(&jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "initialize",
		Params:  mustMarshal(map[string]string{"rootDir": rootDir}),
	})

	if resp != nil && resp.Error != nil {
		t.Fatalf("initialization error: %s", resp.Error.Message)
	}

	return server
}

// extractContentText extracts the text from an MCP content response.
func extractContentText(t *testing.T, resp *jsonrpcMessage) string {
	t.Helper()

	resultBytes, _ := json.Marshal(resp.Result)

	var mcpResult struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}

	if err := json.Unmarshal(resultBytes, &mcpResult); err != nil {
		t.Fatalf("error parsing MCP result: %v", err)
	}

	if len(mcpResult.Content) == 0 {
		t.Fatal("expected content in response")
	}

	return mcpResult.Content[0].Text
}

// mustMarshal serializes a value to JSON or panics.
func mustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}

	return data
}
