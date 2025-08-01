import { Logger } from "../logger.js";
import { ClangdClient } from "../clangd-client.js";
import { SymbolKind, Location } from "vscode-languageserver-protocol";
import * as fs from "fs";
import * as path from "path";
import { symbolKindToString, formatSymbolForDisplay } from "./utils.js";

/**
 * Helper to check if a file exists
 */
async function fileExists(filePath: string): Promise<boolean> {
  try {
    await fs.promises.access(filePath, fs.constants.F_OK);
    return true;
  } catch {
    return false;
  }
}

/**
 * Helper to check if a function/method has a body at the given location
 */
async function hasBody(client: ClangdClient, filePath: string, startLine: number): Promise<boolean> {
  const absolutePath = client.toAbsolutePath(filePath);
  const content = await fs.promises.readFile(absolutePath, 'utf-8');
  const lines = content.split('\n');
  
  // Check the next few lines for an opening brace
  for (let i = startLine; i < Math.min(startLine + 5, lines.length); i++) {
    if (lines[i].includes('{')) {
      return true;
    }
  }
  
  return false;
}

/**
 * Get contextual code around a symbol showing both declaration and definition.
 * This command intelligently handles C++ declaration/definition split.
 * 
 * For functions/methods, it shows BOTH:
 * - Declaration from header (with doc comments and signature)
 * - Definition from source (with implementation details)
 * 
 * Examples:
 * - "Scene::LoadResources" - shows declaration in .h and implementation in .cpp
 * - "ResourceManager" - shows class definition with context
 * - "CreateView" - shows function declaration and implementation
 * 
 * @param client - The ClangdClient instance
 * @param query - The symbol name to search for
 * @param logger - Logger for debug output
 * @returns Human-readable text with contextual code
 * @throws Error if no symbols match the query
 */
