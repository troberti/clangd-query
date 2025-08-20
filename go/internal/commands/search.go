package commands

import (
	"fmt"
	"path/filepath"
	"strings"

	"clangd-query/internal/logger"
	"clangd-query/internal/lsp"
)

// Search performs a workspace-wide symbol search and returns formatted text output.
// The output format exactly matches the TypeScript implementation.
func Search(client *lsp.ClangdClient, query string, limit int, log logger.Logger) (string, error) {
	log.Info("Searching for symbols matching: %s (limit: %d)", query, limit)
	
	// Perform workspace symbol search
	symbols, err := client.WorkspaceSymbol(query)
	if err != nil {
		return "", err
	}
	
	log.Debug("Found %d symbols", len(symbols))

	// Handle no results
	if len(symbols) == 0 {
		// Check if query has multiple words
		if strings.Contains(query, " ") {
			return formatMultiWordQueryHint(query, "search") +
				"\nThen use interface command to see all its methods and members.", nil
		}
		return fmt.Sprintf(`No symbols found matching "%s"`, query), nil
	}

	// Build output
	output := fmt.Sprintf(`Found %d symbols matching "%s":`+"\n\n", len(symbols), query)

	count := 0
	for _, symbol := range symbols {
		// Apply limit
		if limit > 0 && count >= limit {
			break
		}

		// Build the fully qualified name with type prefix
		fullName := formatSymbolWithType(symbol)

		// Get relative path
		absolutePath := strings.TrimPrefix(symbol.Location.URI, "file://")
		// Make path relative if possible
		if relPath, err := filepath.Rel(client.ProjectRoot, absolutePath); err == nil {
			absolutePath = relPath
		}
		
		// Format location
		line := symbol.Location.Range.Start.Line + 1      // Convert to 1-based
		column := symbol.Location.Range.Start.Character + 1
		formattedLocation := fmt.Sprintf("%s:%d:%d", absolutePath, line, column)

		// Format with bullet point, backticks, and "at" prefix
		output += fmt.Sprintf("- `%s` at %s\n", fullName, formattedLocation)
		count++
	}

	// Remove trailing newline
	return strings.TrimRight(output, "\n"), nil
}


