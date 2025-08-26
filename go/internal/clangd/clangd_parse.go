package clangd

import "strings"

// Reads a complete function signature from lines starting at startIdx.
// Handles multi-line signatures by continuing to read lines until parentheses are balanced.
// Returns the complete signature and the index of the last line consumed.
func readCompleteSignature(lines []string, startIdx int, firstLine string) (string, int) {
	if !strings.Contains(firstLine, "(") {
		return firstLine, startIdx
	}

	if hasBalancedParentheses(firstLine) {
		return firstLine, startIdx
	}

	// Multi-line signature - continue reading until balanced
	fullSignature := firstLine
	lastIdx := startIdx
	for i := startIdx + 1; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])
		if line != "" && !strings.HasPrefix(line, "// ") {
			fullSignature += " " + line
			lastIdx = i
			if hasBalancedParentheses(fullSignature) {
				break
			}
		}
	}
	return fullSignature, lastIdx
}

// Processes a signature line, handling both template declarations and regular signatures.
// For templates, reads the next line to get the actual method signature.
// For regular signatures, handles multi-line cases by reading continuation lines.
func processSignatureLine(lines []string, startIdx int, signatureLine string, doc *ParsedDocumentation) int {
	// Check if this is a template declaration
	if strings.HasPrefix(signatureLine, "template") && strings.HasSuffix(signatureLine, ">") {
		// This is a template, look for the actual method signature on the next line
		for i := startIdx + 1; i < len(lines); i++ {
			methodLine := strings.TrimSpace(lines[i])
			if methodLine != "" && !strings.HasPrefix(methodLine, "// ") {
				// Read complete method signature (might be multi-line)
				completeMethodSig, _ := readCompleteSignature(lines, i, methodLine)
				// Combine template and method signature
				doc.Signature = signatureLine + "\n" + formatSignature(completeMethodSig)
				// Extract modifiers and other info from the method signature part
				extractSignatureDetails(completeMethodSig, doc)
				return i
			}
		}
		return startIdx
	}

	// Not a template - read the complete signature (handling multi-line)
	completeSignature, lastIdx := readCompleteSignature(lines, startIdx, signatureLine)
	doc.Signature = formatSignature(completeSignature)
	extractSignatureDetails(completeSignature, doc)
	return lastIdx
}

