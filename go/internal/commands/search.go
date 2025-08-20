package commands

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/firi/clangd-query/internal/lsp"
)

// Search performs a workspace-wide symbol search
func Search(client *lsp.ClangdClient, query string, limit int) ([]SearchResult, error) {
	// Check for common regex patterns that users mistakenly use
	if strings.Contains(query, " ") {
		fmt.Fprintf(os.Stderr, "Warning: Query contains spaces. clangd uses fuzzy matching, not regex.\n")
		fmt.Fprintf(os.Stderr, "Did you mean to search for multiple words? Try searching for each word separately.\n")
	}

	// Perform workspace symbol search
	symbols, err := client.WorkspaceSymbol(query)
	if err != nil {
		return nil, err
	}

	// Convert to our result format
	results := make([]SearchResult, 0, len(symbols))
	for _, symbol := range symbols {
		// Extract file path from URI
		file := strings.TrimPrefix(symbol.Location.URI, "file://")
		// Make path relative if possible
		if relPath, err := filepath.Rel(client.ProjectRoot, file); err == nil {
			file = relPath
		}

		result := SearchResult{
			Kind:   symbol.Kind.String(),
			File:   file,
			Line:   symbol.Location.Range.Start.Line + 1, // Convert to 1-based
			Column: symbol.Location.Range.Start.Character + 1,
			Name:   formatSymbolName(symbol),
		}
		results = append(results, result)

		// Apply limit
		if limit > 0 && len(results) >= limit {
			break
		}
	}

	return results, nil
}

// formatSymbolName formats a symbol name with its container
func formatSymbolName(symbol lsp.WorkspaceSymbol) string {
	if symbol.ContainerName != "" {
		return symbol.ContainerName + "::" + symbol.Name
	}
	return symbol.Name
}

