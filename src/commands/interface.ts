import { Logger } from "../logger.js";
import { ClangdClient } from "../clangd-client.js";
import { SymbolKind, DocumentSymbol } from "vscode-languageserver-protocol";
import { symbolKindToString, wordWrap } from "./utils.js";

/**
 * Display the public interface of a class - only public methods and members.
 * This provides a clean, readable API reference without implementation details.
 * 
 * @param client - The ClangdClient instance
 * @param className - The name of the class to get the interface for
 * @param logger - Logger for debug output
 * @returns Formatted string showing the public interface
 */
export async function getInterfaceAsText(
  client: ClangdClient,
  className: string,
  logger: Logger
): Promise<string> {
  logger.info(`Getting public interface for class '${className}'`);
  
  // Search for the class
  const symbols = await client.searchSymbols(className, 10, logger);
  
  // Filter to only classes/structs
  const classSymbols = symbols.filter(sym => 
    sym.kind === SymbolKind.Class || 
    sym.kind === SymbolKind.Struct ||
    sym.kind === SymbolKind.Interface
  );
  
  if (classSymbols.length === 0) {
    return `No class or struct named '${className}' found in the codebase.`;
  }
  
  // Use the first match
  const classSymbol = classSymbols[0];
  const filePath = client.pathFromFileUri(classSymbol.location.uri);
  
  logger.debug(`Found class at ${filePath}:${classSymbol.location.range.start.line}`);
  
  // Ensure document is open
  await client.ensureDocumentOpen(filePath);
  
  // Get ALL document symbols for the file
  const docSymbols = await client.getDocumentSymbols(filePath);
  
  // Log the document symbols tree for debugging
  logger.debug('Document symbols tree:');
  logDocumentSymbolsTree(docSymbols, logger, 0);
  
  // Find the class symbol in the document symbols
  // This is more robust than using folding ranges
  const classDocSymbol = findClassInSymbolsRobust(docSymbols, className, classSymbol.location.range.start.line);
  
  if (!classDocSymbol) {
    logger.error(`Could not find class ${className} in document symbols`);
    return `Could not find class '${className}' in the document symbols.`;
  }
  
  logger.debug(`Found class ${className} at lines ${classDocSymbol.range.start.line}-${classDocSymbol.range.end.line}`);
  
  let classMembers: DocumentSymbol[] = [];
  
  // If the class has direct children, use those
  if (classDocSymbol.children && classDocSymbol.children.length > 0) {
    logger.debug(`Found ${classDocSymbol.children.length} direct children`);
    classMembers = classDocSymbol.children;
  } else {
    // Some document symbol providers might not nest members as children
    // In this case, find all symbols within the class range
    logger.debug(`No direct children found, searching within class range`);
    const classStartLine = classDocSymbol.range.start.line;
    const classEndLine = classDocSymbol.range.end.line;
    
    const allSymbols = flattenDocumentSymbols(docSymbols);
    classMembers = allSymbols.filter(sym => {
      const symLine = sym.range.start.line;
      // Symbol must be within class range but not be the class itself
      return symLine >= classStartLine && 
             symLine <= classEndLine &&
             sym !== classDocSymbol &&
             // Exclude nested classes/structs/enums (they have their own children)
             !(sym.kind === SymbolKind.Class || 
               sym.kind === SymbolKind.Struct || 
               sym.kind === SymbolKind.Enum);
    });
  }
  
  // Additional filter: exclude symbols that appear to be from nested classes
  // by checking if their name suggests they belong to a nested type
  classMembers = classMembers.filter(member => {
    // If the member name contains ::, it's likely from a nested class
    if (member.name.includes('::')) {
      logger.debug(`Filtering out nested class member: ${member.name}`);
      return false;
    }
    return true;
  });
  
  logger.debug(`Found ${classMembers.length} symbols within class range`);
  logger.debug('Class members:');
  for (const member of classMembers) {
    logger.debug(`  ${member.name} (${symbolKindToString(member.kind)}) at line ${member.range.start.line}`);
  }
  
  // Get public members by checking hover for each
  const publicMembers: PublicMember[] = [];
  
  for (const member of classMembers) {
    // Skip certain symbol types
    if (member.kind === SymbolKind.Namespace ||
        member.kind === SymbolKind.Class ||
        member.kind === SymbolKind.Struct ||
        member.kind === SymbolKind.Enum ||
        member.kind === SymbolKind.TypeParameter ||
        member.kind === SymbolKind.Variable) {  // Skip local variables
      continue;
    }
    
    // Skip nested class members (they have children of their own)
    if (member.children && member.children.length > 0) {
      logger.debug(`Skipping ${member.name} - appears to be a nested class/struct`);
      continue;
    }
    
    try {
      // Use selectionRange for hover position - this should be at the symbol name
      const hoverLine = member.selectionRange?.start.line ?? member.range.start.line;
      const hoverCol = member.selectionRange?.start.character ?? member.range.start.character;
      
      logger.debug(`Getting hover for ${member.name} at ${hoverLine}:${hoverCol}`);
      
      const doc = await client.getDocumentation(
        filePath,
        hoverLine,
        hoverCol
      );
      
      if (doc) {
        logger.debug(`Member ${member.name}: accessLevel='${doc.accessLevel}', signature='${doc.signature?.substring(0, 50)}', hasDoc=${!!doc.description}`);
        
        
        // Only include if explicitly public or if it's a method without access info
        if (doc.accessLevel === 'public' || 
            (!doc.accessLevel && member.kind === SymbolKind.Method)) {
          logger.debug(`Including public member: ${member.name} (${symbolKindToString(member.kind)})`);
          
          publicMembers.push({
            name: member.name,
            kind: member.kind,
            signature: doc.signature || member.name,
            documentation: doc.description,
            modifiers: extractModifiers(doc),
            returnType: doc.returnType,
            parametersText: doc.parametersText,
            line: member.range.start.line,
            type: doc.type,
            defaultValue: doc.defaultValue,
            templateParams: doc.templateParams
          });
        }
      } else {
        logger.debug(`No documentation for member ${member.name}`);
      }
    } catch (error) {
      logger.debug(`Failed to get documentation for member ${member.name}: ${error}`);
    }
  }
  
  logger.info(`Found ${publicMembers.length} public members`);
  
  if (publicMembers.length === 0) {
    // Try alternative approach: look for a specific public section in document symbols
    // Re-use the classDocSymbol we already found
    if (classDocSymbol && classDocSymbol.children) {
      logger.debug(`Trying alternative approach with class children`);
      
      for (const child of classDocSymbol.children) {
        try {
          const doc = await client.getDocumentation(
            filePath,
            child.range.start.line,
            child.range.start.character
          );
          
          if (doc && doc.accessLevel === 'public') {
            logger.debug(`Found public member via children: ${child.name}`);
            
            publicMembers.push({
              name: child.name,
              kind: child.kind,
              signature: doc.signature || child.name,
              documentation: doc.description,
              modifiers: extractModifiers(doc),
              returnType: doc.returnType,
              parametersText: doc.parametersText,
              line: child.range.start.line,
              type: doc.type,
              defaultValue: doc.defaultValue,
              templateParams: doc.templateParams
            });
          }
        } catch (error) {
          logger.debug(`Failed to get documentation for child ${child.name}: ${error}`);
        }
      }
    }
  }
  
  if (publicMembers.length === 0) {
    return formatEmptyInterface(className, classSymbol, client);
  }
  
  // Format the interface
  return formatInterface(className, classSymbol, publicMembers, client);
}

