import { Logger } from "../logger.js";
import { ClangdClient } from "../clangd-client.js";
import { SymbolKind, FoldingRange, DocumentSymbol } from "vscode-languageserver-protocol";
import * as fs from "fs";
import { symbolKindToString, formatSymbolForDisplay, sortSymbolsByRelevance, formatMultiWordQueryHint } from "./utils.js";

/**
 * View the complete source code of a symbol (class, function, method, etc).
 * This is a semantic viewer that understands C++ structure and returns complete implementations.
 * 
 * Features:
 * - Uses fuzzy search to find symbols by name
 * - Returns the COMPLETE implementation (full function bodies, entire class definitions)
 * - Works for both headers (.h) and implementation files (.cc/.cpp)
 * - Automatically handles overloads and template specializations
 * 
 * Examples:
 * - "View::SetSize" - returns the complete SetSize method implementation
 * - "ResourceManager" - returns the entire ResourceManager class definition
 * - "LoadSceneResources" - returns the full function body with all its code
 * - "StringFormat" - returns the complete StringFormat function
 * 
 * Why use this instead of grep/search:
 * - Gets the COMPLETE symbol definition/implementation automatically
 * - Handles C++ syntax correctly (namespaces, overloads, templates)
 * - Much faster than searching through multiple files
 * - Returns properly formatted code in a single operation
 * 
 * @param client - The ClangdClient instance
 * @param query - The symbol name to search for (fuzzy matching)
 * @param logger - Logger for debug output
 * @returns Human-readable text with the complete source code
 * @throws Error if no symbols match the query
 */
