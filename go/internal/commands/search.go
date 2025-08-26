package commands

import (
	"fmt"
	"strings"

	"clangd-query/internal/clangd"
	"clangd-query/internal/logger"
)

// Performs a workspace-wide symbol search and returns formatted text output.
func Search(client *clangd.ClangdClient, query string, limit int, log logger.Logger) (string, error) {
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

	// Apply limit
	if limit > 0 && len(symbols) > limit {
		symbols = symbols[:limit]
	}

	// Build output
	output := fmt.Sprintf(`Found %d symbols matching "%s":`+"\n\n", len(symbols), query)
	for _, symbol := range symbols {
		// Format with bullet point, backticks, "at" prefix, and kind at the end
		output += fmt.Sprintf(
			"- `%s` at %s [%s]\n",
			formatSymbolForDisplay(symbol),
			formatLocation(client, symbol.Location),
			SymbolKindToString(symbol.Kind))
	}
	// Remove trailing newline
	return strings.TrimRight(output, "\n"), nil
}
