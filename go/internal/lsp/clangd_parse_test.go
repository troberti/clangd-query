package lsp

import (
	"reflect"
	"strings"
	"testing"
)

// Test helper functions to reduce boilerplate
func assertEqual(t *testing.T, got, want interface{}, field string) {
	t.Helper()
	if !reflect.DeepEqual(got, want) {
		t.Errorf("%s mismatch:\nwant: %v\ngot:  %v", field, want, got)
	}
}

func assertSliceEqual(t *testing.T, got, want []string, field string) {
	t.Helper()
	if len(got) != len(want) {
		t.Errorf("%s count mismatch:\nwant: %v\ngot:  %v", field, want, got)
		return
	}
	for i := range want {
		if i >= len(got) || got[i] != want[i] {
			t.Errorf("%s mismatch:\nwant: %v\ngot:  %v", field, want, got)
			return
		}
	}
}

// TestParseDocumentation tests the parseDocumentation function with real hover responses from clangd
func TestParseDocumentation(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  ParsedDocumentation
	}{
		{
			name: "GameObject class",
			input: `### class ` + "`GameObject`" + `  
provided by ` + `"core/game_object.h"` + `  

---
@brief Base class for all game objects in the engine  
GameObject represents any entity in the game world. It can contain  
multiple components that define its behavior and properties.  

---
` + "```cpp" + `
// In namespace game_engine
class GameObject : public Updatable, public Renderable, public std::enable_shared_from_this<GameObject>
` + "```",
			want: ParsedDocumentation{
				Description: "@brief Base class for all game objects in the engine GameObject represents any entity in the game world. It can contain multiple components that define its behavior and properties.",
				Signature:   "class GameObject : public Updatable, public Renderable, public std::enable_shared_from_this<GameObject>",
			},
		},
		{
			name: "GameObject constructor with parameter",
			input: `### constructor ` + "`GameObject`" + `  

---
Parameters:  
- ` + "`const std::string & name (aka const basic_string<char> &)`" + `

---
` + "```cpp" + `
// In GameObject
public: explicit GameObject(const std::string &name)
` + "```",
			want: ParsedDocumentation{
				AccessLevel:    "public",
				Signature:      "explicit GameObject(const std::string& name)",
				Modifiers:      []string{"explicit"},
				ParametersText: "Parameters:\n  - `const std::string & name (aka const basic_string<char> &)`",
			},
		},
		{
			name: "Engine::GetInstance static method",
			input: `### static-method ` + "`Engine::GetInstance`" + `  
provided by ` + `"core/engine.h"` + `  

---
→ ` + "`Engine &`" + `  
@brief Gets the singleton instance  
@return Reference to the engine instance  

---
` + "```cpp" + `
// In Engine
public: Engine &Engine::GetInstance()
` + "```",
			want: ParsedDocumentation{
				Description: "@brief Gets the singleton instance @return Reference to the engine instance",
				AccessLevel: "public",
				Signature:   "Engine& Engine::GetInstance()",
				ReturnType:  "Engine &",
				Modifiers:   []string{},
			},
		},
		{
			name: "GameObject::Update virtual override method",
			input: `### instance-method ` + "`Update`" + `  

---
→ ` + "`void`" + `  
Parameters:  
- ` + "`float delta_time`" + `

Updatable interface  

---
` + "```cpp" + `
// In GameObject
public: void Update(float delta_time) override
` + "```",
			want: ParsedDocumentation{
				Description:    "Updatable interface",
				AccessLevel:    "public",
				Signature:      "void Update(float delta_time) override",
				ReturnType:     "void",
				Modifiers:      []string{"override"},
				ParametersText: "Parameters:\n  - `float delta_time`",
			},
		},
		{
			name: "GameObject::GetTransform with reference return",
			input: `### instance-method ` + "`GetTransform`" + `  

---
→ ` + "`Transform &`" + `  
@brief Gets the object's transform  
@return Reference to the transform  

---
` + "```cpp" + `
// In GameObject
public: Transform &GetTransform()
` + "```",
			want: ParsedDocumentation{
				Description: "@brief Gets the object's transform @return Reference to the transform",
				AccessLevel: "public",
				Signature:   "Transform& GetTransform()",
				ReturnType:  "Transform &",
				Modifiers:   []string{},
			},
		},
		{
			name: "const method with const reference return",
			input: `### instance-method ` + "`GetName`" + `  

---
→ ` + "`const std::string & (aka const basic_string<char> &)`" + `  
@brief Gets the object's name  
@return The object's name  

---
` + "```cpp" + `
// In GameObject
public: const std::string &GetName() const
` + "```",
			want: ParsedDocumentation{
				Description: "@brief Gets the object's name @return The object's name",
				AccessLevel: "public",
				Signature:   "const std::string& GetName() const",
				ReturnType:  "const std::string & (aka const basic_string<char> &)",
				Modifiers:   []string{"const"},
			},
		},
		{
			name: "template method",
			input: `### instance-method ` + "`GetComponent`" + `  

---
→ ` + "`std::optional<std::shared_ptr<T>>`" + `  
@brief Gets a component by type  
@tparam T The component type to retrieve  
@return Optional containing the component if found  

---
` + "```cpp" + `
// In GameObject
public: template <typename T>
std::optional<std::shared_ptr<T>> GetComponent() const
` + "```",
			want: ParsedDocumentation{
				Description: "@brief Gets a component by type @tparam T The component type to retrieve @return Optional containing the component if found",
				AccessLevel: "public",
				Signature:   "template <typename T>\nstd::optional<std::shared_ptr<T>> GetComponent() const",
				ReturnType:  "std::optional<std::shared_ptr<T>>",
				Modifiers:   []string{"const"},
			},
		},
		{
			name: "protected virtual method",
			input: `### instance-method ` + "`OnCreate`" + `  

---
→ ` + "`void`" + `  
@brief Called when the object is first created  
Override this to perform initialization logic.  

---
` + "```cpp" + `
// In GameObject
protected: virtual void OnCreate()
` + "```",
			want: ParsedDocumentation{
				Description: "@brief Called when the object is first created Override this to perform initialization logic.",
				AccessLevel: "protected",
				Signature:   "virtual void OnCreate()",
				ReturnType:  "void",
				Modifiers:   []string{"virtual"},
			},
		},
		{
			name: "pure virtual method",
			input: `### instance-method ` + "`Update`" + `  

---
→ ` + "`void`" + `  
Parameters:  
- ` + "`float delta_time`" + `

@brief Updates the object state  
@param delta_time Time elapsed since last update in seconds  

---
` + "```cpp" + `
// In Updatable interface
public: virtual void Update(float delta_time) = 0
` + "```",
			want: ParsedDocumentation{
				Description:    "@brief Updates the object state @param delta_time Time elapsed since last update in seconds",
				AccessLevel:    "public",
				Signature:      "virtual void Update(float delta_time) = 0",
				ReturnType:     "void",
				Modifiers:      []string{"virtual", "pure virtual"},
				ParametersText: "Parameters:\n  - `float delta_time`",
			},
		},
		{
			name: "field with type and size info",
			input: `### field ` + "`id_`" + `  

---
Type: ` + "`uint64_t (aka unsigned long long)`" + `  
Offset: 32 bytes  
Size: 8 bytes, alignment 8 bytes  

---
` + "```cpp" + `
// In GameObject
private: uint64_t id_
` + "```",
			want: ParsedDocumentation{
				AccessLevel: "private",
				Signature:   "uint64_t id_",
				Type:        "uint64_t (aka unsigned long long)",
			},
		},
		{
			name: "static field",
			input: `### static-property ` + "`next_id_`" + `  

---
Type: ` + "`uint64_t (aka unsigned long long)`" + `  

---
` + "```cpp" + `
// In GameObject
private: static uint64_t next_id_
` + "```",
			want: ParsedDocumentation{
				AccessLevel: "private",
				Signature:   "static uint64_t next_id_",
				Type:        "uint64_t (aka unsigned long long)",
				Modifiers:   []string{"static"},
			},
		},
		{
			name: "destructor",
			input: `### destructor ` + "`~GameObject`" + `  

---
` + "```cpp" + `
// In GameObject
public: virtual ~GameObject() noexcept
` + "```",
			want: ParsedDocumentation{
				AccessLevel: "public",
				Signature:   "virtual ~GameObject() noexcept",
				Modifiers:   []string{"virtual", "noexcept"},
			},
		},
		{
			name: "defaulted constructor",
			input: `### constructor ` + "`Transform`" + `  

---
` + "```cpp" + `
// In Transform
public: Transform() = default
` + "```",
			want: ParsedDocumentation{
				AccessLevel: "public",
				Signature:   "Transform() = default",
				Modifiers:   []string{"defaulted"},
			},
		},
		{
			name: "comparison operator",
			input: `### instance-method ` + "`operator<=>`" + `  

---
→ ` + "`std::strong_ordering`" + `  
Parameters:  
- ` + "`const GameObject & other`" + `

Three-way comparison operator  

---
` + "```cpp" + `
// In GameObject
public: std::strong_ordering operator<=>(const GameObject &other) const
` + "```",
			want: ParsedDocumentation{
				Description:    "Three-way comparison operator",
				AccessLevel:    "public",
				Signature:      "std::strong_ordering operator<=>(const GameObject& other) const",
				ReturnType:     "std::strong_ordering",
				Modifiers:      []string{"const"},
				ParametersText: "Parameters:\n  - `const GameObject & other`",
			},
		},
		{
			name: "method with multiple parameters",
			input: `### instance-method ` + "`CreateProjectile`" + `  

---
→ ` + "`std::shared_ptr<GameObject>`" + `  
Parameters:  
- ` + "`const Vector3 & position`" + `
- ` + "`const Vector3 & velocity`" + `
- ` + "`float damage`" + `

---
` + "```cpp" + `
// In WeaponSystem
public: std::shared_ptr<GameObject> CreateProjectile(const Vector3 &position, const Vector3 &velocity, float damage)
` + "```",
			want: ParsedDocumentation{
				AccessLevel:    "public",
				Signature:      "std::shared_ptr<GameObject> CreateProjectile(const Vector3& position, const Vector3& velocity, float damage)",
				ReturnType:     "std::shared_ptr<GameObject>",
				ParametersText: "Parameters:\n  - `const Vector3 & position`\n  - `const Vector3 & velocity`\n  - `float damage`",
			},
		},
		{
			name: "multi-line access specifier",
			input: `### instance-method ` + "`ProcessInput`" + `  

---
→ ` + "`void`" + `  
Parameters:  
- ` + "`const InputEvent & event`" + `

---
` + "```cpp" + `
// In InputHandler
public:
  virtual void ProcessInput(const InputEvent &event) override
` + "```",
			want: ParsedDocumentation{
				AccessLevel:    "public",
				Signature:      "virtual void ProcessInput(const InputEvent& event) override",
				ReturnType:     "void",
				Modifiers:      []string{"virtual", "override"},
				ParametersText: "Parameters:\n  - `const InputEvent & event`",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDocumentation(tt.input)

			// Use helper functions for cleaner assertions
			assertEqual(t, got.Description, tt.want.Description, "Description")
			assertEqual(t, got.AccessLevel, tt.want.AccessLevel, "AccessLevel")
			assertEqual(t, got.Signature, tt.want.Signature, "Signature")
			assertEqual(t, got.ReturnType, tt.want.ReturnType, "ReturnType")
			assertEqual(t, got.Type, tt.want.Type, "Type")
			assertEqual(t, got.ParametersText, tt.want.ParametersText, "ParametersText")
			assertSliceEqual(t, got.Modifiers, tt.want.Modifiers, "Modifiers")
		})
	}
}

