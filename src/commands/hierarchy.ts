import { Logger } from "../logger.js";
import { ClangdClient } from "../clangd-client.js";
import {
  SymbolKind,
  TypeHierarchyItem,
} from "vscode-languageserver-protocol";

/**
 * Represents a node in the type hierarchy tree.
 * Contains the TypeHierarchyItem and its relationships.
 */
interface HierarchyNode {
  /** The type hierarchy item for this node */
  item: TypeHierarchyItem;
  /** Parent classes/types (base classes) */
  supertypes: HierarchyNode[];
  /** Child classes/types (derived classes) */
  subtypes: HierarchyNode[];
}

/**
 * Get the complete type hierarchy for a class as formatted text.
 * Shows both base classes (supertypes) and derived classes (subtypes) in a tree format.
 * @param client - The ClangdClient instance
 * @param className - The name of the class to get hierarchy for
 * @param logger - Logger for debug output
 * @returns Formatted string showing the class hierarchy tree
 */
export async function getTypeHierarchyAsText(
  client: ClangdClient,
  className: string,
  logger: Logger
): Promise<string> {
  logger.info(`Searching for class '${className}' to get type hierarchy`);
  
  // First, find the class symbol
  const symbols = await client.searchSymbols(className, 50, logger);
  
  // Filter to find the exact class (not methods or other symbols)
  const classSymbols = symbols.filter(sym => 
    sym.name === className && 
    (sym.kind === SymbolKind.Class || sym.kind === SymbolKind.Struct || sym.kind === SymbolKind.Interface)
  );
  
  if (classSymbols.length === 0) {
    return `No class named '${className}' found in the codebase.`;
  }
  
  if (classSymbols.length > 1) {
    // Multiple classes with same name, show all locations
    const locations = classSymbols.map(sym => {
      return `  - ${client.formatUriLocation(sym.location.uri, sym.location.range.start.line)}`;
    }).join('\n');
    
    return `Multiple classes named '${className}' found:\n${locations}\n\nPlease use a more specific query.`;
  }
  
  const classSymbol = classSymbols[0];
  const classLocation = classSymbol.location;
  
  // Prepare type hierarchy at this location
  const filePath = client.pathFromFileUri(classLocation.uri);
  const hierarchyItems = await client.prepareTypeHierarchy(
    filePath,
    classLocation.range.start.line,
    classLocation.range.start.character
  );
  
  if (!hierarchyItems || hierarchyItems.length === 0) {
    return `Unable to get type hierarchy for '${className}'. This might be because:
- The class is not properly defined
- Clangd doesn't support type hierarchy for this construct
- The class is in a template or macro`;
  }
  
  const rootItem = hierarchyItems[0];
  
  // Build the complete hierarchy tree
  // We fetch supertypes only once from the root, but subtypes recursively
  const tree = await buildCompleteHierarchy(client, rootItem, logger);
  
  return formatHierarchyTree(tree, client);
}

/**
 * Build complete hierarchy starting from a root item.
 * Fetches all supertypes (non-recursive) and all subtypes (fully recursive).
 */
async function buildCompleteHierarchy(
  client: ClangdClient,
  item: TypeHierarchyItem,
  logger: Logger
): Promise<HierarchyNode> {
  // Get immediate supertypes (base classes)
  const supertypes = await client.getTypeHierarchySupertypes(item);
  
  // Build supertype nodes (non-recursive - just immediate parents)
  const supertypeNodes = supertypes.map(supertype => ({
    item: supertype,
    supertypes: [],
    subtypes: []
  }));
  
  // Build complete subtype tree (fully recursive)
  const subtypeTree = await buildSubtypeTree(client, item, logger, new Set<string>(), 0);
  
  return {
    item,
    supertypes: supertypeNodes,
    subtypes: subtypeTree.subtypes
  };
}

/**
 * Recursively build the complete subtype tree.
 */
async function buildSubtypeTree(
  client: ClangdClient,
  item: TypeHierarchyItem,
  logger: Logger,
  visited: Set<string>,
  depth: number
): Promise<HierarchyNode> {
  const itemId = `${item.uri}:${item.range.start.line}:${item.range.start.character}`;
  
  // Prevent infinite recursion and limit depth
  if (visited.has(itemId) || depth > 20) {
    return {
      item,
      supertypes: [],
      subtypes: []
    };
  }
  
  visited.add(itemId);
  
  // Fetch immediate subtypes
  logger.debug(`Fetching subtypes at depth ${depth} for ${item.name}`);
  const subtypes = await client.getTypeHierarchySubtypes(item);
  
  // Recursively build subtype nodes
  const subtypeNodes = await Promise.all(
    subtypes.map(async subtype => {
      // Create a new branch-specific visited set to allow the same class
      // to appear in multiple branches of the tree
      const branchVisited = new Set(visited);
      return buildSubtypeTree(client, subtype, logger, branchVisited, depth + 1);
    })
  );
  
  return {
    item,
    supertypes: [],
    subtypes: subtypeNodes
  };
}

/**
 * Format the hierarchy tree into a readable string with tree characters.
 */
function formatHierarchyTree(tree: HierarchyNode, client: ClangdClient): string {
  const lines: string[] = [];
  
  // First, show all base classes (supertypes) in reverse order
  if (tree.supertypes.length > 0) {
    lines.push("Inherits from:");
    formatSupertypes(tree.supertypes, lines, client, "");
    lines.push("");
  }
  
  // Show the main class
  const mainClassLocation = client.formatUriLocation(tree.item.uri, tree.item.range.start.line);
  const detail = tree.item.detail ? ` ${tree.item.detail}` : "";
  lines.push(`${tree.item.name}${detail} - ${mainClassLocation}`);
  
  // Show all derived classes (subtypes)
  if (tree.subtypes.length > 0) {
    formatSubtypes(tree.subtypes, lines, client, "");
  }
  
  return lines.join('\n');
}

/**
 * Format supertypes (base classes) recursively.
 */
function formatSupertypes(
  nodes: HierarchyNode[], 
  lines: string[], 
  client: ClangdClient,
  prefix: string
): void {
  nodes.forEach((node, index) => {
    const isLast = index === nodes.length - 1;
    const connector = isLast ? "└── " : "├── ";
    const location = client.formatUriLocation(node.item.uri, node.item.range.start.line);
    const detail = node.item.detail ? ` ${node.item.detail}` : "";
    
    lines.push(`${prefix}${connector}${node.item.name}${detail} - ${location}`);
    
    // Recursively show parent's supertypes
    if (node.supertypes.length > 0) {
      const newPrefix = prefix + (isLast ? "    " : "│   ");
      formatSupertypes(node.supertypes, lines, client, newPrefix);
    }
  });
}

/**
 * Format subtypes (derived classes) recursively.
 */
function formatSubtypes(
  nodes: HierarchyNode[], 
  lines: string[], 
  client: ClangdClient,
  prefix: string
): void {
  nodes.forEach((node, index) => {
    const isLast = index === nodes.length - 1;
    const connector = isLast ? "└── " : "├── ";
    const location = client.formatUriLocation(node.item.uri, node.item.range.start.line);
    const detail = node.item.detail ? ` ${node.item.detail}` : "";
    
    lines.push(`${prefix}${connector}${node.item.name}${detail} - ${location}`);
    
    // Recursively show children's subtypes
    if (node.subtypes.length > 0) {
      const newPrefix = prefix + (isLast ? "    " : "│   ");
      formatSubtypes(node.subtypes, lines, client, newPrefix);
    }
  });
}

