package commands

import (
	"fmt"
	"strings"

	"clangd-query/internal/clangd"
	"clangd-query/internal/logger"
)

// Hierarchy shows the type hierarchy of a class/struct
func Hierarchy(client *clangd.ClangdClient, className string, limit int, log logger.Logger) (string, error) {
	log.Info("Searching for class '%s' to get type hierarchy", className)

	// First, find the class symbol
	symbols, err := client.WorkspaceSymbol(className)
	if err != nil {
		return "", err
	}

	// Filter to find the exact class (not methods or other symbols)
	var classSymbols []clangd.WorkspaceSymbol
	for _, sym := range symbols {
		if sym.Name == className &&
			(sym.Kind == clangd.SymbolKindClass || sym.Kind == clangd.SymbolKindStruct || sym.Kind == clangd.SymbolKindInterface) {
			classSymbols = append(classSymbols, sym)
		}
	}

	if len(classSymbols) == 0 {
		return fmt.Sprintf("No class named '%s' found in the codebase.", className), nil
	}

	if len(classSymbols) > 1 {
		// Multiple classes with same name, show all locations
		var locations []string
		for _, sym := range classSymbols {
			locationStr := formatLocationSimple(client, sym.Location.URI, sym.Location.Range.Start.Line)
			locations = append(locations, fmt.Sprintf("  - %s", locationStr))
		}
		return fmt.Sprintf("Multiple classes named '%s' found:\n%s\n\nPlease use a more specific query.",
			className, strings.Join(locations, "\n")), nil
	}

	classSymbol := classSymbols[0]
	classLocation := classSymbol.Location

	// Prepare type hierarchy at this location
	items, err := client.PrepareTypeHierarchy(classLocation.URI, classLocation.Range.Start)
	if err != nil {
		log.Error("Failed to prepare type hierarchy: %v", err)
		return "", err
	}

	if len(items) == 0 {
		return fmt.Sprintf(`Unable to get type hierarchy for '%s'. This might be because:
- The class is not properly defined
- Clangd doesn't support type hierarchy for this construct
- The class is in a template or macro`, className), nil
	}

	rootItem := items[0]

	// Build the complete hierarchy tree
	tree, err := buildCompleteHierarchy(client, rootItem, log)
	if err != nil {
		return "", err
	}

	return formatHierarchyTree(tree, client), nil
}

// buildCompleteHierarchy builds the complete hierarchy from a root item
func buildCompleteHierarchy(client *clangd.ClangdClient, item clangd.TypeHierarchyItem, log logger.Logger) (*HierarchyNode, error) {
	// Get immediate supertypes (base classes)
	supertypes, err := client.GetSupertypes(item)
	if err != nil {
		log.Debug("Failed to get supertypes: %v", err)
		supertypes = []clangd.TypeHierarchyItem{}
	}

	// Build supertype nodes (non-recursive - just immediate parents)
	supertypeNodes := make([]HierarchyNode, 0, len(supertypes))
	for _, supertype := range supertypes {
		supertypeNodes = append(supertypeNodes, HierarchyNode{
			Item:       supertype,
			Supertypes: []HierarchyNode{},
			Subtypes:   []HierarchyNode{},
		})
	}

	// Build complete subtype tree (fully recursive)
	subtypeTree, err := buildSubtypeTree(client, item, log, make(map[string]bool), 0)
	if err != nil {
		return nil, err
	}

	return &HierarchyNode{
		Item:       item,
		Supertypes: supertypeNodes,
		Subtypes:   subtypeTree.Subtypes,
	}, nil
}

