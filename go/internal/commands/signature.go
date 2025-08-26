package commands

import (
	"fmt"
	"strings"

	"clangd-query/internal/clangd"
	"clangd-query/internal/logger"
)

// Signature shows function signatures with documentation
func Signature(client *clangd.ClangdClient, functionName string, log logger.Logger) (string, error) {
	log.Info("Searching for function/method '%s' to get signatures", functionName)

	// Search for symbols - clangd's fuzzy search handles "View::SetSize" etc
	symbols, err := client.WorkspaceSymbol(functionName)
	if err != nil {
		return "", err
	}

	log.Debug("Found %d total symbols", len(symbols))

	// Filter to only functions and methods
	var functionSymbols []clangd.WorkspaceSymbol
	for _, sym := range symbols {
		if sym.Kind == clangd.SymbolKindFunction ||
			sym.Kind == clangd.SymbolKindMethod ||
			sym.Kind == clangd.SymbolKindConstructor {
			functionSymbols = append(functionSymbols, sym)
		}
	}

	log.Debug("Filtered to %d function/method symbols", len(functionSymbols))

	if len(functionSymbols) == 0 {
		return fmt.Sprintf("No function or method named '%s' found in the codebase.", functionName), nil
	}

	// Limit to top 3 matches to avoid overwhelming output
	maxResults := 3
	if len(functionSymbols) < maxResults {
		maxResults = len(functionSymbols)
	}
	symbolsToShow := functionSymbols[:maxResults]

	// Get documentation for each match
	var results []string

	for _, symbol := range symbolsToShow {
		// Get parsed documentation for the symbol
		doc, err := client.GetDocumentation(symbol.Location.URI, symbol.Location.Range.Start)

		if err != nil {
			log.Error("Failed to get documentation for %s: %v", functionName, err)
			location := formatLocation(client, symbol.Location)
			results = append(results, fmt.Sprintf("%s - %s\n  Error getting documentation: %v",
				functionName, location, err))
			continue
		}

		if doc != nil {
			formatted := formatSignature(client, symbol, doc)
			results = append(results, formatted)
		} else {
			// Fallback if no documentation available
			location := formatLocation(client, symbol.Location)
			results = append(results, fmt.Sprintf("%s - %s\n  No documentation available",
				symbol.Name, location))
		}
	}

	// Join all results with separators
	separator := strings.Repeat("─", 80)
	output := strings.Join(results, "\n\n"+separator+"\n\n")

	// Add note about additional matches if there are more
	remainingCount := len(functionSymbols) - maxResults
	if remainingCount > 0 {
		output += "\n\n" + separator + "\n\n"
		plural := "s"
		if remainingCount == 1 {
			plural = ""
		}
		output += fmt.Sprintf("... and %d more signature%s not shown. Use 'search %s' to see all matches.",
			remainingCount, plural, functionName)
	}

	return output, nil
}

// formatSignature formats a single function signature with its documentation
func formatSignature(client *clangd.ClangdClient, symbol clangd.WorkspaceSymbol, doc *clangd.ParsedDocumentation) string {
	var lines []string

	// Location header
	location := formatLocation(client, symbol.Location)

	// Main signature line
	lines = append(lines, fmt.Sprintf("%s - %s", symbol.Name, location))
	lines = append(lines, "")

	// Access level and signature
	if doc.AccessLevel != "" {
		lines = append(lines, doc.AccessLevel+":")
	}

	if doc.Signature != "" {
		lines = append(lines, "  "+doc.Signature)
	} else {
		lines = append(lines, "  [Signature not available]")
	}

	lines = append(lines, "")

	// Return type
	if doc.ReturnType != "" {
		lines = append(lines, "Return Type: "+doc.ReturnType)
		lines = append(lines, "")
	}

	// Parameters
	if doc.ParametersText != "" {
		lines = append(lines, doc.ParametersText)
		lines = append(lines, "")
	}

	// Description/documentation
	if doc.Description != "" {
		lines = append(lines, "Description:")
		// Word wrap the description for readability
		wrapped := wordWrap(doc.Description, 80)
		for _, line := range wrapped {
			lines = append(lines, "  "+line)
		}
		lines = append(lines, "")
	}

	// Template information
	if doc.TemplateParams != "" {
		lines = append(lines, "Template Parameters: "+doc.TemplateParams)
		lines = append(lines, "")
	}

	// Additional modifiers
	if len(doc.Modifiers) > 0 {
		lines = append(lines, "Modifiers: "+strings.Join(doc.Modifiers, ", "))
	}

	return strings.Join(lines, "\n")
}
