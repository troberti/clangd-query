package commands

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"clangd-query/internal/logger"
	"clangd-query/internal/lsp"
)

// Usages finds all references to a symbol and returns them as formatted text
// Two input modes: symbol names OR file:line:column locations
func Usages(client *lsp.ClangdClient, input string, limit int, log logger.Logger) (string, error) {
	// Check if input is a location string (file:line:column)
	locationMatch := regexp.MustCompile(`^(.+):(\d+):(\d+)$`)
	matches := locationMatch.FindStringSubmatch(input)
	
	if matches != nil {
		// Location mode: parse the location string
		file := matches[1]
		line, err1 := strconv.Atoi(matches[2])
		column, err2 := strconv.Atoi(matches[3])
		
		if err1 != nil || err2 != nil || line < 1 || column < 1 {
			return "", fmt.Errorf(`Invalid location format: "%s"`+"\n"+
				`Expected format: "path/to/file.cpp:line:column" (with line and column as numbers)`+"\n"+
				`Examples:`+"\n"+
				`  - "src/main.cpp:42:15"`+"\n"+
				`  - "include/widget.h:100:8"`+"\n"+
				`Note: Line and column numbers should be 1-indexed (as shown in editors)`, input)
		}
		
		return findReferencesAtLocation(client, file, line, column, input, log)
	} else {
		// Symbol mode: search for symbol first, then find references
		return findReferencesToSymbol(client, input, log)
	}
}

// findReferencesAtLocation finds references at a specific location
func findReferencesAtLocation(client *lsp.ClangdClient, file string, line, column int, originalLocation string, log logger.Logger) (string, error) {
	log.Info("Finding references at location: %s", originalLocation)
	
	// Convert to absolute path if needed
	absolutePath := client.ToAbsolutePath(file)
	
	uri := client.FileURIFromPath(absolutePath)
	position := lsp.Position{
		Line:      line - 1, // Convert to 0-based
		Character: column - 1,
	}
	
	// Find all references (including declaration)
	references, err := client.GetReferences(uri, position, true)
	if err != nil {
		return "", err
	}
	
	if len(references) == 0 {
		return fmt.Sprintf("No references found for symbol at %s", originalLocation), nil
	}
	
	// Build output
	output := fmt.Sprintf("Found %d reference", len(references))
	if len(references) != 1 {
		output += "s"
	}
	output += fmt.Sprintf(" to symbol at %s:\n\n", originalLocation)
	
	// Convert references to human-readable format
	for _, ref := range references {
		formattedLocation := formatLocation(client, ref)
		output += fmt.Sprintf("- %s\n", formattedLocation)
	}
	
	log.Debug("Found %d references", len(references))
	// Remove trailing newline
	return strings.TrimRight(output, "\n"), nil
}

// findReferencesToSymbol finds references to a symbol by searching for it first
func findReferencesToSymbol(client *lsp.ClangdClient, symbolName string, log logger.Logger) (string, error) {
	log.Info("Finding references to symbol: %s", symbolName)
	
	// First, search for the symbol
	symbols, err := client.WorkspaceSymbol(symbolName)
	if err != nil {
		return "", err
	}
	
	if len(symbols) == 0 {
		// Check if query has multiple words
		if strings.Contains(symbolName, " ") {
			return formatMultiWordQueryHint(symbolName, "usages"), nil
		}
		return fmt.Sprintf(`No symbols found matching "%s"`, symbolName), nil
	}
	
	// Use the best match - symbols are already sorted by relevance from clangd
	symbol := symbols[0]
	
	// Build the full symbol name for display
	fullName := formatSymbolForDisplay(symbol)
	
	// Find references to this symbol
	references, err := client.GetReferences(symbol.Location.URI, symbol.Location.Range.Start, true)
	if err != nil {
		return "", err
	}
	
	if len(references) == 0 {
		return fmt.Sprintf("Selected symbol: %s\nNo references found for this symbol", fullName), nil
	}
	
	// Build output
	output := fmt.Sprintf("Selected symbol: %s\n", fullName)
	output += fmt.Sprintf("Found %d reference", len(references))
	if len(references) != 1 {
		output += "s"
	}
	output += ":\n\n"
	
	// Convert references to human-readable format
	for _, ref := range references {
		formattedLocation := formatLocation(client, ref)
		output += fmt.Sprintf("- %s\n", formattedLocation)
	}
	
	log.Debug("Found %d references", len(references))
	// Remove trailing newline
	return strings.TrimRight(output, "\n"), nil
}