/**
 * Log document symbols tree for debugging.
 */
function logDocumentSymbolsTree(symbols: DocumentSymbol[], logger: Logger, indent: number): void {
  const indentStr = '  '.repeat(indent);
  
  for (const symbol of symbols) {
    const kindStr = symbolKindToString(symbol.kind);
    const range = `${symbol.range.start.line}:${symbol.range.start.character}-${symbol.range.end.line}:${symbol.range.end.character}`;
    const selRange = symbol.selectionRange ? 
      ` (sel: ${symbol.selectionRange.start.line}:${symbol.selectionRange.start.character})` : '';
    logger.debug(`${indentStr}${symbol.name} (${kindStr}) at ${range}${selRange}`);
    
    if (symbol.children && symbol.children.length > 0) {
      logger.debug(`${indentStr}  children: ${symbol.children.length}`);
      logDocumentSymbolsTree(symbol.children, logger, indent + 1);
    }
  }
}

/**
 * Flatten document symbols tree into a flat array.
 */
function flattenDocumentSymbols(symbols: DocumentSymbol[]): DocumentSymbol[] {
  const result: DocumentSymbol[] = [];
  
  for (const symbol of symbols) {
    result.push(symbol);
    if (symbol.children) {
      result.push(...flattenDocumentSymbols(symbol.children));
    }
  }
  
  return result;
}

/**
 * Find a class symbol in the document symbols tree robustly.
 * This handles cases where the symbol location from workspace/symbol
 * doesn't exactly match the document symbol location.
 */
function findClassInSymbolsRobust(
  symbols: DocumentSymbol[], 
  className: string, 
  approximateLine: number
): DocumentSymbol | null {
  // Collect all classes/structs with matching name
  const candidates: { symbol: DocumentSymbol; distance: number }[] = [];
  
  function collectCandidates(syms: DocumentSymbol[]) {
    for (const symbol of syms) {
      if ((symbol.kind === SymbolKind.Class || 
           symbol.kind === SymbolKind.Struct) &&
          symbol.name === className) {
        // Calculate distance from approximate line
        // Use the selectionRange if available (more precise), otherwise use range start
        const symbolLine = symbol.selectionRange?.start.line ?? symbol.range.start.line;
        const distance = Math.abs(symbolLine - approximateLine);
        candidates.push({ symbol, distance });
      }
      
      if (symbol.children) {
        collectCandidates(symbol.children);
      }
    }
  }
  
  collectCandidates(symbols);
  
  if (candidates.length === 0) {
    return null;
  }
  
  // If only one candidate, return it
  if (candidates.length === 1) {
    return candidates[0].symbol;
  }
  
  // Multiple candidates: sort by distance and return the closest
  candidates.sort((a, b) => a.distance - b.distance);
  return candidates[0].symbol;
}

