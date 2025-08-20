package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ClangdClient manages the clangd subprocess and LSP communication
type ClangdClient struct {
	cmd           *exec.Cmd
	transport     *Transport
	ProjectRoot   string // Exported for commands to access
	buildDir      string
	indexingDone  chan struct{}
	isIndexing    bool
	indexingMu    sync.RWMutex
	openDocuments map[string]bool
	docMu         sync.RWMutex
	capabilities  *ServerCapabilities
	timeout       time.Duration
}

// NewClangdClient creates and initializes a new clangd client
func NewClangdClient(projectRoot, buildDir string) (*ClangdClient, error) {
	// Find clangd executable
	clangdPath, err := exec.LookPath("clangd")
	if err != nil {
		return nil, fmt.Errorf("clangd not found in PATH")
	}

	// Start clangd process
	cmd := exec.Command(clangdPath,
		"--background-index",
		fmt.Sprintf("--compile-commands-dir=%s", buildDir),
		"--log=verbose",
		"--header-insertion=never",
		"--pch-storage=memory",
		"--ranking-model=decision_forest",
		"--all-scopes-completion",
		"--completion-style=detailed",
		"--function-arg-placeholders",
		"--header-insertion-decorators")

	cmd.Stderr = os.Stderr

	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}

	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("failed to start clangd: %v", err)
	}

	client := &ClangdClient{
		cmd:           cmd,
		transport:     NewTransport(stdoutPipe, stdinPipe, os.Stderr),
		ProjectRoot:   projectRoot,
		buildDir:      buildDir,
		indexingDone:  make(chan struct{}),
		openDocuments: make(map[string]bool),
		timeout:       30 * time.Second,
	}

	// Register notification handlers
	client.transport.RegisterNotificationHandler("$/progress", client.handleProgress)
	client.transport.RegisterNotificationHandler("textDocument/publishDiagnostics", client.handleDiagnostics)
	client.transport.RegisterNotificationHandler("window/logMessage", client.handleLogMessage)

	// Start transport
	client.transport.Start()

	// Initialize
	if err := client.initialize(); err != nil {
		client.Stop()
		return nil, fmt.Errorf("failed to initialize clangd: %v", err)
	}

	return client, nil
}

// initialize sends the initialize request to clangd
func (c *ClangdClient) initialize() error {
	pid := os.Getpid()
	params := InitializeParams{
		ProcessID: &pid,
		RootURI:   "file://" + c.ProjectRoot,
		Capabilities: ClientCapabilities{
			TextDocument: TextDocumentClientCapabilities{
				Synchronization: TextDocumentSyncClientCapabilities{
					DynamicRegistration: false,
					WillSave:            false,
					WillSaveWaitUntil:   false,
					DidSave:             true,
				},
				Hover: HoverClientCapabilities{
					DynamicRegistration: false,
					ContentFormat:       []string{"markdown", "plaintext"},
				},
				Definition: DefinitionClientCapabilities{
					DynamicRegistration: false,
					LinkSupport:         false,
				},
				References: ReferencesClientCapabilities{
					DynamicRegistration: false,
				},
				DocumentSymbol: DocumentSymbolClientCapabilities{
					DynamicRegistration:               false,
					HierarchicalDocumentSymbolSupport: true,
				},
				FoldingRange: FoldingRangeClientCapabilities{
					DynamicRegistration: false,
					RangeLimit:          5000,
					LineFoldingOnly:     false,
				},
				TypeHierarchy: TypeHierarchyClientCapabilities{
					DynamicRegistration: false,
				},
			},
			Workspace: WorkspaceClientCapabilities{
				Symbol: WorkspaceSymbolClientCapabilities{
					DynamicRegistration: false,
				},
				DidChangeWatchedFiles: DidChangeWatchedFilesClientCapabilities{
					DynamicRegistration: false,
				},
			},
		},
	}

	result, err := c.sendRequest("initialize", params)
	if err != nil {
		return err
	}

	var initResult InitializeResult
	if err := json.Unmarshal(result, &initResult); err != nil {
		return err
	}

	c.capabilities = &initResult.Capabilities

	// Send initialized notification
	if err := c.transport.SendNotification("initialized", struct{}{}); err != nil {
		return err
	}

	// Mark indexing as started
	c.indexingMu.Lock()
	c.isIndexing = true
	c.indexingMu.Unlock()

	// Open first source file to trigger indexing
	// This is required for workspace/symbol queries to work
	// See: https://github.com/clangd/clangd/discussions/1339
	if firstFile := c.getFirstSourceFile(); firstFile != "" {
		if err := c.OpenDocument("file://" + firstFile); err != nil {
			// Log but don't fail - indexing may still work
			fmt.Fprintf(os.Stderr, "Warning: Failed to open first source file: %v\n", err)
		}
	}

	// Set indexing timeout
	go func() {
		time.Sleep(5 * time.Second)
		c.indexingMu.Lock()
		if c.isIndexing {
			c.isIndexing = false
			select {
			case <-c.indexingDone:
				// Already closed
			default:
				close(c.indexingDone)
			}
		}
		c.indexingMu.Unlock()
	}()

	return nil
}

