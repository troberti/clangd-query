package commands

import (
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"

	"clangd-query/internal/logger"
	"clangd-query/internal/lsp"
)

// Show displays both declaration and definition of a symbol with contextual code
// This command intelligently handles C++ declaration/definition split.
func Show(client *lsp.ClangdClient, query string, log logger.Logger) (string, error) {
	log.Info("Getting context for: %s", query)

	// Search for the symbol with a higher limit to ensure we find it
	symbols, err := client.WorkspaceSymbol(query)
	if err != nil {
		return "", err
	}

	if len(symbols) == 0 {
		return fmt.Sprintf(`No symbols found matching "%s"`, query), nil
	}

	// Use the best match - symbols are already sorted by relevance from clangd
	symbol := symbols[0]

	symbolKindName := SymbolKindToString(symbol.Kind)
	fullName := formatSymbolForDisplay(symbol)

	// Get the symbol's location
	symbolPath := client.PathFromFileURI(symbol.Location.URI)
	
	log.Debug("Found %s '%s' at %s", symbolKindName, fullName, symbolPath)

	// For methods and functions, try to find both declaration and definition
	type locationInfo struct {
		locType      string
		location     lsp.Location
		path         string
		isDefinition bool
	}

	var locations []locationInfo

	if symbol.Kind == lsp.SymbolKindFunction || 
	   symbol.Kind == lsp.SymbolKindMethod || 
	   symbol.Kind == lsp.SymbolKindConstructor {

		// Determine if the symbol location is a definition or declaration
		symbolHasBody, _ := hasBody(symbolPath, symbol.Location.Range.Start.Line)
		symbolIsInSourceFile := regexp.MustCompile(`\.(cc|cpp|cxx|c\+\+)$`).MatchString(strings.ToLower(symbolPath))

		// In source files, it's almost always a definition
		// In header files, check if it has a body
		symbolIsDefinition := symbolIsInSourceFile || symbolHasBody

		locType := "declaration"
		if symbolIsDefinition {
			locType = "definition"
		}

		locations = append(locations, locationInfo{
			locType:      locType,
			location:     symbol.Location,
			path:         symbolPath,
			isDefinition: symbolIsDefinition,
		})

		// Get definition/declaration via textDocument/definition
		definitions, err := client.GetDefinition(symbol.Location.URI, symbol.Location.Range.Start)
		if err == nil {
			log.Debug("Found %d related location(s) via textDocument/definition", len(definitions))

			// Add the other location(s) if different from current
			for _, def := range definitions {
				defPath := client.PathFromFileURI(def.URI)

				// Skip if it's the same location
				if defPath == symbolPath && 
				   def.Range.Start.Line == symbol.Location.Range.Start.Line {
					continue
				}

				// If we started from a definition, the other location is likely a declaration
				// If we started from a declaration, the other location is likely a definition
				var otherType string

				if symbolIsDefinition {
					// We have the definition, so the other location should be the declaration
					otherType = "declaration"
				} else {
					// We have the declaration, check if the other location has a body
					hasBodyAtDef, _ := hasBody(defPath, def.Range.Start.Line)
					if hasBodyAtDef {
						otherType = "definition"
					} else {
						otherType = "declaration"
					}
				}

				locations = append(locations, locationInfo{
					locType:      otherType,
					location:     def,
					path:         defPath,
					isDefinition: otherType == "definition",
				})
			}
		} else {
			log.Debug("Failed to get related locations: %v", err)
		}

		// Sort to show declaration first, then definition
		sort.Slice(locations, func(i, j int) bool {
			if locations[i].locType == "declaration" && locations[j].locType != "declaration" {
				return true
			}
			if locations[i].locType != "declaration" && locations[j].locType == "declaration" {
				return false
			}
			return false
		})
	} else {
		// For non-function symbols, just add the location
		locations = append(locations, locationInfo{
			locType:      "definition",
			location:     symbol.Location,
			path:         symbolPath,
			isDefinition: true,
		})
	}

	// Build the result
	result := fmt.Sprintf("Found %s '%s'", symbolKindName, fullName)
	if len(symbols) > 1 {
		result += fmt.Sprintf(" (%d matches total, showing most relevant)", len(symbols))
	}
	result += "\n"

	// Get context for each location
	for i, loc := range locations {
		// Read the file
		content, err := os.ReadFile(loc.path)
		if err != nil {
			log.Debug("Failed to read file %s: %v", loc.path, err)
			continue
		}
		lines := strings.Split(string(content), "\n")

		startLine := loc.location.Range.Start.Line

		// Get folding ranges for this file to better understand code structure
		foldingRanges, err := client.GetFoldingRanges(loc.location.URI)
		if err != nil {
			log.Debug("Failed to get folding ranges: %v", err)
			foldingRanges = []lsp.FoldingRange{}
		}

		contextStart := startLine
		contextEnd := startLine

		// Find preceding comments
		commentStart := startLine
		for j := startLine - 1; j >= 0 && j >= startLine-50; j-- {
			if j >= len(lines) {
				continue
			}
			line := strings.TrimSpace(lines[j])
			if strings.HasPrefix(line, "//") || strings.HasPrefix(line, "/*") || 
			   strings.HasPrefix(line, "*") || line == "*/" {
				commentStart = j
			} else if line != "" {
				// Non-comment, non-empty line - stop
				break
			}
		}

		// Handle functions/methods based on whether they are definitions
		if (symbol.Kind == lsp.SymbolKindFunction || 
		    symbol.Kind == lsp.SymbolKindMethod || 
		    symbol.Kind == lsp.SymbolKindConstructor) {

			if loc.isDefinition {
				// This is a definition, show the complete implementation
				contextStart = commentStart

				// Find the folding range that represents this function body
				var functionRange *lsp.FoldingRange
				bestRangeSize := 0

				for _, foldRange := range foldingRanges {
					// Check if this range starts at or near the function start
					// The opening brace might be on the same line or a few lines after
					if foldRange.StartLine >= startLine-1 && foldRange.StartLine <= startLine+5 {
						rangeSize := foldRange.EndLine - foldRange.StartLine
						if rangeSize > bestRangeSize {
							functionRange = &foldRange
							bestRangeSize = rangeSize
						}
					}
				}

				if functionRange != nil {
					// Folding ranges sometimes don't include the closing brace
					// Add 1 to ensure we include it
					contextEnd = min(functionRange.EndLine+1, len(lines)-1)
				} else {
					// If no folding range found, show a reasonable amount
					contextEnd = min(startLine+50, len(lines)-1)
				}
			} else {
				// This is a declaration (no body), show just the declaration
				contextStart = commentStart
				contextEnd = loc.location.Range.End.Line
			}
		} else if symbol.Kind == lsp.SymbolKindClass || 
		          symbol.Kind == lsp.SymbolKindStruct || 
		          symbol.Kind == lsp.SymbolKindEnum {
			// Use comments we already found
			contextStart = commentStart

			// Use folding range for the body
			var classRange *lsp.FoldingRange
			for _, foldRange := range foldingRanges {
				if foldRange.StartLine >= startLine && foldRange.StartLine <= startLine+2 {
					classRange = &foldRange
					break
				}
			}

			if classRange != nil {
				// For classes, show the complete implementation
				contextEnd = classRange.EndLine
			} else {
				// If no folding range found, show a reasonable amount
				contextEnd = min(startLine+100, len(lines)-1)
			}
		} else {
			// For other symbol types (variables, typedefs, etc)
			contextStart = commentStart
			contextEnd = loc.location.Range.End.Line
		}

		// Ensure bounds are valid
		contextStart = max(0, contextStart)
		contextEnd = min(contextEnd, len(lines)-1)

		// Extract the context lines
		var extractedLines []string
		if contextStart <= contextEnd {
			extractedLines = lines[contextStart:contextEnd+1]
		}

		// Format the section header
		result += "\n"
		// Use formatLocation with full location info including column
		formattedLoc := formatLocation(client, loc.location)

		// Always show the type for functions/methods/constructors
		if symbol.Kind == lsp.SymbolKindFunction || 
		   symbol.Kind == lsp.SymbolKindMethod || 
		   symbol.Kind == lsp.SymbolKindConstructor {
			if loc.locType == "declaration" {
				result += fmt.Sprintf("From %s (declaration)\n", formattedLoc)
			} else if loc.locType == "definition" {
				result += fmt.Sprintf("From %s (definition)\n", formattedLoc)
			} else {
				result += fmt.Sprintf("From %s\n", formattedLoc)
			}
		} else {
			result += fmt.Sprintf("From %s\n", formattedLoc)
		}

		// Add the code block
		result += "```cpp\n"
		result += strings.Join(extractedLines, "\n")
		result += "\n```"

		if i < len(locations)-1 {
			result += "\n"
		}
	}

	return result, nil
}

// hasBody checks if a function/method has a body at the given location
func hasBody(filePath string, startLine int) (bool, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return false, err
	}
	
	lines := strings.Split(string(content), "\n")
	
	// Check the next few lines for an opening brace
	for i := startLine; i < min(startLine+5, len(lines)); i++ {
		if i < len(lines) && strings.Contains(lines[i], "{") {
			return true, nil
		}
	}
	
	return false, nil
}


// min returns the minimum of two integers
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max returns the maximum of two integers  
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

