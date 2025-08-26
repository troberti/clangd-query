package commands

import (
	"fmt"
	"os"
	"strings"

	"clangd-query/internal/clangd"
	"clangd-query/internal/logger"
)

// View extracts the complete source code of a symbol
// This is a semantic viewer that understands C++ structure and returns complete implementations
func View(client *clangd.ClangdClient, query string, log logger.Logger) (string, error) {
	log.Info("Viewing source code for: %s", query)

	// Search for the symbol
	symbols, err := client.WorkspaceSymbol(query)
	if err != nil {
		return "", err
	}

	if len(symbols) == 0 {
		// Check if query has multiple words
		if strings.Contains(query, " ") {
			return formatMultiWordQueryHint(query, "view"), nil
		}
		return fmt.Sprintf(`No symbols found matching "%s"`, query), nil
	}

	// Use the best match - symbols are already sorted by relevance from clangd
	symbol := symbols[0]

	// Get the file path
	filePath := client.PathFromFileURI(symbol.Location.URI)

	// Find the symbol line first
	symbolLine := symbol.Location.Range.Start.Line

	// Use folding ranges to get the full extent of symbols
	foldingRanges, err := client.GetFoldingRanges(symbol.Location.URI)
	if err != nil {
		log.Debug("Failed to get folding ranges: %v", err)
		foldingRanges = []clangd.FoldingRange{}
	}

	log.Debug("Got %d folding ranges", len(foldingRanges))

	// Read the file content
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read file: %v", err)
	}
	lines := strings.Split(string(content), "\n")

	var startLine, endLine int
	var foundRange *clangd.FoldingRange

	// For classes/structs/enums/functions/methods, the folding range often starts
	// at or after the declaration line (where the opening brace is).
	if symbol.Kind == clangd.SymbolKindClass ||
		symbol.Kind == clangd.SymbolKindStruct ||
		symbol.Kind == clangd.SymbolKindEnum ||
		symbol.Kind == clangd.SymbolKindInterface ||
		symbol.Kind == clangd.SymbolKindFunction ||
		symbol.Kind == clangd.SymbolKindMethod {

		// Look for folding ranges near the symbol
		var rangeAtSymbol *clangd.FoldingRange
		var rangeAfterSymbol *clangd.FoldingRange

		// First, find a range at the symbol line
		for i := range foldingRanges {
			r := &foldingRanges[i]
			if r.StartLine == symbolLine {
				rangeAtSymbol = r
				break
			}
		}

		// Handle the "consecutive folding ranges" pattern for functions with 3+ parameters
		if rangeAtSymbol != nil {
			for i := range foldingRanges {
				r := &foldingRanges[i]
				// Look for a range that starts at or within 1 line of where the first ends
				if r.StartLine >= rangeAtSymbol.EndLine &&
					r.StartLine <= rangeAtSymbol.EndLine+1 {
					rangeAfterSymbol = r
					break
				}
			}
		}

		// If no range at symbol, find the closest one after
		if rangeAtSymbol == nil {
			bestDistance := 1000000
			for i := range foldingRanges {
				r := &foldingRanges[i]
				if r.StartLine > symbolLine {
					distance := r.StartLine - symbolLine
					if distance < bestDistance {
						bestDistance = distance
						rangeAfterSymbol = r
					}
				}
			}
		}

		// Decision logic: If we have consecutive ranges (parameter list + body), use the second one (the body)
		if rangeAtSymbol != nil && rangeAfterSymbol != nil {
			// We have consecutive ranges - the first is likely a parameter list
			foundRange = rangeAfterSymbol
			log.Debug("Found consecutive folding ranges - using the second one as function body")
		} else if rangeAtSymbol != nil {
			foundRange = rangeAtSymbol
		} else {
			foundRange = rangeAfterSymbol
		}
	}

	// If we didn't find a range yet (for other symbol types), use the original logic
	if foundRange == nil {
		for i := range foldingRanges {
			r := &foldingRanges[i]
			if r.StartLine <= symbolLine && symbolLine <= r.EndLine {
				// This range contains our symbol
				if foundRange == nil ||
					(r.EndLine-r.StartLine) < (foundRange.EndLine-foundRange.StartLine) {
					foundRange = r
				}
			}
		}
	}

	if foundRange != nil {
		// For classes/structs/enums/functions/methods, we want to include the
		// declaration line(s) too, not just the body
		if symbol.Kind == clangd.SymbolKindClass ||
			symbol.Kind == clangd.SymbolKindStruct ||
			symbol.Kind == clangd.SymbolKindEnum ||
			symbol.Kind == clangd.SymbolKindInterface ||
			symbol.Kind == clangd.SymbolKindFunction ||
			symbol.Kind == clangd.SymbolKindMethod {
			startLine = symbolLine       // Start from the declaration
			endLine = foundRange.EndLine // End at the closing brace
			log.Debug("Using adjusted range for %s: %d-%d (symbol at %d, fold at %d-%d)",
				SymbolKindToString(symbol.Kind), startLine, endLine, symbolLine, foundRange.StartLine, foundRange.EndLine)
		} else {
			startLine = foundRange.StartLine
			endLine = foundRange.EndLine
		}
	} else {
		// Fallback: try document symbols (works better for headers)
		docSymbols, err := client.GetDocumentSymbols(symbol.Location.URI)
		if err == nil && len(docSymbols) > 0 {
			// Try to find the matching symbol
			targetLine := symbol.Location.Range.Start.Line

			matchingSymbol := findSymbolAtPosition(docSymbols, targetLine, symbol.Name)

			if matchingSymbol != nil {
				startLine = matchingSymbol.Range.Start.Line
				endLine = matchingSymbol.Range.End.Line
			} else {
				// Final fallback to search result range
				startLine = symbol.Location.Range.Start.Line
				endLine = symbol.Location.Range.End.Line
			}
		} else {
			// Final fallback to search result range
			startLine = symbol.Location.Range.Start.Line
			endLine = symbol.Location.Range.End.Line
		}
	}

	// For classes/structs/enums, check for preceding comment blocks
	commentStartLine := startLine
	if symbol.Kind == clangd.SymbolKindClass ||
		symbol.Kind == clangd.SymbolKindStruct ||
		symbol.Kind == clangd.SymbolKindEnum ||
		symbol.Kind == clangd.SymbolKindInterface {
		// Look backwards from the symbol line to find comment blocks
		inCommentBlock := false
		for i := startLine - 1; i >= 0 && i >= startLine-50; i-- {
			if i >= len(lines) {
				continue
			}
			line := strings.TrimSpace(lines[i])

			// Check if this line is part of a comment
			if strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*") ||
				strings.HasPrefix(line, "*") || line == "*/" {
				commentStartLine = i
				inCommentBlock = true
			} else if line == "" {
				// Empty line - only continue if we're already in a comment block
				if !inCommentBlock {
					break
				}
			} else {
				// Non-comment, non-empty line - stop looking
				break
			}
		}
	}

	// Ensure bounds are valid
	commentStartLine = max(0, commentStartLine)
	endLine = min(endLine, len(lines)-1)

	var codeLines []string
	if commentStartLine <= endLine {
		codeLines = lines[commentStartLine : endLine+1]
	}

	// Build the symbol description
	symbolKindName := SymbolKindToString(symbol.Kind)
	fullName := formatSymbolForDisplay(symbol)

	symbolDescription := fmt.Sprintf("%s '%s'", symbolKindName, fullName)

	// Format the location
	formattedLocation := formatLocationSimple(client, filePath, symbol.Location.Range.Start.Line)

	// Check if we found multiple matches
	var multipleMatchesNote string
	if len(symbols) > 1 {
		multipleMatchesNote = fmt.Sprintf("\n(Found %d matches, showing the most relevant one)", len(symbols))
	}

	// Build the result
	result := fmt.Sprintf("Found %s at %s%s\n\n```cpp\n%s\n```",
		symbolDescription, formattedLocation, multipleMatchesNote, strings.Join(codeLines, "\n"))

	log.Debug("Retrieved %d lines of source code", len(codeLines))
	return result, nil
}

// findSymbolAtPosition recursively finds a document symbol at a specific position
func findSymbolAtPosition(symbols []clangd.DocumentSymbol, targetLine int, name string) *clangd.DocumentSymbol {
	for i := range symbols {
		s := &symbols[i]
		// Check if this symbol's range contains our target position
		if s.Range.Start.Line <= targetLine && targetLine <= s.Range.End.Line {
			// Check if the name matches
			if s.Name == name {
				return s
			}
		}

		// Check children recursively
		if len(s.Children) > 0 {
			if childResult := findSymbolAtPosition(s.Children, targetLine, name); childResult != nil {
				return childResult
			}
		}
	}
	return nil
}
