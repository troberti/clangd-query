package lsp

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"clangd-query/internal/logger"
)

// ClangdClient manages a clangd language server subprocess and handles LSP communication.
// It provides high-level methods for C++ code intelligence operations like finding definitions,
// references, and symbols. The client maintains the lifecycle of the clangd process and
// tracks which documents are open to optimize clangd's memory usage.
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
	logger        logger.Logger
}

// Path helper methods

// Converts a relative or absolute file path to an absolute path based on the project root.
// If the path is already absolute, it returns the path unchanged.
func (c *ClangdClient) ToAbsolutePath(relativePath string) string {
	if filepath.IsAbs(relativePath) {
		return relativePath
	}
	return filepath.Join(c.ProjectRoot, relativePath)
}

// Converts an absolute path to a relative path based on the project root.
// If the path cannot be made relative, it returns the absolute path.
func (c *ClangdClient) ToRelativePath(absolutePath string) string {
	rel, err := filepath.Rel(c.ProjectRoot, absolutePath)
	if err != nil {
		return absolutePath
	}
	return rel
}

// Converts a file path to a proper file URI.
// The path is first converted to an absolute path if needed.
func (c *ClangdClient) FileURIFromPath(filePath string) string {
	absolutePath := c.ToAbsolutePath(filePath)
	// Properly encode the path as a file URI
	u := &url.URL{
		Scheme: "file",
		Path:   absolutePath,
	}
	return u.String()
}

// Extracts the file path from a file URI.
// Properly handles URL encoding (e.g., %20 for spaces, %2B for +).
func (c *ClangdClient) PathFromFileURI(uri string) string {
	if !strings.HasPrefix(uri, "file://") {
		// Not a file URI, return as-is
		return uri
	}

	// Parse the URI to properly decode it
	u, err := url.Parse(uri)
	if err != nil {
		// Fallback to simple trimming if parsing fails
		return strings.TrimPrefix(uri, "file://")
	}

	// The path is already decoded by url.Parse
	return u.Path
}