// TestParseDocumentationSignatureFormatting specifically tests reference and pointer formatting
func TestParseDocumentationSignatureFormatting(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		wantSignature string
	}{
		{
			name: "reference return type formatting",
			input: `### static-method ` + "`GetInstance`" + `
→ ` + "`Engine &`" + `
---
` + "```cpp" + `
public: Engine &GetInstance()
` + "```",
			wantSignature: "Engine& GetInstance()",
		},
		{
			name: "const reference return type",
			input: `### instance-method ` + "`GetName`" + `
→ ` + "`const std::string &`" + `
---
` + "```cpp" + `
public: const std::string &GetName() const
` + "```",
			wantSignature: "const std::string& GetName() const",
		},
		{
			name: "pointer return type",
			input: `### instance-method ` + "`GetParent`" + `
→ ` + "`GameObject *`" + `
---
` + "```cpp" + `
public: GameObject *GetParent()
` + "```",
			wantSignature: "GameObject* GetParent()",
		},
		{
			name: "const pointer return type",
			input: `### instance-method ` + "`GetRoot`" + `
→ ` + "`const GameObject *`" + `
---
` + "```cpp" + `
public: const GameObject *GetRoot() const
` + "```",
			wantSignature: "const GameObject* GetRoot() const",
		},
		{
			name: "reference parameters",
			input: `### instance-method ` + "`SetTransform`" + `
Parameters:
- ` + "`const Transform & transform`" + `
---
` + "```cpp" + `
public: void SetTransform(const Transform &transform)
` + "```",
			wantSignature: "void SetTransform(const Transform& transform)",
		},
		{
			name: "pointer parameters",
			input: `### instance-method ` + "`AttachTo`" + `
Parameters:
- ` + "`GameObject * parent`" + `
---
` + "```cpp" + `
public: void AttachTo(GameObject *parent)
` + "```",
			wantSignature: "void AttachTo(GameObject* parent)",
		},
		{
			name: "multiple reference and pointer params",
			input: `### instance-method ` + "`Connect`" + `
Parameters:
- ` + "`Node * from`" + `
- ` + "`Node * to`" + `
- ` + "`const Options & opts`" + `
---
` + "```cpp" + `
public: bool Connect(Node *from, Node *to, const Options &opts)
` + "```",
			wantSignature: "bool Connect(Node* from, Node* to, const Options& opts)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDocumentation(tt.input)
			assertEqual(t, got.Signature, tt.wantSignature, "Signature")
		})
	}
}

