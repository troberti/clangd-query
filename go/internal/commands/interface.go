package commands

import (
	"fmt"
	"strings"

	"clangd-query/internal/logger"
	"clangd-query/internal/lsp"
)

// Interface extracts the public interface of a class/struct
func Interface(client *lsp.ClangdClient, input string, log logger.Logger) (*InterfaceResult, error) {
	// Parse input  
	uri, position, err := parseLocationOrSymbol(client, input)
	if err != nil {
		return nil, err
	}

	// Get document symbols
	symbols, err := client.GetDocumentSymbols(uri)
	if err != nil {
		return nil, err
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
		return nil, fmt.Errorf("no class or struct found at position")
	}

	// Extract public members
	members := make([]InterfaceMember, 0)
	
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
		
		// Parse hover content for signature
		parsed := parseHoverContent(hover.Contents.Value)
		
		signature := parsed.DeclarationText
		if signature == "" {
			// Fallback to symbol name and detail
			signature = formatSymbolSignature(&child)
		}
		
		member := InterfaceMember{
			Signature:     signature,
			Documentation: parsed.Documentation,
			Access:        access,
		}
		
		members = append(members, member)
	}

	return &InterfaceResult{
		Name:    targetSymbol.Name,
		Members: members,
	}, nil
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