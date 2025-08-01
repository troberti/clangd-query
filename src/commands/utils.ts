import { SymbolKind, SymbolInformation } from "vscode-languageserver-protocol";

/**
 * Convert SymbolKind enum to human-readable string.
 * @param kind - The SymbolKind enum value
 * @returns Human-readable string representation
 */
export function symbolKindToString(kind: SymbolKind): string {
  const kindMap: { [key: number]: string } = {
    1: 'file',
    2: 'module',
    3: 'namespace',
    4: 'package',
    5: 'class',
    6: 'method',
    7: 'property',
    8: 'field',
    9: 'constructor',
    10: 'enum',
    11: 'interface',
    12: 'function',
    13: 'variable',
    14: 'constant',
    15: 'string',
    16: 'number',
    17: 'boolean',
    18: 'array',
    19: 'object',
    20: 'key',
    21: 'null',
    22: 'enum member',
    23: 'struct',
    24: 'event',
    25: 'operator',
    26: 'type parameter',
  };
  return kindMap[kind] || 'symbol';
}

/**
 * Extract base name without parameters from a symbol name.
 * For methods/functions, removes parameter list but keeps parentheses.
 * @param name - The full symbol name
 * @returns Base name with () for functions/methods
 */
export function extractBaseName(name: string): string {
  if (name.includes('(')) {
    return name.substring(0, name.indexOf('(')) + '()';
  }
  return name;
}

/**
 * Build a fully qualified name for a symbol.
 * @param baseName - The base name of the symbol
 * @param containerName - The container name (optional)
 * @returns Fully qualified name like "namespace::class::method"
 */
export function buildQualifiedName(baseName: string, containerName?: string): string {
  if (containerName) {
    return `${containerName}::${baseName}`;
  }
  return baseName;
}

/**
 * Sort symbols by relevance for a given query.
 * Prefers exact matches and deprioritizes namespace symbols.
 * @param symbols - Array of symbols to sort
 * @param query - The search query
 * @returns Sorted array (modifies in place and returns it)
 */
export function sortSymbolsByRelevance(
  symbols: SymbolInformation[], 
  query: string
): SymbolInformation[] {
  return symbols.sort((a, b) => {
    // Prefer exact name matches
    const aExactMatch = a.name === query || 
      (a.containerName && `${a.containerName}::${a.name}` === query);
    const bExactMatch = b.name === query || 
      (b.containerName && `${b.containerName}::${b.name}` === query);
    
    if (aExactMatch && !bExactMatch) return -1;
    if (!aExactMatch && bExactMatch) return 1;
    
    // Avoid namespace/using statements
    if (a.kind === SymbolKind.Namespace && b.kind !== SymbolKind.Namespace) return 1;
    if (a.kind !== SymbolKind.Namespace && b.kind === SymbolKind.Namespace) return -1;
    
    // Otherwise preserve original ordering
    return 0;
  });
}

/**
 * Generate a helpful hint message for multi-word queries.
 * @param query - The multi-word query
 * @param commandName - The command being used (e.g., "search", "view")
 * @returns Formatted hint message
 */
export function formatMultiWordQueryHint(query: string, commandName: string): string {
  const firstWord = query.split(' ')[0];
  const lastWord = query.split(' ').pop() || '';
  
  return `No symbols found matching "${query}"\n\n` +
         `💡 Hint: ${commandName} only searches for single symbol names. ` +
         `Try searching for just the class or method name:\n` +
         `- ${commandName} "${firstWord}"\n` +
         (lastWord !== firstWord ? `Or if looking for a specific method, try just the method name:\n- ${commandName} "${lastWord}"` : '');
}

/**
 * Get a human-readable type prefix for a symbol.
 * Used for display purposes (e.g., "class Foo", "enum Bar").
 * @param symbol - The symbol information
 * @returns Type prefix or empty string
 */
export function getSymbolTypePrefix(symbol: SymbolInformation): string {
  switch (symbol.kind) {
    case SymbolKind.Class: return 'class';
    case SymbolKind.Method: return '';  // Methods show just the name
    case SymbolKind.Function: return '';  // Functions show just the name
    case SymbolKind.Constructor: return '';
    case SymbolKind.Enum: return 'enum';
    case SymbolKind.Interface: return 'interface';
    case SymbolKind.Struct: return 'struct';
    case SymbolKind.Namespace: return 'namespace';
    case SymbolKind.Field: return '';
    case SymbolKind.Property: return '';
    case SymbolKind.Variable: return '';
    case SymbolKind.TypeParameter: return 'template';
    default: return '';
  }
}

/**
 * Format a symbol for display with its qualified name.
 * @param symbol - The symbol to format (SymbolInformation or similar)
 * @returns Formatted string like "Foo::Bar::method()"
 */
export function formatSymbolForDisplay(symbol: any): string {
  const baseName = extractBaseName(symbol.name);
  return buildQualifiedName(baseName, symbol.containerName);
}

/**
 * Format a symbol for display with type prefix and qualified name.
 * Used when showing search results or listings where type context is helpful.
 * @param symbol - The symbol to format (SymbolInformation or similar)
 * @returns Formatted string like "class Foo::Bar" or "enum MyEnum"
 */
export function formatSymbolWithType(symbol: any): string {
  const qualifiedName = formatSymbolForDisplay(symbol);
  const prefix = getSymbolTypePrefix(symbol);
  return prefix ? `${prefix} ${qualifiedName}` : qualifiedName;
}

/**
 * Word wrap text to fit within a maximum line width.
 * Preserves paragraph breaks (empty lines).
 * @param text - The text to wrap
 * @param maxWidth - Maximum characters per line
 * @returns Array of wrapped lines
 */
export function wordWrap(text: string, maxWidth: number): string[] {
  const lines: string[] = [];
  const paragraphs = text.split('\n');
  
  for (const paragraph of paragraphs) {
    if (!paragraph.trim()) {
      lines.push('');
      continue;
    }
    
    const words = paragraph.trim().split(/\s+/);
    let currentLine = '';
    
    for (const word of words) {
      const lineWithWord = currentLine ? `${currentLine} ${word}` : word;
      
      if (lineWithWord.length > maxWidth && currentLine) {
        // Current line would be too long, start a new line
        lines.push(currentLine);
        currentLine = word;
      } else {
        // Add word to current line
        currentLine = lineWithWord;
      }
    }
    
    if (currentLine) {
      lines.push(currentLine);
    }
  }
  
  return lines;
}