package commands

import (
	"encoding/json"
	"fmt"
	"strings"

	"clangd-query/internal/logger"
	"clangd-query/internal/lsp"
)

// Signature shows function signatures with documentation
func Signature(client *lsp.ClangdClient, input string, log logger.Logger) (string, error) {
	// Parse input
	uri, position, err := parseLocationOrSymbol(client, input)
	if err != nil {
		log.Error("Failed to parse input: %v", err)
		return "", err
	}

	// Get hover information
	hover, err := client.GetHover(uri, position)
	if err != nil {
		log.Error("Failed to get hover information: %v", err)
		return "", err
	}

	if hover == nil || hover.Contents.Value == "" {
		log.Error("No signature information found")
		return "", fmt.Errorf("no signature information found")
	}

	// Parse hover content
	parsed := parseHoverContent(hover.Contents.Value)
	
	var output strings.Builder
	
	// Extract signatures from declaration text
	if parsed.DeclarationText != "" {
		// Split by newlines for overloads
		signatures := strings.Split(parsed.DeclarationText, "\n")
		
		for i, sig := range signatures {
			sig = strings.TrimSpace(sig)
			if sig == "" {
				continue
			}
			
			if i > 0 {
				output.WriteString("\n\n")
			}
			
			output.WriteString("Signature: ")
			output.WriteString(sig)
			
			if parsed.Documentation != "" {
				output.WriteString("\n\nDocumentation:\n")
				output.WriteString(parsed.Documentation)
			}
		}
	} else {
		// Fallback: use the raw hover content
		output.WriteString("Signature: ")
		output.WriteString(hover.Contents.Value)
	}

	return output.String(), nil
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
		lines := wordWrap(parsed.Documentation, 80)
		parsed.Documentation = strings.Join(lines, "\n")
	}
	
	return parsed
}

