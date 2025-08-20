package commands

import (
	"fmt"
	"path/filepath"
	"strings"

	"clangd-query/internal/logger"
	"clangd-query/internal/lsp"
)

// Usages finds all references to a symbol
func Usages(client *lsp.ClangdClient, input string, limit int, log logger.Logger) ([]UsageResult, error) {
	// Parse input
	uri, position, err := parseLocationOrSymbol(client, input)
	if err != nil {
		return nil, err
	}

	// Find all references (including declaration)
	locations, err := client.GetReferences(uri, position, true)
	if err != nil {
		return nil, err
	}

	// Convert to results
	results := make([]UsageResult, 0, len(locations))
	
	for _, loc := range locations {
		file := strings.TrimPrefix(loc.URI, "file://")
		
		// Read the line containing the reference
		lines, err := lsp.ReadFileLines(file, loc.Range.Start.Line, loc.Range.Start.Line)
		if err != nil {
			continue
		}
		
		snippet := ""
		if len(lines) > 0 {
			snippet = strings.TrimSpace(lines[0])
		}
		
		// Make path relative
		if relPath, err := filepath.Rel(client.ProjectRoot, file); err == nil {
			file = relPath
		}
		
		result := UsageResult{
			File:    file,
			Line:    loc.Range.Start.Line + 1, // Convert to 1-based
			Column:  loc.Range.Start.Character + 1,
			Snippet: snippet,
		}
		results = append(results, result)
		
		// Apply limit
		if limit > 0 && len(results) >= limit {
			break
		}
	}

	if len(results) == 0 {
		return nil, fmt.Errorf("no usages found")
	}

	return results, nil
}