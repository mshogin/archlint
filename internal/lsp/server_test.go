package lsp

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

// buildLSPMessage формирует LSP-сообщение с заголовком Content-Length.
func buildLSPMessage(msg interface{}) []byte {
	body, _ := json.Marshal(msg)

	return []byte(fmt.Sprintf("Content-Length: %d\r\n\r\n%s", len(body), body))
}

// parseLSPResponse извлекает JSON-RPC ответ из LSP-вывода.
func parseLSPResponse(data []byte) (*jsonrpcMessage, error) {
	str := string(data)

	// Находим все ответы, возвращаем последний с id.
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
		return nil, fmt.Errorf("ответ не найден в данных: %s", string(data))
	}

	return lastResp, nil
}

func TestInitialize(t *testing.T) {
	// Создаём временную директорию с Go-файлом.
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "main.go")

	if err := os.WriteFile(goFile, []byte(`package main

func main() {}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	id := json.RawMessage(`1`)
	initMsg := buildLSPMessage(jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "initialize",
		Params: mustMarshal(InitializeParams{
			RootURI: "file://" + tmpDir,
		}),
	})

	input := bytes.NewReader(initMsg)
	output := &bytes.Buffer{}
	logger := log.New(io.Discard, "", 0)

	server := NewServerWithIO(input, output, logger)

	// Читаем и обрабатываем одно сообщение вручную.
	msg, err := server.readMessage()
	if err != nil {
		t.Fatalf("ошибка чтения сообщения: %v", err)
	}

	resp := server.handleMessage(msg)
	if resp == nil {
		t.Fatal("ожидался ответ на initialize")
	}

	if err := server.writeMessage(resp); err != nil {
		t.Fatalf("ошибка записи ответа: %v", err)
	}

	response, err := parseLSPResponse(output.Bytes())
	if err != nil {
		t.Fatalf("ошибка разбора ответа: %v", err)
	}

	if response.Error != nil {
		t.Fatalf("получена ошибка: %s", response.Error.Message)
	}

	resultBytes, _ := json.Marshal(response.Result)

	var result InitializeResult
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("ошибка разбора результата: %v", err)
	}

	if result.ServerInfo.Name != "archlint-lsp" {
		t.Errorf("ожидалось имя сервера archlint-lsp, получено %s", result.ServerInfo.Name)
	}

	if result.Capabilities.TextDocumentSync != 1 {
		t.Errorf("ожидался TextDocumentSync=1, получено %d", result.Capabilities.TextDocumentSync)
	}

	if result.Capabilities.ExecuteCommandProvider == nil {
		t.Error("ожидался ExecuteCommandProvider")
	} else if len(result.Capabilities.ExecuteCommandProvider.Commands) != 4 {
		t.Errorf("ожидалось 4 команды, получено %d", len(result.Capabilities.ExecuteCommandProvider.Commands))
	}
}

func TestExecuteCommandAnalyzeFile(t *testing.T) {
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

	id := json.RawMessage(`2`)
	resp := server.handleMessage(&jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "workspace/executeCommand",
		Params: mustMarshal(ExecuteCommandParams{
			Command:   "archlint.analyzeFile",
			Arguments: []interface{}{goFile},
		}),
	})

	if resp == nil {
		t.Fatal("ожидался ответ")
	}

	if resp.Error != nil {
		t.Fatalf("получена ошибка: %s", resp.Error.Message)
	}

	resultBytes, _ := json.Marshal(resp.Result)

	var result FileAnalysis
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("ошибка разбора результата: %v", err)
	}

	if len(result.Types) == 0 {
		t.Error("ожидались типы в результате анализа")
	}

	if len(result.Functions) == 0 {
		t.Error("ожидались функции в результате анализа")
	}

	if len(result.Methods) == 0 {
		t.Error("ожидались методы в результате анализа")
	}
}

func TestExecuteCommandGetGraph(t *testing.T) {
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

	id := json.RawMessage(`3`)
	resp := server.handleMessage(&jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "workspace/executeCommand",
		Params: mustMarshal(ExecuteCommandParams{
			Command:   "archlint.getGraph",
			Arguments: nil,
		}),
	})

	if resp == nil {
		t.Fatal("ожидался ответ")
	}

	if resp.Error != nil {
		t.Fatalf("получена ошибка: %s", resp.Error.Message)
	}

	resultBytes, _ := json.Marshal(resp.Result)

	var graph struct {
		Nodes []map[string]interface{} `json:"Nodes"`
		Edges []map[string]interface{} `json:"Edges"`
	}

	if err := json.Unmarshal(resultBytes, &graph); err != nil {
		t.Fatalf("ошибка разбора графа: %v", err)
	}

	if len(graph.Nodes) == 0 {
		t.Error("ожидались узлы в графе")
	}
}

func TestExecuteCommandGetMetrics(t *testing.T) {
	tmpDir := t.TempDir()
	goFile := filepath.Join(tmpDir, "main.go")

	if err := os.WriteFile(goFile, []byte(`package main

func main() {}
`), 0o644); err != nil {
		t.Fatal(err)
	}

	server := createInitializedServer(t, tmpDir)

	// Без аргументов — общая статистика.
	id := json.RawMessage(`4`)
	resp := server.handleMessage(&jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "workspace/executeCommand",
		Params: mustMarshal(ExecuteCommandParams{
			Command:   "archlint.getMetrics",
			Arguments: nil,
		}),
	})

	if resp == nil {
		t.Fatal("ожидался ответ")
	}

	if resp.Error != nil {
		t.Fatalf("получена ошибка: %s", resp.Error.Message)
	}
}

func TestExecuteCommandAnalyzeChange(t *testing.T) {
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

	id := json.RawMessage(`5`)
	resp := server.handleMessage(&jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "workspace/executeCommand",
		Params: mustMarshal(ExecuteCommandParams{
			Command:   "archlint.analyzeChange",
			Arguments: []interface{}{goFile},
		}),
	})

	if resp == nil {
		t.Fatal("ожидался ответ")
	}

	if resp.Error != nil {
		t.Fatalf("получена ошибка: %s", resp.Error.Message)
	}

	resultBytes, _ := json.Marshal(resp.Result)

	var result ChangeAnalysis
	if err := json.Unmarshal(resultBytes, &result); err != nil {
		t.Fatalf("ошибка разбора результата: %v", err)
	}

	if len(result.AffectedNodes) == 0 {
		t.Error("ожидались затронутые узлы")
	}
}

func TestShutdown(t *testing.T) {
	server := NewServerWithIO(
		bytes.NewReader(nil),
		&bytes.Buffer{},
		log.New(io.Discard, "", 0),
	)

	id := json.RawMessage(`99`)
	resp := server.handleMessage(&jsonrpcMessage{
		JSONRPC: "2.0",
		ID:      &id,
		Method:  "shutdown",
	})

	if resp == nil {
		t.Fatal("ожидался ответ на shutdown")
	}

	if resp.Error != nil {
		t.Fatalf("получена ошибка при shutdown: %s", resp.Error.Message)
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
		Method:  "textDocument/completion",
	})

	if resp == nil {
		t.Fatal("ожидался ответ с ошибкой")
	}

	if resp.Error == nil {
		t.Fatal("ожидалась ошибка methodNotFound")
	}

	if resp.Error.Code != codeMethodNotFound {
		t.Errorf("ожидался код ошибки %d, получено %d", codeMethodNotFound, resp.Error.Code)
	}
}

func TestURIToPath(t *testing.T) {
	tests := []struct {
		uri      string
		expected string
	}{
		{"file:///home/user/project", "/home/user/project"},
		{"file:///tmp/test.go", "/tmp/test.go"},
		{"/tmp/test.go", "/tmp/test.go"},
	}

	for _, tt := range tests {
		result := uriToPath(tt.uri)
		if result != tt.expected {
			t.Errorf("uriToPath(%q) = %q, ожидалось %q", tt.uri, result, tt.expected)
		}
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

	// Записываем.
	buf := &bytes.Buffer{}
	server := NewServerWithIO(nil, buf, nil)

	if err := server.writeMessage(original); err != nil {
		t.Fatalf("ошибка записи: %v", err)
	}

	// Читаем.
	server2 := NewServerWithIO(bytes.NewReader(buf.Bytes()), nil, nil)

	msg, err := server2.readMessage()
	if err != nil {
		t.Fatalf("ошибка чтения: %v", err)
	}

	if msg.Method != "test/method" {
		t.Errorf("ожидался метод test/method, получено %s", msg.Method)
	}
}

// createInitializedServer создаёт сервер и выполняет инициализацию.
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
		Params: mustMarshal(InitializeParams{
			RootURI: "file://" + rootDir,
		}),
	})

	if resp != nil && resp.Error != nil {
		t.Fatalf("ошибка инициализации: %s", resp.Error.Message)
	}

	return server
}

// mustMarshal сериализует значение в JSON или паникует.
func mustMarshal(v interface{}) json.RawMessage {
	data, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}

	return data
}