// handleProgress handles progress notifications from clangd
func (c *ClangdClient) handleProgress(params json.RawMessage) {
	var progress ProgressParams
	if err := json.Unmarshal(params, &progress); err != nil {
		return
	}

	c.indexingMu.Lock()
	defer c.indexingMu.Unlock()

	if progress.Value.Kind == "begin" && strings.Contains(strings.ToLower(progress.Value.Title), "index") {
		if !c.isIndexing {
			c.isIndexing = true
		}
	} else if progress.Value.Kind == "end" {
		if c.isIndexing {
			c.isIndexing = false
			select {
			case <-c.indexingDone:
				// Already closed
			default:
				close(c.indexingDone)
			}
		}
	}
}

// handleDiagnostics handles diagnostic notifications from clangd
func (c *ClangdClient) handleDiagnostics(params json.RawMessage) {
	// We ignore diagnostics for now
}

// handleLogMessage handles log messages from clangd
func (c *ClangdClient) handleLogMessage(params json.RawMessage) {
	// We ignore log messages for now
}

// WaitForIndexing waits for clangd to finish indexing
func (c *ClangdClient) WaitForIndexing() {
	select {
	case <-c.indexingDone:
	case <-time.After(5 * time.Second):
		// Timeout - assume indexing is done or not needed
	}
}

// sendRequest sends a request to clangd with timeout
func (c *ClangdClient) sendRequest(method string, params interface{}) (json.RawMessage, error) {
	resultChan := make(chan struct {
		result json.RawMessage
		err    error
	}, 1)

	go func() {
		result, err := c.transport.SendRequest(method, params)
		resultChan <- struct {
			result json.RawMessage
			err    error
		}{result, err}
	}()

	select {
	case res := <-resultChan:
		return res.result, res.err
	case <-time.After(c.timeout):
		return nil, fmt.Errorf("request timeout after %v", c.timeout)
	}
}

// OpenDocument opens a document in clangd
func (c *ClangdClient) OpenDocument(uri string) error {
	c.docMu.Lock()
	if c.openDocuments[uri] {
		c.docMu.Unlock()
		return nil // Already open
	}
	c.openDocuments[uri] = true
	c.docMu.Unlock()

	// Read file content
	path := strings.TrimPrefix(uri, "file://")
	content, err := os.ReadFile(path)
	if err != nil {
		c.docMu.Lock()
		delete(c.openDocuments, uri)
		c.docMu.Unlock()
		return err
	}

	params := DidOpenTextDocumentParams{
		TextDocument: TextDocumentItem{
			URI:        uri,
			LanguageID: getLanguageID(path),
			Version:    1,
			Text:       string(content),
		},
	}

	return c.transport.SendNotification("textDocument/didOpen", params)
}

// CloseDocument closes a document in clangd
func (c *ClangdClient) CloseDocument(uri string) error {
	c.docMu.Lock()
	if !c.openDocuments[uri] {
		c.docMu.Unlock()
		return nil // Not open
	}
	delete(c.openDocuments, uri)
	c.docMu.Unlock()

	params := DidCloseTextDocumentParams{
		TextDocument: TextDocumentIdentifier{
			URI: uri,
		},
	}

	return c.transport.SendNotification("textDocument/didClose", params)
}

// GetDefinition gets the definition location for a symbol
func (c *ClangdClient) GetDefinition(uri string, position Position) ([]Location, error) {
	if err := c.OpenDocument(uri); err != nil {
		return nil, err
	}

	params := DefinitionParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     position,
		},
	}

	result, err := c.sendRequest("textDocument/definition", params)
	if err != nil {
		return nil, err
	}

	// Result can be Location, Location[], or null
	var locations []Location
	
	// Try as array first
	if err := json.Unmarshal(result, &locations); err == nil {
		return locations, nil
	}

	// Try as single location
	var location Location
	if err := json.Unmarshal(result, &location); err == nil {
		return []Location{location}, nil
	}

	return []Location{}, nil
}

