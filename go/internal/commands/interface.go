package commands

import (
	"fmt"
	"strings"

	"clangd-query/internal/logger"
	"clangd-query/internal/lsp"
)

// Interface extracts the public interface of a class/struct
func Interface(client *lsp.ClangdClient, input string, log logger.Logger) (string, error) {
	// Search for the symbol
	symbols, err := client.WorkspaceSymbol(input)
	if err != nil {
		log.Error("Failed to search for symbol: %v", err)
		return "", fmt.Errorf("failed to search for symbol: %v", err)
	}

	if len(symbols) == 0 {
		// Provide a helpful message when symbol is not found
		log.Info("No symbols found matching: %s", input)
		return fmt.Sprintf("No class or struct named '%s' found", input), nil
	}

	// Use the best match - symbols are already sorted by relevance from clangd
	symbol := symbols[0]

	// Check if it's a class or struct
	if symbol.Kind != lsp.SymbolKindClass && symbol.Kind != lsp.SymbolKindStruct {
		log.Info("Symbol '%s' is not a class or struct (it's a %s)", input, SymbolKindToString(symbol.Kind))
		return fmt.Sprintf("'%s' is not a class or struct (it's a %s)\n\nThe interface command only works with classes and structs.",
			formatSymbolForDisplay(symbol), SymbolKindToString(symbol.Kind)), nil
	}

	uri := symbol.Location.URI
	position := symbol.Location.Range.Start

	// Get document symbols
	docSymbols, err := client.GetDocumentSymbols(uri)
	if err != nil {
		log.Error("Failed to get document symbols: %v", err)
		return "", err
	}

	// Find the class/struct at the position
	var targetSymbol *lsp.DocumentSymbol
	var findSymbol func([]lsp.DocumentSymbol)

	findSymbol = func(syms []lsp.DocumentSymbol) {
		for i := range syms {
			s := &syms[i]
			if s.Range.Start.Line <= position.Line && position.Line <= s.Range.End.Line {
				// Check if it's a class or struct
				if s.Kind == lsp.SymbolKindClass || s.Kind == lsp.SymbolKindStruct {
					targetSymbol = s
					return // Found the most specific match
				}
				// Check children
				if len(s.Children) > 0 {
					findSymbol(s.Children)
				}
			}
		}
	}

	findSymbol(docSymbols)

	if targetSymbol == nil {
		log.Error("No class or struct found at position")
		return "", fmt.Errorf("no class or struct found at position")
	}

	// Build formatted output
	var output strings.Builder

	// Format class name with location - use selection range start for more precise location
	location := formatLocation(client, lsp.Location{
		URI:   uri,
		Range: lsp.Range{
			Start: targetSymbol.SelectionRange.Start,
			End:   targetSymbol.SelectionRange.Start,
		},
	})

	// Include container name if present to get full qualified name
	fullName := targetSymbol.Name
	if targetSymbol.Detail != "" && strings.Contains(targetSymbol.Detail, "::") {
		// Extract namespace from detail if available
		if idx := strings.Index(targetSymbol.Detail, targetSymbol.Name); idx > 0 {
			prefix := strings.TrimSpace(targetSymbol.Detail[:idx])
			if strings.HasSuffix(prefix, "::") {
				fullName = prefix + targetSymbol.Name
			}
		}
	}

	// Try to get full name from workspace symbol search for better accuracy
	wsSymbols, _ := client.WorkspaceSymbol(targetSymbol.Name)
	for _, sym := range wsSymbols {
		if sym.Name == targetSymbol.Name && sym.Location.URI == uri {
			fullName = formatSymbolForDisplay(sym)
			break
		}
	}

	// Use the correct keyword (class or struct)
	symbolTypeKeyword := "class"
	if targetSymbol.Kind == lsp.SymbolKindStruct {
		symbolTypeKeyword = "struct"
	}
	output.WriteString(fmt.Sprintf("%s %s - %s\n\n", symbolTypeKeyword, fullName, location))
	output.WriteString("Public Interface:\n\n")

	publicMembersFound := false

	for _, child := range targetSymbol.Children {
		// Get parsed documentation to determine access level and signature
		doc, err := client.GetDocumentation(uri, child.SelectionRange.Start)
		if err != nil || doc == nil {
			continue
		}

		// Only include public members
		if doc.AccessLevel != "public" {
			continue
		}

		publicMembersFound = true

		signature := doc.Signature
		if signature == "" {
			// Fallback to symbol name and detail
			signature = formatSymbolSignature(&child)
		}
		
		// Prepend static if it's a static method
		for _, modifier := range doc.Modifiers {
			if modifier == "static" {
				// Check if signature already contains static
				if !strings.Contains(signature, "static ") {
					signature = "static " + signature
				}
				break
			}
		}

		output.WriteString(signature)
		output.WriteString("\n")

		if doc.Description != "" {
			// Word wrap documentation with 2-space indent
			wrappedLines := wordWrap(doc.Description, 78) // 78 to account for 2-space indent
			for _, line := range wrappedLines {
				if strings.TrimSpace(line) != "" {
					output.WriteString("  ")
					output.WriteString(line)
					output.WriteString("\n")
				}
			}
		}
		output.WriteString("\n")
	}

	if !publicMembersFound {
		output.WriteString("No public members found.")
	}

	// Trim trailing whitespace
	result := strings.TrimRight(output.String(), "\n")
	return result, nil
}

// formatSymbolSignature formats a symbol as a signature string
func formatSymbolSignature(symbol *lsp.DocumentSymbol) string {
	signature := symbol.Name

	// Add detail if available
	if symbol.Detail != "" {
		// For methods, detail often contains the full signature
		if strings.Contains(symbol.Detail, "(") {
			signature = symbol.Detail
		} else {
			// For fields, combine name and type
			signature = symbol.Detail + " " + symbol.Name
		}
	}

	// Add symbol kind prefix if not already present
	kind := symbol.Kind.String()
	if !strings.Contains(signature, kind) {
		switch symbol.Kind {
		case lsp.SymbolKindMethod:
			// Methods already have signatures
		case lsp.SymbolKindField:
			// Fields are fine as is
		case lsp.SymbolKindConstructor:
			signature = "constructor " + signature
		default:
			signature = kind + " " + signature
		}
	}

	return signature
}