// buildSubtypeTree recursively builds the complete subtype tree
func buildSubtypeTree(client *clangd.ClangdClient, item clangd.TypeHierarchyItem, log logger.Logger, visited map[string]bool, depth int) (*HierarchyNode, error) {
	itemID := fmt.Sprintf("%s:%d:%d", item.URI, item.Range.Start.Line, item.Range.Start.Character)

	// Prevent infinite recursion and limit depth
	if visited[itemID] || depth > 20 {
		return &HierarchyNode{
			Item:       item,
			Supertypes: []HierarchyNode{},
			Subtypes:   []HierarchyNode{},
		}, nil
	}

	visited[itemID] = true

	// Fetch immediate subtypes
	log.Debug("Fetching subtypes at depth %d for %s", depth, item.Name)
	subtypes, err := client.GetSubtypes(item)
	if err != nil {
		log.Debug("Failed to get subtypes: %v", err)
		subtypes = []clangd.TypeHierarchyItem{}
	}

	// Recursively build subtype nodes
	subtypeNodes := make([]HierarchyNode, 0, len(subtypes))
	for _, subtype := range subtypes {
		// Create a new branch-specific visited set to allow the same class
		// to appear in multiple branches of the tree
		branchVisited := make(map[string]bool)
		for k, v := range visited {
			branchVisited[k] = v
		}

		subtypeNode, err := buildSubtypeTree(client, subtype, log, branchVisited, depth+1)
		if err != nil {
			log.Debug("Failed to build subtype tree: %v", err)
			continue
		}
		subtypeNodes = append(subtypeNodes, *subtypeNode)
	}

	return &HierarchyNode{
		Item:       item,
		Supertypes: []HierarchyNode{},
		Subtypes:   subtypeNodes,
	}, nil
}

// HierarchyNode represents a node in the type hierarchy tree
type HierarchyNode struct {
	Item       clangd.TypeHierarchyItem
	Supertypes []HierarchyNode
	Subtypes   []HierarchyNode
}

// formatHierarchyTree formats the hierarchy tree into a readable string
func formatHierarchyTree(tree *HierarchyNode, client *clangd.ClangdClient) string {
	var lines []string

	// First, show all base classes (supertypes) if any
	if len(tree.Supertypes) > 0 {
		lines = append(lines, "Inherits from:")
		formatSupertypes(tree.Supertypes, &lines, client, "")
		lines = append(lines, "")
	}

	// Show the main class
	mainClassLocation := formatHierarchyItemLocation(client, tree.Item)
	detail := ""
	if tree.Item.Detail != "" {
		detail = " " + tree.Item.Detail
	}
	lines = append(lines, fmt.Sprintf("%s%s - %s", tree.Item.Name, detail, mainClassLocation))

	// Show all derived classes (subtypes)
	if len(tree.Subtypes) > 0 {
		formatSubtypes(tree.Subtypes, &lines, client, "")
	}

	return strings.Join(lines, "\n")
}

// formatSupertypes formats the base classes recursively
func formatSupertypes(nodes []HierarchyNode, lines *[]string, client *clangd.ClangdClient, prefix string) {
	for i, node := range nodes {
		isLast := i == len(nodes)-1
		connector := "├── "
		if isLast {
			connector = "└── "
		}

		location := formatHierarchyItemLocation(client, node.Item)
		detail := ""
		if node.Item.Detail != "" {
			detail = " " + node.Item.Detail
		}

		*lines = append(*lines, fmt.Sprintf("%s%s%s%s - %s", prefix, connector, node.Item.Name, detail, location))

		// Recursively show parent's supertypes
		if len(node.Supertypes) > 0 {
			newPrefix := prefix
			if isLast {
				newPrefix += "    "
			} else {
				newPrefix += "│   "
			}
			formatSupertypes(node.Supertypes, lines, client, newPrefix)
		}
	}
}

// formatSubtypes formats the derived classes recursively
func formatSubtypes(nodes []HierarchyNode, lines *[]string, client *clangd.ClangdClient, prefix string) {
	for i, node := range nodes {
		isLast := i == len(nodes)-1
		connector := "├── "
		if isLast {
			connector = "└── "
		}

		location := formatHierarchyItemLocation(client, node.Item)
		detail := ""
		if node.Item.Detail != "" {
			detail = " " + node.Item.Detail
		}

		*lines = append(*lines, fmt.Sprintf("%s%s%s%s - %s", prefix, connector, node.Item.Name, detail, location))

		// Recursively show children's subtypes
		if len(node.Subtypes) > 0 {
			newPrefix := prefix
			if isLast {
				newPrefix += "    "
			} else {
				newPrefix += "│   "
			}
			formatSubtypes(node.Subtypes, lines, client, newPrefix)
		}
	}
}
