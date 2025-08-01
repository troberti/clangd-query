import { Logger } from "../logger.js";
import { ClangdClient } from "../clangd-client.js";
import { SymbolKind } from "vscode-languageserver-protocol";
import { formatSymbolForDisplay, sortSymbolsByRelevance, formatMultiWordQueryHint } from "./utils.js";

/**
 * Finds all references to a symbol at the specified location and returns them as formatted text.
 * The location should be provided in the format "file:line:column" with 1-indexed line and column numbers.
 *
 * Example output:
 * ```
 * Found 5 references to symbol at clients/src/view.h:45:10:
 *
 * clients/src/view.h:45:10
 * clients/src/view.cc:123:5
 * clients/src/main.cc:89:12
 * clients/src/widget.cc:234:8
 * clients/src/widget.cc:256:15
 * ```
 *
 * @param client - The ClangdClient instance
 * @param location - Location in format "path/to/file.cpp:line:column" (1-indexed)
 * @param logger - Logger for debug output
 * @returns Human-readable text listing all references
 * @throws Error if the location format is invalid or if the request fails
 */
export async function findReferencesAsText(
  client: ClangdClient,
  location: string,
  logger: Logger
): Promise<string> {
  // Parse the location string
  const match = location.match(/^(.+):(\d+):(\d+)$/);
  if (!match) {
    throw new Error(
      `Invalid location format: "${location}"\n` +
      `Expected format: "path/to/file.cpp:line:column" (with line and column as numbers)\n` +
      `Examples:\n` +
      `  - "src/main.cpp:42:15"\n` +
      `  - "include/widget.h:100:8"\n` +
      `Note: Line and column numbers should be 1-indexed (as shown in editors)`
    );
  }

  const file = match[1];
  const line = parseInt(match[2], 10);
  const column = parseInt(match[3], 10);

  // Validate numbers
  if (isNaN(line) || line < 1) {
    throw new Error(
      `Invalid line number: ${match[2]}\n` +
      `Line numbers must be positive integers (1-indexed)`
    );
  }

  if (isNaN(column) || column < 1) {
    throw new Error(
      `Invalid column number: ${match[3]}\n` +
      `Column numbers must be positive integers (1-indexed)`
    );
  }

  logger.info(`Finding references at location: ${location}`);
  // Convert to 0-indexed for LSP
  const references = await client.findReferences(file, line - 1, column - 1, logger);

  if (references.length === 0) {
    return `No references found for symbol at ${location}`;
  }

  // Build output
  let output = `Found ${references.length} reference${references.length !== 1 ? 's' : ''} to symbol at ${location}:\n\n`;

  // Convert references to human-readable format
  for (const ref of references) {
    const refPath = client.pathFromFileUri(ref.uri);
    const formattedLocation = client.formatLocation(
      refPath,
      ref.range.start.line,
      ref.range.start.character
    );

    output += `- ${formattedLocation}\n`;
  }

  logger.debug(`Found ${references.length} references`);
  // Remove trailing newline
  return output.trimEnd();
}

/**
 * Finds all references to a symbol by name. First searches for the symbol,
 * then finds all references to the best match.
 *
 * @param client - The ClangdClient instance
 * @param symbolName - The name of the symbol to find references for
 * @param logger - Logger for debug output
 * @returns Human-readable text listing all references to the symbol
 * @throws Error if no symbols match the query or if the request fails
 */
export async function findReferencesToSymbolAsText(
  client: ClangdClient,
  symbolName: string,
  logger: Logger
): Promise<string> {
  logger.info(`Finding references to symbol: ${symbolName}`);
  // First, search for the symbol
  const symbols = await client.searchSymbols(symbolName, 20, logger);
  
  if (symbols.length === 0) {
    // Check if query has multiple words
    if (symbolName.includes(' ')) {
      return formatMultiWordQueryHint(symbolName, 'usages');
    }
    return `No symbols found matching "${symbolName}"`;
  }
  
  // Light sorting - mostly rely on clangd's built-in ranking
  const sortedSymbols = sortSymbolsByRelevance([...symbols], symbolName);
  
  // Use the best match
  const symbol = sortedSymbols[0];
  
  // Build the full symbol name for display
  const fullName = formatSymbolForDisplay(symbol);
  
  // Get the file path and position
  const filePath = client.pathFromFileUri(symbol.location.uri);
  const line = symbol.location.range.start.line;
  const column = symbol.location.range.start.character;
  
  // Find references to this symbol
  const references = await client.findReferences(filePath, line, column, logger);
  
  if (references.length === 0) {
    return `Selected symbol: ${fullName}\nNo references found for this symbol`;
  }
  
  // Build output
  let output = `Selected symbol: ${fullName}\n`;
  output += `Found ${references.length} reference${references.length !== 1 ? 's' : ''}:\n\n`;
  
  // Convert references to human-readable format
  for (const ref of references) {
    const refPath = client.pathFromFileUri(ref.uri);
    const formattedLocation = client.formatLocation(
      refPath,
      ref.range.start.line,
      ref.range.start.character
    );
    
    output += `- ${formattedLocation}\n`;
  }
  
  logger.debug(`Found ${references.length} references`);
  // Remove trailing newline
  return output.trimEnd();
}

