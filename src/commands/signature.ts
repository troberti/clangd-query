import { Logger } from "../logger.js";
import { ClangdClient, ParsedDocumentation } from "../clangd-client.js";
import { SymbolKind } from "vscode-languageserver-protocol";
import { wordWrap } from "./utils.js";

/**
 * Get function/method signatures with full type information as formatted text.
 * Shows all overloads if multiple matches are found, including documentation.
 * @param client - The ClangdClient instance
 * @param functionName - The name of the function/method to get signatures for
 * @param logger - Logger for debug output
 * @returns Formatted string showing all function signatures with documentation
 */
export async function getSignatureAsText(
  client: ClangdClient,
  functionName: string,
  logger: Logger
): Promise<string> {
  logger.info(`Searching for function/method '${functionName}' to get signatures`);
  
  // Let clangd's fuzzy search handle the matching
  // It's smart enough to match "View::SetSize" to SetSize methods in View classes
  // Search for more initially since we need to filter to only functions/methods
  const symbols = await client.searchSymbols(functionName, 20, logger);
  
  logger.debug(`Found ${symbols.length} total symbols`);
  
  // Filter to only functions and methods
  const functionSymbols = symbols.filter(sym => 
    sym.kind === SymbolKind.Function || 
    sym.kind === SymbolKind.Method ||
    sym.kind === SymbolKind.Constructor
  );
  
  logger.debug(`Filtered to ${functionSymbols.length} function/method symbols`);
  
  if (functionSymbols.length === 0) {
    return `No function or method named '${functionName}' found in the codebase.`;
  }
  
  // Limit to top 3 matches to avoid overwhelming output
  const maxResults = 3;
  const symbolsToShow = functionSymbols.slice(0, maxResults);
  
  // Get documentation for each match
  const results: string[] = [];
  
  for (const symbol of symbolsToShow) {
    const filePath = client.pathFromFileUri(symbol.location.uri);
    const line = symbol.location.range.start.line;
    const column = symbol.location.range.start.character;
    
    try {
      const doc = await client.getDocumentation(filePath, line, column);
      
      if (doc) {
        const formatted = formatSignature(symbol, doc, client);
        results.push(formatted);
      } else {
        // Fallback if no documentation available
        const location = client.formatLocation(filePath, line, column);
        results.push(`${functionName} - ${location}\n  No documentation available`);
      }
    } catch (error) {
      logger.error(`Failed to get documentation for ${functionName} at ${filePath}:${line}:${column}`, error);
      const location = client.formatLocation(filePath, line, column);
      results.push(`${functionName} - ${location}\n  Error getting documentation: ${error}`);
    }
  }
  
  // Join all results with separators
  let output = results.join('\n\n' + '─'.repeat(80) + '\n\n');
  
  // Add note about additional matches if there are more
  const remainingCount = functionSymbols.length - symbolsToShow.length;
  if (remainingCount > 0) {
    output += '\n\n' + '─'.repeat(80) + '\n\n';
    output += `... and ${remainingCount} more signature${remainingCount === 1 ? '' : 's'} not shown. Use 'search ${functionName}' to see all matches.`;
  }
  
  return output;
}

/**
 * Format a single function signature with its documentation.
 */
function formatSignature(
  symbol: any,
  doc: ParsedDocumentation,
  client: ClangdClient
): string {
  const lines: string[] = [];
  
  // Location header
  const location = client.formatLocation(
    client.pathFromFileUri(symbol.location.uri),
    symbol.location.range.start.line,
    symbol.location.range.start.character
  );
  
  // Main signature line
  lines.push(`${symbol.name} - ${location}`);
  lines.push('');
  
  // Access level and signature
  if (doc.accessLevel) {
    lines.push(`${doc.accessLevel}:`);
  }
  
  if (doc.signature) {
    lines.push(`  ${doc.signature}`);
  } else {
    // If no signature is available, show a placeholder
    lines.push(`  [Signature not available]`);
  }
  
  lines.push('');
  
  // Return type
  if (doc.returnType) {
    lines.push(`Return Type: ${doc.returnType}`);
    lines.push('');
  }
  
  // Parameters
  if (doc.parametersText) {
    lines.push(doc.parametersText);
    lines.push('');
  }
  
  // Description/documentation
  if (doc.description) {
    lines.push('Description:');
    // Word wrap the description for readability
    const wrapped = wordWrap(doc.description, 80);
    wrapped.forEach(line => lines.push(`  ${line}`));
    lines.push('');
  }
  
  // Template information
  if (doc.templateParams) {
    lines.push(`Template Parameters: ${doc.templateParams}`);
    lines.push('');
  }
  
  // Additional modifiers
  if (doc.modifiers && doc.modifiers.length > 0) {
    lines.push(`Modifiers: ${doc.modifiers.join(', ')}`);
  }
  
  return lines.join('\n');
}