export async function viewSourceCodeAsText(
  client: ClangdClient,
  query: string,
  logger: Logger
): Promise<string> {
  logger.info(`Viewing source code for: ${query}`);
  // Search for the symbol
  const symbols = await client.searchSymbols(query, 20, logger); // Get top matches - clangd already ranks well
  
  if (symbols.length === 0) {
    // Check if query has multiple words
    if (query.includes(' ')) {
      return formatMultiWordQueryHint(query, 'view');
    }
    return `No symbols found matching "${query}"`;
  }
  
  // Light sorting - mostly rely on clangd's built-in ranking, just filter out unwanted matches
  const sortedSymbols = sortSymbolsByRelevance([...symbols], query);
  
  // Use the best match
  const symbol = sortedSymbols[0];
  
  // Get the file path and ensure it's open
  const filePath = client.pathFromFileUri(symbol.location.uri);
  const absolutePath = client.toAbsolutePath(filePath);
  await client.ensureDocumentOpen(filePath);
  
  // Find the symbol line first
  const symbolLine = symbol.location.range.start.line;
  
  // Use folding ranges to get the full extent of symbols.
  // This is the key to getting complete function bodies from .cc files!
  // While document symbols only give us limited info for implementation files,
  // folding ranges give us the complete range of foldable regions like function bodies.
  const foldingRanges = await client.getFoldingRanges(filePath, logger);
  
  logger.debug(`Got ${foldingRanges.length} folding ranges`);
  
  // Read the file content
  const content = await fs.promises.readFile(absolutePath, 'utf-8');
  const lines = content.split('\n');
  
  let startLine: number;
  let endLine: number;
  let foundRange: FoldingRange | null = null;
  
  // For classes/structs/enums/functions/methods, the folding range often starts 
  // at or after the declaration line (where the opening brace is).
  // We need to find the right folding range that represents the body of our symbol.
  
  if (symbol.kind === SymbolKind.Class || 
      symbol.kind === SymbolKind.Struct || 
      symbol.kind === SymbolKind.Enum ||
      symbol.kind === SymbolKind.Interface ||
      symbol.kind === SymbolKind.Function ||
      symbol.kind === SymbolKind.Method) {
    // Look for folding ranges near the symbol
    let rangeAtSymbol: FoldingRange | null = null;
    let rangeAfterSymbol: FoldingRange | null = null;
    
    // First, find a range at the symbol line
    for (const range of foldingRanges) {
      if (range.startLine === symbolLine) {
        rangeAtSymbol = range;
        break;
      }
    }
    
    // IMPORTANT: Handle the "consecutive folding ranges" pattern.
    // 
    // For functions with 3+ parameters on multiple lines, clangd creates TWO folding ranges:
    // 1. Parameter list range: starts at symbol line, ends at the closing ')'
    // 2. Function body range: starts at the '{', ends at the closing '}'
    //
    // Example:
    //   void foo(int a,          // <- Symbol at line 10, param range starts here
    //            int b,
    //            int c,
    //            int d) {        // <- Param range ends at line 13, body range starts
    //     // function body      // <- Body range continues
    //   }                       // <- Body range ends
    //
    // The key insight: if we find a folding range that starts right where another ends
    // (or within 1 line), they're likely parameter list + body ranges, and we want the second.
    //
    // Note: Functions with ≤2 parameters don't get parameter list folding ranges, only body ranges.
    if (rangeAtSymbol) {
      for (const range of foldingRanges) {
        // Look for a range that starts at or within 1 line of where the first ends
        if (range.startLine >= rangeAtSymbol.endLine && 
            range.startLine <= rangeAtSymbol.endLine + 1) {
          rangeAfterSymbol = range;
          break;
        }
      }
    }
    
    // If no range at symbol, find the closest one after
    if (!rangeAtSymbol) {
      let bestDistance = Infinity;
      for (const range of foldingRanges) {
        if (range.startLine > symbolLine) {
          const distance = range.startLine - symbolLine;
          if (distance < bestDistance) {
            bestDistance = distance;
            rangeAfterSymbol = range;
          }
        }
      }
    }
    
    // Decision logic:
    // - If we have consecutive ranges (parameter list + body), use the second one (the body)
    // - Otherwise, use whichever range we found (it's the body for all other cases)
    if (rangeAtSymbol && rangeAfterSymbol) {
      // We have consecutive ranges - the first is likely a parameter list
      foundRange = rangeAfterSymbol;
      logger.debug(`Found consecutive folding ranges - using the second one as function body`);
    } else if (rangeAtSymbol) {
      foundRange = rangeAtSymbol;
    } else {
      foundRange = rangeAfterSymbol;
    }
  }
  
  // If we didn't find a range yet (for other symbol types), use the original logic
  if (!foundRange) {
    for (const range of foldingRanges) {
      if (range.startLine <= symbolLine && symbolLine <= range.endLine) {
        // This range contains our symbol
        if (!foundRange || 
            (range.endLine - range.startLine) < (foundRange.endLine - foundRange.startLine)) {
          foundRange = range;
        }
      }
    }
  }
  
  if (foundRange) {
    // For classes/structs/enums/functions/methods, we want to include the 
    // declaration line(s) too, not just the body. The folding range starts 
    // at the '{', but we want to start from the symbol location.
    if (symbol.kind === SymbolKind.Class || 
        symbol.kind === SymbolKind.Struct || 
        symbol.kind === SymbolKind.Enum ||
        symbol.kind === SymbolKind.Interface ||
        symbol.kind === SymbolKind.Function ||
        symbol.kind === SymbolKind.Method) {
      startLine = symbolLine;  // Start from the declaration
      endLine = foundRange.endLine;  // End at the closing brace
      logger.debug(`Using adjusted range for ${symbolKindToString(symbol.kind)}: ${startLine}-${endLine} (symbol at ${symbolLine}, fold at ${foundRange.startLine}-${foundRange.endLine})`);
    } else {
      startLine = foundRange.startLine;
      endLine = foundRange.endLine;
    }
  } else {
    // Fallback: try document symbols (works better for headers)
    const docSymbols = await client.getDocumentSymbols(filePath);
    
    
    if (docSymbols.length > 0) {
      // Try to find the matching symbol
      const targetLine = symbol.location.range.start.line;
      
      const findSymbolAtPosition = (symbols: DocumentSymbol[]): DocumentSymbol | null => {
        for (const docSymbol of symbols) {
          // Check if this symbol's range contains our target position
          if (docSymbol.range.start.line <= targetLine && targetLine <= docSymbol.range.end.line) {
            // Check if the name matches
            if (docSymbol.name === symbol.name) {
              return docSymbol;
            }
          }
          
          // Check children recursively
          if (docSymbol.children && docSymbol.children.length > 0) {
            const childResult = findSymbolAtPosition(docSymbol.children);
            if (childResult) return childResult;
          }
        }
        return null;
      };
      
      const matchingSymbol = findSymbolAtPosition(docSymbols);
      
      if (matchingSymbol) {
        startLine = matchingSymbol.range.start.line;
        endLine = matchingSymbol.range.end.line;
      } else {
        // Final fallback to search result range
        startLine = symbol.location.range.start.line;
        endLine = symbol.location.range.end.line;
      }
    } else {
      // Final fallback to search result range
      startLine = symbol.location.range.start.line;
      endLine = symbol.location.range.end.line;
    }
  }
  
  // For classes/structs/enums, check for preceding comment blocks
  let commentStartLine = startLine;
  if (symbol.kind === SymbolKind.Class || 
      symbol.kind === SymbolKind.Struct || 
      symbol.kind === SymbolKind.Enum ||
      symbol.kind === SymbolKind.Interface) {
    // Look backwards from the symbol line to find comment blocks
    let inCommentBlock = false;
    for (let i = startLine - 1; i >= 0 && i >= startLine - 50; i--) {
      const line = lines[i]?.trim() || '';
      
      // Check if this line is part of a comment
      if (line.startsWith('//') || line.startsWith('/*') || line.startsWith('*') || line === '*/') {
        commentStartLine = i;
        inCommentBlock = true;
      } else if (line === '') {
        // Empty line - only continue if we're already in a comment block
        if (!inCommentBlock) {
          break;
        }
      } else {
        // Non-comment, non-empty line - stop looking
        break;
      }
    }
  }
  
  const codeLines = lines.slice(commentStartLine, endLine + 1);
  
  // Build the symbol description
  const symbolKindName = symbolKindToString(symbol.kind);
  const fullName = formatSymbolForDisplay(symbol);
  
  const symbolDescription = `${symbolKindName} '${fullName}'`;
  
  // Format the location
  const formattedLocation = client.formatLocation(
    filePath,
    symbol.location.range.start.line,
    symbol.location.range.start.character
  );
  
  // Check if we found multiple matches
  let multipleMatchesNote = "";
  if (symbols.length > 1) {
    multipleMatchesNote = `\n(Found ${symbols.length} matches, showing the most relevant one)`;
  }
  
  // Build the result
  const result = `Found ${symbolDescription} at ${formattedLocation}${multipleMatchesNote}\n\n\`\`\`cpp\n${codeLines.join('\n')}\n\`\`\``;
  
  logger.debug(`Retrieved ${codeLines.length} lines of source code`);
  return result;
}

