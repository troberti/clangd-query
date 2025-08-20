package commands

import (
	"fmt"
	"strings"

	"clangd-query/internal/lsp"
)

// formatMultiWordQueryHint generates a helpful hint message for multi-word queries
func formatMultiWordQueryHint(query string, commandName string) string {
	words := strings.Fields(query)
	firstWord := words[0]
	lastWord := words[len(words)-1]

	hint := fmt.Sprintf(`No symbols found matching "%s"`+"\n\n", query)
	hint += fmt.Sprintf("💡 Hint: %s only searches for single symbol names. ", commandName)
	hint += "Try searching for just the class or method name:\n"
	hint += fmt.Sprintf(`- %s "%s"`+"\n", commandName, firstWord)
	
	if lastWord != firstWord {
		hint += "Or if looking for a specific method, try just the method name:\n"
		hint += fmt.Sprintf(`- %s "%s"`, commandName, lastWord)
	}
	
	return hint
}

// getSymbolTypePrefix returns a human-readable type prefix for a symbol
func getSymbolTypePrefix(kind lsp.SymbolKind) string {
	switch kind {
	case lsp.SymbolKindClass:
		return "class"
	case lsp.SymbolKindMethod, lsp.SymbolKindFunction, lsp.SymbolKindConstructor:
		return "" // Methods/functions show just the name
	case lsp.SymbolKindEnum:
		return "enum"
	case lsp.SymbolKindInterface:
		return "interface"
	case lsp.SymbolKindStruct:
		return "struct"
	case lsp.SymbolKindNamespace:
		return "namespace"
	case lsp.SymbolKindField, lsp.SymbolKindProperty, lsp.SymbolKindVariable:
		return ""
	case lsp.SymbolKindTypeParameter:
		return "template"
	default:
		return ""
	}
}

// extractBaseName removes parameters from a symbol name but keeps parentheses for functions
func extractBaseName(name string) string {
	if idx := strings.Index(name, "("); idx != -1 {
		return name[:idx] + "()"
	}
	return name
}

// formatSymbolForDisplay formats a symbol with its qualified name
func formatSymbolForDisplay(symbol lsp.WorkspaceSymbol) string {
	baseName := extractBaseName(symbol.Name)
	if symbol.ContainerName != "" {
		return symbol.ContainerName + "::" + baseName
	}
	return baseName
}

// formatSymbolWithType formats a symbol with type prefix and qualified name
func formatSymbolWithType(symbol lsp.WorkspaceSymbol) string {
	qualifiedName := formatSymbolForDisplay(symbol)
	prefix := getSymbolTypePrefix(symbol.Kind)
	if prefix != "" {
		return prefix + " " + qualifiedName
	}
	return qualifiedName
}

// wordWrap wraps text to fit within a maximum line width
// Preserves paragraph breaks (empty lines)
func wordWrap(text string, maxWidth int) []string {
	var lines []string
	paragraphs := strings.Split(text, "\n")

	for _, paragraph := range paragraphs {
		if strings.TrimSpace(paragraph) == "" {
			lines = append(lines, "")
			continue
		}

		words := strings.Fields(strings.TrimSpace(paragraph))
		var currentLine string

		for _, word := range words {
			var lineWithWord string
			if currentLine != "" {
				lineWithWord = currentLine + " " + word
			} else {
				lineWithWord = word
			}

			if len(lineWithWord) > maxWidth && currentLine != "" {
				// Current line would be too long, start a new line
				lines = append(lines, currentLine)
				currentLine = word
			} else {
				// Add word to current line
				currentLine = lineWithWord
			}
		}

		if currentLine != "" {
			lines = append(lines, currentLine)
		}
	}

	return lines
}