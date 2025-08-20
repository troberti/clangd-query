package commands

import (
	"fmt"
	"strings"

	"clangd-query/internal/logger"
	"clangd-query/internal/lsp"
)

// Interface extracts the public interface of a class/struct
func Interface(client *lsp.ClangdClient, input string, log logger.Logger) (string, error) {
	// Parse input  
	uri, position, err := parseLocationOrSymbol(client, input)
	if err != nil {
		log.Error("Failed to parse input: %v", err)
		return "", err
	}

	// Get document symbols
	symbols, err := client.GetDocumentSymbols(uri)
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
	
	findSymbol(symbols)
	
	if targetSymbol == nil {
		log.Error("No class or struct found at position")
		return "", fmt.Errorf("no class or struct found at position")
	}

	// Build formatted output
	var output strings.Builder
	
	output.WriteString(fmt.Sprintf("Interface for %s:\n\n", targetSymbol.Name))
	
	publicMembersFound := false
	
	for _, child := range targetSymbol.Children {
		// Get hover information to determine access level
		hover, err := client.GetHover(uri, child.SelectionRange.Start)
		if err != nil {
			continue
		}
		
		access := extractAccessLevel(hover)
		
		// Only include public members
		if access != "public" {
			continue
		}
		
		publicMembersFound = true
		
		// Parse hover content for signature
		parsed := parseHoverContent(hover.Contents.Value)
		
		signature := parsed.DeclarationText
		if signature == "" {
			// Fallback to symbol name and detail
			signature = formatSymbolSignature(&child)
		}
		
		output.WriteString("- ")
		output.WriteString(signature)
		output.WriteString("\n")
		
		if parsed.Documentation != "" {
			// Word wrap documentation
			wrappedLines := wordWrap(parsed.Documentation, 80)
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

	return output.String(), nil
}

// extractAccessLevel extracts the access level from hover information
func extractAccessLevel(hover *lsp.Hover) string {
	if hover == nil || hover.Contents.Value == "" {
		return "unknown"
	}
	
	content := strings.ToLower(hover.Contents.Value)
	
	// Look for access specifiers in the hover content
	if strings.Contains(content, "public:") || strings.Contains(content, "public ") {
		return "public"
	}
	if strings.Contains(content, "protected:") || strings.Contains(content, "protected ") {
		return "protected"
	}
	if strings.Contains(content, "private:") || strings.Contains(content, "private ") {
		return "private"
	}
	
	// Default based on symbol type
	// In C++, struct members are public by default, class members are private
	if strings.Contains(content, "struct") {
		return "public"
	}
	
	return "private"
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