/**
 * Extract modifiers from documentation.
 */
function extractModifiers(doc: any): string[] {
  // Use the properly extracted modifiers field
  return doc.modifiers || [];
}

/**
 * Format an empty interface.
 */
function formatEmptyInterface(
  className: string,
  classSymbol: any,
  client: ClangdClient
): string {
  const location = client.formatLocation(
    client.pathFromFileUri(classSymbol.location.uri),
    classSymbol.location.range.start.line,
    classSymbol.location.range.start.character
  );
  
  // Build fully qualified name with type prefix
  let fullName = className;
  if (classSymbol.containerName) {
    fullName = `${classSymbol.containerName}::${className}`;
  }
  
  // Add class/struct prefix
  const typePrefix = classSymbol.kind === SymbolKind.Struct ? 'struct' : 'class';
  fullName = `${typePrefix} ${fullName}`;
  
  return `${fullName} - ${location}

Public Interface:

No public members found.`;
}

/**
 * Format the interface output.
 */
function formatInterface(
  className: string,
  classSymbol: any,
  publicMembers: PublicMember[],
  client: ClangdClient
): string {
  const lines: string[] = [];
  
  // Header
  const location = client.formatLocation(
    client.pathFromFileUri(classSymbol.location.uri),
    classSymbol.location.range.start.line,
    classSymbol.location.range.start.character
  );
  
  // Build fully qualified name with type prefix
  let fullName = className;
  if (classSymbol.containerName) {
    fullName = `${classSymbol.containerName}::${className}`;
  }
  
  // Add class/struct prefix
  const typePrefix = classSymbol.kind === SymbolKind.Struct ? 'struct' : 'class';
  fullName = `${typePrefix} ${fullName}`;
  
  lines.push(`${fullName} - ${location}`);
  lines.push('');
  lines.push('Public Interface:');
  lines.push('');
  
  // Output members in the order they appear in the file
  for (let i = 0; i < publicMembers.length; i++) {
    const formatted = formatMember(publicMembers[i]);
    lines.push(...formatted.split('\n'));
    
    // Add blank line between members
    if (i < publicMembers.length - 1) {
      lines.push('');
    }
  }
  
  return lines.join('\n');
}

/**
 * Format a single member with full documentation.
 */
function formatMember(member: PublicMember): string {
  const lines: string[] = [];
  
  // Format the signature first
  let signature = '';
  
  // For fields, format with type and default value
  if (member.kind === SymbolKind.Field || member.kind === SymbolKind.Property) {
    if (member.type) {
      signature = `${member.type} ${member.name}`;
      if (member.defaultValue) {
        signature += ` = ${member.defaultValue}`;
      }
    } else if (member.signature) {
      // Use provided signature
      signature = member.signature;
      signature = signature.replace(/^(public|private|protected):\s*/, '');
      signature = signature.trim();
    } else {
      // Fallback
      signature = member.name;
    }
  } else if (member.signature) {
    // For methods/functions, use the signature
    signature = member.signature;
    signature = signature.replace(/^(public|private|protected):\s*/, '');
    signature = signature.trim();
    
    // Clean up the signature
    // Remove any backticks around types
    signature = signature.replace(/`([^`]+)`/g, '$1');
    
    // Remove trailing semicolon if present (we'll handle it separately)
    signature = signature.replace(/;\s*$/, '');
  } else {
    // Fallback format
    if (member.kind === SymbolKind.Method || member.kind === SymbolKind.Function) {
      signature = `${member.name}()`;
    } else {
      signature = member.name;
    }
  }
  
  // Add template parameters if present
  if (member.templateParams) {
    lines.push(`template ${member.templateParams}`);
  }
  
  // Add the signature line (modifiers are already part of the signature)
  lines.push(signature);
  
  // Add documentation below, indented
  if (member.documentation && member.documentation.trim()) {
    const docText = member.documentation.trim();
    
    // Clean up common documentation prefixes
    const cleanedDoc = docText
      .replace(/^\/\*\*\s*/, '')
      .replace(/^\*\/\s*$/, '')
      .replace(/^\/\/\/\s*/gm, '')
      .replace(/^\*\s*/gm, '');
    
    // Word wrap the documentation at 78 columns (80 - 2 spaces indent)
    const maxLineLength = 78;
    const wrappedLines = wordWrap(cleanedDoc, maxLineLength);
    
    for (const line of wrappedLines) {
      lines.push(`  ${line}`);
    }
  }
  
  return lines.join('\n');
}




interface PublicMember {
  name: string;
  kind: SymbolKind;
  signature: string;
  documentation?: string;
  modifiers: string[];
  returnType?: string;
  parametersText?: string;
  line?: number;  // Line number in source file for debugging
  type?: string;  // Type for fields
  defaultValue?: string;  // Default value for fields
  templateParams?: string;  // Template parameters for template methods/functions
}