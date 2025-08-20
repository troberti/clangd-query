package commands

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/firi/clangd-query/internal/lsp"
)

// Signature shows function signatures with documentation
func Signature(client *lsp.ClangdClient, input string) ([]SignatureResult, error) {
	// Parse input
	uri, position, err := parseLocationOrSymbol(client, input)
	if err != nil {
		return nil, err
	}

	// Get hover information
	hover, err := client.GetHover(uri, position)
	if err != nil {
		return nil, err
	}

	if hover == nil || hover.Contents.Value == "" {
		return nil, fmt.Errorf("no signature information found")
	}

	// Parse hover content
	parsed := parseHoverContent(hover.Contents.Value)
	
	results := make([]SignatureResult, 0)
	
	// Extract signatures from declaration text
	if parsed.DeclarationText != "" {
		// Split by newlines for overloads
		signatures := strings.Split(parsed.DeclarationText, "\n")
		
		for _, sig := range signatures {
			sig = strings.TrimSpace(sig)
			if sig == "" {
				continue
			}
			
			result := SignatureResult{
				Signature:     sig,
				Documentation: parsed.Documentation,
			}
			results = append(results, result)
		}
	}

	if len(results) == 0 {
		// Fallback: use the raw hover content
		result := SignatureResult{
			Signature:     hover.Contents.Value,
			Documentation: "",
		}
		results = append(results, result)
	}

	return results, nil
}

// HoverParsed represents parsed hover information
type HoverParsed struct {
	DeclarationText string
	Documentation   string
	Parameters      []ParameterInfo
}

// ParameterInfo represents parameter information
type ParameterInfo struct {
	Name          string
	Type          string
	Documentation string
}

// parseHoverContent parses structured hover content
func parseHoverContent(content string) HoverParsed {
	parsed := HoverParsed{}
	
	// Try to parse as clangd's structured format
	// Clangd returns hover info in a specific format
	
	// Look for code blocks
	if strings.Contains(content, "```") {
		// Extract code block
		start := strings.Index(content, "```")
		end := strings.Index(content[start+3:], "```")
		if end > 0 {
			codeBlock := content[start+3 : start+3+end]
			// Remove language identifier if present
			if strings.HasPrefix(codeBlock, "cpp\n") || strings.HasPrefix(codeBlock, "c\n") {
				codeBlock = codeBlock[strings.Index(codeBlock, "\n")+1:]
			}
			parsed.DeclarationText = strings.TrimSpace(codeBlock)
			
			// Rest is documentation
			docStart := start + 3 + end + 3
			if docStart < len(content) {
				parsed.Documentation = strings.TrimSpace(content[docStart:])
			}
		}
	} else {
		// Try to parse as JSON (some versions of clangd return JSON)
		var jsonHover map[string]interface{}
		if err := json.Unmarshal([]byte(content), &jsonHover); err == nil {
			if decl, ok := jsonHover["declarationText"].(string); ok {
				parsed.DeclarationText = decl
			}
			if doc, ok := jsonHover["documentation"].(string); ok {
				parsed.Documentation = doc
			}
		} else {
			// Plain text
			lines := strings.Split(content, "\n")
			if len(lines) > 0 {
				parsed.DeclarationText = lines[0]
				if len(lines) > 1 {
					parsed.Documentation = strings.Join(lines[1:], "\n")
				}
			}
		}
	}
	
	// Clean up documentation
	parsed.Documentation = strings.TrimSpace(parsed.Documentation)
	
	// Word wrap documentation at 80 characters
	if parsed.Documentation != "" {
		parsed.Documentation = wordWrap(parsed.Documentation, 80)
	}
	
	return parsed
}

// wordWrap wraps text at the specified column
func wordWrap(text string, width int) string {
	if width <= 0 {
		return text
	}
	
	var result strings.Builder
	lines := strings.Split(text, "\n")
	
	for _, line := range lines {
		if len(line) <= width {
			result.WriteString(line)
			result.WriteString("\n")
			continue
		}
		
		// Wrap long lines
		words := strings.Fields(line)
		currentLine := ""
		
		for _, word := range words {
			if currentLine == "" {
				currentLine = word
			} else if len(currentLine)+1+len(word) <= width {
				currentLine += " " + word
			} else {
				result.WriteString(currentLine)
				result.WriteString("\n")
				currentLine = word
			}
		}
		
		if currentLine != "" {
			result.WriteString(currentLine)
			result.WriteString("\n")
		}
	}
	
	return strings.TrimSuffix(result.String(), "\n")
}