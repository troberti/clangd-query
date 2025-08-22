package commands

import (
	"fmt"
	"strings"

	"clangd-query/internal/lsp"
)

// Generates a helpful hint message when a user searches for multiple words.
// This function provides guidance on how to properly use single-word symbol searches
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

// Returns a human-readable type prefix (like "class" or "enum") for a given symbol kind.
// This is used when displaying search results to provide context about the symbol type
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

// Removes parameter lists from a symbol name while preserving parentheses for functions.
// For example, "foo(int, char)" becomes "foo()" to simplify display
func extractBaseName(name string) string {
	if idx := strings.Index(name, "("); idx != -1 {
		return name[:idx] + "()"
	}
	return name
}

// Formats a workspace symbol with its fully qualified name including any container.
// Combines the container name and base name with "::" separator for C++ style
func formatSymbolForDisplay(symbol lsp.WorkspaceSymbol) string {
	baseName := extractBaseName(symbol.Name)
	if symbol.ContainerName != "" {
		return symbol.ContainerName + "::" + baseName
	}
	return baseName
}

// Formats a workspace symbol with both its type prefix and fully qualified name.
// For example, a class Foo in namespace Bar becomes "class Bar::Foo"
func formatSymbolWithType(symbol lsp.WorkspaceSymbol) string {
	qualifiedName := formatSymbolForDisplay(symbol)
	prefix := getSymbolTypePrefix(symbol.Kind)
	if prefix != "" {
		return prefix + " " + qualifiedName
	}
	return qualifiedName
}

// Wraps text to fit within a maximum line width while preserving paragraph breaks.
// Empty lines in the input are maintained as paragraph separators in the output.
// Words that would exceed the line width are moved to the next line
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

// Formats a complete location with file path, line, and column numbers for display.
// The URI is converted to a relative path from the project root, and line/column
// numbers are converted from 0-based to 1-based indexing for editor compatibility
func formatLocation(client *lsp.ClangdClient, location lsp.Location) string {
	// Extract path from URI
	absolutePath := client.PathFromFileURI(location.URI)

	// Make path relative
	relativePath := client.ToRelativePath(absolutePath)

	// Format with 1-based line and column numbers
	return fmt.Sprintf("%s:%d:%d", relativePath,
		location.Range.Start.Line+1,
		location.Range.Start.Character+1)
}

// Formats a file location with just the path and line number (no column).
// This simpler format is used when column information is not relevant or available.
// The URI is converted to a relative path and the line number to 1-based indexing
func formatLocationSimple(client *lsp.ClangdClient, uri string, line int) string {
	// Convert URI to path
	absolutePath := client.PathFromFileURI(uri)
	// Make path relative
	relativePath := client.ToRelativePath(absolutePath)
	return fmt.Sprintf("%s:%d", relativePath, line+1)
}

// Formats the location of a type hierarchy item for display in command output.
// This convenience function extracts the URI and line information from a TypeHierarchyItem
// and formats it as a relative path with line number (e.g., "src/foo.cpp:42").
// The function is primarily used by the hierarchy command to consistently format
// locations of base classes, derived classes, and the main class being analyzed.
func formatHierarchyItemLocation(client *lsp.ClangdClient, item lsp.TypeHierarchyItem) string {
	return formatLocationSimple(client, item.URI, item.Range.Start.Line)
}

// Converts a SymbolKind enum value to its human-readable string representation.
// Used throughout the codebase to display symbol types in command output
func SymbolKindToString(kind lsp.SymbolKind) string {
	switch kind {
	case lsp.SymbolKindFile:
		return "file"
	case lsp.SymbolKindModule:
		return "module"
	case lsp.SymbolKindNamespace:
		return "namespace"
	case lsp.SymbolKindPackage:
		return "package"
	case lsp.SymbolKindClass:
		return "class"
	case lsp.SymbolKindMethod:
		return "method"
	case lsp.SymbolKindProperty:
		return "property"
	case lsp.SymbolKindField:
		return "field"
	case lsp.SymbolKindConstructor:
		return "constructor"
	case lsp.SymbolKindEnum:
		return "enum"
	case lsp.SymbolKindInterface:
		return "interface"
	case lsp.SymbolKindFunction:
		return "function"
	case lsp.SymbolKindVariable:
		return "variable"
	case lsp.SymbolKindConstant:
		return "constant"
	case lsp.SymbolKindString:
		return "string"
	case lsp.SymbolKindNumber:
		return "number"
	case lsp.SymbolKindBoolean:
		return "boolean"
	case lsp.SymbolKindArray:
		return "array"
	case lsp.SymbolKindObject:
		return "object"
	case lsp.SymbolKindKey:
		return "key"
	case lsp.SymbolKindNull:
		return "null"
	case lsp.SymbolKindEnumMember:
		return "enum member"
	case lsp.SymbolKindStruct:
		return "struct"
	case lsp.SymbolKindEvent:
		return "event"
	case lsp.SymbolKindOperator:
		return "operator"
	case lsp.SymbolKindTypeParameter:
		return "type parameter"
	default:
		return "symbol"
	}
}
