// Package lsp реализует LSP-сервер для архитектурного анализа Go-проектов.
package lsp

// LSP protocol types — минимальный набор, необходимый для работы сервера.

// InitializeParams содержит параметры запроса initialize.
type InitializeParams struct {
	ProcessID    int                `json:"processId"`
	RootURI      string             `json:"rootUri"`
	RootPath     string             `json:"rootPath"`
	Capabilities ClientCapabilities `json:"capabilities"`
}

// ClientCapabilities описывает возможности клиента.
type ClientCapabilities struct {
	TextDocument TextDocumentClientCapabilities `json:"textDocument,omitempty"`
	Workspace    WorkspaceClientCapabilities    `json:"workspace,omitempty"`
}

// TextDocumentClientCapabilities описывает возможности клиента по работе с документами.
type TextDocumentClientCapabilities struct {
	PublishDiagnostics PublishDiagnosticsCapability `json:"publishDiagnostics,omitempty"`
}

// PublishDiagnosticsCapability описывает поддержку диагностик клиентом.
type PublishDiagnosticsCapability struct {
	RelatedInformation bool `json:"relatedInformation,omitempty"`
}

// WorkspaceClientCapabilities описывает возможности клиента по работе с workspace.
type WorkspaceClientCapabilities struct {
	DidChangeWatchedFiles DidChangeWatchedFilesCapability `json:"didChangeWatchedFiles,omitempty"`
}

// DidChangeWatchedFilesCapability описывает поддержку отслеживания изменений файлов.
type DidChangeWatchedFilesCapability struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

// InitializeResult содержит результат запроса initialize.
type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
	ServerInfo   ServerInfo         `json:"serverInfo,omitempty"`
}

// ServerInfo содержит информацию о сервере.
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version,omitempty"`
}

// ServerCapabilities описывает возможности сервера.
type ServerCapabilities struct {
	TextDocumentSync        int                      `json:"textDocumentSync"`
	ExecuteCommandProvider  *ExecuteCommandOptions   `json:"executeCommandProvider,omitempty"`
	DiagnosticProvider      *DiagnosticOptions       `json:"diagnosticProvider,omitempty"`
	DidChangeWatchedFiles   bool                     `json:"-"`
	WorkspaceFileOperations *WorkspaceFileOperations `json:"workspace,omitempty"`
	CompletionProvider      *CompletionOptions       `json:"completionProvider,omitempty"`
	HoverProvider           bool                     `json:"hoverProvider,omitempty"`
	CodeActionProvider      bool                     `json:"codeActionProvider,omitempty"`
	CodeLensProvider        *CodeLensOptions         `json:"codeLensProvider,omitempty"`
}

// ExecuteCommandOptions описывает поддерживаемые команды.
type ExecuteCommandOptions struct {
	Commands []string `json:"commands"`
}

// DiagnosticOptions описывает параметры провайдера диагностик.
type DiagnosticOptions struct {
	InterFileDependencies bool `json:"interFileDependencies"`
	WorkspaceDiagnostics  bool `json:"workspaceDiagnostics"`
}

// WorkspaceFileOperations описывает поддержку workspace операций.
type WorkspaceFileOperations struct {
	FileOperations *FileOperations `json:"fileOperations,omitempty"`
}

// FileOperations описывает поддержку файловых операций.
type FileOperations struct{}

// CompletionOptions описывает параметры провайдера автодополнения.
type CompletionOptions struct {
	TriggerCharacters []string `json:"triggerCharacters,omitempty"`
}

// CodeLensOptions описывает параметры провайдера code lens.
type CodeLensOptions struct {
	ResolveProvider bool `json:"resolveProvider,omitempty"`
}

// DidOpenTextDocumentParams содержит параметры уведомления textDocument/didOpen.
type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

// DidChangeTextDocumentParams содержит параметры уведомления textDocument/didChange.
type DidChangeTextDocumentParams struct {
	TextDocument   VersionedTextDocumentIdentifier  `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

// DidSaveTextDocumentParams содержит параметры уведомления textDocument/didSave.
type DidSaveTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// DidCloseTextDocumentParams содержит параметры уведомления textDocument/didClose.
type DidCloseTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

// DidChangeWatchedFilesParams содержит параметры уведомления workspace/didChangeWatchedFiles.
type DidChangeWatchedFilesParams struct {
	Changes []FileEvent `json:"changes"`
}

// ExecuteCommandParams содержит параметры запроса workspace/executeCommand.
type ExecuteCommandParams struct {
	Command   string        `json:"command"`
	Arguments []interface{} `json:"arguments,omitempty"`
}

// TextDocumentItem описывает открытый документ.
type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

// TextDocumentIdentifier идентифицирует документ.
type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

// VersionedTextDocumentIdentifier идентифицирует документ с версией.
type VersionedTextDocumentIdentifier struct {
	URI     string `json:"uri"`
	Version int    `json:"version"`
}

// TextDocumentContentChangeEvent описывает изменение содержимого документа.
type TextDocumentContentChangeEvent struct {
	Text string `json:"text"`
}

// FileEvent описывает событие изменения файла.
type FileEvent struct {
	URI  string `json:"uri"`
	Type int    `json:"type"`
}

// Константы типов файловых событий.
const (
	FileEventCreated = 1
	FileEventChanged = 2
	FileEventDeleted = 3
)

// Diagnostic описывает диагностическое сообщение.
type Diagnostic struct {
	Range    Range           `json:"range"`
	Severity int             `json:"severity,omitempty"`
	Source   string          `json:"source,omitempty"`
	Message  string          `json:"message"`
	Code     string          `json:"code,omitempty"`
	Tags     []DiagnosticTag `json:"tags,omitempty"`
}

// DiagnosticTag описывает тег диагностики.
type DiagnosticTag int

// Константы серьёзности диагностик.
const (
	SeverityError       = 1
	SeverityWarning     = 2
	SeverityInformation = 3
	SeverityHint        = 4
)

// Range описывает диапазон в документе.
type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

// Position описывает позицию в документе.
type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

// PublishDiagnosticsParams содержит параметры уведомления textDocument/publishDiagnostics.
type PublishDiagnosticsParams struct {
	URI         string       `json:"uri"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// LogMessageParams содержит параметры уведомления window/logMessage.
type LogMessageParams struct {
	Type    int    `json:"type"`
	Message string `json:"message"`
}

// Константы типов логирования.
const (
	LogError   = 1
	LogWarning = 2
	LogInfo    = 3
	LogDebug   = 4
)