// Parse hover content from clangd into structured documentation.
//
// This function extracts various pieces of information from the markdown-formatted
// hover response, including signatures, return types, parameters, modifiers, and
// documentation text. It handles various C++ constructs like templates, constructors,
// destructors, and different access levels.
func parseDocumentation(content string) *ParsedDocumentation {
	doc := &ParsedDocumentation{
		raw: content,
	}

	// Extract code block if present
	codeBlock := ""
	if idx := strings.Index(content, "```"); idx >= 0 {
		start := idx + 3
		// Skip language identifier
		if nlIdx := strings.Index(content[start:], "\n"); nlIdx >= 0 {
			start += nlIdx + 1
		}
		if endIdx := strings.Index(content[start:], "```"); endIdx >= 0 {
			codeBlock = strings.TrimSpace(content[start : start+endIdx])
		}
	}

	// Parse code block for signature and modifiers
	if codeBlock != "" {
		lines := strings.Split(codeBlock, "\n")

		// Sometimes clangd returns the signature on multiple lines
		// e.g., "public:\n  virtual void Update(...)"
		// We need to handle this case properly

		for i, line := range lines {
			line = strings.TrimSpace(line)

			// Skip context lines
			if strings.HasPrefix(line, "// In ") {
				continue
			}

			// Check for access level on its own line
			if line == "public:" || line == "private:" || line == "protected:" {
				doc.AccessLevel = strings.TrimSuffix(line, ":")
				// The next non-empty line(s) should be the signature
				// For templates, this might span multiple lines
				for j := i + 1; j < len(lines); j++ {
					nextLine := strings.TrimSpace(lines[j])
					if nextLine != "" && !strings.HasPrefix(nextLine, "// ") {
						processSignatureLine(lines, j, nextLine, doc)
						break
					}
				}
				continue
			}

			// Check if line starts with access level (e.g., "public: virtual void...")
			if strings.HasPrefix(line, "public: ") {
				doc.AccessLevel = "public"
				line = strings.TrimPrefix(line, "public: ")
			} else if strings.HasPrefix(line, "private: ") {
				doc.AccessLevel = "private"
				line = strings.TrimPrefix(line, "private: ")
			} else if strings.HasPrefix(line, "protected: ") {
				doc.AccessLevel = "protected"
				line = strings.TrimPrefix(line, "protected: ")
			}

			// This is likely the signature (if we haven't found it yet)
			if doc.Signature == "" && line != "" && !strings.HasSuffix(line, ":") {
				// Remove access level prefix if present in the signature
				signatureLine := line
				if strings.HasPrefix(line, "public: ") {
					signatureLine = strings.TrimPrefix(line, "public: ")
				} else if strings.HasPrefix(line, "private: ") {
					signatureLine = strings.TrimPrefix(line, "private: ")
				} else if strings.HasPrefix(line, "protected: ") {
					signatureLine = strings.TrimPrefix(line, "protected: ")
				}

				processSignatureLine(lines, i, signatureLine, doc)
			}
		}
	}

	// Extract documentation text from content
	// Parse content line by line to extract various pieces of information
	lines := strings.Split(content, "\n")
	var descLines []string
	inParameters := false

	for _, line := range lines {
		// Stop processing if we hit the code block
		if strings.HasPrefix(line, "```") {
			break
		}

		line = strings.TrimSpace(line)

		// Skip empty lines and separator lines
		if line == "" || line == "---" {
			continue
		}

		// Skip header lines
		if strings.HasPrefix(line, "###") || strings.HasPrefix(line, "provided by") {
			continue
		}

		// Extract Type field for variables/fields
		if strings.HasPrefix(line, "Type:") {
			typeStr := strings.TrimSpace(strings.TrimPrefix(line, "Type:"))
			doc.Type = strings.Trim(typeStr, "`")
			continue
		}

		// Skip other technical details
		if strings.HasPrefix(line, "Size:") ||
			strings.HasPrefix(line, "Offset:") ||
			strings.Contains(line, "alignment") {
			continue
		}

		// Check for return type indicator
		if strings.HasPrefix(line, "→") {
			if doc.ReturnType == "" {
				doc.ReturnType = strings.TrimSpace(strings.TrimPrefix(line, "→"))
				doc.ReturnType = strings.Trim(doc.ReturnType, "`")
			}
			continue
		}

		// Check for Parameters section
		if strings.HasPrefix(line, "Parameters:") {
			inParameters = true
			doc.ParametersText = "Parameters:"
			continue
		}

		// Handle parameter lines (they start with -)
		if inParameters && strings.HasPrefix(line, "-") {
			doc.ParametersText += "\n  " + line
			continue
		} else if inParameters && line != "" && !strings.HasPrefix(line, "-") {
			// End of parameters section
			inParameters = false
		}

		// Documentation lines (@brief, @param, etc. or just plain text)
		if strings.HasPrefix(line, "@") || (!inParameters && line != "") {
			descLines = append(descLines, line)
		}
	}

	// Join description lines
	if len(descLines) > 0 {
		doc.Description = strings.Join(descLines, " ")
	}

	return doc
}

// extractModifiers extracts C++ modifiers from a signature line
// hasBalancedParentheses checks if a string has balanced parentheses.
// This is used to determine if a function signature is complete, handling
// cases like std::function<void()> where there are nested parentheses.
func hasBalancedParentheses(s string) bool {
	count := 0
	for _, ch := range s {
		switch ch {
		case '(':
			count++
		case ')':
			count--
			if count < 0 {
				return false // More closing than opening
			}
		}
	}
	return count == 0
}

func extractModifiers(line string) []string {
	var modifiers []string

	// For const, only consider it a modifier if it appears after the closing parenthesis
	// (i.e., it's a const member function)
	if parenIdx := strings.LastIndex(line, ")"); parenIdx >= 0 {
		afterParen := line[parenIdx:]
		if strings.Contains(afterParen, " const") || strings.HasSuffix(afterParen, " const") {
			modifiers = append(modifiers, "const")
		}
	}

	// Other modifiers can appear anywhere in the signature
	// but we should be smarter about word boundaries
	modifierKeywords := []string{"virtual", "static", "override", "inline", "explicit", "noexcept"}

	// Split into words to check for exact matches
	words := strings.Fields(line)
	for _, word := range words {
		// Remove punctuation for comparison
		cleanWord := strings.Trim(word, "(),;")
		for _, mod := range modifierKeywords {
			if cleanWord == mod {
				modifiers = append(modifiers, mod)
				break
			}
		}
	}

	// Check for pure virtual
	if strings.Contains(line, "= 0") {
		modifiers = append(modifiers, "pure virtual")
	}

	// Check for deleted/defaulted
	if strings.Contains(line, "= delete") {
		modifiers = append(modifiers, "deleted")
	}
	if strings.Contains(line, "= default") {
		modifiers = append(modifiers, "defaulted")
	}

	return modifiers
}