export async function getShowAsText(
  client: ClangdClient,
  query: string,
  logger: Logger
): Promise<string> {
  logger.info(`Getting context for: ${query}`);
  
  // Search for the symbol with a higher limit to ensure we find it
  const symbols = await client.searchSymbols(query, 50, logger);
  
  if (symbols.length === 0) {
    return `No symbols found matching "${query}"`;
  }
  
  // Use the best match - symbols are already sorted by relevance from clangd
  const symbol = symbols[0];
  
  const symbolKindName = symbolKindToString(symbol.kind);
  const fullName = formatSymbolForDisplay(symbol);
  
  // Get the symbol's location
  const symbolPath = client.pathFromFileUri(symbol.location.uri);
  const absoluteSymbolPath = client.toAbsolutePath(symbolPath);
  await client.ensureDocumentOpen(symbolPath);
  
  logger.debug(`Found ${symbolKindName} '${fullName}' at ${symbolPath}`);
  
  // For methods and functions, try to find both declaration and definition
  const locations: Array<{type: string, location: Location, path: string, isDefinition: boolean}> = [];
  
  if (symbol.kind === SymbolKind.Function || 
      symbol.kind === SymbolKind.Method || 
      symbol.kind === SymbolKind.Constructor) {
    
    // Determine if the symbol location is a definition or declaration
    const symbolHasBody = await hasBody(client, symbolPath, symbol.location.range.start.line);
    const symbolIsInSourceFile = symbolPath.match(/\.(cc|cpp|cxx|c\+\+)$/i) !== null;
    
    // In source files, it's almost always a definition
    // In header files, check if it has a body
    const symbolIsDefinition = symbolIsInSourceFile || symbolHasBody;
    
    locations.push({
      type: symbolIsDefinition ? 'definition' : 'declaration',
      location: symbol.location,
      path: symbolPath,
      isDefinition: symbolIsDefinition
    });
    
    // Get definition/declaration via textDocument/definition
    try {
      const definitions = await client.getDefinition(
        symbolPath,
        symbol.location.range.start.line,
        symbol.location.range.start.character,
        logger
      );
      
      logger.debug(`Found ${definitions.length} related location(s) via textDocument/definition`);
      
      // Add the other location(s) if different from current
      for (const def of definitions) {
        const defPath = client.pathFromFileUri(def.uri);
        
        // Skip if it's the same location
        if (defPath === symbolPath && 
            def.range.start.line === symbol.location.range.start.line) {
          continue;
        }
        
        // If we started from a definition, the other location is likely a declaration
        // If we started from a declaration, the other location is likely a definition
        let otherType: 'declaration' | 'definition';
        
        if (symbolIsDefinition) {
          // We have the definition, so the other location should be the declaration
          otherType = 'declaration';
        } else {
          // We have the declaration, check if the other location has a body
          const hasBodyAtDef = await hasBody(client, defPath, def.range.start.line);
          otherType = hasBodyAtDef ? 'definition' : 'declaration';
        }
        
        locations.push({
          type: otherType,
          location: def,
          path: defPath,
          isDefinition: otherType === 'definition'
        });
      }
    } catch (error) {
      logger.debug(`Failed to get related locations: ${error}`);
    }
    
    // Sort to show declaration first, then definition
    locations.sort((a, b) => {
      if (a.type === 'declaration' && b.type !== 'declaration') return -1;
      if (a.type !== 'declaration' && b.type === 'declaration') return 1;
      return 0;
    });
  } else {
    // For non-function symbols, just add the location
    locations.push({
      type: 'definition',
      location: symbol.location,
      path: symbolPath,
      isDefinition: true
    });
  }
  
  // Build the result
  let result = `Found ${symbolKindName} '${fullName}'`;
  if (symbols.length > 1) {
    result += ` (${symbols.length} matches total, showing most relevant)`;
  }
  result += '\n';
  
  // Get context for each location
  for (let i = 0; i < locations.length; i++) {
    const loc = locations[i];
    const absolutePath = client.toAbsolutePath(loc.path);
    
    // Read the file
    const content = await fs.promises.readFile(absolutePath, 'utf-8');
    const lines = content.split('\n');
    
    const startLine = loc.location.range.start.line;
    
    // Get folding ranges for this file to better understand code structure
    await client.ensureDocumentOpen(loc.path);
    const foldingRanges = await client.getFoldingRanges(loc.path, logger);
    
    let contextStart = startLine;
    let contextEnd = startLine;
    
    // Find preceding comments
    let commentStart = startLine;
    for (let j = startLine - 1; j >= 0 && j >= startLine - 50; j--) {
      const line = lines[j]?.trim() || '';
      if (line.startsWith('//') || line.startsWith('/*') || line.startsWith('*') || line === '*/') {
        commentStart = j;
      } else if (line !== '') {
        // Non-comment, non-empty line - stop
        break;
      }
    }
    
    // Handle functions/methods based on whether they are definitions
    if ((symbol.kind === SymbolKind.Function || 
         symbol.kind === SymbolKind.Method || 
         symbol.kind === SymbolKind.Constructor)) {
      
      if (loc.isDefinition) {
        // This is a definition, show the complete implementation
        contextStart = commentStart;
        
        // Find the folding range that represents this function body
        // Functions often have multiple folding ranges (signature, body, etc.)
        // We want the largest one that encompasses the entire function
        let functionRange = null;
        let bestRangeSize = 0;
        
        for (const range of foldingRanges) {
          // Check if this range starts at or near the function start
          // The opening brace might be on the same line or a few lines after
          if (range.startLine >= startLine - 1 && range.startLine <= startLine + 5) {
            const rangeSize = range.endLine - range.startLine;
            if (rangeSize > bestRangeSize) {
              functionRange = range;
              bestRangeSize = rangeSize;
            }
          }
        }
        
        if (functionRange) {
          // Folding ranges sometimes don't include the closing brace
          // Add 1 to ensure we include it
          contextEnd = Math.min(functionRange.endLine + 1, lines.length - 1);
        } else {
          // Fallback: manual brace counting
          let braceCount = 0;
          let foundOpeningBrace = false;
          
          for (let j = startLine; j < lines.length && j < startLine + 100; j++) {
            const line = lines[j];
            for (const char of line) {
              if (char === '{') {
                braceCount++;
                foundOpeningBrace = true;
              } else if (char === '}') {
                braceCount--;
              }
            }
            
            contextEnd = j;
            
            if (foundOpeningBrace && braceCount === 0) {
              break;
            }
          }
        }
      } else {
        // This is a declaration (no body), show declaration + a bit of context
        contextStart = commentStart;
        contextEnd = loc.location.range.end.line;
        
        // Include a couple lines after to show the next method/member for context
        contextEnd = Math.min(contextEnd + 2, lines.length - 1);
      }
    }
    
    // For other types (classes, structs, etc), use folding ranges if available
    else if (symbol.kind === SymbolKind.Class || 
             symbol.kind === SymbolKind.Struct || 
             symbol.kind === SymbolKind.Enum) {
      
      // Use comments we already found
      contextStart = commentStart;
      
      // Use folding range for the body
      let classRange = null;
      for (const range of foldingRanges) {
        if (range.startLine >= startLine && range.startLine <= startLine + 2) {
          classRange = range;
          break;
        }
      }
      
      if (classRange) {
        // For classes, show up to 30 lines to give a good overview
        contextEnd = Math.min(classRange.endLine, startLine + 30);
      } else {
        // Fallback: show 20 lines for class/struct/enum definitions
        contextEnd = Math.min(startLine + 20, lines.length - 1);
      }
    }
    // For other symbol types (variables, typedefs, etc)
    else {
      contextStart = commentStart;
      contextEnd = loc.location.range.end.line;
      // Show a few more lines for context
      contextEnd = Math.min(contextEnd + 5, lines.length - 1);
    }
    
    // Extract the context lines
    const extractedLines = lines.slice(contextStart, contextEnd + 1);
    
    // Format the section header
    result += '\n';
    const formattedLoc = client.formatLocation(loc.path, startLine, 0);
    
    // Always show the type for functions/methods/constructors
    if (symbol.kind === SymbolKind.Function || 
        symbol.kind === SymbolKind.Method || 
        symbol.kind === SymbolKind.Constructor) {
      if (loc.type === 'declaration') {
        result += `From ${formattedLoc} (declaration)\n`;
      } else if (loc.type === 'definition') {
        result += `From ${formattedLoc} (definition)\n`;
      } else {
        result += `From ${formattedLoc}\n`;
      }
    } else {
      result += `From ${formattedLoc}\n`;
    }
    
    // Add the code block
    result += '```cpp\n';
    result += extractedLines.join('\n');
    result += '\n```';
    
    if (i < locations.length - 1) {
      result += '\n';
    }
  }
  
  return result;
}

