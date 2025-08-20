package lsp

import "encoding/json"

// Basic LSP types

type Position struct {
	Line      int `json:"line"`
	Character int `json:"character"`
}

type Range struct {
	Start Position `json:"start"`
	End   Position `json:"end"`
}

type Location struct {
	URI   string `json:"uri"`
	Range Range  `json:"range"`
}

type TextDocumentIdentifier struct {
	URI string `json:"uri"`
}

type TextDocumentItem struct {
	URI        string `json:"uri"`
	LanguageID string `json:"languageId"`
	Version    int    `json:"version"`
	Text       string `json:"text"`
}

type TextDocumentPositionParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
	Position     Position               `json:"position"`
}

// Initialize request/response

type InitializeParams struct {
	ProcessID             *int                   `json:"processId"`
	RootURI               string                 `json:"rootUri,omitempty"`
	InitializationOptions map[string]interface{} `json:"initializationOptions,omitempty"`
	Capabilities          ClientCapabilities     `json:"capabilities"`
}

type ClientCapabilities struct {
	TextDocument TextDocumentClientCapabilities `json:"textDocument,omitempty"`
	Workspace    WorkspaceClientCapabilities    `json:"workspace,omitempty"`
}

type TextDocumentClientCapabilities struct {
	Synchronization    TextDocumentSyncClientCapabilities `json:"synchronization,omitempty"`
	Hover              HoverClientCapabilities             `json:"hover,omitempty"`
	Definition         DefinitionClientCapabilities        `json:"definition,omitempty"`
	References         ReferencesClientCapabilities        `json:"references,omitempty"`
	DocumentSymbol     DocumentSymbolClientCapabilities    `json:"documentSymbol,omitempty"`
	FoldingRange       FoldingRangeClientCapabilities      `json:"foldingRange,omitempty"`
	TypeHierarchy      TypeHierarchyClientCapabilities     `json:"typeHierarchy,omitempty"`
}

type TextDocumentSyncClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	WillSave            bool `json:"willSave,omitempty"`
	WillSaveWaitUntil   bool `json:"willSaveWaitUntil,omitempty"`
	DidSave             bool `json:"didSave,omitempty"`
}

type HoverClientCapabilities struct {
	DynamicRegistration bool     `json:"dynamicRegistration,omitempty"`
	ContentFormat       []string `json:"contentFormat,omitempty"`
}

type DefinitionClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	LinkSupport         bool `json:"linkSupport,omitempty"`
}

type ReferencesClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type DocumentSymbolClientCapabilities struct {
	DynamicRegistration               bool `json:"dynamicRegistration,omitempty"`
	SymbolKind                        map[string]interface{} `json:"symbolKind,omitempty"`
	HierarchicalDocumentSymbolSupport bool `json:"hierarchicalDocumentSymbolSupport,omitempty"`
}

type FoldingRangeClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
	RangeLimit          int  `json:"rangeLimit,omitempty"`
	LineFoldingOnly     bool `json:"lineFoldingOnly,omitempty"`
}

type TypeHierarchyClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type WorkspaceClientCapabilities struct {
	Symbol              WorkspaceSymbolClientCapabilities `json:"symbol,omitempty"`
	DidChangeWatchedFiles DidChangeWatchedFilesClientCapabilities `json:"didChangeWatchedFiles,omitempty"`
}

type WorkspaceSymbolClientCapabilities struct {
	DynamicRegistration bool                   `json:"dynamicRegistration,omitempty"`
	SymbolKind          map[string]interface{} `json:"symbolKind,omitempty"`
}

type DidChangeWatchedFilesClientCapabilities struct {
	DynamicRegistration bool `json:"dynamicRegistration,omitempty"`
}

type InitializeResult struct {
	Capabilities ServerCapabilities `json:"capabilities"`
}

type ServerCapabilities struct {
	TextDocumentSync           interface{}         `json:"textDocumentSync,omitempty"`
	HoverProvider              bool                `json:"hoverProvider,omitempty"`
	DefinitionProvider         bool                `json:"definitionProvider,omitempty"`
	DeclarationProvider        bool                `json:"declarationProvider,omitempty"`
	ReferencesProvider         bool                `json:"referencesProvider,omitempty"`
	DocumentSymbolProvider     bool                `json:"documentSymbolProvider,omitempty"`
	WorkspaceSymbolProvider    bool                `json:"workspaceSymbolProvider,omitempty"`
	FoldingRangeProvider       bool                `json:"foldingRangeProvider,omitempty"`
	TypeHierarchyProvider      bool                `json:"typeHierarchyProvider,omitempty"`
}

// Document operations

type DidOpenTextDocumentParams struct {
	TextDocument TextDocumentItem `json:"textDocument"`
}

type DidCloseTextDocumentParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type DidChangeTextDocumentParams struct {
	TextDocument   VersionedTextDocumentIdentifier `json:"textDocument"`
	ContentChanges []TextDocumentContentChangeEvent `json:"contentChanges"`
}

type VersionedTextDocumentIdentifier struct {
	TextDocumentIdentifier
	Version int `json:"version"`
}