// TestExtractTypeFromField tests that we properly extract the Type field for variables
func TestExtractTypeFromField(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		wantType string
	}{
		{
			name: "simple type field",
			input: `### field ` + "`count_`" + `
Type: ` + "`int`" + `
---
` + "```cpp" + `
private: int count_
` + "```",
			wantType: "int",
		},
		{
			name: "complex template type",
			input: `### field ` + "`components_`" + `
Type: ` + "`std::vector<std::shared_ptr<Component>>`" + `
---
` + "```cpp" + `
private: std::vector<std::shared_ptr<Component>> components_
` + "```",
			wantType: "std::vector<std::shared_ptr<Component>>",
		},
		{
			name: "type with alias",
			input: `### field ` + "`id_`" + `
Type: ` + "`uint64_t (aka unsigned long long)`" + `
---
` + "```cpp" + `
private: uint64_t id_
` + "```",
			wantType: "uint64_t (aka unsigned long long)",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseDocumentation(tt.input)
			assertEqual(t, got.Type, tt.wantType, "Type")
		})
	}
}

// TestComplexRealWorldExamples tests complex real-world hover responses
func TestComplexRealWorldExamples(t *testing.T) {
	// Test a complex template method with constraints
	complexTemplate := `### instance-method ` + "`emplace`" + `
→ ` + "`std::pair<iterator, bool>`" + `
Parameters:
- ` + "`Args &&... args`" + `

@brief Constructs element in-place
@tparam Args Types of arguments to forward to the constructor
@param args Arguments to forward to the constructor of the element
@return A pair consisting of an iterator to the inserted element and a bool denoting success

---
` + "```cpp" + `
// In Container
public: template <typename... Args>
  requires std::constructible_from<T, Args...>
std::pair<iterator, bool> emplace(Args &&...args)
` + "```"

	got := parseDocumentation(complexTemplate)

	// Validate the complex template parsing
	if !strings.Contains(got.Signature, "template") {
		t.Errorf("Expected template in signature, got: %q", got.Signature)
	}

	assertEqual(t, got.ReturnType, "std::pair<iterator, bool>", "ReturnType")

	if !strings.Contains(got.Description, "@brief Constructs element in-place") {
		t.Errorf("Expected description to contain brief, got: %q", got.Description)
	}
}
