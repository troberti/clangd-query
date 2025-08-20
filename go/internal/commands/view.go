package commands

import (
	"fmt"
	"path/filepath"
	"strings"

	"clangd-query/internal/logger"
	"clangd-query/internal/lsp"
)

// View extracts complete source code using folding ranges
func View(client *lsp.ClangdClient, input string, log logger.Logger) (*ViewResult, error) {
	// Parse input
	uri, position, err := parseLocationOrSymbol(client, input)
	if err != nil {
		return nil, err
	}

	// Get folding ranges for the document
	ranges, err := client.GetFoldingRanges(uri)
	if err != nil {
		// Fallback to document symbols if folding ranges not available
		return viewWithSymbols(client, uri, position)
	}

	// Find the folding range that contains our position
	var bestRange *lsp.FoldingRange
	
	for i := range ranges {
		r := &ranges[i]
		if r.StartLine <= position.Line && position.Line <= r.EndLine {
			// Check for consecutive folding ranges pattern
			// For functions with 3+ parameters, clangd creates separate ranges
			if i+1 < len(ranges) {
				next := &ranges[i+1]
				if next.StartLine == r.EndLine+1 || next.StartLine == r.EndLine {
					// Use the second range (the body)
					r = next
				}
			}
			
			// Use the smallest containing range
			if bestRange == nil || 
			   (r.EndLine-r.StartLine < bestRange.EndLine-bestRange.StartLine) {
				bestRange = r
			}
		}
	}

	if bestRange == nil {
		return nil, fmt.Errorf("no folding range found at position")
	}

	// Read the content
	file := strings.TrimPrefix(uri, "file://")
	
	// For classes/structs/enums, include preceding comments
	startLine := bestRange.StartLine
	if shouldIncludeComments(ranges, bestRange) {
		// Look for comments before the range
		if startLine > 0 {
			lines, _ := lsp.ReadFileLines(file, startLine-5, startLine-1)
			for i := len(lines) - 1; i >= 0; i-- {
				trimmed := strings.TrimSpace(lines[i])
				if trimmed != "" && !strings.HasPrefix(trimmed, "//") && !strings.HasPrefix(trimmed, "/*") {
					break
				}
				startLine--
			}
		}
	}

	lines, err := lsp.ReadFileLines(file, startLine, bestRange.EndLine)
	if err != nil {
		return nil, err
	}

	content := strings.Join(lines, "\n")
	
	// Make path relative
	if relPath, err := filepath.Rel(client.ProjectRoot, file); err == nil {
		file = relPath
	}

	return &ViewResult{
		File:    file,
		Line:    startLine + 1, // Convert to 1-based
		Column:  1,
		Content: content,
	}, nil
}

// viewWithSymbols uses document symbols as fallback
func viewWithSymbols(client *lsp.ClangdClient, uri string, position lsp.Position) (*ViewResult, error) {
	symbols, err := client.GetDocumentSymbols(uri)
	if err != nil {
		return nil, err
	}

	// Find symbol at position
	var targetSymbol *lsp.DocumentSymbol
	var findSymbol func([]lsp.DocumentSymbol)
	
	findSymbol = func(syms []lsp.DocumentSymbol) {
		for i := range syms {
			s := &syms[i]
			if s.Range.Start.Line <= position.Line && position.Line <= s.Range.End.Line {
				targetSymbol = s
				// Check children for more specific match
				if len(s.Children) > 0 {
					findSymbol(s.Children)
				}
			}
		}
	}
	
	findSymbol(symbols)
	
	if targetSymbol == nil {
		return nil, fmt.Errorf("no symbol found at position")
	}

	// Read the content
	file := strings.TrimPrefix(uri, "file://")
	lines, err := lsp.ReadFileLines(file, targetSymbol.Range.Start.Line, targetSymbol.Range.End.Line)
	if err != nil {
		return nil, err
	}

	content := strings.Join(lines, "\n")
	
	// Make path relative
	if relPath, err := filepath.Rel(client.ProjectRoot, file); err == nil {
		file = relPath
	}

	return &ViewResult{
		File:    file,
		Line:    targetSymbol.Range.Start.Line + 1,
		Column:  targetSymbol.Range.Start.Character + 1,
		Content: content,
	}, nil
}

// shouldIncludeComments checks if we should include preceding comments
func shouldIncludeComments(ranges []lsp.FoldingRange, target *lsp.FoldingRange) bool {
	// This is a heuristic - include comments for top-level definitions
	// Check if this range is not nested inside another
	for i := range ranges {
		r := &ranges[i]
		if r != target && r.StartLine < target.StartLine && target.EndLine < r.EndLine {
			return false // Nested inside another range
		}
	}
	return true
}