// isModifier checks if a word is a C++ modifier
func isModifier(word string) bool {
	modifiers := []string{"virtual", "static", "override", "const", "inline", "explicit", "noexcept"}
	for _, mod := range modifiers {
		if word == mod {
			return true
		}
	}
	return false
}

// extractSignatureDetails extracts return type, modifiers, and parameters from a signature
func extractSignatureDetails(signature string, doc *ParsedDocumentation) {
	// Extract modifiers
	doc.Modifiers = extractModifiers(signature)

	// Extract return type and parameters if it's a function
	if strings.Contains(signature, "(") {
		parenIdx := strings.Index(signature, "(")
		beforeParen := signature[:parenIdx]

		// Check if this is a constructor or destructor (they don't have return types)
		// Constructors/destructors contain the class name right before the parenthesis
		// and don't have a separate return type
		parts := strings.Fields(beforeParen)
		isConstructorOrDestructor := false

		// Check for destructor (starts with ~) or constructor (class name before parenthesis)
		if len(parts) > 0 {
			lastPart := parts[len(parts)-1]
			// Check if it's a destructor
			if strings.HasPrefix(lastPart, "~") {
				isConstructorOrDestructor = true
			} else {
				// Check if it's a constructor by looking for known constructor patterns
				// Constructor names typically appear after "explicit" or as the last identifier
				for _, part := range parts {
					if part == "explicit" || part == lastPart && !isModifier(part) && !strings.Contains(part, "::") {
						// Check if the name matches a typical constructor pattern
						// (starts with uppercase letter, which is common for class names)
						if len(lastPart) > 0 && lastPart[0] >= 'A' && lastPart[0] <= 'Z' {
							isConstructorOrDestructor = true
							break
						}
					}
				}
			}
		}

		// Only extract return type if it's not a constructor/destructor
		// and if ReturnType hasn't already been set (e.g., from the → line)
		if !isConstructorOrDestructor && doc.ReturnType == "" {
			// Skip known modifiers and class qualifiers to find return type
			for _, part := range parts {
				if !isModifier(part) && !strings.Contains(part, "::") {
					// Don't set return type if it looks like a method/function name
					// (We already have the return type from the → line in most cases)
					break
				}
			}
		}

		// Extract parameters
		if closeIdx := strings.Index(signature[parenIdx:], ")"); closeIdx > 0 {
			paramStr := signature[parenIdx+1 : parenIdx+closeIdx]
			if paramStr != "" && paramStr != "void" {
				// Only set ParametersText if it hasn't been set already
				if doc.ParametersText == "" {
					params := strings.Split(paramStr, ",")
					doc.ParametersText = "Parameters:"
					for _, param := range params {
						doc.ParametersText += "\n  - `" + strings.TrimSpace(param) + "`"
					}
				}
			}
		}
	}
}

// formatSignature normalizes the formatting of a C++ signature by ensuring that
// reference (&) and pointer (*) symbols are placed next to the type rather than
// the variable or function name. This provides consistent formatting that matches
// C++ style conventions where these symbols are part of the type specification.
func formatSignature(signature string) string {
	// Handle the case where signature might be multiline (e.g., template on one line, method on next)
	if strings.Contains(signature, "\n") {
		return signature // Keep multiline signatures as-is for now
	}

	// First, normalize spaces around & and *
	// Replace patterns like "Type &" with "Type&" and "Type *" with "Type*"
	result := signature

	// Handle references - move & next to the type but keep space after for names
	result = strings.ReplaceAll(result, " &", "&")

	// Handle pointers - move * next to the type but keep space after for names
	result = strings.ReplaceAll(result, " *", "*")

	// Now we need to ensure there's a space between Type& and the next identifier
	// We'll process the signature to add spaces where needed
	finalResult := ""
	i := 0
	for i < len(result) {
		if i < len(result)-1 && (result[i] == '&' || result[i] == '*') {
			// Add the & or *
			finalResult += string(result[i])
			i++
			// Check if the next character is alphanumeric (part of an identifier)
			// If so, add a space before it
			if i < len(result) && isIdentifierChar(result[i]) {
				finalResult += " "
			}
		} else {
			finalResult += string(result[i])
			i++
		}
	}

	return finalResult
}

// isIdentifierChar checks if a character can be part of a C++ identifier
func isIdentifierChar(ch byte) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') ||
		(ch >= '0' && ch <= '9') || ch == '_'
}