// Creates and initializes a new clangd client for the given project.
// This function starts the clangd subprocess, establishes LSP communication,
// and waits for initial indexing to complete. The buildDir should contain
// a compile_commands.json file for accurate code intelligence.
func NewClangdClient(projectRoot, buildDir string, log logger.Logger) (*ClangdClient, error) {
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

	// Create a pipe to capture and parse clangd's stderr
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}

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

	transport := NewTransport(stdoutPipe, stdinPipe, os.Stderr)

	client := &ClangdClient{
		cmd:           cmd,
		transport:     transport,
		ProjectRoot:   projectRoot,
		buildDir:      buildDir,
		indexingDone:  make(chan struct{}),
		openDocuments: make(map[string]bool),
		timeout:       30 * time.Second,
		logger:        log,
	}

	// Start goroutine to parse clangd stderr
	go client.parseClangdLogs(stderrPipe)

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
		RootURI:   c.FileURIFromPath(c.ProjectRoot),
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
		c.logger.Debug("Opening initial file to trigger indexing: %s", firstFile)
		if err := c.OpenDocument(c.FileURIFromPath(firstFile)); err != nil {
			// Log but don't fail - indexing may still work
			c.logger.Info("Warning: Failed to open initial source file %s: %v", firstFile, err)
		} else {
			c.logger.Info("Successfully opened initial file for indexing: %s", firstFile)
		}
	} else {
		c.logger.Info("Warning: No source file found to trigger initial indexing - workspace/symbol queries may not work")
	}

	// Set indexing timeout
	go func() {
		time.Sleep(5 * time.Second)
		c.indexingMu.Lock()
		defer c.indexingMu.Unlock()
		if c.isIndexing {
			c.isIndexing = false
			// Use sync.Once or similar pattern would be better, but for now just be careful
			select {
			case <-c.indexingDone:
				// Already closed
			default:
				close(c.indexingDone)
			}
		}
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

// Sends a request to clangd and waits for the response.
// The underlying transport handles timeouts (30 seconds by default), so this method
// will not block indefinitely. If the connection to clangd is lost, this method
// returns an error immediately rather than attempting to reconnect.
func (c *ClangdClient) sendRequest(method string, params interface{}) (json.RawMessage, error) {
	result, err := c.transport.SendRequest(method, params)
	if err != nil {
		c.logger.Error("Request %s failed: %v", method, err)
	}
	return result, err
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
	path := c.PathFromFileURI(uri)
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
		uri := c.FileURIFromPath(file)

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
			URI:  c.FileURIFromPath(file),
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

// Reads the first suitable source file from compile_commands.json to trigger
// clangd indexing. This is necessary for workspace/symbol queries to work properly.
// The function prefers implementation files (.cc, .cpp) over headers to ensure
// better indexing coverage. Returns an empty string if no suitable file is found.
func (c *ClangdClient) getFirstSourceFile() string {
	compileCommandsPath := filepath.Join(c.buildDir, "compile_commands.json")
	data, err := os.ReadFile(compileCommandsPath)
	if err != nil {
		if os.IsNotExist(err) {
			c.logger.Error("compile_commands.json not found at %s - indexing may not work properly", compileCommandsPath)
		} else {
			c.logger.Error("Failed to read compile_commands.json: %v", err)
		}
		return ""
	}

	var commands []struct {
		File string `json:"file"`
	}

	if err := json.Unmarshal(data, &commands); err != nil {
		c.logger.Error("Failed to parse compile_commands.json: %v", err)
		return ""
	}

	if len(commands) == 0 {
		c.logger.Info("Warning: compile_commands.json is empty - no files to index")
		return ""
	}

	// Find the first .cc or .cpp file (skip headers)
	for _, cmd := range commands {
		if strings.HasSuffix(cmd.File, ".cc") || strings.HasSuffix(cmd.File, ".cpp") {
			c.logger.Info("Selected source file for initial indexing: %s", cmd.File)
			return cmd.File
		}
	}

	// If no implementation files found, just use the first file
	firstFile := commands[0].File
	c.logger.Info("No .cc/.cpp files found, using first file for indexing: %s", firstFile)
	return firstFile
}

// Stop stops the clangd process
func (c *ClangdClient) Stop() error {
	// Try graceful shutdown first
	if err := c.Shutdown(); err != nil {
		c.logger.Debug("Shutdown request failed: %v", err)
	}
	if err := c.Exit(); err != nil {
		c.logger.Debug("Exit notification failed: %v", err)
	}

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

// Reads and processes clangd's stderr output, parsing log levels and forwarding to our logger.
// This function handles the verbose output from clangd which can include very long lines
// (e.g., C++ template errors or AST dumps). It uses a 10MB buffer to handle these cases
// without failing, and intelligently truncates extremely long lines for logging to keep
// log files manageable while preserving the most important information.
func (c *ClangdClient) parseClangdLogs(stderr io.Reader) {
	scanner := bufio.NewScanner(stderr)

	// Set a much larger buffer for the scanner (10MB instead of default 64KB)
	// This handles long C++ template errors and verbose diagnostic output
	const maxLineSize = 10 * 1024 * 1024 // 10MB
	buf := make([]byte, 0, 64*1024)      // Start with 64KB
	scanner.Buffer(buf, maxLineSize)

	for scanner.Scan() {
		line := scanner.Text()

		// Truncate extremely long lines for logging (keep first and last part)
		const maxLogLength = 4096 // Log at most 4KB per line
		if len(line) > maxLogLength {
			// Keep first 2KB and last 1KB with ellipsis in middle
			truncated := line[:2048] + " ... [truncated " +
				strconv.Itoa(len(line)-3072) + " bytes] ... " +
				line[len(line)-1024:]
			line = truncated
		}

		// Parse clangd log levels
		// V[timestamp] = verbose/debug
		// I[timestamp] = info
		// E[timestamp] = error
		if len(line) > 0 {
			switch line[0] {
			case 'V':
				c.logger.Debug("[CLANGD] %s", line)
			case 'I':
				c.logger.Info("[CLANGD] %s", line)
			case 'E':
				c.logger.Error("[CLANGD] %s", line)
			default:
				// Unknown format, log as info
				c.logger.Info("[CLANGD] %s", line)
			}
		}
	}

	// Check for scanner error. These are serious errors, as not completely
	// reading from clangd can block the clangd process.
	if err := scanner.Err(); err != nil {
		c.logger.Error("Error reading clangd logs: %v", err)
	}
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

// GetDocumentation gets parsed documentation for a symbol at a position
func (c *ClangdClient) GetDocumentation(uri string, position Position) (*ParsedDocumentation, error) {
	hover, err := c.GetHover(uri, position)
	if err != nil {
		return nil, err
	}
	if hover == nil || hover.Contents.Value == "" {
		return nil, nil
	}

	// Log the raw hover content for debugging
	c.logger.Debug("Raw hover content:\n%s", hover.Contents.Value)

	parsed := parseDocumentation(hover.Contents.Value)

	// Log what we parsed
	c.logger.Debug("Parsed: AccessLevel='%s', Signature='%s', ReturnType='%s'",
		parsed.AccessLevel, parsed.Signature, parsed.ReturnType)

	return parsed, nil
}