// GetDeclaration gets the declaration location for a symbol
func (c *ClangdClient) GetDeclaration(uri string, position Position) ([]Location, error) {
	if err := c.OpenDocument(uri); err != nil {
		return nil, err
	}

	params := DeclarationParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     position,
		},
	}

	result, err := c.sendRequest("textDocument/declaration", params)
	if err != nil {
		return nil, err
	}

	// Result can be Location, Location[], or null
	var locations []Location
	
	// Try as array first
	if err := json.Unmarshal(result, &locations); err == nil {
		return locations, nil
	}

	// Try as single location
	var location Location
	if err := json.Unmarshal(result, &location); err == nil {
		return []Location{location}, nil
	}

	return []Location{}, nil
}

// GetReferences finds all references to a symbol
func (c *ClangdClient) GetReferences(uri string, position Position, includeDeclaration bool) ([]Location, error) {
	if err := c.OpenDocument(uri); err != nil {
		return nil, err
	}

	params := ReferenceParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     position,
		},
		Context: ReferenceContext{
			IncludeDeclaration: includeDeclaration,
		},
	}

	result, err := c.sendRequest("textDocument/references", params)
	if err != nil {
		return nil, err
	}

	var locations []Location
	if err := json.Unmarshal(result, &locations); err != nil {
		return nil, err
	}

	return locations, nil
}

// GetHover gets hover information for a position
func (c *ClangdClient) GetHover(uri string, position Position) (*Hover, error) {
	if err := c.OpenDocument(uri); err != nil {
		return nil, err
	}

	params := HoverParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     position,
		},
	}

	result, err := c.sendRequest("textDocument/hover", params)
	if err != nil {
		return nil, err
	}

	var hover Hover
	if err := json.Unmarshal(result, &hover); err != nil {
		return nil, err
	}

	return &hover, nil
}

// GetDocumentSymbols gets all symbols in a document
func (c *ClangdClient) GetDocumentSymbols(uri string) ([]DocumentSymbol, error) {
	if err := c.OpenDocument(uri); err != nil {
		return nil, err
	}

	params := DocumentSymbolParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
	}

	result, err := c.sendRequest("textDocument/documentSymbol", params)
	if err != nil {
		return nil, err
	}

	var symbols []DocumentSymbol
	if err := json.Unmarshal(result, &symbols); err != nil {
		return nil, err
	}

	return symbols, nil
}

// GetFoldingRanges gets folding ranges for a document
func (c *ClangdClient) GetFoldingRanges(uri string) ([]FoldingRange, error) {
	if err := c.OpenDocument(uri); err != nil {
		return nil, err
	}

	params := FoldingRangeParams{
		TextDocument: TextDocumentIdentifier{URI: uri},
	}

	result, err := c.sendRequest("textDocument/foldingRange", params)
	if err != nil {
		return nil, err
	}

	var ranges []FoldingRange
	if err := json.Unmarshal(result, &ranges); err != nil {
		return nil, err
	}

	return ranges, nil
}

// WorkspaceSymbol searches for symbols across the workspace
func (c *ClangdClient) WorkspaceSymbol(query string) ([]WorkspaceSymbol, error) {
	c.WaitForIndexing()

	params := WorkspaceSymbolParams{
		Query: query,
	}

	result, err := c.sendRequest("workspace/symbol", params)
	if err != nil {
		return nil, err
	}

	var symbols []WorkspaceSymbol
	if err := json.Unmarshal(result, &symbols); err != nil {
		return nil, err
	}

	return symbols, nil
}

// PrepareTypeHierarchy prepares type hierarchy for a position
func (c *ClangdClient) PrepareTypeHierarchy(uri string, position Position) ([]TypeHierarchyItem, error) {
	if err := c.OpenDocument(uri); err != nil {
		return nil, err
	}

	params := TypeHierarchyPrepareParams{
		TextDocumentPositionParams: TextDocumentPositionParams{
			TextDocument: TextDocumentIdentifier{URI: uri},
			Position:     position,
		},
	}

	result, err := c.sendRequest("textDocument/prepareTypeHierarchy", params)
	if err != nil {
		return nil, err
	}

	var items []TypeHierarchyItem
	if err := json.Unmarshal(result, &items); err != nil {
		return nil, err
	}

	return items, nil
}