type TextDocumentContentChangeEvent struct {
	Range       *Range `json:"range,omitempty"`
	RangeLength *int   `json:"rangeLength,omitempty"`
	Text        string `json:"text"`
}

// Hover

type HoverParams struct {
	TextDocumentPositionParams
}

type Hover struct {
	Contents MarkupContent `json:"contents"`
	Range    *Range        `json:"range,omitempty"`
}

type MarkupContent struct {
	Kind  string `json:"kind"`
	Value string `json:"value"`
}

// Document symbols

type DocumentSymbolParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type DocumentSymbol struct {
	Name           string           `json:"name"`
	Detail         string           `json:"detail,omitempty"`
	Kind           SymbolKind       `json:"kind"`
	Deprecated     bool             `json:"deprecated,omitempty"`
	Range          Range            `json:"range"`
	SelectionRange Range            `json:"selectionRange"`
	Children       []DocumentSymbol `json:"children,omitempty"`
}

type SymbolKind int

const (
	SymbolKindFile          SymbolKind = 1
	SymbolKindModule        SymbolKind = 2
	SymbolKindNamespace     SymbolKind = 3
	SymbolKindPackage       SymbolKind = 4
	SymbolKindClass         SymbolKind = 5
	SymbolKindMethod        SymbolKind = 6
	SymbolKindProperty      SymbolKind = 7
	SymbolKindField         SymbolKind = 8
	SymbolKindConstructor   SymbolKind = 9
	SymbolKindEnum          SymbolKind = 10
	SymbolKindInterface     SymbolKind = 11
	SymbolKindFunction      SymbolKind = 12
	SymbolKindVariable      SymbolKind = 13
	SymbolKindConstant      SymbolKind = 14
	SymbolKindString        SymbolKind = 15
	SymbolKindNumber        SymbolKind = 16
	SymbolKindBoolean       SymbolKind = 17
	SymbolKindArray         SymbolKind = 18
	SymbolKindObject        SymbolKind = 19
	SymbolKindKey           SymbolKind = 20
	SymbolKindNull          SymbolKind = 21
	SymbolKindEnumMember    SymbolKind = 22
	SymbolKindStruct        SymbolKind = 23
	SymbolKindEvent         SymbolKind = 24
	SymbolKindOperator      SymbolKind = 25
	SymbolKindTypeParameter SymbolKind = 26
)

func (k SymbolKind) String() string {
	switch k {
	case SymbolKindFile:
		return "File"
	case SymbolKindModule:
		return "Module"
	case SymbolKindNamespace:
		return "Namespace"
	case SymbolKindPackage:
		return "Package"
	case SymbolKindClass:
		return "Class"
	case SymbolKindMethod:
		return "Method"
	case SymbolKindProperty:
		return "Property"
	case SymbolKindField:
		return "Field"
	case SymbolKindConstructor:
		return "Constructor"
	case SymbolKindEnum:
		return "Enum"
	case SymbolKindInterface:
		return "Interface"
	case SymbolKindFunction:
		return "Function"
	case SymbolKindVariable:
		return "Variable"
	case SymbolKindConstant:
		return "Constant"
	case SymbolKindString:
		return "String"
	case SymbolKindNumber:
		return "Number"
	case SymbolKindBoolean:
		return "Boolean"
	case SymbolKindArray:
		return "Array"
	case SymbolKindObject:
		return "Object"
	case SymbolKindKey:
		return "Key"
	case SymbolKindNull:
		return "Null"
	case SymbolKindEnumMember:
		return "EnumMember"
	case SymbolKindStruct:
		return "Struct"
	case SymbolKindEvent:
		return "Event"
	case SymbolKindOperator:
		return "Operator"
	case SymbolKindTypeParameter:
		return "TypeParameter"
	default:
		return "Unknown"
	}
}

// Workspace symbols

type WorkspaceSymbolParams struct {
	Query string `json:"query"`
}

type WorkspaceSymbol struct {
	Name          string     `json:"name"`
	Kind          SymbolKind `json:"kind"`
	Location      Location   `json:"location"`
	ContainerName string     `json:"containerName,omitempty"`
}

// Folding ranges

type FoldingRangeParams struct {
	TextDocument TextDocumentIdentifier `json:"textDocument"`
}

type FoldingRange struct {
	StartLine      int     `json:"startLine"`
	StartCharacter *int    `json:"startCharacter,omitempty"`
	EndLine        int     `json:"endLine"`
	EndCharacter   *int    `json:"endCharacter,omitempty"`
	Kind           *string `json:"kind,omitempty"`
}

// References

type ReferenceParams struct {
	TextDocumentPositionParams
	Context ReferenceContext `json:"context"`
}

type ReferenceContext struct {
	IncludeDeclaration bool `json:"includeDeclaration"`
}

// Type hierarchy

type TypeHierarchyPrepareParams struct {
	TextDocumentPositionParams
}

type TypeHierarchyItem struct {
	Name           string          `json:"name"`
	Kind           SymbolKind      `json:"kind"`
	Tags           []int           `json:"tags,omitempty"`
	Detail         string          `json:"detail,omitempty"`
	URI            string          `json:"uri"`
	Range          Range           `json:"range"`
	SelectionRange Range           `json:"selectionRange"`
	Data           json.RawMessage `json:"data,omitempty"`
}

