package commands

// SearchResult represents a search result
type SearchResult struct {
	Kind   string `json:"kind"`
	File   string `json:"file"`
	Line   int    `json:"line"`
	Column int    `json:"column"`
	Name   string `json:"name"`
}

// ShowResult represents a show command result
type ShowResult struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Content string `json:"content"`
	Type    string `json:"type"` // "declaration" or "definition"
}

// ViewResult represents a view command result
type ViewResult struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Content string `json:"content"`
}

// UsageResult represents a usage/reference result
type UsageResult struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Column  int    `json:"column"`
	Snippet string `json:"snippet"`
}

// HierarchyResult represents a type hierarchy result
type HierarchyResult struct {
	Tree string `json:"tree"`
}

// SignatureResult represents a signature result
type SignatureResult struct {
	Signature     string `json:"signature"`
	Documentation string `json:"documentation"`
}

// InterfaceMember represents a member of an interface
type InterfaceMember struct {
	Signature     string `json:"signature"`
	Documentation string `json:"documentation"`
	Access        string `json:"access"` // "public", "protected", "private"
}

// InterfaceResult represents an interface command result
type InterfaceResult struct {
	Name    string            `json:"name"`
	Members []InterfaceMember `json:"members"`
}