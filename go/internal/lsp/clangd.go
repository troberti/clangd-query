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
	"strings"
	"sync"
	"time"

	"clangd-query/internal/logger"
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

// NewClangdClient creates and initializes a new clangd client
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

	client := &ClangdClient{
		cmd:           cmd,
		transport:     NewTransport(stdoutPipe, stdinPipe, os.Stderr),
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

// parseClangdLogs reads clangd's stderr and logs it with appropriate levels
func (c *ClangdClient) parseClangdLogs(stderr io.Reader) {
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()

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

// parseDocumentation parses hover content from clangd into structured documentation.
// This function extracts various pieces of information from the markdown-formatted
// hover response, including signatures, return types, parameters, modifiers, and
// documentation text. It handles various C++ constructs like templates, constructors,
// destructors, and different access levels.
func parseDocumentation(content string) *ParsedDocumentation {
	doc := &ParsedDocumentation{
		raw: content,
	}

	// Extract code block if present
	codeBlock := ""
	if idx := strings.Index(content, "```"); idx >= 0 {
		start := idx + 3
		// Skip language identifier
		if nlIdx := strings.Index(content[start:], "\n"); nlIdx >= 0 {
			start += nlIdx + 1
		}
		if endIdx := strings.Index(content[start:], "```"); endIdx >= 0 {
			codeBlock = strings.TrimSpace(content[start : start+endIdx])
		}
	}

	// Parse code block for signature and modifiers
	if codeBlock != "" {
		lines := strings.Split(codeBlock, "\n")

		// Sometimes clangd returns the signature on multiple lines
		// e.g., "public:\n  virtual void Update(...)"
		// We need to handle this case properly

		for i, line := range lines {
			line = strings.TrimSpace(line)

			// Skip context lines
			if strings.HasPrefix(line, "// In ") {
				continue
			}

			// Check for access level on its own line
			if line == "public:" || line == "private:" || line == "protected:" {
				doc.AccessLevel = strings.TrimSuffix(line, ":")
				// The next non-empty line(s) should be the signature
				// For templates, this might span multiple lines
				for j := i + 1; j < len(lines); j++ {
					nextLine := strings.TrimSpace(lines[j])
					if nextLine != "" && !strings.HasPrefix(nextLine, "// ") {
						// Check if this is a template declaration
						if strings.HasPrefix(nextLine, "template") && strings.HasSuffix(nextLine, ">") {
							// This is a template, look for the actual method signature on the next line
							for k := j + 1; k < len(lines); k++ {
								methodLine := strings.TrimSpace(lines[k])
								if methodLine != "" && !strings.HasPrefix(methodLine, "// ") {
									// Combine template and method signature
									doc.Signature = nextLine + "\n" + formatSignature(methodLine)
									// Extract modifiers and other info from the method signature part
									extractSignatureDetails(methodLine, doc)
									break
								}
							}
						} else {
							doc.Signature = formatSignature(nextLine)
							// Extract modifiers and other info from the signature
							extractSignatureDetails(nextLine, doc)
						}
						break
					}
				}
				continue
			}

			// Check if line starts with access level (e.g., "public: virtual void...")
			if strings.HasPrefix(line, "public: ") {
				doc.AccessLevel = "public"
				line = strings.TrimPrefix(line, "public: ")
			} else if strings.HasPrefix(line, "private: ") {
				doc.AccessLevel = "private"
				line = strings.TrimPrefix(line, "private: ")
			} else if strings.HasPrefix(line, "protected: ") {
				doc.AccessLevel = "protected"
				line = strings.TrimPrefix(line, "protected: ")
			}

			// This is likely the signature (if we haven't found it yet)
			if doc.Signature == "" && line != "" && !strings.HasSuffix(line, ":") {
				// Remove access level prefix if present in the signature
				signatureLine := line
				if strings.HasPrefix(line, "public: ") {
					signatureLine = strings.TrimPrefix(line, "public: ")
				} else if strings.HasPrefix(line, "private: ") {
					signatureLine = strings.TrimPrefix(line, "private: ")
				} else if strings.HasPrefix(line, "protected: ") {
					signatureLine = strings.TrimPrefix(line, "protected: ")
				}

				// Check if this is a template declaration
				if strings.HasPrefix(signatureLine, "template") && strings.HasSuffix(signatureLine, ">") {
					// This is a template, look for the actual method signature on the next line
					for j := i + 1; j < len(lines); j++ {
						methodLine := strings.TrimSpace(lines[j])
						if methodLine != "" && !strings.HasPrefix(methodLine, "// ") {
							// Combine template and method signature
							doc.Signature = signatureLine + "\n" + formatSignature(methodLine)
							// Extract modifiers and other info from the method signature part
							extractSignatureDetails(methodLine, doc)
							break
						}
					}
				} else {
					doc.Signature = formatSignature(signatureLine)
					// Extract modifiers and other info from the signature
					extractSignatureDetails(signatureLine, doc)
				}
			}
		}
	}

	// Extract documentation text from content
	// Parse content line by line to extract various pieces of information
	lines := strings.Split(content, "\n")
	var descLines []string
	inParameters := false

	for _, line := range lines {
		// Stop processing if we hit the code block
		if strings.HasPrefix(line, "```") {
			break
		}

		line = strings.TrimSpace(line)

		// Skip empty lines and separator lines
		if line == "" || line == "---" {
			continue
		}

		// Skip header lines
		if strings.HasPrefix(line, "###") || strings.HasPrefix(line, "provided by") {
			continue
		}

		// Extract Type field for variables/fields
		if strings.HasPrefix(line, "Type:") {
			typeStr := strings.TrimSpace(strings.TrimPrefix(line, "Type:"))
			doc.Type = strings.Trim(typeStr, "`")
			continue
		}

		// Skip other technical details
		if strings.HasPrefix(line, "Size:") ||
			strings.HasPrefix(line, "Offset:") ||
			strings.Contains(line, "alignment") {
			continue
		}

		// Check for return type indicator
		if strings.HasPrefix(line, "→") {
			if doc.ReturnType == "" {
				doc.ReturnType = strings.TrimSpace(strings.TrimPrefix(line, "→"))
				doc.ReturnType = strings.Trim(doc.ReturnType, "`")
			}
			continue
		}

		// Check for Parameters section
		if strings.HasPrefix(line, "Parameters:") {
			inParameters = true
			doc.ParametersText = "Parameters:"
			continue
		}

		// Handle parameter lines (they start with -)
		if inParameters && strings.HasPrefix(line, "-") {
			doc.ParametersText += "\n  " + line
			continue
		} else if inParameters && line != "" && !strings.HasPrefix(line, "-") {
			// End of parameters section
			inParameters = false
		}

		// Documentation lines (@brief, @param, etc. or just plain text)
		if strings.HasPrefix(line, "@") || (!inParameters && line != "") {
			descLines = append(descLines, line)
		}
	}

	// Join description lines
	if len(descLines) > 0 {
		doc.Description = strings.Join(descLines, " ")
	}

	return doc
}

// extractModifiers extracts C++ modifiers from a signature line
func extractModifiers(line string) []string {
	var modifiers []string

	// For const, only consider it a modifier if it appears after the closing parenthesis
	// (i.e., it's a const member function)
	if parenIdx := strings.LastIndex(line, ")"); parenIdx >= 0 {
		afterParen := line[parenIdx:]
		if strings.Contains(afterParen, " const") || strings.HasSuffix(afterParen, " const") {
			modifiers = append(modifiers, "const")
		}
	}

	// Other modifiers can appear anywhere in the signature
	// but we should be smarter about word boundaries
	modifierKeywords := []string{"virtual", "static", "override", "inline", "explicit", "noexcept"}

	// Split into words to check for exact matches
	words := strings.Fields(line)
	for _, word := range words {
		// Remove punctuation for comparison
		cleanWord := strings.Trim(word, "(),;")
		for _, mod := range modifierKeywords {
			if cleanWord == mod {
				modifiers = append(modifiers, mod)
				break
			}
		}
	}

	// Check for pure virtual
	if strings.Contains(line, "= 0") {
		modifiers = append(modifiers, "pure virtual")
	}

	// Check for deleted/defaulted
	if strings.Contains(line, "= delete") {
		modifiers = append(modifiers, "deleted")
	}
	if strings.Contains(line, "= default") {
		modifiers = append(modifiers, "defaulted")
	}

	return modifiers
}

// isModifier checks if a word is a C++ modifier
func isModifier(word string) bool {
	modifiers := []string{"virtual", "static", "override", "const", "inline", "explicit", "noexcept"}
	for _, mod := range modifiers {
		if word == mod {
			return true
		}
	}
	return false
}

// extractSignatureDetails extracts return type, modifiers, and parameters from a signature
func extractSignatureDetails(signature string, doc *ParsedDocumentation) {
	// Extract modifiers
	doc.Modifiers = extractModifiers(signature)

	// Extract return type and parameters if it's a function
	if strings.Contains(signature, "(") {
		parenIdx := strings.Index(signature, "(")
		beforeParen := signature[:parenIdx]

		// Check if this is a constructor or destructor (they don't have return types)
		// Constructors/destructors contain the class name right before the parenthesis
		// and don't have a separate return type
		parts := strings.Fields(beforeParen)
		isConstructorOrDestructor := false

		// Check for destructor (starts with ~) or constructor (class name before parenthesis)
		if len(parts) > 0 {
			lastPart := parts[len(parts)-1]
			// Check if it's a destructor
			if strings.HasPrefix(lastPart, "~") {
				isConstructorOrDestructor = true
			} else {
				// Check if it's a constructor by looking for known constructor patterns
				// Constructor names typically appear after "explicit" or as the last identifier
				for _, part := range parts {
					if part == "explicit" || part == lastPart && !isModifier(part) && !strings.Contains(part, "::") {
						// Check if the name matches a typical constructor pattern
						// (starts with uppercase letter, which is common for class names)
						if len(lastPart) > 0 && lastPart[0] >= 'A' && lastPart[0] <= 'Z' {
							isConstructorOrDestructor = true
							break
						}
					}
				}
			}
		}

		// Only extract return type if it's not a constructor/destructor
		// and if ReturnType hasn't already been set (e.g., from the → line)
		if !isConstructorOrDestructor && doc.ReturnType == "" {
			// Skip known modifiers and class qualifiers to find return type
			for _, part := range parts {
				if !isModifier(part) && !strings.Contains(part, "::") {
					// Don't set return type if it looks like a method/function name
					// (We already have the return type from the → line in most cases)
					break
				}
			}
		}

		// Extract parameters
		if closeIdx := strings.Index(signature[parenIdx:], ")"); closeIdx > 0 {
			paramStr := signature[parenIdx+1 : parenIdx+closeIdx]
			if paramStr != "" && paramStr != "void" {
				// Only set ParametersText if it hasn't been set already
				if doc.ParametersText == "" {
					params := strings.Split(paramStr, ",")
					doc.ParametersText = "Parameters:"
					for _, param := range params {
						doc.ParametersText += "\n  - `" + strings.TrimSpace(param) + "`"
					}
				}
			}
		}
	}
}

// formatSignature normalizes the formatting of a C++ signature by ensuring that
// reference (&) and pointer (*) symbols are placed next to the type rather than
// the variable or function name. This provides consistent formatting that matches
// C++ style conventions where these symbols are part of the type specification.
func formatSignature(signature string) string {
	// Handle the case where signature might be multiline (e.g., template on one line, method on next)
	if strings.Contains(signature, "\n") {
		return signature // Keep multiline signatures as-is for now
	}

	// First, normalize spaces around & and *
	// Replace patterns like "Type &" with "Type&" and "Type *" with "Type*"
	result := signature

	// Handle references - move & next to the type but keep space after for names
	result = strings.ReplaceAll(result, " &", "&")

	// Handle pointers - move * next to the type but keep space after for names
	result = strings.ReplaceAll(result, " *", "*")

	// Now we need to ensure there's a space between Type& and the next identifier
	// We'll process the signature to add spaces where needed
	finalResult := ""
	i := 0
	for i < len(result) {
		if i < len(result)-1 && (result[i] == '&' || result[i] == '*') {
			// Add the & or *
			finalResult += string(result[i])
			i++
			// Check if the next character is alphanumeric (part of an identifier)
			// If so, add a space before it
			if i < len(result) && isIdentifierChar(result[i]) {
				finalResult += " "
			}
		} else {
			finalResult += string(result[i])
			i++
		}
	}

	return finalResult
}

// isIdentifierChar checks if a character can be part of a C++ identifier
func isIdentifierChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9') || ch == '_'
}