type TypeHierarchySupertypesParams struct {
	Item TypeHierarchyItem `json:"item"`
}

type TypeHierarchySubtypesParams struct {
	Item TypeHierarchyItem `json:"item"`
}

// File watching

type DidChangeWatchedFilesParams struct {
	Changes []FileEvent `json:"changes"`
}

type FileEvent struct {
	URI  string           `json:"uri"`
	Type FileChangeType   `json:"type"`
}

type FileChangeType int

const (
	FileChangeTypeCreated FileChangeType = 1
	FileChangeTypeChanged FileChangeType = 2
	FileChangeTypeDeleted FileChangeType = 3
)

// Progress notifications

type ProgressParams struct {
	Token interface{}     `json:"token"`
	Value ProgressValue   `json:"value"`
}

type ProgressValue struct {
	Kind        string  `json:"kind"`
	Title       string  `json:"title,omitempty"`
	Message     string  `json:"message,omitempty"`
	Percentage  *int    `json:"percentage,omitempty"`
	Cancellable bool    `json:"cancellable,omitempty"`
}

// Definition/Declaration

type DefinitionParams struct {
	TextDocumentPositionParams
}

type DeclarationParams struct {
	TextDocumentPositionParams
}

// Shutdown

type ShutdownParams struct{}

type ExitParams struct{}

// ParsedDocumentation represents structured documentation extracted from clangd's hover response.
// This struct provides a consistent interface for accessing various pieces of documentation
// without needing to parse raw markdown in individual commands. All parsing logic should be
// centralized in the GetDocumentation method to ensure consistency across the codebase.
type ParsedDocumentation struct {
	// raw contains the raw markdown content from clangd's hover response. This field
	// is private and for debugging purposes only. It stores the original unprocessed
	// hover text to help diagnose parsing issues when they occur.
	//
	// WARNING: This field is for debugging purposes only! Do not use this field to
	// implement parsing hacks or extract information. All necessary information should
	// be properly extracted into the other public fields. If information is missing,
	// improve the GetDocumentation() parsing logic instead of accessing this field directly.
	raw string
	
	// Description contains the cleaned documentation text without technical details like
	// size/offset/alignment information. This is the human-readable documentation that
	// explains what a symbol does, typically extracted from doc comments like @brief or
	// plain documentation text. Line breaks and formatting are preserved where meaningful.
	Description string
	
	// Inheritance describes the inheritance chain for classes and structs. This includes
	// base classes and their access levels, formatted as they appear in the class
	// declaration. For example: "public BaseClass, private Interface" indicates that
	// the class publicly inherits from BaseClass and privately inherits from Interface.
	// This field is extracted from class declarations found in code blocks.
	Inheritance string
	
	// AccessLevel indicates the access level of a member or method within its containing
	// class. Valid values are "public", "private", or "protected". This information is
	// determined by looking for access specifiers in the code blocks of hover documentation.
	// For free functions or global symbols, this field will be empty.
	AccessLevel string
	
	// Signature contains the complete signature for methods and functions, including the
	// return type, function name, and parameter list. For example: "void setValue(int x, int y)"
	// or "std::string getName() const". This is typically extracted from code blocks in the
	// hover documentation and represents how the function would be declared.
	Signature string
	
	// Type specifies the type of a field or member variable. This includes the complete
	// type specification with all qualifiers and template parameters. Examples include
	// "const View*", "std::vector<int>", or "std::optional<std::string>". This field
	// is populated for member variables but not for functions or methods.
	Type string
	
	// DefaultValue contains the default initialization value for fields that have one.
	// This is the value assigned at declaration, such as "nullptr", "42", or "{1, 2, 3}".
	// The value is stored as it appears in the source code, including any necessary
	// syntax like braces for aggregate initialization.
	DefaultValue string
	
	// ReturnType specifies the return type for methods and functions. This is extracted
	// from the "→ Type" notation that appears in clangd's hover documentation. The type
	// is stored without the arrow prefix and may include complex types with templates,
	// such as "std::optional<int>" or "const Widget*".
	ReturnType string
	
	// ParametersText contains the raw parameter documentation for methods and functions
	// if available. This is typically the full "Parameters:" section from the hover
	// documentation, formatted as a single string with proper indentation. It includes
	// parameter names, types, and any associated documentation for each parameter.
	ParametersText string
	
	// Modifiers contains method and function modifiers extracted from the signature.
	// Common modifiers include "static", "virtual", "override", "const", "explicit",
	// "inline", and "noexcept". Special markers are also included: "pure virtual" for
	// pure virtual functions (= 0), "deleted" for deleted functions (= delete), and
	// "defaulted" for defaulted functions (= default). The modifiers help understand
	// the function's behavior and constraints.
	Modifiers []string
	
	// TemplateParams contains the template parameters if this is a template function,
	// method, or class. The format includes the angle brackets and parameter declarations,
	// such as "<typename T, typename U>" or "<class T, int N>". This helps identify
	// generic code and understand the template constraints.
	TemplateParams string
}