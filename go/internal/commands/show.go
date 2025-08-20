package commands

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/firi/clangd-query/internal/lsp"
)

// Show displays both declaration and definition of a symbol
func Show(client *lsp.ClangdClient, input string) ([]ShowResult, error) {
	// Parse input - can be either symbol name or file:line:column
	uri, position, err := parseLocationOrSymbol(client, input)
	if err != nil {
		return nil, err
	}

	// Get definition locations
	definitions, err := client.GetDefinition(uri, position)
	if err != nil {
		return nil, err
	}

	// Also get declaration locations
	declarations, err := client.GetDeclaration(uri, position)
	if err != nil {
		// Some symbols might not have separate declarations
		declarations = []lsp.Location{}
	}

	// Combine and deduplicate locations
	locationMap := make(map[string]lsp.Location)
	for _, loc := range definitions {
		key := fmt.Sprintf("%s:%d:%d", loc.URI, loc.Range.Start.Line, loc.Range.Start.Character)
		locationMap[key] = loc
	}
	for _, loc := range declarations {
		key := fmt.Sprintf("%s:%d:%d", loc.URI, loc.Range.Start.Line, loc.Range.Start.Character)
		locationMap[key] = loc
	}

	// Convert to results
	results := make([]ShowResult, 0)
	
	// Process each unique location
	for _, loc := range locationMap {
		file := strings.TrimPrefix(loc.URI, "file://")
		
		// Read the content
		startLine := loc.Range.Start.Line
		endLine := loc.Range.End.Line
		
		lines, err := lsp.ReadFileLines(file, startLine, endLine)
		if err != nil {
			continue
		}
		
		content := strings.Join(lines, "\n")
		
		// Determine if this is declaration or definition
		// Heuristic: header files usually contain declarations
		isHeader := strings.HasSuffix(file, ".h") || strings.HasSuffix(file, ".hpp") || 
		           strings.HasSuffix(file, ".hxx") || strings.HasSuffix(file, ".h++")
		
		// Check if content has a body (contains '{')
		hasBody := strings.Contains(content, "{")
		
		locType := "declaration"
		if hasBody && !isHeader {
			locType = "definition"
		}
		
		// Make path relative
		if relPath, err := filepath.Rel(client.ProjectRoot, file); err == nil {
			file = relPath
		}
		
		result := ShowResult{
			File:    file,
			Line:    startLine + 1, // Convert to 1-based
			Column:  loc.Range.Start.Character + 1,
			Content: content,
			Type:    locType,
		}
		results = append(results, result)
	}
	
	// Sort: declarations first, then definitions
	sortResults := func(results []ShowResult) []ShowResult {
		decls := make([]ShowResult, 0)
		defs := make([]ShowResult, 0)
		
		for _, r := range results {
			if r.Type == "declaration" {
				decls = append(decls, r)
			} else {
				defs = append(defs, r)
			}
		}
		
		return append(decls, defs...)
	}
	
	return sortResults(results), nil
}

// parseLocationOrSymbol parses input as either file:line:column or symbol name
func parseLocationOrSymbol(client *lsp.ClangdClient, input string) (string, lsp.Position, error) {
	// Try to parse as file:line:column
	parts := strings.Split(input, ":")
	if len(parts) >= 3 {
		// Might be a location
		file := parts[0]
		
		// Handle absolute paths that contain ':'
		if len(parts) > 3 {
			file = strings.Join(parts[:len(parts)-2], ":")
		}
		
		line, err1 := strconv.Atoi(parts[len(parts)-2])
		col, err2 := strconv.Atoi(parts[len(parts)-1])
		
		if err1 == nil && err2 == nil {
			// It's a location
			if !filepath.IsAbs(file) {
				file = filepath.Join(client.ProjectRoot, file)
			}
			
			uri := "file://" + file
			position := lsp.Position{
				Line:      line - 1, // Convert to 0-based
				Character: col - 1,
			}
			return uri, position, nil
		}
	}
	
	// Not a location, treat as symbol name
	// Search for the symbol
	symbols, err := client.WorkspaceSymbol(input)
	if err != nil {
		return "", lsp.Position{}, err
	}
	
	if len(symbols) == 0 {
		return "", lsp.Position{}, fmt.Errorf("symbol not found: %s", input)
	}
	
	// Use the first match
	symbol := symbols[0]
	return symbol.Location.URI, symbol.Location.Range.Start, nil
}