// GetSupertypes gets the supertypes of a type hierarchy item
func (c *ClangdClient) GetSupertypes(item TypeHierarchyItem) ([]TypeHierarchyItem, error) {
	params := TypeHierarchySupertypesParams{
		Item: item,
	}

	result, err := c.sendRequest("typeHierarchy/supertypes", params)
	if err != nil {
		return nil, err
	}

	var items []TypeHierarchyItem
	if err := json.Unmarshal(result, &items); err != nil {
		return nil, err
	}

	return items, nil
}

// GetSubtypes gets the subtypes of a type hierarchy item
func (c *ClangdClient) GetSubtypes(item TypeHierarchyItem) ([]TypeHierarchyItem, error) {
	params := TypeHierarchySubtypesParams{
		Item: item,
	}

	result, err := c.sendRequest("typeHierarchy/subtypes", params)
	if err != nil {
		return nil, err
	}

	var items []TypeHierarchyItem
	if err := json.Unmarshal(result, &items); err != nil {
		return nil, err
	}

	return items, nil
}

// OnFilesChanged handles file change notifications
func (c *ClangdClient) OnFilesChanged(files []string) {
	// Implement the close/reopen workaround for reindexing
	for _, file := range files {
		uri := "file://" + file
		
		c.docMu.RLock()
		isOpen := c.openDocuments[uri]
		c.docMu.RUnlock()
		
		if isOpen {
			// Close and reopen to force reindexing
			c.CloseDocument(uri)
			c.OpenDocument(uri)
		}
	}

	// Also send didChangeWatchedFiles notification
	events := make([]FileEvent, len(files))
	for i, file := range files {
		events[i] = FileEvent{
			URI:  "file://" + file,
			Type: FileChangeTypeChanged,
		}
	}

	params := DidChangeWatchedFilesParams{
		Changes: events,
	}

	c.transport.SendNotification("workspace/didChangeWatchedFiles", params)
}

// Shutdown sends the shutdown request
func (c *ClangdClient) Shutdown() error {
	_, err := c.sendRequest("shutdown", ShutdownParams{})
	return err
}

// Exit sends the exit notification
func (c *ClangdClient) Exit() error {
	return c.transport.SendNotification("exit", ExitParams{})
}

// getFirstSourceFile reads the first source file from compile_commands.json
// This is needed to trigger indexing in clangd
func (c *ClangdClient) getFirstSourceFile() string {
	compileCommandsPath := filepath.Join(c.buildDir, "compile_commands.json")
	
	data, err := os.ReadFile(compileCommandsPath)
	if err != nil {
		return ""
	}
	
	var commands []struct {
		File string `json:"file"`
	}
	
	if err := json.Unmarshal(data, &commands); err != nil {
		return ""
	}
	
	// Find the first .cc or .cpp file (skip headers)
	for _, cmd := range commands {
		if strings.HasSuffix(cmd.File, ".cc") || strings.HasSuffix(cmd.File, ".cpp") {
			return cmd.File
		}
	}
	
	// If no implementation files found, just use the first file
	if len(commands) > 0 {
		return commands[0].File
	}
	
	return ""
}

// Stop stops the clangd process
func (c *ClangdClient) Stop() error {
	// Try graceful shutdown first
	c.Shutdown()
	c.Exit()

	// Give it time to exit
	done := make(chan error, 1)
	go func() {
		done <- c.cmd.Wait()
	}()

	select {
	case <-done:
		return nil
	case <-time.After(2 * time.Second):
		// Force kill if it doesn't exit
		return c.cmd.Process.Kill()
	}
}

// ReadFileLines reads specific lines from a file
func ReadFileLines(path string, startLine, endLine int) ([]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var lines []string
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		if lineNum >= startLine && lineNum <= endLine {
			lines = append(lines, scanner.Text())
		}
		lineNum++
		if lineNum > endLine {
			break
		}
	}

	return lines, scanner.Err()
}

// getLanguageID returns the language ID for a file
func getLanguageID(path string) string {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".c":
		return "c"
	case ".cpp", ".cc", ".cxx", ".c++":
		return "cpp"
	case ".h", ".hpp", ".hxx", ".h++", ".hh":
		return "cpp" // Assume C++ for headers
	default:
		return "cpp"
	}
}