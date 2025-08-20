package commands

import (
	"fmt"
	"strings"

	"clangd-query/internal/logger"
	"clangd-query/internal/lsp"
)

// Hierarchy shows the type hierarchy of a class/struct
func Hierarchy(client *lsp.ClangdClient, input string, limit int, log logger.Logger) (*HierarchyResult, error) {
	// Parse input
	uri, position, err := parseLocationOrSymbol(client, input)
	if err != nil {
		return nil, err
	}

	// Prepare type hierarchy
	items, err := client.PrepareTypeHierarchy(uri, position)
	if err != nil {
		return nil, err
	}

	if len(items) == 0 {
		return nil, fmt.Errorf("no type hierarchy found at position")
	}

	item := items[0]
	
	var tree strings.Builder
	tree.WriteString(fmt.Sprintf("%s\n", item.Name))
	
	// Get supertypes (base classes)
	supertypes, err := client.GetSupertypes(item)
	if err == nil && len(supertypes) > 0 {
		tree.WriteString("\nBase classes:\n")
		for _, super := range supertypes {
			tree.WriteString(fmt.Sprintf("  └── %s\n", super.Name))
		}
	}
	
	// Get subtypes (derived classes) recursively
	subtypes, err := getSubtypesRecursive(client, item, 0, limit, make(map[string]bool))
	if err == nil && len(subtypes) > 0 {
		tree.WriteString("\nDerived classes:\n")
		printSubtypeTree(&tree, subtypes, "", true)
	}

	return &HierarchyResult{
		Tree: tree.String(),
	}, nil
}

// getSubtypesRecursive recursively gets all subtypes with cycle detection
func getSubtypesRecursive(client *lsp.ClangdClient, item lsp.TypeHierarchyItem, depth, maxDepth int, visited map[string]bool) ([]TypeNode, error) {
	if maxDepth > 0 && depth >= maxDepth {
		return nil, nil
	}
	
	// Create unique key for this item
	key := fmt.Sprintf("%s:%s", item.URI, item.Name)
	if visited[key] {
		return nil, nil // Cycle detected
	}
	visited[key] = true
	
	subtypes, err := client.GetSubtypes(item)
	if err != nil {
		return nil, err
	}
	
	nodes := make([]TypeNode, 0, len(subtypes))
	for _, subtype := range subtypes {
		node := TypeNode{
			Name: subtype.Name,
		}
		
		// Get children recursively
		children, _ := getSubtypesRecursive(client, subtype, depth+1, maxDepth, visited)
		node.Children = children
		
		nodes = append(nodes, node)
	}
	
	return nodes, nil
}

// TypeNode represents a node in the type hierarchy tree
type TypeNode struct {
	Name     string
	Children []TypeNode
}

// printSubtypeTree prints the subtype tree with Unicode box drawing
func printSubtypeTree(tree *strings.Builder, nodes []TypeNode, prefix string, isRoot bool) {
	for i, node := range nodes {
		isLast := i == len(nodes)-1
		
		if !isRoot {
			tree.WriteString(prefix)
			if isLast {
				tree.WriteString("└── ")
			} else {
				tree.WriteString("├── ")
			}
		} else {
			tree.WriteString("  ")
			if isLast {
				tree.WriteString("└── ")
			} else {
				tree.WriteString("├── ")
			}
		}
		
		tree.WriteString(node.Name)
		tree.WriteString("\n")
		
		// Print children
		childPrefix := prefix
		if !isRoot {
			if isLast {
				childPrefix += "    "
			} else {
				childPrefix += "│   "
			}
		} else {
			if isLast {
				childPrefix = "      "
			} else {
				childPrefix = "  │   "
			}
		}
		
		if len(node.Children) > 0 {
			printSubtypeTree(tree, node.Children, childPrefix, false)
		}
	}
}