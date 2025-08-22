package commands

import (
	"fmt"
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

	log.Debug("Found %d total symbols", len(symbols))

	// Handle no results
	if len(symbols) == 0 {
		// Check if query has multiple words
		if strings.Contains(query, " ") {
			return formatMultiWordQueryHint(query, "search") +
				"\nThen use interface command to see all its methods and members.", nil
		}
		return fmt.Sprintf(`No symbols found matching "%s"`, query), nil
	}

	// Apply limit to match TypeScript behavior
	if limit > 0 && len(symbols) > limit {
		symbols = symbols[:limit]
	}

	// Build output - report actual number of symbols we'll show
	output := fmt.Sprintf(`Found %d symbols matching "%s":`+"\n\n", len(symbols), query)

	for _, symbol := range symbols {

		// Build the fully qualified name with type prefix
		fullName := formatSymbolWithType(symbol)

		// Get absolute path from URI
		absolutePath := client.PathFromFileURI(symbol.Location.URI)
		// Convert to relative path for display
		relativePath := client.ToRelativePath(absolutePath)

		// Format location
		line := symbol.Location.Range.Start.Line + 1 // Convert to 1-based
		column := symbol.Location.Range.Start.Character + 1
		formattedLocation := fmt.Sprintf("%s:%d:%d", relativePath, line, column)

		// Format with bullet point, backticks, and "at" prefix
		output += fmt.Sprintf("- `%s` at %s\n", fullName, formattedLocation)
	}

	// Remove trailing newline
	return strings.TrimRight(output, "\n"), nil
}
